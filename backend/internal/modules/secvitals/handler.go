package secvitals

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/sechealth-app/sechealth/internal/shared/auditlog"
	"github.com/sechealth-app/sechealth/internal/shared/pagination"
)

// Handler handles HTTP requests for ComplyKit.
type Handler struct {
	service       *Service
	validate      *validator.Validate
	uploadDir     string
	db            *pgxpool.Pool
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
func (h *Handler) WithDB(db *pgxpool.Pool) *Handler {
	h.db = db
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

// GetControlByID handles GET /api/v1/secvitals/controls/:id.
func (h *Handler) GetControlByID(c echo.Context) error {
	ctrl, err := h.service.GetControl(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		return errResp(c, http.StatusNotFound, "control not found", "CK_CONTROL_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, ctrl)
}

// GetControlMappings handles GET /secvitals/controls/:id/mappings.
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
	return c.JSON(http.StatusOK, map[string]interface{}{"mappings": mappings})
}

// UpdateControl handles PATCH /api/v1/secvitals/controls/:id.
// Accepts not_applicable, reason, and manual_status fields.
func (h *Handler) UpdateControl(c echo.Context) error {
	var in UpdateControlInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	ctrl, err := h.service.UpdateControl(c.Request().Context(), orgID(c), c.Param("id"), in)
	if err != nil {
		if strings.Contains(err.Error(), "maturity_score must be between") {
			return errResp(c, http.StatusUnprocessableEntity, err.Error(), "CK_VALIDATION_ERROR")
		}
		log.Error().Err(err).Msg("update control")
		return errResp(c, http.StatusInternalServerError, "failed to update control", "CK_UPDATE_CONTROL_FAILED")
	}
	auditlog.Log(c.Request().Context(), h.db, auditlog.Entry{
		OrgID:        orgID(c),
		UserID:       userID(c),
		Action:       "update",
		ResourceType: "control",
		ResourceID:   ctrl.ID,
		ResourceName: ctrl.Title,
		IPAddress:    c.RealIP(),
	})
	return c.JSON(http.StatusOK, ctrl)
}

// UploadEvidence handles POST /api/v1/secvitals/controls/:id/evidence/upload.
// Accepts multipart/form-data with fields: file (required), title (required), notes (optional).
func (h *Handler) UploadEvidence(c echo.Context) error {
	controlID := c.Param("id")

	title := c.FormValue("title")
	if title == "" {
		return errResp(c, http.StatusBadRequest, "title is required", "CK_BAD_REQUEST")
	}
	notes := c.FormValue("notes")

	fh, err := c.FormFile("file")
	if err != nil {
		return errResp(c, http.StatusBadRequest, "file is required", "CK_BAD_REQUEST")
	}

	src, err := fh.Open()
	if err != nil {
		return errResp(c, http.StatusInternalServerError, "failed to open uploaded file", "CK_UPLOAD_FAILED")
	}
	defer src.Close()

	uploadDir := h.uploadDir
	if uploadDir == "" {
		uploadDir = "./data/uploads"
	}
	orgDir := filepath.Join(uploadDir, orgID(c), "evidence")
	if err := os.MkdirAll(orgDir, 0o750); err != nil {
		return errResp(c, http.StatusInternalServerError, "failed to create upload directory", "CK_UPLOAD_FAILED")
	}

	ext := filepath.Ext(fh.Filename)
	destName := uuid.New().String() + ext
	destPath := filepath.Join(orgDir, destName)

	dst, err := os.Create(destPath)
	if err != nil {
		return errResp(c, http.StatusInternalServerError, "failed to save file", "CK_UPLOAD_FAILED")
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return errResp(c, http.StatusInternalServerError, "failed to write file", "CK_UPLOAD_FAILED")
	}

	input := AddEvidenceInput{
		Title:       title,
		Description: notes,
		Source:      "manual",
		FilePath:    destPath,
		FileSize:    fh.Size,
	}
	ev, err := h.service.AddEvidence(c.Request().Context(), orgID(c), controlID, userID(c), input)
	if err != nil {
		_ = os.Remove(destPath)
		log.Error().Err(err).Str("control_id", controlID).Msg("upload evidence")
		return errResp(c, http.StatusInternalServerError, "failed to add evidence", "CK_ADD_EVIDENCE_FAILED")
	}
	return c.JSON(http.StatusCreated, ev)
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

// AddEvidence handles POST /api/v1/secvitals/controls/:id/evidence.
func (h *Handler) AddEvidence(c echo.Context) error {
	controlID := c.Param("id")
	var input AddEvidenceInput
	if err := c.Bind(&input); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(input); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": "Ungültige Eingabe",
			"code":  "VALIDATION_ERROR",
		})
	}

	ev, err := h.service.AddEvidence(c.Request().Context(), orgID(c), controlID, userID(c), input)
	if err != nil {
		log.Error().Err(err).Str("control_id", controlID).Msg("add evidence")
		return errResp(c, http.StatusInternalServerError, "failed to add evidence", "CK_ADD_EVIDENCE_FAILED")
	}
	return c.JSON(http.StatusCreated, ev)
}

// ListEvidence handles GET /api/v1/secvitals/controls/:id/evidence.
func (h *Handler) ListEvidence(c echo.Context) error {
	controlID := c.Param("id")
	items, err := h.service.ListEvidence(c.Request().Context(), orgID(c), controlID)
	if err != nil {
		log.Error().Err(err).Str("control_id", controlID).Msg("list evidence")
		return errResp(c, http.StatusInternalServerError, "failed to list evidence", "CK_LIST_EVIDENCE_FAILED")
	}
	return c.JSON(http.StatusOK, items)
}

// ReviewEvidence handles POST /api/v1/secvitals/evidence/:id/review.
func (h *Handler) ReviewEvidence(c echo.Context) error {
	evidenceID := c.Param("id")
	var body struct {
		Status string `json:"status" validate:"required,oneof=approved rejected"`
		Notes  string `json:"notes"`
	}
	if err := c.Bind(&body); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(body); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": "Ungültige Eingabe",
			"code":  "VALIDATION_ERROR",
		})
	}

	if err := h.service.ReviewEvidence(c.Request().Context(), orgID(c), evidenceID, userID(c), body.Status, body.Notes); err != nil {
		log.Error().Err(err).Str("evidence_id", evidenceID).Msg("review evidence")
		return errResp(c, http.StatusInternalServerError, "failed to review evidence", "CK_REVIEW_EVIDENCE_FAILED")
	}
	return c.JSON(http.StatusOK, map[string]string{"status": body.Status})
}

// CreateAuditorLink handles POST /api/v1/secvitals/frameworks/:id/auditor-link.
func (h *Handler) CreateAuditorLink(c echo.Context) error {
	frameworkID := c.Param("id")
	var body struct {
		ExpiresInHours int `json:"expires_in_hours" validate:"required,min=1,max=8760"`
	}
	if err := c.Bind(&body); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(body); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": "Ungültige Eingabe",
			"code":  "VALIDATION_ERROR",
		})
	}

	expiresIn := time.Duration(body.ExpiresInHours) * time.Hour
	rawToken, err := h.service.CreateAuditorLink(c.Request().Context(), orgID(c), frameworkID, userID(c), expiresIn)
	if err != nil {
		log.Error().Err(err).Str("framework_id", frameworkID).Msg("create auditor link")
		return errResp(c, http.StatusInternalServerError, "failed to create auditor link", "CK_AUDITOR_LINK_FAILED")
	}

	return c.JSON(http.StatusCreated, map[string]string{
		"auditor_url": "/api/v1/secvitals/auditor/" + rawToken,
	})
}

// AuditorView handles GET /api/v1/secvitals/auditor/:token — no auth required.
func (h *Handler) AuditorView(c echo.Context) error {
	rawToken := c.Param("token")
	fw, err := h.service.ValidateAuditorLink(c.Request().Context(), rawToken)
	if err != nil {
		log.Debug().Err(err).Msg("auditor link validation failed")
		return errResp(c, http.StatusNotFound, "invalid or expired auditor link", "CK_AUDITOR_LINK_INVALID")
	}

	// Return a read-only framework view — report without org-internal details.
	report, err := h.service.GetReadinessReport(c.Request().Context(), fw.OrgID, fw.ID)
	if err != nil {
		log.Error().Err(err).Str("framework_id", fw.ID).Msg("auditor view: readiness report")
		return errResp(c, http.StatusInternalServerError, "failed to generate report", "CK_AUDITOR_REPORT_FAILED")
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"framework": fw,
		"report":    report,
	})
}

// ListAuditorLinks handles GET /api/v1/secvitals/auditor-links.
func (h *Handler) ListAuditorLinks(c echo.Context) error {
	links, err := h.service.ListAuditorLinks(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list auditor links")
		return errResp(c, http.StatusInternalServerError, "failed to list auditor links", "CK_LIST_AUDITOR_LINKS_FAILED")
	}
	return c.JSON(http.StatusOK, links)
}

// RevokeAuditorLink handles DELETE /api/v1/secvitals/auditor-links/:id.
// Returns HTTP 410 Gone if the link is already revoked.
func (h *Handler) RevokeAuditorLink(c echo.Context) error {
	linkID := c.Param("id")
	if linkID == "" {
		return errResp(c, http.StatusBadRequest, "link id is required", "CK_BAD_REQUEST")
	}

	if err := h.service.RevokeAuditorLink(c.Request().Context(), orgID(c), linkID); err != nil {
		log.Error().Err(err).Str("link_id", linkID).Msg("revoke auditor link")
		return errResp(c, http.StatusGone, "auditor link not found or already revoked", "CK_AUDITOR_LINK_GONE")
	}
	return c.NoContent(http.StatusNoContent)
}

// AuditorExportBundle handles GET /api/v1/secvitals/auditor/:token/export — no auth required.
// Returns a ZIP archive of all framework controls with their evidence metadata.
func (h *Handler) AuditorExportBundle(c echo.Context) error {
	rawToken := c.Param("token")
	ctx := c.Request().Context()

	// ExportAuditorBundle validates the token, writes the ZIP to the writer,
	// and returns the framework name for the Content-Disposition header.
	// We must set headers before writing, so we buffer via ExportAuditorBundle
	// which streams directly; we pre-set headers and let it write.
	//
	// To keep headers accurate we do a lightweight token check first.
	fwName, err := h.service.PreflightAuditorExport(ctx, rawToken)
	if err != nil {
		log.Debug().Err(err).Msg("auditor export bundle: preflight failed")
		return errResp(c, http.StatusNotFound, "invalid or expired auditor link", "CK_AUDITOR_LINK_INVALID")
	}

	filename := fmt.Sprintf("%s-evidence-bundle.zip", fwName)
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Response().Header().Set("Content-Type", "application/zip")
	c.Response().WriteHeader(http.StatusOK)

	if _, err := h.service.ExportAuditorBundle(ctx, rawToken, c.Response().Writer); err != nil {
		log.Error().Err(err).Msg("auditor export bundle: write failed")
		// Headers already sent; cannot return JSON — log only.
		return nil
	}
	return nil
}

// --- Risk Assessment (FR-CK12) ---

