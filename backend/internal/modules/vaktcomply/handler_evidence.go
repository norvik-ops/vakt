package vaktcomply

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// UploadEvidence handles POST /api/v1/vaktcomply/controls/:id/evidence/upload.
// Deprecated: use POST /controls/:id/evidence-files (UploadEvidenceFile) instead,
// which routes through EvidenceFileService with proper MIME validation and
// a separate ck_evidence_files table. This legacy route is retained for
// backwards compatibility and will be removed in a future release.
func (h *Handler) UploadEvidence(c echo.Context) error {
	c.Response().Header().Set("Deprecation", "true")
	c.Response().Header().Set("Link", `</api/v1/vaktcomply/controls/{id}/evidence-files>; rel="successor-version"`)

	controlID := c.Param("id")

	title := c.FormValue("title")
	if title == "" {
		return errResp(c, http.StatusBadRequest, "title is required", "CK_BAD_REQUEST")
	}
	notes := c.FormValue("notes")

	// Parse optional expires_at from form field (RFC3339 or YYYY-MM-DD).
	var expiresAt *time.Time
	if expiresAtStr := c.FormValue("expires_at"); expiresAtStr != "" {
		var t time.Time
		var parseErr error
		if t, parseErr = time.Parse(time.RFC3339, expiresAtStr); parseErr != nil {
			if t, parseErr = time.Parse("2006-01-02", expiresAtStr); parseErr != nil {
				return errResp(c, http.StatusBadRequest, "invalid expires_at format, use RFC3339 or YYYY-MM-DD", "CK_BAD_REQUEST")
			}
		}
		t = t.UTC()
		expiresAt = &t
	}

	fh, err := c.FormFile("file")
	if err != nil {
		return errResp(c, http.StatusBadRequest, "file is required", "CK_BAD_REQUEST")
	}

	allowed := map[string]bool{
		".pdf": true, ".png": true, ".jpg": true, ".jpeg": true,
		".gif": true, ".webp": true, ".txt": true, ".csv": true,
		".xlsx": true, ".docx": true, ".zip": true,
	}
	ext := strings.ToLower(filepath.Ext(fh.Filename))
	if !allowed[ext] {
		return echo.NewHTTPError(http.StatusBadRequest, "Dateityp nicht erlaubt")
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
		ExpiresAt:   expiresAt,
	}
	ev, err := h.service.AddEvidence(c.Request().Context(), orgID(c), controlID, userID(c), input)
	if err != nil {
		_ = os.Remove(destPath)
		log.Error().Err(err).Str("control_id", controlID).Msg("upload evidence")
		return errResp(c, http.StatusInternalServerError, "failed to add evidence", "CK_ADD_EVIDENCE_FAILED")
	}
	return c.JSON(http.StatusCreated, ev)
}

// AddEvidence handles POST /api/v1/vaktcomply/controls/:id/evidence.
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

// ListEvidence handles GET /api/v1/vaktcomply/controls/:id/evidence.
func (h *Handler) ListEvidence(c echo.Context) error {
	controlID := c.Param("id")
	items, err := h.service.ListEvidence(c.Request().Context(), orgID(c), controlID)
	if err != nil {
		log.Error().Err(err).Str("control_id", controlID).Msg("list evidence")
		return errResp(c, http.StatusInternalServerError, "failed to list evidence", "CK_LIST_EVIDENCE_FAILED")
	}
	return c.JSON(http.StatusOK, items)
}

// ReviewEvidence handles POST /api/v1/vaktcomply/evidence/:id/review.
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

// CollectEvidence handles POST /api/v1/vaktcomply/controls/:id/collect.
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

// ExportEvidenceBundle handles GET /api/v1/vaktcomply/controls/:id/export.
// Returns a ZIP archive containing the control metadata and all evidence items as JSON.
func (h *Handler) ExportEvidenceBundle(c echo.Context) error {
	controlID := c.Param("id")
	ctx := c.Request().Context()
	org := orgID(c)

	ctrl, err := h.service.Policy.GetControl(ctx, org, controlID)
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

// GetEvidenceHistory handles GET /api/v1/vaktcomply/evidence/:id/history.
// Returns the audit history for a single evidence item, newest-first.
func (h *Handler) GetEvidenceHistory(c echo.Context) error {
	evidenceID := c.Param("id")
	items, err := h.service.GetEvidenceHistory(c.Request().Context(), orgID(c), evidenceID)
	if err != nil {
		log.Error().Err(err).Str("evidence_id", evidenceID).Msg("get evidence history")
		return errResp(c, http.StatusInternalServerError, "failed to get evidence history", "CK_EVIDENCE_HISTORY_FAILED")
	}
	return c.JSON(http.StatusOK, items)
}

// GetExpiringEvidence handles GET /api/v1/vaktcomply/evidence/expiring.
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

// WithEvidenceFileService attaches the EvidenceFileService to the handler.
func (h *Handler) WithEvidenceFileService(s *EvidenceFileService) *Handler {
	h.evidenceFiles = s
	return h
}

// UploadEvidenceFile handles POST /vaktcomply/controls/:id/evidence-files.
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

// ListEvidenceFilesByControl handles GET /vaktcomply/controls/:id/evidence-files.
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

// ListEvidenceFiles handles GET /vaktcomply/evidence/:eid/files.
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

// DownloadEvidenceFile handles GET /vaktcomply/evidence-files/:fid/download.
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

// DeleteEvidenceFile handles DELETE /vaktcomply/evidence-files/:fid.
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
