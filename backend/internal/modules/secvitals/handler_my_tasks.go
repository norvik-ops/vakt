// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package secvitals

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

// GetMyTasks handles GET /secvitals/my-tasks.
// Returns controls and risks where the authenticated user is the owner.
func (h *Handler) GetMyTasks(c echo.Context) error {
	uID := userID(c)
	oID := orgID(c)

	// Resolve current user's display_name.
	var displayName string
	err := h.db.QueryRow(c.Request().Context(),
		`SELECT COALESCE(display_name, email) FROM users WHERE id = $1::uuid`, uID,
	).Scan(&displayName)
	if err != nil {
		log.Error().Err(err).Str("user_id", uID).Msg("get my tasks: resolve display_name")
		return errResp(c, http.StatusInternalServerError, "failed to resolve user", "MY_TASKS_USER_ERROR")
	}

	var tasks []MyTask

	// Controls where owner = display_name.
	ctrlRows, err := h.db.Query(c.Request().Context(), `
		SELECT id::text, title, COALESCE(manual_status, ''), framework_id::text
		FROM ck_controls
		WHERE org_id = $1::uuid
		  AND owner = $2
		  AND NOT not_applicable
		ORDER BY control_id ASC
		LIMIT 50`, oID, displayName)
	if err != nil {
		log.Error().Err(err).Msg("get my tasks: controls")
		return errResp(c, http.StatusInternalServerError, "failed to list tasks", "MY_TASKS_ERROR")
	}
	defer ctrlRows.Close()
	for ctrlRows.Next() {
		var t MyTask
		var frameworkID string
		if err := ctrlRows.Scan(&t.ID, &t.Title, &t.Status, &frameworkID); err != nil {
			continue
		}
		t.Type = "control"
		t.FrameworkID = frameworkID
		tasks = append(tasks, t)
	}

	// Risks where owner = display_name.
	riskRows, err := h.db.Query(c.Request().Context(), `
		SELECT id::text, title, status
		FROM ck_risks
		WHERE org_id = $1::uuid
		  AND owner = $2
		  AND status NOT IN ('accepted', 'resolved')
		ORDER BY created_at DESC
		LIMIT 50`, oID, displayName)
	if err != nil {
		log.Error().Err(err).Msg("get my tasks: risks")
		return errResp(c, http.StatusInternalServerError, "failed to list tasks", "MY_TASKS_ERROR")
	}
	defer riskRows.Close()
	for riskRows.Next() {
		var t MyTask
		if err := riskRows.Scan(&t.ID, &t.Title, &t.Status); err != nil {
			continue
		}
		t.Type = "risk"
		tasks = append(tasks, t)
	}

	if tasks == nil {
		tasks = []MyTask{}
	}
	return c.JSON(http.StatusOK, tasks)
}
