package alerting

import (
	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

// Register mounts all alerting routes under the provided Echo group.
// The caller is responsible for passing a group rooted at /api/v1.
func Register(g *echo.Group, db *pgxpool.Pool, masterKey []byte, smtpCfg SMTPConfig, auth echo.MiddlewareFunc) {
	svc := NewService(db, masterKey, smtpCfg)
	h := &Handler{svc: svc, validate: validator.New()}

	channels := g.Group("/alerting/channels", auth)
	channels.GET("", h.ListChannels)
	channels.POST("", h.CreateChannel)
	channels.DELETE("/:id", h.DeleteChannel)
	channels.PUT("/:id/toggle", h.ToggleChannel)
	channels.POST("/:id/test", h.TestChannel)
	// CRITICAL: /deliveries must be registered BEFORE any bare /:id routes to avoid route conflicts.
	channels.GET("/:id/deliveries", h.ListChannelDeliveries)

	g.GET("/alerting/history", h.ListDeliveryLog, auth)
}
