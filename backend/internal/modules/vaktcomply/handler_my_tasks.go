// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// MyTask represents a control or risk assigned to the current user.
type MyTask struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Type        string `json:"type"` // "control" or "risk"
	Status      string `json:"status"`
	FrameworkID string `json:"framework_id,omitempty"`
	RiskID      string `json:"risk_id,omitempty"`
}

// GetMyTasks handles GET /vaktcomply/my-tasks.
// Returns controls and risks where the authenticated user is the owner.
func (h *Handler) GetMyTasks(c echo.Context) error {
	ctx := c.Request().Context()
	uID := userID(c)
	oID := orgID(c)

	// Resolve current user's display_name.
	displayName, err := h.service.repo.GetUserDisplayName(ctx, uID)
	if err != nil {
		log.Error().Err(err).Str("user_id", uID).Msg("get my tasks: resolve display_name")
		return errResp(c, http.StatusInternalServerError, "failed to resolve user", "MY_TASKS_USER_ERROR")
	}

	// Controls where owner = display_name.
	ctrlTasks, err := h.service.repo.GetMyTaskControls(ctx, oID, displayName)
	if err != nil {
		log.Error().Err(err).Msg("get my tasks: controls")
		return errResp(c, http.StatusInternalServerError, "failed to list tasks", "MY_TASKS_ERROR")
	}

	// Risks where owner = display_name.
	riskTasks, err := h.service.repo.GetMyTaskRisks(ctx, oID, displayName)
	if err != nil {
		log.Error().Err(err).Msg("get my tasks: risks")
		return errResp(c, http.StatusInternalServerError, "failed to list tasks", "MY_TASKS_ERROR")
	}

	tasks := make([]MyTask, 0, len(ctrlTasks)+len(riskTasks))
	tasks = append(tasks, ctrlTasks...)
	tasks = append(tasks, riskTasks...)

	return c.JSON(http.StatusOK, tasks)
}
