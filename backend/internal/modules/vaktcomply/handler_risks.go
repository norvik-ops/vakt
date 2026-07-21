package vaktcomply

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/matharnica/vakt/internal/shared/audit"
	"github.com/matharnica/vakt/internal/shared/pagination"
	"github.com/rs/zerolog/log"
)

func (h *Handler) GetRisk(c echo.Context) error {
	id := c.Param("id")
	risk, err := h.service.Risk.GetRisk(c.Request().Context(), orgID(c), id)
	if err != nil {
		return errResp(c, http.StatusNotFound, "risk not found", "CK_RISK_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, risk)
}

// UpdateRisk handles PATCH /api/v1/vaktcomply/risks/:id.
func (h *Handler) UpdateRisk(c echo.Context) error {
	id := c.Param("id")
	var in UpdateRiskInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	risk, err := h.service.Risk.UpdateRisk(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		log.Error().Err(err).Msg("update risk")
		return errResp(c, http.StatusInternalServerError, "failed to update risk", "CK_UPDATE_RISK_FAILED")
	}
	return c.JSON(http.StatusOK, risk)
}

// UpdateRiskTreatment handles PATCH /api/v1/vaktcomply/risks/:id/treatment.
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
	risk, err := h.service.Risk.UpdateRiskTreatment(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "risk not found", "CK_NOT_FOUND")
		}
		log.Error().Err(err).Msg("update risk treatment")
		return errResp(c, http.StatusInternalServerError, "failed to update risk treatment", "CK_UPDATE_RISK_TREATMENT_FAILED")
	}
	return c.JSON(http.StatusOK, risk)
}

// ListRisks handles GET /api/v1/vaktcomply/risks.
// Cursor mode (preferred): ?cursor=<opaque>&limit=25
// Offset mode (deprecated): ?page=1&limit=25 — sends Deprecation header
func (h *Handler) ListRisks(c echo.Context) error {
	if c.QueryParam("page") == "" {
		cp := pagination.CursorFromRequest(c)
		cursorID, cursorTS := pagination.DecodeCursor(cp.Cursor)
		rows, err := h.service.Risk.ListRisksCursor(c.Request().Context(), orgID(c), cursorID, cursorTS, cp.Limit)
		if err != nil {
			log.Error().Err(err).Msg("list risks cursor")
			return errResp(c, http.StatusInternalServerError, "failed to list risks", "CK_LIST_RISKS_FAILED")
		}
		resp := pagination.WrapCursor(rows, cp, func(r Risk) string {
			return pagination.EncodeCursor(r.ID, r.CreatedAt)
		})
		return c.JSON(http.StatusOK, resp)
	}
	c.Response().Header().Set("Deprecation", "true")
	c.Response().Header().Set("Sunset", "2027-01-01")
	offset, limit, meta := pagination.FromRequest(c)
	risks, total, err := h.service.Risk.ListRisksPaged(c.Request().Context(), orgID(c), offset, limit)
	if err != nil {
		log.Error().Err(err).Msg("list risks")
		return errResp(c, http.StatusInternalServerError, "failed to list risks", "CK_LIST_RISKS_FAILED")
	}
	pagination.Complete(&meta, total)
	return c.JSON(http.StatusOK, pagination.Wrap(risks, meta))
}

// CreateRisk handles POST /api/v1/vaktcomply/risks.
func (h *Handler) CreateRisk(c echo.Context) error {
	var in CreateRiskInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	risk, err := h.service.Risk.CreateRisk(c.Request().Context(), orgID(c), in)
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

// DeleteRisk handles DELETE /api/v1/vaktcomply/risks/:id.
func (h *Handler) DeleteRisk(c echo.Context) error {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid risk id", "CK_BAD_REQUEST")
	}
	if err := h.service.Risk.DeleteRisk(c.Request().Context(), orgID(c), id); err != nil {
		// S121-D4 (P3): not-found → 404, not 500
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "risk not found", "CK_RISK_NOT_FOUND")
		}
		log.Error().Err(err).Str("id", id).Msg("delete risk")
		return errResp(c, http.StatusInternalServerError, "failed to delete risk", "CK_DELETE_RISK_FAILED")
	}
	audit.Write(c.Request().Context(), h.db, audit.WriteEntry{
		OrgID:        orgID(c),
		UserID:       userID(c),
		Action:       "delete",
		ResourceType: "vakt-comply/risk",
		ResourceID:   id,
		IPAddress:    c.RealIP(),
	})
	return c.NoContent(http.StatusNoContent)
}

