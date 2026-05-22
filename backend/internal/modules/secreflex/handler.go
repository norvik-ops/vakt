package secreflex

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/shared/audit"
	"github.com/matharnica/vakt/internal/shared/pagination"
)

// Handler handles HTTP requests for PhishGuard.
type Handler struct {
	service  *Service
	validate *validator.Validate
}

// NewHandler creates a new PhishGuard handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service, validate: validator.New()}
}

func errJSON(c echo.Context, code int, msg, errCode string) error {
	return c.JSON(code, map[string]string{"error": msg, "code": errCode})
}

func (h *Handler) audit(c echo.Context, action, resourceType, resourceID, resourceName string) {
	orgID, _ := c.Get("org_id").(string)
	userID, _ := c.Get("user_id").(string)
	audit.Write(c.Request().Context(), h.service.db, audit.WriteEntry{
		OrgID:        orgID,
		UserID:       userID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		ResourceName: resourceName,
		IPAddress:    c.RealIP(),
	})
}

// ── Templates ─────────────────────────────────────────────────────────────────

func (h *Handler) ListTemplates(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	items, err := h.service.ListTemplates(c.Request().Context(), orgID)
	if err != nil {
		return errJSON(c, http.StatusInternalServerError, "failed to list templates", "PG_ERROR")
	}
	if items == nil {
		items = []Template{}
	}
	return c.JSON(http.StatusOK, items)
}

func (h *Handler) ListPresets(c echo.Context) error {
	return c.JSON(http.StatusOK, h.service.GetPresetTemplates())
}

func (h *Handler) ListTrainingPresets(c echo.Context) error {
	return c.JSON(http.StatusOK, h.service.GetPresetTrainingModules())
}

func (h *Handler) CreateTemplate(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	userID, _ := c.Get("user_id").(string)
	var input CreateTemplateInput
	if err := c.Bind(&input); err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid body", "PG_BAD_REQUEST")
	}
	if err := h.validate.Struct(input); err != nil {
		return errJSON(c, http.StatusUnprocessableEntity, "Ungültige Eingabe", "VALIDATION_ERROR")
	}
	t, err := h.service.CreateTemplate(c.Request().Context(), orgID, userID, input)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("create template failed")
		return errJSON(c, http.StatusBadRequest, "Vorlage konnte nicht erstellt werden", "PG_TEMPLATE_ERROR")
	}
	h.audit(c, "create", "secreflex/template", t.ID, t.Name)
	return c.JSON(http.StatusCreated, t)
}

// ── Target groups ─────────────────────────────────────────────────────────────

func (h *Handler) ListTargetGroups(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	items, err := h.service.ListTargetGroups(c.Request().Context(), orgID)
	if err != nil {
		return errJSON(c, http.StatusInternalServerError, "failed to list groups", "PG_ERROR")
	}
	if items == nil {
		items = []TargetGroup{}
	}
	return c.JSON(http.StatusOK, items)
}

func (h *Handler) CreateTargetGroup(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	var body struct {
		Name   string `json:"name"   validate:"required"`
		Source string `json:"source"`
	}
	if err := c.Bind(&body); err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid body", "PG_BAD_REQUEST")
	}
	if err := h.validate.Struct(body); err != nil {
		return errJSON(c, http.StatusUnprocessableEntity, "Ungültige Eingabe", "VALIDATION_ERROR")
	}
	if body.Source == "" {
		body.Source = "manual"
	}
	g, err := h.service.CreateTargetGroup(c.Request().Context(), orgID, body.Name, body.Source)
	if err != nil {
		return errJSON(c, http.StatusInternalServerError, "failed to create group", "PG_ERROR")
	}
	h.audit(c, "create", "secreflex/target-group", g.ID, g.Name)
	return c.JSON(http.StatusCreated, g)
}

func (h *Handler) ListTargets(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	items, err := h.service.ListTargets(c.Request().Context(), orgID, c.Param("id"))
	if err != nil {
		return errJSON(c, http.StatusInternalServerError, "failed to list targets", "PG_ERROR")
	}
	if items == nil {
		items = []Target{}
	}
	return c.JSON(http.StatusOK, items)
}

