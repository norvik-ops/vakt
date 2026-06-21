// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/shared/platform/features"
	"github.com/matharnica/vakt/internal/shared/xlsxexport"
)

// GetSoA handles GET /api/v1/vaktcomply/soa
func (h *Handler) GetSoA(c echo.Context) error {
	entries, err := h.service.GetSoAEntries(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("get soa")
		return errResp(c, http.StatusInternalServerError, "failed to get SoA", "CK_SOA_FAILED")
	}
	return c.JSON(http.StatusOK, entries)
}

// GetSoACSV handles GET /api/v1/vaktcomply/soa.csv
func (h *Handler) GetSoACSV(c echo.Context) error {
	entries, err := h.service.GetSoAEntries(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("get soa csv")
		return errResp(c, http.StatusInternalServerError, "failed to generate SoA CSV", "CK_SOA_FAILED")
	}

	filename := fmt.Sprintf("vakt-soa-%s.csv", time.Now().UTC().Format("2006-01-02"))
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))
	c.Response().Header().Set("Content-Type", "text/csv; charset=utf-8")

	w := csv.NewWriter(c.Response().Writer)
	_ = w.Write([]string{"Framework", "Domain", "Kontrolle", "Anwendbar", "Status", "Begründung (Anwendbar)", "Begründung (Nicht anwendbar)"})
	for _, e := range entries {
		applicable := "Nein"
		if e.Applicable {
			applicable = "Ja"
		}
		_ = w.Write([]string{
			e.FrameworkName, e.Domain, e.Title, applicable, e.Status,
			e.JustificationApplicable, e.JustificationNotApplicable,
		})
	}
	w.Flush()
	return nil
}