// ListRiskControls handles GET /api/v1/vaktcomply/risks/:id/controls.
func (h *Handler) ListRiskControls(c echo.Context) error {
	id := c.Param("id")
	controls, err := h.service.ListRiskControls(c.Request().Context(), orgID(c), id)
	if err != nil {
		log.Error().Err(err).Msg("list risk controls")
		return errResp(c, http.StatusInternalServerError, "failed to list risk controls", "CK_LIST_RISK_CONTROLS_FAILED")
	}
	return c.JSON(http.StatusOK, controls)
}

// LinkRiskControl handles POST /api/v1/vaktcomply/risks/:id/controls.
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

// UnlinkRiskControl handles DELETE /api/v1/vaktcomply/risks/:id/controls/:controlId.
func (h *Handler) UnlinkRiskControl(c echo.Context) error {
	riskID := c.Param("id")
	controlID := c.Param("controlId")
	if err := h.service.UnlinkRiskControl(c.Request().Context(), orgID(c), riskID, controlID); err != nil {
		log.Error().Err(err).Msg("unlink risk control")
		return errResp(c, http.StatusNotFound, "link not found", "CK_RISK_CONTROL_NOT_FOUND")
	}
	return c.NoContent(http.StatusNoContent)
}

// UpdateRiskResidualFields handles PATCH /api/v1/vaktcomply/risks/:id/residual (S61-4).
func (h *Handler) UpdateRiskResidualFields(c echo.Context) error {
	id := c.Param("id")
	var in UpdateRiskResidualInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	if err := h.service.Risk.UpdateRiskResidualFields(c.Request().Context(), orgID(c), id, in); err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "risk not found", "CK_NOT_FOUND")
		}
		log.Error().Err(err).Msg("update risk residual fields")
		return errResp(c, http.StatusInternalServerError, "failed to update residual fields", "CK_UPDATE_RESIDUAL_FAILED")
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "updated"})
}

// AcceptRisk handles POST /api/v1/vaktcomply/risks/:id/accept (S61-4).
func (h *Handler) AcceptRisk(c echo.Context) error {
	id := c.Param("id")
	var in AcceptRiskInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	if err := h.service.Risk.AcceptRisk(c.Request().Context(), orgID(c), id, userID(c), in); err != nil {
		if err.Error() == "risk must have treatment_status=accepted before formal acceptance" {
			return errResp(c, http.StatusConflict, err.Error(), "CK_RISK_NOT_ACCEPTED_TREATMENT")
		}
		log.Error().Err(err).Msg("accept risk")
		return errResp(c, http.StatusInternalServerError, "failed to accept risk", "CK_ACCEPT_RISK_FAILED")
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "accepted"})
}

func (h *Handler) ListControlExceptions(c echo.Context) error {
	controlID := c.QueryParam("control_id")
	if controlID == "" {
		exceptions, err := h.service.Risk.ListAllControlExceptions(c.Request().Context(), orgID(c))
		if err != nil {
			log.Error().Err(err).Msg("list all control exceptions")
			return errResp(c, http.StatusInternalServerError, "failed to list exceptions", "CK_LIST_EXCEPTIONS_FAILED")
		}
		return c.JSON(http.StatusOK, exceptions)
	}
	if _, err := uuid.Parse(controlID); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid control_id", "CK_INVALID_ID")
	}
	exceptions, err := h.service.Risk.ListControlExceptions(c.Request().Context(), orgID(c), controlID)
	if err != nil {
		log.Error().Err(err).Str("control_id", controlID).Msg("list control exceptions")
		return errResp(c, http.StatusInternalServerError, "failed to list exceptions", "CK_LIST_EXCEPTIONS_FAILED")
	}
	return c.JSON(http.StatusOK, exceptions)
}

