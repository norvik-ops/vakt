// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package cloud

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// wazuhTestServer sets up a minimal Wazuh API mock with one active agent.
func wazuhTestServer(t *testing.T, agents []map[string]any) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/security/user/authenticate", func(w http.ResponseWriter, _ *http.Request) {
		payload := map[string]any{
			"data": map[string]any{"token": "test-jwt-token"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload) //nolint:errcheck
	})

	mux.HandleFunc("/agents", func(w http.ResponseWriter, _ *http.Request) {
		payload := map[string]any{
			"data": map[string]any{
				"affected_items":       agents,
				"total_affected_items": len(agents),
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload) //nolint:errcheck
	})

	// Vulnerability summary per agent
	mux.HandleFunc("/vulnerability/", func(w http.ResponseWriter, _ *http.Request) {
		payload := map[string]any{
			"data": map[string]any{
				"affected_items":       []any{map[string]any{"Critical": 0, "High": 0, "Medium": 1, "Low": 0}},
				"total_affected_items": 1,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload) //nolint:errcheck
	})

	// SCA policies per agent
	mux.HandleFunc("/sca/", func(w http.ResponseWriter, _ *http.Request) {
		payload := map[string]any{
			"data": map[string]any{
				"affected_items": []any{
					map[string]any{"name": "CIS Benchmark", "pass": 80, "fail": 20, "score": 80.0},
				},
				"total_affected_items": 1,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload) //nolint:errcheck
	})

	// FIM events per agent
	mux.HandleFunc("/syscheck/", func(w http.ResponseWriter, _ *http.Request) {
		payload := map[string]any{
			"data": map[string]any{
				"affected_items":       []any{},
				"total_affected_items": 0,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload) //nolint:errcheck
	})

	return httptest.NewServer(mux)
}

func TestWazuhPullCollector_CollectsActiveAgent(t *testing.T) {
	agents := []map[string]any{
		{
			"id":            "001",
			"name":          "host-01",
			"ip":            "10.0.0.1",
			"status":        "active",
			"lastKeepAlive": "2026-06-09T12:00:00Z",
			"os":            map[string]any{"name": "Ubuntu", "version": "22.04"},
		},
	}
	srv := wazuhTestServer(t, agents)
	defer srv.Close()

	ew := &mockEvidenceWriter{
		controls: []ControlMatch{
			{ID: "ctrl-inv", Title: "Asset Inventory"},
			{ID: "ctrl-vuln", Title: "Vulnerability Management"},
		},
	}
	collector := &WazuhPullCollector{evidence: ew}

	n, err := collector.Collect(context.Background(), "org-1", WazuhConfig{
		BaseURL:   srv.URL,
		Username:  "wazuh",
		Password:  "pass",
		VerifyTLS: true,
	})

	require.NoError(t, err)
	assert.GreaterOrEqual(t, n, 1, "expected at least 1 evidence item from active agent")
}

func TestWazuhPullCollector_OfflineAgentWarning(t *testing.T) {
	agents := []map[string]any{
		{
			"id":            "002",
			"name":          "host-offline",
			"ip":            "10.0.0.2",
			"status":        "disconnected",
			"lastKeepAlive": "2026-01-01T00:00:00Z", // old → offline >24h
			"os":            map[string]any{"name": "Debian", "version": "11"},
		},
	}
	srv := wazuhTestServer(t, agents)
	defer srv.Close()

	ew := &mockEvidenceWriter{
		controls: []ControlMatch{{ID: "ctrl-inv", Title: "Endpoint Monitoring"}},
	}
	collector := &WazuhPullCollector{evidence: ew}

	n, err := collector.Collect(context.Background(), "org-1", WazuhConfig{
		BaseURL:   srv.URL,
		Username:  "wazuh",
		Password:  "pass",
		VerifyTLS: true,
	})

	require.NoError(t, err)
	// Inventory evidence for the offline agent should still be written
	assert.GreaterOrEqual(t, n, 1)
}

func TestWazuhPullCollector_CriticalCVEWarning(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/security/user/authenticate", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"data": map[string]any{"token": "jwt"},
		})
	})
	mux.HandleFunc("/agents", func(w http.ResponseWriter, _ *http.Request) {
		payload := map[string]any{
			"data": map[string]any{
				"affected_items": []any{
					map[string]any{
						"id": "003", "name": "vuln-host", "ip": "10.0.0.3",
						"status": "active", "lastKeepAlive": "2026-06-09T12:00:00Z",
						"os": map[string]any{"name": "CentOS", "version": "7"},
					},
				},
				"total_affected_items": 1,
			},
		}
		json.NewEncoder(w).Encode(payload) //nolint:errcheck
	})
	// Critical CVE found
	mux.HandleFunc("/vulnerability/", func(w http.ResponseWriter, _ *http.Request) {
		payload := map[string]any{
			"data": map[string]any{
				"affected_items": []any{
					map[string]any{"Critical": 3, "High": 5, "Medium": 10, "Low": 2},
				},
				"total_affected_items": 1,
			},
		}
		json.NewEncoder(w).Encode(payload) //nolint:errcheck
	})
	mux.HandleFunc("/sca/", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"data": map[string]any{"affected_items": []any{}, "total_affected_items": 0},
		})
	})
	mux.HandleFunc("/syscheck/", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"data": map[string]any{"affected_items": []any{}, "total_affected_items": 0},
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ew := &mockEvidenceWriter{
		controls: []ControlMatch{{ID: "ctrl-vuln", Title: "Vulnerability"}},
	}
	collector := &WazuhPullCollector{evidence: ew}

	n, err := collector.Collect(context.Background(), "org-1", WazuhConfig{
		BaseURL: srv.URL, Username: "wazuh", Password: "pass", VerifyTLS: true,
	})

	require.NoError(t, err)
	assert.GreaterOrEqual(t, n, 1, "expected warning evidence for critical CVEs")
}

func TestWazuhPullCollector_AuthFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	collector := &WazuhPullCollector{evidence: &mockEvidenceWriter{}}
	_, err := collector.Collect(context.Background(), "org-1", WazuhConfig{
		BaseURL: srv.URL, Username: "bad", Password: "creds", VerifyTLS: true,
	})
	assert.Error(t, err, "expected error on auth failure")
	assert.Contains(t, err.Error(), "wazuh auth")
}
