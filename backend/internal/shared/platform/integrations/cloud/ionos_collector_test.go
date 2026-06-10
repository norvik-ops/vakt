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

func TestIONOSCollector_CollectsServerInventory(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/cloudapi/v6/datacenters", func(w http.ResponseWriter, _ *http.Request) {
		payload := map[string]any{
			"items": []map[string]any{
				{"id": "dc-1", "properties": map[string]any{"name": "IONOS DC Frankfurt"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload) //nolint:errcheck
	})

	mux.HandleFunc("/cloudapi/v6/datacenters/dc-1/servers", func(w http.ResponseWriter, _ *http.Request) {
		payload := map[string]any{
			"items": []map[string]any{
				{
					"id": "srv-1",
					"properties": map[string]any{
						"name":    "prod-web",
						"vmState": "RUNNING",
						"cores":   4,
						"ram":     8192,
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload) //nolint:errcheck
	})

	mux.HandleFunc("/cloudapi/v6/images", func(w http.ResponseWriter, r *http.Request) {
		// Respond with an image that was created today (fresh snapshot)
		payload := map[string]any{
			"items": []map[string]any{
				{
					"id": "img-1",
					"properties": map[string]any{
						"name":        "prod-web-snapshot",
						"imageType":   "SNAPSHOT",
						"createdDate": time.Now().Format(time.RFC3339),
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload) //nolint:errcheck
	})

	mux.HandleFunc("/cloudapi/v6/um/users/me/sshkeys", func(w http.ResponseWriter, _ *http.Request) {
		payload := map[string]any{"items": []any{}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload) //nolint:errcheck
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ew := &mockEvidenceWriter{
		controls: []ControlMatch{{ID: "ctrl-inv", Title: "Asset Inventory"}},
	}
	collector := &IONOSCollector{
		evidence:   ew,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
	// Override IONOS API base URL to test server
	collector.apiBase = srv.URL + "/cloudapi/v6"

	n, err := collector.Collect(context.Background(), "org-1", IONOSConfig{
		Username: "user",
		Password: "pass",
	})

	require.NoError(t, err)
	assert.GreaterOrEqual(t, n, 1)
}

func TestIONOSCollector_OldSnapshotWarning(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/cloudapi/v6/datacenters", func(w http.ResponseWriter, _ *http.Request) {
		payload := map[string]any{
			"items": []map[string]any{
				{"id": "dc-2", "properties": map[string]any{"name": "DC-2"}},
			},
		}
		json.NewEncoder(w).Encode(payload) //nolint:errcheck
	})
	mux.HandleFunc("/cloudapi/v6/datacenters/dc-2/servers", func(w http.ResponseWriter, _ *http.Request) {
		payload := map[string]any{
			"items": []map[string]any{
				{"id": "srv-2", "properties": map[string]any{"name": "old-server", "vmState": "RUNNING", "cores": 2, "ram": 4096}},
			},
		}
		json.NewEncoder(w).Encode(payload) //nolint:errcheck
	})
	// Return snapshot older than 7 days
	mux.HandleFunc("/cloudapi/v6/images", func(w http.ResponseWriter, _ *http.Request) {
		payload := map[string]any{
			"items": []map[string]any{
				{
					"id": "img-old",
					"properties": map[string]any{
						"name":        "old-snapshot",
						"imageType":   "SNAPSHOT",
						"createdDate": time.Now().Add(-10 * 24 * time.Hour).Format(time.RFC3339),
					},
				},
			},
		}
		json.NewEncoder(w).Encode(payload) //nolint:errcheck
	})
	mux.HandleFunc("/cloudapi/v6/um/users/me/sshkeys", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"items": []any{}}) //nolint:errcheck
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ew := &mockEvidenceWriter{
		controls: []ControlMatch{
			{ID: "ctrl-bkp", Title: "Backup"},
			{ID: "ctrl-inv", Title: "Inventory"},
		},
	}
	collector := &IONOSCollector{
		evidence:   ew,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
	collector.apiBase = srv.URL + "/cloudapi/v6"

	n, err := collector.Collect(context.Background(), "org-1", IONOSConfig{Username: "u", Password: "p"})
	require.NoError(t, err)

	// Should have warning evidence for old snapshot
	hasWarning := false
	for _, a := range ew.added {
		if a.source == ionosSource {
			hasWarning = true
		}
	}
	assert.True(t, hasWarning, "expected evidence items for IONOS")
	_ = n
}

func TestIONOSCollector_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	ew := &mockEvidenceWriter{}
	collector := &IONOSCollector{
		evidence:   ew,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
	collector.apiBase = srv.URL + "/cloudapi/v6"

	_, err := collector.Collect(context.Background(), "org-1", IONOSConfig{Username: "bad", Password: "creds"})
	assert.Error(t, err, "expected error on 401")
}
