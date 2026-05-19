package secvitals

import (
	"github.com/labstack/echo/v4"
	"github.com/matharnica/vakt/internal/auth"
)

// registerExceptionRoutes registers all control exception (waiver) routes on the given group.
func registerExceptionRoutes(g *echo.Group, h *Handler) {
	ar := auth.RequireRole("Admin", "SecurityAnalyst")
	admin := auth.RequireRole("Admin")
	g.GET("/exceptions", h.ListControlExceptions, ar)
	g.POST("/controls/:controlId/exceptions", h.CreateControlException, ar)
	g.PUT("/exceptions/:id", h.UpdateControlException, ar)
	g.DELETE("/exceptions/:id", h.DeleteControlException, admin)
}
