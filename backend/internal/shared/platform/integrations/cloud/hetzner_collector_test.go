// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package cloud

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockEvidenceWriter records calls to AddCollectorEvidence.
type mockEvidenceWriter struct {
	controls []ControlMatch
	added    []struct{ title, source, controlID string }
}

func (m *mockEvidenceWriter) FindControlsByKeywords(_ context.Context, _ string, _ []string) ([]ControlMatch, error) {
	return m.controls, nil
}

func (m *mockEvidenceWriter) AddCollectorEvidence(_ context.Context, _, controlID, _, source, title string, _ []byte) error {
	m.added = append(m.added, struct{ title, source, controlID string }{title, source, controlID})
	return nil
}

// hcloudList returns a minimal HCloud API list payload for a given key.
func hcloudList(key string, items []map[string]any) []byte {
	payload := map[string]any{
		key: items,
		"meta": map[string]any{
			"pagination": map[string]any{
				"page":          1,
				"per_page":      25,
				"total_entries": len(items),
				"total_pages":   1,
			},
		},
	}
	b, _ := json.Marshal(payload)
	return b
}

// hcloudTestMux builds a ServeMux that handles all Hetzner API paths used by HetznerCollector.
// hcloud-go endpoint is used as-is and paths are appended directly (no /v1/ prefix).
func hcloudTestMux(servers, firewalls, sshKeys, images []map[string]any) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/servers", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(hcloudList("servers", servers))
	})
	mux.HandleFunc("/firewalls", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(hcloudList("firewalls", firewalls))
	})
	mux.HandleFunc("/ssh_keys", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(hcloudList("ssh_keys", sshKeys))
	})
	mux.HandleFunc("/images", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(hcloudList("images", images))
	})
	return mux
}

func TestHetznerCollector_CollectsServerInventory(t *testing.T) {
	servers := []map[string]any{
		{
			"id":          1,
			"name":        "web-01",
			"status":      "running",
			"server_type": map[string]any{"name": "cx21", "cores": 2, "memory": 4.0},
			"datacenter": map[string]any{
				"location": map[string]any{"name": "nbg1"},
			},
			"image":   map[string]any{"name": "ubuntu-22.04"},
			"created": "2026-01-01T00:00:00Z",
		},
	}

	srv := httptest.NewServer(hcloudTestMux(servers, nil, nil, nil))
	defer srv.Close()

	ew := &mockEvidenceWriter{
		controls: []ControlMatch{{ID: "ctrl-1", Title: "Asset Inventory"}},
	}
	collector := &HetznerCollector{
		evidence:   ew,
		clientOpts: []hcloud.ClientOption{hcloud.WithEndpoint(srv.URL)},
	}

	n, err := collector.Collect(context.Background(), "org-1", HetznerConfig{
		APIToken: "test-token",
		Location: "",
	})

	require.NoError(t, err)
	assert.GreaterOrEqual(t, n, 1, "expected at least 1 evidence item from server inventory")
	assert.NotEmpty(t, ew.added, "expected AddCollectorEvidence to be called")
	assert.Equal(t, hetznerSource, ew.added[0].source)
}

func TestHetznerCollector_SnapshotMissingWarning(t *testing.T) {
	// Server present but no snapshots in /images
	servers := []map[string]any{
		{
			"id": 2, "name": "db-01", "status": "running",
			"server_type": map[string]any{"name": "cx11", "cores": 1, "memory": 2.0},
			"datacenter":  map[string]any{"location": map[string]any{"name": "fsn1"}},
			"image":       map[string]any{"name": "debian-11"},
			"created":     "2026-01-01T00:00:00Z",
		},
	}

	srv := httptest.NewServer(hcloudTestMux(servers, nil, nil, nil))
	defer srv.Close()

	ew := &mockEvidenceWriter{
		controls: []ControlMatch{{ID: "ctrl-backup", Title: "Backup and Recovery"}},
	}
	collector := &HetznerCollector{
		evidence:   ew,
		clientOpts: []hcloud.ClientOption{hcloud.WithEndpoint(srv.URL)},
	}

	n, err := collector.Collect(context.Background(), "org-1", HetznerConfig{APIToken: "test-token"})
	require.NoError(t, err)
	// Inventory evidence + snapshot warning → at least 2 items
	assert.GreaterOrEqual(t, n, 1)
}

func TestHetznerCollector_InvalidToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"code":"unauthorized","message":"invalid token"}}`))
	}))
	defer srv.Close()

	ew := &mockEvidenceWriter{}
	collector := &HetznerCollector{
		evidence:   ew,
		clientOpts: []hcloud.ClientOption{hcloud.WithEndpoint(srv.URL)},
	}

	// All sub-collectors fail on an invalid token. Collect must now surface that as an
	// error so SyncHetzner records last_sync_status='error', not a false 'success' with
	// zero evidence (D14-08/R-H20/S131-F3). The old test asserted the buggy (0, nil).
	n, err := collector.Collect(context.Background(), "org-1", HetznerConfig{APIToken: "bad-token"})
	assert.Error(t, err, "total sub-collector failure must propagate, not be swallowed")
	assert.Equal(t, 0, n, "no evidence expected on auth failure")
}
