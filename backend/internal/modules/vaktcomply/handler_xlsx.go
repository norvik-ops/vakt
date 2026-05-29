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
)

// xlsxContentType is the IANA media type for Excel OOXML spreadsheets.
// Most versions of Excel will open a CSV served with this type.
const xlsxContentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

// ExportRisksXLSX handles GET /api/v1/vaktcomply/risks/export/xlsx.
// Returns all risks for the org as a spreadsheet-compatible CSV.
func (h *Handler) ExportRisksXLSX(c echo.Context) error {
	ctx := c.Request().Context()
	org := orgID(c)

	risks, _, err := h.service.ListRisksPaged(ctx, org, 0, 10_000)
	if err != nil {
		log.Error().Err(err).Str("org_id", org).Msg("export risks xlsx")
		return errResp(c, http.StatusInternalServerError, "export failed", "CK_EXPORT_ERROR")
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"Title", "Probability", "Impact", "Score", "Status"})
	for _, r := range risks {
		_ = w.Write([]string{
			r.Title,
			fmt.Sprintf("%d", r.Likelihood),
			fmt.Sprintf("%d", r.Impact),
			fmt.Sprintf("%d", r.RiskScore),
			r.Status,
		})
	}
	w.Flush()

	c.Response().Header().Set("Content-Disposition", `attachment; filename="risks.xlsx"`)
	return c.Blob(http.StatusOK, xlsxContentType, buf.Bytes())
}

// ExportControlsXLSX handles GET /api/v1/vaktcomply/controls/export/xlsx.
// Optional query param: framework_id to filter controls by framework.
// Returns columns: Title, Framework, Status, Owner, Due Date.
func (h *Handler) ExportControlsXLSX(c echo.Context) error {
	ctx := c.Request().Context()
	org := orgID(c)
	frameworkID := c.QueryParam("framework_id")

	controls, err := h.service.ListControls(ctx, org, frameworkID)
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
