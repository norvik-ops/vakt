// Package errorbudget tracks SLO compliance and computes remaining error budget.
// The budget is defined in environment variables and evaluated weekly.
package errorbudget

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// Config holds the SLO targets (read from env at startup).
type Config struct {
	// UptimeSLO is the target uptime percentage, e.g. 99.9
	UptimeSLO float64
	// P99LatencyMs is the target p99 latency in milliseconds
	P99LatencyMs int
}

// LoadConfig reads SLO config from environment variables.
// VAKT_SLO_UPTIME (default 99.9), VAKT_SLO_P99_LATENCY_MS (default 500)
func LoadConfig() Config {
	uptime := 99.9
	if v := os.Getenv("VAKT_SLO_UPTIME"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			uptime = f
		}
	}
	p99 := 500
	if v := os.Getenv("VAKT_SLO_P99_LATENCY_MS"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			p99 = i
		}
	}
	return Config{UptimeSLO: uptime, P99LatencyMs: p99}
}

// WeeklyReport computes and logs the error budget status for the past 7 days.
// It reads request metrics from the audit_log table (as a proxy for uptime evidence).
func WeeklyReport(ctx context.Context, db *pgxpool.Pool, cfg Config) error {
	// Count total requests and failed requests (5xx) in last 7 days from audit_log.
	// This is a proxy measure — in production, use a dedicated metrics store.
	var total, failures int
	err := db.QueryRow(ctx, `
		SELECT
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE details->>'status_code' ~ '^5') AS failures
		FROM audit_log
		WHERE created_at >= NOW() - INTERVAL '7 days'
		  AND deleted_at IS NULL
	`).Scan(&total, &failures)
	if err != nil {
		return fmt.Errorf("errorbudget: query audit_log: %w", err)
	}

	if total == 0 {
		log.Info().Msg("errorbudget: no requests in last 7 days, skipping report")
		return nil
	}

	successRate := float64(total-failures) / float64(total) * 100
	budgetConsumed := 0.0
	if successRate < cfg.UptimeSLO {
		// Error budget consumed = (actual_error_rate - allowed_error_rate) / allowed_error_rate * 100
		allowedErrorRate := 100 - cfg.UptimeSLO
		actualErrorRate := 100 - successRate
		if allowedErrorRate > 0 {
			budgetConsumed = (actualErrorRate - allowedErrorRate) / allowedErrorRate * 100
		}
	}

	weekEnd := time.Now().UTC()
	weekStart := weekEnd.Add(-7 * 24 * time.Hour)

	log.Info().
		Str("period_start", weekStart.Format(time.RFC3339)).
		Str("period_end", weekEnd.Format(time.RFC3339)).
		Int("total_requests", total).
		Int("failed_requests", failures).
		Float64("success_rate_pct", successRate).
		Float64("slo_target_pct", cfg.UptimeSLO).
		Float64("budget_consumed_pct", budgetConsumed).
		Msg("errorbudget: weekly report")

	return nil
}
