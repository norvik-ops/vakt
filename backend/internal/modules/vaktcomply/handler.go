package vaktcomply

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/matharnica/vakt/internal/db"
	"github.com/matharnica/vakt/internal/modules/vaktcomply/policy"
	"github.com/matharnica/vakt/internal/shared/audit"
	"github.com/matharnica/vakt/internal/shared/pagination"
	"github.com/matharnica/vakt/internal/shared/platform/features"
	"github.com/rs/zerolog/log"
)

type Handler struct {
	service       *Service
	validate      *validator.Validate
	uploadDir     string
	db            *pgxpool.Pool
	q             *db.Queries
	paCfg         PolicyAcceptanceHandlerConfig
	evidenceFiles *EvidenceFileService
}

// NewHandler creates a new ComplyKit handler.
func NewHandler(service *Service) *Handler {
	return &Handler{
		service:  service,
		validate: validator.New(),
	}
}

// WithDB attaches a DB pool used for audit logging.
func (h *Handler) WithDB(dbPool *pgxpool.Pool) *Handler {
	h.db = dbPool
	h.q = db.New(dbPool)
	return h
}

// orgID extracts the authenticated organisation ID from the Echo context.
func orgID(c echo.Context) string {
	v, _ := c.Get("org_id").(string)
	return v
}

// userID extracts the authenticated user ID from the Echo context.
func userID(c echo.Context) string {
	v, _ := c.Get("user_id").(string)
	return v
}

// errResp returns a standardised JSON error response.
func errResp(c echo.Context, code int, msg, errCode string) error {
	return c.JSON(code, map[string]string{
		"error": msg,
		"code":  errCode,
	})
}

func (h *Handler) BulkUpdateControls(c echo.Context) error {
	var in BulkUpdateControlsInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	if err := h.service.BulkUpdateControlStatus(c.Request().Context(), orgID(c), in.IDs, in.Status); err != nil {
		log.Error().Err(err).Msg("bulk update controls")
		return errResp(c, http.StatusInternalServerError, "failed to bulk update controls", "CK_BULK_UPDATE_FAILED")
	}
	audit.Write(c.Request().Context(), h.db, audit.WriteEntry{
		OrgID:        orgID(c),
		UserID:       userID(c),
		Action:       "bulk_update",
		ResourceType: "vakt-comply/control",
		ResourceName: "bulk status update",
		IPAddress:    c.RealIP(),
	})
	return c.NoContent(http.StatusNoContent)
}

