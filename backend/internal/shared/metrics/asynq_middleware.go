// Package metrics asynq_middleware records per-task-type timings into Redis
// so that the API's /metrics endpoint can expose them in Prometheus format.
//
// We avoid a dedicated metrics endpoint on the worker (no extra Caddy route,
// no extra scrape target) by funneling everything through Redis. Counters
// are aggregated, not histogrammed — for MVP this is good enough to detect
// "this job has gotten slower" or "this job started failing". Histograms
// (p50/p95/p99) can be added later if needed.
package metrics

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

const (
	asynqMetricPrefix = "metric:asynq:"
	// R1-19-02: `count` und `duration_ms_sum` tragen bewusst KEINE Ablauffrist
	// mehr.
	//
	// Vorher liefen beide nach sieben Tagen ab. Für einen kumulativen Zähler
	// ist das kein Aufräumen, sondern Datenverlust: Die Zeitreihe
	// `vakt_asynq_jobs_total{result="err"}` verschwand aus /metrics, sobald
	// sieben Tage lang nichts fehlgeschlagen war. Auf eine Zeitreihe, die es
	// nicht gibt, kann kein Alarm auswerten — und Fehler sind selten, der
	// Verfall traf also ausgerechnet den Zähler, auf den es ankommt.
	//
	// Die alte Begründung („ein Worker, der eine Woche stillsteht, soll keine
	// alten Daten behalten") trägt bei einem Zähler nicht: Ob ein Wert frisch
	// ist, leitet Prometheus aus dem Verlauf ab (`rate()`), nicht aus dem
	// Verfall des Schlüssels. Die Zahl der Schlüssel ist durch die Menge der
	// Task-Typen im Code begrenzt und wächst nicht mit der Laufzeit.
	//
	// Bestehende Schlüssel in einer laufenden Redis-Instanz tragen noch die
	// alte Frist. Deshalb steht in der Pipeline ein ausdrückliches PERSIST
	// statt eines weggelassenen EXPIRE: Ein weggelassenes EXPIRE würde die
	// vorhandene Frist stehen lassen, der Zähler wäre beim nächsten Ablauf
	// erneut weg. PERSIST nimmt sie beim ersten Lauf aktiv weg.
	//
	// `duration_ms_max` behält eine Frist: Das ist ein Momentanwert, und ein
	// Höchstwert von vor Monaten führt in die Irre. Sein Fehlen schadet
	// dagegen nichts — darauf alarmiert niemand.
	asynqMaxMetricTTL = 90 * 24 * time.Hour
)

// AsynqInstrumentingMiddleware records every task execution into Redis:
//   - metric:asynq:count:<task>:<result>           — INCR
//   - metric:asynq:duration_ms_sum:<task>:<result> — INCRBY duration
//   - metric:asynq:duration_ms_max:<task>:<result> — MAX semantic (best-effort)
//
// Result is "ok" or "err". Task type is sanitised to be Prometheus-label-safe.
//
// The middleware does not block on Redis: a Redis hiccup must never affect
// task execution. We log the error and move on.
func AsynqInstrumentingMiddleware(rdb *redis.Client) asynq.MiddlewareFunc {
	return func(next asynq.Handler) asynq.Handler {
		return asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
			start := time.Now()
			err := next.ProcessTask(ctx, t)
			durationMs := time.Since(start).Milliseconds()

			result := "ok"
			if err != nil {
				result = "err"
			}
			taskType := sanitiseLabel(t.Type())

			recordAsynqMetric(ctx, rdb, taskType, result, durationMs)
			return err
		})
	}
}

func recordAsynqMetric(ctx context.Context, rdb *redis.Client, taskType, result string, durationMs int64) {
	// Detach from request ctx so the metric write outlives the task ctx
	// being cancelled on shutdown / timeout. Bound by 1s.
	mCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = ctx // intentionally unused — we deliberately use a fresh ctx

	keyCount := fmt.Sprintf("%scount:%s:%s", asynqMetricPrefix, taskType, result)
	keySum := fmt.Sprintf("%sduration_ms_sum:%s:%s", asynqMetricPrefix, taskType, result)
	keyMax := fmt.Sprintf("%sduration_ms_max:%s:%s", asynqMetricPrefix, taskType, result)

	// Kein Expire auf keyCount/keySum — siehe asynqMaxMetricTTL oben.
	pipe := rdb.Pipeline()
	pipe.Incr(mCtx, keyCount)
	pipe.IncrBy(mCtx, keySum, durationMs)
	pipe.Persist(mCtx, keyCount)
	pipe.Persist(mCtx, keySum)
	if _, err := pipe.Exec(mCtx); err != nil {
		log.Warn().Err(err).Str("task", taskType).Msg("asynq metric: pipeline failed")
		return
	}

	// MAX is best-effort: we read the current max and SETNX if our value is
	// larger. Race against concurrent workers is acceptable — it can briefly
	// under-report but converges.
	currentStr, _ := rdb.Get(mCtx, keyMax).Result()
	var current int64
	_, _ = fmt.Sscanf(currentStr, "%d", &current)
	if durationMs > current {
		if err := rdb.Set(mCtx, keyMax, durationMs, asynqMaxMetricTTL).Err(); err != nil {
			log.Warn().Err(err).Str("task", taskType).Msg("asynq metric: max update failed")
		}
	}
}

// sanitiseLabel turns "vaktcomply:dora_deadline_status" into
// "vaktcomply_dora_deadline_status" so it is a valid Prometheus label value
// when used inside curly braces (we still quote it, but this avoids ambiguity).
func sanitiseLabel(s string) string {
	r := strings.NewReplacer(":", "_", "-", "_", " ", "_", ".", "_")
	return r.Replace(s)
}
