package secpulse

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/sechealth-app/sechealth/internal/shared/pagination"
)

// Handler handles HTTP requests for VulnBoard.
type Handler struct {
	service  *Service
	validate *validator.Validate
}

// NewHandler creates a new VulnBoard handler.
func NewHandler(service *Service) *Handler {
	return &Handler{
		service:  service,
		validate: validator.New(),
	}
}

// CreateAsset handles POST /api/v1/secpulse/assets.
func (h *Handler) CreateAsset(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	userID, _ := c.Get("user_id").(string)

	var input CreateAssetInput
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
			"code":  "VB_BAD_REQUEST",
		})
	}
	if err := h.validate.Struct(input); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": "Ungültige Eingabe",
			"code":  "VALIDATION_ERROR",
		})
	}

	asset, err := h.service.CreateAsset(c.Request().Context(), orgID, userID, input)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("create asset failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to create asset",
			"code":  "VB_CREATE_ASSET_ERROR",
		})
	}
	return c.JSON(http.StatusCreated, asset)
}

// ListAssets handles GET /api/v1/secpulse/assets.
func (h *Handler) ListAssets(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	_, limit, meta := pagination.FromRequest(c)
	// The existing service uses page number rather than offset.
	page := meta.Page
	tag := c.QueryParam("tag")

	assets, total, err := h.service.ListAssets(c.Request().Context(), orgID, page, limit, tag)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("list assets failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to list assets",
			"code":  "VB_LIST_ASSETS_ERROR",
		})
	}
	pagination.Complete(&meta, total)
	return c.JSON(http.StatusOK, pagination.Wrap(assets, meta))
}

// GetAsset handles GET /api/v1/secpulse/assets/:id.
func (h *Handler) GetAsset(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	assetID := c.Param("id")

	asset, err := h.service.GetAsset(c.Request().Context(), orgID, assetID)
	if err != nil {
		log.Debug().Err(err).Str("asset_id", assetID).Msg("get asset failed")
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "asset not found",
			"code":  "VB_ASSET_NOT_FOUND",
		})
	}
	return c.JSON(http.StatusOK, asset)
}

// UpdateAsset handles PUT /api/v1/secpulse/assets/:id.
func (h *Handler) UpdateAsset(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	assetID := c.Param("id")

	var input UpdateAssetInput
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
			"code":  "VB_BAD_REQUEST",
		})
	}
	if err := h.validate.Struct(input); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": "Ungültige Eingabe",
			"code":  "VALIDATION_ERROR",
		})
	}

	asset, err := h.service.UpdateAsset(c.Request().Context(), orgID, assetID, input)
	if err != nil {
		log.Error().Err(err).Str("asset_id", assetID).Msg("update asset failed")
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "asset not found or update failed",
			"code":  "VB_ASSET_NOT_FOUND",
		})
	}
	return c.JSON(http.StatusOK, asset)
}

// DeleteAsset handles DELETE /api/v1/secpulse/assets/:id.
func (h *Handler) DeleteAsset(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	assetID := c.Param("id")

	if err := h.service.DeleteAsset(c.Request().Context(), orgID, assetID); err != nil {
		log.Debug().Err(err).Str("asset_id", assetID).Msg("delete asset failed")
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "asset not found",
			"code":  "VB_ASSET_NOT_FOUND",
		})
	}
	return c.NoContent(http.StatusNoContent)
}

// GetSLADashboard handles GET /api/v1/secpulse/sla-dashboard.
func (h *Handler) GetSLADashboard(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)

	entries, err := h.service.GetSLADashboard(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("get sla dashboard failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to retrieve SLA dashboard",
			"code":  "VB_SLA_ERROR",
		})
	}
	if entries == nil {
		entries = []SLAEntry{}
	}
	return c.JSON(http.StatusOK, entries)
}

// GetSLAConfig handles GET /api/v1/secpulse/sla-config.
func (h *Handler) GetSLAConfig(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)

	cfg, err := h.service.GetSLAConfig(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("get sla config failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to retrieve SLA configuration",
			"code":  "VB_SLA_ERROR",
		})
	}
	return c.JSON(http.StatusOK, cfg)
}

