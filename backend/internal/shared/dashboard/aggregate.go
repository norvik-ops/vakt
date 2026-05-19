// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package dashboard

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
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
