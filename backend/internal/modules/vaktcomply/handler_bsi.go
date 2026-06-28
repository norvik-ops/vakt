package vaktcomply

import (
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	bsi "github.com/matharnica/vakt/internal/modules/vaktcomply/bsi"
	"github.com/rs/zerolog/log"
)

func (h *Handler) ListBSIObjectDependencies(c echo.Context) error {
	id := c.Param("id")
	deps, err := h.service.BSI.ListBSIObjectDependencies(c.Request().Context(), orgID(c), id)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("list bsi object dependencies")
		return errResp(c, http.StatusInternalServerError, "failed to list dependencies", "CK_BSI_DEP_LIST_FAILED")
	}
	if deps == nil {
		deps = []bsi.BSIObjectDependency{}
	}
	return c.JSON(http.StatusOK, deps)
}

// CreateBSIObjectDependency handles POST /api/v1/vaktcomply/bsi/target-objects/:id/dependencies
func (h *Handler) CreateBSIObjectDependency(c echo.Context) error {
	sourceID := c.Param("id")
	var in bsi.CreateBSIObjectDependencyInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	dep, err := h.service.BSI.CreateBSIObjectDependency(c.Request().Context(), orgID(c), sourceID, in)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "target object not found", "CK_BSI_TO_NOT_FOUND")
		}
		if errors.Is(err, bsi.ErrCycle) {
			return errResp(c, http.StatusUnprocessableEntity, "adding this dependency would create a cycle", "CK_BSI_DEP_CYCLE")
		}
		if errors.Is(err, bsi.ErrConflict) {
			return errResp(c, http.StatusConflict, "dependency already exists", "CK_BSI_DEP_CONFLICT")
		}
		log.Error().Err(err).Str("source_id", sourceID).Msg("create bsi object dependency")
		return errResp(c, http.StatusInternalServerError, "failed to create dependency", "CK_BSI_DEP_CREATE_FAILED")
	}
	return c.JSON(http.StatusCreated, dep)
}

// DeleteBSIObjectDependency handles DELETE /api/v1/vaktcomply/bsi/target-objects/:id/dependencies/:depId
func (h *Handler) DeleteBSIObjectDependency(c echo.Context) error {
	depID := c.Param("depId")
	err := h.service.BSI.DeleteBSIObjectDependency(c.Request().Context(), orgID(c), depID)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "dependency not found", "CK_BSI_DEP_NOT_FOUND")
		}
		log.Error().Err(err).Str("dep_id", depID).Msg("delete bsi object dependency")
		return errResp(c, http.StatusInternalServerError, "failed to delete dependency", "CK_BSI_DEP_DELETE_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// UpdateBSIObjectProtectionOverride handles PUT /api/v1/vaktcomply/bsi/target-objects/:id/protection-override
func (h *Handler) UpdateBSIObjectProtectionOverride(c echo.Context) error {
	id := c.Param("id")
	var in bsi.UpdateBSIObjectProtectionOverrideInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	obj, err := h.service.BSI.UpdateBSIObjectProtectionOverride(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "target object not found", "CK_BSI_TO_NOT_FOUND")
		}
		if errors.Is(err, bsi.ErrOverrideReasonMissing) {
			return errResp(c, http.StatusUnprocessableEntity, err.Error(), "CK_BSI_OVERRIDE_REASON_REQUIRED")
		}
		log.Error().Err(err).Str("id", id).Msg("update bsi protection override")
		return errResp(c, http.StatusInternalServerError, "failed to update protection override", "CK_BSI_OVERRIDE_FAILED")
	}
	return c.JSON(http.StatusOK, obj)
}

func (h *Handler) GetBSIModelingMatrix(c echo.Context) error {
	entries, err := h.service.BSI.GetBSIModelingMatrix(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("get bsi modeling matrix")
		return errResp(c, http.StatusInternalServerError, "failed to get BSI modeling matrix", "CK_BSI_MATRIX_FAILED")
	}
	if entries == nil {
		entries = []bsi.BSIModelingEntry{}
	}
	return c.JSON(http.StatusOK, entries)
}

// CreateBSIModeling handles POST /api/v1/vaktcomply/bsi-modeling.
func (h *Handler) CreateBSIModeling(c echo.Context) error {
	var in bsi.CreateBSIModelingInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	entry, err := h.service.BSI.CreateBSIModeling(c.Request().Context(), orgID(c), userID(c), in)
	if err != nil {
		if strings.Contains(err.Error(), "mapping already exists") {
			return errResp(c, http.StatusConflict, "A mapping for this asset and control already exists", "CK_BSI_DUPLICATE")
		}
		log.Error().Err(err).Msg("create bsi modeling")
		return errResp(c, http.StatusInternalServerError, "failed to create BSI modeling entry", "CK_BSI_CREATE_FAILED")
	}
	return c.JSON(http.StatusCreated, entry)
}