// CreateControlException handles POST /api/v1/vaktcomply/controls/:controlId/exceptions
func (h *Handler) CreateControlException(c echo.Context) error {
	controlID := c.Param("controlId")
	if _, err := uuid.Parse(controlID); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid control ID", "CK_INVALID_ID")
	}
	var in CreateControlExceptionInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	exception, err := h.service.Risk.CreateControlException(c.Request().Context(), orgID(c), controlID, in, userID(c))
	if err != nil {
		log.Error().Err(err).Str("control_id", controlID).Msg("create control exception")
		return errResp(c, http.StatusInternalServerError, "failed to create exception", "CK_CREATE_EXCEPTION_FAILED")
	}
	return c.JSON(http.StatusCreated, exception)
}

// UpdateControlException handles PUT /api/v1/vaktcomply/exceptions/:id
func (h *Handler) UpdateControlException(c echo.Context) error {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid exception ID", "CK_INVALID_ID")
	}
	var in UpdateControlExceptionInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	exception, err := h.service.Risk.UpdateControlException(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "exception not found", "CK_NOT_FOUND")
		}
		log.Error().Err(err).Str("id", id).Msg("update control exception")
		return errResp(c, http.StatusInternalServerError, "failed to update exception", "CK_UPDATE_EXCEPTION_FAILED")
	}
	return c.JSON(http.StatusOK, exception)
}

// DeleteControlException handles DELETE /api/v1/vaktcomply/exceptions/:id
func (h *Handler) DeleteControlException(c echo.Context) error {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid exception ID", "CK_INVALID_ID")
	}
	if err := h.service.Risk.DeleteControlException(c.Request().Context(), orgID(c), id); err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "exception not found", "CK_NOT_FOUND")
		}
		log.Error().Err(err).Str("id", id).Msg("delete control exception")
		return errResp(c, http.StatusInternalServerError, "failed to delete exception", "CK_DELETE_EXCEPTION_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) BulkUpdateCAPAs(c echo.Context) error {
	var in BulkUpdateCAPAsInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	if err := h.service.Risk.BulkUpdateCAPAStatus(c.Request().Context(), orgID(c), in.IDs, in.Status); err != nil {
		log.Error().Err(err).Msg("bulk update capas")
		return errResp(c, http.StatusInternalServerError, "failed to bulk update capas", "CK_BULK_UPDATE_FAILED")
	}
	audit.Write(c.Request().Context(), h.db, audit.WriteEntry{
		OrgID:        orgID(c),
		UserID:       userID(c),
		Action:       "bulk_update",
		ResourceType: "vakt-comply/capa",
		ResourceName: "bulk status update",
		IPAddress:    c.RealIP(),
	})
	return c.NoContent(http.StatusNoContent)
}

// ListCAPAs handles GET /vaktcomply/capas?status=open.
func (h *Handler) ListCAPAs(c echo.Context) error {
	statusFilter := c.QueryParam("status")
	offset, limit, meta := pagination.FromRequest(c)
	capas, total, err := h.service.ListCAPAsPaged(c.Request().Context(), orgID(c), statusFilter, offset, limit)
	if err != nil {
		log.Error().Err(err).Msg("list capas")
		return errResp(c, http.StatusInternalServerError, "failed to list capas", "CK_LIST_CAPAS_FAILED")
	}
	pagination.Complete(&meta, total)
	return c.JSON(http.StatusOK, pagination.Wrap(capas, meta))
}

