// Package metrics exposes a Prometheus-compatible /metrics endpoint.
// No external Prometheus client library is used — metrics are written directly
// in the Prometheus text exposition format (version 0.0.4).
package metrics

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
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

	// ── vakt_findings_total ───────────────────────────────────────────────────
	fmt.Fprintln(w, "# HELP vakt_findings_total Total open findings by severity")
	fmt.Fprintln(w, "# TYPE vakt_findings_total gauge")
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
				fmt.Fprintf(w, "vakt_findings_total{severity=%q} %d\n", severity, count)
			}
		}
	}

	// ── vakt_score_current ────────────────────────────────────────────────────
	fmt.Fprintln(w, "# HELP vakt_score_current Current security score")
	fmt.Fprintln(w, "# TYPE vakt_score_current gauge")
	var score float64
	err = h.db.QueryRow(ctx, `
		SELECT COALESCE(AVG(score), 0)
		FROM   ck_score_snapshots
		WHERE  taken_at = (SELECT MAX(taken_at) FROM ck_score_snapshots)`).Scan(&score)
	if err != nil {
		log.Error().Err(err).Msg("metrics: query score")
		score = 0
	}
	fmt.Fprintf(w, "vakt_score_current %g\n", score)

	// ── vakt_dsr_open_total ───────────────────────────────────────────────────
	fmt.Fprintln(w, "# HELP vakt_dsr_open_total Open DSRs")
	fmt.Fprintln(w, "# TYPE vakt_dsr_open_total gauge")
	var dsrOpen int64
	err = h.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM po_dsr
		WHERE  status NOT IN ('completed','rejected')`).Scan(&dsrOpen)
	if err != nil {
		log.Error().Err(err).Msg("metrics: query dsr_open")
		dsrOpen = 0
	}
	fmt.Fprintf(w, "vakt_dsr_open_total %d\n", dsrOpen)

	// ── vakt_dsr_overdue_total ────────────────────────────────────────────────
	fmt.Fprintln(w, "# HELP vakt_dsr_overdue_total Overdue DSRs (past due_date)")
	fmt.Fprintln(w, "# TYPE vakt_dsr_overdue_total gauge")
	var dsrOverdue int64
	err = h.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM po_dsr
		WHERE  status NOT IN ('completed','rejected')
		  AND  due_date < CURRENT_DATE`).Scan(&dsrOverdue)
	if err != nil {
		log.Error().Err(err).Msg("metrics: query dsr_overdue")
		dsrOverdue = 0
	}
	fmt.Fprintf(w, "vakt_dsr_overdue_total %d\n", dsrOverdue)

	// ── vakt_backup_age_hours ─────────────────────────────────────────────────
	fmt.Fprintln(w, "# HELP vakt_backup_age_hours Hours since last backup (999 if never)")
	fmt.Fprintln(w, "# TYPE vakt_backup_age_hours gauge")
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
	fmt.Fprintf(w, "vakt_backup_age_hours %g\n", backupAgeHours)

	// ── per-org business metrics ──────────────────────────────────────────────
	// Collect all org IDs first, then query metrics per org concurrently.
	orgIDs, err := h.listOrgIDs(ctx)
	if err != nil {
		log.Error().Err(err).Msg("metrics: list org ids")
		return nil
	}

	bm, err := h.collectBusinessMetrics(ctx, orgIDs)
	if err != nil {
		log.Error().Err(err).Msg("metrics: collect business metrics")
		return nil
	}

	// vakt_open_risks_total
	fmt.Fprintln(w, "# HELP vakt_open_risks_total Open risks per organisation")
	fmt.Fprintln(w, "# TYPE vakt_open_risks_total gauge")
	for orgID, v := range bm.openRisks {
		fmt.Fprintf(w, "vakt_open_risks_total{org_id=%q} %d\n", orgID, v)
	}

	// vakt_open_capas_total
	fmt.Fprintln(w, "# HELP vakt_open_capas_total Open or in-progress CAPAs per organisation")
	fmt.Fprintln(w, "# TYPE vakt_open_capas_total gauge")
	for orgID, v := range bm.openCapas {
		fmt.Fprintf(w, "vakt_open_capas_total{org_id=%q} %d\n", orgID, v)
	}

	// vakt_overdue_capas_total
	fmt.Fprintln(w, "# HELP vakt_overdue_capas_total Overdue open CAPAs per organisation")
	fmt.Fprintln(w, "# TYPE vakt_overdue_capas_total gauge")
	for orgID, v := range bm.overdueCapas {
		fmt.Fprintf(w, "vakt_overdue_capas_total{org_id=%q} %d\n", orgID, v)
	}

	// vakt_open_incidents_total
	fmt.Fprintln(w, "# HELP vakt_open_incidents_total Open incidents per organisation")
	fmt.Fprintln(w, "# TYPE vakt_open_incidents_total gauge")
	for orgID, v := range bm.openIncidents {
		fmt.Fprintf(w, "vakt_open_incidents_total{org_id=%q} %d\n", orgID, v)
	}

	// vakt_controls_total
	fmt.Fprintln(w, "# HELP vakt_controls_total Total controls per org and framework")
	fmt.Fprintln(w, "# TYPE vakt_controls_total gauge")
	for k, v := range bm.controlsTotal {
		fmt.Fprintf(w, "vakt_controls_total{org_id=%q,framework_id=%q} %d\n", k.orgID, k.frameworkID, v)
	}

	// vakt_controls_implemented
	fmt.Fprintln(w, "# HELP vakt_controls_implemented Implemented controls per org and framework")
	fmt.Fprintln(w, "# TYPE vakt_controls_implemented gauge")
	for k, v := range bm.controlsImplemented {
		fmt.Fprintf(w, "vakt_controls_implemented{org_id=%q,framework_id=%q} %d\n", k.orgID, k.frameworkID, v)
	}

	return nil
}