// GetControlByID handles GET /api/v1/vaktcomply/controls/:id.
func (h *Handler) GetControlByID(c echo.Context) error {
	ctrl, err := h.service.GetControl(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		return errResp(c, http.StatusNotFound, "control not found", "CK_CONTROL_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, ctrl)
}

// GetControlMappings handles GET /vaktcomply/controls/:id/mappings.
// Returns all cross-framework control mappings for the given control, resolved to org-specific UUIDs.
func (h *Handler) GetControlMappings(c echo.Context) error {
	mappings, err := h.service.GetControlMappings(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		log.Error().Err(err).Msg("get control mappings")
		return errResp(c, http.StatusInternalServerError, "failed to get control mappings", "CK_CONTROL_MAPPINGS_FAILED")
	}
	if mappings == nil {
		mappings = []ControlMapping{}
	}
	return c.JSON(http.StatusOK, map[string]any{"mappings": mappings})
}

// GetControlChangelog handles GET /vaktcomply/controls/:id/changelog.
// Returns the last 50 field-level change log entries for the given control.
func (h *Handler) GetControlChangelog(c echo.Context) error {
	entries, err := h.service.repo.ListControlChanges(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		log.Error().Err(err).Msg("get control changelog")
		return errResp(c, http.StatusInternalServerError, "failed to get control changelog", "CK_CHANGELOG_FAILED")
	}
	if entries == nil {
		entries = []ChangeLogEntry{}
	}
	return c.JSON(http.StatusOK, map[string]any{"changelog": entries})
}

// UpdateControl handles PATCH /api/v1/vaktcomply/controls/:id.
// Accepts not_applicable, reason, and manual_status fields.
func (h *Handler) UpdateControl(c echo.Context) error {
	var in UpdateControlInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}

	// Snapshot old values for changelog comparison.
	oldCtrl, _ := h.service.GetControl(c.Request().Context(), orgID(c), c.Param("id"))

	ctrl, err := h.service.UpdateControl(c.Request().Context(), orgID(c), c.Param("id"), in)
	if err != nil {
		if errors.Is(err, ErrInvalidMaturityScore) {
			return errResp(c, http.StatusUnprocessableEntity, err.Error(), "CK_VALIDATION_ERROR")
		}
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "control not found", "CK_CONTROL_NOT_FOUND")
		}
		log.Error().Err(err).Msg("update control")
		return errResp(c, http.StatusInternalServerError, "failed to update control", "CK_UPDATE_CONTROL_FAILED")
	}

	// Append changelog entries for each changed field.
	if oldCtrl != nil {
		uid := userID(c)
		uemail, _ := c.Get("user_email").(string)
		appendIfChanged := func(field, oldVal, newVal string) {
			if oldVal != newVal {
				h.service.repo.AppendControlChange(c.Request().Context(), orgID(c), ctrl.ID, uid, uemail, field, oldVal, newVal)
			}
		}
		oldNA := "false"
		newNA := "false"
		if oldCtrl.NotApplicable {
			oldNA = "true"
		}
		if ctrl.NotApplicable {
			newNA = "true"
		}
		appendIfChanged("not_applicable", oldNA, newNA)
		appendIfChanged("not_applicable_reason", oldCtrl.NotApplicableReason, ctrl.NotApplicableReason)
		appendIfChanged("manual_status", oldCtrl.ManualStatus, ctrl.ManualStatus)
		oldScore := strconv.Itoa(oldCtrl.MaturityScore)
		newScore := strconv.Itoa(ctrl.MaturityScore)
		appendIfChanged("maturity_score", oldScore, newScore)
	}

	audit.Write(c.Request().Context(), h.db, audit.WriteEntry{
		OrgID:        orgID(c),
		UserID:       userID(c),
		Action:       "update",
		ResourceType: "vakt-comply/control",
		ResourceID:   ctrl.ID,
		ResourceName: ctrl.Title,
		IPAddress:    c.RealIP(),
	})
	return c.JSON(http.StatusOK, ctrl)
}

// UpdateControlSoAMetadata handles PATCH /api/v1/vaktcomply/controls/:id/soa.
func (h *Handler) UpdateControlSoAMetadata(c echo.Context) error {
	var in UpdateSoAMetadataInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request", "CK_BAD_REQUEST")
	}
	if err := h.service.UpdateSoAMetadata(c.Request().Context(), orgID(c), c.Param("id"), in); err != nil {
		log.Error().Err(err).Str("control_id", c.Param("id")).Msg("update soa metadata")
		return errResp(c, http.StatusInternalServerError, "failed to update soa metadata", "CK_SOA_UPDATE_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// ListControlTasks handles GET /api/v1/vaktcomply/controls/:id/tasks.
func (h *Handler) ListControlTasks(c echo.Context) error {
	controlID := c.Param("id")
	ctx := c.Request().Context()
	tasks, err := h.service.ListControlTasks(ctx, orgID(c), controlID)
	if err != nil {
		return errResp(c, http.StatusInternalServerError, "failed to list tasks", "CK_LIST_TASKS_FAILED")
	}
	return c.JSON(http.StatusOK, tasks)
}

// CreateControlTask handles POST /api/v1/vaktcomply/controls/:id/tasks.
func (h *Handler) CreateControlTask(c echo.Context) error {
	controlID := c.Param("id")
	ctx := c.Request().Context()
	var in CreateControlTaskInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_INVALID_INPUT")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, "Ungültige Eingabe", "VALIDATION_ERROR")
	}
	task, err := h.service.CreateControlTask(ctx, orgID(c), controlID, in)
	if err != nil {
		return errResp(c, http.StatusInternalServerError, "failed to create task", "CK_CREATE_TASK_FAILED")
	}
	return c.JSON(http.StatusCreated, task)
}

// UpdateControlTask handles PATCH /api/v1/vaktcomply/controls/:id/tasks/:taskId.
func (h *Handler) UpdateControlTask(c echo.Context) error {
	controlID := c.Param("id")
	taskID := c.Param("taskId")
	ctx := c.Request().Context()
	var in UpdateControlTaskInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_INVALID_INPUT")
	}
	task, err := h.service.UpdateControlTask(ctx, orgID(c), controlID, taskID, in)
	if err != nil {
		return errResp(c, http.StatusInternalServerError, "failed to update task", "CK_UPDATE_TASK_FAILED")
	}
	return c.JSON(http.StatusOK, task)
}

