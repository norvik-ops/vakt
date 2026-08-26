// Package metrics exposes a Prometheus-compatible /metrics endpoint.
// No external Prometheus client library is used — metrics are written directly
// in the Prometheus text exposition format (version 0.0.4).
package metrics

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	"github.com/matharnica/vakt/internal/shared/audit"
	"github.com/matharnica/vakt/internal/shared/queuemetrics"
	"github.com/matharnica/vakt/internal/shared/redisopt"
)

// Handler serves Prometheus-format metrics.
type Handler struct {
	db *pgxpool.Pool
	// redisOpt are the full parsed connection options — optional; when nil the
	// queue-depth and per-task Asynq metrics are omitted entirely.
	redisOpt *redis.Options
}

// NewHandler constructs a Handler.
func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{db: db}
}

// WithRedis sets the Redis connection used by the queue-depth and per-task
// Asynq metrics. When not set, both metric families are omitted.
//
// S121-C3 (I1): the shipped compose default runs Redis with --requirepass, but
// the metrics Redis/Asynq clients were built with only {Addr} and no Password.
// On any auth-protected Redis (i.e. the default deployment) every SCAN/Inspector
// call failed with NOAUTH, so vakt_queue_depth and vakt_asynq_jobs_* were
// silently absent — a Zabbix blind spot for queue backlog.
//
// R1-14b-01: this takes the full options rather than (addr, password), because
// the same class of bug came back one field over. Both consumers below read
// data the WORKER wrote — queue keys via the Inspector, metric:asynq:* via
// SCAN — and both were pinned to DB 0 regardless of the configured database.
// On a URL like redis://host:6379/1 they scraped an empty database and emitted
// a calm, zero-depth queue: a metric that is not missing but wrong, which is
// the harder kind to notice.
func (h *Handler) WithRedis(opts *redis.Options) *Handler {
	h.redisOpt = opts
	return h
}