// UpdateSoAApplicability handles PATCH /api/v1/vaktcomply/soa/:control_id
func (h *Handler) UpdateSoAApplicability(c echo.Context) error {
	var in struct {
		Applicable                 bool   `json:"applicable"`
		JustificationApplicable    string `json:"justification_applicable"`
		JustificationNotApplicable string `json:"justification_not_applicable"`
	}
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.service.UpdateSoAApplicability(c.Request().Context(), orgID(c), c.Param("control_id"), in.Applicable, in.JustificationApplicable, in.JustificationNotApplicable); err != nil {
		log.Error().Err(err).Msg("update soa applicability")
		return errResp(c, http.StatusInternalServerError, "failed to update SoA", "CK_SOA_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// ── Dedicated SoA (S68-1) ────────────────────────────────────────────────────

// InitDedicatedSoA handles POST /api/v1/vaktcomply/soa/init
// Creates version 1 with all 93 ISO 27001:2022 Annex A controls for the org.
func (h *Handler) InitDedicatedSoA(c echo.Context) error {
	if err := h.service.InitDedicatedSoA(c.Request().Context(), orgID(c)); err != nil {
		log.Error().Err(err).Msg("init dedicated soa")
		return errResp(c, http.StatusInternalServerError, "failed to initialize SoA", "CK_SOA_INIT_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// GetDedicatedSoAEntries handles GET /api/v1/vaktcomply/soa/entries
func (h *Handler) GetDedicatedSoAEntries(c echo.Context) error {
	entries, err := h.service.ListDedicatedSoAEntries(c.Request().Context(), orgID(c))
	if err != nil {
		if errors.Is(err, ErrSoANotInitialized) {
			return errResp(c, http.StatusNotFound, err.Error(), "CK_SOA_NOT_INITIALIZED")
		}
		log.Error().Err(err).Msg("get dedicated soa entries")
		return errResp(c, http.StatusInternalServerError, "failed to get SoA entries", "CK_SOA_FAILED")
	}
	return c.JSON(http.StatusOK, entries)
}

// GetDedicatedSoAEntry handles GET /api/v1/vaktcomply/soa/entries/:control_ref
func (h *Handler) GetDedicatedSoAEntry(c echo.Context) error {
	entry, err := h.service.GetDedicatedSoAEntry(c.Request().Context(), orgID(c), c.Param("control_ref"))
	if err != nil {
		if errors.Is(err, ErrSoANotInitialized) {
			return errResp(c, http.StatusNotFound, err.Error(), "CK_SOA_NOT_INITIALIZED")
		}
		return errResp(c, http.StatusNotFound, "SoA entry not found", "CK_SOA_ENTRY_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, entry)
}

// UpdateDedicatedSoAEntry handles PUT /api/v1/vaktcomply/soa/entries/:control_ref
func (h *Handler) UpdateDedicatedSoAEntry(c echo.Context) error {
	var in UpdateSoAEntryInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	if err := h.service.UpdateDedicatedSoAEntry(c.Request().Context(), orgID(c), c.Param("control_ref"), in); err != nil {
		if errors.Is(err, ErrSoANotInitialized) {
			return errResp(c, http.StatusNotFound, err.Error(), "CK_SOA_NOT_INITIALIZED")
		}
		log.Error().Err(err).Msg("update dedicated soa entry")
		return errResp(c, http.StatusBadRequest, err.Error(), "CK_SOA_UPDATE_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// ApproveDedicatedSoA handles POST /api/v1/vaktcomply/soa/approve
func (h *Handler) ApproveDedicatedSoA(c echo.Context) error {
	var in struct {
		Notes string `json:"notes"`
	}
	_ = c.Bind(&in)
	if err := h.service.ApproveDedicatedSoA(c.Request().Context(), orgID(c), userID(c)); err != nil {
		if errors.Is(err, ErrExclusionReasonRequired) {
			return errResp(c, http.StatusUnprocessableEntity, err.Error(), "CK_SOA_EXCLUSION_REASON_MISSING")
		}
		log.Error().Err(err).Msg("approve dedicated soa")
		return errResp(c, http.StatusInternalServerError, "failed to approve SoA", "CK_SOA_APPROVE_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// GetDedicatedSoAVersions handles GET /api/v1/vaktcomply/soa/versions
func (h *Handler) GetDedicatedSoAVersions(c echo.Context) error {
	versions, err := h.service.GetDedicatedSoAVersions(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("get dedicated soa versions")
		return errResp(c, http.StatusInternalServerError, "failed to get SoA versions", "CK_SOA_FAILED")
	}
	return c.JSON(http.StatusOK, versions)
}

// GetDedicatedSoASummary handles GET /api/v1/vaktcomply/soa/summary
func (h *Handler) GetDedicatedSoASummary(c echo.Context) error {
	summary, err := h.service.GetDedicatedSoASummary(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("get dedicated soa summary")
		return errResp(c, http.StatusInternalServerError, "failed to get SoA summary", "CK_SOA_FAILED")
	}
	return c.JSON(http.StatusOK, summary)
}

// ExportDedicatedSoAXLSX handles GET /api/v1/vaktcomply/soa/export.xlsx
// Requires FeatureAuditPDF (same gate as PDF export).
func (h *Handler) ExportDedicatedSoAXLSX(c echo.Context) error {
	if !features.IsEnabled(c, features.FeatureAuditPDF) {
		return errResp(c, http.StatusPaymentRequired, "XLSX export requires Pro", "CK_FEATURE_REQUIRED")
	}
	ctx := c.Request().Context()
	org := orgID(c)

	entries, err := h.service.ListDedicatedSoAEntries(ctx, org)
	if err != nil {
		if errors.Is(err, ErrSoANotInitialized) {
			return errResp(c, http.StatusNotFound, err.Error(), "CK_SOA_NOT_INITIALIZED")
		}
		log.Error().Err(err).Msg("export soa xlsx: list entries")
		return errResp(c, http.StatusInternalServerError, "export failed", "CK_SOA_EXPORT_FAILED")
	}

	summary, err := h.service.GetDedicatedSoASummary(ctx, org)
	if err != nil {
		log.Error().Err(err).Msg("export soa xlsx: get summary")
		return errResp(c, http.StatusInternalServerError, "export failed", "CK_SOA_EXPORT_FAILED")
	}

	rows := make([]xlsxexport.SoARow, len(entries))
	for i, e := range entries {
		justification := e.Justification
		if !e.Applicable && e.ExclusionReason != "" {
			justification = e.ExclusionReason
		}
		owner := ""
		if e.ApprovedBy != nil {
			owner = *e.ApprovedBy
		}
		rows[i] = xlsxexport.SoARow{
			ControlRef:           e.ControlRef,
			ControlName:          e.ControlName,
			ControlGroup:         e.ControlGroup,
			Applicable:           e.Applicable,
			Justification:        justification,
			ImplementationStatus: e.ImplementationStatus,
			Owner:                owner,
			UpdatedAt:            e.UpdatedAt,
		}
	}

	var xlsSummary xlsxexport.SoASummary
	if summary != nil {
		xlsSummary = xlsxexport.SoASummary{
			ApplicableCount:   summary.ApplicableCount,
			ExcludedCount:     summary.ExcludedCount,
			ImplementedCount:  summary.ImplementedCount,
			PartialCount:      summary.PartialCount,
			PlannedCount:      summary.PlannedCount,
			NotStartedCount:   summary.NotStartedCount,
			ImplementationPct: summary.ImplementationPct,
		}
	}

	data, err := xlsxexport.RenderSoA(rows, xlsSummary)
	if err != nil {
		log.Error().Err(err).Msg("export soa xlsx: render")
		return errResp(c, http.StatusInternalServerError, "export failed", "CK_SOA_EXPORT_FAILED")
	}

	filename := fmt.Sprintf("soa-%s.xlsx", time.Now().UTC().Format("2006-01-02"))
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))
	return c.Blob(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", data)
}

// ExportDedicatedSoA handles GET /api/v1/vaktcomply/soa/export
func (h *Handler) ExportDedicatedSoA(c echo.Context) error {
	format := c.QueryParam("format")
	if format == "" {
		format = "pdf"
	}
	ctx := c.Request().Context()
	org := orgID(c)

	switch format {
	case "pdf":
		data, err := h.service.ExportDedicatedSoAPDF(ctx, org)
		if err != nil {
			if errors.Is(err, ErrSoANotInitialized) {
				return errResp(c, http.StatusNotFound, err.Error(), "CK_SOA_NOT_INITIALIZED")
			}
			log.Error().Err(err).Msg("export dedicated soa pdf")
			return errResp(c, http.StatusInternalServerError, "export failed", "CK_SOA_EXPORT_FAILED")
		}
		filename := fmt.Sprintf("vakt-soa-v%s.pdf", time.Now().UTC().Format("2006-01-02"))
		c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))
		return c.Blob(http.StatusOK, "application/pdf", data)

	default: // csv / xlsx
		rows, err := h.service.ExportDedicatedSoACSV(ctx, org)
		if err != nil {
			if errors.Is(err, ErrSoANotInitialized) {
				return errResp(c, http.StatusNotFound, err.Error(), "CK_SOA_NOT_INITIALIZED")
			}
			return errResp(c, http.StatusInternalServerError, "export failed", "CK_SOA_EXPORT_FAILED")
		}
		filename := fmt.Sprintf("vakt-soa-%s.csv", time.Now().UTC().Format("2006-01-02"))
		c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))
		c.Response().Header().Set("Content-Type", "text/csv; charset=utf-8")
		w := csv.NewWriter(c.Response().Writer)
		for _, row := range rows {
			_ = w.Write(row)
		}
		w.Flush()
		return nil
	}
}
