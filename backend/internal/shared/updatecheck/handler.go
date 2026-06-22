package updatecheck

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/matharnica/vakt/internal/auth"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Register mounts update check routes on the given group (protected).
func Register(g *echo.Group, svc *Service) {
	h := NewHandler(svc)
	g.GET("/system/update", h.Get)
	g.PUT("/system/update", h.Put, auth.RequireRole("Admin"))
}

func (h *Handler) Get(c echo.Context) error {
	info := h.svc.GetUpdateInfo(c.Request().Context())
	return c.JSON(http.StatusOK, info)
}

func (h *Handler) Put(c echo.Context) error {
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.ErrBadRequest
	}
	if err := h.svc.SetEnabled(c.Request().Context(), body.Enabled); err != nil {
		return err
	}
	info := h.svc.GetUpdateInfo(c.Request().Context())
	return c.JSON(http.StatusOK, info)
}