// UpdateBSIModeling handles PATCH /api/v1/vaktcomply/bsi-modeling/:id.
func (h *Handler) UpdateBSIModeling(c echo.Context) error {
	id := c.Param("id")
	var in bsi.UpdateBSIModelingInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	entry, err := h.service.BSI.UpdateBSIModeling(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return errResp(c, http.StatusNotFound, "BSI modeling entry not found", "CK_BSI_NOT_FOUND")
		}
		log.Error().Err(err).Str("id", id).Msg("update bsi modeling")
		return errResp(c, http.StatusInternalServerError, "failed to update BSI modeling entry", "CK_BSI_UPDATE_FAILED")
	}
	return c.JSON(http.StatusOK, entry)
}

// DeleteBSIModeling handles DELETE /api/v1/vaktcomply/bsi-modeling/:id.
func (h *Handler) DeleteBSIModeling(c echo.Context) error {
	id := c.Param("id")
	if err := h.service.BSI.DeleteBSIModeling(c.Request().Context(), orgID(c), id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return errResp(c, http.StatusNotFound, "BSI modeling entry not found", "CK_BSI_NOT_FOUND")
		}
		log.Error().Err(err).Str("id", id).Msg("delete bsi modeling")
		return errResp(c, http.StatusInternalServerError, "failed to delete BSI modeling entry", "CK_BSI_DELETE_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// GetBSIBausteinSuggestions handles GET /api/v1/vaktcomply/bsi-modeling/suggestions.
// Query param: ?asset_type=server
func (h *Handler) GetBSIBausteinSuggestions(c echo.Context) error {
	assetType := c.QueryParam("asset_type")
	suggestions := h.service.BSI.GetSuggestedBausteine(assetType)
	return c.JSON(http.StatusOK, map[string][]string{"suggestions": suggestions})
}

// ExportBSIModelingPDF handles GET /api/v1/vaktcomply/bsi-modeling/export-pdf.
// Not yet implemented — returns 501.
func (h *Handler) ExportBSIModelingPDF(c echo.Context) error {
	return errResp(c, http.StatusNotImplemented, "PDF export not yet implemented", "CK_BSI_PDF_NOT_IMPLEMENTED")
}

// ExportBSIModelingXLSX handles GET /api/v1/vaktcomply/bsi-modeling/export-xlsx.
// Not yet implemented — returns 501.
func (h *Handler) ExportBSIModelingXLSX(c echo.Context) error {
	return errResp(c, http.StatusNotImplemented, "XLSX export not yet implemented", "CK_BSI_XLSX_NOT_IMPLEMENTED")
}

// GetBSIModelingStats handles GET /api/v1/vaktcomply/bsi-modeling/stats.
func (h *Handler) GetBSIModelingStats(c echo.Context) error {
	stats, err := h.service.BSI.GetBSIModelingStats(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("get bsi modeling stats")
		return errResp(c, http.StatusInternalServerError, "failed to get BSI modeling stats", "CK_BSI_STATS_FAILED")
	}
	return c.JSON(http.StatusOK, stats)
}

func (h *Handler) ListBSIReportExports(c echo.Context) error {
	exports, err := h.service.BSI.ListBSIReportExports(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list bsi report exports")
		return errResp(c, http.StatusInternalServerError, "failed to list report exports", "CK_BSI_REPORTS_FAILED")
	}
	return c.JSON(http.StatusOK, exports)
}

// GenerateBSIReport handles GET /api/v1/vaktcomply/bsi/reports/:type
func (h *Handler) GenerateBSIReport(c echo.Context) error {
	reportType := c.Param("type")
	data, err := h.service.BSI.GenerateBSIReport(c.Request().Context(), orgID(c), userID(c), reportType)
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
	preview, err := h.service.BSI.GetBSIReportPreview(c.Request().Context(), orgID(c), reportType)
	if err != nil {
		log.Error().Err(err).Str("type", reportType).Msg("bsi report preview")
		return errResp(c, http.StatusInternalServerError, "failed to get report preview", "CK_BSI_PREVIEW_FAILED")
	}
	return c.JSON(http.StatusOK, preview)
}

// ExportBCMHandbuchPDF handles GET /api/v1/vaktcomply/bcm/report.pdf
// Requires FeatureAuditPDF.
func (h *Handler) ExportBCMHandbuchPDF(c echo.Context) error {
	ctx := c.Request().Context()
	data, err := h.service.BCM.GenerateBCMHandbuchPDF(ctx, orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("generate bcm handbuch pdf")
		return errResp(c, http.StatusInternalServerError, "failed to generate Notfallhandbuch PDF", "CK_BCM_PDF_FAILED")
	}
	h.service.BSI.LogBCMReportExport(ctx, orgID(c), userID(c), data)
	c.Response().Header().Set("Content-Disposition", `attachment; filename="notfallhandbuch.pdf"`)
	return c.Blob(http.StatusOK, "application/pdf", data)
}

func (h *Handler) ListBSIThreats(c echo.Context) error {
	threats, err := h.service.BSI.ListBSIThreats(c.Request().Context())
	if err != nil {
		log.Error().Err(err).Msg("list bsi threats")
		return errResp(c, http.StatusInternalServerError, "failed to list threats", "CK_BSI_THREATS_FAILED")
	}
	if threats == nil {
		threats = []bsi.BSIThreat{}
	}
	return c.JSON(http.StatusOK, threats)
}

// ListBSIRisks handles GET /api/v1/vaktcomply/bsi/target-objects/:id/risks
func (h *Handler) ListBSIRisks(c echo.Context) error {
	id := c.Param("id")
	risks, err := h.service.BSI.ListBSIRisks(c.Request().Context(), orgID(c), id)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("list bsi risks")
		return errResp(c, http.StatusInternalServerError, "failed to list risks", "CK_BSI_RISK_LIST_FAILED")
	}
	if risks == nil {
		risks = []bsi.BSIRiskAssessment{}
	}
	return c.JSON(http.StatusOK, risks)
}

// CreateBSIRisk handles POST /api/v1/vaktcomply/bsi/target-objects/:id/risks
func (h *Handler) CreateBSIRisk(c echo.Context) error {
	id := c.Param("id")
	var in bsi.CreateBSIRiskInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	risk, err := h.service.BSI.CreateBSIRisk(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("create bsi risk")
		return errResp(c, http.StatusInternalServerError, "failed to create risk", "CK_BSI_RISK_CREATE_FAILED")
	}
	return c.JSON(http.StatusCreated, risk)
}

// UpdateBSIRisk handles PUT /api/v1/vaktcomply/bsi/target-objects/:id/risks/:riskId
func (h *Handler) UpdateBSIRisk(c echo.Context) error {
	id := c.Param("id")
	riskID := c.Param("riskId")
	var in bsi.UpdateBSIRiskInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	risk, err := h.service.BSI.UpdateBSIRisk(c.Request().Context(), orgID(c), id, riskID, in)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "risk not found", "CK_BSI_RISK_NOT_FOUND")
		}
		log.Error().Err(err).Str("id", id).Str("riskId", riskID).Msg("update bsi risk")
		return errResp(c, http.StatusInternalServerError, "failed to update risk", "CK_BSI_RISK_UPDATE_FAILED")
	}
	return c.JSON(http.StatusOK, risk)
}

