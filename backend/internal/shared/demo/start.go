package demo

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/sechealth-app/sechealth/internal/shared/demoseed"
)

// StartHandler handles POST /api/v1/demo/start.
type StartHandler struct {
	db           *pgxpool.Pool
	masterKeyHex string
}

// NewStartHandler constructs a StartHandler.
func NewStartHandler(db *pgxpool.Pool, masterKeyHex string) *StartHandler {
	return &StartHandler{db: db, masterKeyHex: masterKeyHex}
}

// Start creates an ephemeral demo org and returns the pre-fill credentials for the login form.
func (h *StartHandler) Start(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 60*time.Second)
	defer cancel()

	sess, err := demoseed.RunEphemeral(ctx, h.db, h.masterKeyHex)
	if err != nil {
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

	return c.JSON(http.StatusOK, map[string]interface{}{
		"admin_email":   adminEmail,
		"analyst_email": analystEmail,
	})
}

// RegisterStart registers the demo/start endpoint.
func RegisterStart(g *echo.Group, h *StartHandler) {
	g.POST("/start", h.Start)
}
