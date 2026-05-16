// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Package dashboard aggregates cross-module metrics into a single security
// score and manages in-app notifications stored in the user_notifications
// table. It queries SecPulse findings, SecPrivacy breaches, and SecVitals
// frameworks directly via raw SQL so it remains decoupled from each module's
// service layer.
package dashboard

import (
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

// Handler holds the database connection pool and optional Redis client shared
// across all dashboard endpoint implementations.
type Handler struct {
	db  *pgxpool.Pool
	rdb *redis.Client // may be nil — caching is skipped when absent
}

// NewHandler wires up a Handler backed by the provided connection pool and
// optional Redis client. Pass nil for rdb to disable aggregate caching.
func NewHandler(db *pgxpool.Pool, rdb *redis.Client) *Handler {
	return &Handler{db: db, rdb: rdb}
}

// ScoreConfig holds the configurable weights and caps for the security score formula.
type ScoreConfig struct {
	BaseScore       int `json:"base_score"`
	CritPenalty     int `json:"crit_penalty"`
	CritPenaltyCap  int `json:"crit_penalty_cap"`
	HighPenalty     int `json:"high_penalty"`
	HighPenaltyCap  int `json:"high_penalty_cap"`
	BreachPenalty   int `json:"breach_penalty"`
	BreachPenaltyCap int `json:"breach_penalty_cap"`
	FwBonus         int `json:"fw_bonus"`
	FwBonusCap      int `json:"fw_bonus_cap"`
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
// with the raw component counts used to derive it. The score formula is:
//
//	base_score
//	  − crit_penalty   (per critical finding, capped at crit_penalty_cap)
//	  − high_penalty   (per high finding,     capped at high_penalty_cap)
//	  − breach_penalty (per open breach,      capped at breach_penalty_cap)
//	  + fw_bonus       (per active framework, capped at fw_bonus_cap)
//
// Weights are loaded from the score_config table; hardcoded defaults are used
// when no row is present. The result is clamped to [0, 100]. Query failures for
// individual components are logged but do not abort the request.
func (h *Handler) GetScore(c echo.Context) error {
	orgID, ok := c.Get("org_id").(string)
	if !ok || orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	ctx := c.Request().Context()

	// Load configurable weights (falls back to defaults if no row).
	var cfg ScoreConfig
	err := h.db.QueryRow(ctx,
		`SELECT base_score, crit_penalty, crit_penalty_cap, high_penalty, high_penalty_cap,
		        breach_penalty, breach_penalty_cap, fw_bonus, fw_bonus_cap
		   FROM score_config WHERE org_id=$1::uuid`, orgID).Scan(
		&cfg.BaseScore,
		&cfg.CritPenalty,
		&cfg.CritPenaltyCap,
		&cfg.HighPenalty,
		&cfg.HighPenaltyCap,
		&cfg.BreachPenalty,
		&cfg.BreachPenaltyCap,
		&cfg.FwBonus,
		&cfg.FwBonusCap,
	)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			log.Error().Err(err).Msg("dashboard: load score config")
		}
		cfg = defaultScoreConfig()
	}

	var critCount, highCount, breachCount, fwCount int64

	if err := h.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM vb_findings WHERE org_id=$1::uuid AND severity='critical' AND status NOT IN ('resolved','false_positive')`,
		orgID).Scan(&critCount); err != nil {
		log.Error().Err(err).Msg("dashboard: count critical findings")
	}
	if err := h.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM vb_findings WHERE org_id=$1::uuid AND severity='high' AND status NOT IN ('resolved','false_positive')`,
		orgID).Scan(&highCount); err != nil {
		log.Error().Err(err).Msg("dashboard: count high findings")
	}
	if err := h.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM po_breaches WHERE org_id=$1::uuid AND status='open'`,
		orgID).Scan(&breachCount); err != nil {
		log.Error().Err(err).Msg("dashboard: count breaches")
	}
	if err := h.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM ck_frameworks WHERE org_id=$1::uuid`,
		orgID).Scan(&fwCount); err != nil {
		log.Error().Err(err).Msg("dashboard: count frameworks")
	}

	critPenalty := int(critCount) * cfg.CritPenalty
	if critPenalty > cfg.CritPenaltyCap {
		critPenalty = cfg.CritPenaltyCap
	}
	highPenalty := int(highCount) * cfg.HighPenalty
	if highPenalty > cfg.HighPenaltyCap {
		highPenalty = cfg.HighPenaltyCap
	}
	breachPenalty := int(breachCount) * cfg.BreachPenalty
	if breachPenalty > cfg.BreachPenaltyCap {
		breachPenalty = cfg.BreachPenaltyCap
	}
	fwBonus := int(fwCount) * cfg.FwBonus
	if fwBonus > cfg.FwBonusCap {
		fwBonus = cfg.FwBonusCap
	}

	score := cfg.BaseScore - critPenalty - highPenalty - breachPenalty + fwBonus
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"score": score,
		"components": map[string]int64{
			"critical_findings": critCount,
			"high_findings":     highCount,
			"open_breaches":     breachCount,
			"active_frameworks": fwCount,
		},
	})
}

