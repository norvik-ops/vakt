package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/matharnica/vakt/internal/config"
)

func testConfig() *config.Config {
	cfg, _ := config.Load()
	if cfg == nil {
		cfg = &config.Config{
			Version:        "0.1.0",
			APIPort:        "8080",
			ModulesEnabled: "vaktscan,vaktcomply,vaktvault,vaktaware,vaktprivacy,vakthr",
		}
	}
	return cfg
}

// TestHealthEndpoint deckt das in v0.6.2 erweiterte /health-Schema ab.
// Frontend (useDemoMode, Login.tsx, Layout.tsx) hängt an den Feldern demo,
// sso_enabled und version — Pflichtfelder gemäß ADR-0017 + openapi.yaml.
// S46-3: response now includes `components` — existing fields must not be removed.
func TestHealthEndpoint(t *testing.T) {
	e := setupEcho(context.Background(), testConfig())
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Verify the mandatory frontend-contract fields are present (ADR-0017).
	// We use Contains checks rather than JSONEq so that adding fields never
	// breaks this test — only removing mandatory ones does.
	body := rec.Body.String()
	assert.Contains(t, body, `"status"`)
	assert.Contains(t, body, `"version"`)
	assert.Contains(t, body, `"demo"`)
	assert.Contains(t, body, `"sso_enabled"`)
	// S46-3: components block must be present
	assert.Contains(t, body, `"components"`)
}

// TestHealthEndpointRequiredFields verifies that demo, sso_enabled, and version
// are always present in the /health response — these are quality-gate fields
// checked before every release tag (see docs/dev/api-contract-checklist.md).
func TestHealthEndpointRequiredFields(t *testing.T) {
	cfg := &config.Config{
		Version:    "1.2.3",
		DemoSeed:   true,
		CasdoorURL: "https://auth.example.com",
	}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	// Call healthHandler directly to test the response structure.
	e := setupEcho(context.Background(), cfg)
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	body := rec.Body.String()
	assert.Contains(t, body, `"demo":true`, "demo field must be present")
	assert.Contains(t, body, `"version":"1.2.3"`, "version field must be present")
}
