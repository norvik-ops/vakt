package admin

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/matharnica/vakt/internal/shared/logsafe"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

// SecurityHandler handles admin security-event endpoints.
type SecurityHandler struct {
	db  *pgxpool.Pool
	rdb *redis.Client
}

// NewSecurityHandler constructs a SecurityHandler.
func NewSecurityHandler(db *pgxpool.Pool, rdb *redis.Client) *SecurityHandler {
	return &SecurityHandler{db: db, rdb: rdb}
}

// LockedAccount is a currently locked-out account as derived from Redis.
type LockedAccount struct {
	Email       string    `json:"email"`
	LockedAt    time.Time `json:"locked_at"`
	LockedUntil time.Time `json:"locked_until"`
}

// RecentFailure is a recent failed login event aggregated from the audit log.
type RecentFailure struct {
	Email     string    `json:"email"`
	IPAddress string    `json:"ip,omitempty"`
	At        time.Time `json:"at"`
	Count     int       `json:"count"`
}

// SecurityEventsResponse is the payload for GET /api/v1/admin/security-events.
type SecurityEventsResponse struct {
	LockedAccounts  []LockedAccount `json:"locked_accounts"`
	RecentFailures  []RecentFailure `json:"recent_failures"`
	TotalLocked     int             `json:"total_locked"`
	FailuresLast24h int             `json:"failures_last_24h"`
}

// loginFailMax must stay in sync with auth/service.go.
const securityLoginFailMax int64 = 5

// lockoutTTL must stay in sync with auth/service.go.
const lockoutTTL = 15 * time.Minute

// GetSecurityEvents handles GET /api/v1/admin/security-events.
func (h *SecurityHandler) GetSecurityEvents(c echo.Context) error {
	ctx := c.Request().Context()
	orgID, _ := c.Get("org_id").(string)

	locked, err := h.listLockedAccounts(ctx)
	if err != nil {
		log.Error().Err(err).Msg("admin security-events: list locked accounts failed")
		locked = []LockedAccount{}
	}

	failures, err := h.listRecentFailures(ctx, orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("admin security-events: list recent failures failed")
		failures = []RecentFailure{}
	}

	failuresLast24h := 0
	for _, f := range failures {
		failuresLast24h += f.Count
	}

	return c.JSON(http.StatusOK, SecurityEventsResponse{
		LockedAccounts:  locked,
		RecentFailures:  failures,
		TotalLocked:     len(locked),
		FailuresLast24h: failuresLast24h,
	})
}

// listLockedAccounts scans Redis for all login_fail:* keys where the count has
// reached the lockout threshold and the key still has a remaining TTL.
func (h *SecurityHandler) listLockedAccounts(ctx context.Context) ([]LockedAccount, error) {
	scanCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var cursor uint64
	var locked []LockedAccount

	for {
		keys, nextCursor, err := h.rdb.Scan(scanCtx, cursor, "login_fail:*", 100).Result()
		if err != nil {
			return nil, fmt.Errorf("scan redis login_fail keys: %w", err)
		}

		for _, key := range keys {
			val, err := h.rdb.Get(scanCtx, key).Int64()
			if err != nil {
				continue // key may have expired between SCAN and GET
			}
			if val < securityLoginFailMax {
				continue
			}

			ttl, err := h.rdb.TTL(scanCtx, key).Result()
			if err != nil || ttl <= 0 {
				continue
			}

			email := strings.TrimPrefix(key, "login_fail:")
			now := time.Now().UTC()
			// Approximate locked_at: lockoutTTL minus remaining TTL from now.
			lockedAt := now.Add(-(lockoutTTL - ttl))
			lockedUntil := now.Add(ttl)

			locked = append(locked, LockedAccount{
				Email:       email,
				LockedAt:    lockedAt,
				LockedUntil: lockedUntil,
			})
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return locked, nil
}

// listRecentFailures queries the audit_log table for login_failed events in the
// last 24 hours, grouped by user email and IP address.
func (h *SecurityHandler) listRecentFailures(ctx context.Context, orgID string) ([]RecentFailure, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := h.db.Query(queryCtx, `
		SELECT user_email, ip_address, MAX(created_at) AS last_at, COUNT(*)::int AS cnt
		FROM audit_log
		WHERE action = 'login_failed'
		  AND created_at > NOW() - INTERVAL '24 hours'
		  AND ($1::text = '' OR org_id::text = $1)
		  AND deleted_at IS NULL
		GROUP BY user_email, ip_address
		ORDER BY last_at DESC
		LIMIT 100`, orgID)
	if err != nil {
		return nil, fmt.Errorf("query recent login failures: %w", err)
	}
	defer rows.Close()

	var failures []RecentFailure
	for rows.Next() {
		var (
			email  *string
			ip     *string
			lastAt time.Time
			count  int
		)
		if err := rows.Scan(&email, &ip, &lastAt, &count); err != nil {
			return nil, fmt.Errorf("scan login failure row: %w", err)
		}
		f := RecentFailure{At: lastAt, Count: count}
		if email != nil {
			f.Email = *email
		}
		if ip != nil {
			f.IPAddress = *ip
		}
		failures = append(failures, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate login failure rows: %w", err)
	}

	return failures, nil
}

// UnlockAccount handles DELETE /api/v1/admin/accounts/:email/unlock.
// It deletes the Redis login_fail:<email> key, immediately releasing the lockout.
func (h *SecurityHandler) UnlockAccount(c echo.Context) error {
	ctx := c.Request().Context()
	email := c.Param("email")
	if email == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "email parameter is required",
			"code":  "ADMIN_BAD_REQUEST",
		})
	}

	key := "login_fail:" + email
	if err := h.rdb.Del(ctx, key).Err(); err != nil && err != redis.Nil {
		log.Error().Err(err).Str("email_redacted", logsafe.RedactEmail(email)).Msg("admin: unlock account failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to unlock account",
			"code":  "ADMIN_UNLOCK_ERROR",
		})
	}

	log.Info().Str("email_redacted", logsafe.RedactEmail(email)).Msg("admin: account lockout cleared")
	return c.JSON(http.StatusOK, map[string]string{
		"message": "account unlocked",
	})
}