// GetRisk handles GET /api/v1/secvitals/risks/:id.
func (h *Handler) GetRisk(c echo.Context) error {
	id := c.Param("id")
	risk, err := h.service.GetRisk(c.Request().Context(), orgID(c), id)
	if err != nil {
		return errResp(c, http.StatusNotFound, "risk not found", "CK_RISK_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, risk)
}

// UpdateRisk handles PATCH /api/v1/secvitals/risks/:id.
func (h *Handler) UpdateRisk(c echo.Context) error {
	id := c.Param("id")
	var in UpdateRiskInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	risk, err := h.service.UpdateRisk(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		log.Error().Err(err).Msg("update risk")
		return errResp(c, http.StatusInternalServerError, "failed to update risk", "CK_UPDATE_RISK_FAILED")
	}
	return c.JSON(http.StatusOK, risk)
}

// UpdateRiskTreatment handles PATCH /api/v1/secvitals/risks/:id/treatment.
// Patches only the ISO 27001 Clause 6 treatment workflow fields.
func (h *Handler) UpdateRiskTreatment(c echo.Context) error {
	id := c.Param("id")
	var in UpdateRiskTreatmentInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	risk, err := h.service.UpdateRiskTreatment(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		log.Error().Err(err).Msg("update risk treatment")
		return errResp(c, http.StatusInternalServerError, "failed to update risk treatment", "CK_UPDATE_RISK_TREATMENT_FAILED")
	}
	return c.JSON(http.StatusOK, risk)
}

// ListRisks handles GET /api/v1/secvitals/risks.
func (h *Handler) ListRisks(c echo.Context) error {
	offset, limit, meta := pagination.FromRequest(c)
	risks, total, err := h.service.ListRisksPaged(c.Request().Context(), orgID(c), offset, limit)
	if err != nil {
		log.Error().Err(err).Msg("list risks")
		return errResp(c, http.StatusInternalServerError, "failed to list risks", "CK_LIST_RISKS_FAILED")
	}
	pagination.Complete(&meta, total)
	return c.JSON(http.StatusOK, pagination.Wrap(risks, meta))
}

// CreateRisk handles POST /api/v1/secvitals/risks.
func (h *Handler) CreateRisk(c echo.Context) error {
	var in CreateRiskInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	risk, err := h.service.CreateRisk(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create risk")
		return errResp(c, http.StatusInternalServerError, "failed to create risk", "CK_CREATE_RISK_FAILED")
	}
	auditlog.Log(c.Request().Context(), h.db, auditlog.Entry{
		OrgID:        orgID(c),
		UserID:       userID(c),
		Action:       "create",
		ResourceType: "risk",
		ResourceID:   risk.ID,
		ResourceName: risk.Title,
		IPAddress:    c.RealIP(),
	})
	return c.JSON(http.StatusCreated, risk)
}

// --- Risk ↔ Control Links ---

// ListRiskControls handles GET /api/v1/secvitals/risks/:id/controls.
func (h *Handler) ListRiskControls(c echo.Context) error {
	id := c.Param("id")
	controls, err := h.service.ListRiskControls(c.Request().Context(), orgID(c), id)
	if err != nil {
		log.Error().Err(err).Msg("list risk controls")
		return errResp(c, http.StatusInternalServerError, "failed to list risk controls", "CK_LIST_RISK_CONTROLS_FAILED")
	}
	return c.JSON(http.StatusOK, controls)
}

// LinkRiskControl handles POST /api/v1/secvitals/risks/:id/controls.
// Body: {"control_id": "<uuid>"}
func (h *Handler) LinkRiskControl(c echo.Context) error {
	id := c.Param("id")
	var body struct {
		ControlID string `json:"control_id" validate:"required,uuid"`
	}
	if err := c.Bind(&body); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(body); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	if err := h.service.LinkRiskControl(c.Request().Context(), orgID(c), id, body.ControlID); err != nil {
		log.Error().Err(err).Msg("link risk control")
		return errResp(c, http.StatusInternalServerError, "failed to link control", "CK_LINK_RISK_CONTROL_FAILED")
	}
	return c.JSON(http.StatusCreated, map[string]string{"status": "linked"})
}

// UnlinkRiskControl handles DELETE /api/v1/secvitals/risks/:id/controls/:controlId.
func (h *Handler) UnlinkRiskControl(c echo.Context) error {
	riskID := c.Param("id")
	controlID := c.Param("controlId")
	if err := h.service.UnlinkRiskControl(c.Request().Context(), orgID(c), riskID, controlID); err != nil {
		log.Error().Err(err).Msg("unlink risk control")
		return errResp(c, http.StatusNotFound, "link not found", "CK_RISK_CONTROL_NOT_FOUND")
	}
	return c.NoContent(http.StatusNoContent)
}

// --- Incident Register (FR-CK13) ---

// GetIncident handles GET /api/v1/secvitals/incidents/:id.
func (h *Handler) GetIncident(c echo.Context) error {
	id := c.Param("id")
	inc, err := h.service.GetIncident(c.Request().Context(), orgID(c), id)
	if err != nil {
		return errResp(c, http.StatusNotFound, "incident not found", "CK_INCIDENT_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, inc)
}

// UpdateIncident handles PATCH /api/v1/secvitals/incidents/:id.
func (h *Handler) UpdateIncident(c echo.Context) error {
	id := c.Param("id")
	var in UpdateIncidentInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	inc, err := h.service.UpdateIncident(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		log.Error().Err(err).Msg("update incident")
		return errResp(c, http.StatusInternalServerError, "failed to update incident", "CK_UPDATE_INCIDENT_FAILED")
	}
	return c.JSON(http.StatusOK, inc)
}

// ListIncidents handles GET /api/v1/secvitals/incidents.
func (h *Handler) ListIncidents(c echo.Context) error {
	offset, limit, meta := pagination.FromRequest(c)
	incidents, total, err := h.service.ListIncidentsPaged(c.Request().Context(), orgID(c), offset, limit)
	if err != nil {
		log.Error().Err(err).Msg("list incidents")
		return errResp(c, http.StatusInternalServerError, "failed to list incidents", "CK_LIST_INCIDENTS_FAILED")
	}
	pagination.Complete(&meta, total)
	return c.JSON(http.StatusOK, pagination.Wrap(incidents, meta))
}

// CreateIncident handles POST /api/v1/secvitals/incidents.
func (h *Handler) CreateIncident(c echo.Context) error {
	var in CreateIncidentInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	incident, err := h.service.CreateIncident(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create incident")
		return errResp(c, http.StatusInternalServerError, "failed to create incident", "CK_CREATE_INCIDENT_FAILED")
	}
	auditlog.Log(c.Request().Context(), h.db, auditlog.Entry{
		OrgID:        orgID(c),
		UserID:       userID(c),
		Action:       "create",
		ResourceType: "incident",
		ResourceID:   incident.ID,
		ResourceName: incident.Title,
		IPAddress:    c.RealIP(),
	})
	return c.JSON(http.StatusCreated, incident)
}

// --- Policy Management (FR-CK14) ---

// GetPolicy handles GET /api/v1/secvitals/policies/:id.
func (h *Handler) GetPolicy(c echo.Context) error {
	id := c.Param("id")
	policy, err := h.service.GetPolicy(c.Request().Context(), orgID(c), id)
	if err != nil {
		return errResp(c, http.StatusNotFound, "policy not found", "CK_POLICY_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, policy)
}

// UpdatePolicy handles PATCH /api/v1/secvitals/policies/:id.
func (h *Handler) UpdatePolicy(c echo.Context) error {
	id := c.Param("id")
	var in UpdatePolicyInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	policy, err := h.service.UpdatePolicy(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		log.Error().Err(err).Msg("update policy")
		return errResp(c, http.StatusInternalServerError, "failed to update policy", "CK_UPDATE_POLICY_FAILED")
	}
	return c.JSON(http.StatusOK, policy)
}

// ListPolicies handles GET /api/v1/secvitals/policies.
func (h *Handler) ListPolicies(c echo.Context) error {
	offset, limit, meta := pagination.FromRequest(c)
	policies, total, err := h.service.ListPoliciesPaged(c.Request().Context(), orgID(c), offset, limit)
	if err != nil {
		log.Error().Err(err).Msg("list policies")
		return errResp(c, http.StatusInternalServerError, "failed to list policies", "CK_LIST_POLICIES_FAILED")
	}
	pagination.Complete(&meta, total)
	return c.JSON(http.StatusOK, pagination.Wrap(policies, meta))
}

// CreatePolicy handles POST /api/v1/secvitals/policies.
func (h *Handler) CreatePolicy(c echo.Context) error {
	var in CreatePolicyInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	policy, err := h.service.CreatePolicy(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create policy")
		return errResp(c, http.StatusInternalServerError, "failed to create policy", "CK_CREATE_POLICY_FAILED")
	}
	auditlog.Log(c.Request().Context(), h.db, auditlog.Entry{
		OrgID:        orgID(c),
		UserID:       userID(c),
		Action:       "create",
		ResourceType: "policy",
		ResourceID:   policy.ID,
		ResourceName: policy.Title,
		IPAddress:    c.RealIP(),
	})
	return c.JSON(http.StatusCreated, policy)
}

// ListPolicyVersions handles GET /api/v1/secvitals/policies/:id/versions.
// Returns all historical version snapshots for a policy, newest first.
func (h *Handler) ListPolicyVersions(c echo.Context) error {
	policyID := c.Param("id")
	versions, err := h.service.ListPolicyVersions(c.Request().Context(), orgID(c), policyID)
	if err != nil {
		log.Error().Err(err).Str("policy_id", policyID).Msg("list policy versions")
		return errResp(c, http.StatusInternalServerError, "failed to list policy versions", "CK_LIST_POLICY_VERSIONS_FAILED")
	}
	return c.JSON(http.StatusOK, versions)
}

// GetPolicyVersion handles GET /api/v1/secvitals/policies/:id/versions/:v.
// Returns a single historical version snapshot by version number.
func (h *Handler) GetPolicyVersion(c echo.Context) error {
	policyID := c.Param("id")
	vStr := c.Param("v")
	vNum, err := strconv.Atoi(vStr)
	if err != nil || vNum < 1 {
		return errResp(c, http.StatusBadRequest, "invalid version number", "CK_BAD_REQUEST")
	}
	pv, err := h.service.GetPolicyVersion(c.Request().Context(), orgID(c), policyID, vNum)
	if err != nil {
		return errResp(c, http.StatusNotFound, "policy version not found", "CK_POLICY_VERSION_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, pv)
}

// --- Internal Audit Records (FR-CK15) ---

// GetAuditRecord handles GET /api/v1/secvitals/audits/:id.
func (h *Handler) GetAuditRecord(c echo.Context) error {
	id := c.Param("id")
	record, err := h.service.GetAuditRecord(c.Request().Context(), orgID(c), id)
	if err != nil {
		return errResp(c, http.StatusNotFound, "audit record not found", "CK_AUDIT_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, record)
}

// UpdateAuditRecord handles PATCH /api/v1/secvitals/audits/:id.
func (h *Handler) UpdateAuditRecord(c echo.Context) error {
	id := c.Param("id")
	var in UpdateAuditRecordInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	record, err := h.service.UpdateAuditRecord(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		log.Error().Err(err).Msg("update audit record")
		return errResp(c, http.StatusInternalServerError, "failed to update audit record", "CK_UPDATE_AUDIT_FAILED")
	}
	return c.JSON(http.StatusOK, record)
}

// ListAuditRecords handles GET /api/v1/secvitals/audits.
func (h *Handler) ListAuditRecords(c echo.Context) error {
	records, err := h.service.ListAuditRecords(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list audit records")
		return errResp(c, http.StatusInternalServerError, "failed to list audit records", "CK_LIST_AUDITS_FAILED")
	}
	return c.JSON(http.StatusOK, records)
}

// CreateAuditRecord handles POST /api/v1/secvitals/audits.
func (h *Handler) CreateAuditRecord(c echo.Context) error {
	var in CreateAuditRecordInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	record, err := h.service.CreateAuditRecord(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create audit record")
		return errResp(c, http.StatusInternalServerError, "failed to create audit record", "CK_CREATE_AUDIT_FAILED")
	}
	return c.JSON(http.StatusCreated, record)
}

// CollectEvidence handles POST /api/v1/secvitals/controls/:id/collect.
func (h *Handler) CollectEvidence(c echo.Context) error {
	controlID := c.Param("id")
	var body struct {
		Type   string            `json:"type"   validate:"required,oneof=github aws azure ad"`
		Params map[string]string `json:"params"`
	}
	if err := c.Bind(&body); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(body); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": "Ungültige Eingabe",
			"code":  "VALIDATION_ERROR",
		})
	}

	cfg := CollectorConfig{Type: body.Type, Params: body.Params}
	ev, err := h.service.CollectEvidence(c.Request().Context(), orgID(c), controlID, userID(c), cfg)
	if err != nil {
		log.Error().Err(err).Str("control_id", controlID).Str("type", body.Type).Msg("collect evidence")
		return errResp(c, http.StatusInternalServerError, "evidence collection failed", "CK_COLLECT_FAILED")
	}
	return c.JSON(http.StatusCreated, ev)
}

// ExportEvidenceBundle handles GET /api/v1/secvitals/controls/:id/export.
// Returns a ZIP archive containing the control metadata and all evidence items as JSON.
func (h *Handler) ExportEvidenceBundle(c echo.Context) error {
	controlID := c.Param("id")
	ctx := c.Request().Context()
	org := orgID(c)

	ctrl, err := h.service.GetControl(ctx, org, controlID)
	if err != nil {
		return errResp(c, http.StatusNotFound, "control not found", "CK_CONTROL_NOT_FOUND")
	}

	items, err := h.service.ListEvidence(ctx, org, controlID)
	if err != nil {
		return errResp(c, http.StatusInternalServerError, "failed to list evidence", "CK_LIST_EVIDENCE_FAILED")
	}

	filename := fmt.Sprintf("evidence-%s.zip", ctrl.ControlID)
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Response().Header().Set("Content-Type", "application/zip")
	c.Response().WriteHeader(http.StatusOK)

	w := zip.NewWriter(c.Response().Writer)
	defer w.Close()

	// Write control metadata.
	ctrlFile, err := w.Create("control.json")
	if err != nil {
		return err
	}
	if err := json.NewEncoder(ctrlFile).Encode(ctrl); err != nil {
		return err
	}

	// Write evidence index.
	evidenceFile, err := w.Create("evidence.json")
	if err != nil {
		return err
	}
	if err := json.NewEncoder(evidenceFile).Encode(items); err != nil {
		return err
	}

	return nil
}

// --- Policy Templates ---

// ListPolicyTemplates handles GET /api/v1/secvitals/policy-templates.
func (h *Handler) ListPolicyTemplates(c echo.Context) error {
	return c.JSON(http.StatusOK, BuiltinPolicyTemplates())
}

// CreatePolicyFromTemplate handles POST /api/v1/secvitals/policy-templates/:id/apply.
// Creates a new policy using the template content as description.
func (h *Handler) CreatePolicyFromTemplate(c echo.Context) error {
	orgID := orgID(c)
	if orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	templateID := c.Param("id")
	templates := BuiltinPolicyTemplates()
	var found *PolicyTemplate
	for i := range templates {
		if templates[i].ID == templateID {
			found = &templates[i]
			break
		}
	}
	if found == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "template not found"})
	}

	in := CreatePolicyInput{
		Title:       found.Title,
		Category:    found.Category,
		Description: found.Content,
		Version:     "1.0",
	}
	policy, err := h.service.CreatePolicy(c.Request().Context(), orgID, in)
	if err != nil {
		log.Error().Err(err).Msg("CreatePolicyFromTemplate: create policy failed")
		return errResp(c, http.StatusInternalServerError, "failed to create policy from template", "CK_CREATE_POLICY_FAILED")
	}
	return c.JSON(http.StatusCreated, policy)
}

// --- Control Tasks ---

// ListControlTasks handles GET /api/v1/secvitals/controls/:id/tasks.
func (h *Handler) ListControlTasks(c echo.Context) error {
	controlID := c.Param("id")
	ctx := c.Request().Context()
	tasks, err := h.service.ListControlTasks(ctx, orgID(c), controlID)
	if err != nil {
		return errResp(c, http.StatusInternalServerError, "failed to list tasks", "CK_LIST_TASKS_FAILED")
	}
	return c.JSON(http.StatusOK, tasks)
}

// CreateControlTask handles POST /api/v1/secvitals/controls/:id/tasks.
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

// UpdateControlTask handles PATCH /api/v1/secvitals/controls/:id/tasks/:taskId.
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

// DeleteControlTask handles DELETE /api/v1/secvitals/controls/:id/tasks/:taskId.
func (h *Handler) DeleteControlTask(c echo.Context) error {
	controlID := c.Param("id")
	taskID := c.Param("taskId")
	ctx := c.Request().Context()
	if err := h.service.DeleteControlTask(ctx, orgID(c), controlID, taskID); err != nil {
		return errResp(c, http.StatusInternalServerError, "failed to delete task", "CK_DELETE_TASK_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
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

// GetExpiringEvidence handles GET /api/v1/secvitals/evidence/expiring.
// Returns evidence items expiring within the next N days (default: 30, max: 365).
func (h *Handler) GetExpiringEvidence(c echo.Context) error {
	days := 30
	if d := c.QueryParam("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 && parsed <= 365 {
			days = parsed
		}
	}
	items, err := h.service.GetExpiringEvidenceAll(c.Request().Context(), orgID(c), days)
	if err != nil {
		log.Error().Err(err).Msg("get expiring evidence")
		return errResp(c, http.StatusInternalServerError, "failed to get expiring evidence", "CK_EXPIRING_EVIDENCE_FAILED")
	}
	return c.JSON(http.StatusOK, items)
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

// UpdateControlSoAMetadata handles PATCH /api/v1/secvitals/controls/:id/soa.
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

// IncidentReportPDF handles GET /api/v1/secvitals/incidents/:id/report-pdf.
// It streams a BaFin-style DORA incident report as a PDF download.
func (h *Handler) IncidentReportPDF(c echo.Context) error {
	id := c.Param("id")
	inc, err := h.service.GetIncident(c.Request().Context(), orgID(c), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "no rows") {
			return errResp(c, http.StatusNotFound, "incident not found", "CK_INCIDENT_NOT_FOUND")
		}
		log.Error().Err(err).Str("incident_id", id).Msg("get incident for pdf")
		return errResp(c, http.StatusInternalServerError, "failed to retrieve incident", "CK_GET_INCIDENT_FAILED")
	}

	// Use org_id as a stand-in for org name when no name is available via context.
	// In production the org name can be resolved from the claims or a lookup.
	org := orgID(c)

	pdfBytes, err := GenerateIncidentReportPDF(inc, org)
	if err != nil {
		log.Error().Err(err).Str("incident_id", id).Msg("generate incident report pdf")
		return errResp(c, http.StatusInternalServerError, "failed to generate PDF", "CK_PDF_FAILED")
	}

	filename := fmt.Sprintf("incident-%s-bafin.pdf", inc.ID)
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	return c.Blob(http.StatusOK, "application/pdf", pdfBytes)
}

// AssessReportability handles POST /api/v1/secvitals/incidents/:id/assess-reportability.
func (h *Handler) AssessReportability(c echo.Context) error {
	id := c.Param("id")
	var in AssessReportabilityInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	result, err := h.service.AssessReportability(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "no rows") {
			return errResp(c, http.StatusNotFound, "incident not found", "CK_INCIDENT_NOT_FOUND")
		}
		log.Error().Err(err).Str("incident_id", id).Msg("assess reportability")
		return errResp(c, http.StatusInternalServerError, "failed to assess reportability", "CK_ASSESS_FAILED")
	}
	return c.JSON(http.StatusOK, result)
}

// GenerateIncidentReportForm handles POST /api/v1/secvitals/incidents/:id/reports.
func (h *Handler) GenerateIncidentReportForm(c echo.Context) error {
	id := c.Param("id")
	var body struct {
		ReportType string `json:"report_type" validate:"required,oneof=24h 72h 30d"`
	}
	if err := c.Bind(&body); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(body); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	report, _, err := h.service.GenerateIncidentReportForm(c.Request().Context(), orgID(c), id, body.ReportType, orgID(c))
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "no rows") {
			return errResp(c, http.StatusNotFound, "incident not found", "CK_INCIDENT_NOT_FOUND")
		}
		log.Error().Err(err).Str("incident_id", id).Msg("generate incident report form")
		return errResp(c, http.StatusInternalServerError, "failed to generate report", "CK_REPORT_FAILED")
	}
	return c.JSON(http.StatusCreated, report)
}

// ListIncidentReports handles GET /api/v1/secvitals/incidents/:id/reports.
func (h *Handler) ListIncidentReports(c echo.Context) error {
	id := c.Param("id")
	reports, err := h.service.ListIncidentReports(c.Request().Context(), orgID(c), id)
	if err != nil {
		log.Error().Err(err).Str("incident_id", id).Msg("list incident reports")
		return errResp(c, http.StatusInternalServerError, "failed to list reports", "CK_LIST_FAILED")
	}
	if reports == nil {
		reports = []IncidentReport{}
	}
	return c.JSON(http.StatusOK, reports)
}

// DownloadIncidentReportPDF handles GET /api/v1/secvitals/incident-reports/:reportId/pdf.
func (h *Handler) DownloadIncidentReportPDF(c echo.Context) error {
	reportID := c.Param("reportId")
	pdfBytes, err := h.service.GetIncidentReportPDF(c.Request().Context(), orgID(c), reportID)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return errResp(c, http.StatusNotFound, "report not found", "CK_REPORT_NOT_FOUND")
		}
		log.Error().Err(err).Str("report_id", reportID).Msg("download incident report pdf")
		return errResp(c, http.StatusInternalServerError, "failed to retrieve PDF", "CK_PDF_FAILED")
	}
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", "nis2-meldung-"+reportID+".pdf"))
	return c.Blob(http.StatusOK, "application/pdf", pdfBytes)
}

// MarkDeadlineReported handles POST /api/v1/secvitals/incidents/:id/mark-reported.
func (h *Handler) MarkDeadlineReported(c echo.Context) error {
	id := c.Param("id")
	var in MarkDeadlineReportedInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	inc, err := h.service.MarkDeadlineReported(c.Request().Context(), orgID(c), id, in.Deadline)
	if err != nil {
		log.Error().Err(err).Msg("mark deadline reported")
		return errResp(c, http.StatusInternalServerError, "failed to mark deadline", "CK_MARK_DEADLINE_FAILED")
	}
	return c.JSON(http.StatusOK, inc)
}

// --- Supplier Register ---

// ListSuppliers handles GET /api/v1/secvitals/suppliers.
// Supports optional query params: criticality, assessment_status.
func (h *Handler) ListSuppliers(c echo.Context) error {
	filter := &SupplierFilter{
		Criticality:      c.QueryParam("criticality"),
		AssessmentStatus: c.QueryParam("assessment_status"),
	}
	if filter.Criticality == "" && filter.AssessmentStatus == "" {
		filter = nil
	}
	suppliers, err := h.service.ListSuppliers(c.Request().Context(), orgID(c), filter)
	if err != nil {
		log.Error().Err(err).Msg("list suppliers")
		return errResp(c, http.StatusInternalServerError, "failed to list suppliers", "CK_LIST_SUPPLIERS_FAILED")
	}
	return c.JSON(http.StatusOK, suppliers)
}

// CreateSupplier handles POST /api/v1/secvitals/suppliers.
func (h *Handler) CreateSupplier(c echo.Context) error {
	var in CreateSupplierInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	s, err := h.service.CreateSupplier(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create supplier")
		return errResp(c, http.StatusInternalServerError, "failed to create supplier", "CK_CREATE_SUPPLIER_FAILED")
	}
	return c.JSON(http.StatusCreated, s)
}

// GetSupplier handles GET /api/v1/secvitals/suppliers/:id.
func (h *Handler) GetSupplier(c echo.Context) error {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid supplier id", "CK_BAD_REQUEST")
	}
	s, err := h.service.GetSupplier(c.Request().Context(), orgID(c), id)
	if err != nil {
		return errResp(c, http.StatusNotFound, "supplier not found", "CK_SUPPLIER_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, s)
}

// UpdateSupplier handles PATCH /api/v1/secvitals/suppliers/:id.
func (h *Handler) UpdateSupplier(c echo.Context) error {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid supplier id", "CK_BAD_REQUEST")
	}
	var in UpdateSupplierInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	s, err := h.service.UpdateSupplier(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		log.Error().Err(err).Msg("update supplier")
		return errResp(c, http.StatusInternalServerError, "failed to update supplier", "CK_UPDATE_SUPPLIER_FAILED")
	}
	return c.JSON(http.StatusOK, s)
}

// DeleteSupplier handles DELETE /api/v1/secvitals/suppliers/:id.
func (h *Handler) DeleteSupplier(c echo.Context) error {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid supplier id", "CK_BAD_REQUEST")
	}
	if err := h.service.DeleteSupplier(c.Request().Context(), orgID(c), id); err != nil {
		return errResp(c, http.StatusNotFound, "supplier not found", "CK_SUPPLIER_NOT_FOUND")
	}
	return c.NoContent(http.StatusNoContent)
}