// GetBackupStatus handles GET /api/v1/dashboard/backup-status.
// Returns whether a backup has been taken in the last 7 days.
func (h *Handler) GetBackupStatus(c echo.Context) error {
	orgID, ok := c.Get("org_id").(string)
	if !ok || orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	var lastBackup *time.Time
	err := h.db.QueryRow(c.Request().Context(),
		`SELECT backed_up_at FROM backup_log WHERE org_id=$1::uuid ORDER BY backed_up_at DESC LIMIT 1`,
		orgID).Scan(&lastBackup)
	stale := true
	if err == nil && lastBackup != nil {
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
	ctx := c.Request().Context()

	var cfg ScoreConfig
	err := h.db.QueryRow(ctx,
		`SELECT base_score, crit_penalty, crit_penalty_cap, high_penalty, high_penalty_cap,
		        breach_penalty, breach_penalty_cap, fw_bonus, fw_bonus_cap
		   FROM score_config WHERE org_id=$1::uuid`, orgID).Scan(
		&cfg.BaseScore,
		&cfg.CritPenalty,
		&cfg.CritPenaltyCap,
		&cfg.HighPenalty,
		&cfg.HighPenaltyCap,
		&cfg.BreachPenalty,
		&cfg.BreachPenaltyCap,
		&cfg.FwBonus,
		&cfg.FwBonusCap,
	)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			log.Error().Err(err).Msg("dashboard: get score config")
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load score config"})
		}
		cfg = defaultScoreConfig()
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
	ctx := c.Request().Context()

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

	_, err := h.db.Exec(ctx,
		`INSERT INTO score_config
		   (org_id, base_score, crit_penalty, crit_penalty_cap, high_penalty, high_penalty_cap,
		    breach_penalty, breach_penalty_cap, fw_bonus, fw_bonus_cap, updated_at)
		 VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $10, now())
		 ON CONFLICT (org_id) DO UPDATE SET
		   base_score        = EXCLUDED.base_score,
		   crit_penalty      = EXCLUDED.crit_penalty,
		   crit_penalty_cap  = EXCLUDED.crit_penalty_cap,
		   high_penalty      = EXCLUDED.high_penalty,
		   high_penalty_cap  = EXCLUDED.high_penalty_cap,
		   breach_penalty    = EXCLUDED.breach_penalty,
		   breach_penalty_cap = EXCLUDED.breach_penalty_cap,
		   fw_bonus          = EXCLUDED.fw_bonus,
		   fw_bonus_cap      = EXCLUDED.fw_bonus_cap,
		   updated_at        = now()`,
		orgID,
		in.BaseScore, in.CritPenalty, in.CritPenaltyCap,
		in.HighPenalty, in.HighPenaltyCap,
		in.BreachPenalty, in.BreachPenaltyCap,
		in.FwBonus, in.FwBonusCap,
	)
	if err != nil {
		log.Error().Err(err).Msg("dashboard: upsert score config")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to save score config"})
	}
	return c.JSON(http.StatusOK, in)
}