// UpdateSLAConfig handles PUT /api/v1/secpulse/sla-config.
func (h *Handler) UpdateSLAConfig(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)

	var input SLAConfig
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
			"code":  "VB_BAD_REQUEST",
		})
	}

	if err := h.service.UpdateSLAConfig(c.Request().Context(), orgID, input); err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("update sla config failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to update SLA configuration",
			"code":  "VB_SLA_ERROR",
		})
	}
	return c.JSON(http.StatusOK, map[string]string{
		"message": "SLA configuration updated",
	})
}

// ImportAssets handles POST /api/v1/secpulse/assets/import (multipart/form-data, field "file").
func (h *Handler) ImportAssets(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	userID, _ := c.Get("user_id").(string)

	file, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "field 'file' is required",
			"code":  "VB_BAD_REQUEST",
		})
	}

	src, err := file.Open()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to open uploaded file",
			"code":  "VB_IMPORT_ERROR",
		})
	}
	defer src.Close()

	inserted, errored, errs, err := h.service.ImportAssetsCSV(c.Request().Context(), orgID, userID, src)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("CSV import failed")
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
			"code":  "VB_IMPORT_PARSE_ERROR",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"inserted": inserted,
		"errored":  errored,
		"errors":   errs,
	})
}

// TriggerScan handles POST /api/v1/secpulse/assets/:id/scans
func (h *Handler) TriggerScan(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	assetID := c.Param("id")

	var input CreateScanInput
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body", "code": "VB_BAD_REQUEST"})
	}
	if err := h.validate.Struct(input); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}

	scan, err := h.service.TriggerScan(c.Request().Context(), orgID, assetID, input)
	if err != nil {
		log.Error().Err(err).Str("asset_id", assetID).Msg("trigger scan failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to trigger scan", "code": "VB_SCAN_ERROR"})
	}
	return c.JSON(http.StatusAccepted, scan)
}

// GetScan handles GET /api/v1/secpulse/scans/:id
func (h *Handler) GetScan(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	scan, err := h.service.GetScan(c.Request().Context(), orgID, c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "scan not found", "code": "VB_SCAN_NOT_FOUND"})
	}
	return c.JSON(http.StatusOK, scan)
}

// ListFindings handles GET /api/v1/secpulse/findings
func (h *Handler) ListFindings(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	_, limit, meta := pagination.FromRequest(c)
	filter := FindingFilter{
		Severity: c.QueryParam("severity"),
		Status:   c.QueryParam("status"),
		AssetID:  c.QueryParam("asset_id"),
		SortBy:   c.QueryParam("sort"),
		Order:    c.QueryParam("order"),
		Page:     meta.Page,
		Limit:    limit,
	}

	findings, err := h.service.ListFindings(c.Request().Context(), orgID, filter)
	if err != nil {
		log.Error().Err(err).Msg("list findings failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list findings", "code": "VB_LIST_FINDINGS_ERROR"})
	}
	total, err := h.service.CountFindings(c.Request().Context(), orgID, filter)
	if err != nil {
		log.Error().Err(err).Msg("count findings failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to count findings", "code": "VB_LIST_FINDINGS_ERROR"})
	}
	pagination.Complete(&meta, total)
	return c.JSON(http.StatusOK, pagination.Wrap(findings, meta))
}

// GetFinding handles GET /api/v1/secpulse/findings/:id
func (h *Handler) GetFinding(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	finding, err := h.service.GetFinding(c.Request().Context(), orgID, c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "finding not found", "code": "VB_FINDING_NOT_FOUND"})
	}
	return c.JSON(http.StatusOK, finding)
}

// UpdateFinding handles PATCH /api/v1/secpulse/findings/:id
func (h *Handler) UpdateFinding(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	var input UpdateFindingInput
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body", "code": "VB_BAD_REQUEST"})
	}
	finding, err := h.service.UpdateFinding(c.Request().Context(), orgID, c.Param("id"), input)
	if err != nil {
		log.Error().Err(err).Msg("update finding failed")
		if err.Error() == "justification is required when setting status to accepted_risk" {
			return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": err.Error(), "code": "VB_VALIDATION_ERROR"})
		}
		return c.JSON(http.StatusNotFound, map[string]string{"error": "finding not found", "code": "VB_FINDING_NOT_FOUND"})
	}
	return c.JSON(http.StatusOK, finding)
}