// GetSupplierIncidents handles GET /api/v1/secvitals/suppliers/:id/incidents.
func (h *Handler) GetSupplierIncidents(c echo.Context) error {
	supplierID := c.Param("id")
	if _, err := uuid.Parse(supplierID); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid supplier id"})
	}
	incidents, err := h.service.ListIncidentsBySupplier(c.Request().Context(), orgID(c), supplierID)
	if err != nil {
		log.Error().Err(err).Str("supplier_id", supplierID).Msg("get supplier incidents")
		return errResp(c, http.StatusInternalServerError, "failed to list supplier incidents", "CK_LIST_SUPPLIER_INCIDENTS_FAILED")
	}
	return c.JSON(http.StatusOK, incidents)
}

// ExportSuppliers handles GET /api/v1/secvitals/suppliers/export.
// Returns a CSV file with all suppliers for the organisation.
func (h *Handler) ExportSuppliers(c echo.Context) error {
	suppliers, err := h.service.ListSuppliers(c.Request().Context(), orgID(c), nil)
	if err != nil {
		log.Error().Err(err).Msg("export suppliers: list suppliers")
		return errResp(c, http.StatusInternalServerError, "failed to list suppliers", "CK_LIST_SUPPLIERS_FAILED")
	}
	data, err := GenerateSupplierCSV(suppliers)
	if err != nil {
		log.Error().Err(err).Msg("export suppliers: generate csv")
		return errResp(c, http.StatusInternalServerError, "failed to generate CSV", "CK_EXPORT_SUPPLIERS_FAILED")
	}
	c.Response().Header().Set("Content-Disposition", `attachment; filename=suppliers-export.csv`)
	return c.Blob(http.StatusOK, "text/csv", data)
}

