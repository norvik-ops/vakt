package secvitals

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/shared/audit"
	"github.com/matharnica/vakt/internal/shared/pagination"
)

// GetRisk handles GET /api/v1/secvitals/risks/:id.
func (h *Handler) GetRisk(c echo.Context) error {
	id := c.Param("id")
	risk, err := h.service.GetRisk(c.Request().Context(), orgID(c), id)
	if err != nil {
		return errResp(c, http.StatusNotFound, "risk not found", "CK_RISK_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, risk)
}

// UpdateRisk handles PATCH /api/v1/secvitals/risks/:id.
func (h *Handler) UpdateRisk(c echo.Context) error {
	id := c.Param("id")
	var in UpdateRiskInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	risk, err := h.service.UpdateRisk(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		log.Error().Err(err).Msg("update risk")
		return errResp(c, http.StatusInternalServerError, "failed to update risk", "CK_UPDATE_RISK_FAILED")
	}
	return c.JSON(http.StatusOK, risk)
}

// UpdateRiskTreatment handles PATCH /api/v1/secvitals/risks/:id/treatment.
// Patches only the ISO 27001 Clause 6 treatment workflow fields.
func (h *Handler) UpdateRiskTreatment(c echo.Context) error {
	id := c.Param("id")
	var in UpdateRiskTreatmentInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	risk, err := h.service.UpdateRiskTreatment(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		log.Error().Err(err).Msg("update risk treatment")
		return errResp(c, http.StatusInternalServerError, "failed to update risk treatment", "CK_UPDATE_RISK_TREATMENT_FAILED")
	}
	return c.JSON(http.StatusOK, risk)
}

// ListRisks handles GET /api/v1/secvitals/risks.
func (h *Handler) ListRisks(c echo.Context) error {
	offset, limit, meta := pagination.FromRequest(c)
	risks, total, err := h.service.ListRisksPaged(c.Request().Context(), orgID(c), offset, limit)
	if err != nil {
		log.Error().Err(err).Msg("list risks")
		return errResp(c, http.StatusInternalServerError, "failed to list risks", "CK_LIST_RISKS_FAILED")
	}
	pagination.Complete(&meta, total)
	return c.JSON(http.StatusOK, pagination.Wrap(risks, meta))
}

// CreateRisk handles POST /api/v1/secvitals/risks.
func (h *Handler) CreateRisk(c echo.Context) error {
	var in CreateRiskInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	risk, err := h.service.CreateRisk(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create risk")
		return errResp(c, http.StatusInternalServerError, "failed to create risk", "CK_CREATE_RISK_FAILED")
	}
	audit.Write(c.Request().Context(), h.db, audit.WriteEntry{
		OrgID:        orgID(c),
		UserID:       userID(c),
		Action:       "create",
		ResourceType: "vakt-comply/risk",
		ResourceID:   risk.ID,
		ResourceName: risk.Title,
		IPAddress:    c.RealIP(),
	})
	return c.JSON(http.StatusCreated, risk)
}

// ListRiskControls handles GET /api/v1/secvitals/risks/:id/controls.
func (h *Handler) ListRiskControls(c echo.Context) error {
	id := c.Param("id")
	controls, err := h.service.ListRiskControls(c.Request().Context(), orgID(c), id)
	if err != nil {
		log.Error().Err(err).Msg("list risk controls")
		return errResp(c, http.StatusInternalServerError, "failed to list risk controls", "CK_LIST_RISK_CONTROLS_FAILED")
	}
	return c.JSON(http.StatusOK, controls)
}

// LinkRiskControl handles POST /api/v1/secvitals/risks/:id/controls.
// Body: {"control_id": "<uuid>"}
func (h *Handler) LinkRiskControl(c echo.Context) error {
	id := c.Param("id")
	var body struct {
		ControlID string `json:"control_id" validate:"required,uuid"`
	}
	if err := c.Bind(&body); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(body); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	if err := h.service.LinkRiskControl(c.Request().Context(), orgID(c), id, body.ControlID); err != nil {
		log.Error().Err(err).Msg("link risk control")
		return errResp(c, http.StatusInternalServerError, "failed to link control", "CK_LINK_RISK_CONTROL_FAILED")
	}
	return c.JSON(http.StatusCreated, map[string]string{"status": "linked"})
}

// UnlinkRiskControl handles DELETE /api/v1/secvitals/risks/:id/controls/:controlId.
func (h *Handler) UnlinkRiskControl(c echo.Context) error {
	riskID := c.Param("id")
	controlID := c.Param("controlId")
	if err := h.service.UnlinkRiskControl(c.Request().Context(), orgID(c), riskID, controlID); err != nil {
		log.Error().Err(err).Msg("unlink risk control")
		return errResp(c, http.StatusNotFound, "link not found", "CK_RISK_CONTROL_NOT_FOUND")
	}
	return c.NoContent(http.StatusNoContent)
}
