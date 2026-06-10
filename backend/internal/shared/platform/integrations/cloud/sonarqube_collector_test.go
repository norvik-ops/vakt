// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package cloud

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sonarTestServer builds a minimal SonarQube API mock.
func sonarTestServer(t *testing.T, qualityGateStatus string, hotspots []map[string]any) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/api/projects/search", func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{
			"paging":     map[string]any{"pageIndex": 1, "pageSize": 500, "total": 1},
			"components": []map[string]any{{"key": "corp:backend", "name": "Backend"}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	})

	mux.HandleFunc("/api/qualitygates/project_status", func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{
			"projectStatus": map[string]any{"status": qualityGateStatus},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	})

	mux.HandleFunc("/api/hotspots/search", func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{
			"paging":   map[string]any{"total": len(hotspots)},
			"hotspots": hotspots,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	})

	mux.HandleFunc("/api/issues/search", func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{
			"total":  0,
			"issues": []map[string]any{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	})

	return httptest.NewServer(mux)
}

func newTestSonarCollector(srv *httptest.Server) (*SonarQubeCollector, *mockEvidenceWriter) {
	mock := &mockEvidenceWriter{
		controls: []ControlMatch{{ID: "ctrl-sq", Title: "Secure Development"}},
	}
	c := NewSonarQubeCollector(nil, mock)
	c.httpClient = srv.Client()
	return c, mock
}

// TestSonarQube_NormalCollect verifies that a normal collect run produces quality gate evidence.
func TestSonarQube_NormalCollect(t *testing.T) {
	srv := sonarTestServer(t, "OK", nil)
	defer srv.Close()

	c, mock := newTestSonarCollector(srv)
	cfg := SonarQubeConfig{BaseURL: srv.URL, Token: "test-token"}
	count, err := c.Collect(context.Background(), "00000000-0000-0000-0000-000000000001", cfg)

	require.NoError(t, err)
	assert.Greater(t, count, 0)

	// Should have "alle grün" summary
	found := false
	for _, ev := range mock.added {
		if strings.Contains(ev.title, "grün") || strings.Contains(ev.title, "Inventar") {
			found = true
		}
	}
	assert.True(t, found, "expected quality gate OK summary evidence")
}

// TestSonarQube_FailedQualityGate verifies that a FAILED quality gate produces a warning.
func TestSonarQube_FailedQualityGate(t *testing.T) {
	srv := sonarTestServer(t, "ERROR", nil)
	defer srv.Close()

	c, mock := newTestSonarCollector(srv)
	cfg := SonarQubeConfig{BaseURL: srv.URL, Token: "test-token"}
	_, err := c.Collect(context.Background(), "00000000-0000-0000-0000-000000000001", cfg)

	require.NoError(t, err)

	warningFound := false
	for _, ev := range mock.added {
		if strings.Contains(ev.title, "FAILED") || strings.Contains(ev.title, "Quality Gate") {
			warningFound = true
		}
	}
	assert.True(t, warningFound, "expected quality gate FAILED warning evidence")
}

// TestSonarQube_InvalidToken verifies that a 401 response produces an error.
func TestSonarQube_InvalidToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c, _ := newTestSonarCollector(srv)
	cfg := SonarQubeConfig{BaseURL: srv.URL, Token: "bad-token"}
	_, err := c.Collect(context.Background(), "00000000-0000-0000-0000-000000000001", cfg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unauthorized")
}

// TestSonarQube_URLHash verifies that the sonarURLHash function is stable and
// strips trailing slashes consistently.
func TestSonarQube_URLHash(t *testing.T) {
	h1 := sonarURLHash("https://sonarqube.example.com/")
	h2 := sonarURLHash("https://sonarqube.example.com")
	assert.Equal(t, h1, h2, "URL hash must be slash-insensitive")
	assert.NotEmpty(t, h1)
}