// BulkUpdateFindings handles POST /api/v1/secpulse/findings/bulk
func (h *Handler) BulkUpdateFindings(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	var input BulkFindingInput
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body", "code": "VB_BAD_REQUEST"})
	}
	if err := h.validate.Struct(input); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	n, err := h.service.BulkUpdateFindings(c.Request().Context(), orgID, input)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "bulk update failed", "code": "VB_BULK_UPDATE_ERROR"})
	}
	return c.JSON(http.StatusOK, map[string]int{"updated": n})
}

// ListSuppressions handles GET /api/v1/secpulse/suppressions
func (h *Handler) ListSuppressions(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	rules, err := h.service.ListSuppressionRules(c.Request().Context(), orgID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list suppressions", "code": "VB_ERROR"})
	}
	if rules == nil {
		rules = []SuppressionRule{}
	}
	return c.JSON(http.StatusOK, rules)
}

// CreateSuppression handles POST /api/v1/secpulse/suppressions
func (h *Handler) CreateSuppression(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	userID, _ := c.Get("user_id").(string)
	var input CreateSuppressionInput
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body", "code": "VB_BAD_REQUEST"})
	}
	if err := h.validate.Struct(input); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	rule, err := h.service.CreateSuppressionRule(c.Request().Context(), orgID, userID, input)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create suppression", "code": "VB_ERROR"})
	}
	return c.JSON(http.StatusCreated, rule)
}

// DeleteSuppression handles DELETE /api/v1/secpulse/suppressions/:id
func (h *Handler) DeleteSuppression(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	if err := h.service.DeleteSuppressionRule(c.Request().Context(), orgID, c.Param("id")); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "suppression not found", "code": "VB_NOT_FOUND"})
	}
	return c.NoContent(http.StatusNoContent)
}

// ListScanSchedules handles GET /api/v1/secpulse/assets/:id/schedules
func (h *Handler) ListScanSchedules(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	schedules, err := h.service.ListScanSchedules(c.Request().Context(), orgID, c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list schedules", "code": "VB_ERROR"})
	}
	if schedules == nil {
		schedules = []ScanSchedule{}
	}
	return c.JSON(http.StatusOK, schedules)
}

// CreateScanSchedule handles POST /api/v1/secpulse/assets/:id/schedules
func (h *Handler) CreateScanSchedule(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	var input CreateScanScheduleInput
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body", "code": "VB_BAD_REQUEST"})
	}
	if err := h.validate.Struct(input); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	schedule, err := h.service.CreateScanSchedule(c.Request().Context(), orgID, c.Param("id"), input)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create schedule", "code": "VB_ERROR"})
	}
	return c.JSON(http.StatusCreated, schedule)
}

// DeleteScanSchedule handles DELETE /api/v1/secpulse/assets/:id/schedules/:schedule_id
func (h *Handler) DeleteScanSchedule(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	if err := h.service.DeleteScanSchedule(c.Request().Context(), orgID, c.Param("schedule_id")); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "schedule not found", "code": "VB_NOT_FOUND"})
	}
	return c.NoContent(http.StatusNoContent)
}

// GetRiskTrend handles GET /api/v1/secpulse/reports/risk-trend
func (h *Handler) GetRiskTrend(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	days := 90
	if d, err := strconv.Atoi(c.QueryParam("days")); err == nil && d > 0 {
		days = d
	}
	trend, err := h.service.GetRiskTrend(c.Request().Context(), orgID, days)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to get risk trend", "code": "VB_ERROR"})
	}
	if trend == nil {
		trend = []RiskTrendPoint{}
	}
	return c.JSON(http.StatusOK, trend)
}

