// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package cloud

import (
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

var validate = validator.New()

// Handler handles HTTP requests for cloud integrations (AWS + Azure).
type Handler struct {
	svc *Service
}

// NewHandler creates a new cloud integration handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes wires cloud integration routes under the provided echo group.
// Expected group prefix: /integrations/cloud
func RegisterRoutes(g *echo.Group, db *pgxpool.Pool, masterKey []byte, evidence EvidenceWriter) {
	svc := NewService(db, masterKey, evidence)
	h := NewHandler(svc)

	// AWS
	aws := g.Group("/aws")
	aws.GET("/config", h.GetAWSConfig)
	aws.PUT("/config", h.SaveAWSConfig)
	aws.POST("/test", h.TestAWSConnection)
	aws.POST("/sync", h.SyncAWS)
	aws.GET("/status", h.GetAWSStatus)
	aws.GET("/evidence", h.GetAWSEvidence)

	// Azure
	az := g.Group("/azure")
	az.GET("/config", h.GetAzureConfig)
	az.PUT("/config", h.SaveAzureConfig)
	az.POST("/test", h.TestAzureConnection)
	az.POST("/sync", h.SyncAzure)
	az.GET("/status", h.GetAzureStatus)
	az.GET("/evidence", h.GetAzureEvidence)
}

// --- AWS handlers ---

// GetAWSConfig returns the AWS config with secrets masked.
func (h *Handler) GetAWSConfig(c echo.Context) error {
	orgID := mustString(c, "org_id")
	cfg, err := h.svc.GetAWSConfig(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Msg("GetAWSConfig failed")
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, cfg)
}

// SaveAWSConfig persists the AWS configuration.
func (h *Handler) SaveAWSConfig(c echo.Context) error {
	orgID := mustString(c, "org_id")

	var in SaveAWSConfigInput
	if err := c.Bind(&in); err != nil {
		return badRequest(c, "invalid request body")
	}
	if err := validate.Struct(in); err != nil {
		return validationError(c, err)
	}

	if err := h.svc.SaveAWSConfig(c.Request().Context(), orgID, in); err != nil {
		log.Error().Err(err).Msg("SaveAWSConfig failed")
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// TestAWSConnection tests the configured AWS credentials.
func (h *Handler) TestAWSConnection(c echo.Context) error {
	orgID := mustString(c, "org_id")
	if err := h.svc.TestAWSConnection(c.Request().Context(), orgID); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("TestAWSConnection failed")
		return c.JSON(http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
}

// SyncAWS triggers an immediate AWS evidence collection run.
func (h *Handler) SyncAWS(c echo.Context) error {
	orgID := mustString(c, "org_id")
	count, err := h.svc.SyncAWS(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("SyncAWS failed")
		return c.JSON(http.StatusOK, map[string]any{"ok": false, "error": err.Error(), "evidence_created": 0})
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true, "evidence_created": count})
}

// GetAWSStatus returns the last sync status and evidence count for AWS.
func (h *Handler) GetAWSStatus(c echo.Context) error {
	orgID := mustString(c, "org_id")
	st, err := h.svc.GetAWSStatus(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Msg("GetAWSStatus failed")
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, st)
}

// GetAWSEvidence returns the 5 most recent AWS evidence items.
func (h *Handler) GetAWSEvidence(c echo.Context) error {
	orgID := mustString(c, "org_id")
	items, err := h.svc.RecentEvidence(c.Request().Context(), orgID, "aws")
	if err != nil {
		log.Error().Err(err).Msg("GetAWSEvidence failed")
		return serverError(c, err)
	}
	if items == nil {
		items = []EvidenceItem{}
	}
	return c.JSON(http.StatusOK, items)
}

// --- Azure handlers ---

// GetAzureConfig returns the Azure config with secrets masked.
func (h *Handler) GetAzureConfig(c echo.Context) error {
	orgID := mustString(c, "org_id")
	cfg, err := h.svc.GetAzureConfig(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Msg("GetAzureConfig failed")
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, cfg)
}

// SaveAzureConfig persists the Azure configuration.
func (h *Handler) SaveAzureConfig(c echo.Context) error {
	orgID := mustString(c, "org_id")

	var in SaveAzureConfigInput
	if err := c.Bind(&in); err != nil {
		return badRequest(c, "invalid request body")
	}
	if err := validate.Struct(in); err != nil {
		return validationError(c, err)
	}

	if err := h.svc.SaveAzureConfig(c.Request().Context(), orgID, in); err != nil {
		log.Error().Err(err).Msg("SaveAzureConfig failed")
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// TestAzureConnection tests the configured Azure credentials.
func (h *Handler) TestAzureConnection(c echo.Context) error {
	orgID := mustString(c, "org_id")
	if err := h.svc.TestAzureConnection(c.Request().Context(), orgID); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("TestAzureConnection failed")
		return c.JSON(http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
}

// SyncAzure triggers an immediate Azure evidence collection run.
func (h *Handler) SyncAzure(c echo.Context) error {
	orgID := mustString(c, "org_id")
	count, err := h.svc.SyncAzure(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("SyncAzure failed")
		return c.JSON(http.StatusOK, map[string]any{"ok": false, "error": err.Error(), "evidence_created": 0})
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true, "evidence_created": count})
}

// GetAzureStatus returns the last sync status and evidence count for Azure.
func (h *Handler) GetAzureStatus(c echo.Context) error {
	orgID := mustString(c, "org_id")
	st, err := h.svc.GetAzureStatus(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Msg("GetAzureStatus failed")
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, st)
}

// GetAzureEvidence returns the 5 most recent Azure evidence items.
func (h *Handler) GetAzureEvidence(c echo.Context) error {
	orgID := mustString(c, "org_id")
	items, err := h.svc.RecentEvidence(c.Request().Context(), orgID, "azure")
	if err != nil {
		log.Error().Err(err).Msg("GetAzureEvidence failed")
		return serverError(c, err)
	}
	if items == nil {
		items = []EvidenceItem{}
	}
	return c.JSON(http.StatusOK, items)
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
	log.Error().Err(err).Msg("cloud integration operation failed")
	return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error", "code": "INTERNAL_ERROR"})
}

func validationError(c echo.Context, err error) error {
	return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "validation error", "code": "VALIDATION_ERROR"})
}
