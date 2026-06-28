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

func (h *Handler) ListBackupJobs(c echo.Context) error {
	jobs, err := h.service.ListBackupJobs(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list backup jobs")
		return errResp(c, http.StatusInternalServerError, "failed to list backup jobs", "CK_BACKUP_LIST_FAILED")
	}
	return c.JSON(http.StatusOK, jobs)
}

// GetBackupSummary handles GET /api/v1/vaktcomply/backup/summary
func (h *Handler) GetBackupSummary(c echo.Context) error {
	sum, err := h.service.GetBackupSummary(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("backup summary")
		return errResp(c, http.StatusInternalServerError, "failed to compute backup summary", "CK_BACKUP_SUMMARY_FAILED")
	}
	return c.JSON(http.StatusOK, sum)
}

// CreateBackupJob handles POST /api/v1/vaktcomply/backup/jobs
func (h *Handler) CreateBackupJob(c echo.Context) error {
	var in BackupJobInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, err.Error(), "CK_VALIDATION_ERROR")
	}
	job, err := h.service.CreateBackupJob(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create backup job")
		return errResp(c, http.StatusInternalServerError, "failed to create backup job", "CK_BACKUP_CREATE_FAILED")
	}
	return c.JSON(http.StatusCreated, job)
}

// UpdateBackupJob handles PUT /api/v1/vaktcomply/backup/jobs/:id
func (h *Handler) UpdateBackupJob(c echo.Context) error {
	var in BackupJobInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, err.Error(), "CK_VALIDATION_ERROR")
	}
	job, err := h.service.UpdateBackupJob(c.Request().Context(), orgID(c), c.Param("id"), in)
	if err != nil {
		if err.Error() == "backup job not found" {
			return errResp(c, http.StatusNotFound, "backup job not found", "CK_BACKUP_NOT_FOUND")
		}
		log.Error().Err(err).Msg("update backup job")
		return errResp(c, http.StatusInternalServerError, "failed to update backup job", "CK_BACKUP_UPDATE_FAILED")
	}
	return c.JSON(http.StatusOK, job)
}

// DeleteBackupJob handles DELETE /api/v1/vaktcomply/backup/jobs/:id
func (h *Handler) DeleteBackupJob(c echo.Context) error {
	if err := h.service.DeleteBackupJob(c.Request().Context(), orgID(c), c.Param("id")); err != nil {
		if err.Error() == "backup job not found" {
			return errResp(c, http.StatusNotFound, "backup job not found", "CK_BACKUP_NOT_FOUND")
		}
		log.Error().Err(err).Msg("delete backup job")
		return errResp(c, http.StatusInternalServerError, "failed to delete backup job", "CK_BACKUP_DELETE_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// ListRestoreTests handles GET /api/v1/vaktcomply/backup/jobs/:id/restore-tests
func (h *Handler) ListRestoreTests(c echo.Context) error {
	tests, err := h.service.ListRestoreTests(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		log.Error().Err(err).Msg("list restore tests")
		return errResp(c, http.StatusInternalServerError, "failed to list restore tests", "CK_RESTORE_LIST_FAILED")
	}
	return c.JSON(http.StatusOK, tests)
}

// CreateRestoreTest handles POST /api/v1/vaktcomply/backup/jobs/:id/restore-tests
func (h *Handler) CreateRestoreTest(c echo.Context) error {
	var in RestoreTestInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, err.Error(), "CK_VALIDATION_ERROR")
	}
	test, err := h.service.CreateRestoreTest(c.Request().Context(), orgID(c), c.Param("id"), in)
	if err != nil {
		if err.Error() == "backup job not found" {
			return errResp(c, http.StatusNotFound, "backup job not found", "CK_BACKUP_NOT_FOUND")
		}
		log.Error().Err(err).Msg("create restore test")
		return errResp(c, http.StatusInternalServerError, "failed to create restore test", "CK_RESTORE_CREATE_FAILED")
	}
	return c.JSON(http.StatusCreated, test)
}

func (h *Handler) ListControlVVTLinks(c echo.Context) error {
	links, err := h.service.ListLinksForControl(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		log.Error().Err(err).Msg("list control vvt links")
		return errResp(c, http.StatusInternalServerError, "failed to list VVT links", "CK_VVT_LINKS_FAILED")
	}
	return c.JSON(http.StatusOK, links)
}

// ListVVTControlLinks handles GET /api/v1/vaktcomply/vvt-links?vvt_id=...
func (h *Handler) ListVVTControlLinks(c echo.Context) error {
	vvtID := c.QueryParam("vvt_id")
	if vvtID == "" {
		return errResp(c, http.StatusBadRequest, "vvt_id query param required", "CK_BAD_REQUEST")
	}
	links, err := h.service.ListLinksForVVT(c.Request().Context(), orgID(c), vvtID)
	if err != nil {
		log.Error().Err(err).Msg("list vvt control links")
		return errResp(c, http.StatusInternalServerError, "failed to list control links", "CK_VVT_LINKS_FAILED")
	}
	return c.JSON(http.StatusOK, links)
}

// CreateVVTControlLink handles POST /api/v1/vaktcomply/vvt-links
func (h *Handler) CreateVVTControlLink(c echo.Context) error {
	var in LinkVVTToControlInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, err.Error(), "CK_VALIDATION_ERROR")
	}
	link, err := h.service.LinkVVTToControl(c.Request().Context(), orgID(c), in)
	if err != nil {
		if err.Error() == "control not found" {
			return errResp(c, http.StatusNotFound, "control not found", "CK_CONTROL_NOT_FOUND")
		}
		log.Error().Err(err).Msg("create vvt control link")
		return errResp(c, http.StatusInternalServerError, "failed to link VVT", "CK_VVT_LINK_FAILED")
	}
	return c.JSON(http.StatusCreated, link)
}

// DeleteVVTControlLink handles DELETE /api/v1/vaktcomply/vvt-links/:id
func (h *Handler) DeleteVVTControlLink(c echo.Context) error {
	if err := h.service.UnlinkVVTFromControl(c.Request().Context(), orgID(c), c.Param("id")); err != nil {
		if err.Error() == "link not found" {
			return errResp(c, http.StatusNotFound, "link not found", "CK_VVT_LINK_NOT_FOUND")
		}
		log.Error().Err(err).Msg("delete vvt control link")
		return errResp(c, http.StatusInternalServerError, "failed to remove link", "CK_VVT_UNLINK_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

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

// UpdateCollabTask handles PATCH /vaktcomply/collab-tasks/:tid.
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

// DeleteCollabTask handles DELETE /vaktcomply/collab-tasks/:tid.
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

// DeleteComment handles DELETE /vaktcomply/comments/:cid.
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
