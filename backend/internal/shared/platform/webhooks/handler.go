// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package webhooks

import (
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/auth"
	"github.com/matharnica/vakt/internal/shared/httputil"
)

// Handler holds HTTP handler methods for outgoing webhook endpoints.
type Handler struct {
	svc      *WebhookService
	validate *validator.Validate
}

// NewHandler constructs a webhook Handler.
func NewHandler(svc *WebhookService) *Handler {
	return &Handler{svc: svc, validate: validator.New()}
}

// Register mounts webhook CRUD routes under g.
// All routes expect an authenticated echo.Context (org_id set by AuthMiddleware).
func Register(g *echo.Group, h *Handler) {
	rw := auth.RequireRole("Admin", "SecurityAnalyst")
	admin := auth.RequireRole("Admin")

	g.GET("", h.List, rw)
	g.POST("", h.Create, admin)
	g.PUT("/:id", h.Update, admin)
	g.DELETE("/:id", h.Delete, admin)
	g.POST("/:id/test", h.Test, rw)
}

// List handles GET /api/v1/webhooks.
func (h *Handler) List(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)

	hooks, err := h.svc.ListWebhooks(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("list webhooks failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to list webhooks",
			"code":  "WEBHOOK_LIST_ERROR",
		})
	}

	if hooks == nil {
		hooks = []Webhook{}
	}
	return c.JSON(http.StatusOK, map[string]any{"data": hooks})
}

// Create handles POST /api/v1/webhooks.
func (h *Handler) Create(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)

	var input CreateWebhookInput
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
			"code":  "WEBHOOK_BAD_REQUEST",
		})
	}
	if err := h.validate.Struct(input); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": httputil.HumanValidationError(err),
			"code":  "WEBHOOK_VALIDATION_ERROR",
		})
	}

	wh, err := h.svc.CreateWebhook(c.Request().Context(), orgID, input)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("create webhook failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to create webhook",
			"code":  "WEBHOOK_CREATE_ERROR",
		})
	}
	return c.JSON(http.StatusCreated, wh)
}

// Update handles PUT /api/v1/webhooks/:id.
func (h *Handler) Update(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	id := c.Param("id")

	var input UpdateWebhookInput
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
			"code":  "WEBHOOK_BAD_REQUEST",
		})
	}
	if err := h.validate.Struct(input); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": httputil.HumanValidationError(err),
			"code":  "WEBHOOK_VALIDATION_ERROR",
		})
	}

	wh, err := h.svc.UpdateWebhook(c.Request().Context(), id, orgID, input)
	if err != nil {
		log.Error().Err(err).Str("webhook_id", id).Msg("update webhook failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to update webhook",
			"code":  "WEBHOOK_UPDATE_ERROR",
		})
	}
	return c.JSON(http.StatusOK, wh)
}

// Delete handles DELETE /api/v1/webhooks/:id.
func (h *Handler) Delete(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	id := c.Param("id")

	if err := h.svc.DeleteWebhook(c.Request().Context(), id, orgID); err != nil {
		if err.Error() == "webhook not found" {
			return c.JSON(http.StatusNotFound, map[string]string{
				"error": "webhook not found",
				"code":  "WEBHOOK_NOT_FOUND",
			})
		}
		log.Error().Err(err).Str("webhook_id", id).Msg("delete webhook failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to delete webhook",
			"code":  "WEBHOOK_DELETE_ERROR",
		})
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "webhook deleted"})
}

// Test handles POST /api/v1/webhooks/:id/test.
// It sends a test ping to the configured URL and returns the HTTP status code received.
func (h *Handler) Test(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	id := c.Param("id")

	statusCode, err := h.svc.TestWebhook(c.Request().Context(), id, orgID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "webhook not found",
			"code":  "WEBHOOK_NOT_FOUND",
		})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"status_code": statusCode,
		"message":     "test ping delivered",
	})
}
