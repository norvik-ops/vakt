package vaktcomply

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// ListMeasures handles GET /api/v1/vaktcomply/controls/:id/measures.
func (h *Handler) ListMeasures(c echo.Context) error {
	controlID := c.Param("id")
	if controlID == "" {
		return errResp(c, http.StatusBadRequest, "control id is required", "CK_BAD_REQUEST")
	}
	measures, err := h.service.ListMeasures(c.Request().Context(), orgID(c), controlID)
	if err != nil {
		log.Error().Err(err).Str("control_id", controlID).Msg("list measures")
		return errResp(c, http.StatusInternalServerError, "failed to list measures", "CK_INTERNAL")
	}
	if measures == nil {
		measures = []ControlMeasure{}
	}
	return c.JSON(http.StatusOK, measures)
}

// CreateMeasure handles POST /api/v1/vaktcomply/controls/:id/measures.
func (h *Handler) CreateMeasure(c echo.Context) error {
	controlID := c.Param("id")
	if controlID == "" {
		return errResp(c, http.StatusBadRequest, "control id is required", "CK_BAD_REQUEST")
	}
	var in CreateMeasureInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusBadRequest, "Ungültige Eingabe", "VALIDATION_ERROR")
	}
	measure, err := h.service.CreateMeasure(c.Request().Context(), orgID(c), controlID, in)
	if err != nil {
		log.Error().Err(err).Str("control_id", controlID).Msg("create measure")
		return errResp(c, http.StatusInternalServerError, "failed to create measure", "CK_INTERNAL")
	}
	return c.JSON(http.StatusCreated, measure)
}

// UpdateMeasure handles PATCH /api/v1/vaktcomply/controls/:id/measures/:mid.
func (h *Handler) UpdateMeasure(c echo.Context) error {
	measureID := c.Param("mid")
	if measureID == "" {
		return errResp(c, http.StatusBadRequest, "measure id is required", "CK_BAD_REQUEST")
	}
	var in UpdateMeasureInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusBadRequest, "Ungültige Eingabe", "VALIDATION_ERROR")
	}
	measure, err := h.service.UpdateMeasure(c.Request().Context(), orgID(c), measureID, in)
	if err != nil {
		log.Error().Err(err).Str("measure_id", measureID).Msg("update measure")
		return errResp(c, http.StatusInternalServerError, "failed to update measure", "CK_INTERNAL")
	}
	return c.JSON(http.StatusOK, measure)
}

// DeleteMeasure handles DELETE /api/v1/vaktcomply/controls/:id/measures/:mid.
func (h *Handler) DeleteMeasure(c echo.Context) error {
	measureID := c.Param("mid")
	if measureID == "" {
		return errResp(c, http.StatusBadRequest, "measure id is required", "CK_BAD_REQUEST")
	}
	if err := h.service.DeleteMeasure(c.Request().Context(), orgID(c), measureID); err != nil {
		log.Error().Err(err).Str("measure_id", measureID).Msg("delete measure")
		return errResp(c, http.StatusInternalServerError, "failed to delete measure", "CK_INTERNAL")
	}
	return c.NoContent(http.StatusNoContent)
}
