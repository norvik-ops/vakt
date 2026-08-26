// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth

import (
	"net/http"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

// RefreshSessionInfo is returned by GET /auth/sessions.
type RefreshSessionInfo struct {
	ID         string    `json:"id"`
	DeviceHint string    `json:"device_hint,omitempty"`
	LastUsed   time.Time `json:"last_used"`
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	IsCurrent  bool      `json:"is_current,omitempty"`
}

// SessionHandler handles per-device session listing and revocation.
type SessionHandler struct {
	db    *pgxpool.Pool
	redis *redis.Client
}

// NewSessionHandler constructs a SessionHandler.
func NewSessionHandler(db *pgxpool.Pool, rdb *redis.Client) *SessionHandler {
	return &SessionHandler{db: db, redis: rdb}
}

// ListSessions returns all active (non-expired) sessions for the authenticated user.
// GET /api/v1/auth/sessions
func (h *SessionHandler) ListSessions(c echo.Context) error {
	userID, ok := c.Get("user_id").(string)
	if !ok || userID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	// orgid-lint: global — caller's own sessions, scoped by user_id from the auth token
	rows, err := h.db.Query(c.Request().Context(), `
		SELECT id::text, device_hint, last_used, created_at, expires_at
		FROM refresh_sessions
		WHERE user_id = $1::uuid AND expires_at > NOW()
		ORDER BY last_used DESC`,
		userID,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	defer rows.Close()

	// Frontend sendet die im Login bekommene session_id als Header mit, damit wir
	// die aktuelle Session im UI markieren können. Kein Sicherheitsmechanismus —
	// nur kosmetisch ("diese hier").
	currentSessionID := c.Request().Header.Get("X-Vakt-Session-Id")

	var sessions []RefreshSessionInfo
	for rows.Next() {
		var s RefreshSessionInfo
		if err := rows.Scan(&s.ID, &s.DeviceHint, &s.LastUsed, &s.CreatedAt, &s.ExpiresAt); err != nil {
			continue
		}
		if currentSessionID != "" && s.ID == currentSessionID {
			s.IsCurrent = true
		}
		sessions = append(sessions, s)
	}
	if sessions == nil {
		sessions = []RefreshSessionInfo{}
	}
	return c.JSON(http.StatusOK, sessions)
}

// RevokeSession deletes a specific session owned by the authenticated user and
// removes the corresponding refresh token from Redis.
// DELETE /api/v1/auth/sessions/:id
func (h *SessionHandler) RevokeSession(c echo.Context) error {
	userID, ok := c.Get("user_id").(string)
	if !ok || userID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	sessionID := c.Param("id")

	// Delete the row and return token_hash so we can remove it from Redis.
	// orgid-lint: global — caller's own session, scoped by (id, user_id) from the auth token
	var tokenHash string
	err := h.db.QueryRow(c.Request().Context(), `
		DELETE FROM refresh_sessions
		WHERE id = $1::uuid AND user_id = $2::uuid
		RETURNING token_hash`,
		sessionID, userID,
	).Scan(&tokenHash)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "session not found"})
	}

	// Best-effort Redis removal; the 30-day TTL is a fallback.
	if h.redis != nil {
		_ = h.redis.Del(c.Request().Context(), "refresh:"+tokenHash)
	}

	return c.NoContent(http.StatusNoContent)
}

// RevokeAllOtherSessions deletes all refresh sessions for the user except the
// current one (identified by the token in the Authorization header or cookie).
// DELETE /api/v1/auth/sessions  (no :id = "all others")
func (h *SessionHandler) RevokeAllOtherSessions(c echo.Context) error {
	userID, ok := c.Get("user_id").(string)
	if !ok || userID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	// Collect token hashes to delete from Redis before removing rows.
	var rows []string
	var query string
	var args []any

	// Frontend signalisiert die "current" Session über den X-Vakt-Session-Id Header.
	// Ohne Header → revoke ALL (Panic-Button-Pfad), inklusive der aktuellen Session.
	currentSessionID := c.Request().Header.Get("X-Vakt-Session-Id")
	if currentSessionID != "" {
		// orgid-lint: global — caller's own sessions, scoped by user_id from the auth token
		query = `DELETE FROM refresh_sessions WHERE user_id = $1::uuid AND id != $2::uuid RETURNING token_hash`
		args = []any{userID, currentSessionID}
	} else {
		// orgid-lint: global — caller's own sessions, scoped by user_id from the auth token
		query = `DELETE FROM refresh_sessions WHERE user_id = $1::uuid RETURNING token_hash`
		args = []any{userID}
	}

	dbRows, err := h.db.Query(c.Request().Context(), query, args...)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	defer dbRows.Close()
	for dbRows.Next() {
		var hash string
		if scanErr := dbRows.Scan(&hash); scanErr == nil {
			rows = append(rows, "refresh:"+hash)
		}
	}
	// R1-21-A06: the enumeration used to run unchecked. A failure part-way
	// through leaves the DELETE committed but only SOME refresh keys removed from
	// Redis, and the caller was told "other sessions revoked" all the same — the
	// success-for-work-not-done shape. The rows are gone either way (the DELETE
	// ran), so the honest answer is to say the cleanup was incomplete.
	if err := dbRows.Err(); err != nil {
		log.Error().Err(err).Str("user_id", userID).Msg("revoke sessions: enumerating revoked tokens failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "sessions were removed but their tokens could not all be invalidated — please retry",
			"code":  "AUTH_REVOKE_INCOMPLETE",
		})
	}
	dbRows.Close()

	// Remove from Redis in bulk.
	if h.redis != nil && len(rows) > 0 {
		_ = h.redis.Del(c.Request().Context(), rows...)
	}

	// The panic-button path — no X-Vakt-Session-Id, so EVERY session including
	// the caller's own was just deleted. Deleting refresh rows alone leaves the
	// access tokens already handed out valid for the rest of their hour: they are
	// stateless Paseto, and only a pw_version bump makes the middleware reject
	// them. Someone who clicks "revoke everything" has to be rid of the access,
	// not of the ability to renew it (R1-21-A06).
	//
	// Deliberately NOT done on the "all others" path above: pw_version is
	// per-user, so bumping it there would invalidate the caller's own access
	// token too, and the frontend answers a 401 with a hard logout (no refresh
	// retry — api/client.ts). That half of the finding needs a per-session claim
	// in the token and is reported, not silently claimed.
	//
	// R1-W7A-N3: the bump reports its outcome now. On the panic-button path a
	// failed bump means the thing the button promises did not happen — the
	// access tokens are still good — so this answers 503 rather than the usual
	// "revoked". The refresh rows above ARE gone either way, which is why the
	// message says the revocation is incomplete rather than that it failed.
	if currentSessionID == "" {
		if err := bumpPwVersionWith(c.Request().Context(), h.db, h.redis, userID); err != nil {
			log.Error().Err(err).Str("user_id", userID).
				Msg("revoke all sessions: pw_version bump not confirmed — access tokens still valid")
			return c.JSON(http.StatusServiceUnavailable, map[string]string{
				"error": "sessions were removed but the access tokens could not be invalidated — please try again",
				"code":  "SESSION_REVOCATION_INCOMPLETE",
			})
		}
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "other sessions revoked"})
}