func (h *Handler) ImportTargetsCSV(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	var body struct {
		CSVContent string `json:"csv_content" validate:"required"`
	}
	if err := c.Bind(&body); err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid body", "PG_BAD_REQUEST")
	}
	imported, errs := h.service.ImportTargetsCSV(c.Request().Context(), orgID, c.Param("id"), body.CSVContent)
	return c.JSON(http.StatusOK, map[string]any{"imported": imported, "errors": errs})
}

// ── Landing pages ─────────────────────────────────────────────────────────────

func (h *Handler) ListLandingPages(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	items, err := h.service.ListLandingPages(c.Request().Context(), orgID)
	if err != nil {
		return errJSON(c, http.StatusInternalServerError, "failed to list landing pages", "PG_ERROR")
	}
	if items == nil {
		items = []LandingPage{}
	}
	return c.JSON(http.StatusOK, items)
}

func (h *Handler) CreateLandingPage(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	var body struct {
		Name string `json:"name"         validate:"required"`
		HTML string `json:"html_content" validate:"required"`
	}
	if err := c.Bind(&body); err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid body", "PG_BAD_REQUEST")
	}
	if err := h.validate.Struct(body); err != nil {
		return errJSON(c, http.StatusUnprocessableEntity, "Ungültige Eingabe", "VALIDATION_ERROR")
	}
	lp, err := h.service.CreateLandingPage(c.Request().Context(), orgID, body.Name, body.HTML)
	if err != nil {
		return errJSON(c, http.StatusInternalServerError, "failed to create landing page", "PG_ERROR")
	}
	h.audit(c, "create", "secreflex/landing-page", lp.ID, lp.Name)
	return c.JSON(http.StatusCreated, lp)
}

// ── Campaigns ─────────────────────────────────────────────────────────────────

func (h *Handler) ListCampaigns(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	if c.QueryParam("page") == "" {
		cp := pagination.CursorFromRequest(c)
		cursorID, cursorTS := pagination.DecodeCursor(cp.Cursor)
		rows, err := h.service.ListCampaignsCursor(c.Request().Context(), orgID, cursorID, cursorTS, cp.Limit)
		if err != nil {
			log.Error().Err(err).Msg("list campaigns cursor")
			return errJSON(c, http.StatusInternalServerError, "failed to list campaigns", "PG_ERROR")
		}
		resp := pagination.WrapCursor(rows, cp, func(cam Campaign) string {
			return pagination.EncodeCursor(cam.ID, cam.CreatedAt)
		})
		return c.JSON(http.StatusOK, resp)
	}
	c.Response().Header().Set("Deprecation", "true")
	c.Response().Header().Set("Sunset", "2027-01-01")
	items, err := h.service.ListCampaigns(c.Request().Context(), orgID)
	if err != nil {
		return errJSON(c, http.StatusInternalServerError, "failed to list campaigns", "PG_ERROR")
	}
	if items == nil {
		items = []Campaign{}
	}
	return c.JSON(http.StatusOK, items)
}

func (h *Handler) CreateCampaign(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	userID, _ := c.Get("user_id").(string)
	var input CreateCampaignInput
	if err := c.Bind(&input); err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid body", "PG_BAD_REQUEST")
	}
	if err := h.validate.Struct(input); err != nil {
		return errJSON(c, http.StatusUnprocessableEntity, "Ungültige Eingabe", "VALIDATION_ERROR")
	}
	campaign, err := h.service.CreateCampaign(c.Request().Context(), orgID, userID, input)
	if err != nil {
		return errJSON(c, http.StatusInternalServerError, "failed to create campaign", "PG_ERROR")
	}
	h.audit(c, "create", "secreflex/campaign", campaign.ID, campaign.Name)
	return c.JSON(http.StatusCreated, campaign)
}

