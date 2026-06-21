// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	audit "github.com/matharnica/vakt/internal/modules/vaktcomply/audit"
)

// Audit-program domain types (AuditPlan, AuditProgramAudit, AuditFinding,
// AuditProgramSummary, CreateAuditPlanInput, CreateAuditProgramAuditInput,
// CompleteAuditInput, CreateAuditFindingInput) now live in the audit sub-package.

// ── Audit Plan handlers ──────────────────────────────────────────────────────

// ListAuditPlans handles GET /api/v1/vaktcomply/audit-plans
func (h *Handler) ListAuditPlans(c echo.Context) error {
	plans, err := h.service.Audit.ListAuditPlans(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list audit plans")
		return errResp(c, http.StatusInternalServerError, "failed to list audit plans", "CK_AUDIT_PLANS_FAILED")
	}
	return c.JSON(http.StatusOK, plans)
}

// CreateAuditPlan handles POST /api/v1/vaktcomply/audit-plans
func (h *Handler) CreateAuditPlan(c echo.Context) error {
	var in audit.CreateAuditPlanInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	plan, err := h.service.Audit.CreateAuditPlan(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create audit plan")
		return errResp(c, http.StatusInternalServerError, "failed to create audit plan", "CK_AUDIT_PLAN_CREATE_FAILED")
	}
	return c.JSON(http.StatusCreated, plan)
}

// UpdateAuditPlan handles PUT /api/v1/vaktcomply/audit-plans/:id
func (h *Handler) UpdateAuditPlan(c echo.Context) error {
	var in audit.CreateAuditPlanInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	plan, err := h.service.Audit.UpdateAuditPlan(c.Request().Context(), orgID(c), c.Param("id"), in)
	if err != nil {
		log.Error().Err(err).Msg("update audit plan")
		return errResp(c, http.StatusInternalServerError, "failed to update audit plan", "CK_AUDIT_PLAN_UPDATE_FAILED")
	}
	return c.JSON(http.StatusOK, plan)
}

// ── Audit handlers ───────────────────────────────────────────────────────────

// ListAuditProgramAudits handles GET /api/v1/vaktcomply/audit-program
func (h *Handler) ListAuditProgramAudits(c echo.Context) error {
	audits, err := h.service.Audit.ListAuditProgramAudits(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list audit program audits")
		return errResp(c, http.StatusInternalServerError, "failed to list audits", "CK_AUDIT_PROGRAM_FAILED")
	}
	return c.JSON(http.StatusOK, audits)
}

// CreateAuditProgramAudit handles POST /api/v1/vaktcomply/audit-program
func (h *Handler) CreateAuditProgramAudit(c echo.Context) error {
	var in audit.CreateAuditProgramAuditInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	auditRec, err := h.service.Audit.CreateAuditProgramAudit(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create audit program audit")
		return errResp(c, http.StatusInternalServerError, "failed to create audit", "CK_AUDIT_CREATE_FAILED")
	}
	return c.JSON(http.StatusCreated, auditRec)
}

// GetAuditProgramAudit handles GET /api/v1/vaktcomply/audit-program/:id
func (h *Handler) GetAuditProgramAudit(c echo.Context) error {
	auditRec, err := h.service.Audit.GetAuditProgramAudit(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		return errResp(c, http.StatusNotFound, "audit not found", "CK_AUDIT_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, auditRec)
}

// UpdateAuditProgramAudit handles PUT /api/v1/vaktcomply/audit-program/:id
func (h *Handler) UpdateAuditProgramAudit(c echo.Context) error {
	var in audit.CreateAuditProgramAuditInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	auditRec, err := h.service.Audit.UpdateAuditProgramAudit(c.Request().Context(), orgID(c), c.Param("id"), in)
	if err != nil {
		log.Error().Err(err).Msg("update audit program audit")
		return errResp(c, http.StatusInternalServerError, "failed to update audit", "CK_AUDIT_UPDATE_FAILED")
	}
	return c.JSON(http.StatusOK, auditRec)
}

// CompleteAuditProgramAudit handles PATCH /api/v1/vaktcomply/audit-program/:id/complete
func (h *Handler) CompleteAuditProgramAudit(c echo.Context) error {
	var in audit.CompleteAuditInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "audit_report is required", "code": "VALIDATION_ERROR"})
	}
	if err := h.service.Audit.CompleteAudit(c.Request().Context(), orgID(c), c.Param("id"), in); err != nil {
		log.Error().Err(err).Msg("complete audit")
		return errResp(c, http.StatusInternalServerError, "failed to complete audit", "CK_AUDIT_COMPLETE_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// ── Audit Findings handlers ──────────────────────────────────────────────────

// ListAuditFindings handles GET /api/v1/vaktcomply/audit-program/:id/findings
func (h *Handler) ListAuditFindings(c echo.Context) error {
	findings, err := h.service.Audit.ListAuditFindings(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		log.Error().Err(err).Msg("list audit findings")
		return errResp(c, http.StatusInternalServerError, "failed to list findings", "CK_AUDIT_FINDINGS_FAILED")
	}
	return c.JSON(http.StatusOK, findings)
}

// CreateAuditFinding handles POST /api/v1/vaktcomply/audit-program/:id/findings
func (h *Handler) CreateAuditFinding(c echo.Context) error {
	var in audit.CreateAuditFindingInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	finding, err := h.service.Audit.CreateAuditFinding(c.Request().Context(), orgID(c), c.Param("id"), in)
	if err != nil {
		log.Error().Err(err).Msg("create audit finding")
		return errResp(c, http.StatusInternalServerError, "failed to create finding", "CK_AUDIT_FINDING_CREATE_FAILED")
	}
	return c.JSON(http.StatusCreated, finding)
}

// GetAuditProgramSummary handles GET /api/v1/vaktcomply/audit-program/summary
func (h *Handler) GetAuditProgramSummary(c echo.Context) error {
	summary, err := h.service.Audit.GetAuditProgramSummary(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("get audit program summary")
		return errResp(c, http.StatusInternalServerError, "failed to get audit program summary", "CK_AUDIT_SUMMARY_FAILED")
	}
	return c.JSON(http.StatusOK, summary)
}

// ExportAuditProgramReport handles GET /api/v1/vaktcomply/audit-program/:id/export
func (h *Handler) ExportAuditProgramReport(c echo.Context) error {
	data, err := h.service.Audit.ExportAuditReport(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		log.Error().Err(err).Msg("export audit report")
		return errResp(c, http.StatusInternalServerError, "export failed", "CK_AUDIT_EXPORT_FAILED")
	}
	filename := fmt.Sprintf("vakt-audit-bericht-%s.pdf", time.Now().UTC().Format("2006-01-02"))
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))
	return c.Blob(http.StatusOK, "application/pdf", data)
}
