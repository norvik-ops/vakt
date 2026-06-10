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

func entraIDTokenHandler(t *testing.T) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"test-token","expires_in":3600}`)
	}
}

func entraIDEmptyValueHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"value":[]}`)
	}
}

// TestEntraIDCollect_NormalFlow verifies that a full collect creates 5 evidence items.
func TestEntraIDCollect_NormalFlow(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/testtenant/oauth2/v2.0/token", entraIDTokenHandler(t))
	mux.HandleFunc("/v1.0/reports/credentialUserRegistrationDetails", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"value":[
			{"isMfaRegistered":true},
			{"isMfaRegistered":true},
			{"isMfaRegistered":false}
		]}`)
	})
	mux.HandleFunc("/v1.0/identity/conditionalAccess/policies", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"value":[
			{"displayName":"MFA for All","state":"enabled"},
			{"displayName":"Block Legacy Auth","state":"enabled"},
			{"displayName":"Draft","state":"disabled"}
		]}`)
	})
	mux.HandleFunc("/v1.0/identityProtection/riskyUsers", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"value":[]}`)
	})
	mux.HandleFunc("/v1.0/roleManagement/directory/roleAssignments", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"value":[
			{"roleDefinition":{"displayName":"Global Administrator"}},
			{"roleDefinition":{"displayName":"User Administrator"}}
		]}`)
	})
	mux.HandleFunc("/v1.0/users", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"value":[]}`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ew := &mockEvidenceWriter{controls: []ControlMatch{{ID: "ctrl-1", Title: "Access"}}}
	collector := &EntraIDCollector{
		evidence:     ew,
		loginBaseURL: srv.URL,
		graphBaseURL: srv.URL,
		httpClient:   &http.Client{Timeout: 5 * time.Second},
	}

	count, err := collector.Collect(context.Background(), "org-1", EntraIDConfig{
		TenantID:     "testtenant",
		ClientID:     "client-1",
		ClientSecret: "secret",
	})

	require.NoError(t, err)
	assert.Equal(t, 5, count, "expected 5 evidence items (one per collection type)")

	// Verify MFA evidence contains enrollment percentage
	var mfaEvidence struct{ title, source, controlID string }
	for _, a := range ew.added {
		if strings.Contains(a.title, "MFA") {
			mfaEvidence = a
			break
		}
	}
	assert.Contains(t, mfaEvidence.title, "67%")
	assert.Equal(t, entraidSource, mfaEvidence.source)
}

// TestEntraIDCollect_LowMFAWarning verifies that MFA below 80% produces warning in evidence.
func TestEntraIDCollect_LowMFAWarning(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/testtenant/oauth2/v2.0/token", entraIDTokenHandler(t))
	mux.HandleFunc("/v1.0/reports/credentialUserRegistrationDetails", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// 1 of 5 = 20%
		fmt.Fprint(w, `{"value":[
			{"isMfaRegistered":true},
			{"isMfaRegistered":false},
			{"isMfaRegistered":false},
			{"isMfaRegistered":false},
			{"isMfaRegistered":false}
		]}`)
	})
	mux.HandleFunc("/v1.0/identity/conditionalAccess/policies", entraIDEmptyValueHandler())
	mux.HandleFunc("/v1.0/identityProtection/riskyUsers", entraIDEmptyValueHandler())
	mux.HandleFunc("/v1.0/roleManagement/directory/roleAssignments", entraIDEmptyValueHandler())
	mux.HandleFunc("/v1.0/users", entraIDEmptyValueHandler())

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ew := &mockEvidenceWriter{controls: []ControlMatch{{ID: "ctrl-1", Title: "Access"}}}
	collector := &EntraIDCollector{
		evidence:     ew,
		loginBaseURL: srv.URL,
		graphBaseURL: srv.URL,
		httpClient:   &http.Client{Timeout: 5 * time.Second},
	}

	_, err := collector.Collect(context.Background(), "org-1", EntraIDConfig{
		TenantID: "testtenant", ClientID: "c", ClientSecret: "s",
	})
	require.NoError(t, err)

	// MFA at 20% → title should contain percentage
	found := false
	for _, a := range ew.added {
		if strings.Contains(a.title, "MFA") && strings.Contains(a.title, "20%") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected MFA evidence with 20% enrollment")
}

// TestEntraIDCollect_RiskyUsersWarning verifies that risky users produce evidence.
func TestEntraIDCollect_RiskyUsersWarning(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/testtenant/oauth2/v2.0/token", entraIDTokenHandler(t))
	mux.HandleFunc("/v1.0/reports/credentialUserRegistrationDetails", entraIDEmptyValueHandler())
	mux.HandleFunc("/v1.0/identity/conditionalAccess/policies", entraIDEmptyValueHandler())
	mux.HandleFunc("/v1.0/identityProtection/riskyUsers", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"value":[
			{"id":"user1","riskLevel":"high"},
			{"id":"user2","riskLevel":"medium"}
		]}`)
	})
	mux.HandleFunc("/v1.0/roleManagement/directory/roleAssignments", entraIDEmptyValueHandler())
	mux.HandleFunc("/v1.0/users", entraIDEmptyValueHandler())

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ew := &mockEvidenceWriter{controls: []ControlMatch{{ID: "ctrl-1", Title: "Monitoring"}}}
	collector := &EntraIDCollector{
		evidence:     ew,
		loginBaseURL: srv.URL,
		graphBaseURL: srv.URL,
		httpClient:   &http.Client{Timeout: 5 * time.Second},
	}

	_, err := collector.Collect(context.Background(), "org-1", EntraIDConfig{
		TenantID: "testtenant", ClientID: "c", ClientSecret: "s",
	})
	require.NoError(t, err)

	var riskyTitle string
	for _, a := range ew.added {
		if strings.Contains(a.title, "Risky") {
			riskyTitle = a.title
			break
		}
	}
	assert.Contains(t, riskyTitle, "2", "expected risky user count in evidence title")
}