// ImportSuppliersCSV handles POST /api/v1/secvitals/suppliers/import-csv.
// Accepts a multipart form with field "file" containing a CSV.
func (h *Handler) ImportSuppliersCSV(c echo.Context) error {
	if err := c.Request().ParseMultipartForm(10 << 20); err != nil { // 10 MB
		return errResp(c, http.StatusBadRequest, "failed to parse multipart form", "CK_BAD_REQUEST")
	}
	file, _, err := c.Request().FormFile("file")
	if err != nil {
		return errResp(c, http.StatusBadRequest, "missing file field in multipart form", "CK_BAD_REQUEST")
	}
	defer file.Close()

	result, err := h.service.ParseAndImportSupplierCSV(c.Request().Context(), orgID(c), file)
	if err != nil {
		log.Error().Err(err).Msg("import suppliers csv")
		return errResp(c, http.StatusInternalServerError, "failed to import CSV", "CK_IMPORT_SUPPLIERS_FAILED")
	}
	return c.JSON(http.StatusOK, result)
}

// LinkSupplierRisk handles POST /api/v1/secvitals/suppliers/:id/risks.
func (h *Handler) LinkSupplierRisk(c echo.Context) error {
	supplierID := c.Param("id")
	if _, err := uuid.Parse(supplierID); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid supplier id", "CK_BAD_REQUEST")
	}
	var body struct {
		RiskID string `json:"risk_id"`
	}
	if err := c.Bind(&body); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if _, err := uuid.Parse(body.RiskID); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid risk_id", "CK_BAD_REQUEST")
	}
	if err := h.service.LinkSupplierRisk(c.Request().Context(), orgID(c), supplierID, body.RiskID); err != nil {
		log.Error().Err(err).Str("supplier_id", supplierID).Str("risk_id", body.RiskID).Msg("link supplier risk")
		if strings.Contains(err.Error(), "not found") {
			return errResp(c, http.StatusNotFound, err.Error(), "CK_NOT_FOUND")
		}
		return errResp(c, http.StatusInternalServerError, "failed to link risk", "CK_LINK_SUPPLIER_RISK_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// UnlinkSupplierRisk handles DELETE /api/v1/secvitals/suppliers/:id/risks/:riskId.
func (h *Handler) UnlinkSupplierRisk(c echo.Context) error {
	supplierID := c.Param("id")
	riskID := c.Param("riskId")
	if _, err := uuid.Parse(supplierID); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid supplier id", "CK_BAD_REQUEST")
	}
	if _, err := uuid.Parse(riskID); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid risk id", "CK_BAD_REQUEST")
	}
	if err := h.service.UnlinkSupplierRisk(c.Request().Context(), orgID(c), supplierID, riskID); err != nil {
		log.Error().Err(err).Str("supplier_id", supplierID).Str("risk_id", riskID).Msg("unlink supplier risk")
		if strings.Contains(err.Error(), "not found") {
			return errResp(c, http.StatusNotFound, err.Error(), "CK_NOT_FOUND")
		}
		return errResp(c, http.StatusInternalServerError, "failed to unlink risk", "CK_UNLINK_SUPPLIER_RISK_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// ListSupplierRisks handles GET /api/v1/secvitals/suppliers/:id/risks.
func (h *Handler) ListSupplierRisks(c echo.Context) error {
	supplierID := c.Param("id")
	if _, err := uuid.Parse(supplierID); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid supplier id", "CK_BAD_REQUEST")
	}
	risks, err := h.service.ListSupplierRisks(c.Request().Context(), orgID(c), supplierID)
	if err != nil {
		log.Error().Err(err).Str("supplier_id", supplierID).Msg("list supplier risks")
		if strings.Contains(err.Error(), "not found") {
			return errResp(c, http.StatusNotFound, err.Error(), "CK_NOT_FOUND")
		}
		return errResp(c, http.StatusInternalServerError, "failed to list supplier risks", "CK_LIST_SUPPLIER_RISKS_FAILED")
	}
	return c.JSON(http.StatusOK, risks)
}

// --- AI System Inventory ---

// ListAISystems handles GET /api/v1/secvitals/ai-systems.
func (h *Handler) ListAISystems(c echo.Context) error {
	filters := AISystemFilters{
		RiskClass: c.QueryParam("risk_class"),
		Status:    c.QueryParam("status"),
	}
	systems, err := h.service.ListAISystems(c.Request().Context(), orgID(c), filters)
	if err != nil {
		log.Error().Err(err).Msg("list ai systems")
		return errResp(c, http.StatusInternalServerError, "failed to list AI systems", "CK_LIST_AI_SYSTEMS_FAILED")
	}
	return c.JSON(http.StatusOK, systems)
}

// DeleteAISystem handles DELETE /api/v1/secvitals/ai-systems/:id.
func (h *Handler) DeleteAISystem(c echo.Context) error {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid AI system ID", "CK_INVALID_ID")
	}
	if err := h.service.DeleteAISystem(c.Request().Context(), orgID(c), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			return errResp(c, http.StatusNotFound, "AI system not found", "CK_NOT_FOUND")
		}
		return errResp(c, http.StatusInternalServerError, "failed to delete AI system", "CK_INTERNAL")
	}
	return c.NoContent(http.StatusNoContent)
}

// CreateAISystem handles POST /api/v1/secvitals/ai-systems.
func (h *Handler) CreateAISystem(c echo.Context) error {
	var in CreateAISystemInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	a, err := h.service.CreateAISystem(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create ai system")
		return errResp(c, http.StatusInternalServerError, "failed to create AI system", "CK_CREATE_AI_SYSTEM_FAILED")
	}
	return c.JSON(http.StatusCreated, a)
}

// GetAISystem handles GET /api/v1/secvitals/ai-systems/:id.
func (h *Handler) GetAISystem(c echo.Context) error {
	a, err := h.service.GetAISystem(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		return errResp(c, http.StatusNotFound, "AI system not found", "CK_AI_SYSTEM_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, a)
}

// UpdateAISystem handles PATCH /api/v1/secvitals/ai-systems/:id.
func (h *Handler) UpdateAISystem(c echo.Context) error {
	id := c.Param("id")
	var in UpdateAISystemInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	a, err := h.service.UpdateAISystem(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		log.Error().Err(err).Msg("update ai system")
		return errResp(c, http.StatusInternalServerError, "failed to update AI system", "CK_UPDATE_AI_SYSTEM_FAILED")
	}
	return c.JSON(http.StatusOK, a)
}

// ClassifyAISystem handles POST /api/v1/secvitals/ai-systems/:id/classify.
func (h *Handler) ClassifyAISystem(c echo.Context) error {
	id := c.Param("id")
	var in ClassifyAISystemInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.service.ClassifyAISystem(c.Request().Context(), orgID(c), id, in); err != nil {
		if errors.Is(err, ErrNotFound) {
			return errResp(c, http.StatusNotFound, "AI system not found", "CK_NOT_FOUND")
		}
		log.Error().Err(err).Msg("classify ai system")
		return errResp(c, http.StatusInternalServerError, "failed to classify AI system", "CK_CLASSIFY_AI_SYSTEM_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// ListAIClassifications handles GET /api/v1/secvitals/ai-systems/:id/classifications.
func (h *Handler) ListAIClassifications(c echo.Context) error {
	id := c.Param("id")
	list, err := h.service.ListAIClassifications(c.Request().Context(), orgID(c), id)
	if err != nil {
		log.Error().Err(err).Msg("list ai classifications")
		return errResp(c, http.StatusInternalServerError, "failed to list classifications", "CK_LIST_CLASSIFICATIONS_FAILED")
	}
	if list == nil {
		list = []AIClassification{}
	}
	return c.JSON(http.StatusOK, list)
}

// SaveAIDocumentation handles POST /api/v1/secvitals/ai-systems/:id/documentation.
func (h *Handler) SaveAIDocumentation(c echo.Context) error {
	id := c.Param("id")
	var in UpsertAIDocumentationInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	doc, err := h.service.SaveAIDocumentation(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		log.Error().Err(err).Msg("save ai documentation")
		return errResp(c, http.StatusInternalServerError, "failed to save documentation", "CK_SAVE_AI_DOC_FAILED")
	}
	return c.JSON(http.StatusOK, doc)
}

// GetLatestAIDocumentation handles GET /api/v1/secvitals/ai-systems/:id/documentation.
func (h *Handler) GetLatestAIDocumentation(c echo.Context) error {
	id := c.Param("id")
	doc, err := h.service.GetLatestAIDocumentation(c.Request().Context(), orgID(c), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return errResp(c, http.StatusNotFound, "documentation not found", "CK_NOT_FOUND")
		}
		return errResp(c, http.StatusInternalServerError, "failed to get documentation", "CK_GET_AI_DOC_FAILED")
	}
	return c.JSON(http.StatusOK, doc)
}

// ListAIDocumentationVersions handles GET /api/v1/secvitals/ai-systems/:id/documentation/versions.
func (h *Handler) ListAIDocumentationVersions(c echo.Context) error {
	id := c.Param("id")
	versions, err := h.service.ListAIDocumentationVersions(c.Request().Context(), orgID(c), id)
	if err != nil {
		return errResp(c, http.StatusInternalServerError, "failed to list documentation versions", "CK_LIST_AI_DOC_FAILED")
	}
	if versions == nil {
		versions = []AIDocumentation{}
	}
	return c.JSON(http.StatusOK, versions)
}

// ExportAIDocumentationPDF handles GET /api/v1/secvitals/ai-systems/:id/documentation/export-pdf.
func (h *Handler) ExportAIDocumentationPDF(c echo.Context) error {
	id := c.Param("id")
	pdfBytes, filename, err := h.service.ExportAIDocumentationPDF(c.Request().Context(), orgID(c), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return errResp(c, http.StatusNotFound, "AI system not found", "CK_NOT_FOUND")
		}
		log.Error().Err(err).Msg("export ai documentation pdf")
		return errResp(c, http.StatusInternalServerError, "failed to export PDF", "CK_EXPORT_AI_DOC_PDF_FAILED")
	}
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Response().Header().Set("Content-Type", "application/pdf")
	_, err = c.Response().Write(pdfBytes)
	return err
}

// GetOrgSector handles GET /api/v1/secvitals/org-sector.
func (h *Handler) GetOrgSector(c echo.Context) error {
	settings, err := h.service.GetOrgSector(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("get org sector")
		return errResp(c, http.StatusInternalServerError, "failed to get org sector", "CK_GET_SECTOR_FAILED")
	}
	return c.JSON(http.StatusOK, settings)
}

// UpdateOrgSector handles PATCH /api/v1/secvitals/org-sector.
func (h *Handler) UpdateOrgSector(c echo.Context) error {
	var in UpdateOrgSectorInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	settings, err := h.service.UpdateOrgSector(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("update org sector")
		return errResp(c, http.StatusInternalServerError, "failed to update org sector", "CK_UPDATE_SECTOR_FAILED")
	}
	return c.JSON(http.StatusOK, settings)
}

// ListAuthorities handles GET /api/v1/secvitals/authorities.
func (h *Handler) ListAuthorities(c echo.Context) error {
	all := ListAllAuthorities()
	return c.JSON(http.StatusOK, all)
}

// GetOrgAuthorities handles GET /api/v1/secvitals/org-authorities — sector-specific.
func (h *Handler) GetOrgAuthorities(c echo.Context) error {
	authorities, err := h.service.GetAuthoritiesForOrg(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("get org authorities")
		return errResp(c, http.StatusInternalServerError, "failed to get authorities", "CK_GET_AUTHORITIES_FAILED")
	}
	return c.JSON(http.StatusOK, authorities)
}

// GetEUAIActDashboard handles GET /api/v1/secvitals/eu-ai-act/dashboard.
func (h *Handler) GetEUAIActDashboard(c echo.Context) error {
	dashboard, err := h.service.GetEUAIActDashboard(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("get eu ai act dashboard")
		return errResp(c, http.StatusInternalServerError, "failed to get EU AI Act dashboard", "CK_EU_AI_ACT_DASHBOARD_FAILED")
	}
	return c.JSON(http.StatusOK, dashboard)
}

// GetEUAIActReportPDF handles GET /api/v1/secvitals/eu-ai-act/report-pdf.
func (h *Handler) GetEUAIActReportPDF(c echo.Context) error {
	pdfBytes, err := h.service.ExportEUAIActReportPDF(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("get eu ai act report pdf")
		return errResp(c, http.StatusInternalServerError, "failed to generate EU AI Act report PDF", "CK_EU_AI_ACT_PDF_FAILED")
	}
	c.Response().Header().Set("Content-Disposition", `attachment; filename="eu-ai-act-report.pdf"`)
	c.Response().Header().Set("Content-Type", "application/pdf")
	_, err = c.Response().Write(pdfBytes)
	return err
}

// --- Resilience Tests (DORA Art. 24-27) ---

// ListResilienceTests handles GET /api/v1/secvitals/resilience-tests.
func (h *Handler) ListResilienceTests(c echo.Context) error {
	tests, tlptOverdue, err := h.service.ListResilienceTests(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list resilience tests")
		return errResp(c, http.StatusInternalServerError, "failed to list resilience tests", "CK_LIST_RESILIENCE_TESTS_FAILED")
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"tests":               tests,
		"tlpt_overdue_warning": tlptOverdue,
	})
}

// CreateResilienceTest handles POST /api/v1/secvitals/resilience-tests.
func (h *Handler) CreateResilienceTest(c echo.Context) error {
	var in CreateResilienceTestInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	t, err := h.service.CreateResilienceTest(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create resilience test")
		return errResp(c, http.StatusInternalServerError, "failed to create resilience test", "CK_CREATE_RESILIENCE_TEST_FAILED")
	}
	return c.JSON(http.StatusCreated, t)
}

// GetResilienceTest handles GET /api/v1/secvitals/resilience-tests/:id.
func (h *Handler) GetResilienceTest(c echo.Context) error {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid resilience test id", "CK_BAD_REQUEST")
	}
	t, err := h.service.GetResilienceTest(c.Request().Context(), orgID(c), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "no rows") {
			return errResp(c, http.StatusNotFound, "resilience test not found", "CK_RESILIENCE_TEST_NOT_FOUND")
		}
		log.Error().Err(err).Str("id", id).Msg("get resilience test")
		return errResp(c, http.StatusInternalServerError, "failed to get resilience test", "CK_GET_RESILIENCE_TEST_FAILED")
	}
	return c.JSON(http.StatusOK, t)
}

