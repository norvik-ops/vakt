package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/matharnica/vakt/internal/config"
)

// TestSetupEchoEmptySecretKey confirms that setupEcho itself does not validate
// SecretKey — that check lives in main() so the HTTP layer can always be
// constructed for testing.  This is a compile + smoke test only.
func TestSetupEchoEmptySecretKey(t *testing.T) {
	cfg := &config.Config{
		Version:        "0.1.0",
		APIPort:        "8080",
		ModulesEnabled: "",
		SecretKey:      "", // intentionally empty
	}

	// setupEcho must not panic with an empty key.
	e, _ := setupEcho(context.Background(), cfg)
	assert.NotNil(t, e)

	// Health endpoint must still respond.
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}
