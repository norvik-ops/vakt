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
			ModulesEnabled: "secpulse,secvitals,secvault,secreflex,secprivacy",
		}
	}
	return cfg
}

// TestHealthEndpoint deckt das in v0.6.2 erweiterte /health-Schema ab.
// Frontend (useDemoMode, Login.tsx, Layout.tsx) hängt an den Feldern demo,
// sso_enabled und version — Pflichtfelder gemäß ADR-0017 + openapi.yaml.
func TestHealthEndpoint(t *testing.T) {
	e := setupEcho(context.Background(), testConfig())
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{
		"status":      "ok",
		"version":     "0.1.0",
		"demo":        false,
		"sso_enabled": false
	}`, rec.Body.String())
}