// UpdateResilienceTest handles PATCH /api/v1/secvitals/resilience-tests/:id.
func (h *Handler) UpdateResilienceTest(c echo.Context) error {
	id := c.Param("id")
	var in UpdateResilienceTestInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	t, err := h.service.UpdateResilienceTest(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "no rows") {
			return errResp(c, http.StatusNotFound, "resilience test not found", "CK_RESILIENCE_TEST_NOT_FOUND")
		}
		log.Error().Err(err).Msg("update resilience test")
		return errResp(c, http.StatusInternalServerError, "failed to update resilience test", "CK_UPDATE_RESILIENCE_TEST_FAILED")
	}
	return c.JSON(http.StatusOK, t)
}

// DeleteResilienceTest handles DELETE /api/v1/secvitals/resilience-tests/:id.
func (h *Handler) DeleteResilienceTest(c echo.Context) error {
	id := c.Param("id")
	if err := h.service.DeleteResilienceTest(c.Request().Context(), orgID(c), id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return errResp(c, http.StatusNotFound, "resilience test not found", "CK_RESILIENCE_TEST_NOT_FOUND")
		}
		log.Error().Err(err).Msg("delete resilience test")
		return errResp(c, http.StatusInternalServerError, "failed to delete resilience test", "CK_DELETE_RESILIENCE_TEST_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// UploadResilienceTestAttachment handles POST /api/v1/secvitals/resilience-tests/:id/attachment.
// Accepts multipart/form-data with a "file" field. Max size: 20 MB.
func (h *Handler) UploadResilienceTestAttachment(c echo.Context) error {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid resilience test id", "CK_BAD_REQUEST")
	}

	const maxSize = 20 << 20 // 20 MB
	if err := c.Request().ParseMultipartForm(maxSize); err != nil {
		return errResp(c, http.StatusBadRequest, "failed to parse multipart form", "CK_BAD_REQUEST")
	}

	fh, err := c.FormFile("file")
	if err != nil {
		return errResp(c, http.StatusBadRequest, "file is required", "CK_BAD_REQUEST")
	}
	if fh.Size > maxSize {
		return errResp(c, http.StatusRequestEntityTooLarge, "file too large (max 20 MB)", "CK_FILE_TOO_LARGE")
	}

	src, err := fh.Open()
	if err != nil {
		return errResp(c, http.StatusInternalServerError, "failed to open uploaded file", "CK_UPLOAD_FAILED")
	}
	defer src.Close()

	fileBytes, err := io.ReadAll(src)
	if err != nil {
		return errResp(c, http.StatusInternalServerError, "failed to read uploaded file", "CK_UPLOAD_FAILED")
	}

	uploadDir := h.uploadDir
	if uploadDir == "" {
		uploadDir = "./data/uploads"
	}

	t, err := h.service.AttachResilienceTestFile(c.Request().Context(), orgID(c), id, uploadDir, fileBytes, filepath.Base(fh.Filename))
	if err != nil {
		log.Error().Err(err).Str("resilience_test_id", id).Msg("upload resilience test attachment")
		return errResp(c, http.StatusInternalServerError, "failed to save attachment", "CK_UPLOAD_FAILED")
	}
	return c.JSON(http.StatusOK, t)
}

// --- DORA Dashboard (Story 27.5) ---

// GetDORADashboard handles GET /api/v1/secvitals/dora/dashboard.
func (h *Handler) GetDORADashboard(c echo.Context) error {
	dashboard, err := h.service.GetDORADashboard(c.Request().Context(), orgID(c))
	if err != nil {
		if errors.Is(err, ErrDORANotEnabled) {
			return c.JSON(http.StatusNotFound, map[string]string{
				"error": "DORA framework not enabled",
				"code":  "CK_DORA_NOT_ENABLED",
			})
		}
		log.Error().Err(err).Msg("get dora dashboard")
		return errResp(c, http.StatusInternalServerError, "failed to get DORA dashboard", "CK_DORA_DASHBOARD_FAILED")
	}
	return c.JSON(http.StatusOK, dashboard)
}

// GetDORAPDF handles GET /api/v1/secvitals/dora/report-pdf.
func (h *Handler) GetDORAPDF(c echo.Context) error {
	pdfBytes, err := h.service.ExportDORAPDF(c.Request().Context(), orgID(c))
	if err != nil {
		if errors.Is(err, ErrDORANotEnabled) {
			return c.JSON(http.StatusNotFound, map[string]string{
				"error": "DORA framework not enabled",
				"code":  "CK_DORA_NOT_ENABLED",
			})
		}
		log.Error().Err(err).Msg("generate dora pdf")
		return errResp(c, http.StatusInternalServerError, "failed to generate PDF", "CK_DORA_PDF_FAILED")
	}
	c.Response().Header().Set("Content-Disposition", `attachment; filename="dora-bericht.pdf"`)
	return c.Blob(http.StatusOK, "application/pdf", pdfBytes)
}

// --- Framework Mappings (Story 28.2) ---

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

// --- Questionnaire Builder (Story 29.2) ---

// ListTemplates handles GET /api/v1/secvitals/questionnaires/templates.
func (h *Handler) ListTemplates(c echo.Context) error {
	templates, err := h.service.ListTemplates(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list templates")
		return errResp(c, http.StatusInternalServerError, "failed to list templates", "CK_LIST_TEMPLATES_FAILED")
	}
	return c.JSON(http.StatusOK, templates)
}

// ListQuestionnaires handles GET /api/v1/secvitals/questionnaires.
func (h *Handler) ListQuestionnaires(c echo.Context) error {
	var isTemplate *bool
	if raw := c.QueryParam("is_template"); raw != "" {
		v := raw == "true"
		isTemplate = &v
	}
	questionnaires, err := h.service.ListQuestionnaires(c.Request().Context(), orgID(c), isTemplate)
	if err != nil {
		log.Error().Err(err).Msg("list questionnaires")
		return errResp(c, http.StatusInternalServerError, "failed to list questionnaires", "CK_LIST_QUESTIONNAIRES_FAILED")
	}
	return c.JSON(http.StatusOK, questionnaires)
}

// CreateQuestionnaire handles POST /api/v1/secvitals/questionnaires.
func (h *Handler) CreateQuestionnaire(c echo.Context) error {
	var in CreateQuestionnaireInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	if in.CloneFromID != "" {
		if _, err := uuid.Parse(in.CloneFromID); err != nil {
			return errResp(c, http.StatusBadRequest, "invalid clone_from_id", "CK_BAD_REQUEST")
		}
	}
	q, err := h.service.CreateQuestionnaire(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create questionnaire")
		return errResp(c, http.StatusInternalServerError, "failed to create questionnaire", "CK_CREATE_QUESTIONNAIRE_FAILED")
	}
	return c.JSON(http.StatusCreated, q)
}

// GetQuestionnaire handles GET /api/v1/secvitals/questionnaires/:id.
func (h *Handler) GetQuestionnaire(c echo.Context) error {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid questionnaire id", "CK_BAD_REQUEST")
	}
	q, err := h.service.GetQuestionnaire(c.Request().Context(), orgID(c), id)
	if err != nil {
		return errResp(c, http.StatusNotFound, "questionnaire not found", "CK_QUESTIONNAIRE_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, q)
}

// UpdateQuestionnaire handles PATCH /api/v1/secvitals/questionnaires/:id.
func (h *Handler) UpdateQuestionnaire(c echo.Context) error {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid questionnaire id", "CK_BAD_REQUEST")
	}
	var in UpdateQuestionnaireInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	q, err := h.service.UpdateQuestionnaire(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return errResp(c, http.StatusNotFound, "questionnaire not found", "CK_QUESTIONNAIRE_NOT_FOUND")
		}
		log.Error().Err(err).Str("id", id).Msg("update questionnaire")
		return errResp(c, http.StatusInternalServerError, "failed to update questionnaire", "CK_UPDATE_QUESTIONNAIRE_FAILED")
	}
	return c.JSON(http.StatusOK, q)
}