// GenerateReport handles POST /api/v1/secpulse/reports
func (h *Handler) GenerateReport(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	userID, _ := c.Get("user_id").(string)
	var body struct {
		Title string `json:"title"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body", "code": "VB_BAD_REQUEST"})
	}
	scope := map[string]interface{}{"title": body.Title}
	report, err := h.service.GenerateReport(c.Request().Context(), orgID, userID, scope)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to generate report", "code": "VB_ERROR"})
	}
	return c.JSON(http.StatusAccepted, report)
}

// DownloadReport handles GET /api/v1/secpulse/reports/:id/download
func (h *Handler) DownloadReport(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	reportID := c.Param("id")
	content, title, err := h.service.GetReportContent(c.Request().Context(), orgID, reportID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "report not found or not ready"})
	}
	filename := fmt.Sprintf("sechealth-report-%s.pdf", reportID[:8])
	if title != "" {
		// sanitise title for filename
		safe := strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
				return r
			}
			return '-'
		}, title)
		filename = safe + ".pdf"
	}
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))
	return c.Blob(http.StatusOK, "application/pdf", content)
}

// ListReports handles GET /api/v1/secpulse/reports
func (h *Handler) ListReports(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	reports, err := h.service.ListReports(c.Request().Context(), orgID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list reports", "code": "VB_ERROR"})
	}
	if reports == nil {
		reports = []Report{}
	}
	return c.JSON(http.StatusOK, reports)
}

// GetReport handles GET /api/v1/secpulse/reports/:id
func (h *Handler) GetReport(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	report, err := h.service.GetReport(c.Request().Context(), orgID, c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "report not found", "code": "VB_NOT_FOUND"})
	}
	return c.JSON(http.StatusOK, report)
}

// TriggerSBOMScan handles POST /api/v1/secpulse/assets/:id/sbom.
// Enqueues a Syft SBOM generation job for the given asset.
func (h *Handler) TriggerSBOMScan(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	assetID := c.Param("id")

	if err := h.service.TriggerSBOMScan(c.Request().Context(), orgID, assetID); err != nil {
		log.Error().Err(err).Str("asset_id", assetID).Msg("trigger SBOM scan failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to trigger SBOM scan",
			"code":  "VB_SBOM_ERROR",
		})
	}
	return c.JSON(http.StatusAccepted, map[string]string{
		"message": "SBOM scan enqueued",
	})
}

// GetAssetSBOM handles GET /api/v1/secpulse/assets/:id/sbom.
// Returns the latest SBOM summary and its components for the given asset.
func (h *Handler) GetAssetSBOM(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	assetID := c.Param("id")

	sbom, components, err := h.service.GetAssetSBOM(c.Request().Context(), orgID, assetID)
	if err != nil {
		log.Debug().Err(err).Str("asset_id", assetID).Msg("get asset SBOM failed")
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "no SBOM found for this asset",
			"code":  "VB_SBOM_NOT_FOUND",
		})
	}
	if components == nil {
		components = []ComponentSummary{}
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"sbom":       sbom,
		"components": components,
	})
}

// GetEOLDashboard handles GET /api/v1/secpulse/sbom/eol.
// Returns components across the org with their EOL status (paginated, up to 500 per page).
// Query params: eol_only=true, page=1 (1-based).
func (h *Handler) GetEOLDashboard(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	eolOnly := c.QueryParam("eol_only") == "true"
	page, _ := strconv.Atoi(c.QueryParam("page"))
	if page < 1 {
		page = 1
	}

	components, err := h.service.GetEOLDashboard(c.Request().Context(), orgID, eolOnly, page)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("get EOL dashboard failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to retrieve EOL dashboard",
			"code":  "VB_EOL_ERROR",
		})
	}
	if components == nil {
		components = []ComponentSummary{}
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": components,
	})
}

// ExportFindings handles GET /api/v1/secpulse/findings/export
func (h *Handler) ExportFindings(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	format := c.QueryParam("format")
	if format == "" {
		format = "json"
	}
	filter := FindingFilter{
		Severity: c.QueryParam("severity"),
		Status:   c.QueryParam("status"),
	}

	reader, err := h.service.ExportFindings(c.Request().Context(), orgID, format, filter)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "export failed", "code": "VB_EXPORT_ERROR"})
	}

	if format == "csv" {
		c.Response().Header().Set("Content-Type", "text/csv")
		c.Response().Header().Set("Content-Disposition", `attachment; filename="findings.csv"`)
	} else {
		c.Response().Header().Set("Content-Type", "application/json")
	}
	c.Response().WriteHeader(http.StatusOK)
	io.Copy(c.Response().Writer, reader) //nolint:errcheck
	return nil
}
