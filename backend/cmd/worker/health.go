// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/hibiken/asynq"
)

// dbPinger is the minimum surface buildHealthHandlers needs from the DB
// pool. Used both by the running worker (real pgxpool) and the unit tests
// (in-memory stub).
type dbPinger interface {
	Ping(ctx context.Context) error
}

// queueInspector is the minimum surface for Asynq's inspector that we use.
// Mockable in tests; in production it is *asynq.Inspector.
type queueInspector interface {
	Queues() ([]string, error)
	GetQueueInfo(qname string) (*asynq.QueueInfo, error)
}

// buildHealthHandlers wires the health endpoints onto a fresh mux and returns
// it.  Keeping the construction in a pure function (no goroutines, no global
// state, no http.Server) makes the handlers unit-testable with a
// httptest.ResponseRecorder.
//
// /health        — liveness; always 200 while the process is up
// /health/ready  — readiness; 200 only if DB + queue transport both reachable
// /health/queue  — JSON snapshot of all queues, useful for dashboards
// /metrics       — Prometheus text format; exposes vakt_worker_up gauge
//
// Audit P1-5 closure: the original endpoint did not probe Asynq, so a Redis
// outage or backed-up queue did not surface in the readiness signal.
func buildHealthHandlers(db dbPinger, inspector queueInspector) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", liveness)
	mux.HandleFunc("/health/ready", readiness(db, inspector))
	mux.HandleFunc("/health/queue", queueStats(inspector))
	mux.HandleFunc("/metrics", workerMetrics(db, inspector))
	return mux
}

func liveness(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func readiness(db dbPinger, inspector queueInspector) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := db.Ping(r.Context()); err != nil {
			http.Error(w, `{"status":"unavailable","component":"database"}`, http.StatusServiceUnavailable)
			return
		}
		if inspector != nil {
			if _, err := inspector.Queues(); err != nil {
				http.Error(w, `{"status":"unavailable","component":"queue"}`, http.StatusServiceUnavailable)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}
}

// workerMetrics serves a minimal Prometheus text-format /metrics endpoint for
// the worker process.  It emits vakt_worker_up (1 = ready, 0 = degraded) by
// running the same DB + queue probe that /health/ready uses.
//
// No external Prometheus client library is used — metrics are written directly
// in the Prometheus text exposition format (version 0.0.4), consistent with
// the pattern in internal/shared/metrics/handler.go.
func workerMetrics(db dbPinger, inspector queueInspector) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		up := 1
		if err := db.Ping(r.Context()); err != nil {
			up = 0
		} else if inspector != nil {
			if _, err := inspector.Queues(); err != nil {
				up = 0
			}
		}

		_, _ = w.Write([]byte("# HELP vakt_worker_up Worker health status: 1=healthy, 0=degraded\n"))
		_, _ = w.Write([]byte("# TYPE vakt_worker_up gauge\n"))
		_, _ = w.Write([]byte("vakt_worker_up " + strconv.Itoa(up) + "\n"))
	}
}

func queueStats(inspector queueInspector) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if inspector == nil {
			_, _ = w.Write([]byte(`{}`))
			return
		}
		queues, err := inspector.Queues()
		if err != nil {
			http.Error(w, `{"status":"unavailable","component":"queue"}`, http.StatusServiceUnavailable)
			return
		}
		payload := make(map[string]any, len(queues))
		for _, q := range queues {
			info, infoErr := inspector.GetQueueInfo(q)
			if infoErr != nil {
				payload[q] = map[string]string{"error": infoErr.Error()}
				continue
			}
			payload[q] = map[string]int{
				"size":      info.Size,
				"pending":   info.Pending,
				"active":    info.Active,
				"retry":     info.Retry,
				"archived":  info.Archived,
				"completed": info.Completed,
				"scheduled": info.Scheduled,
			}
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(payload)
	}
}
