package secvitals

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/shared/audit"
	"github.com/matharnica/vakt/internal/shared/pagination"
)

// ListFrameworks handles GET /api/v1/secvitals/frameworks.
func (h *Handler) ListFrameworks(c echo.Context) error {
	frameworks, err := h.service.ListFrameworks(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list frameworks")
		return errResp(c, http.StatusInternalServerError, "failed to list frameworks", "CK_LIST_FRAMEWORKS_FAILED")
	}
	return c.JSON(http.StatusOK, frameworks)
}

// EnableFramework handles POST /api/v1/secvitals/frameworks/:name/enable.
func (h *Handler) EnableFramework(c echo.Context) error {
	name := c.Param("name")
	if name == "" {
		return errResp(c, http.StatusBadRequest, "framework name is required", "CK_BAD_REQUEST")
	}

	fw, err := h.service.EnableFramework(c.Request().Context(), orgID(c), name)
	if err != nil {
		log.Error().Err(err).Str("name", name).Msg("enable framework")
		return errResp(c, http.StatusInternalServerError, "failed to enable framework", "CK_ENABLE_FRAMEWORK_FAILED")
	}
	audit.Write(c.Request().Context(), h.db, audit.WriteEntry{
		OrgID: orgID(c), UserID: userID(c), Action: "create",
		ResourceType: "vakt-comply/framework", ResourceID: fw.ID, ResourceName: fw.Name,
		IPAddress: c.RealIP(),
	})
	return c.JSON(http.StatusCreated, fw)
}

// DeleteFramework handles DELETE /api/v1/secvitals/frameworks/:id.
func (h *Handler) DeleteFramework(c echo.Context) error {
	frameworkID := c.Param("id")
	if frameworkID == "" {
		return errResp(c, http.StatusBadRequest, "framework id is required", "CK_BAD_REQUEST")
	}
	if err := h.service.DeleteFramework(c.Request().Context(), orgID(c), frameworkID); err != nil {
		log.Error().Err(err).Str("framework_id", frameworkID).Msg("delete framework")
		return errResp(c, http.StatusInternalServerError, "failed to delete framework", "CK_DELETE_FRAMEWORK_FAILED")
	}
	audit.Write(c.Request().Context(), h.db, audit.WriteEntry{
		OrgID: orgID(c), UserID: userID(c), Action: "delete",
		ResourceType: "vakt-comply/framework", ResourceID: frameworkID,
		IPAddress: c.RealIP(),
	})
	return c.NoContent(http.StatusNoContent)
}

