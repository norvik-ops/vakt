// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S74-4: BSI Referenzberichte A1–A6 Handler

package vaktcomply

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// ListBSIReportExports handles GET /api/v1/vaktcomply/bsi/reports
func (h *Handler) ListBSIReportExports(c echo.Context) error {
	exports, err := h.service.ListBSIReportExports(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list bsi report exports")
		return errResp(c, http.StatusInternalServerError, "failed to list report exports", "CK_BSI_REPORTS_FAILED")
	}
	return c.JSON(http.StatusOK, exports)
}

// GenerateBSIReport handles GET /api/v1/vaktcomply/bsi/reports/:type
func (h *Handler) GenerateBSIReport(c echo.Context) error {
	reportType := c.Param("type")
	data, err := h.service.GenerateBSIReport(c.Request().Context(), orgID(c), userID(c), reportType)
	if err != nil {
		log.Error().Err(err).Str("type", reportType).Msg("generate bsi report")
		return errResp(c, http.StatusInternalServerError, "failed to generate report", "CK_BSI_REPORT_GEN_FAILED")
	}
	filename := "bsi-report-" + reportType + ".pdf"
	c.Response().Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	return c.Blob(http.StatusOK, "application/pdf", data)
}

// GetBSIReportPreview handles GET /api/v1/vaktcomply/bsi/reports/:type/preview
func (h *Handler) GetBSIReportPreview(c echo.Context) error {
	reportType := c.Param("type")
	preview, err := h.service.GetBSIReportPreview(c.Request().Context(), orgID(c), reportType)
	if err != nil {
		log.Error().Err(err).Str("type", reportType).Msg("bsi report preview")
		return errResp(c, http.StatusInternalServerError, "failed to get report preview", "CK_BSI_PREVIEW_FAILED")
	}
	return c.JSON(http.StatusOK, preview)
}
