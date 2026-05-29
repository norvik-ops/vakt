// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// ListMilestones handles GET /api/v1/vaktcomply/milestones
// Query param: ?status=upcoming|completed|missed|cancelled
func (h *Handler) ListMilestones(c echo.Context) error {
	statusFilter := c.QueryParam("status")
	milestones, err := h.service.repo.ListMilestones(c.Request().Context(), orgID(c), statusFilter)
	if err != nil {
		log.Error().Err(err).Msg("list milestones")
		return errResp(c, http.StatusInternalServerError, "failed to list milestones", "CK_LIST_MILESTONES_FAILED")
	}
	if milestones == nil {
		milestones = []AuditMilestone{}
	}
	return c.JSON(http.StatusOK, milestones)
}

// CreateMilestone handles POST /api/v1/vaktcomply/milestones
func (h *Handler) CreateMilestone(c echo.Context) error {
	var in CreateMilestoneInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_INVALID_INPUT")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusBadRequest, err.Error(), "CK_VALIDATION_FAILED")
	}
	m, err := h.service.repo.CreateMilestone(c.Request().Context(), orgID(c), userID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create milestone")
		return errResp(c, http.StatusInternalServerError, "failed to create milestone", "CK_CREATE_MILESTONE_FAILED")
	}
	return c.JSON(http.StatusCreated, m)
}

// UpdateMilestone handles PUT /api/v1/vaktcomply/milestones/:id
func (h *Handler) UpdateMilestone(c echo.Context) error {
	id := c.Param("id")
	var in UpdateMilestoneInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_INVALID_INPUT")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusBadRequest, err.Error(), "CK_VALIDATION_FAILED")
	}
	m, err := h.service.repo.UpdateMilestone(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || err.Error() == "milestone not found" {
			return errResp(c, http.StatusNotFound, "milestone not found", "CK_MILESTONE_NOT_FOUND")
		}
		log.Error().Err(err).Msg("update milestone")
		return errResp(c, http.StatusInternalServerError, "failed to update milestone", "CK_UPDATE_MILESTONE_FAILED")
	}
	return c.JSON(http.StatusOK, m)
}

// DeleteMilestone handles DELETE /api/v1/vaktcomply/milestones/:id
func (h *Handler) DeleteMilestone(c echo.Context) error {
	id := c.Param("id")
	if err := h.service.repo.DeleteMilestone(c.Request().Context(), orgID(c), id); err != nil {
		if err.Error() == "milestone not found" {
			return errResp(c, http.StatusNotFound, "milestone not found", "CK_MILESTONE_NOT_FOUND")
		}
		log.Error().Err(err).Msg("delete milestone")
		return errResp(c, http.StatusInternalServerError, "failed to delete milestone", "CK_DELETE_MILESTONE_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// GetNextMilestone handles GET /api/v1/vaktcomply/milestones/next
// Returns the nearest upcoming milestone (for dashboard widget).
func (h *Handler) GetNextMilestone(c echo.Context) error {
	m, err := h.service.repo.NextMilestone(c.Request().Context(), orgID(c))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return c.JSON(http.StatusOK, nil)
		}
		log.Error().Err(err).Msg("get next milestone")
		return errResp(c, http.StatusInternalServerError, "failed to get next milestone", "CK_NEXT_MILESTONE_FAILED")
	}
	return c.JSON(http.StatusOK, m)
}
