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

// securityLoginFailMax must stay in sync with auth.ipEmailLockoutFailMax.
// S121-F4: the pure per-email lockout (threshold 5) was removed as an account-DoS
// vector; the primary lockout is now the NAT-safe (IP, email) counter at 10.
const securityLoginFailMax int64 = 10

// lockoutKeyPrefix is the Redis prefix of the primary (IP, email) lockout counter
// written by auth.recordIPEmailLoginFailure: "login_fail_ip_email:<ip>:<email>".
const lockoutKeyPrefix = "login_fail_ip_email:"

// emailFromLockoutKey extracts the email from a lockout key. The IP segment may
// itself contain colons (IPv6), but an email address cannot — so the address is
// everything after the LAST colon.
func emailFromLockoutKey(key string) string {
	i := strings.LastIndex(key, ":")
	if i < 0 || i+1 >= len(key) {
		return ""
	}
	return key[i+1:]
}

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

// listLockedAccounts scans Redis for (IP, email) lockout counters that have
// reached the threshold and still have a remaining TTL.
//
// S121-F4: this used to scan the pure per-email counter (login_fail:*), which no
// longer exists. It now reads the primary NAT-safe counter. Because the same
// account can be locked from several source IPs at once, entries are deduplicated
// per email, keeping the one that unlocks last.
func (h *SecurityHandler) listLockedAccounts(ctx context.Context) ([]LockedAccount, error) {
	scanCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var cursor uint64
	byEmail := make(map[string]LockedAccount)

	for {
		keys, nextCursor, err := h.rdb.Scan(scanCtx, cursor, lockoutKeyPrefix+"*", 100).Result()
		if err != nil {
			return nil, fmt.Errorf("scan redis lockout keys: %w", err)
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

			email := emailFromLockoutKey(key)
			if email == "" {
				continue
			}
			now := time.Now().UTC()
			// Approximate locked_at: lockoutTTL minus remaining TTL from now.
			entry := LockedAccount{
				Email:       email,
				LockedAt:    now.Add(-(lockoutTTL - ttl)),
				LockedUntil: now.Add(ttl),
			}
			// Same account locked from multiple IPs — surface the latest unlock time.
			if prev, ok := byEmail[email]; !ok || entry.LockedUntil.After(prev.LockedUntil) {
				byEmail[email] = entry
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	locked := make([]LockedAccount, 0, len(byEmail))
	for _, e := range byEmail {
		locked = append(locked, e)
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
//
// S121-F4: the lockout is keyed on (IP, email), so one account can be locked from
// several source IPs at once. Deleting a single key would leave the account locked
// from the other IPs, so we scan for every counter belonging to this address and
// drop them all.
func (h *SecurityHandler) UnlockAccount(c echo.Context) error {
	ctx := c.Request().Context()
	email := c.Param("email")
	if email == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "email parameter is required",
			"code":  "ADMIN_BAD_REQUEST",
		})
	}

	scanCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var cursor uint64
	var toDelete []string
	for {
		keys, next, err := h.rdb.Scan(scanCtx, cursor, lockoutKeyPrefix+"*", 100).Result()
		if err != nil {
			log.Error().Err(err).Str("email_redacted", logsafe.RedactEmail(email)).Msg("admin: unlock account scan failed")
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "failed to unlock account",
				"code":  "ADMIN_UNLOCK_ERROR",
			})
		}
		for _, k := range keys {
			if emailFromLockoutKey(k) == email {
				toDelete = append(toDelete, k)
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}

	if len(toDelete) > 0 {
		if err := h.rdb.Del(scanCtx, toDelete...).Err(); err != nil && err != redis.Nil {
			log.Error().Err(err).Str("email_redacted", logsafe.RedactEmail(email)).Msg("admin: unlock account failed")
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "failed to unlock account",
				"code":  "ADMIN_UNLOCK_ERROR",
			})
		}
	}

	log.Info().Str("email_redacted", logsafe.RedactEmail(email)).Msg("admin: account lockout cleared")
	return c.JSON(http.StatusOK, map[string]string{
		"message": "account unlocked",
	})
}
