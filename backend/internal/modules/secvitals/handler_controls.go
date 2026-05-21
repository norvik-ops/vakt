package secvitals

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/shared/audit"
)

// BulkUpdateControls handles PATCH /api/v1/secvitals/controls/bulk.
// Updates manual_status for multiple controls in a single request.
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
	return c.JSON(http.StatusOK, map[string]any{"mappings": mappings})
}

// GetControlChangelog handles GET /secvitals/controls/:id/changelog.
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

	// Snapshot old values for changelog comparison.
	oldCtrl, _ := h.service.GetControl(c.Request().Context(), orgID(c), c.Param("id"))

	ctrl, err := h.service.UpdateControl(c.Request().Context(), orgID(c), c.Param("id"), in)
	if err != nil {
		if strings.Contains(err.Error(), "maturity_score must be between") {
			return errResp(c, http.StatusUnprocessableEntity, err.Error(), "CK_VALIDATION_ERROR")
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

// GetScoreHistory handles GET /api/v1/secvitals/score-history?days=30
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