// DeleteBSIRisk handles DELETE /api/v1/vaktcomply/bsi/target-objects/:id/risks/:riskId
func (h *Handler) DeleteBSIRisk(c echo.Context) error {
	id := c.Param("id")
	riskID := c.Param("riskId")
	if err := h.service.BSI.DeleteBSIRisk(c.Request().Context(), orgID(c), id, riskID); err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "risk not found", "CK_BSI_RISK_NOT_FOUND")
		}
		log.Error().Err(err).Str("id", id).Str("riskId", riskID).Msg("delete bsi risk")
		return errResp(c, http.StatusInternalServerError, "failed to delete risk", "CK_BSI_RISK_DELETE_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// GetBSIRiskSummary handles GET /api/v1/vaktcomply/bsi/target-objects/:id/risks/summary
func (h *Handler) GetBSIRiskSummary(c echo.Context) error {
	id := c.Param("id")
	summary, err := h.service.BSI.GetBSIRiskSummary(c.Request().Context(), orgID(c), id)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("get bsi risk summary")
		return errResp(c, http.StatusInternalServerError, "failed to get risk summary", "CK_BSI_RISK_SUMMARY_FAILED")
	}
	return c.JSON(http.StatusOK, summary)
}

func (h *Handler) ListBSITargetObjects(c echo.Context) error {
	objects, err := h.service.BSI.ListBSITargetObjects(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list bsi target objects")
		return errResp(c, http.StatusInternalServerError, "failed to list target objects", "CK_BSI_TO_LIST_FAILED")
	}
	if objects == nil {
		objects = []bsi.BSITargetObject{}
	}
	return c.JSON(http.StatusOK, objects)
}

