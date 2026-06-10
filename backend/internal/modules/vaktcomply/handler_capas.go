package vaktcomply

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/shared/audit"
	"github.com/matharnica/vakt/internal/shared/pagination"
)

// BulkUpdateCAPAs handles PATCH /api/v1/vaktcomply/capas/bulk.
// Updates status for multiple CAPAs in a single request.
func (h *Handler) BulkUpdateCAPAs(c echo.Context) error {
	var in BulkUpdateCAPAsInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	if err := h.service.BulkUpdateCAPAStatus(c.Request().Context(), orgID(c), in.IDs, in.Status); err != nil {
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
		if errors.Is(err, ErrNotFound) {
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
		if errors.Is(err, ErrNotFound) {
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
		if errors.Is(err, ErrNotFound) {
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
	if err := h.service.UpdateCAPANCFields(c.Request().Context(), orgID(c), c.Param("id"), in); err != nil {
		if errors.Is(err, ErrNotFound) {
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
	if err := h.service.CompleteEffectivenessCheck(c.Request().Context(), orgID(c), c.Param("id"), userID(c), in); err != nil {
		if errors.Is(err, ErrNotFound) {
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