// DeleteQuestionnaire handles DELETE /api/v1/secvitals/questionnaires/:id.
func (h *Handler) DeleteQuestionnaire(c echo.Context) error {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid questionnaire id", "CK_BAD_REQUEST")
	}
	if err := h.service.DeleteQuestionnaire(c.Request().Context(), orgID(c), id); err != nil {
		log.Error().Err(err).Str("id", id).Msg("delete questionnaire")
		return errResp(c, http.StatusInternalServerError, "failed to delete questionnaire", "CK_DELETE_QUESTIONNAIRE_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// AddQuestion handles POST /api/v1/secvitals/questionnaires/:id/questions.
func (h *Handler) AddQuestion(c echo.Context) error {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid questionnaire id", "CK_BAD_REQUEST")
	}
	var in CreateQuestionInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	if in.ControlID != "" {
		if _, err := uuid.Parse(in.ControlID); err != nil {
			return errResp(c, http.StatusBadRequest, "invalid control_id", "CK_BAD_REQUEST")
		}
	}
	q, err := h.service.AddQuestion(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		if strings.Contains(err.Error(), "multiple_choice question requires non-empty options") {
			return errResp(c, http.StatusUnprocessableEntity, err.Error(), "CK_VALIDATION_ERROR")
		}
		log.Error().Err(err).Str("questionnaire_id", id).Msg("add question")
		return errResp(c, http.StatusInternalServerError, "failed to add question", "CK_ADD_QUESTION_FAILED")
	}
	return c.JSON(http.StatusCreated, q)
}

// UpdateQuestion handles PATCH /api/v1/secvitals/questionnaires/:id/questions/:qid.
func (h *Handler) UpdateQuestion(c echo.Context) error {
	id := c.Param("id")
	qid := c.Param("qid")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid questionnaire id", "CK_BAD_REQUEST")
	}
	if _, err := uuid.Parse(qid); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid question id", "CK_BAD_REQUEST")
	}
	var in CreateQuestionInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	if in.ControlID != "" {
		if _, err := uuid.Parse(in.ControlID); err != nil {
			return errResp(c, http.StatusBadRequest, "invalid control_id", "CK_BAD_REQUEST")
		}
	}
	q, err := h.service.UpdateQuestion(c.Request().Context(), orgID(c), id, qid, in)
	if err != nil {
		if strings.Contains(err.Error(), "multiple_choice question requires non-empty options") {
			return errResp(c, http.StatusUnprocessableEntity, err.Error(), "CK_VALIDATION_ERROR")
		}
		log.Error().Err(err).Str("questionnaire_id", id).Str("question_id", qid).Msg("update question")
		return errResp(c, http.StatusInternalServerError, "failed to update question", "CK_UPDATE_QUESTION_FAILED")
	}
	return c.JSON(http.StatusOK, q)
}

// DeleteQuestion handles DELETE /api/v1/secvitals/questionnaires/:id/questions/:qid.
func (h *Handler) DeleteQuestion(c echo.Context) error {
	id := c.Param("id")
	qid := c.Param("qid")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid questionnaire id", "CK_BAD_REQUEST")
	}
	if _, err := uuid.Parse(qid); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid question id", "CK_BAD_REQUEST")
	}
	if err := h.service.DeleteQuestion(c.Request().Context(), orgID(c), id, qid); err != nil {
		log.Error().Err(err).Str("questionnaire_id", id).Str("question_id", qid).Msg("delete question")
		return errResp(c, http.StatusInternalServerError, "failed to delete question", "CK_DELETE_QUESTION_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// ReorderQuestions handles POST /api/v1/secvitals/questionnaires/:id/questions/reorder.
func (h *Handler) ReorderQuestions(c echo.Context) error {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid questionnaire id", "CK_BAD_REQUEST")
	}
	var in ReorderQuestionsInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	for _, qid := range in.Order {
		if _, err := uuid.Parse(qid); err != nil {
			return errResp(c, http.StatusBadRequest, fmt.Sprintf("invalid question id in order: %s", qid), "CK_BAD_REQUEST")
		}
	}
	if err := h.service.ReorderQuestions(c.Request().Context(), orgID(c), id, in.Order); err != nil {
		log.Error().Err(err).Str("questionnaire_id", id).Msg("reorder questions")
		return errResp(c, http.StatusInternalServerError, "failed to reorder questions", "CK_REORDER_QUESTIONS_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// --- Supplier Portal Assessments (Story 29.3) ---

// CreateSupplierAssessment handles POST /api/v1/secvitals/suppliers/:id/assessments.
func (h *Handler) CreateSupplierAssessment(c echo.Context) error {
	supplierID := c.Param("id")
	if _, err := uuid.Parse(supplierID); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid supplier id", "CK_BAD_REQUEST")
	}
	var in CreateAssessmentInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}

	req := c.Request()
	scheme := "https"
	if req.TLS == nil {
		scheme = "http"
	}
	baseURL := scheme + "://" + req.Host

	assessment, _, err := h.service.CreateAssessment(c.Request().Context(), orgID(c), supplierID, in, baseURL)
	if err != nil {
		log.Error().Err(err).Str("supplier_id", supplierID).Msg("create supplier assessment")
		return errResp(c, http.StatusInternalServerError, "failed to create assessment", "CK_CREATE_ASSESSMENT_FAILED")
	}
	return c.JSON(http.StatusCreated, map[string]string{
		"id":        assessment.ID,
		"share_url": assessment.ShareURL,
	})
}

// ListSupplierAssessments handles GET /api/v1/secvitals/suppliers/:id/assessments.
func (h *Handler) ListSupplierAssessments(c echo.Context) error {
	supplierID := c.Param("id")
	if _, err := uuid.Parse(supplierID); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid supplier id", "CK_BAD_REQUEST")
	}
	assessments, err := h.service.ListAssessmentsForSupplier(c.Request().Context(), orgID(c), supplierID)
	if err != nil {
		log.Error().Err(err).Str("supplier_id", supplierID).Msg("list supplier assessments")
		return errResp(c, http.StatusInternalServerError, "failed to list assessments", "CK_LIST_ASSESSMENTS_FAILED")
	}
	if assessments == nil {
		assessments = []Assessment{}
	}
	return c.JSON(http.StatusOK, assessments)
}

// GetAssessment handles GET /api/v1/secvitals/assessments/:id.
func (h *Handler) GetAssessment(c echo.Context) error {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid assessment id", "CK_BAD_REQUEST")
	}
	a, err := h.service.GetAssessment(c.Request().Context(), orgID(c), id)
	if err != nil {
		return errResp(c, http.StatusNotFound, "assessment not found", "CK_ASSESSMENT_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, a)
}

// PortalGetAssessment handles GET /supplier/:token (public, no auth).
func (h *Handler) PortalGetAssessment(c echo.Context) error {
	token := c.Param("token")
	a, err := h.service.GetAssessmentForPortal(c.Request().Context(), token)
	if err != nil {
		if errors.Is(err, ErrAssessmentExpiredOrSubmitted) {
			return c.JSON(http.StatusGone, map[string]string{"error": "assessment_expired_or_submitted"})
		}
		log.Error().Err(err).Msg("portal get assessment")
		return errResp(c, http.StatusInternalServerError, "failed to load assessment", "CK_ASSESSMENT_LOAD_FAILED")
	}
	return c.JSON(http.StatusOK, a)
}

// PortalSaveAnswers handles POST /supplier/:token/save (public, no auth).
func (h *Handler) PortalSaveAnswers(c echo.Context) error {
	token := c.Param("token")
	var in SaveAnswersInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	if err := h.service.SaveAnswers(c.Request().Context(), token, in); err != nil {
		if errors.Is(err, ErrAssessmentExpiredOrSubmitted) {
			return c.JSON(http.StatusGone, map[string]string{"error": "assessment_expired_or_submitted"})
		}
		log.Error().Err(err).Msg("portal save answers")
		return errResp(c, http.StatusInternalServerError, "failed to save answers", "CK_SAVE_ANSWERS_FAILED")
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "saved"})
}

// PortalSubmitAssessment handles POST /supplier/:token/submit (public, no auth).
func (h *Handler) PortalSubmitAssessment(c echo.Context) error {
	token := c.Param("token")
	var in SaveAnswersInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	clientIP := c.RealIP()
	userAgent := c.Request().Header.Get("User-Agent")
	if len(userAgent) > 512 {
		userAgent = userAgent[:512]
	}
	if err := h.service.SubmitAssessment(c.Request().Context(), token, clientIP, userAgent, in); err != nil {
		if errors.Is(err, ErrAssessmentExpiredOrSubmitted) {
			return c.JSON(http.StatusGone, map[string]string{"error": "assessment_expired_or_submitted"})
		}
		log.Error().Err(err).Msg("portal submit assessment")
		return errResp(c, http.StatusInternalServerError, "failed to submit assessment", "CK_SUBMIT_ASSESSMENT_FAILED")
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "submitted"})
}

// PortalUploadFile handles POST /supplier/:token/upload (public, no auth).
// Accepts a file (max 20 MB, allowed MIMEs: PDF/PNG/JPEG/XLSX).
func (h *Handler) PortalUploadFile(c echo.Context) error {
	token := c.Param("token")

	// Validate token is for a live assessment.
	a, err := h.service.GetAssessmentForPortal(c.Request().Context(), token)
	if err != nil {
		if errors.Is(err, ErrAssessmentExpiredOrSubmitted) {
			return c.JSON(http.StatusGone, map[string]string{"error": "assessment_expired_or_submitted"})
		}
		log.Error().Err(err).Msg("portal upload: validate token")
		return errResp(c, http.StatusInternalServerError, "failed to validate assessment", "CK_ASSESSMENT_LOAD_FAILED")
	}

	const maxUploadSize = 20 << 20 // 20 MB
	if err := c.Request().ParseMultipartForm(maxUploadSize); err != nil {
		return errResp(c, http.StatusBadRequest, "failed to parse multipart form", "CK_BAD_REQUEST")
	}

	fh, err := c.FormFile("file")
	if err != nil {
		return errResp(c, http.StatusBadRequest, "file is required", "CK_BAD_REQUEST")
	}
	if fh.Size > maxUploadSize {
		return errResp(c, http.StatusRequestEntityTooLarge, "file exceeds 20 MB limit", "CK_FILE_TOO_LARGE")
	}

	src, err := fh.Open()
	if err != nil {
		return errResp(c, http.StatusInternalServerError, "failed to open uploaded file", "CK_UPLOAD_FAILED")
	}
	defer src.Close()

	// Read first 512 bytes for MIME detection.
	buf := make([]byte, 512)
	n, _ := src.Read(buf)
	detectedMIME := http.DetectContentType(buf[:n])

	ext := strings.ToLower(filepath.Ext(fh.Filename))
	allowedMIMEs := map[string]bool{
		"application/pdf": true,
		"image/png":       true,
		"image/jpeg":      true,
	}
	// XLSX is a ZIP archive: http.DetectContentType returns "application/zip".
	// Accept only when extension AND detected type agree to prevent file-rename bypass.
	xlsxAllowed := ext == ".xlsx" && detectedMIME == "application/zip"
	if !allowedMIMEs[detectedMIME] && !xlsxAllowed {
		return errResp(c, http.StatusUnsupportedMediaType, "unsupported file type", "CK_UNSUPPORTED_MIME")
	}

	uploadDir := h.uploadDir
	if uploadDir == "" {
		uploadDir = "./data/uploads"
	}
	assessmentDir := filepath.Join(uploadDir, "supplier-assessments", a.ID)
	if err := os.MkdirAll(assessmentDir, 0o750); err != nil {
		return errResp(c, http.StatusInternalServerError, "failed to create upload directory", "CK_UPLOAD_FAILED")
	}

	destName := uuid.New().String() + ext
	destPath := filepath.Join(assessmentDir, destName)

	dst, err := os.Create(destPath)
	if err != nil {
		return errResp(c, http.StatusInternalServerError, "failed to save file", "CK_UPLOAD_FAILED")
	}
	defer dst.Close()

	// Write already-read bytes first.
	if _, err := dst.Write(buf[:n]); err != nil {
		return errResp(c, http.StatusInternalServerError, "failed to write file", "CK_UPLOAD_FAILED")
	}
	if _, err := io.Copy(dst, src); err != nil {
		return errResp(c, http.StatusInternalServerError, "failed to write file", "CK_UPLOAD_FAILED")
	}

	// Return a relative URL rather than the raw filesystem path.
	fileURL := "/uploads/supplier-assessments/" + a.ID + "/" + destName
	return c.JSON(http.StatusOK, map[string]string{"file_url": fileURL})
}

// --- Assessment Review (Story 29.4) ---

