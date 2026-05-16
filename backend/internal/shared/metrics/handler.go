// Package metrics exposes a Prometheus-compatible /metrics endpoint.
// No external Prometheus client library is used — metrics are written directly
// in the Prometheus text exposition format (version 0.0.4).
package metrics

import (
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// Handler serves Prometheus-format metrics.
type Handler struct {
	db *pgxpool.Pool
}

// NewHandler constructs a Handler.
func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{db: db}
}

// ServeMetrics writes Prometheus-format metrics (text/plain; version=0.0.4).
// No auth required — Prometheus scrapes this endpoint directly.
func (h *Handler) ServeMetrics(c echo.Context) error {
	ctx := c.Request().Context()
	w := c.Response()
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	// ── sechealth_findings_total ──────────────────────────────────────────────
	fmt.Fprintln(w, "# HELP sechealth_findings_total Total open findings by severity")
	fmt.Fprintln(w, "# TYPE sechealth_findings_total gauge")
	rows, err := h.db.Query(ctx, `
		SELECT severity, COUNT(*) AS cnt
		FROM   vb_findings
		WHERE  status NOT IN ('resolved','false_positive')
		GROUP  BY severity`)
	if err != nil {
		log.Error().Err(err).Msg("metrics: query findings")
	} else {
		defer rows.Close()
		for rows.Next() {
			var severity string
			var count int64
			if err := rows.Scan(&severity, &count); err == nil {
				fmt.Fprintf(w, "sechealth_findings_total{severity=%q} %d\n", severity, count)
			}
		}
	}

	// ── sechealth_score_current ───────────────────────────────────────────────
	fmt.Fprintln(w, "# HELP sechealth_score_current Current security score")
	fmt.Fprintln(w, "# TYPE sechealth_score_current gauge")
	var score float64
	err = h.db.QueryRow(ctx, `
		SELECT COALESCE(AVG(score), 0)
		FROM   ck_score_snapshots
		WHERE  taken_at = (SELECT MAX(taken_at) FROM ck_score_snapshots)`).Scan(&score)
	if err != nil {
		log.Error().Err(err).Msg("metrics: query score")
		score = 0
	}
	fmt.Fprintf(w, "sechealth_score_current %g\n", score)

	// ── sechealth_dsr_open_total ──────────────────────────────────────────────
	fmt.Fprintln(w, "# HELP sechealth_dsr_open_total Open DSRs")
	fmt.Fprintln(w, "# TYPE sechealth_dsr_open_total gauge")
	var dsrOpen int64
	err = h.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM po_dsr
		WHERE  status NOT IN ('completed','rejected')`).Scan(&dsrOpen)
	if err != nil {
		log.Error().Err(err).Msg("metrics: query dsr_open")
		dsrOpen = 0
	}
	fmt.Fprintf(w, "sechealth_dsr_open_total %d\n", dsrOpen)

	// ── sechealth_dsr_overdue_total ───────────────────────────────────────────
	fmt.Fprintln(w, "# HELP sechealth_dsr_overdue_total Overdue DSRs (past due_date)")
	fmt.Fprintln(w, "# TYPE sechealth_dsr_overdue_total gauge")
	var dsrOverdue int64
	err = h.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM po_dsr
		WHERE  status NOT IN ('completed','rejected')
		  AND  due_date < CURRENT_DATE`).Scan(&dsrOverdue)
	if err != nil {
		log.Error().Err(err).Msg("metrics: query dsr_overdue")
		dsrOverdue = 0
	}
	fmt.Fprintf(w, "sechealth_dsr_overdue_total %d\n", dsrOverdue)

	// ── sechealth_backup_age_hours ────────────────────────────────────────────
	fmt.Fprintln(w, "# HELP sechealth_backup_age_hours Hours since last backup (999 if never)")
	fmt.Fprintln(w, "# TYPE sechealth_backup_age_hours gauge")
	var backupAgeHours float64
	err = h.db.QueryRow(ctx, `
		SELECT COALESCE(
		    EXTRACT(EPOCH FROM (now() - MAX(backed_up_at))) / 3600,
		    999
		)
		FROM backup_log`).Scan(&backupAgeHours)
	if err != nil {
		log.Error().Err(err).Msg("metrics: query backup_age")
		backupAgeHours = 999
	}
	fmt.Fprintf(w, "sechealth_backup_age_hours %g\n", backupAgeHours)

	return nil
}
