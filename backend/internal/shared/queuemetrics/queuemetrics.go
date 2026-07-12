// Package queuemetrics holds a tiny in-process counter for Asynq enqueue
// failures on the producer (API) side.
//
// Why in-process and not Redis-backed like the worker's task metrics
// (internal/shared/metrics/asynq_middleware.go): an enqueue error usually means
// the write to Redis itself failed (NOAUTH on a --requirepass Redis, connection
// refused, …). Recording that failure *into* the same Redis would fail for the
// same reason, so the signal would vanish exactly when it matters most.
//
// S122-B3 (INC-01): the NOAUTH bug (missing Password on the Asynq clients) made
// every enqueue fail, yet the handlers still returned 201 and the queue-depth
// gauge stayed flat — Zabbix was blind to a silent DSGVO breach-bridge / DSR /
// scan / campaign backlog. This counter makes that failure observable:
// vakt_asynq_enqueue_errors_total{queue} on the API's /metrics endpoint, with a
// Zabbix trigger firing on a non-zero count.
package queuemetrics

import "sync"

var (
	mu     sync.Mutex
	counts = make(map[string]int64)
)

// RecordError increments the enqueue-error counter for the given queue.
// An empty queue name is normalised to "default" (asynq's default queue).
func RecordError(queue string) {
	if queue == "" {
		queue = "default"
	}
	mu.Lock()
	counts[queue]++
	mu.Unlock()
}

// Snapshot returns a copy of the current per-queue error counts.
func Snapshot() map[string]int64 {
	mu.Lock()
	defer mu.Unlock()
	out := make(map[string]int64, len(counts))
	for k, v := range counts {
		out[k] = v
	}
	return out
}