// CreateBSITargetObject handles POST /api/v1/vaktcomply/bsi/target-objects
func (h *Handler) CreateBSITargetObject(c echo.Context) error {
	var in bsi.CreateBSITargetObjectInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	obj, err := h.service.BSI.CreateBSITargetObject(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create bsi target object")
		return errResp(c, http.StatusInternalServerError, "failed to create target object", "CK_BSI_TO_CREATE_FAILED")
	}
	return c.JSON(http.StatusCreated, obj)
}

// GetBSITargetObject handles GET /api/v1/vaktcomply/bsi/target-objects/:id
func (h *Handler) GetBSITargetObject(c echo.Context) error {
	id := c.Param("id")
	obj, err := h.service.BSI.GetBSITargetObject(c.Request().Context(), orgID(c), id)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "target object not found", "CK_BSI_TO_NOT_FOUND")
		}
		log.Error().Err(err).Str("id", id).Msg("get bsi target object")
		return errResp(c, http.StatusInternalServerError, "failed to get target object", "CK_BSI_TO_GET_FAILED")
	}
	return c.JSON(http.StatusOK, obj)
}

// UpdateBSITargetObject handles PUT /api/v1/vaktcomply/bsi/target-objects/:id
func (h *Handler) UpdateBSITargetObject(c echo.Context) error {
	id := c.Param("id")
	var in bsi.UpdateBSITargetObjectInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	obj, err := h.service.BSI.UpdateBSITargetObject(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "target object not found", "CK_BSI_TO_NOT_FOUND")
		}
		log.Error().Err(err).Str("id", id).Msg("update bsi target object")
		return errResp(c, http.StatusInternalServerError, "failed to update target object", "CK_BSI_TO_UPDATE_FAILED")
	}
	return c.JSON(http.StatusOK, obj)
}

