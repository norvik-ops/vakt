package alerting

import (
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// Handler handles HTTP requests for the alerting endpoints.
type Handler struct {
	svc      *Service
	validate *validator.Validate
}

func orgIDFromCtx(c echo.Context) (string, bool) {
	orgID, ok := c.Get("org_id").(string)
	return orgID, ok && orgID != ""
}

func alertErrResp(c echo.Context, code int, msg, errCode string) error {
	return c.JSON(code, map[string]string{"error": msg, "code": errCode})
}

// ListChannels handles GET /alerting/channels.
func (h *Handler) ListChannels(c echo.Context) error {
	orgID, ok := orgIDFromCtx(c)
	if !ok {
		return alertErrResp(c, http.StatusUnauthorized, "unauthorized", "ALERTING_UNAUTHORIZED")
	}
	channels, err := h.svc.ListChannels(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Msg("list alert channels")
		return alertErrResp(c, http.StatusInternalServerError, "failed to list channels", "ALERTING_LIST_FAILED")
	}
	if channels == nil {
		channels = []Channel{}
	}
	return c.JSON(http.StatusOK, channels)
}

// CreateChannel handles POST /alerting/channels.
func (h *Handler) CreateChannel(c echo.Context) error {
	orgID, ok := orgIDFromCtx(c)
	if !ok {
		return alertErrResp(c, http.StatusUnauthorized, "unauthorized", "ALERTING_UNAUTHORIZED")
	}
	var in CreateChannelInput
	if err := c.Bind(&in); err != nil {
		return alertErrResp(c, http.StatusBadRequest, "invalid request body", "ALERTING_INVALID_BODY")
	}
	if err := h.validate.Struct(in); err != nil {
		return alertErrResp(c, http.StatusUnprocessableEntity, err.Error(), "ALERTING_VALIDATION_ERROR")
	}
	ch, hmacSecret, err := h.svc.CreateChannel(c.Request().Context(), orgID, in)
	if err != nil {
		log.Error().Err(err).Msg("create alert channel")
		return alertErrResp(c, http.StatusInternalServerError, "failed to create channel", "ALERTING_CREATE_FAILED")
	}
	return c.JSON(http.StatusCreated, CreateChannelResponse{Channel: *ch, HmacSecret: hmacSecret})
}

// DeleteChannel handles DELETE /alerting/channels/:id.
func (h *Handler) DeleteChannel(c echo.Context) error {
	orgID, ok := orgIDFromCtx(c)
	if !ok {
		return alertErrResp(c, http.StatusUnauthorized, "unauthorized", "ALERTING_UNAUTHORIZED")
	}
	if err := h.svc.DeleteChannel(c.Request().Context(), orgID, c.Param("id")); err != nil {
		log.Error().Err(err).Msg("delete alert channel")
		return alertErrResp(c, http.StatusNotFound, "channel not found", "ALERTING_NOT_FOUND")
	}
	return c.NoContent(http.StatusNoContent)
}

// ToggleChannel handles PUT /alerting/channels/:id/toggle.
func (h *Handler) ToggleChannel(c echo.Context) error {
	orgID, ok := orgIDFromCtx(c)
	if !ok {
		return alertErrResp(c, http.StatusUnauthorized, "unauthorized", "ALERTING_UNAUTHORIZED")
	}
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.Bind(&body); err != nil {
		return alertErrResp(c, http.StatusBadRequest, "invalid request body", "ALERTING_INVALID_BODY")
	}
	if err := h.svc.ToggleChannel(c.Request().Context(), orgID, c.Param("id"), body.Enabled); err != nil {
		log.Error().Err(err).Msg("toggle alert channel")
		return alertErrResp(c, http.StatusNotFound, "channel not found", "ALERTING_NOT_FOUND")
	}
	return c.NoContent(http.StatusNoContent)
}

// TestChannel handles POST /alerting/channels/:id/test.
func (h *Handler) TestChannel(c echo.Context) error {
	orgID, ok := orgIDFromCtx(c)
	if !ok {
		return alertErrResp(c, http.StatusUnauthorized, "unauthorized", "ALERTING_UNAUTHORIZED")
	}
	if err := h.svc.TestChannel(c.Request().Context(), orgID, c.Param("id")); err != nil {
		log.Error().Err(err).Msg("test alert channel")
		return alertErrResp(c, http.StatusBadGateway, "Testbenachrichtigung konnte nicht gesendet werden", "ALERTING_DELIVERY_FAILED")
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// ListDeliveryLog handles GET /alerting/history.
func (h *Handler) ListDeliveryLog(c echo.Context) error {
	orgID, ok := orgIDFromCtx(c)
	if !ok {
		return alertErrResp(c, http.StatusUnauthorized, "unauthorized", "ALERTING_UNAUTHORIZED")
	}
	entries, err := h.svc.ListDeliveryLog(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Msg("list delivery log")
		return alertErrResp(c, http.StatusInternalServerError, "failed to list delivery log", "ALERTING_LOG_FAILED")
	}
	if entries == nil {
		entries = []DeliveryLogEntry{}
	}
	return c.JSON(http.StatusOK, entries)
}

// ListChannelDeliveries handles GET /alerting/channels/:id/deliveries.
func (h *Handler) ListChannelDeliveries(c echo.Context) error {
	orgID, ok := orgIDFromCtx(c)
	if !ok {
		return alertErrResp(c, http.StatusUnauthorized, "unauthorized", "ALERTING_UNAUTHORIZED")
	}
	entries, err := h.svc.ListChannelDeliveries(c.Request().Context(), orgID, c.Param("id"))
	if err != nil {
		log.Error().Err(err).Msg("list channel deliveries")
		return alertErrResp(c, http.StatusInternalServerError, "failed to list channel deliveries", "ALERTING_LOG_FAILED")
	}
	if entries == nil {
		entries = []DeliveryLogEntry{}
	}
	return c.JSON(http.StatusOK, entries)
}
