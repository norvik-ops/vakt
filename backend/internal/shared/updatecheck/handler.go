package updatecheck

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Register mounts GET /system/update on the given group (should be the protected group).
func Register(g *echo.Group, svc *Service) {
	h := NewHandler(svc)
	g.GET("/system/update", h.Get)
}

func (h *Handler) Get(c echo.Context) error {
	info := h.svc.GetUpdateInfo(c.Request().Context())
	return c.JSON(http.StatusOK, info)
}
