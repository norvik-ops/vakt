// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package github

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

func testGitHubClient(srv *httptest.Server) *Client {
	return &Client{
		token:      "test-token",
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

// makeGHASServer returns a test server that handles dependabot/secret/code scanning
// endpoints using paths relative to a test-overridden base. The Client.doGitHubRequest
// uses full URLs, so we set up a redirect mux.
func makeGHASServer(
	dependabotAlerts []any,
	secretAlerts []any,
	codeAlerts []any,
) *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/repos/myorg/myrepo/dependabot/alerts", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(dependabotAlerts) //nolint:errcheck
	})
	mux.HandleFunc("/repos/myorg/myrepo/secret-scanning/alerts", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(secretAlerts) //nolint:errcheck
	})
	mux.HandleFunc("/repos/myorg/myrepo/code-scanning/alerts", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(codeAlerts) //nolint:errcheck
	})

	return httptest.NewServer(mux)
}

// rewriteURLClient wraps an HTTP client to rewrite github.com URLs to the test server.
func rewriteURLClient(srv *httptest.Server) *http.Client {
	return &http.Client{
		Timeout: 5 * time.Second,
		Transport: &rewriteTransport{base: srv.URL, inner: http.DefaultTransport},
	}
}

type rewriteTransport struct {
	base  string
	inner http.RoundTripper
}

func (rt *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Replace https://api.github.com with test server URL
	newURL := *req.URL
	newURL.Scheme = "http"
	newURL.Host = req.URL.Host
	if req.URL.Host == "api.github.com" {
		newURL.Host = ""
		rewritten, _ := http.NewRequestWithContext(req.Context(), req.Method, rt.base+req.URL.Path+"?"+req.URL.RawQuery, req.Body)
		rewritten.Header = req.Header
		return rt.inner.RoundTrip(rewritten)
	}
	return rt.inner.RoundTrip(req)
}

func TestListDependabotAlerts_ReturnsAlerts(t *testing.T) {
	alerts := []any{
		map[string]any{
			"number": 1,
			"state":  "open",
			"dependency": map[string]any{
				"package": map[string]any{"name": "lodash"},
			},
			"security_advisory": map[string]any{
				"summary":  "Prototype pollution in lodash",
				"severity": "high",
				"identifiers": []any{
					map[string]any{"value": "CVE-2019-10744"},
				},
			},
			"security_vulnerability": map[string]any{
				"severity": "high",
			},
		},
	}

	srv := makeGHASServer(alerts, []any{}, []any{})
	defer srv.Close()

	client := &Client{token: "test", httpClient: rewriteURLClient(srv)}
	result, err := client.ListDependabotAlerts(context.Background(), "myorg", "myrepo")

	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, 1, result[0].Number)
	assert.Equal(t, "high", result[0].Severity)
	assert.Equal(t, "lodash", result[0].Package)
	assert.Contains(t, result[0].CVEIDs, "CVE-2019-10744")
}

func TestListDependabotAlerts_GHASNotEnabled_Returns403(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	client := &Client{token: "test", httpClient: rewriteURLClient(srv)}
	result, err := client.ListDependabotAlerts(context.Background(), "myorg", "myrepo")

	require.NoError(t, err, "403 should be treated as silent skip, not error")
	assert.Nil(t, result, "should return nil for GHAS-disabled repos")
}

func TestListSecretScanningAlerts_ReturnsAlerts(t *testing.T) {
	secrets := []any{
		map[string]any{
			"number":      5,
			"state":       "open",
			"secret_type": "github_personal_access_token",
		},
	}

	srv := makeGHASServer([]any{}, secrets, []any{})
	defer srv.Close()

	client := &Client{token: "test", httpClient: rewriteURLClient(srv)}
	result, err := client.ListSecretScanningAlerts(context.Background(), "myorg", "myrepo")

	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, 5, result[0].Number)
	assert.Equal(t, "github_personal_access_token", result[0].SecretType)
	assert.Equal(t, "myorg/myrepo", result[0].Repo)
}

func TestListCodeScanningAlerts_FiltersLowSeverity(t *testing.T) {
	codeAlerts := []any{
		map[string]any{
			"number": 10,
			"state":  "open",
			"rule": map[string]any{
				"id":                     "js/sql-injection",
				"security_severity_level": "critical",
				"severity":               "error",
			},
			"tool": map[string]any{"name": "CodeQL"},
		},
		map[string]any{
			"number": 11,
			"state":  "open",
			"rule": map[string]any{
				"id":                     "js/xss",
				"security_severity_level": "low", // should be filtered out
				"severity":               "warning",
			},
			"tool": map[string]any{"name": "CodeQL"},
		},
	}

	srv := makeGHASServer([]any{}, []any{}, codeAlerts)
	defer srv.Close()

	client := &Client{token: "test", httpClient: rewriteURLClient(srv)}
	result, err := client.ListCodeScanningAlerts(context.Background(), "myorg", "myrepo")

	require.NoError(t, err)
	require.Len(t, result, 1, "only high+critical should be returned")
	assert.Equal(t, 10, result[0].Number)
	assert.Equal(t, "critical", result[0].Severity)
	assert.Equal(t, "CodeQL", result[0].Tool)
}
