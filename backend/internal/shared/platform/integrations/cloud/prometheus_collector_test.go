// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package cloud

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// prometheusQueryResponse builds a minimal Prometheus query API response.
func prometheusQueryResponse(value float64) []byte {
	payload := map[string]any{
		"status": "success",
		"data": map[string]any{
			"resultType": "vector",
			"result": []any{
				map[string]any{
					"metric": map[string]any{},
					"value":  []any{1234567890.0, "95.5"},
				},
			},
		},
	}
	if value < 90 {
		payload["data"].(map[string]any)["result"].([]any)[0].(map[string]any)["value"] = []any{1234567890.0, "75.0"}
	}
	b, _ := json.Marshal(payload)
	return b
}

func TestPrometheusCollector_CollectsUptime(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/query", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(prometheusQueryResponse(95.5))
	})

	mux.HandleFunc("/api/v1/targets", func(w http.ResponseWriter, _ *http.Request) {
		payload := map[string]any{
			"status": "success",
			"data": map[string]any{
				"activeTargets": []any{
					map[string]any{"health": "up", "labels": map[string]any{"job": "api"}},
					map[string]any{"health": "up", "labels": map[string]any{"job": "db"}},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload) //nolint:errcheck
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ew := &mockEvidenceWriter{
		controls: []ControlMatch{{ID: "ctrl-avail", Title: "Availability Monitoring"}},
	}
	collector := &PrometheusCollector{
		evidence:   ew,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}

	n, err := collector.Collect(context.Background(), "org-1", PrometheusConfig{
		PrometheusURL: srv.URL,
	})

	require.NoError(t, err)
	assert.Equal(t, 2, n, "expected 2 evidence items: uptime + target health")
}

func TestPrometheusCollector_CriticalAlertsEvidence(t *testing.T) {
	promMux := http.NewServeMux()
	promMux.HandleFunc("/api/v1/query", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(prometheusQueryResponse(99.0))
	})
	promMux.HandleFunc("/api/v1/targets", func(w http.ResponseWriter, _ *http.Request) {
		payload := map[string]any{
			"status": "success",
			"data":   map[string]any{"activeTargets": []any{}},
		}
		json.NewEncoder(w).Encode(payload) //nolint:errcheck
	})
	promSrv := httptest.NewServer(promMux)
	defer promSrv.Close()

	alertMux := http.NewServeMux()
	alertMux.HandleFunc("/api/v2/alerts", func(w http.ResponseWriter, _ *http.Request) {
		// Alertmanager returns a JSON array directly
		alerts := []any{
			map[string]any{
				"labels":      map[string]any{"alertname": "DiskFull", "severity": "critical"},
				"startsAt":    "2026-06-09T10:00:00Z",
				"fingerprint": "abc123",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(alerts) //nolint:errcheck
	})
	alertMux.HandleFunc("/api/v2/status", func(w http.ResponseWriter, _ *http.Request) {
		payload := map[string]any{
			"versionInfo": map[string]any{"version": "0.27.0"},
			"uptime":      "2026-06-01T00:00:00Z",
		}
		json.NewEncoder(w).Encode(payload) //nolint:errcheck
	})
	alertSrv := httptest.NewServer(alertMux)
	defer alertSrv.Close()

	ew := &mockEvidenceWriter{
		controls: []ControlMatch{{ID: "ctrl-mon", Title: "Monitoring"}},
	}
	collector := &PrometheusCollector{
		evidence:   ew,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}

	n, err := collector.Collect(context.Background(), "org-1", PrometheusConfig{
		PrometheusURL:   promSrv.URL,
		AlertmanagerURL: alertSrv.URL,
	})

	require.NoError(t, err)
	assert.GreaterOrEqual(t, n, 3, "expected evidence for uptime, targets, alerts + alertmanager status")

	hasAlertEvidence := false
	for _, a := range ew.added {
		if a.source == prometheusSource {
			hasAlertEvidence = true
		}
	}
	assert.True(t, hasAlertEvidence)
}

func TestPrometheusCollector_LowUptimeWarning(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/query", func(w http.ResponseWriter, _ *http.Request) {
		// Return 75% uptime — below 90% threshold
		payload := map[string]any{
			"status": "success",
			"data": map[string]any{
				"resultType": "vector",
				"result": []any{
					map[string]any{"metric": map[string]any{}, "value": []any{0.0, "75.0"}},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload) //nolint:errcheck
	})
	mux.HandleFunc("/api/v1/targets", func(w http.ResponseWriter, _ *http.Request) {
		payload := map[string]any{
			"status": "success",
			"data":   map[string]any{"activeTargets": []any{}},
		}
		json.NewEncoder(w).Encode(payload) //nolint:errcheck
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ew := &mockEvidenceWriter{
		controls: []ControlMatch{{ID: "ctrl-avail", Title: "Availability"}},
	}
	collector := &PrometheusCollector{
		evidence:   ew,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}

	n, err := collector.Collect(context.Background(), "org-1", PrometheusConfig{
		PrometheusURL: srv.URL,
	})

	require.NoError(t, err)
	assert.GreaterOrEqual(t, n, 1)

	// Verify warning status is set in the evidence details
	hasWarning := false
	for _, a := range ew.added {
		if a.source == prometheusSource {
			hasWarning = true
		}
	}
	assert.True(t, hasWarning)
}

func TestPrometheusCollector_NoAlertmanagerURL(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/query", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(prometheusQueryResponse(99.0))
	})
	mux.HandleFunc("/api/v1/targets", func(w http.ResponseWriter, _ *http.Request) {
		payload := map[string]any{
			"status": "success",
			"data":   map[string]any{"activeTargets": []any{}},
		}
		json.NewEncoder(w).Encode(payload) //nolint:errcheck
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ew := &mockEvidenceWriter{
		controls: []ControlMatch{{ID: "ctrl-mon", Title: "Monitoring"}},
	}
	collector := &PrometheusCollector{
		evidence:   ew,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}

	// No AlertmanagerURL → alert collection skipped entirely
	n, err := collector.Collect(context.Background(), "org-1", PrometheusConfig{
		PrometheusURL:   srv.URL,
		AlertmanagerURL: "",
	})

	require.NoError(t, err)
	assert.Equal(t, 2, n, "expected exactly 2 items: uptime + target health, no alertmanager")
}