func (h *Handler) GetCampaign(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	campaign, err := h.service.GetCampaign(c.Request().Context(), orgID, c.Param("id"))
	if err != nil {
		return errJSON(c, http.StatusNotFound, "campaign not found", "PG_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, campaign)
}

func (h *Handler) LaunchCampaign(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	campaignID := c.Param("id")
	if err := h.service.LaunchCampaign(c.Request().Context(), orgID, campaignID); err != nil {
		log.Error().Err(err).Str("org_id", orgID).Str("campaign_id", campaignID).Msg("launch campaign failed")
		return errJSON(c, http.StatusBadRequest, "Kampagne konnte nicht gestartet werden", "PG_LAUNCH_ERROR")
	}
	h.audit(c, "action", "secreflex/campaign", campaignID, "launch")
	return c.JSON(http.StatusOK, map[string]string{"status": "running"})
}

func (h *Handler) AbortCampaign(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	campaignID := c.Param("id")
	if err := h.service.AbortCampaign(c.Request().Context(), orgID, campaignID); err != nil {
		return errJSON(c, http.StatusNotFound, "campaign not found", "PG_NOT_FOUND")
	}
	h.audit(c, "action", "secreflex/campaign", campaignID, "abort")
	return c.JSON(http.StatusOK, map[string]string{"status": "aborted"})
}

func (h *Handler) GetCampaignStats(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	stats, err := h.service.GetCampaignStats(c.Request().Context(), orgID, c.Param("id"))
	if err != nil {
		return errJSON(c, http.StatusInternalServerError, "failed to get stats", "PG_ERROR")
	}
	return c.JSON(http.StatusOK, stats)
}

func (h *Handler) ExportCampaignReport(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	pdfBytes, filename, err := h.service.ExportCampaignReport(c.Request().Context(), orgID, c.Param("id"))
	if err != nil {
		log.Error().Err(err).Str("campaign_id", c.Param("id")).Msg("export campaign report")
		return errJSON(c, http.StatusInternalServerError, "failed to generate report", "PG_REPORT_ERROR")
	}
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))
	return c.Blob(http.StatusOK, "application/pdf", pdfBytes)
}

// ── Tracking (public endpoints) ───────────────────────────────────────────────

func (h *Handler) TrackClick(c echo.Context) error {
	html, err := h.service.RecordEvent(c.Request().Context(), c.Param("token"), "click", c.RealIP(), c.Request().UserAgent())
	if err != nil {
		return c.String(http.StatusNotFound, "Invalid link")
	}
	return c.HTML(http.StatusOK, html)
}

func (h *Handler) TrackFormSubmission(c echo.Context) error {
	_, err := h.service.RecordEvent(c.Request().Context(), c.Param("token"), "form_submission", c.RealIP(), c.Request().UserAgent())
	if err != nil {
		log.Warn().Err(err).Msg("form submission tracking failed")
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "recorded"})
}

// TrackOpen handles the open-tracking pixel request embedded in campaign emails.
// It records an "open" event and returns a 1×1 transparent GIF so that email
// clients that load the image trigger the event silently.
func (h *Handler) TrackOpen(c echo.Context) error {
	// Fire-and-forget: record the open event but never fail the response.
	h.service.RecordOpen(c.Request().Context(), c.Param("token"), c.RealIP(), c.Request().UserAgent())

	// 1×1 transparent GIF (43 bytes, GIF89a spec).
	c.Response().Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	c.Response().Header().Set("Pragma", "no-cache")
	return c.Blob(http.StatusOK, "image/gif", transparentGIF)
}

// transparentGIF is a minimal 1×1 transparent GIF89a image (43 bytes).
var transparentGIF = []byte{
	0x47, 0x49, 0x46, 0x38, 0x39, 0x61, // GIF89a header
	0x01, 0x00, 0x01, 0x00, 0x80, 0x00, 0x00, // 1×1, GCT flag
	0xFF, 0xFF, 0xFF, 0x00, 0x00, 0x00, // palette: white, black
	0x21, 0xF9, 0x04, 0x01, 0x00, 0x00, 0x01, 0x00, // graphic control (transparent idx 1)
	0x2C, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, // image descriptor
	0x02, 0x02, 0x4C, 0x01, 0x00, // LZW min code 2, compressed pixel, EOI
	0x3B, // GIF trailer
}

// ── Training modules ──────────────────────────────────────────────────────────

func (h *Handler) ListModules(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	items, err := h.service.ListModules(c.Request().Context(), orgID)
	if err != nil {
		return errJSON(c, http.StatusInternalServerError, "failed to list modules", "PG_ERROR")
	}
	if items == nil {
		items = []TrainingModule{}
	}
	return c.JSON(http.StatusOK, items)
}

