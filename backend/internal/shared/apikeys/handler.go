// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package apikeys

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// Handler holds HTTP handler methods for API key endpoints.
type Handler struct {
	service  *Service
	validate *validator.Validate
}

// NewHandler constructs a Handler backed by the given service.
func NewHandler(svc *Service) *Handler {
	return &Handler{
		service:  svc,
		validate: validator.New(),
	}
}

// createRequest is the JSON body accepted by POST /api-keys.
type createRequest struct {
	Name      string   `json:"name"       validate:"required,min=1,max=100"`
	ExpiresAt *string  `json:"expires_at"` // optional RFC3339
	Scopes    []string `json:"scopes"`
}

// CreateKey handles POST /api-keys.
// The raw key is returned exactly once in the response — it is never stored.
func (h *Handler) CreateKey(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	userID, _ := c.Get("user_id").(string)

	var req createRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
			"code":  "APIKEYS_INVALID_INPUT",
		})
	}
	if err := h.validate.Struct(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": err.Error(),
			"code":  "APIKEYS_VALIDATION_ERROR",
		})
	}

	input := CreateInput{
		Name:   req.Name,
		Scopes: req.Scopes,
	}
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "expires_at must be RFC3339",
				"code":  "APIKEYS_INVALID_EXPIRY",
			})
		}
		input.ExpiresAt = &t
	}

	result, err := h.service.Create(c.Request().Context(), orgID, userID, input)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("create api key failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to create API key",
			"code":  "APIKEYS_CREATE_ERROR",
		})
	}

	return c.JSON(http.StatusCreated, result)
}

// ListKeys handles GET /api-keys.
// Raw keys are never returned — only prefix, metadata.
func (h *Handler) ListKeys(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	userID, _ := c.Get("user_id").(string)

	keys, err := h.service.List(c.Request().Context(), orgID, userID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("list api keys failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to list API keys",
			"code":  "APIKEYS_LIST_ERROR",
		})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"data": keys,
	})
}

// RevokeKey handles DELETE /api-keys/:id.
func (h *Handler) RevokeKey(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	userID, _ := c.Get("user_id").(string)
	keyID := c.Param("id")

	err := h.service.Revoke(c.Request().Context(), orgID, userID, keyID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{
				"error": "API key not found",
				"code":  "APIKEYS_NOT_FOUND",
			})
		}
		log.Error().Err(err).Str("org_id", orgID).Str("key_id", keyID).Msg("revoke api key failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to revoke API key",
			"code":  "APIKEYS_REVOKE_ERROR",
		})
	}

	return c.NoContent(http.StatusNoContent)
}

// RotateKey handles POST /api-keys/:id/rotate.
// Sprint 20 S20-2: rotates the key with a 24h grace-period where both
// old + new hash are valid. Returns the new raw key (one-shot).
func (h *Handler) RotateKey(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	keyID := c.Param("id")
	res, err := h.service.RotateKey(c.Request().Context(), orgID, keyID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Str("key_id", keyID).Msg("rotate api key failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to rotate API key",
			"code":  "APIKEYS_ROTATE_ERROR",
		})
	}
	return c.JSON(http.StatusOK, res)
}