// CreateCAPA handles POST /vaktcomply/capas.
func (h *Handler) CreateCAPA(c echo.Context) error {
	var in CreateCAPAInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	capa, err := h.service.CreateCAPA(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create capa")
		return errResp(c, http.StatusInternalServerError, "failed to create capa", "CK_CREATE_CAPA_FAILED")
	}
	audit.Write(c.Request().Context(), h.db, audit.WriteEntry{
		OrgID: orgID(c), UserID: userID(c), Action: "create",
		ResourceType: "vakt-comply/capa", ResourceID: capa.ID, ResourceName: capa.Title,
		IPAddress: c.RealIP(),
	})
	return c.JSON(http.StatusCreated, capa)
}

// GetCAPA handles GET /vaktcomply/capas/:id.
func (h *Handler) GetCAPA(c echo.Context) error {
	capa, err := h.service.GetCAPA(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "capa not found", "CK_CAPA_NOT_FOUND")
		}
		log.Error().Err(err).Msg("get capa")
		return errResp(c, http.StatusInternalServerError, "failed to get capa", "CK_GET_CAPA_FAILED")
	}
	return c.JSON(http.StatusOK, capa)
}

// UpdateCAPA handles PATCH /vaktcomply/capas/:id.
func (h *Handler) UpdateCAPA(c echo.Context) error {
	var in UpdateCAPAInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	capa, err := h.service.UpdateCAPA(c.Request().Context(), orgID(c), c.Param("id"), in)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "capa not found", "CK_CAPA_NOT_FOUND")
		}
		log.Error().Err(err).Msg("update capa")
		return errResp(c, http.StatusInternalServerError, "failed to update capa", "CK_UPDATE_CAPA_FAILED")
	}
	audit.Write(c.Request().Context(), h.db, audit.WriteEntry{
		OrgID: orgID(c), UserID: userID(c), Action: "update",
		ResourceType: "vakt-comply/capa", ResourceID: capa.ID, ResourceName: capa.Title,
		IPAddress: c.RealIP(),
	})
	return c.JSON(http.StatusOK, capa)
}

// DeleteCAPA handles DELETE /vaktcomply/capas/:id.
func (h *Handler) DeleteCAPA(c echo.Context) error {
	id := c.Param("id")
	if err := h.service.DeleteCAPA(c.Request().Context(), orgID(c), id); err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "capa not found", "CK_CAPA_NOT_FOUND")
		}
		log.Error().Err(err).Msg("delete capa")
		return errResp(c, http.StatusInternalServerError, "failed to delete capa", "CK_DELETE_CAPA_FAILED")
	}
	audit.Write(c.Request().Context(), h.db, audit.WriteEntry{
		OrgID: orgID(c), UserID: userID(c), Action: "delete",
		ResourceType: "vakt-comply/capa", ResourceID: id,
		IPAddress: c.RealIP(),
	})
	return c.NoContent(http.StatusNoContent)
}

// ListCAPAsForAudit handles GET /vaktcomply/audits/:id/capas.
func (h *Handler) ListCAPAsForAudit(c echo.Context) error {
	capas, err := h.service.ListCAPAsForSource(c.Request().Context(), orgID(c), "audit", c.Param("id"))
	if err != nil {
		log.Error().Err(err).Msg("list capas for audit")
		return errResp(c, http.StatusInternalServerError, "failed to list capas", "CK_LIST_CAPAS_FAILED")
	}
	if capas == nil {
		capas = []CAPA{}
	}
	return c.JSON(http.StatusOK, capas)
}

// CreateCAPAFromAudit handles POST /vaktcomply/audits/:id/capas.
func (h *Handler) CreateCAPAFromAudit(c echo.Context) error {
	var in CreateCAPAInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	in.SourceType = "audit"
	in.SourceID = c.Param("id")
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	capa, err := h.service.CreateCAPA(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create capa from audit")
		return errResp(c, http.StatusInternalServerError, "failed to create capa", "CK_CREATE_CAPA_FAILED")
	}
	return c.JSON(http.StatusCreated, capa)
}

