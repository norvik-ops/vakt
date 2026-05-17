// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package jira

import (
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

var validate = validator.New()

// Handler handles HTTP requests for Jira integration.
type Handler struct {
	svc *Service
}

// NewHandler creates a new Jira integration handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes wires Jira integration routes under the provided echo group.
// Expected group prefix: /integrations/jira
func RegisterRoutes(g *echo.Group, db *pgxpool.Pool, masterKey []byte) {
	svc := NewService(db, masterKey)
	h := NewHandler(svc)

	g.GET("/config", h.GetConfig)
	g.PUT("/config", h.SaveConfig)
	g.POST("/test", h.TestConnection)
	g.POST("/findings/:id/create-issue", h.CreateIssue)
}

// GetConfig returns the current Jira config (api token masked).
func (h *Handler) GetConfig(c echo.Context) error {
	orgID := mustString(c, "org_id")
	cfg, err := h.svc.GetConfig(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Msg("GetJiraConfig failed")
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, cfg)
}

// SaveConfig persists the Jira configuration (encrypts the api token).
func (h *Handler) SaveConfig(c echo.Context) error {
	orgID := mustString(c, "org_id")

	var in SaveConfigInput
	if err := c.Bind(&in); err != nil {
		return badRequest(c, "invalid request body")
	}
	if err := validate.Struct(in); err != nil {
		return validationError(c, err)
	}

	if err := h.svc.SaveConfig(c.Request().Context(), orgID, in); err != nil {
		log.Error().Err(err).Msg("SaveJiraConfig failed")
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// TestConnection verifies the configured Jira credentials.
func (h *Handler) TestConnection(c echo.Context) error {
	orgID := mustString(c, "org_id")
	result, err := h.svc.TestConnection(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Msg("TestJiraConnection error")
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, result)
}

// CreateIssue creates a Jira issue for the given finding ID.
func (h *Handler) CreateIssue(c echo.Context) error {
	orgID := mustString(c, "org_id")
	findingID := c.Param("id")
	if findingID == "" {
		return badRequest(c, "finding id is required")
	}

	result, err := h.svc.CreateIssueForFinding(c.Request().Context(), orgID, findingID)
	if err != nil {
		log.Error().Err(err).Str("finding_id", findingID).Msg("CreateJiraIssue failed")
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": err.Error(),
			"code":  "JIRA_ERROR",
		})
	}
	return c.JSON(http.StatusCreated, result)
}

// --- helpers ---

func mustString(c echo.Context, key string) string {
	v, _ := c.Get(key).(string)
	return v
}

func badRequest(c echo.Context, msg string) error {
	return c.JSON(http.StatusBadRequest, map[string]string{"error": msg, "code": "BAD_REQUEST"})
}

func serverError(c echo.Context, err error) error {
	log.Error().Err(err).Msg("jira operation failed")
	return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error", "code": "INTERNAL_ERROR"})
}

func validationError(c echo.Context, err error) error {
	log.Error().Err(err).Msg("jira validation error")
	return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "validation error", "code": "VALIDATION_ERROR"})
}