func (h *Handler) CreateModule(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	userID, _ := c.Get("user_id").(string)
	var input CreateModuleInput
	if err := c.Bind(&input); err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid body", "PG_BAD_REQUEST")
	}
	if err := h.validate.Struct(input); err != nil {
		return errJSON(c, http.StatusUnprocessableEntity, "Ungültige Eingabe", "VALIDATION_ERROR")
	}
	m, err := h.service.CreateModule(c.Request().Context(), orgID, userID, input)
	if err != nil {
		return errJSON(c, http.StatusInternalServerError, "failed to create module", "PG_ERROR")
	}
	h.audit(c, "create", "secreflex/training-module", m.ID, m.Title)
	return c.JSON(http.StatusCreated, m)
}

// ── Assignments ───────────────────────────────────────────────────────────────

func (h *Handler) ListAssignments(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	items, err := h.service.ListAssignments(c.Request().Context(), orgID, c.QueryParam("status"))
	if err != nil {
		return errJSON(c, http.StatusInternalServerError, "failed to list assignments", "PG_ERROR")
	}
	if items == nil {
		items = []Assignment{}
	}
	return c.JSON(http.StatusOK, items)
}

func (h *Handler) CompleteAssignment(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	var input CompleteAssignmentInput
	_ = c.Bind(&input)
	completion, err := h.service.CompleteAssignment(c.Request().Context(), orgID, c.Param("id"), input)
	if err != nil {
		return errJSON(c, http.StatusNotFound, "assignment not found", "PG_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, completion)
}

func (h *Handler) GetAssignmentCertificate(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	id := c.Param("id")
	pdfBytes, filename, err := h.service.GetAssignmentCertificate(c.Request().Context(), orgID, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errJSON(c, http.StatusNotFound, "certificate not found", "SR_NOT_FOUND")
		}
		log.Error().Err(err).Str("assignment_id", id).Msg("generate certificate failed")
		return errJSON(c, http.StatusInternalServerError, "failed to generate certificate", "SR_CERT_FAILED")
	}
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))
	return c.Blob(http.StatusOK, "application/pdf", pdfBytes)
}

// ── Phish-Button (Feature 5) ──────────────────────────────────────────────────

// ReceivePhishReport is a public endpoint (no Bearer auth) that accepts phishing
// reports from the Outlook/Gmail add-in. The request is authenticated via the
// org_token field in the body, which is matched against organizations.phish_report_token.
func (h *Handler) ReceivePhishReport(c echo.Context) error {
	var input PhishReportWebhookInput
	if err := c.Bind(&input); err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid body", "PG_BAD_REQUEST")
	}
	if err := h.validate.Struct(input); err != nil {
		return errJSON(c, http.StatusUnprocessableEntity, "Ungültige Eingabe", "VALIDATION_ERROR")
	}
	report, err := h.service.RecordPhishReport(c.Request().Context(), input)
	if err != nil {
		log.Warn().Err(err).Msg("phish report rejected")
		// Return 401 for bad token, generic error for anything else
		if err.Error() == "invalid org token" {
			return errJSON(c, http.StatusUnauthorized, "invalid org token", "PG_UNAUTHORIZED")
		}
		return errJSON(c, http.StatusInternalServerError, "failed to record report", "PG_ERROR")
	}
	return c.JSON(http.StatusOK, map[string]any{
		"status":        "recorded",
		"is_simulation": report.IsSimulation,
	})
}

// ListPhishReports returns all phishing reports for the authenticated org.
func (h *Handler) ListPhishReports(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	items, err := h.service.ListPhishReports(c.Request().Context(), orgID)
	if err != nil {
		return errJSON(c, http.StatusInternalServerError, "failed to list reports", "PG_ERROR")
	}
	if items == nil {
		items = []PhishReport{}
	}
	return c.JSON(http.StatusOK, items)
}

// GetPhishReportStats returns aggregate phishing report stats for the authenticated org.
func (h *Handler) GetPhishReportStats(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	stats, err := h.service.GetPhishReportStats(c.Request().Context(), orgID)
	if err != nil {
		return errJSON(c, http.StatusInternalServerError, "failed to get stats", "PG_ERROR")
	}
	return c.JSON(http.StatusOK, stats)
}

// RegeneratePhishToken regenerates the org's phish_report_token and returns the new value.
func (h *Handler) RegeneratePhishToken(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	token, err := h.service.RegeneratePhishToken(c.Request().Context(), orgID)
	if err != nil {
		return errJSON(c, http.StatusInternalServerError, "failed to regenerate token", "PG_ERROR")
	}
	return c.JSON(http.StatusOK, map[string]string{"token": token})
}
