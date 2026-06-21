// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/shared/platform/features"
	"github.com/matharnica/vakt/internal/shared/xlsxexport"
)

// xlsxContentType is the IANA media type for Excel OOXML spreadsheets.
const xlsxContentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

// ExportRisksXLSX handles GET /api/v1/vaktcomply/risks/export/xlsx.
// Returns all org risks as a proper XLSX workbook (two sheets: Risiken + Matrix).
// Requires FeatureAuditPDF.
func (h *Handler) ExportRisksXLSX(c echo.Context) error {
	if !features.IsEnabled(c, features.FeatureAuditPDF) {
		return errResp(c, http.StatusPaymentRequired, "XLSX export requires Pro", "CK_FEATURE_REQUIRED")
	}
	ctx := c.Request().Context()
	org := orgID(c)

	risks, _, err := h.service.Risk.ListRisksPaged(ctx, org, 0, 10_000)
	if err != nil {
		log.Error().Err(err).Str("org_id", org).Msg("export risks xlsx")
		return errResp(c, http.StatusInternalServerError, "export failed", "CK_EXPORT_ERROR")
	}

	rows := make([]xlsxexport.RiskRow, len(risks))
	for i, r := range risks {
		rows[i] = xlsxexport.RiskRow{
			ID:            r.ID,
			Title:         r.Title,
			Category:      r.Category,
			Likelihood:    r.Likelihood,
			Impact:        r.Impact,
			RiskScore:     r.RiskScore,
			Treatment:     r.Treatment,
			Status:        r.Status,
			Owner:         r.Owner,
			DueDate:       r.TreatmentDueDate,
			ResidualScore: r.ResidualScore,
		}
	}

	data, err := xlsxexport.RenderRisiken(rows)
	if err != nil {
		log.Error().Err(err).Msg("export risks xlsx: render")
		return errResp(c, http.StatusInternalServerError, "export failed", "CK_EXPORT_ERROR")
	}

	filename := fmt.Sprintf("risikoregister-%s.xlsx", time.Now().UTC().Format("2006-01-02"))
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))
	return c.Blob(http.StatusOK, xlsxContentType, data)
}

// ExportControlsXLSX handles GET /api/v1/vaktcomply/controls/export/xlsx.
// Optional query param: framework_id to filter controls by framework.
// Returns columns: Title, Framework, Status, Owner, Due Date.
func (h *Handler) ExportControlsXLSX(c echo.Context) error {
	ctx := c.Request().Context()
	org := orgID(c)
	frameworkID := c.QueryParam("framework_id")
	if frameworkID == "" {
		return errResp(c, http.StatusBadRequest, "framework_id is required", "CK_MISSING_PARAM")
	}

	controls, err := h.service.Policy.ListControls(ctx, org, frameworkID)
	if err != nil {
		log.Error().Err(err).Str("org_id", org).Str("framework_id", frameworkID).Msg("export controls xlsx")
		return errResp(c, http.StatusInternalServerError, "export failed", "CK_EXPORT_ERROR")
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"Title", "Framework", "Status", "Owner", "Due Date"})
	for _, ctrl := range controls {
		dueDate := ""
		if ctrl.NextReviewDue != nil {
			dueDate = ctrl.NextReviewDue.Format(time.DateOnly)
		}
		_ = w.Write([]string{
			ctrl.Title,
			ctrl.FrameworkID,
			ctrl.Status,
			ctrl.LastReviewedBy,
			dueDate,
		})
	}
	w.Flush()

	c.Response().Header().Set("Content-Disposition", `attachment; filename="controls.xlsx"`)
	return c.Blob(http.StatusOK, xlsxContentType, buf.Bytes())
}