// ListCAPAsForIncident handles GET /vaktcomply/incidents/:id/capas.
func (h *Handler) ListCAPAsForIncident(c echo.Context) error {
	capas, err := h.service.ListCAPAsForSource(c.Request().Context(), orgID(c), "incident", c.Param("id"))
	if err != nil {
		log.Error().Err(err).Msg("list capas for incident")
		return errResp(c, http.StatusInternalServerError, "failed to list capas", "CK_LIST_CAPAS_FAILED")
	}
	if capas == nil {
		capas = []CAPA{}
	}
	return c.JSON(http.StatusOK, capas)
}

// CreateCAPAFromIncident handles POST /vaktcomply/incidents/:id/capas.
func (h *Handler) CreateCAPAFromIncident(c echo.Context) error {
	var in CreateCAPAInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	in.SourceType = "incident"
	in.SourceID = c.Param("id")
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	capa, err := h.service.CreateCAPA(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create capa from incident")
		return errResp(c, http.StatusInternalServerError, "failed to create capa", "CK_CREATE_CAPA_FAILED")
	}
	return c.JSON(http.StatusCreated, capa)
}

// UpdateCAPANCFields handles PATCH /api/v1/vaktcomply/capas/:id/nc-fields.
// Updates the NC root-cause and effectiveness planning fields of a CAPA.
func (h *Handler) UpdateCAPANCFields(c echo.Context) error {
	var in CAPANCFields
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.service.Risk.UpdateCAPANCFields(c.Request().Context(), orgID(c), c.Param("id"), in); err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "capa not found", "CK_CAPA_NOT_FOUND")
		}
		log.Error().Err(err).Msg("update capa nc fields")
		return errResp(c, http.StatusInternalServerError, "failed to update capa nc fields", "CK_UPDATE_NC_FIELDS_FAILED")
	}
	audit.Write(c.Request().Context(), h.db, audit.WriteEntry{
		OrgID:        orgID(c),
		UserID:       userID(c),
		Action:       "update",
		ResourceType: "vakt-comply/capa",
		ResourceID:   c.Param("id"),
		ResourceName: "nc-fields",
		IPAddress:    c.RealIP(),
	})
	return c.NoContent(http.StatusNoContent)
}

// CompleteEffectivenessCheck handles POST /api/v1/vaktcomply/capas/:id/effectiveness-check.
// Records the result of a CAPA effectiveness review.
func (h *Handler) CompleteEffectivenessCheck(c echo.Context) error {
	var in EffectivenessCheckInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.service.Risk.CompleteEffectivenessCheck(c.Request().Context(), orgID(c), c.Param("id"), userID(c), in); err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "capa not found", "CK_CAPA_NOT_FOUND")
		}
		log.Error().Err(err).Msg("complete effectiveness check")
		return errResp(c, http.StatusInternalServerError, "failed to complete effectiveness check", "CK_EFFECTIVENESS_CHECK_FAILED")
	}
	audit.Write(c.Request().Context(), h.db, audit.WriteEntry{
		OrgID:        orgID(c),
		UserID:       userID(c),
		Action:       "update",
		ResourceType: "vakt-comply/capa",
		ResourceID:   c.Param("id"),
		ResourceName: "effectiveness-check",
		IPAddress:    c.RealIP(),
	})
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) ListProtectionNeedAssessments(c echo.Context) error {
	items, err := h.service.Risk.ListProtectionNeedAssessments(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list protection need assessments")
		return errResp(c, http.StatusInternalServerError, "failed to list assessments", "CK_LIST_PNA_FAILED")
	}
	if items == nil {
		items = []ProtectionNeedAssessment{}
	}
	return c.JSON(http.StatusOK, items)
}

// CreateProtectionNeedAssessment handles POST /api/v1/vaktcomply/protection-needs/assessments.
func (h *Handler) CreateProtectionNeedAssessment(c echo.Context) error {
	var in CreateProtectionNeedInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	pna, err := h.service.Risk.CreateProtectionNeedAssessment(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create protection need assessment")
		return errResp(c, http.StatusInternalServerError, "failed to create assessment", "CK_CREATE_PNA_FAILED")
	}
	return c.JSON(http.StatusCreated, pna)
}