// DeleteControlTask handles DELETE /api/v1/vaktcomply/controls/:id/tasks/:taskId.
func (h *Handler) DeleteControlTask(c echo.Context) error {
	controlID := c.Param("id")
	taskID := c.Param("taskId")
	ctx := c.Request().Context()
	if err := h.service.DeleteControlTask(ctx, orgID(c), controlID, taskID); err != nil {
		return errResp(c, http.StatusInternalServerError, "failed to delete task", "CK_DELETE_TASK_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// RecordControlReview handles POST /vaktcomply/controls/:id/review.
func (h *Handler) RecordControlReview(c echo.Context) error {
	controlID := c.Param("id")
	if controlID == "" {
		return errResp(c, http.StatusBadRequest, "control id is required", "CK_BAD_REQUEST")
	}
	var in RecordReviewInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	ctrl, err := h.service.RecordControlReview(c.Request().Context(), orgID(c), controlID, in)
	if err != nil {
		log.Error().Err(err).Str("control_id", controlID).Msg("record control review")
		return errResp(c, http.StatusInternalServerError, "failed to record review", "CK_RECORD_REVIEW_FAILED")
	}
	return c.JSON(http.StatusOK, ctrl)
}

// ListControlReviews handles GET /vaktcomply/controls/:id/reviews.
func (h *Handler) ListControlReviews(c echo.Context) error {
	controlID := c.Param("id")
	if controlID == "" {
		return errResp(c, http.StatusBadRequest, "control id is required", "CK_BAD_REQUEST")
	}
	reviews, err := h.service.ListControlReviews(c.Request().Context(), orgID(c), controlID)
	if err != nil {
		log.Error().Err(err).Str("control_id", controlID).Msg("list control reviews")
		return errResp(c, http.StatusInternalServerError, "failed to list reviews", "CK_LIST_REVIEWS_FAILED")
	}
	if reviews == nil {
		reviews = []ControlReview{}
	}
	return c.JSON(http.StatusOK, reviews)
}

// ListOverdueControls handles GET /vaktcomply/controls/overdue-reviews.
func (h *Handler) ListOverdueControls(c echo.Context) error {
	controls, err := h.service.ListOverdueControls(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list overdue controls")
		return errResp(c, http.StatusInternalServerError, "failed to list overdue controls", "CK_LIST_OVERDUE_FAILED")
	}
	if controls == nil {
		controls = []Control{}
	}
	return c.JSON(http.StatusOK, controls)
}

// GetScoreHistory handles GET /api/v1/vaktcomply/score-history?days=30
// Returns daily compliance score snapshots for the organisation.
func (h *Handler) GetScoreHistory(c echo.Context) error {
	days := 30
	if d := c.QueryParam("days"); d != "" {
		if n, err := strconv.Atoi(d); err == nil && n > 0 {
			days = n
		}
	}
	entries, err := h.service.GetScoreHistory(c.Request().Context(), orgID(c), days)
	if err != nil {
		log.Error().Err(err).Msg("get score history")
		return errResp(c, http.StatusInternalServerError, "failed to get score history", "CK_SCORE_HISTORY_FAILED")
	}
	if entries == nil {
		entries = []ScoreHistoryEntry{}
	}
	return c.JSON(http.StatusOK, entries)
}

// GetComplianceScore handles GET /api/v1/vaktcomply/compliance-score (S67-4).
// Returns the staleness-aware compliance score.
func (h *Handler) GetComplianceScore(c echo.Context) error {
	score, err := h.service.GetComplianceScore(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("get compliance score")
		return errResp(c, http.StatusInternalServerError, "failed to get compliance score", "CK_COMPLIANCE_SCORE_FAILED")
	}
	return c.JSON(http.StatusOK, score)
}

// ListStaleControls handles GET /api/v1/vaktcomply/controls/stale (S67-4).
// Returns controls with evidence_status = 'stale', sorted by expiry date.
func (h *Handler) ListStaleControls(c echo.Context) error {
	controls, err := h.service.ListStaleControls(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list stale controls")
		return errResp(c, http.StatusInternalServerError, "failed to list stale controls", "CK_STALE_CONTROLS_FAILED")
	}
	if controls == nil {
		controls = []Control{}
	}
	return c.JSON(http.StatusOK, controls)
}

func (h *Handler) ListFrameworks(c echo.Context) error {
	frameworks, err := h.service.ListFrameworks(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list frameworks")
		return errResp(c, http.StatusInternalServerError, "failed to list frameworks", "CK_LIST_FRAMEWORKS_FAILED")
	}
	return c.JSON(http.StatusOK, frameworks)
}

// enableFrameworkNamed wraps EnableFramework for the static, feature-gated
// enable routes (e.g. /frameworks/CRA/enable) which don't declare a :name
// path segment — so c.Param("name") would otherwise always be empty and
// every one of these frameworks would 400 with "framework name is required".
func (h *Handler) enableFrameworkNamed(name string) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.SetParamNames("name")
		c.SetParamValues(name)
		return h.EnableFramework(c)
	}
}

// frameworkFeatureGate mirrors the per-framework feature gates applied at
// the route level in routes.go (features.Require(...) on the static
// /frameworks/CRA/enable-style routes). It is re-checked here, keyed by the
// case-normalised name, so license enforcement doesn't depend on which route
// matched: Echo's router is case-sensitive, so a request for
// /frameworks/cra/enable (or any other casing) falls through the literal
// static routes and hits the generic, feature-gate-less
// /frameworks/:name/enable — without this check, that's a paywall bypass
// (any Admin/SecurityAnalyst on any license tier could enable any
// Pro/Enterprise framework just by varying casing in the URL).
var frameworkFeatureGate = map[string]features.Feature{
	"CRA":      features.FeatureCRA,
	"EUAIACT":  features.FeatureEUAIAct,
	"BSI":      features.FeatureBSIGrundschutz,
	"TISAX":    features.FeatureTISAX,
	"DORA":     features.FeatureDORA,
	"ISO42001": features.FeatureISO42001,
	"ISO27017": features.FeatureMultiFramework,
	"ISO27018": features.FeatureMultiFramework,
}

// EnableFramework handles POST /api/v1/vaktcomply/frameworks/:name/enable.
// Accepts optional body {"variant": "full"|"simplified"} for DORA Art. 16.
func (h *Handler) EnableFramework(c echo.Context) error {
	name := strings.ToUpper(c.Param("name"))
	if name == "" {
		return errResp(c, http.StatusBadRequest, "framework name is required", "CK_BAD_REQUEST")
	}
	if feature, gated := frameworkFeatureGate[name]; gated && !features.IsEnabled(c, feature) {
		return c.JSON(http.StatusPaymentRequired, map[string]string{
			"error":   "feature_not_available",
			"message": "This feature requires Vakt Pro. Visit https://vakt.norvikops.de for details.",
			"feature": feature,
		})
	}

	var input EnableFrameworkInput
	if c.Request().ContentLength != 0 {
		if err := c.Bind(&input); err != nil {
			return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
		}
		if err := h.validate.Struct(&input); err != nil {
			return errResp(c, http.StatusUnprocessableEntity, err.Error(), "CK_VALIDATION_FAILED")
		}
	}

	fw, err := h.service.EnableFramework(c.Request().Context(), orgID(c), name, input.Variant)
	if err != nil {
		if errors.Is(err, policy.ErrFrameworkDraft) {
			return errResp(c, http.StatusForbidden, "framework is in draft status and not yet available", "CK_FRAMEWORK_DRAFT")
		}
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

// SwitchDORAVariant handles PUT /api/v1/vaktcomply/frameworks/dora/variant.
func (h *Handler) SwitchDORAVariant(c echo.Context) error {
	var input SwitchDORAVariantInput
	if err := c.Bind(&input); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(&input); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, err.Error(), "CK_VALIDATION_FAILED")
	}

	// Find the org's DORA framework.
	fw, err := h.service.FindFrameworkByName(c.Request().Context(), orgID(c), "DORA")
	if err != nil || fw == nil {
		return errResp(c, http.StatusNotFound, "DORA framework not enabled for this organisation", "CK_DORA_NOT_FOUND")
	}

	updated, err := h.service.SwitchDORAVariant(c.Request().Context(), orgID(c), fw.ID, input.Variant)
	if err != nil {
		log.Error().Err(err).Str("variant", input.Variant).Msg("switch dora variant")
		return errResp(c, http.StatusInternalServerError, "failed to switch DORA variant", "CK_DORA_VARIANT_FAILED")
	}
	audit.Write(c.Request().Context(), h.db, audit.WriteEntry{
		OrgID: orgID(c), UserID: userID(c), Action: "update",
		ResourceType: "vakt-comply/framework", ResourceID: fw.ID, ResourceName: "DORA",
		IPAddress: c.RealIP(),
	})
	return c.JSON(http.StatusOK, updated)
}

// DeleteFramework handles DELETE /api/v1/vaktcomply/frameworks/:id.
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

// GetFrameworkByID handles GET /api/v1/vaktcomply/frameworks/:id.
func (h *Handler) GetFrameworkByID(c echo.Context) error {
	fw, err := h.service.GetFramework(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		return errResp(c, http.StatusNotFound, "framework not found", "CK_FRAMEWORK_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, fw)
}

// GetReadinessReport handles GET /api/v1/vaktcomply/frameworks/:id/report.
func (h *Handler) GetReadinessReport(c echo.Context) error {
	frameworkID := c.Param("id")
	report, err := h.service.GetReadinessReport(c.Request().Context(), orgID(c), frameworkID)
	if err != nil {
		log.Error().Err(err).Str("framework_id", frameworkID).Msg("get readiness report")
		return errResp(c, http.StatusInternalServerError, "failed to generate readiness report", "CK_READINESS_REPORT_FAILED")
	}
	return c.JSON(http.StatusOK, report)
}

// GetGapAnalysis handles GET /api/v1/vaktcomply/frameworks/:id/gaps.
func (h *Handler) GetGapAnalysis(c echo.Context) error {
	frameworkID := c.Param("id")
	analysis, err := h.service.GetGapAnalysis(c.Request().Context(), orgID(c), frameworkID)
	if err != nil {
		log.Error().Err(err).Str("framework_id", frameworkID).Msg("get gap analysis")
		return errResp(c, http.StatusInternalServerError, "failed to generate gap analysis", "CK_GAP_ANALYSIS_FAILED")
	}
	return c.JSON(http.StatusOK, analysis)
}

// ListControls handles GET /api/v1/vaktcomply/frameworks/:id/controls.
// Cursor mode (preferred): ?cursor=<opaque>&limit=25
// Offset mode (deprecated): ?page=1&limit=25 — sends Deprecation header
func (h *Handler) ListControls(c echo.Context) error {
	frameworkID := c.Param("id")
	scopeFilter := c.QueryParam("scope")

	if c.QueryParam("page") == "" {
		cp := pagination.CursorFromRequest(c)
		cursorControlID, cursorID := pagination.DecodeControlCursor(cp.Cursor)
		rows, err := h.service.ListControlsCursor(c.Request().Context(), orgID(c), frameworkID, cursorControlID, cursorID, cp.Limit)
		if err != nil {
			log.Error().Err(err).Str("framework_id", frameworkID).Msg("list controls cursor")
			return errResp(c, http.StatusInternalServerError, "failed to list controls", "CK_LIST_CONTROLS_FAILED")
		}
		enrichControlsWithNIS2Meta(rows)
		rows = filterControlsByScope(rows, scopeFilter)
		resp := pagination.WrapCursor(rows, cp, func(ctrl Control) string {
			return pagination.EncodeControlCursor(ctrl.ControlID, ctrl.ID)
		})
		return c.JSON(http.StatusOK, resp)
	}
	c.Response().Header().Set("Deprecation", "true")
	c.Response().Header().Set("Sunset", "2027-01-01")
	offset, limit, meta := pagination.FromRequest(c)
	controls, _, err := h.service.ListControlsPaged(c.Request().Context(), orgID(c), frameworkID, offset, limit)
	if err != nil {
		log.Error().Err(err).Str("framework_id", frameworkID).Msg("list controls")
		return errResp(c, http.StatusInternalServerError, "failed to list controls", "CK_LIST_CONTROLS_FAILED")
	}
	enrichControlsWithNIS2Meta(controls)
	controls = filterControlsByScope(controls, scopeFilter)
	pagination.Complete(&meta, len(controls))
	return c.JSON(http.StatusOK, pagination.Wrap(controls, meta))
}

// ListAvailableFrameworks handles GET /api/v1/vaktcomply/frameworks/available.
// Returns all frameworks (builtin + installed plugins) with their enabled status for this org.
func (h *Handler) ListAvailableFrameworks(c echo.Context) error {
	available, err := h.service.ListAvailableFrameworks(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list available frameworks")
		return errResp(c, http.StatusInternalServerError, "failed to list available frameworks", "CK_LIST_AVAILABLE_FAILED")
	}
	return c.JSON(http.StatusOK, available)
}

// InstallFrameworkPlugin handles POST /api/v1/vaktcomply/frameworks/install.
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

// ListFrameworkMappings handles GET /api/v1/vaktcomply/framework-mappings.
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

// DeleteFrameworkMapping handles DELETE /api/v1/vaktcomply/framework-mappings/:id.
func (h *Handler) DeleteFrameworkMapping(c echo.Context) error {
	mappingID := c.Param("id")
	if mappingID == "" {
		return errResp(c, http.StatusBadRequest, "mapping id is required", "CK_BAD_REQUEST")
	}
	if err := h.service.DeleteFrameworkMapping(c.Request().Context(), orgID(c), mappingID); err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "mapping not found", "CK_MAPPING_NOT_FOUND")
		}
		log.Error().Err(err).Str("mapping_id", mappingID).Msg("delete framework mapping")
		return errResp(c, http.StatusInternalServerError, "failed to delete framework mapping", "CK_DELETE_MAPPING_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// GetTISAXControls handles GET /api/v1/vaktcomply/frameworks/:id/tisax-controls.
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

// GetTISAXGapAnalysis handles GET /api/v1/vaktcomply/frameworks/:id/tisax-gaps.
func (h *Handler) GetTISAXGapAnalysis(c echo.Context) error {
	frameworkID := c.Param("id")
	analysis, err := h.service.GetTISAXGapAnalysis(c.Request().Context(), orgID(c), frameworkID)
	if err != nil {
		log.Error().Err(err).Str("framework_id", frameworkID).Msg("get tisax gap analysis")
		return errResp(c, http.StatusInternalServerError, "failed to generate TISAX gap analysis", "CK_TISAX_GAP_ANALYSIS_FAILED")
	}
	return c.JSON(http.StatusOK, analysis)
}

// GetTISAXISOMapping handles GET /api/v1/vaktcomply/frameworks/tisax/iso-mapping.
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

// GetTISAXCoverageAfterISO handles GET /api/v1/vaktcomply/frameworks/tisax/coverage-after-iso.
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

// GetDSGVOTOMCoverage handles GET /api/v1/vaktcomply/dsgvo/tom-coverage.
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
			return c.JSON(http.StatusOK, []MappingResult{})
		}
		frameworkID = fw.ID
	}
	results, err := h.service.GetDSGVOTOMCoverage(ctx, org, frameworkID)
	if err != nil {
		log.Error().Err(err).Str("framework_id", frameworkID).Msg("get dsgvo tom coverage")
		return echo.ErrInternalServerError
	}
	if results == nil {
		results = []MappingResult{}
	}
	return c.JSON(http.StatusOK, results)
}

