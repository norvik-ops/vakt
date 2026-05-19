package secvitals

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

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