// ReviewAnswer handles PATCH /secvitals/assessments/:id/answers/:aid.
func (h *Handler) ReviewAnswer(c echo.Context) error {
	orgID := orgID(c)
	assessmentID := c.Param("id")
	answerID := c.Param("aid")
	if _, err := uuid.Parse(assessmentID); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid assessment ID", "CK_INVALID_ID")
	}
	if _, err := uuid.Parse(answerID); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid answer ID", "CK_INVALID_ID")
	}
	var in ReviewAnswerInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusBadRequest, "Ungültige Eingabe", "VALIDATION_ERROR")
	}
	evidenceID, err := h.service.ReviewAnswer(c.Request().Context(), orgID, assessmentID, answerID, in)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return errResp(c, http.StatusNotFound, "answer not found", "CK_NOT_FOUND")
		}
		return errResp(c, http.StatusBadRequest, err.Error(), "CK_VALIDATION_ERROR")
	}
	resp := map[string]any{"ok": true}
	if evidenceID != nil {
		resp["evidence_id"] = *evidenceID
	}
	return c.JSON(http.StatusOK, resp)
}

// GetSupplierStatus handles GET /secvitals/suppliers/:id/status.
func (h *Handler) GetSupplierStatus(c echo.Context) error {
	orgID := orgID(c)
	supplierID := c.Param("id")
	if _, err := uuid.Parse(supplierID); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid supplier ID", "CK_INVALID_ID")
	}
	status, err := h.service.ComputeSupplierStatus(c.Request().Context(), orgID, supplierID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return errResp(c, http.StatusNotFound, "supplier not found", "CK_NOT_FOUND")
		}
		return errResp(c, http.StatusInternalServerError, "failed to compute status", "CK_INTERNAL")
	}
	return c.JSON(http.StatusOK, status)
}

// UpdateAssessment handles PATCH /secvitals/assessments/:id (status=reviewed only).
func (h *Handler) UpdateAssessment(c echo.Context) error {
	orgID := orgID(c)
	assessmentID := c.Param("id")
	if _, err := uuid.Parse(assessmentID); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid assessment ID", "CK_INVALID_ID")
	}
	var in UpdateAssessmentInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusBadRequest, "Ungültige Eingabe", "VALIDATION_ERROR")
	}
	if err := h.service.MarkAssessmentReviewed(c.Request().Context(), orgID, assessmentID); err != nil {
		if errors.Is(err, ErrNotFound) {
			return errResp(c, http.StatusNotFound, "assessment not found or not in submitted state", "CK_NOT_FOUND")
		}
		return errResp(c, http.StatusInternalServerError, "failed to update assessment", "CK_INTERNAL")
	}
	return c.JSON(http.StatusOK, map[string]bool{"ok": true})
}

// GetAssessmentAnswers handles GET /secvitals/assessments/:id/answers.
func (h *Handler) GetAssessmentAnswers(c echo.Context) error {
	orgID := orgID(c)
	assessmentID := c.Param("id")
	if _, err := uuid.Parse(assessmentID); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid assessment ID", "CK_INVALID_ID")
	}
	answers, err := h.service.GetAnswersForAssessment(c.Request().Context(), orgID, assessmentID)
	if err != nil {
		return errResp(c, http.StatusInternalServerError, "failed to load answers", "CK_INTERNAL")
	}
	return c.JSON(http.StatusOK, answers)
}

// ExportAuditPackage handles GET /frameworks/:id/audit-package.zip.
// Returns a ZIP archive with INDEX.pdf, summary.json, and per-control evidence files.
func (h *Handler) ExportAuditPackage(c echo.Context) error {
	data, filename, err := h.service.ExportAuditPackage(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		log.Error().Err(err).Str("framework_id", c.Param("id")).Msg("export audit package")
		return errResp(c, http.StatusInternalServerError, "failed to generate audit package", "CK_AUDIT_PACKAGE_FAILED")
	}
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	return c.Blob(http.StatusOK, "application/zip", data)
}

// GeneratePolicyDraft handles POST /api/v1/secvitals/policies/generate-draft.
// Generates an AI-written policy draft in German using the configured AI provider.
func (h *Handler) GeneratePolicyDraft(c echo.Context) error {
	var in GeneratePolicyDraftInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_INVALID_BODY")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, "Ungültige Eingabe", "VALIDATION_ERROR")
	}
	draft, err := h.service.GeneratePolicyDraft(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("generate policy draft")
		if strings.Contains(err.Error(), "nicht konfiguriert") {
			return errResp(c, http.StatusServiceUnavailable, err.Error(), "CK_AI_NOT_CONFIGURED")
		}
		return errResp(c, http.StatusInternalServerError, "AI generation failed", "CK_AI_FAILED")
	}
	return c.JSON(http.StatusOK, map[string]string{"draft": draft})
}

// GetAssessmentReportPDF handles GET /secvitals/assessments/:id/report-pdf.
func (h *Handler) GetAssessmentReportPDF(c echo.Context) error {
	orgID := orgID(c)
	assessmentID := c.Param("id")
	if _, err := uuid.Parse(assessmentID); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid assessment ID", "CK_INVALID_ID")
	}
	pdf, err := h.service.GenerateAssessmentReportPDF(c.Request().Context(), orgID, assessmentID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return errResp(c, http.StatusNotFound, "assessment not found", "CK_NOT_FOUND")
		}
		return errResp(c, http.StatusInternalServerError, "failed to generate PDF", "CK_INTERNAL")
	}
	c.Response().Header().Set("Content-Type", "application/pdf")
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", "assessment-"+assessmentID+".pdf"))
	return c.Blob(http.StatusOK, "application/pdf", pdf)
}

// --- Maßnahmen-Katalog (control measures) ---

// ListMeasures handles GET /api/v1/secvitals/controls/:id/measures.
func (h *Handler) ListMeasures(c echo.Context) error {
	controlID := c.Param("id")
	if controlID == "" {
		return errResp(c, http.StatusBadRequest, "control id is required", "CK_BAD_REQUEST")
	}
	measures, err := h.service.ListMeasures(c.Request().Context(), orgID(c), controlID)
	if err != nil {
		log.Error().Err(err).Str("control_id", controlID).Msg("list measures")
		return errResp(c, http.StatusInternalServerError, "failed to list measures", "CK_INTERNAL")
	}
	if measures == nil {
		measures = []ControlMeasure{}
	}
	return c.JSON(http.StatusOK, measures)
}

// CreateMeasure handles POST /api/v1/secvitals/controls/:id/measures.
func (h *Handler) CreateMeasure(c echo.Context) error {
	controlID := c.Param("id")
	if controlID == "" {
		return errResp(c, http.StatusBadRequest, "control id is required", "CK_BAD_REQUEST")
	}
	var in CreateMeasureInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusBadRequest, "Ungültige Eingabe", "VALIDATION_ERROR")
	}
	measure, err := h.service.CreateMeasure(c.Request().Context(), orgID(c), controlID, in)
	if err != nil {
		log.Error().Err(err).Str("control_id", controlID).Msg("create measure")
		return errResp(c, http.StatusInternalServerError, "failed to create measure", "CK_INTERNAL")
	}
	return c.JSON(http.StatusCreated, measure)
}

// UpdateMeasure handles PATCH /api/v1/secvitals/controls/:id/measures/:mid.
func (h *Handler) UpdateMeasure(c echo.Context) error {
	measureID := c.Param("mid")
	if measureID == "" {
		return errResp(c, http.StatusBadRequest, "measure id is required", "CK_BAD_REQUEST")
	}
	var in UpdateMeasureInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusBadRequest, "Ungültige Eingabe", "VALIDATION_ERROR")
	}
	measure, err := h.service.UpdateMeasure(c.Request().Context(), orgID(c), measureID, in)
	if err != nil {
		log.Error().Err(err).Str("measure_id", measureID).Msg("update measure")
		return errResp(c, http.StatusInternalServerError, "failed to update measure", "CK_INTERNAL")
	}
	return c.JSON(http.StatusOK, measure)
}

// DeleteMeasure handles DELETE /api/v1/secvitals/controls/:id/measures/:mid.
func (h *Handler) DeleteMeasure(c echo.Context) error {
	measureID := c.Param("mid")
	if measureID == "" {
		return errResp(c, http.StatusBadRequest, "measure id is required", "CK_BAD_REQUEST")
	}
	if err := h.service.DeleteMeasure(c.Request().Context(), orgID(c), measureID); err != nil {
		log.Error().Err(err).Str("measure_id", measureID).Msg("delete measure")
		return errResp(c, http.StatusInternalServerError, "failed to delete measure", "CK_INTERNAL")
	}
	return c.NoContent(http.StatusNoContent)
}

// --- Collaborative Tasks & Comments ---

// urlEntityType maps the plural URL segment (e.g. "controls") to the singular DB entity_type value.
var urlEntityType = map[string]string{
	"controls":  "control",
	"risks":     "risk",
	"incidents": "incident",
	"policies":  "policy",
	"audits":    "audit",
}

// listTasksFor returns an Echo handler that lists collab tasks for the given entity type.
func (h *Handler) listTasksFor(entityType string) echo.HandlerFunc {
	return func(c echo.Context) error {
		entityID := c.Param("id")
		if entityID == "" {
			return errResp(c, http.StatusBadRequest, "entity id is required", "CK_BAD_REQUEST")
		}
		tasks, err := h.service.ListTasks(c.Request().Context(), orgID(c), entityType, entityID)
		if err != nil {
			log.Error().Err(err).Str("entity_type", entityType).Str("entity_id", entityID).Msg("list collab tasks")
			return errResp(c, http.StatusInternalServerError, "failed to list tasks", "CK_INTERNAL")
		}
		return c.JSON(http.StatusOK, tasks)
	}
}

// createTaskFor returns an Echo handler that creates a collab task for the given entity type.
func (h *Handler) createTaskFor(entityType string) echo.HandlerFunc {
	return func(c echo.Context) error {
		entityID := c.Param("id")
		if entityID == "" {
			return errResp(c, http.StatusBadRequest, "entity id is required", "CK_BAD_REQUEST")
		}
		var in CreateTaskInput
		if err := c.Bind(&in); err != nil {
			return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
		}
		if err := h.validate.Struct(in); err != nil {
			return c.JSON(http.StatusUnprocessableEntity, map[string]string{
				"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR",
			})
		}
		task, err := h.service.CreateTask(c.Request().Context(), orgID(c), entityType, entityID, in)
		if err != nil {
			log.Error().Err(err).Str("entity_type", entityType).Str("entity_id", entityID).Msg("create collab task")
			return errResp(c, http.StatusInternalServerError, "failed to create task", "CK_INTERNAL")
		}
		return c.JSON(http.StatusCreated, task)
	}
}

// UpdateCollabTask handles PATCH /secvitals/collab-tasks/:tid.
func (h *Handler) UpdateCollabTask(c echo.Context) error {
	taskID := c.Param("tid")
	if taskID == "" {
		return errResp(c, http.StatusBadRequest, "task id is required", "CK_BAD_REQUEST")
	}
	var in UpdateTaskInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR",
		})
	}
	task, err := h.service.UpdateTask(c.Request().Context(), orgID(c), taskID, in)
	if err != nil {
		log.Error().Err(err).Str("task_id", taskID).Msg("update collab task")
		return errResp(c, http.StatusInternalServerError, "failed to update task", "CK_INTERNAL")
	}
	return c.JSON(http.StatusOK, task)
}

// DeleteCollabTask handles DELETE /secvitals/collab-tasks/:tid.
func (h *Handler) DeleteCollabTask(c echo.Context) error {
	taskID := c.Param("tid")
	if taskID == "" {
		return errResp(c, http.StatusBadRequest, "task id is required", "CK_BAD_REQUEST")
	}
	if err := h.service.DeleteTask(c.Request().Context(), orgID(c), taskID); err != nil {
		log.Error().Err(err).Str("task_id", taskID).Msg("delete collab task")
		return errResp(c, http.StatusInternalServerError, "failed to delete task", "CK_INTERNAL")
	}
	return c.NoContent(http.StatusNoContent)
}