// ExportSoAPDF handles GET /api/v1/vaktcomply/frameworks/:id/soa.pdf.
func (h *Handler) ExportSoAPDF(c echo.Context) error {
	pdfBytes, filename, err := h.service.ExportSoAPDF(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		log.Error().Err(err).Str("framework_id", c.Param("id")).Msg("export soa pdf")
		return errResp(c, http.StatusInternalServerError, "failed to generate soa pdf", "CK_SOA_PDF_FAILED")
	}
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	return c.Blob(http.StatusOK, "application/pdf", pdfBytes)
}

// ExportFrameworkPDF handles GET /api/v1/vaktcomply/frameworks/:id/export-pdf.
func (h *Handler) ExportFrameworkPDF(c echo.Context) error {
	pdfBytes, filename, err := h.service.ExportFrameworkPDF(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		log.Error().Err(err).Str("framework_id", c.Param("id")).Msg("export framework pdf")
		return errResp(c, http.StatusInternalServerError, "failed to generate pdf", "CK_PDF_EXPORT_FAILED")
	}
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	return c.Blob(http.StatusOK, "application/pdf", pdfBytes)
}

// ExportTISAXReportPDF handles GET /api/v1/vaktcomply/frameworks/:id/tisax-report-pdf.
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
		if errors.Is(err, ErrInvalidProtection) || errors.Is(err, ErrInvalidAssessment) {
			return errResp(c, http.StatusBadRequest, err.Error(), "CK_TISAX_PDF_BAD_PARAMS")
		}
		log.Error().Err(err).Str("framework_id", frameworkID).Msg("export tisax report pdf")
		return errResp(c, http.StatusInternalServerError, "failed to generate TISAX report PDF", "CK_TISAX_PDF_FAILED")
	}
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	return c.Blob(http.StatusOK, "application/pdf", pdfBytes)
}

// GetMappingCoverage handles GET /api/v1/vaktcomply/frameworks/mapping-coverage.
// Returns the cross-framework mapping coverage matrix for the org.
func (h *Handler) GetMappingCoverage(c echo.Context) error {
	resp, err := h.service.GetMappingCoverage(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("get mapping coverage")
		return errResp(c, http.StatusInternalServerError, "failed to get mapping coverage", "CK_MAPPING_COVERAGE_FAILED")
	}
	return c.JSON(http.StatusOK, resp)
}

// GetImplementationPath handles GET /api/v1/vaktcomply/frameworks/:id/implementation-path.
// Returns controls in topological order based on prerequisite chains.
func (h *Handler) GetImplementationPath(c echo.Context) error {
	steps, err := h.service.GetImplementationPath(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		log.Error().Err(err).Str("framework_id", c.Param("id")).Msg("get implementation path")
		return errResp(c, http.StatusInternalServerError, "failed to get implementation path", "CK_IMPL_PATH_FAILED")
	}
	return c.JSON(http.StatusOK, steps)
}
