// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// GetKPIDashboard handles GET /api/v1/vaktcomply/kpi-dashboard.
// Returns the latest KPI snapshot and 90-day history for the authenticated organisation.
func (h *Handler) GetKPIDashboard(c echo.Context) error {
	dashboard, err := h.service.GetKPIDashboard(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("get kpi dashboard")
		return errResp(c, http.StatusInternalServerError, "failed to load KPI dashboard", "CK_KPI_DASHBOARD_FAILED")
	}
	return c.JSON(http.StatusOK, dashboard)
}

// ExportKPIReportPDF handles GET /api/v1/vaktcomply/kpi-dashboard/export-pdf.
// PDF export is not yet implemented — returns 501 Not Implemented.
func (h *Handler) ExportKPIReportPDF(c echo.Context) error {
	return c.JSON(http.StatusNotImplemented, map[string]string{
		"error": "PDF export not yet implemented",
		"code":  "CK_KPI_PDF_NOT_IMPLEMENTED",
	})
}