// TestEntraIDCollect_AuthError verifies that a 401 on token request returns an error immediately.
func TestEntraIDCollect_AuthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":"invalid_client","error_description":"Client authentication failed"}`)
	}))
	defer srv.Close()

	ew := &mockEvidenceWriter{}
	collector := &EntraIDCollector{
		evidence:     ew,
		loginBaseURL: srv.URL,
		graphBaseURL: srv.URL,
		httpClient:   &http.Client{Timeout: 5 * time.Second},
	}

	_, err := collector.Collect(context.Background(), "org-1", EntraIDConfig{
		TenantID: "testtenant", ClientID: "c", ClientSecret: "bad",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "entraid auth")
	assert.Empty(t, ew.added)
}

// TestEntraIDCollect_Pagination verifies that @odata.nextLink is followed.
func TestEntraIDCollect_Pagination(t *testing.T) {
	callCount := 0
	var srvURL string

	mux := http.NewServeMux()
	mux.HandleFunc("/testtenant/oauth2/v2.0/token", entraIDTokenHandler(t))
	mux.HandleFunc("/v1.0/reports/credentialUserRegistrationDetails", func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		if r.URL.RawQuery == "" {
			// First call — return nextLink
			fmt.Fprintf(w, `{"value":[{"isMfaRegistered":true}],"@odata.nextLink":"%s/v1.0/reports/credentialUserRegistrationDetails?$skiptoken=abc"}`,
				srvURL)
		} else {
			// Second call — no nextLink
			fmt.Fprint(w, `{"value":[{"isMfaRegistered":true}]}`)
		}
	})
	mux.HandleFunc("/v1.0/identity/conditionalAccess/policies", entraIDEmptyValueHandler())
	mux.HandleFunc("/v1.0/identityProtection/riskyUsers", entraIDEmptyValueHandler())
	mux.HandleFunc("/v1.0/roleManagement/directory/roleAssignments", entraIDEmptyValueHandler())
	mux.HandleFunc("/v1.0/users", entraIDEmptyValueHandler())

	srv := httptest.NewServer(mux)
	defer srv.Close()
	srvURL = srv.URL

	ew := &mockEvidenceWriter{controls: []ControlMatch{{ID: "ctrl-1", Title: "Auth"}}}
	collector := &EntraIDCollector{
		evidence:     ew,
		loginBaseURL: srv.URL,
		graphBaseURL: srv.URL,
		httpClient:   &http.Client{Timeout: 5 * time.Second},
	}

	_, err := collector.Collect(context.Background(), "org-1", EntraIDConfig{
		TenantID: "testtenant", ClientID: "c", ClientSecret: "s",
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, callCount, 2, "should follow @odata.nextLink pagination")
}

// ensure json package is used in test
var _ = json.Marshal
