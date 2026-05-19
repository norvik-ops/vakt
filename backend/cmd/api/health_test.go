package main

import (
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

func TestHealthEndpoint(t *testing.T) {
	e := setupEcho(testConfig())
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"status":"ok"}`, rec.Body.String())
}
