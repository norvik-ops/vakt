package alerting

import (
	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/matharnica/vakt/internal/auth"
)

// Register mounts all alerting routes under the provided Echo group.
// The caller passes the `protected` group (/api/v1 with auth/CSRF/MFA/OrgRL).
//
// S121-B5 (R6): alerting channels define an outbound egress target (webhook/SMTP)
// — a data-exfiltration vector. Previously mounted on the bare `api` group with
// only inline auth: no CSRF and no role gate, so a Viewer could add an egress
// channel and send test deliveries. Mounting on `protected` restores CSRF; every
// mutating channel route and the test-delivery trigger are gated to Admin. Reads
// (list channels, delivery history) stay open to authenticated users.
func Register(g *echo.Group, db *pgxpool.Pool, masterKey []byte, smtpCfg SMTPConfig) {
	svc := NewService(db, masterKey, smtpCfg)
	h := &Handler{svc: svc, validate: validator.New()}
	admin := auth.RequireRole("Admin")

	channels := g.Group("/alerting/channels")
	channels.GET("", h.ListChannels)
	channels.POST("", h.CreateChannel, admin)
	channels.DELETE("/:id", h.DeleteChannel, admin)
	channels.PUT("/:id/toggle", h.ToggleChannel, admin)
	channels.POST("/:id/test", h.TestChannel, admin)
	// CRITICAL: /deliveries must be registered BEFORE any bare /:id routes to avoid route conflicts.
	channels.GET("/:id/deliveries", h.ListChannelDeliveries)

	g.GET("/alerting/history", h.ListDeliveryLog)
}
