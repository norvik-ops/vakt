// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S69-3: SLA Policy management endpoints.

package vaktscan

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// ListSLAPolicies handles GET /api/v1/vaktscan/sla-policies.
// Seeds DACH defaults on first access if no policies exist yet.
func (h *Handler) ListSLAPolicies(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	ctx := c.Request().Context()
	if err := h.service.EnsureDefaultSLAPolicies(ctx, orgID); err != nil {
		// Non-fatal: log and continue; worst case the list is empty.
		log.Warn().Err(err).Msg("ensure default SLA policies")
	}
	policies, err := h.service.repo.ListSLAPolicies(ctx, orgID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to list SLA policies",
			"code":  "VB_INTERNAL",
		})
	}
	return c.JSON(http.StatusOK, policies)
}

// UpsertSLAPolicy handles PUT /api/v1/vaktscan/sla-policies/:severity.
func (h *Handler) UpsertSLAPolicy(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	severity := c.Param("severity")

	var input struct {
		RemediationDays         int `json:"remediation_days" validate:"required,min=1,max=3650"`
		NotificationAdvanceDays int `json:"notification_advance_days" validate:"min=0,max=30"`
	}
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request", "code": "VB_BAD_REQUEST"})
	}
	if err := c.Validate(input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error(), "code": "VB_VALIDATION"})
	}

	if err := h.service.repo.UpsertSLAPolicy(c.Request().Context(), orgID, severity,
		input.RemediationDays, input.NotificationAdvanceDays); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to save policy", "code": "VB_INTERNAL"})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "updated"})
}

// ResetSLAPolicies handles POST /api/v1/vaktscan/sla-policies/reset.
// Deletes org-specific policies and recreates DACH defaults.
func (h *Handler) ResetSLAPolicies(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	ctx := c.Request().Context()

	if _, err := h.service.repo.DB().Exec(ctx,
		`DELETE FROM vb_sla_policies WHERE org_id = $1::uuid`, orgID,
	); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "reset failed", "code": "VB_INTERNAL"})
	}
	if err := h.service.EnsureDefaultSLAPolicies(ctx, orgID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "recreate defaults failed", "code": "VB_INTERNAL"})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "reset"})
}

// GetSLASummary handles GET /api/v1/vaktscan/sla/summary.
func (h *Handler) GetSLASummary(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	summary, err := h.service.GetSLASummary(c.Request().Context(), orgID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to get SLA summary", "code": "VB_INTERNAL"})
	}
	return c.JSON(http.StatusOK, summary)
}