// GetFrameworkByID handles GET /api/v1/secvitals/frameworks/:id.
func (h *Handler) GetFrameworkByID(c echo.Context) error {
	fw, err := h.service.GetFramework(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		return errResp(c, http.StatusNotFound, "framework not found", "CK_FRAMEWORK_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, fw)
}

// GetReadinessReport handles GET /api/v1/secvitals/frameworks/:id/report.
func (h *Handler) GetReadinessReport(c echo.Context) error {
	frameworkID := c.Param("id")
	report, err := h.service.GetReadinessReport(c.Request().Context(), orgID(c), frameworkID)
	if err != nil {
		log.Error().Err(err).Str("framework_id", frameworkID).Msg("get readiness report")
		return errResp(c, http.StatusInternalServerError, "failed to generate readiness report", "CK_READINESS_REPORT_FAILED")
	}
	return c.JSON(http.StatusOK, report)
}

// GetGapAnalysis handles GET /api/v1/secvitals/frameworks/:id/gaps.
func (h *Handler) GetGapAnalysis(c echo.Context) error {
	frameworkID := c.Param("id")
	analysis, err := h.service.GetGapAnalysis(c.Request().Context(), orgID(c), frameworkID)
	if err != nil {
		log.Error().Err(err).Str("framework_id", frameworkID).Msg("get gap analysis")
		return errResp(c, http.StatusInternalServerError, "failed to generate gap analysis", "CK_GAP_ANALYSIS_FAILED")
	}
	return c.JSON(http.StatusOK, analysis)
}

// ListControls handles GET /api/v1/secvitals/frameworks/:id/controls.
func (h *Handler) ListControls(c echo.Context) error {
	frameworkID := c.Param("id")
	offset, limit, meta := pagination.FromRequest(c)
	controls, total, err := h.service.ListControlsPaged(c.Request().Context(), orgID(c), frameworkID, offset, limit)
	if err != nil {
		log.Error().Err(err).Str("framework_id", frameworkID).Msg("list controls")
		return errResp(c, http.StatusInternalServerError, "failed to list controls", "CK_LIST_CONTROLS_FAILED")
	}
	pagination.Complete(&meta, total)
	return c.JSON(http.StatusOK, pagination.Wrap(controls, meta))
}

// ListAvailableFrameworks handles GET /api/v1/secvitals/frameworks/available.
// Returns all frameworks (builtin + installed plugins) with their enabled status for this org.
func (h *Handler) ListAvailableFrameworks(c echo.Context) error {
	available, err := h.service.ListAvailableFrameworks(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list available frameworks")
		return errResp(c, http.StatusInternalServerError, "failed to list available frameworks", "CK_LIST_AVAILABLE_FAILED")
	}
	return c.JSON(http.StatusOK, available)
}

// InstallFrameworkPlugin handles POST /api/v1/secvitals/frameworks/install.
// Accepts a YAML plugin file (multipart field "file") and installs the framework.
func (h *Handler) InstallFrameworkPlugin(c echo.Context) error {
	file, err := c.FormFile("file")
	if err != nil {
		return errResp(c, http.StatusBadRequest, "multipart field 'file' is required", "CK_BAD_REQUEST")
	}
	if file.Size > 1<<20 { // 1 MB max
		return errResp(c, http.StatusRequestEntityTooLarge, "plugin file too large (max 1 MB)", "CK_PLUGIN_TOO_LARGE")
	}

	src, err := file.Open()
	if err != nil {
		return errResp(c, http.StatusInternalServerError, "failed to open uploaded file", "CK_PLUGIN_OPEN_ERROR")
	}
	defer src.Close()

	data, err := io.ReadAll(src)
	if err != nil {
		return errResp(c, http.StatusInternalServerError, "failed to read uploaded file", "CK_PLUGIN_READ_ERROR")
	}

	var plugin FrameworkPlugin
	if err := yamlUnmarshal(data, &plugin); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, "invalid plugin YAML: "+err.Error(), "CK_PLUGIN_INVALID_YAML")
	}
	if plugin.Name == "" {
		return errResp(c, http.StatusUnprocessableEntity, "plugin 'name' field is required", "CK_PLUGIN_MISSING_NAME")
	}

	fw, err := h.service.InstallFrameworkPlugin(c.Request().Context(), orgID(c), &plugin)
	if err != nil {
		log.Error().Err(err).Str("plugin", plugin.Name).Msg("install framework plugin")
		return errResp(c, http.StatusInternalServerError, "failed to install framework plugin", "CK_PLUGIN_INSTALL_FAILED")
	}
	return c.JSON(http.StatusCreated, fw)
}

// ListFrameworkMappings handles GET /api/v1/secvitals/framework-mappings.
func (h *Handler) ListFrameworkMappings(c echo.Context) error {
	mappings, err := h.service.ListFrameworkMappings(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list framework mappings")
		return errResp(c, http.StatusInternalServerError, "failed to list framework mappings", "CK_LIST_MAPPINGS_FAILED")
	}
	if mappings == nil {
		mappings = []FrameworkMapping{}
	}
	return c.JSON(http.StatusOK, mappings)
}