// DeleteBSITargetObject handles DELETE /api/v1/vaktcomply/bsi/target-objects/:id
func (h *Handler) DeleteBSITargetObject(c echo.Context) error {
	id := c.Param("id")
	if err := h.service.BSI.DeleteBSITargetObject(c.Request().Context(), orgID(c), id); err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "target object not found", "CK_BSI_TO_NOT_FOUND")
		}
		log.Error().Err(err).Str("id", id).Msg("delete bsi target object")
		return errResp(c, http.StatusInternalServerError, "failed to delete target object", "CK_BSI_TO_DELETE_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// AssignBausteinToTargetObject handles POST /api/v1/vaktcomply/bsi/target-objects/:id/modeling
func (h *Handler) AssignBausteinToTargetObject(c echo.Context) error {
	id := c.Param("id")
	var in bsi.AssignBausteinInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	if err := h.service.BSI.AssignBausteinToTargetObject(c.Request().Context(), orgID(c), id, in.BausteinID); err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "target object not found", "CK_BSI_TO_NOT_FOUND")
		}
		log.Error().Err(err).Str("id", id).Msg("assign baustein")
		return errResp(c, http.StatusInternalServerError, "failed to assign baustein", "CK_BSI_ASSIGN_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// RemoveBausteinFromTargetObject handles DELETE /api/v1/vaktcomply/bsi/target-objects/:id/modeling/:bausteinId
func (h *Handler) RemoveBausteinFromTargetObject(c echo.Context) error {
	id := c.Param("id")
	bausteinID := c.Param("bausteinId")
	if err := h.service.BSI.RemoveBausteinFromTargetObject(c.Request().Context(), orgID(c), id, bausteinID); err != nil {
		log.Error().Err(err).Str("id", id).Msg("remove baustein")
		return errResp(c, http.StatusInternalServerError, "failed to remove baustein", "CK_BSI_REMOVE_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// GetBSICheckSheet handles GET /api/v1/vaktcomply/bsi/target-objects/:id/check
func (h *Handler) GetBSICheckSheet(c echo.Context) error {
	id := c.Param("id")
	results, err := h.service.BSI.GetCheckSheet(c.Request().Context(), orgID(c), id)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("get check sheet")
		return errResp(c, http.StatusInternalServerError, "failed to get check sheet", "CK_BSI_CHECK_FAILED")
	}
	if results == nil {
		results = []bsi.BSICheckResult{}
	}
	return c.JSON(http.StatusOK, results)
}

// SetBSICheckResult handles PUT /api/v1/vaktcomply/bsi/target-objects/:id/check/:anforderungId
func (h *Handler) SetBSICheckResult(c echo.Context) error {
	id := c.Param("id")
	anforderungID := c.Param("anforderungId")
	var in bsi.SetCheckResultInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	result, err := h.service.BSI.SetCheckResult(c.Request().Context(), orgID(c), id, anforderungID, in)
	if err != nil {
		if strings.Contains(err.Error(), "begruendung_required") {
			return errResp(c, http.StatusBadRequest, "Begründung ist bei Status 'entbehrlich' Pflicht", "BEGRUENDUNG_REQUIRED")
		}
		log.Error().Err(err).Str("id", id).Str("anforderung", anforderungID).Msg("set check result")
		return errResp(c, http.StatusInternalServerError, "failed to set check result", "CK_BSI_SET_RESULT_FAILED")
	}
	return c.JSON(http.StatusOK, result)
}

// BulkSetBSICheckResults handles POST /api/v1/vaktcomply/bsi/target-objects/:id/check/bulk
func (h *Handler) BulkSetBSICheckResults(c echo.Context) error {
	id := c.Param("id")
	var items []bsi.BulkCheckResultItem
	if err := c.Bind(&items); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.service.BSI.BulkSetCheckResults(c.Request().Context(), orgID(c), id, items); err != nil {
		if strings.Contains(err.Error(), "begruendung_required") {
			return errResp(c, http.StatusBadRequest, err.Error(), "BEGRUENDUNG_REQUIRED")
		}
		log.Error().Err(err).Str("id", id).Msg("bulk set check results")
		return errResp(c, http.StatusInternalServerError, "failed to bulk set check results", "CK_BSI_BULK_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// GetBSICheckSummary handles GET /api/v1/vaktcomply/bsi/target-objects/:id/check/summary
func (h *Handler) GetBSICheckSummary(c echo.Context) error {
	id := c.Param("id")
	summary, err := h.service.BSI.GetCheckSummary(c.Request().Context(), orgID(c), id)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("get check summary")
		return errResp(c, http.StatusInternalServerError, "failed to get check summary", "CK_BSI_SUMMARY_FAILED")
	}
	return c.JSON(http.StatusOK, summary)
}

// GetBSICockpit handles GET /api/v1/vaktcomply/bsi/cockpit
func (h *Handler) GetBSICockpit(c echo.Context) error {
	cockpit, err := h.service.BSI.GetBSICockpit(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("get bsi cockpit")
		return errResp(c, http.StatusInternalServerError, "failed to get BSI cockpit", "CK_BSI_COCKPIT_FAILED")
	}
	return c.JSON(http.StatusOK, cockpit)
}

// GetBSIGapReport handles GET /api/v1/vaktcomply/bsi/gap-report
func (h *Handler) GetBSIGapReport(c echo.Context) error {
	report, err := h.service.BSI.GetBSIGapReport(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("get bsi gap report")
		return errResp(c, http.StatusInternalServerError, "failed to get BSI gap report", "CK_BSI_GAP_FAILED")
	}

	if c.QueryParam("format") == "csv" {
		csv := buildGapReportCSV(report)
		c.Response().Header().Set("Content-Type", "text/csv; charset=utf-8")
		c.Response().Header().Set("Content-Disposition", `attachment; filename="bsi-gap-report.csv"`)
		return c.String(http.StatusOK, csv)
	}

	return c.JSON(http.StatusOK, report)
}

func buildGapReportCSV(r bsi.BSIGapReport) string {
	var b strings.Builder
	b.WriteString("baustein_id,anforderung_id,anforderung_titel,zielobjekt,umsetzungsstatus,verantwortlicher,umsetzungsdatum\n")
	for _, g := range r.Gaps {
		b.WriteString(csvEscape(g.BausteinID) + "," +
			csvEscape(g.AnforderungID) + "," +
			csvEscape(g.AnforderungTitle) + "," +
			csvEscape(g.Zielobjekt) + "," +
			csvEscape(g.Umsetzungsstatus) + "," +
			csvEscape(g.Verantwortlicher) + "," +
			csvEscape(g.Umsetzungsdatum) + "\n")
	}
	return b.String()
}

func csvEscape(s string) string {
	if strings.ContainsAny(s, `",`+"\n") {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return s
}
