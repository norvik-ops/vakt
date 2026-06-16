// Package metrics exposes a Prometheus-compatible /metrics endpoint.
// No external Prometheus client library is used — metrics are written directly
// in the Prometheus text exposition format (version 0.0.4).
package metrics

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	"github.com/matharnica/vakt/internal/shared/audit"
)

// Handler serves Prometheus-format metrics.
type Handler struct {
	db        *pgxpool.Pool
	redisAddr string // optional — used for queue-depth metrics
}

// NewHandler constructs a Handler.
func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{db: db}
}

// WithRedisAddr sets the Redis address for queue-depth metrics.
// When not set, queue-depth metrics are omitted.
func (h *Handler) WithRedisAddr(addr string) *Handler {
	h.redisAddr = addr
	return h
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
	// orgid-lint: global — Prometheus /metrics endpoint: intentional cross-org aggregate for instance-level monitoring
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
	// orgid-lint: global — Prometheus /metrics: cross-org average for instance monitoring
	err = h.db.QueryRow(ctx, `
		SELECT COALESCE(AVG(score), 0)
		FROM   ck_score_history
		WHERE  recorded_at = (SELECT MAX(recorded_at) FROM ck_score_history)`).Scan(&score)
	if err != nil {
		log.Error().Err(err).Msg("metrics: query score")
		score = 0
	}
	fmt.Fprintf(w, "vakt_score_current %g\n", score)

	// ── vakt_dsr_open_total ───────────────────────────────────────────────────
	fmt.Fprintln(w, "# HELP vakt_dsr_open_total Open DSRs")
	fmt.Fprintln(w, "# TYPE vakt_dsr_open_total gauge")
	var dsrOpen int64
	// orgid-lint: global — Prometheus /metrics: cross-org aggregate for instance monitoring
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
	// orgid-lint: global — Prometheus /metrics: cross-org aggregate for instance monitoring
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

	// ── vakt_organizations_total ─────────────────────────────────────────────
	fmt.Fprintln(w, "# HELP vakt_organizations_total Total number of organizations")
	fmt.Fprintln(w, "# TYPE vakt_organizations_total gauge")
	var orgsTotal int64
	err = h.db.QueryRow(ctx, `SELECT COUNT(*) FROM organizations`).Scan(&orgsTotal)
	if err != nil {
		log.Error().Err(err).Msg("metrics: query organizations_total")
		orgsTotal = 0
	}
	fmt.Fprintf(w, "vakt_organizations_total %d\n", orgsTotal)

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

	// ── S46-1: runtime + session + pool metrics ───────────────────────────────

	// vakt_active_sessions_total — active (non-expired) sessions from auth table
	fmt.Fprintln(w, "# HELP vakt_active_sessions_total Number of currently active user sessions")
	fmt.Fprintln(w, "# TYPE vakt_active_sessions_total gauge")
	var activeSessions int64
	err = h.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM sessions
		WHERE expires_at > NOW() AND revoked_at IS NULL`).Scan(&activeSessions)
	if err != nil {
		log.Error().Err(err).Msg("metrics: query active_sessions")
		activeSessions = 0
	}
	fmt.Fprintf(w, "vakt_active_sessions_total %d\n", activeSessions)

	// vakt_db_pool_in_use — pgxpool connections currently checked out
	fmt.Fprintln(w, "# HELP vakt_db_pool_in_use Database connections currently in use")
	fmt.Fprintln(w, "# TYPE vakt_db_pool_in_use gauge")
	poolStats := h.db.Stat()
	fmt.Fprintf(w, "vakt_db_pool_in_use %d\n", poolStats.AcquiredConns())

	// vakt_db_pool_idle — pgxpool idle connections
	fmt.Fprintln(w, "# HELP vakt_db_pool_idle Database connections idle in pool")
	fmt.Fprintln(w, "# TYPE vakt_db_pool_idle gauge")
	fmt.Fprintf(w, "vakt_db_pool_idle %d\n", poolStats.IdleConns())

	// vakt_queue_depth — Asynq queue depths (only when Redis is configured)
	fmt.Fprintln(w, "# HELP vakt_queue_depth Asynq queue depth by queue name")
	fmt.Fprintln(w, "# TYPE vakt_queue_depth gauge")
	if h.redisAddr != "" {
		h.writeQueueDepth(ctx, w)
	}

	// S58-1: per-task job-duration counters written by the worker middleware.
	if h.redisAddr != "" {
		h.writeAsynqJobMetrics(ctx, w)
	}

	// S88-6: audit-forward counters (in-process atomics; 0 when forwarder off).
	auditSent, auditDropped, auditFailed := audit.ForwardStats()
	fmt.Fprintln(w, "# HELP vakt_audit_forward_sent Audit events forwarded to the Syslog/SIEM sink")
	fmt.Fprintln(w, "# TYPE vakt_audit_forward_sent counter")
	fmt.Fprintf(w, "vakt_audit_forward_sent %d\n", auditSent)
	fmt.Fprintln(w, "# HELP vakt_audit_forward_dropped Audit events dropped (forward buffer full)")
	fmt.Fprintln(w, "# TYPE vakt_audit_forward_dropped counter")
	fmt.Fprintf(w, "vakt_audit_forward_dropped %d\n", auditDropped)
	fmt.Fprintln(w, "# HELP vakt_audit_forward_failed Audit events that failed to deliver to the sink")
	fmt.Fprintln(w, "# TYPE vakt_audit_forward_failed counter")
	fmt.Fprintf(w, "vakt_audit_forward_failed %d\n", auditFailed)

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

// listOrgIDs returns all organisation IDs from the organizations table.
func (h *Handler) listOrgIDs(ctx context.Context) ([]string, error) {
	rows, err := h.db.Query(ctx, `SELECT id::text FROM organizations ORDER BY id`)
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
			WHERE manual_status = 'implemented'
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

// writeAsynqJobMetrics reads per-task counters that the worker's
// AsynqInstrumentingMiddleware writes into Redis and emits them as
// Prometheus counters. We deliberately don't use Asynq's Inspector here
// because it only knows about queues, not the per-task-type breakdown
// we need ("which specific job is slow / failing?").
func (h *Handler) writeAsynqJobMetrics(ctx context.Context, w io.Writer) {
	if h.redisAddr == "" {
		return
	}
	rdb := redis.NewClient(&redis.Options{Addr: h.redisAddr})
	defer func() { _ = rdb.Close() }()

	// SCAN once for all metric keys; parse each into (kind, task, result).
	var cursor uint64
	var allKeys []string
	for {
		keys, next, err := rdb.Scan(ctx, cursor, "metric:asynq:*", 500).Result()
		if err != nil {
			log.Warn().Err(err).Msg("metrics: scan asynq metric keys")
			return
		}
		allKeys = append(allKeys, keys...)
		cursor = next
		if cursor == 0 {
			break
		}
	}
	if len(allKeys) == 0 {
		return
	}

	// Bulk MGET for values.
	values, err := rdb.MGet(ctx, allKeys...).Result()
	if err != nil {
		log.Warn().Err(err).Msg("metrics: mget asynq metric keys")
		return
	}

	type entry struct{ task, result, kind, value string }
	entries := make([]entry, 0, len(allKeys))
	for i, key := range allKeys {
		// metric:asynq:<kind>:<task>:<result>
		parts := strings.SplitN(strings.TrimPrefix(key, "metric:asynq:"), ":", 3)
		if len(parts) != 3 {
			continue
		}
		v, ok := values[i].(string)
		if !ok || v == "" {
			continue
		}
		entries = append(entries, entry{task: parts[1], result: parts[2], kind: parts[0], value: v})
	}

	// Group by kind to emit a clean Prometheus block per metric family.
	byKind := map[string][]entry{}
	for _, e := range entries {
		byKind[e.kind] = append(byKind[e.kind], e)
	}

	emit := func(name, help, metricType string, kind string) {
		es, ok := byKind[kind]
		if !ok || len(es) == 0 {
			return
		}
		fmt.Fprintln(w, "# HELP "+name+" "+help)
		fmt.Fprintln(w, "# TYPE "+name+" "+metricType)
		for _, e := range es {
			fmt.Fprintf(w, "%s{task=%q,result=%q} %s\n", name, e.task, e.result, e.value)
		}
	}

	emit("vakt_asynq_jobs_total",
		"Total Asynq jobs processed per task type and result",
		"counter", "count")
	emit("vakt_asynq_jobs_duration_ms_sum",
		"Cumulative wall-clock duration of Asynq jobs per task type and result, milliseconds",
		"counter", "duration_ms_sum")
	emit("vakt_asynq_jobs_duration_ms_max",
		"Maximum observed wall-clock duration of an Asynq job per task type and result, milliseconds",
		"gauge", "duration_ms_max")
}

// writeQueueDepth queries Asynq queue depths and writes them to w.
// One gauge line per known queue; errors are logged but don't abort the output.
func (h *Handler) writeQueueDepth(_ context.Context, w io.Writer) {
	inspector := asynq.NewInspector(asynq.RedisClientOpt{Addr: h.redisAddr})
	defer func() { _ = inspector.Close() }()

	queues, err := inspector.Queues()
	if err != nil {
		log.Error().Err(err).Msg("metrics: list asynq queues")
		return
	}
	for _, name := range queues {
		info, err := inspector.GetQueueInfo(name)
		if err != nil {
			log.Error().Err(err).Str("queue", name).Msg("metrics: get queue info")
			continue
		}
		fmt.Fprintf(w, "vakt_queue_depth{queue=%q} %d\n", name, info.Pending+info.Active)
	}
}
