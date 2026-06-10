// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S69-4: JML Mover Workflow HTTP handlers.

package vakthr

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// ListMoverEvents handles GET /api/v1/vakthr/mover-events.
func (h *Handler) ListMoverEvents(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	events, err := h.Service.ListMoverEvents(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Msg("list mover events")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list mover events", "code": "HR_INTERNAL"})
	}
	return c.JSON(http.StatusOK, events)
}

// CreateMoverEvent handles POST /api/v1/vakthr/mover-events.
func (h *Handler) CreateMoverEvent(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	userID, _ := c.Get("user_id").(string)

	var input CreateMoverEventInput
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request", "code": "HR_BAD_REQUEST"})
	}
	if err := h.validate.Struct(input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error(), "code": "HR_VALIDATION"})
	}

	ev, err := h.Service.CreateMoverEvent(c.Request().Context(), orgID, userID, input)
	if err != nil {
		log.Error().Err(err).Msg("create mover event")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create mover event", "code": "HR_INTERNAL"})
	}
	return c.JSON(http.StatusCreated, ev)
}

// GetMoverEvent handles GET /api/v1/vakthr/mover-events/:id.
func (h *Handler) GetMoverEvent(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	ev, err := h.Service.GetMoverEvent(c.Request().Context(), orgID, c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "mover event not found", "code": "HR_NOT_FOUND"})
	}
	return c.JSON(http.StatusOK, ev)
}

// UpdateMoverEventStatus handles PATCH /api/v1/vakthr/mover-events/:id/status.
func (h *Handler) UpdateMoverEventStatus(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)

	var input struct {
		Status string `json:"status" validate:"required,oneof=pending in_progress completed overdue cancelled"`
	}
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request", "code": "HR_BAD_REQUEST"})
	}
	if err := h.validate.Struct(input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error(), "code": "HR_VALIDATION"})
	}

	ev, err := h.Service.UpdateMoverEventStatus(c.Request().Context(), orgID, c.Param("id"), input.Status)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "mover event not found", "code": "HR_NOT_FOUND"})
	}
	return c.JSON(http.StatusOK, ev)
}

// ListMoverTemplates handles GET /api/v1/vakthr/mover-templates.
// Seeds the default template on first access if none exist.
func (h *Handler) ListMoverTemplates(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	ctx := c.Request().Context()
	if err := h.Service.EnsureDefaultMoverTemplate(ctx, orgID); err != nil {
		log.Warn().Err(err).Msg("ensure default mover template")
	}
	templates, err := h.Service.ListMoverTemplates(ctx, orgID)
	if err != nil {
		log.Error().Err(err).Msg("list mover templates")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list templates", "code": "HR_INTERNAL"})
	}
	return c.JSON(http.StatusOK, templates)
}