// DeleteFrameworkMapping handles DELETE /api/v1/secvitals/framework-mappings/:id.
func (h *Handler) DeleteFrameworkMapping(c echo.Context) error {
	mappingID := c.Param("id")
	if mappingID == "" {
		return errResp(c, http.StatusBadRequest, "mapping id is required", "CK_BAD_REQUEST")
	}
	if err := h.service.DeleteFrameworkMapping(c.Request().Context(), orgID(c), mappingID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return errResp(c, http.StatusNotFound, "mapping not found", "CK_MAPPING_NOT_FOUND")
		}
		log.Error().Err(err).Str("mapping_id", mappingID).Msg("delete framework mapping")
		return errResp(c, http.StatusInternalServerError, "failed to delete framework mapping", "CK_DELETE_MAPPING_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// GetTISAXControls handles GET /api/v1/secvitals/frameworks/:id/tisax-controls.
// Query param: protection_level (default: "normal"). Use "very_high" to include chapter 15 controls.
func (h *Handler) GetTISAXControls(c echo.Context) error {
	frameworkID := c.Param("id")
	protectionLevel := c.QueryParam("protection_level")
	if protectionLevel == "" {
		protectionLevel = "normal"
	}
	controls, err := h.service.ListTISAXControls(c.Request().Context(), orgID(c), frameworkID, protectionLevel)
	if err != nil {
		log.Error().Err(err).Str("framework_id", frameworkID).Str("protection_level", protectionLevel).Msg("get tisax controls")
		return errResp(c, http.StatusInternalServerError, "failed to list TISAX controls", "CK_LIST_TISAX_CONTROLS_FAILED")
	}
	return c.JSON(http.StatusOK, controls)
}

// GetTISAXGapAnalysis handles GET /api/v1/secvitals/frameworks/:id/tisax-gaps.
func (h *Handler) GetTISAXGapAnalysis(c echo.Context) error {
	frameworkID := c.Param("id")
	analysis, err := h.service.GetTISAXGapAnalysis(c.Request().Context(), orgID(c), frameworkID)
	if err != nil {
		log.Error().Err(err).Str("framework_id", frameworkID).Msg("get tisax gap analysis")
		return errResp(c, http.StatusInternalServerError, "failed to generate TISAX gap analysis", "CK_TISAX_GAP_ANALYSIS_FAILED")
	}
	return c.JSON(http.StatusOK, analysis)
}

// GetTISAXISOMapping handles GET /api/v1/secvitals/frameworks/tisax/iso-mapping.
// Query param: framework_id (optional). If omitted, the TISAX framework is looked up by name.
func (h *Handler) GetTISAXISOMapping(c echo.Context) error {
	ctx := c.Request().Context()
	oid := orgID(c)

	frameworkID := c.QueryParam("framework_id")
	if frameworkID == "" {
		fw, err := h.service.FindFrameworkByName(ctx, oid, "TISAX")
		if err != nil || fw == nil {
			return c.JSON(http.StatusOK, []MappingResult{})
		}
		frameworkID = fw.ID
	}

	results, err := h.service.GetTISAXCoverageByISO(ctx, oid, frameworkID)
	if err != nil {
		log.Error().Err(err).Str("framework_id", frameworkID).Msg("get tisax iso mapping")
		return errResp(c, http.StatusInternalServerError, "failed to compute TISAX↔ISO mapping", "CK_TISAX_ISO_MAPPING_FAILED")
	}
	return c.JSON(http.StatusOK, results)
}

// GetTISAXCoverageAfterISO handles GET /api/v1/secvitals/frameworks/tisax/coverage-after-iso.
// Returns only TISAX controls NOT covered by their mapped ISO 27001 control.
func (h *Handler) GetTISAXCoverageAfterISO(c echo.Context) error {
	ctx := c.Request().Context()
	oid := orgID(c)

	frameworkID := c.QueryParam("framework_id")
	if frameworkID == "" {
		fw, err := h.service.FindFrameworkByName(ctx, oid, "TISAX")
		if err != nil || fw == nil {
			return c.JSON(http.StatusOK, []Control{})
		}
		frameworkID = fw.ID
	}

	gaps, err := h.service.GetTISAXGapsAfterISO(ctx, oid, frameworkID)
	if err != nil {
		log.Error().Err(err).Str("framework_id", frameworkID).Msg("get tisax coverage after iso")
		return errResp(c, http.StatusInternalServerError, "failed to compute TISAX gaps after ISO", "CK_TISAX_GAPS_FAILED")
	}
	if gaps == nil {
		gaps = []Control{}
	}
	return c.JSON(http.StatusOK, gaps)
}

// GetDSGVOTOMCoverage handles GET /api/v1/secvitals/dsgvo/tom-coverage.
func (h *Handler) GetDSGVOTOMCoverage(c echo.Context) error {
	ctx := c.Request().Context()
	org := orgID(c)
	frameworkID := c.QueryParam("framework_id")
	if frameworkID == "" {
		fw, err := h.service.FindFrameworkByName(ctx, org, "DSGVO-TOM")
		if err != nil {
			log.Error().Err(err).Msg("get dsgvo-tom framework")
			return echo.ErrInternalServerError
		}
		if fw == nil {
			return c.JSON(http.StatusOK, map[string]any{"data": []any{}})
		}
		frameworkID = fw.ID
	}
	results, err := h.service.GetDSGVOTOMCoverage(ctx, org, frameworkID)
	if err != nil {
		log.Error().Err(err).Str("framework_id", frameworkID).Msg("get dsgvo tom coverage")
		return echo.ErrInternalServerError
	}
	return c.JSON(http.StatusOK, map[string]any{"data": results})
}

// ExportSoAPDF handles GET /api/v1/secvitals/frameworks/:id/soa.pdf.
func (h *Handler) ExportSoAPDF(c echo.Context) error {
	pdfBytes, filename, err := h.service.ExportSoAPDF(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		log.Error().Err(err).Str("framework_id", c.Param("id")).Msg("export soa pdf")
		return errResp(c, http.StatusInternalServerError, "failed to generate soa pdf", "CK_SOA_PDF_FAILED")
	}
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	return c.Blob(http.StatusOK, "application/pdf", pdfBytes)
}

// ExportFrameworkPDF handles GET /api/v1/secvitals/frameworks/:id/export-pdf.
func (h *Handler) ExportFrameworkPDF(c echo.Context) error {
	pdfBytes, filename, err := h.service.ExportFrameworkPDF(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		log.Error().Err(err).Str("framework_id", c.Param("id")).Msg("export framework pdf")
		return errResp(c, http.StatusInternalServerError, "failed to generate pdf", "CK_PDF_EXPORT_FAILED")
	}
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	return c.Blob(http.StatusOK, "application/pdf", pdfBytes)
}

// ExportTISAXReportPDF handles GET /api/v1/secvitals/frameworks/:id/tisax-report-pdf.
// Query params: protection_level (default "normal"), assessment_level (default "AL2").
func (h *Handler) ExportTISAXReportPDF(c echo.Context) error {
	frameworkID := c.Param("id")
	protectionLevel := c.QueryParam("protection_level")
	if protectionLevel == "" {
		protectionLevel = "normal"
	}
	assessmentLevel := c.QueryParam("assessment_level")
	if assessmentLevel == "" {
		assessmentLevel = "AL2"
	}

	pdfBytes, filename, err := h.service.ExportTISAXReportPDF(
		c.Request().Context(), orgID(c), frameworkID, protectionLevel, assessmentLevel,
	)
	if err != nil {
		if strings.Contains(err.Error(), "invalid protection_level") || strings.Contains(err.Error(), "invalid assessment_level") {
			return errResp(c, http.StatusBadRequest, err.Error(), "CK_TISAX_PDF_BAD_PARAMS")
		}
		log.Error().Err(err).Str("framework_id", frameworkID).Msg("export tisax report pdf")
		return errResp(c, http.StatusInternalServerError, "failed to generate TISAX report PDF", "CK_TISAX_PDF_FAILED")
	}
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	return c.Blob(http.StatusOK, "application/pdf", pdfBytes)
}
