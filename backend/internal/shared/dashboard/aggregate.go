// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
)

// FrameworkScore holds the per-framework compliance score.
type FrameworkScore struct {
	FrameworkID         string  `json:"framework_id"`
	FrameworkName       string  `json:"framework_name"`
	TotalControls       int     `json:"total_controls"`
	ImplementedControls int     `json:"implemented_controls"`
	ScorePct            float64 `json:"score_pct"`
}

// RiskSummary is a lightweight risk row for the top-risks list.
type RiskSummary struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Likelihood int    `json:"likelihood"`
	Impact     int    `json:"impact"`
	Score      int    `json:"score"`
	Status     string `json:"status"`
}

// ActivityEntry is a single audit-log row surfaced on the dashboard.
type ActivityEntry struct {
	ID         string    `json:"id"`
	Action     string    `json:"action"`
	EntityType string    `json:"entity_type"`
	UserEmail  string    `json:"user_email"`
	CreatedAt  time.Time `json:"created_at"`
}

// AggregateResponse is the full payload returned by GET /api/v1/dashboard/aggregate.
type AggregateResponse struct {
	FrameworkScores  []FrameworkScore `json:"framework_scores"`
	OpenCAPAs        int              `json:"open_capas"`
	OverdueControls  int              `json:"overdue_controls"`
	OverdueTasks     int              `json:"overdue_tasks"`
	CriticalRisks    int              `json:"critical_risks"`
	TopRisks         []RiskSummary    `json:"top_risks"`
	RecentActivity   []ActivityEntry  `json:"recent_activity"`
	PoliciesTotal    int              `json:"policies_total"`
	PoliciesApproved int              `json:"policies_approved"`
}

// aggregateCacheTTL is the Redis TTL for the dashboard aggregate payload.
const aggregateCacheTTL = 60 * time.Second

// aggregateCacheKey returns the Redis key for an org's dashboard aggregate.
func aggregateCacheKey(orgID string) string {
	return fmt.Sprintf("dashboard:aggregate:%s", orgID)
}

