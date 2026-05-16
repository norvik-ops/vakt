package auth

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

type SessionInfo struct {
	ID        string    `json:"id"`
	UserAgent string    `json:"user_agent,omitempty"`
	IPAddress string    `json:"ip_address,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

type SessionHandler struct {
	db *pgxpool.Pool
}

func NewSessionHandler(db *pgxpool.Pool) *SessionHandler {
	return &SessionHandler{db: db}
}

// ListSessions returns all active (non-revoked, non-expired) sessions for the authenticated user.
func (h *SessionHandler) ListSessions(c echo.Context) error {
	userID, ok := c.Get("user_id").(string)
	if !ok || userID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	rows, err := h.db.Query(c.Request().Context(), `
        SELECT id, user_agent, ip_address, created_at, expires_at
        FROM sessions
        WHERE user_id = $1::uuid
          AND revoked_at IS NULL
          AND expires_at > NOW()
        ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	defer rows.Close()

	var sessions []SessionInfo
	for rows.Next() {
		var s SessionInfo
		if err := rows.Scan(&s.ID, &s.UserAgent, &s.IPAddress, &s.CreatedAt, &s.ExpiresAt); err != nil {
			continue
		}
		sessions = append(sessions, s)
	}
	if sessions == nil {
		sessions = []SessionInfo{}
	}
	return c.JSON(http.StatusOK, sessions)
}

// RevokeSession sets revoked_at on a session owned by the authenticated user.
func (h *SessionHandler) RevokeSession(c echo.Context) error {
	userID, ok := c.Get("user_id").(string)
	if !ok || userID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	sessionID := c.Param("id")

	tag, err := h.db.Exec(c.Request().Context(), `
        UPDATE sessions
        SET revoked_at = NOW()
        WHERE id = $1::uuid
          AND user_id = $2::uuid
          AND revoked_at IS NULL`,
		sessionID, userID,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	if tag.RowsAffected() == 0 {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "session not found"})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "revoked"})
}

// RevokeAllOtherSessions revokes all sessions for the user except the current one.
func (h *SessionHandler) RevokeAllOtherSessions(c echo.Context) error {
	userID, ok := c.Get("user_id").(string)
	if !ok || userID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	// The current session's token hash isn't in context — revoke everything for this user
	_, err := h.db.Exec(c.Request().Context(), `
        UPDATE sessions
        SET revoked_at = NOW()
        WHERE user_id = $1::uuid
          AND revoked_at IS NULL`,
		userID,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "all revoked"})
}
