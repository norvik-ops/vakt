package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sechealth-app/sechealth/internal/auth"
	"github.com/sechealth-app/sechealth/internal/config"
)

// TestOIDCLogin_CasdoorNotConfigured checks that the sentinel error is returned
// when CasdoorURL is empty — no network calls should happen.
func TestOIDCLogin_CasdoorNotConfigured(t *testing.T) {
	svc := auth.NewService(nil, nil, mustKey(t))
	cfg := &config.Config{CasdoorURL: ""}

	_, err := svc.OIDCLogin(context.Background(), cfg, "google", "code", "state")
	require.Error(t, err)
	assert.ErrorIs(t, err, auth.ErrCasdoorNotConfigured)
}

// TestSAMLLogin_CasdoorNotConfigured verifies the same guard for SAML.
func TestSAMLLogin_CasdoorNotConfigured(t *testing.T) {
	svc := auth.NewService(nil, nil, mustKey(t))
	cfg := &config.Config{CasdoorURL: ""}

	_, err := svc.SAMLLogin(context.Background(), cfg, "base64response", "relayState")
	require.Error(t, err)
	assert.ErrorIs(t, err, auth.ErrCasdoorNotConfigured)
}

// TestOIDCLogin_CasdoorTokenError checks that a Casdoor token endpoint error
// propagates correctly without panicking.
func TestOIDCLogin_CasdoorTokenError(t *testing.T) {
	// Spin up a mock Casdoor that returns an error on the token endpoint.
	mux := http.NewServeMux()
	mux.HandleFunc("/api/login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "invalid_client",
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	svc := auth.NewService(nil, nil, mustKey(t))
	cfg := &config.Config{
		CasdoorURL:          srv.URL,
		CasdoorClientID:     "test-client",
		CasdoorClientSecret: "test-secret",
		FrontendURL:         "http://localhost:5173",
	}

	_, err := svc.OIDCLogin(context.Background(), cfg, "google", "code123", "state123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid_client")
}

// TestSAMLLogin_CasdoorError checks that a Casdoor SAML endpoint error propagates.
func TestSAMLLogin_CasdoorError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/saml/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "invalid_saml_response",
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	svc := auth.NewService(nil, nil, mustKey(t))
	cfg := &config.Config{
		CasdoorURL:  srv.URL,
		FrontendURL: "http://localhost:5173",
	}

	_, err := svc.SAMLLogin(context.Background(), cfg, "badresponse", "relay")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid_saml_response")
}

// mustKey is a test helper that generates a symmetric key from the shared test hex key.
func mustKey(t *testing.T) auth.SymmetricKey {
	t.Helper()
	key, err := auth.GenerateSymmetricKey(testHexKey)
	require.NoError(t, err)
	return key
}