// listCommentsFor returns an Echo handler that lists comments for the given entity type.
func (h *Handler) listCommentsFor(entityType string) echo.HandlerFunc {
	return func(c echo.Context) error {
		entityID := c.Param("id")
		if entityID == "" {
			return errResp(c, http.StatusBadRequest, "entity id is required", "CK_BAD_REQUEST")
		}
		comments, err := h.service.ListComments(c.Request().Context(), orgID(c), entityType, entityID)
		if err != nil {
			log.Error().Err(err).Str("entity_type", entityType).Str("entity_id", entityID).Msg("list comments")
			return errResp(c, http.StatusInternalServerError, "failed to list comments", "CK_INTERNAL")
		}
		return c.JSON(http.StatusOK, comments)
	}
}

// createCommentFor returns an Echo handler that creates a comment for the given entity type.
func (h *Handler) createCommentFor(entityType string) echo.HandlerFunc {
	return func(c echo.Context) error {
		entityID := c.Param("id")
		if entityID == "" {
			return errResp(c, http.StatusBadRequest, "entity id is required", "CK_BAD_REQUEST")
		}
		var in CreateCommentInput
		if err := c.Bind(&in); err != nil {
			return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
		}
		if err := h.validate.Struct(in); err != nil {
			return c.JSON(http.StatusUnprocessableEntity, map[string]string{
				"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR",
			})
		}
		comment, err := h.service.CreateComment(c.Request().Context(), orgID(c), entityType, entityID, in)
		if err != nil {
			log.Error().Err(err).Str("entity_type", entityType).Str("entity_id", entityID).Msg("create comment")
			return errResp(c, http.StatusInternalServerError, "failed to create comment", "CK_INTERNAL")
		}
		return c.JSON(http.StatusCreated, comment)
	}
}

// DeleteComment handles DELETE /secvitals/comments/:cid.
func (h *Handler) DeleteCollabComment(c echo.Context) error {
	commentID := c.Param("cid")
	if commentID == "" {
		return errResp(c, http.StatusBadRequest, "comment id is required", "CK_BAD_REQUEST")
	}
	if err := h.service.DeleteComment(c.Request().Context(), orgID(c), commentID); err != nil {
		log.Error().Err(err).Str("comment_id", commentID).Msg("delete comment")
		return errResp(c, http.StatusInternalServerError, "failed to delete comment", "CK_INTERNAL")
	}
	return c.NoContent(http.StatusNoContent)
}

// --- CAPA Handlers ---

// ListCAPAs handles GET /secvitals/capas?status=open.
func (h *Handler) ListCAPAs(c echo.Context) error {
	statusFilter := c.QueryParam("status")
	offset, limit, meta := pagination.FromRequest(c)
	capas, total, err := h.service.ListCAPAsPaged(c.Request().Context(), orgID(c), statusFilter, offset, limit)
	if err != nil {
		log.Error().Err(err).Msg("list capas")
		return errResp(c, http.StatusInternalServerError, "failed to list capas", "CK_LIST_CAPAS_FAILED")
	}
	pagination.Complete(&meta, total)
	return c.JSON(http.StatusOK, pagination.Wrap(capas, meta))
}

// CreateCAPA handles POST /secvitals/capas.
func (h *Handler) CreateCAPA(c echo.Context) error {
	var in CreateCAPAInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	capa, err := h.service.CreateCAPA(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create capa")
		return errResp(c, http.StatusInternalServerError, "failed to create capa", "CK_CREATE_CAPA_FAILED")
	}
	return c.JSON(http.StatusCreated, capa)
}

// GetCAPA handles GET /secvitals/capas/:id.
func (h *Handler) GetCAPA(c echo.Context) error {
	capa, err := h.service.GetCAPA(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return errResp(c, http.StatusNotFound, "capa not found", "CK_CAPA_NOT_FOUND")
		}
		log.Error().Err(err).Msg("get capa")
		return errResp(c, http.StatusInternalServerError, "failed to get capa", "CK_GET_CAPA_FAILED")
	}
	return c.JSON(http.StatusOK, capa)
}

// UpdateCAPA handles PATCH /secvitals/capas/:id.
func (h *Handler) UpdateCAPA(c echo.Context) error {
	var in UpdateCAPAInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	capa, err := h.service.UpdateCAPA(c.Request().Context(), orgID(c), c.Param("id"), in)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return errResp(c, http.StatusNotFound, "capa not found", "CK_CAPA_NOT_FOUND")
		}
		log.Error().Err(err).Msg("update capa")
		return errResp(c, http.StatusInternalServerError, "failed to update capa", "CK_UPDATE_CAPA_FAILED")
	}
	return c.JSON(http.StatusOK, capa)
}

// DeleteCAPA handles DELETE /secvitals/capas/:id.
func (h *Handler) DeleteCAPA(c echo.Context) error {
	if err := h.service.DeleteCAPA(c.Request().Context(), orgID(c), c.Param("id")); err != nil {
		if errors.Is(err, ErrNotFound) {
			return errResp(c, http.StatusNotFound, "capa not found", "CK_CAPA_NOT_FOUND")
		}
		log.Error().Err(err).Msg("delete capa")
		return errResp(c, http.StatusInternalServerError, "failed to delete capa", "CK_DELETE_CAPA_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// ListCAPAsForAudit handles GET /secvitals/audits/:id/capas.
func (h *Handler) ListCAPAsForAudit(c echo.Context) error {
	capas, err := h.service.ListCAPAsForSource(c.Request().Context(), orgID(c), "audit", c.Param("id"))
	if err != nil {
		log.Error().Err(err).Msg("list capas for audit")
		return errResp(c, http.StatusInternalServerError, "failed to list capas", "CK_LIST_CAPAS_FAILED")
	}
	if capas == nil {
		capas = []CAPA{}
	}
	return c.JSON(http.StatusOK, capas)
}

// CreateCAPAFromAudit handles POST /secvitals/audits/:id/capas.
func (h *Handler) CreateCAPAFromAudit(c echo.Context) error {
	var in CreateCAPAInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	in.SourceType = "audit"
	in.SourceID = c.Param("id")
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	capa, err := h.service.CreateCAPA(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create capa from audit")
		return errResp(c, http.StatusInternalServerError, "failed to create capa", "CK_CREATE_CAPA_FAILED")
	}
	return c.JSON(http.StatusCreated, capa)
}

// ListCAPAsForIncident handles GET /secvitals/incidents/:id/capas.
func (h *Handler) ListCAPAsForIncident(c echo.Context) error {
	capas, err := h.service.ListCAPAsForSource(c.Request().Context(), orgID(c), "incident", c.Param("id"))
	if err != nil {
		log.Error().Err(err).Msg("list capas for incident")
		return errResp(c, http.StatusInternalServerError, "failed to list capas", "CK_LIST_CAPAS_FAILED")
	}
	if capas == nil {
		capas = []CAPA{}
	}
	return c.JSON(http.StatusOK, capas)
}

// CreateCAPAFromIncident handles POST /secvitals/incidents/:id/capas.
func (h *Handler) CreateCAPAFromIncident(c echo.Context) error {
	var in CreateCAPAInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	in.SourceType = "incident"
	in.SourceID = c.Param("id")
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	capa, err := h.service.CreateCAPA(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create capa from incident")
		return errResp(c, http.StatusInternalServerError, "failed to create capa", "CK_CREATE_CAPA_FAILED")
	}
	return c.JSON(http.StatusCreated, capa)
}

// --- Evidence Files (Migration 074) ---

// WithEvidenceFileService attaches the EvidenceFileService to the handler.
func (h *Handler) WithEvidenceFileService(s *EvidenceFileService) *Handler {
	h.evidenceFiles = s
	return h
}

// UploadEvidenceFile handles POST /secvitals/controls/:id/evidence-files.
// Accepts multipart form with a single "file" field.
func (h *Handler) UploadEvidenceFile(c echo.Context) error {
	if h.evidenceFiles == nil {
		return errResp(c, http.StatusServiceUnavailable, "evidence file service unavailable", "CK_SERVICE_UNAVAILABLE")
	}
	controlID := c.Param("id")
	evidenceID := c.FormValue("evidence_id") // optional

	fh, err := c.FormFile("file")
	if err != nil {
		return errResp(c, http.StatusBadRequest, "file is required", "CK_BAD_REQUEST")
	}
	src, err := fh.Open()
	if err != nil {
		return errResp(c, http.StatusInternalServerError, "failed to open uploaded file", "CK_UPLOAD_FAILED")
	}
	defer src.Close()

	ef, err := h.evidenceFiles.Upload(
		c.Request().Context(),
		orgID(c), controlID, evidenceID, userID(c),
		src, fh,
	)
	if err != nil {
		log.Error().Err(err).Str("control_id", controlID).Msg("upload evidence file")
		return errResp(c, http.StatusBadRequest, err.Error(), "CK_UPLOAD_FAILED")
	}
	return c.JSON(http.StatusCreated, ef)
}

// ListEvidenceFilesByControl handles GET /secvitals/controls/:id/evidence-files.
func (h *Handler) ListEvidenceFilesByControl(c echo.Context) error {
	if h.evidenceFiles == nil {
		return errResp(c, http.StatusServiceUnavailable, "evidence file service unavailable", "CK_SERVICE_UNAVAILABLE")
	}
	controlID := c.Param("id")
	items, err := h.evidenceFiles.ListForControl(c.Request().Context(), orgID(c), controlID)
	if err != nil {
		log.Error().Err(err).Str("control_id", controlID).Msg("list evidence files by control")
		return errResp(c, http.StatusInternalServerError, "failed to list evidence files", "CK_LIST_FAILED")
	}
	if items == nil {
		items = []EvidenceFile{}
	}
	return c.JSON(http.StatusOK, items)
}

// ListEvidenceFiles handles GET /secvitals/evidence/:eid/files.
func (h *Handler) ListEvidenceFiles(c echo.Context) error {
	if h.evidenceFiles == nil {
		return errResp(c, http.StatusServiceUnavailable, "evidence file service unavailable", "CK_SERVICE_UNAVAILABLE")
	}
	evidenceID := c.Param("eid")
	items, err := h.evidenceFiles.ListForEvidence(c.Request().Context(), orgID(c), evidenceID)
	if err != nil {
		log.Error().Err(err).Str("evidence_id", evidenceID).Msg("list evidence files")
		return errResp(c, http.StatusInternalServerError, "failed to list evidence files", "CK_LIST_FAILED")
	}
	if items == nil {
		items = []EvidenceFile{}
	}
	return c.JSON(http.StatusOK, items)
}

// DownloadEvidenceFile handles GET /secvitals/evidence-files/:fid/download.
// Streams the file to the client with Content-Disposition: attachment.
func (h *Handler) DownloadEvidenceFile(c echo.Context) error {
	if h.evidenceFiles == nil {
		return errResp(c, http.StatusServiceUnavailable, "evidence file service unavailable", "CK_SERVICE_UNAVAILABLE")
	}
	fileID := c.Param("fid")
	ef, diskPath, err := h.evidenceFiles.Download(c.Request().Context(), orgID(c), fileID)
	if err != nil {
		log.Error().Err(err).Str("file_id", fileID).Msg("download evidence file")
		return errResp(c, http.StatusNotFound, "file not found", "CK_NOT_FOUND")
	}
	safeName := filepath.Base(ef.OriginalName)
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", safeName))
	if ef.MimeType != "" {
		c.Response().Header().Set("Content-Type", ef.MimeType)
	}
	return c.File(diskPath)
}

// DeleteEvidenceFile handles DELETE /secvitals/evidence-files/:fid.
func (h *Handler) DeleteEvidenceFile(c echo.Context) error {
	if h.evidenceFiles == nil {
		return errResp(c, http.StatusServiceUnavailable, "evidence file service unavailable", "CK_SERVICE_UNAVAILABLE")
	}
	fileID := c.Param("fid")
	if err := h.evidenceFiles.Delete(c.Request().Context(), orgID(c), fileID); err != nil {
		log.Error().Err(err).Str("file_id", fileID).Msg("delete evidence file")
		return errResp(c, http.StatusInternalServerError, "failed to delete evidence file", "CK_DELETE_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// --- Control Review Cycles (Migration 075) ---

// RecordControlReview handles POST /secvitals/controls/:id/review.
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

// ListControlReviews handles GET /secvitals/controls/:id/reviews.
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

// ListOverdueControls handles GET /secvitals/controls/overdue-reviews.
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
