// Package audit provides an immutable audit log writer and Echo middleware.
// Audit log entries are append-only; no UPDATE or DELETE queries are ever
// issued against the audit_log table.
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// Entry describes a single audit event.
type Entry struct {
	OrgID        string `json:"org_id"`
	UserID       string `json:"user_id,omitempty"`
	Action       string `json:"action"`
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id,omitempty"`
	IPAddress    string `json:"ip_address,omitempty"`
	UserAgent    string `json:"user_agent,omitempty"`
	StatusCode   int    `json:"status_code"`
}

// Logger writes audit entries to PostgreSQL.
type Logger struct {
	db *pgxpool.Pool
}

// NewLogger constructs a Logger backed by the given connection pool.
func NewLogger(db *pgxpool.Pool) *Logger {
	return &Logger{db: db}
}

// Log appends a single audit entry.  The insert is intentionally INSERT-only.
// Writes to the unified audit_log table (migration 064 / 082).
func (l *Logger) Log(ctx context.Context, e Entry) error {
	var userID *string
	if e.UserID != "" {
		userID = &e.UserID
	}
	var resourceID *string
	if e.ResourceID != "" {
		resourceID = &e.ResourceID
	}
	var ip *string
	if e.IPAddress != "" {
		ip = &e.IPAddress
	}

	_, err := l.db.Exec(ctx, `
		INSERT INTO audit_log
			(org_id, user_id, action, resource_type, resource_id, ip_address)
		VALUES
			($1::uuid, $2::uuid, $3, $4, $5, $6)`,
		e.OrgID, userID, e.Action, e.ResourceType, resourceID, ip,
	)
	if err != nil {
		return fmt.Errorf("audit log insert: %w", err)
	}
	return nil
}

// sensitiveKeys lists key substrings that should be redacted from request bodies.
var sensitiveKeys = []string{"password", "secret", "token", "key"}

// redactBody removes sensitive fields from a raw JSON body and re-encodes it.
// If the body is not valid JSON, a placeholder is returned.
func redactBody(raw []byte) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		// Not a JSON object — return sanitised placeholder
		return json.RawMessage(`"[non-json body]"`)
	}

	for k := range obj {
		lower := strings.ToLower(k)
		for _, sensitive := range sensitiveKeys {
			if strings.Contains(lower, sensitive) {
				obj[k] = json.RawMessage(`"[REDACTED]"`)
				break
			}
		}
	}

	out, err := json.Marshal(obj)
	if err != nil {
		return json.RawMessage(`"[marshal error]"`)
	}
	return out
}

// AuditMiddleware returns Echo middleware that persists an audit entry for every
// mutating HTTP request (POST, PUT, PATCH, DELETE).
//
// It reads user_id and org_id from echo.Context values set by AuthMiddleware.
// Sensitive fields (password, secret, token, key) are redacted from the body.
func AuditMiddleware(logger *Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Run the actual handler first so we can capture the status code.
			err := next(c)

			method := c.Request().Method
			if method != "POST" && method != "PUT" && method != "PATCH" && method != "DELETE" {
				return err
			}

			orgID, _ := c.Get("org_id").(string)
			userID, _ := c.Get("user_id").(string)

			if orgID == "" {
				// Anonymous mutating request — skip audit (no org context).
				return err
			}

			statusCode := c.Response().Status

			entry := Entry{
				OrgID:        orgID,
				UserID:       userID,
				Action:       method + " " + c.Path(),
				ResourceType: resourceTypeFromPath(c.Path()),
				IPAddress:    c.RealIP(),
				UserAgent:    c.Request().UserAgent(),
				StatusCode:   statusCode,
			}

			// Fire-and-forget: audit failures must not affect the response.
			go func() {
				defer func() {
					if r := recover(); r != nil {
						log.Error().Interface("panic", r).Msg("goroutine panic recovered")
					}
				}()
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if logErr := logger.Log(ctx, entry); logErr != nil {
					log.Error().Err(logErr).Msg("audit log write failed")
				}
			}()

			return err
		}
	}
}

// resourceTypeFromPath derives a short resource type label from an Echo route path.
// e.g. "/api/v1/secpulse/assets/:id" → "secpulse/assets"
func resourceTypeFromPath(path string) string {
	// Strip leading /api/v1/ prefix if present
	path = strings.TrimPrefix(path, "/api/v1/")
	// Drop trailing parameter segments (":id", ":param", ...)
	parts := strings.Split(path, "/")
	var cleaned []string
	for _, p := range parts {
		if p != "" && !strings.HasPrefix(p, ":") {
			cleaned = append(cleaned, p)
		}
	}
	return strings.Join(cleaned, "/")
}
