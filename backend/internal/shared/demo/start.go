package demo

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/auth"
	"github.com/matharnica/vakt/internal/shared/demoseed"
)

// StartHandler handles the demo-session endpoints.
type StartHandler struct {
	db           *pgxpool.Pool
	masterKeyHex string
	authSvc      *auth.Service
}

// NewStartHandler constructs a StartHandler. authSvc may be nil — only the
// Start endpoint is usable in that case; Login requires the auth service to
// issue Paseto tokens.
func NewStartHandler(db *pgxpool.Pool, masterKeyHex string, authSvc *auth.Service) *StartHandler {
	return &StartHandler{db: db, masterKeyHex: masterKeyHex, authSvc: authSvc}
}

// Start creates an ephemeral demo org and returns the pre-fill credentials for the login form.
//
// Antwort enthält BEIDE Random-Passwörter (admin + analyst), damit das
// Frontend die Login-Form korrekt vorbefüllen kann. Die Passwörter sind
// 16-stellig (hex) — sie verlassen den Server nur dieses eine Mal als
// Klartext, da der Bcrypt-Hash zu spät kommt um ihn zur Anmeldung zu nutzen.
//
// Deprecated for new UIs: prefer POST /api/v1/demo/login which issues a real
// session server-side and never returns the password to the client.
// Kept here for backward compat with older clients embedded in marketing /
// docs pages that still scrape the demo creds.
func (h *StartHandler) Start(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 60*time.Second)
	defer cancel()

	sess, err := runEphemeralWithRetry(ctx, h.db, h.masterKeyHex)
	if err != nil {
		log.Error().Err(err).Msg("demo: RunEphemeral failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "demo session creation failed",
			"code":  "DEMO_SEED_ERROR",
		})
	}

	var adminEmail, analystEmail string
	_ = h.db.QueryRow(ctx, `SELECT email FROM users WHERE id=$1::uuid`, sess.AdminID).
		Scan(&adminEmail)
	_ = h.db.QueryRow(ctx, `
		SELECT u.email FROM users u
		JOIN org_members om ON om.user_id = u.id
		WHERE om.org_id = $1::uuid AND u.id <> $2::uuid
		ORDER BY u.created_at LIMIT 1`, sess.OrgID, sess.AdminID).
		Scan(&analystEmail)

	if adminEmail == "" {
		log.Error().Msg("demo: admin email lookup returned empty — seed may have failed silently")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "demo user lookup failed",
			"code":  "DEMO_USER_LOOKUP_FAILED",
		})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"admin_email":      adminEmail,
		"admin_password":   sess.AdminPassword,
		"analyst_email":    analystEmail,
		"analyst_password": sess.AnalystPassword,
		"expires_in":       4 * 60 * 60, // 4h in seconds; cleanup job purges thereafter
	})
}

// Login creates an ephemeral demo session AND issues a server-side auth token
// for the chosen role. Unlike Start, the random password is consumed
// internally and never returned to the client (audit F041).
//
// Body: {"role": "admin"|"analyst"}. Unknown / empty role → admin.
func (h *StartHandler) Login(c echo.Context) error {
	if h.authSvc == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "demo login not configured",
			"code":  "DEMO_LOGIN_DISABLED",
		})
	}
	var body struct {
		Role string `json:"role"`
	}
	_ = c.Bind(&body)

	ctx, cancel := context.WithTimeout(c.Request().Context(), 60*time.Second)
	defer cancel()

	sess, err := runEphemeralWithRetry(ctx, h.db, h.masterKeyHex)
	if err != nil {
		log.Error().Err(err).Msg("demo: RunEphemeral failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "demo session creation failed",
			"code":  "DEMO_SEED_ERROR",
		})
	}

	var targetEmail, targetPassword string
	switch body.Role {
	case "analyst":
		_ = h.db.QueryRow(ctx, `
			SELECT u.email FROM users u
			JOIN org_members om ON om.user_id = u.id
			WHERE om.org_id = $1::uuid AND u.id <> $2::uuid
			ORDER BY u.created_at LIMIT 1`, sess.OrgID, sess.AdminID).
			Scan(&targetEmail)
		targetPassword = sess.AnalystPassword
	default:
		_ = h.db.QueryRow(ctx, `SELECT email FROM users WHERE id=$1::uuid`, sess.AdminID).
			Scan(&targetEmail)
		targetPassword = sess.AdminPassword
	}
	if targetEmail == "" || targetPassword == "" {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "demo user lookup failed",
			"code":  "DEMO_USER_LOOKUP_FAILED",
		})
	}

	deviceHint := c.Request().Header.Get("User-Agent")
	if len(deviceHint) > 120 {
		deviceHint = deviceHint[:120]
	}
	resp, err := h.authSvc.Login(ctx, targetEmail, targetPassword, deviceHint)
	if err != nil {
		log.Error().Err(err).Msg("demo: internal Login failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "demo login failed",
			"code":  "DEMO_LOGIN_FAILED",
		})
	}

	secure := auth.CookieSecure(c)
	c.SetCookie(&http.Cookie{ // nosemgrep: cookie-missing-secure -- Secure is set via variable; static analysis can't resolve it
		Name:     "access_token",
		Value:    resp.AccessToken,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		Path:     "/api/v1",
		MaxAge:   3600,
	})
	auth.SetCSRFCookie(c, auth.GenerateCSRFToken())

	return c.JSON(http.StatusOK, resp)
}

// isTransientSeedError returns true for DB errors that are safe to retry:
// connection pool exhaustion, server-side timeouts, and deadlocks.
// Unique-constraint violations and other logical errors are NOT transient.
func isTransientSeedError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	if strings.Contains(msg, "connection") || strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "deadlock") || strings.Contains(msg, "too many clients") {
		return true
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		// 40001 = serialization failure, 40P01 = deadlock, 53300 = too_many_connections,
		// 08* = connection errors, 57014 = query_canceled
		return pgErr.Code == "40001" || pgErr.Code == "40P01" ||
			pgErr.Code == "53300" || pgErr.Code == "57014" ||
			strings.HasPrefix(pgErr.Code, "08")
	}
	return false
}

// runEphemeralWithRetry calls demoseed.RunEphemeral, retrying once on transient
// DB errors (pool exhaustion, deadlock, timeout). The 60-second context from
// the HTTP handler wraps both attempts combined.
func runEphemeralWithRetry(ctx context.Context, db *pgxpool.Pool, masterKeyHex string) (*demoseed.EphemeralSession, error) {
	sess, err := demoseed.RunEphemeral(ctx, db, masterKeyHex)
	if err == nil {
		return sess, nil
	}
	if !isTransientSeedError(err) {
		return nil, err
	}
	log.Warn().Err(err).Msg("demo: RunEphemeral transient error, retrying in 1s")
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(time.Second):
	}
	return demoseed.RunEphemeral(ctx, db, masterKeyHex)
}

// RegisterStart registers the demo/start endpoint.
func RegisterStart(g *echo.Group, h *StartHandler) {
	g.POST("/start", h.Start)
	g.POST("/login", h.Login)
}
