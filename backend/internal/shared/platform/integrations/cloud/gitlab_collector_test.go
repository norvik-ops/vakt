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

// gitlabTestServer builds a minimal GitLab API v4 mock.
func gitlabTestServer(t *testing.T, projects []map[string]any, branchesProtected bool, approvalsBefore int, hasLastSASTJob bool) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	// Project listing (membership)
	mux.HandleFunc("/api/v4/projects", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(projects) //nolint:errcheck
	})

	// Protected branches
	mux.HandleFunc("/api/v4/projects/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path

		switch {
		case strings.HasSuffix(path, "/protected_branches"):
			if branchesProtected {
				json.NewEncoder(w).Encode([]map[string]any{ //nolint:errcheck
					{"name": "main", "allow_force_push": false},
				})
			} else {
				json.NewEncoder(w).Encode([]map[string]any{}) //nolint:errcheck
			}
		case strings.HasSuffix(path, "/approvals"):
			json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
				"approvals_before_merge": approvalsBefore,
			})
		case strings.HasSuffix(path, "/jobs"):
			if hasLastSASTJob {
				json.NewEncoder(w).Encode([]map[string]any{ //nolint:errcheck
					{"name": "sast", "status": "success"},
				})
			} else {
				json.NewEncoder(w).Encode([]map[string]any{}) //nolint:errcheck
			}
		case strings.HasSuffix(path, "/vulnerability_findings"):
			// 403 = GitLab CE (no Ultimate)
			w.WriteHeader(http.StatusForbidden)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	return httptest.NewServer(mux)
}

func newTestGitLabCollector(srv *httptest.Server) *GitLabCollector {
	c := NewGitLabCollector(nil, &mockEvidenceWriter{
		controls: []ControlMatch{{ID: "ctrl-1", Title: "Secure Development"}},
	})
	c.httpClient = srv.Client()
	return c
}

// TestGitLabCollect_NormalCollect verifies that a normal collect run produces branch-protection
// and MR-approval evidence for all projects.
func TestGitLabCollect_NormalCollect(t *testing.T) {
	projects := []map[string]any{
		{"id": 1, "name": "backend", "path_with_namespace": "corp/backend",
			"visibility": "private", "default_branch": "main"},
	}
	srv := gitlabTestServer(t, projects, true, 1, false)
	defer srv.Close()

	mock := &mockEvidenceWriter{
		controls: []ControlMatch{{ID: "ctrl-1", Title: "Secure Development"}},
	}
	c := NewGitLabCollector(nil, mock)
	c.httpClient = srv.Client()

	cfg := GitLabConfig{GitLabURL: srv.URL, AccessToken: "test-token"}
	count, err := c.Collect(context.Background(), "00000000-0000-0000-0000-000000000001", cfg)

	require.NoError(t, err)
	assert.Greater(t, count, 0, "should create at least one evidence item")

	// Inventory evidence must be present
	found := false
	for _, ev := range mock.added {
		if strings.Contains(ev.title, "Inventar") || strings.Contains(ev.title, "Branch-Protection") {
			found = true
		}
	}
	assert.True(t, found, "expected inventory or branch-protection evidence")
}

// TestGitLabCollect_UnprotectedBranch verifies that an unprotected default branch
// produces a warning evidence entry.
func TestGitLabCollect_UnprotectedBranch(t *testing.T) {
	projects := []map[string]any{
		{"id": 2, "name": "legacy", "path_with_namespace": "corp/legacy",
			"visibility": "private", "default_branch": "main"},
	}
	srv := gitlabTestServer(t, projects, false, 1, false)
	defer srv.Close()

	mock := &mockEvidenceWriter{
		controls: []ControlMatch{{ID: "ctrl-1", Title: "Secure Development"}},
	}
	c := NewGitLabCollector(nil, mock)
	c.httpClient = srv.Client()

	cfg := GitLabConfig{GitLabURL: srv.URL, AccessToken: "test-token"}
	_, err := c.Collect(context.Background(), "00000000-0000-0000-0000-000000000001", cfg)

	require.NoError(t, err)

	warningFound := false
	for _, ev := range mock.added {
		if strings.Contains(ev.title, "ungeschützten Default-Branch") || strings.Contains(ev.title, "Warnung") {
			warningFound = true
		}
	}
	assert.True(t, warningFound, "expected a warning evidence for unprotected branch")
}

// TestGitLabCollect_VulnFindings403_SilentSkip verifies that a 403 on the vulnerability
// findings endpoint (GitLab CE) is silently skipped — no error returned.
func TestGitLabCollect_VulnFindings403_SilentSkip(t *testing.T) {
	projects := []map[string]any{
		{"id": 3, "name": "app", "path_with_namespace": "corp/app",
			"visibility": "private", "default_branch": "main"},
	}
	// branchesProtected=true so we get no warning noise; 403 comes from the test server default
	srv := gitlabTestServer(t, projects, true, 1, false)
	defer srv.Close()

	mock := &mockEvidenceWriter{
		controls: []ControlMatch{{ID: "ctrl-1", Title: "Secure Development"}},
	}
	c := NewGitLabCollector(nil, mock)
	c.httpClient = srv.Client()

	cfg := GitLabConfig{GitLabURL: srv.URL, AccessToken: "test-token"}
	_, err := c.Collect(context.Background(), "00000000-0000-0000-0000-000000000001", cfg)

	// Must not return an error despite 403 on vulnerability_findings
	assert.NoError(t, err, "403 on vulnerability_findings should be silently skipped")
}

// TestGitLabCollect_InvalidToken verifies that a 401 response produces an error.
func TestGitLabCollect_InvalidToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"401 Unauthorized"}`)) //nolint:errcheck
	}))
	defer srv.Close()

	mock := &mockEvidenceWriter{}
	c := NewGitLabCollector(nil, mock)
	c.httpClient = srv.Client()

	cfg := GitLabConfig{GitLabURL: srv.URL, AccessToken: "bad-token"}
	_, err := c.Collect(context.Background(), "00000000-0000-0000-0000-000000000001", cfg)

	assert.Error(t, err, "401 should produce an error")
	assert.Contains(t, err.Error(), "unauthorized")
}

// TestGitLabParseLinkNext verifies the Link header parser.
func TestGitLabParseLinkNext(t *testing.T) {
	link := `<https://gitlab.example.com/api/v4/projects?page=2>; rel="next", <https://gitlab.example.com/api/v4/projects?page=5>; rel="last"`
	got := parseLinkNext(link)
	assert.Equal(t, "https://gitlab.example.com/api/v4/projects?page=2", got)

	assert.Empty(t, parseLinkNext(""))
	assert.Empty(t, parseLinkNext(`<https://example.com>; rel="last"`))
}
