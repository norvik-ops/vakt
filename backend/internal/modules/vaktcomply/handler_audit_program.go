// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// AuditPlan is a yearly audit planning document for ISO 27001 Clause 9.2.
type AuditPlan struct {
	ID            string  `json:"id"`
	OrgID         string  `json:"org_id"`
	Year          int     `json:"year"`
	Scope         string  `json:"scope,omitempty"`
	ResponsibleID *string `json:"responsible_id,omitempty"`
	Status        string  `json:"status"`
	Notes         string  `json:"notes,omitempty"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
}

// AuditProgramAudit is an individual audit within an audit plan.
type AuditProgramAudit struct {
	ID            string   `json:"id"`
	OrgID         string   `json:"org_id"`
	AuditPlanID   *string  `json:"audit_plan_id,omitempty"`
	Title         string   `json:"title"`
	AuditType     string   `json:"audit_type"`
	Scope         string   `json:"scope"`
	Methodology   string   `json:"methodology"`
	PlannedDate   string   `json:"planned_date"`
	ActualDate    *string  `json:"actual_date,omitempty"`
	LeadAuditorID *string  `json:"lead_auditor_id,omitempty"`
	AuditorIDs    []string `json:"auditor_ids"`
	SupplierID    *string  `json:"supplier_id,omitempty"`
	Status        string   `json:"status"`
	AuditReport   string   `json:"audit_report,omitempty"`
	FindingsCount int      `json:"findings_count"`
	CreatedAt     string   `json:"created_at"`
	UpdatedAt     string   `json:"updated_at"`
}

// AuditFinding is a finding recorded during an audit.
type AuditFinding struct {
	ID                string  `json:"id"`
	OrgID             string  `json:"org_id"`
	AuditID           string  `json:"audit_id"`
	Title             string  `json:"title"`
	Description       string  `json:"description"`
	Severity          string  `json:"severity"`
	AffectedControlID *string `json:"affected_control_id,omitempty"`
	CAPAid            *string `json:"capa_id,omitempty"`
	CreatedAt         string  `json:"created_at"`
}

// AuditProgramSummary holds aggregate stats for the audit program dashboard.
type AuditProgramSummary struct {
	AuditsPlannedThisYear  int `json:"audits_planned_this_year"`
	AuditsCompleted        int `json:"audits_completed"`
	OpenFindings           int `json:"open_findings"`
	OverdueCAPAsFromAudits int `json:"overdue_capas_from_audits"`
}

// CreateAuditPlanInput holds validated input for a new audit plan.
type CreateAuditPlanInput struct {
	Year          int     `json:"year"  validate:"required,min=2000,max=2100"`
	Scope         string  `json:"scope,omitempty"`
	ResponsibleID *string `json:"responsible_id,omitempty"`
	Notes         string  `json:"notes,omitempty"`
}

// CreateAuditProgramAuditInput holds validated input for an individual audit.
type CreateAuditProgramAuditInput struct {
	AuditPlanID   *string  `json:"audit_plan_id,omitempty"`
	Title         string   `json:"title"       validate:"required,max=300"`
	AuditType     string   `json:"audit_type"  validate:"required,oneof=isms_internal compliance_check supplier_audit process_audit"`
	Scope         string   `json:"scope"       validate:"required,max=5000"`
	Methodology   string   `json:"methodology" validate:"omitempty,oneof=document_review interview technical_check combined"`
	PlannedDate   string   `json:"planned_date" validate:"required"`
	LeadAuditorID *string  `json:"lead_auditor_id,omitempty"`
	AuditorIDs    []string `json:"auditor_ids,omitempty"`
	SupplierID    *string  `json:"supplier_id,omitempty"`
}

// CompleteAuditInput holds the audit report and actual completion date.
type CompleteAuditInput struct {
	AuditReport string `json:"audit_report" validate:"required,min=10,max=50000"`
	ActualDate  string `json:"actual_date"  validate:"required"`
}

// CreateAuditFindingInput holds validated input for a finding.
type CreateAuditFindingInput struct {
	Title             string  `json:"title"       validate:"required,max=300"`
	Description       string  `json:"description" validate:"required,max=10000"`
	Severity          string  `json:"severity"    validate:"required,oneof=major_nc minor_nc observation ofi"`
	AffectedControlID *string `json:"affected_control_id,omitempty"`
}

// ── Audit Plan handlers ──────────────────────────────────────────────────────

// ListAuditPlans handles GET /api/v1/vaktcomply/audit-plans
func (h *Handler) ListAuditPlans(c echo.Context) error {
	plans, err := h.service.ListAuditPlans(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list audit plans")
		return errResp(c, http.StatusInternalServerError, "failed to list audit plans", "CK_AUDIT_PLANS_FAILED")
	}
	return c.JSON(http.StatusOK, plans)
}

// CreateAuditPlan handles POST /api/v1/vaktcomply/audit-plans
func (h *Handler) CreateAuditPlan(c echo.Context) error {
	var in CreateAuditPlanInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	plan, err := h.service.CreateAuditPlan(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create audit plan")
		return errResp(c, http.StatusInternalServerError, "failed to create audit plan", "CK_AUDIT_PLAN_CREATE_FAILED")
	}
	return c.JSON(http.StatusCreated, plan)
}

// UpdateAuditPlan handles PUT /api/v1/vaktcomply/audit-plans/:id
func (h *Handler) UpdateAuditPlan(c echo.Context) error {
	var in CreateAuditPlanInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	plan, err := h.service.UpdateAuditPlan(c.Request().Context(), orgID(c), c.Param("id"), in)
	if err != nil {
		log.Error().Err(err).Msg("update audit plan")
		return errResp(c, http.StatusInternalServerError, "failed to update audit plan", "CK_AUDIT_PLAN_UPDATE_FAILED")
	}
	return c.JSON(http.StatusOK, plan)
}

// ── Audit handlers ───────────────────────────────────────────────────────────

// ListAuditProgramAudits handles GET /api/v1/vaktcomply/audit-program
func (h *Handler) ListAuditProgramAudits(c echo.Context) error {
	audits, err := h.service.ListAuditProgramAudits(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list audit program audits")
		return errResp(c, http.StatusInternalServerError, "failed to list audits", "CK_AUDIT_PROGRAM_FAILED")
	}
	return c.JSON(http.StatusOK, audits)
}

// CreateAuditProgramAudit handles POST /api/v1/vaktcomply/audit-program
func (h *Handler) CreateAuditProgramAudit(c echo.Context) error {
	var in CreateAuditProgramAuditInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	audit, err := h.service.CreateAuditProgramAudit(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create audit program audit")
		return errResp(c, http.StatusInternalServerError, "failed to create audit", "CK_AUDIT_CREATE_FAILED")
	}
	return c.JSON(http.StatusCreated, audit)
}

// GetAuditProgramAudit handles GET /api/v1/vaktcomply/audit-program/:id
func (h *Handler) GetAuditProgramAudit(c echo.Context) error {
	audit, err := h.service.GetAuditProgramAudit(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		return errResp(c, http.StatusNotFound, "audit not found", "CK_AUDIT_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, audit)
}

// UpdateAuditProgramAudit handles PUT /api/v1/vaktcomply/audit-program/:id
func (h *Handler) UpdateAuditProgramAudit(c echo.Context) error {
	var in CreateAuditProgramAuditInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	audit, err := h.service.UpdateAuditProgramAudit(c.Request().Context(), orgID(c), c.Param("id"), in)
	if err != nil {
		log.Error().Err(err).Msg("update audit program audit")
		return errResp(c, http.StatusInternalServerError, "failed to update audit", "CK_AUDIT_UPDATE_FAILED")
	}
	return c.JSON(http.StatusOK, audit)
}

// CompleteAuditProgramAudit handles PATCH /api/v1/vaktcomply/audit-program/:id/complete
func (h *Handler) CompleteAuditProgramAudit(c echo.Context) error {
	var in CompleteAuditInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "audit_report is required", "code": "VALIDATION_ERROR"})
	}
	if err := h.service.CompleteAudit(c.Request().Context(), orgID(c), c.Param("id"), in); err != nil {
		log.Error().Err(err).Msg("complete audit")
		return errResp(c, http.StatusInternalServerError, "failed to complete audit", "CK_AUDIT_COMPLETE_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// ── Audit Findings handlers ──────────────────────────────────────────────────

// ListAuditFindings handles GET /api/v1/vaktcomply/audit-program/:id/findings
func (h *Handler) ListAuditFindings(c echo.Context) error {
	findings, err := h.service.ListAuditFindings(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		log.Error().Err(err).Msg("list audit findings")
		return errResp(c, http.StatusInternalServerError, "failed to list findings", "CK_AUDIT_FINDINGS_FAILED")
	}
	return c.JSON(http.StatusOK, findings)
}

// CreateAuditFinding handles POST /api/v1/vaktcomply/audit-program/:id/findings
func (h *Handler) CreateAuditFinding(c echo.Context) error {
	var in CreateAuditFindingInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	finding, err := h.service.CreateAuditFinding(c.Request().Context(), orgID(c), c.Param("id"), in)
	if err != nil {
		log.Error().Err(err).Msg("create audit finding")
		return errResp(c, http.StatusInternalServerError, "failed to create finding", "CK_AUDIT_FINDING_CREATE_FAILED")
	}
	return c.JSON(http.StatusCreated, finding)
}

// GetAuditProgramSummary handles GET /api/v1/vaktcomply/audit-program/summary
func (h *Handler) GetAuditProgramSummary(c echo.Context) error {
	summary, err := h.service.GetAuditProgramSummary(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("get audit program summary")
		return errResp(c, http.StatusInternalServerError, "failed to get audit program summary", "CK_AUDIT_SUMMARY_FAILED")
	}
	return c.JSON(http.StatusOK, summary)
}

// ExportAuditProgramReport handles GET /api/v1/vaktcomply/audit-program/:id/export
func (h *Handler) ExportAuditProgramReport(c echo.Context) error {
	data, err := h.service.ExportAuditReport(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		log.Error().Err(err).Msg("export audit report")
		return errResp(c, http.StatusInternalServerError, "export failed", "CK_AUDIT_EXPORT_FAILED")
	}
	filename := fmt.Sprintf("vakt-audit-bericht-%s.pdf", time.Now().UTC().Format("2006-01-02"))
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))
	return c.Blob(http.StatusOK, "application/pdf", data)
}
