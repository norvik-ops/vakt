package vaktcomply

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	auditmod "github.com/matharnica/vakt/internal/modules/vaktcomply/audit"
	"github.com/matharnica/vakt/internal/shared/audit"
)

// GetAuditRecord handles GET /api/v1/vaktcomply/audits/:id.
func (h *Handler) GetAuditRecord(c echo.Context) error {
	id := c.Param("id")
	record, err := h.service.Audit.GetAuditRecord(c.Request().Context(), orgID(c), id)
	if err != nil {
		return errResp(c, http.StatusNotFound, "audit record not found", "CK_AUDIT_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, record)
}

// UpdateAuditRecord handles PATCH /api/v1/vaktcomply/audits/:id.
func (h *Handler) UpdateAuditRecord(c echo.Context) error {
	id := c.Param("id")
	var in auditmod.UpdateAuditRecordInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	record, err := h.service.Audit.UpdateAuditRecord(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		log.Error().Err(err).Msg("update audit record")
		return errResp(c, http.StatusInternalServerError, "failed to update audit record", "CK_UPDATE_AUDIT_FAILED")
	}
	audit.Write(c.Request().Context(), h.db, audit.WriteEntry{
		OrgID: orgID(c), UserID: userID(c), Action: "update",
		ResourceType: "vakt-comply/audit", ResourceID: id, ResourceName: record.Title,
		IPAddress: c.RealIP(),
	})
	return c.JSON(http.StatusOK, record)
}

// ListAuditRecords handles GET /api/v1/vaktcomply/audits.
func (h *Handler) ListAuditRecords(c echo.Context) error {
	records, err := h.service.Audit.ListAuditRecords(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list audit records")
		return errResp(c, http.StatusInternalServerError, "failed to list audit records", "CK_LIST_AUDITS_FAILED")
	}
	return c.JSON(http.StatusOK, records)
}

// CreateAuditRecord handles POST /api/v1/vaktcomply/audits.
func (h *Handler) CreateAuditRecord(c echo.Context) error {
	var in auditmod.CreateAuditRecordInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	record, err := h.service.Audit.CreateAuditRecord(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create audit record")
		return errResp(c, http.StatusInternalServerError, "failed to create audit record", "CK_CREATE_AUDIT_FAILED")
	}
	audit.Write(c.Request().Context(), h.db, audit.WriteEntry{
		OrgID: orgID(c), UserID: userID(c), Action: "create",
		ResourceType: "vakt-comply/audit", ResourceID: record.ID, ResourceName: record.Title,
		IPAddress: c.RealIP(),
	})
	return c.JSON(http.StatusCreated, record)
}