// GetProtectionNeedAssessment handles GET /api/v1/vaktcomply/protection-needs/assessments/:id.
func (h *Handler) GetProtectionNeedAssessment(c echo.Context) error {
	id := c.Param("id")
	pna, err := h.service.Risk.GetProtectionNeedAssessment(c.Request().Context(), orgID(c), id)
	if err != nil {
		return errResp(c, http.StatusNotFound, "assessment not found", "CK_PNA_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, pna)
}

// UpdateProtectionNeedAssessment handles PATCH /api/v1/vaktcomply/protection-needs/assessments/:id.
// Sets C/I/A ratings and recomputes the overall protection need level.
func (h *Handler) UpdateProtectionNeedAssessment(c echo.Context) error {
	id := c.Param("id")
	var in UpdateProtectionNeedInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	pna, err := h.service.Risk.UpdateProtectionNeedAssessment(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "assessment not found", "CK_PNA_NOT_FOUND")
		}
		log.Error().Err(err).Str("id", id).Msg("update protection need assessment")
		return errResp(c, http.StatusInternalServerError, "failed to update assessment", "CK_UPDATE_PNA_FAILED")
	}
	return c.JSON(http.StatusOK, pna)
}

// FinalizeProtectionNeedAssessment handles POST /api/v1/vaktcomply/protection-needs/assessments/:id/finalize.
func (h *Handler) FinalizeProtectionNeedAssessment(c echo.Context) error {
	id := c.Param("id")
	pna, err := h.service.Risk.FinalizeProtectionNeedAssessment(c.Request().Context(), orgID(c), id)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "assessment not found", "CK_PNA_NOT_FOUND")
		}
		log.Error().Err(err).Str("id", id).Msg("finalize protection need assessment")
		return errResp(c, http.StatusInternalServerError, "failed to finalize assessment", "CK_FINALIZE_PNA_FAILED")
	}
	return c.JSON(http.StatusOK, pna)
}

// DeleteProtectionNeedAssessment handles DELETE /api/v1/vaktcomply/protection-needs/assessments/:id.
func (h *Handler) DeleteProtectionNeedAssessment(c echo.Context) error {
	id := c.Param("id")
	if err := h.service.Risk.DeleteProtectionNeedAssessment(c.Request().Context(), orgID(c), id); err != nil {
		log.Error().Err(err).Str("id", id).Msg("delete protection need assessment")
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "assessment not found", "CK_PNA_NOT_FOUND")
		}
		return errResp(c, http.StatusInternalServerError, "failed to delete assessment", "CK_DELETE_PNA_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// LinkPNAAsset handles PATCH /api/v1/vaktcomply/protection-needs/assessments/:id/asset-link.
// Body: {"vb_asset_id": "uuid"} to link, {"vb_asset_id": null} to unlink.
func (h *Handler) LinkPNAAsset(c echo.Context) error {
	id := c.Param("id")
	var body struct {
		VBAssetID *string `json:"vb_asset_id"`
	}
	if err := c.Bind(&body); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.service.Risk.LinkAssetToPNA(c.Request().Context(), orgID(c), id, body.VBAssetID); err != nil {
		if isNotFound(err) { // S131-A1: 0 rows = assessment does not exist → 404, not silent 200
			return errResp(c, http.StatusNotFound, "protection need assessment not found", "CK_PNA_NOT_FOUND")
		}
		log.Error().Err(err).Str("id", id).Msg("link asset to pna")
		return errResp(c, http.StatusInternalServerError, "failed to link asset", "CK_LINK_ASSET_FAILED")
	}
	return c.JSON(http.StatusOK, map[string]any{"pna_id": id, "vb_asset_id": body.VBAssetID})
}
