// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S89-6: Word/DOCX export for auditors who require editable documents. Mirrors
// the XLSX/PDF gating: Pro-gated (FeatureAuditPDF) + SHA-256 audit-log entry.

package vaktcomply

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/shared/audit"
	"github.com/matharnica/vakt/internal/shared/docxexport"
	"github.com/matharnica/vakt/internal/shared/platform/features"
)

// logDocxExport records a SHA-256 audit-log entry for a generated .docx,
// consistent with the PDF/XLSX export audit pattern.
func (h *Handler) logDocxExport(c echo.Context, resourceType, resourceName string, data []byte) {
	sum := sha256.Sum256(data)
	audit.Write(c.Request().Context(), h.db, audit.WriteEntry{
		OrgID: orgID(c), UserID: userID(c), Action: "export",
		ResourceType: resourceType, ResourceName: resourceName,
		IPAddress: c.RealIP(),
		Details:   map[string]string{"format": "docx", "sha256": hex.EncodeToString(sum[:]), "bytes": fmt.Sprintf("%d", len(data))},
	})
}

// ExportRisksDOCX handles GET /api/v1/vaktcomply/risks/export/docx (Pro).
func (h *Handler) ExportRisksDOCX(c echo.Context) error {
	if !features.IsEnabled(c, features.FeatureAuditPDF) {
		return errResp(c, http.StatusPaymentRequired, "DOCX export requires Pro", "CK_FEATURE_REQUIRED")
	}
	ctx := c.Request().Context()
	org := orgID(c)

	risks, _, err := h.service.ListRisksPaged(ctx, org, 0, 10_000)
	if err != nil {
		log.Error().Err(err).Str("org_id", org).Msg("export risks docx")
		return errResp(c, http.StatusInternalServerError, "export failed", "CK_EXPORT_ERROR")
	}

	rows := make([]docxexport.RiskRow, len(risks))
	for i, r := range risks {
		rows[i] = docxexport.RiskRow{
			ID: r.ID, Title: r.Title, Category: r.Category,
			Likelihood: r.Likelihood, Impact: r.Impact, RiskScore: r.RiskScore,
			Treatment: r.Treatment, Status: r.Status, Owner: r.Owner,
			DueDate: r.TreatmentDueDate, ResidualScore: r.ResidualScore,
		}
	}

	data, err := docxexport.RenderRisiken(rows)
	if err != nil {
		log.Error().Err(err).Msg("export risks docx: render")
		return errResp(c, http.StatusInternalServerError, "export failed", "CK_EXPORT_ERROR")
	}
	h.logDocxExport(c, "vakt-comply/risk-register", "Risikoregister", data)

	filename := fmt.Sprintf("risikoregister-%s.docx", time.Now().UTC().Format("2006-01-02"))
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))
	return c.Blob(http.StatusOK, docxexport.ContentType, data)
}

// ExportSoADOCX handles GET /api/v1/vaktcomply/soa/export.docx (Pro).
func (h *Handler) ExportSoADOCX(c echo.Context) error {
	if !features.IsEnabled(c, features.FeatureAuditPDF) {
		return errResp(c, http.StatusPaymentRequired, "DOCX export requires Pro", "CK_FEATURE_REQUIRED")
	}
	ctx := c.Request().Context()
	org := orgID(c)

	entries, err := h.service.ListDedicatedSoAEntries(ctx, org)
	if err != nil {
		if errors.Is(err, ErrSoANotInitialized) {
			return errResp(c, http.StatusNotFound, err.Error(), "CK_SOA_NOT_INITIALIZED")
		}
		log.Error().Err(err).Msg("export soa docx: list entries")
		return errResp(c, http.StatusInternalServerError, "export failed", "CK_SOA_EXPORT_FAILED")
	}
	summary, err := h.service.GetDedicatedSoASummary(ctx, org)
	if err != nil {
		log.Error().Err(err).Msg("export soa docx: get summary")
		return errResp(c, http.StatusInternalServerError, "export failed", "CK_SOA_EXPORT_FAILED")
	}

	rows := make([]docxexport.SoARow, len(entries))
	for i, e := range entries {
		justification := e.Justification
		if !e.Applicable && e.ExclusionReason != "" {
			justification = e.ExclusionReason
		}
		owner := ""
		if e.ApprovedBy != nil {
			owner = *e.ApprovedBy
		}
		rows[i] = docxexport.SoARow{
			ControlRef: e.ControlRef, ControlName: e.ControlName, ControlGroup: e.ControlGroup,
			Applicable: e.Applicable, Justification: justification,
			ImplementationStatus: e.ImplementationStatus, Owner: owner, UpdatedAt: e.UpdatedAt,
		}
	}
	var sum docxexport.SoASummary
	if summary != nil {
		sum = docxexport.SoASummary{
			ApplicableCount:   summary.ApplicableCount,
			ExcludedCount:     summary.ExcludedCount,
			ImplementedCount:  summary.ImplementedCount,
			ImplementationPct: summary.ImplementationPct,
		}
	}

	data, err := docxexport.RenderSoA(rows, sum)
	if err != nil {
		log.Error().Err(err).Msg("export soa docx: render")
		return errResp(c, http.StatusInternalServerError, "export failed", "CK_SOA_EXPORT_FAILED")
	}
	h.logDocxExport(c, "vakt-comply/soa", "Statement of Applicability", data)

	filename := fmt.Sprintf("soa-%s.docx", time.Now().UTC().Format("2006-01-02"))
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))
	return c.Blob(http.StatusOK, docxexport.ContentType, data)
}
