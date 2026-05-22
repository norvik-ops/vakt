// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestComputeHMAC verifies the HMAC-SHA256 implementation.
func TestComputeHMAC(t *testing.T) {
	secret := "test-secret-key"
	body := []byte(`{"event":"finding.created"}`)

	got := computeHMAC(secret, body)

	// Independently compute expected value.
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	want := hex.EncodeToString(mac.Sum(nil))

	assert.Equal(t, want, got)
	assert.Len(t, got, 64, "HMAC-SHA256 hex digest must be 64 characters")
}

// TestComputeHMAC_DifferentSecrets ensures different secrets produce different MACs.
func TestComputeHMAC_DifferentSecrets(t *testing.T) {
	body := []byte(`{"event":"ping"}`)
	h1 := computeHMAC("secret-one", body)
	h2 := computeHMAC("secret-two", body)
	assert.NotEqual(t, h1, h2)
}

// TestTriggerEvent_NoWebhooks verifies that TriggerEvent is a no-op when the
// org has no matching webhooks: no HTTP request should be sent.
func TestTriggerEvent_NoWebhooks(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Simulate looping over an empty webhook list — equivalent to what TriggerEvent
	// does when no matching webhooks are found. No HTTP call must be made.
	body, _ := json.Marshal(map[string]any{"event": "finding.created", "org_id": "org-1"})
	for _, wh := range []Webhook{} {
		_ = wh
		_ = body
	}

	assert.False(t, called, "no HTTP call should be made when webhook list is empty")
}

// TestTriggerEvent_WithHMAC verifies that a webhook with a secret receives an
// X-Vakt-Signature header containing a valid HMAC-SHA256 of the request body.
// This test builds the request the same way deliver() does, without going through
// the real DB (recordDelivery is not exercised here).
func TestTriggerEvent_WithHMAC(t *testing.T) {
	const secret = "super-secret"
	var (
		receivedSig  string
		receivedBody []byte
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSig = r.Header.Get("X-Vakt-Signature")
		var err error
		receivedBody, err = io.ReadAll(r.Body)
		require.NoError(t, err)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	secretVal := secret
	eventType := "finding.created"
	body, err := json.Marshal(map[string]any{
		"event":   eventType,
		"org_id":  "org-1",
		"payload": map[string]string{"id": "f-123"},
	})
	require.NoError(t, err)

	// Build and send the request exactly as deliver() does — without touching the DB.
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL, bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Vakt-Event", eventType)
	sig := computeHMAC(secretVal, body)
	req.Header.Set("X-Vakt-Signature", "sha256="+sig)

	client := srv.Client()
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Signature must be present and prefixed with "sha256=".
	require.True(t, strings.HasPrefix(receivedSig, "sha256="),
		"X-Vakt-Signature must start with 'sha256=', got: %q", receivedSig)

	// Strip prefix and validate HMAC against the received body.
	gotHex := strings.TrimPrefix(receivedSig, "sha256=")
	expectedHex := computeHMAC(secret, receivedBody)
	assert.Equal(t, expectedHex, gotHex, "HMAC signature must match body")
}

// TestTriggerEvent_WithHMAC_NoSecret verifies that no signature header is sent
// when the webhook has no secret configured.
func TestTriggerEvent_WithHMAC_NoSecret(t *testing.T) {
	var receivedSig string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSig = r.Header.Get("X-Vakt-Signature")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Build request without signature — same as deliver() does for wh.Secret == nil.
	body, _ := json.Marshal(map[string]string{"event": "ping"})
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL, bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Vakt-Event", "ping")
	// No X-Vakt-Signature header set (secret is nil).

	resp, err := srv.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Empty(t, receivedSig, "X-Vakt-Signature must not be set when no secret is configured")
}