// orgFrameworkKey is used as a map key for per-(org, framework) metrics.
type orgFrameworkKey struct {
	orgID       string
	frameworkID string
}

// businessMetrics holds the collected per-org and per-(org,framework) metric values.
type businessMetrics struct {
	openRisks           map[string]int64
	openCapas           map[string]int64
	overdueCapas        map[string]int64
	openIncidents       map[string]int64
	controlsTotal       map[orgFrameworkKey]int64
	controlsImplemented map[orgFrameworkKey]int64
}

// listOrgIDs returns all organisation IDs from the organisations table.
func (h *Handler) listOrgIDs(ctx context.Context) ([]string, error) {
	rows, err := h.db.Query(ctx, `SELECT id::text FROM organisations ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("query org ids: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan org id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// collectBusinessMetrics runs all per-org queries concurrently using errgroup.
func (h *Handler) collectBusinessMetrics(ctx context.Context, orgIDs []string) (*businessMetrics, error) {
	bm := &businessMetrics{
		openRisks:           make(map[string]int64, len(orgIDs)),
		openCapas:           make(map[string]int64, len(orgIDs)),
		overdueCapas:        make(map[string]int64, len(orgIDs)),
		openIncidents:       make(map[string]int64, len(orgIDs)),
		controlsTotal:       make(map[orgFrameworkKey]int64),
		controlsImplemented: make(map[orgFrameworkKey]int64),
	}

	var mu sync.Mutex
	g, gctx := errgroup.WithContext(ctx)

	// ── per-org scalar queries ────────────────────────────────────────────────

	g.Go(func() error {
		rows, err := h.db.Query(gctx, `
			SELECT org_id::text, COUNT(*) FROM ck_risks
			WHERE status = 'open'
			GROUP BY org_id`)
		if err != nil {
			log.Error().Err(err).Msg("metrics: query open_risks")
			return nil // soft-error: don't fail the whole handler
		}
		defer rows.Close()
		for rows.Next() {
			var orgID string
			var cnt int64
			if err := rows.Scan(&orgID, &cnt); err == nil {
				mu.Lock()
				bm.openRisks[orgID] = cnt
				mu.Unlock()
			}
		}
		return nil
	})

	g.Go(func() error {
		rows, err := h.db.Query(gctx, `
			SELECT org_id::text, COUNT(*) FROM ck_capas
			WHERE status IN ('open', 'in_progress')
			GROUP BY org_id`)
		if err != nil {
			log.Error().Err(err).Msg("metrics: query open_capas")
			return nil
		}
		defer rows.Close()
		for rows.Next() {
			var orgID string
			var cnt int64
			if err := rows.Scan(&orgID, &cnt); err == nil {
				mu.Lock()
				bm.openCapas[orgID] = cnt
				mu.Unlock()
			}
		}
		return nil
	})

	g.Go(func() error {
		rows, err := h.db.Query(gctx, `
			SELECT org_id::text, COUNT(*) FROM ck_capas
			WHERE status IN ('open', 'in_progress')
			  AND due_date < NOW()
			GROUP BY org_id`)
		if err != nil {
			log.Error().Err(err).Msg("metrics: query overdue_capas")
			return nil
		}
		defer rows.Close()
		for rows.Next() {
			var orgID string
			var cnt int64
			if err := rows.Scan(&orgID, &cnt); err == nil {
				mu.Lock()
				bm.overdueCapas[orgID] = cnt
				mu.Unlock()
			}
		}
		return nil
	})

	g.Go(func() error {
		rows, err := h.db.Query(gctx, `
			SELECT org_id::text, COUNT(*) FROM ck_incidents
			WHERE status = 'open'
			GROUP BY org_id`)
		if err != nil {
			log.Error().Err(err).Msg("metrics: query open_incidents")
			return nil
		}
		defer rows.Close()
		for rows.Next() {
			var orgID string
			var cnt int64
			if err := rows.Scan(&orgID, &cnt); err == nil {
				mu.Lock()
				bm.openIncidents[orgID] = cnt
				mu.Unlock()
			}
		}
		return nil
	})

	// ── per-(org, framework) controls queries ─────────────────────────────────

	g.Go(func() error {
		rows, err := h.db.Query(gctx, `
			SELECT org_id::text, framework_id::text, COUNT(*)
			FROM ck_controls
			GROUP BY org_id, framework_id`)
		if err != nil {
			log.Error().Err(err).Msg("metrics: query controls_total")
			return nil
		}
		defer rows.Close()
		for rows.Next() {
			var orgID, frameworkID string
			var cnt int64
			if err := rows.Scan(&orgID, &frameworkID, &cnt); err == nil {
				mu.Lock()
				bm.controlsTotal[orgFrameworkKey{orgID, frameworkID}] = cnt
				mu.Unlock()
			}
		}
		return nil
	})

	g.Go(func() error {
		rows, err := h.db.Query(gctx, `
			SELECT org_id::text, framework_id::text, COUNT(*)
			FROM ck_controls
			WHERE status = 'implemented'
			GROUP BY org_id, framework_id`)
		if err != nil {
			log.Error().Err(err).Msg("metrics: query controls_implemented")
			return nil
		}
		defer rows.Close()
		for rows.Next() {
			var orgID, frameworkID string
			var cnt int64
			if err := rows.Scan(&orgID, &frameworkID, &cnt); err == nil {
				mu.Lock()
				bm.controlsImplemented[orgFrameworkKey{orgID, frameworkID}] = cnt
				mu.Unlock()
			}
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return bm, nil
}
