// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Package dashboard aggregates cross-module metrics into a single security
// score and manages in-app notifications stored in the user_notifications
// table. It queries SecPulse findings, SecPrivacy breaches, and SecVitals
// frameworks directly via raw SQL so it remains decoupled from each module's
// service layer.
package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

// Handler holds the service and optional Redis client used for aggregate caching.
type Handler struct {
	svc *Service
	rdb *redis.Client // may be nil — caching is skipped when absent
}

// NewHandler wires up a Handler backed by the provided service and optional Redis client.
// Pass nil for rdb to disable aggregate caching.
func NewHandler(svc *Service, rdb *redis.Client) *Handler {
	return &Handler{svc: svc, rdb: rdb}
}

// ScoreConfig holds the configurable weights and caps for the security score formula.
type ScoreConfig struct {
	BaseScore        int `json:"base_score"`
	CritPenalty      int `json:"crit_penalty"`
	CritPenaltyCap   int `json:"crit_penalty_cap"`
	HighPenalty      int `json:"high_penalty"`
	HighPenaltyCap   int `json:"high_penalty_cap"`
	BreachPenalty    int `json:"breach_penalty"`
	BreachPenaltyCap int `json:"breach_penalty_cap"`
	FwBonus          int `json:"fw_bonus"`
	FwBonusCap       int `json:"fw_bonus_cap"`
}

// defaultScoreConfig returns the hardcoded defaults used when no row exists in score_config.
func defaultScoreConfig() ScoreConfig {
	return ScoreConfig{
		BaseScore:        70,
		CritPenalty:      5,
		CritPenaltyCap:   30,
		HighPenalty:      2,
		HighPenaltyCap:   10,
		BreachPenalty:    20,
		BreachPenaltyCap: 20,
		FwBonus:          10,
		FwBonusCap:       30,
	}
}

// GetScore returns the organisation's composite security health score along
// with the raw component counts used to derive it.
func (h *Handler) GetScore(c echo.Context) error {
	orgID, ok := c.Get("org_id").(string)
	if !ok || orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	ctx := c.Request().Context()

	cfg, err := h.svc.LoadScoreConfig(ctx, orgID)
	if err != nil {
		log.Error().Err(err).Msg("dashboard: load score config")
	}

	inp := h.svc.LoadScoreInputs(ctx, orgID, cfg)
	score, components := ComputeScore(inp)

	return c.JSON(http.StatusOK, map[string]any{
		"score":      score,
		"components": components,
	})
}

// GetBackupStatus handles GET /api/v1/dashboard/backup-status.
// Returns whether a backup has been taken in the last 7 days.
func (h *Handler) GetBackupStatus(c echo.Context) error {
	orgID, ok := c.Get("org_id").(string)
	if !ok || orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	lastBackup, err := h.svc.LastBackupAt(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Msg("dashboard: backup status")
	}
	stale := true
	if lastBackup != nil {
		stale = time.Since(*lastBackup) > 7*24*time.Hour
	}
	return c.JSON(http.StatusOK, map[string]any{
		"last_backup_at": lastBackup,
		"stale":          stale,
	})
}

// GetScoreConfig handles GET /api/v1/dashboard/score/config.
// Returns the current score configuration for the organisation, or defaults if none set.
func (h *Handler) GetScoreConfig(c echo.Context) error {
	orgID, ok := c.Get("org_id").(string)
	if !ok || orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	cfg, err := h.svc.LoadScoreConfig(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Msg("dashboard: get score config")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load score config"})
	}
	return c.JSON(http.StatusOK, cfg)
}

// UpdateScoreConfig handles PUT /api/v1/dashboard/score/config.
// Validates all values are in [1, 100] and upserts into score_config.
func (h *Handler) UpdateScoreConfig(c echo.Context) error {
	orgID, ok := c.Get("org_id").(string)
	if !ok || orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	var in ScoreConfig
	if err := c.Bind(&in); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body", "code": "INVALID_BODY"})
	}

	fields := []int{
		in.BaseScore, in.CritPenalty, in.CritPenaltyCap,
		in.HighPenalty, in.HighPenaltyCap,
		in.BreachPenalty, in.BreachPenaltyCap,
		in.FwBonus, in.FwBonusCap,
	}
	for _, v := range fields {
		if v < 1 || v > 100 {
			return c.JSON(http.StatusUnprocessableEntity, map[string]string{
				"error": "all config values must be between 1 and 100",
				"code":  "SCORE_CONFIG_VALIDATION_ERROR",
			})
		}
	}

	if err := h.svc.UpsertScoreConfig(c.Request().Context(), orgID, in); err != nil {
		log.Error().Err(err).Msg("dashboard: upsert score config")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to save score config"})
	}
	return c.JSON(http.StatusOK, in)
}

// GetAggregate handles GET /api/v1/dashboard/aggregate.
// Results are cached in Redis for 60 seconds to avoid hammering the DB.
func (h *Handler) GetAggregate(c echo.Context) error {
	orgID, ok := c.Get("org_id").(string)
	if !ok || orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	ctx := c.Request().Context()

	if h.rdb != nil {
		if cached, err := h.rdb.Get(ctx, aggregateCacheKey(orgID)).Bytes(); err == nil {
			return c.JSONBlob(http.StatusOK, cached)
		} else if err != redis.Nil {
			log.Warn().Err(err).Str("org_id", orgID).Msg("dashboard aggregate: redis get failed")
		}
	}

	resp, err := h.svc.LoadAggregate(ctx, orgID)
	if err != nil {
		log.Error().Err(err).Msg("dashboard: load aggregate")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load aggregate"})
	}

	if h.rdb != nil {
		if blob, err := json.Marshal(resp); err == nil {
			cacheCtx, cacheCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cacheCancel()
			if err := h.rdb.Set(cacheCtx, aggregateCacheKey(orgID), blob, aggregateCacheTTL).Err(); err != nil {
				log.Warn().Err(err).Str("org_id", orgID).Msg("dashboard aggregate: redis set failed")
			}
		}
	}

	return c.JSON(http.StatusOK, resp)
}

// ListNotifications returns the 50 most recent notifications for the authenticated organisation.
func (h *Handler) ListNotifications(c echo.Context) error {
	orgID, ok := c.Get("org_id").(string)
	if !ok || orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	result, err := h.svc.LoadNotifications(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Msg("list notifications")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	return c.JSON(http.StatusOK, result)
}

// MarkNotificationRead marks a single notification as read. Responds 204 on success.
func (h *Handler) MarkNotificationRead(c echo.Context) error {
	orgID, ok := c.Get("org_id").(string)
	if !ok || orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	if err := h.svc.MarkNotificationRead(c.Request().Context(), orgID, c.Param("id")); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	return c.NoContent(http.StatusNoContent)
}

// MarkAllRead marks every unread notification for the organisation as read. Responds 204 on success.
func (h *Handler) MarkAllRead(c echo.Context) error {
	orgID, ok := c.Get("org_id").(string)
	if !ok || orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	if err := h.svc.MarkAllNotificationsRead(c.Request().Context(), orgID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	return c.NoContent(http.StatusNoContent)
}