// redisConfigured reports whether Redis-backed metrics can be emitted at all.
func (h *Handler) redisConfigured() bool {
	return h.redisOpt != nil && h.redisOpt.Addr != ""
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
	// orgid-lint: global — platform-wide operational metric (self-hosted single instance), not per-org data
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
	// orgid-lint: global — platform-wide operational metric (self-hosted single instance), not per-org data
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

	// vakt_queue_depth / _retry / _archived — Zustand der Asynq-Warteschlangen.
	// Die Stichproben gibt es nur mit konfiguriertem Redis; die Familien werden
	// trotzdem immer deklariert, damit eine Zeitreihe nicht ganz verschwindet.
	var snaps []queueSnapshot
	if h.redisConfigured() {
		snaps = h.collectQueueSnapshots()
	}
	writeQueueMetrics(w, snaps)

	// S58-1: per-task job-duration counters written by the worker middleware.
	if h.redisConfigured() {
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

	// S122-B3 (INC-01): producer-side Asynq enqueue failures, keyed by queue.
	// In-process (the failure is often "cannot reach Redis", so it cannot be
	// recorded into Redis). A non-zero value means jobs were accepted by the API
	// but never queued — the class the NOAUTH bug produced silently.
	fmt.Fprintln(w, "# HELP vakt_asynq_enqueue_errors_total Asynq enqueue failures on the producer (API) side")
	fmt.Fprintln(w, "# TYPE vakt_asynq_enqueue_errors_total counter")
	enqErrs := queuemetrics.Snapshot()
	if len(enqErrs) == 0 {
		// Emit a zero series so the metric always exists and the Zabbix trigger
		// has something to evaluate against before the first failure.
		fmt.Fprintf(w, "vakt_asynq_enqueue_errors_total{queue=%q} 0\n", "default")
	}
	for queue, n := range enqErrs {
		fmt.Fprintf(w, "vakt_asynq_enqueue_errors_total{queue=%q} %d\n", queue, n)
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
	if !h.redisConfigured() {
		return
	}
	// R1-14b-01: redisopt.GoRedis copies the options — these keys are written
	// by the WORKER into the configured database, so the client MUST select the
	// same one. It used to be rebuilt from {Addr, Password} and always landed in
	// DB 0, which returned an empty SCAN and therefore no job metrics at all.
	rdb := redis.NewClient(redisopt.GoRedis(h.redisOpt))
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

	entries := make([]asynqMetricEntry, 0, len(allKeys))
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
		entries = append(entries, asynqMetricEntry{task: parts[1], result: parts[2], kind: parts[0], value: v})
	}

	byKind := groupAsynqEntries(entries)

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

// asynqMetricEntry ist ein aus Redis gelesener Zählerstand.
type asynqMetricEntry struct{ task, result, kind, value string }

// groupAsynqEntries ordnet die Zählerstände nach Metrik-Familie, ergänzt
// fehlende Fehler-Zeitreihen und sortiert stabil.
//
// R1-19-02: Für jeden Task, der überhaupt läuft, muss die Zeitreihe
// `result="err"` existieren — auch wenn sie 0 ist.
//
// Bisher gab es die Zeile nur, wenn tatsächlich etwas fehlgeschlagen war, und
// sie verschwand nach sieben Tagen wieder (Schlüssel-Verfall, siehe
// asynq_middleware.go). Eine fehlende Zeitreihe ist für einen Alarm aber nicht
// dasselbe wie eine 0: Zabbix bekommt gar keinen Wert, das Item wird „nicht
// unterstützt", und der Trigger feuert nie. Ausgerechnet der Fall, auf den es
// ankommt — ein Task, der lange sauber lief und dann kippt — hatte damit keine
// Grundlinie, gegen die man ihn hätte vergleichen können.
//
// Grundlage ist der ok-Zähler: Wer erfolgreich lief, existiert und bekommt
// eine err-Zeile. Ein Task, von dem es überhaupt keine Spur gibt, wird nicht
// erfunden.
//
// Herausgelöst aus writeAsynqJobMetrics, damit diese Regel ohne Redis prüfbar
// ist.
func groupAsynqEntries(entries []asynqMetricEntry) map[string][]asynqMetricEntry {
	haveCount := map[string]bool{}
	tasksWithCount := map[string]bool{}
	for _, e := range entries {
		if e.kind != "count" {
			continue
		}
		haveCount[e.task+"|"+e.result] = true
		tasksWithCount[e.task] = true
	}
	for task := range tasksWithCount {
		if !haveCount[task+"|err"] {
			entries = append(entries, asynqMetricEntry{task: task, result: "err", kind: "count", value: "0"})
		}
	}

	byKind := map[string][]asynqMetricEntry{}
	for _, e := range entries {
		byKind[e.kind] = append(byKind[e.kind], e)
	}
	// Stabile Reihenfolge: SCAN liefert die Schlüssel in beliebiger Folge, und
	// die Ergänzung oben läuft über eine Map. Ohne Sortierung wechselt die
	// Ausgabe bei jedem Abruf ihre Zeilenreihenfolge.
	for kind := range byKind {
		es := byKind[kind]
		sort.Slice(es, func(i, j int) bool {
			if es[i].task != es[j].task {
				return es[i].task < es[j].task
			}
			return es[i].result < es[j].result
		})
	}
	return byKind
}

// queueSnapshot ist der Zustand einer Warteschlange, wie ihn der Asynq-
// Inspector meldet. Eigener Typ, damit die Formatierung ohne laufendes Redis
// prüfbar ist — vorher steckte sie in der Schleife, die den Inspector abfragt,
// und war damit nur gegen eine echte Redis-Instanz testbar.
type queueSnapshot struct {
	Name     string
	Pending  int
	Active   int
	Retry    int
	Archived int
}

// collectQueueSnapshots fragt den Asynq-Inspector nach allen Warteschlangen.
// Fehler werden geloggt, brechen die Metrik-Ausgabe aber nicht ab.
func (h *Handler) collectQueueSnapshots() []queueSnapshot {
	inspector := asynq.NewInspector(redisopt.Asynq(h.redisOpt))
	defer func() { _ = inspector.Close() }()

	queues, err := inspector.Queues()
	if err != nil {
		log.Error().Err(err).Msg("metrics: list asynq queues")
		return nil
	}
	snaps := make([]queueSnapshot, 0, len(queues))
	for _, name := range queues {
		info, err := inspector.GetQueueInfo(name)
		if err != nil {
			log.Error().Err(err).Str("queue", name).Msg("metrics: get queue info")
			continue
		}
		snaps = append(snaps, queueSnapshot{
			Name:     name,
			Pending:  info.Pending,
			Active:   info.Active,
			Retry:    info.Retry,
			Archived: info.Archived,
		})
	}
	return snaps
}

// writeQueueMetrics schreibt die drei Warteschlangen-Familien im Prometheus-
// Textformat.
//
// R1-19-02: vorher gab es nur `vakt_queue_depth` als `Pending + Active`.
// Ein Auftrag, der dauerhaft fehlschlägt, wandert aber von `Pending` nach
// `Retry` und von dort nach `Archived` — beides zählte die Kennzahl nicht,
// also meldete jede Warteschlange 0, während live 8 Wiederholungs- und 20
// Archiv-Einträge lagen. Ein wachsender Rückstand war damit unsichtbar.
//
// Bewusst DREI Kennzahlen statt einer Summe, weil es drei verschiedene
// Zustände sind:
//
//   - depth (pending + active) — Arbeit, die noch ansteht. Läuft von selbst
//     leer. Ein Schwellwert darauf misst Last.
//   - retry — fehlgeschlagen, wird aber erneut versucht. Vorübergehend; ein
//     dauerhaft erhöhter Wert bedeutet, dass etwas nicht durchkommt.
//   - archived — endgültig aufgegeben, nach allen Versuchen. Läuft NIE von
//     selbst leer, sondern nur durch einen Menschen (`asynq queue purge`).
//
// Archivierte Aufträge dürfen deshalb nicht in `depth` einfließen: Sie würden
// den Wert dauerhaft anheben und jeden Lastschwellwert unbrauchbar machen —
// entweder er feuert für immer, oder er wird so hoch gesetzt, dass er nie
// feuert. Aus demselben Grund bleibt `depth` bei seiner alten Bedeutung: ein
// bestehendes Zabbix-Item würde sonst still etwas anderes messen als vorher.
func writeQueueMetrics(w io.Writer, snaps []queueSnapshot) {
	fmt.Fprintln(w, "# HELP vakt_queue_depth Asynq queue depth by queue name (pending + active)")
	fmt.Fprintln(w, "# TYPE vakt_queue_depth gauge")
	for _, s := range snaps {
		fmt.Fprintf(w, "vakt_queue_depth{queue=%q} %d\n", s.Name, s.Pending+s.Active)
	}

	fmt.Fprintln(w, "# HELP vakt_queue_retry Asynq jobs awaiting a retry after a failure, by queue name")
	fmt.Fprintln(w, "# TYPE vakt_queue_retry gauge")
	for _, s := range snaps {
		fmt.Fprintf(w, "vakt_queue_retry{queue=%q} %d\n", s.Name, s.Retry)
	}

	fmt.Fprintln(w, "# HELP vakt_queue_archived Asynq jobs given up on after exhausting all retries, by queue name")
	fmt.Fprintln(w, "# TYPE vakt_queue_archived gauge")
	for _, s := range snaps {
		fmt.Fprintf(w, "vakt_queue_archived{queue=%q} %d\n", s.Name, s.Archived)
	}
}
