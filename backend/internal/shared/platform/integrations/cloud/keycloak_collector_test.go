// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func keycloakTokenHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"test-token","expires_in":300}`)
	}
}

func buildKeycloakMux(users []any, passwordPolicy string, ssoMaxLifespan int) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/realms/company/protocol/openid-connect/token", keycloakTokenHandler())

	// Realm representation (password policy + session config)
	mux.HandleFunc("/admin/realms/company", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		payload := map[string]any{
			"realm":                  "company",
			"passwordPolicy":         passwordPolicy,
			"ssoSessionMaxLifespan":  ssoMaxLifespan,
		}
		json.NewEncoder(w).Encode(payload) //nolint:errcheck
	})

	// Users list (first page only for simplicity)
	mux.HandleFunc("/admin/realms/company/users", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(users) //nolint:errcheck
	})

	// Per-user credentials (no OTP by default)
	mux.HandleFunc("/admin/realms/company/users/", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		last := parts[len(parts)-1]
		if last == "credentials" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `[]`)
		} else if last == "realm" {
			// role-mappings/realm
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `[]`)
		} else {
			http.NotFound(w, r)
		}
	})

	return mux
}

// TestKeycloakCollect_NormalFlow verifies a successful collect creates evidence.
func TestKeycloakCollect_NormalFlow(t *testing.T) {
	users := []any{
		map[string]any{"id": "u1", "username": "alice", "createdTimestamp": time.Now().Add(-10 * 24 * time.Hour).UnixMilli()},
		map[string]any{"id": "u2", "username": "bob", "createdTimestamp": time.Now().Add(-5 * 24 * time.Hour).UnixMilli()},
	}

	srv := httptest.NewServer(buildKeycloakMux(users, "length(12) and upperCase(1)", 28800))
	defer srv.Close()

	ew := &mockEvidenceWriter{controls: []ControlMatch{{ID: "ctrl-1", Title: "Auth"}}}
	collector := &KeycloakCollector{
		evidence:   ew,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}

	count, err := collector.Collect(context.Background(), "org-1", KeycloakConfig{
		KeycloakURL:  srv.URL,
		Realm:        "company",
		ClientID:     "vakt-collector",
		ClientSecret: "secret",
	})

	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, 4, "expected at least 4 evidence items")
}

// TestKeycloakCollect_WeakPasswordPolicyWarning checks that a short password policy triggers warning.
func TestKeycloakCollect_WeakPasswordPolicyWarning(t *testing.T) {
	srv := httptest.NewServer(buildKeycloakMux([]any{}, "length(6)", 3600))
	defer srv.Close()

	ew := &mockEvidenceWriter{controls: []ControlMatch{{ID: "ctrl-1", Title: "Password"}}}
	collector := &KeycloakCollector{
		evidence:   ew,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}

	_, err := collector.Collect(context.Background(), "org-1", KeycloakConfig{
		KeycloakURL: srv.URL, Realm: "company", ClientID: "c", ClientSecret: "s",
	})
	require.NoError(t, err)

	var pwTitle string
	for _, a := range ew.added {
		if strings.Contains(a.title, "Password") || strings.Contains(a.title, "Policy") {
			pwTitle = a.title
			break
		}
	}
	assert.NotEmpty(t, pwTitle, "expected password policy evidence")
}

// TestKeycloakCollect_LongSessionWarning checks that SSO timeout >8h gets evidence.
func TestKeycloakCollect_LongSessionWarning(t *testing.T) {
	longTimeout := 12 * 3600 // 12 hours in seconds

	srv := httptest.NewServer(buildKeycloakMux([]any{}, "length(12)", longTimeout))
	defer srv.Close()

	ew := &mockEvidenceWriter{controls: []ControlMatch{{ID: "ctrl-1", Title: "Session"}}}
	collector := &KeycloakCollector{
		evidence:   ew,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}

	_, err := collector.Collect(context.Background(), "org-1", KeycloakConfig{
		KeycloakURL: srv.URL, Realm: "company", ClientID: "c", ClientSecret: "s",
	})
	require.NoError(t, err)

	var sessionTitle string
	for _, a := range ew.added {
		if strings.Contains(a.title, "Session") || strings.Contains(a.title, "Timeout") {
			sessionTitle = a.title
			break
		}
	}
	assert.Contains(t, sessionTitle, "12h", "expected session timeout hours in evidence")
}

// TestKeycloakCollect_AuthError verifies that a 401 token response returns an error.
func TestKeycloakCollect_AuthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":"unauthorized_client"}`)
	}))
	defer srv.Close()

	ew := &mockEvidenceWriter{}
	collector := &KeycloakCollector{
		evidence:   ew,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}

	_, err := collector.Collect(context.Background(), "org-1", KeycloakConfig{
		KeycloakURL: srv.URL, Realm: "company", ClientID: "c", ClientSecret: "bad",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "keycloak auth")
}

// --- parsePasswordPolicyLength unit tests ---

func TestParsePasswordPolicyLength(t *testing.T) {
	tests := []struct {
		policy   string
		expected int
	}{
		{"length(12) and upperCase(1) and digits(1)", 12},
		{"length(8)", 8},
		{"", 0},
		{"upperCase(1) and digits(1)", 0},
		{"length(0)", 0},
	}

	for _, tc := range tests {
		got := parsePasswordPolicyLength(tc.policy)
		assert.Equal(t, tc.expected, got, "policy: %q", tc.policy)
	}
}