// GetAggregate handles GET /api/v1/dashboard/aggregate.
// It runs all sub-queries concurrently and returns a single JSON payload that
// powers the executive compliance dashboard. Results are cached in Redis for
// 60 seconds to avoid hammering the DB on repeated dashboard refreshes.
func (h *Handler) GetAggregate(c echo.Context) error {
	orgID, ok := c.Get("org_id").(string)
	if !ok || orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	ctx := c.Request().Context()

	// ── Redis cache check ──────────────────────────────────────────────────────
	if h.rdb != nil {
		if cached, err := h.rdb.Get(ctx, aggregateCacheKey(orgID)).Bytes(); err == nil {
			// Cache hit — return directly without touching the DB.
			return c.JSONBlob(http.StatusOK, cached)
		} else if err != redis.Nil {
			log.Warn().Err(err).Str("org_id", orgID).Msg("dashboard aggregate: redis get failed")
		}
	}

	var (
		fwScores         []FrameworkScore
		openCAPAs        int64
		overdueControls  int64
		overdueTasks     int64
		criticalRisks    int64
		topRisks         []RiskSummary
		recentActivity   []ActivityEntry
		policiesTotal    int64
		policiesApproved int64
	)

	g, gctx := errgroup.WithContext(ctx)

	// Framework compliance scores
	g.Go(func() error {
		rows, err := h.db.Query(gctx, `
			SELECT f.id::text, f.name,
			       COUNT(c.id)::int                                                              AS total,
			       COUNT(c.id) FILTER (WHERE c.manual_status IN ('implemented','partially_implemented'))::int AS implemented
			FROM ck_frameworks f
			LEFT JOIN ck_controls c ON c.framework_id = f.id AND c.org_id = f.org_id
			WHERE f.org_id = $1::uuid
			GROUP BY f.id, f.name
			ORDER BY f.name`, orgID)
		if err != nil {
			log.Error().Err(err).Msg("dashboard aggregate: framework scores")
			return nil // soft-fail — return empty slice
		}
		defer rows.Close()
		for rows.Next() {
			var fs FrameworkScore
			if err := rows.Scan(&fs.FrameworkID, &fs.FrameworkName, &fs.TotalControls, &fs.ImplementedControls); err != nil {
				log.Error().Err(err).Msg("dashboard aggregate: scan framework score")
				continue
			}
			if fs.TotalControls > 0 {
				fs.ScorePct = float64(fs.ImplementedControls) / float64(fs.TotalControls) * 100
			}
			fwScores = append(fwScores, fs)
		}
		return rows.Err()
	})

	// Open CAPAs
	g.Go(func() error {
		if err := h.db.QueryRow(gctx,
			`SELECT COUNT(*)::bigint FROM ck_capas WHERE org_id=$1::uuid AND status != 'closed'`,
			orgID).Scan(&openCAPAs); err != nil {
			log.Error().Err(err).Msg("dashboard aggregate: open capas")
		}
		return nil
	})

	// Overdue controls (next_review_due < now)
	g.Go(func() error {
		if err := h.db.QueryRow(gctx,
			`SELECT COUNT(*)::bigint FROM ck_controls
			 WHERE org_id=$1::uuid AND next_review_due IS NOT NULL AND next_review_due < NOW()`,
			orgID).Scan(&overdueControls); err != nil {
			log.Error().Err(err).Msg("dashboard aggregate: overdue controls")
		}
		return nil
	})

	// Overdue tasks
	g.Go(func() error {
		if err := h.db.QueryRow(gctx,
			`SELECT COUNT(*)::bigint FROM ck_tasks
			 WHERE org_id=$1::uuid AND due_date IS NOT NULL AND due_date < NOW() AND status != 'done'`,
			orgID).Scan(&overdueTasks); err != nil {
			log.Error().Err(err).Msg("dashboard aggregate: overdue tasks")
		}
		return nil
	})

	// Critical risks (score >= 15)
	g.Go(func() error {
		if err := h.db.QueryRow(gctx,
			`SELECT COUNT(*)::bigint FROM ck_risks
			 WHERE org_id=$1::uuid AND (likelihood * impact) >= 15`,
			orgID).Scan(&criticalRisks); err != nil {
			log.Error().Err(err).Msg("dashboard aggregate: critical risks")
		}
		return nil
	})

	// Top 5 risks by score
	g.Go(func() error {
		rows, err := h.db.Query(gctx, `
			SELECT id::text, title, likelihood::int, impact::int,
			       (likelihood * impact)::int AS score, status
			FROM ck_risks
			WHERE org_id = $1::uuid
			ORDER BY score DESC, updated_at DESC
			LIMIT 5`, orgID)
		if err != nil {
			log.Error().Err(err).Msg("dashboard aggregate: top risks")
			return nil
		}
		defer rows.Close()
		for rows.Next() {
			var r RiskSummary
			if err := rows.Scan(&r.ID, &r.Title, &r.Likelihood, &r.Impact, &r.Score, &r.Status); err != nil {
				log.Error().Err(err).Msg("dashboard aggregate: scan risk")
				continue
			}
			topRisks = append(topRisks, r)
		}
		return rows.Err()
	})

	// Recent activity (last 10 audit_log entries)
	g.Go(func() error {
		rows, err := h.db.Query(gctx, `
			SELECT id::text,
			       action,
			       resource_type,
			       COALESCE(user_email, '') AS user_email,
			       created_at
			FROM audit_log
			WHERE org_id = $1::uuid
			ORDER BY created_at DESC
			LIMIT 10`, orgID)
		if err != nil {
			log.Error().Err(err).Msg("dashboard aggregate: recent activity")
			return nil
		}
		defer rows.Close()
		for rows.Next() {
			var e ActivityEntry
			if err := rows.Scan(&e.ID, &e.Action, &e.EntityType, &e.UserEmail, &e.CreatedAt); err != nil {
				log.Error().Err(err).Msg("dashboard aggregate: scan activity")
				continue
			}
			recentActivity = append(recentActivity, e)
		}
		return rows.Err()
	})

	// Policies total and approved (active)
	g.Go(func() error {
		if err := h.db.QueryRow(gctx,
			`SELECT COUNT(*)::bigint,
			        COUNT(*) FILTER (WHERE status = 'active')::bigint
			 FROM ck_policies WHERE org_id=$1::uuid`,
			orgID).Scan(&policiesTotal, &policiesApproved); err != nil {
			log.Error().Err(err).Msg("dashboard aggregate: policies")
		}
		return nil
	})

	// Wait for all goroutines. We use soft-fail (nil returns) for each query
	// so the overall response is never blocked by a single missing table.
	_ = g.Wait()

	// Ensure non-nil slices for clean JSON output.
	if fwScores == nil {
		fwScores = []FrameworkScore{}
	}
	if topRisks == nil {
		topRisks = []RiskSummary{}
	}
	if recentActivity == nil {
		recentActivity = []ActivityEntry{}
	}

	resp := AggregateResponse{
		FrameworkScores:  fwScores,
		OpenCAPAs:        int(openCAPAs),
		OverdueControls:  int(overdueControls),
		OverdueTasks:     int(overdueTasks),
		CriticalRisks:    int(criticalRisks),
		TopRisks:         topRisks,
		RecentActivity:   recentActivity,
		PoliciesTotal:    int(policiesTotal),
		PoliciesApproved: int(policiesApproved),
	}

	// ── Redis cache store ──────────────────────────────────────────────────────
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

// InvalidateDashboardCache deletes the cached aggregate payload for the given
// org from Redis, forcing the next request to re-query the database.
// It is a no-op when rdb is nil (Redis not configured). Service layers should
// call this after any write that affects the dashboard aggregate (risks,
// controls, findings, policies).
func InvalidateDashboardCache(ctx context.Context, rdb *redis.Client, orgID string) error {
	if rdb == nil {
		return nil
	}
	return rdb.Del(ctx, aggregateCacheKey(orgID)).Err()
}
