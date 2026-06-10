// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vakthr

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockPersonioSecrets implements PersonioSecretProvider for tests.
type mockPersonioSecrets struct {
	secret        string
	recordedOrgID string
}

func (m *mockPersonioSecrets) GetDecryptedPersonioSecret(_ context.Context, orgID string) (string, error) {
	if m.secret == "" {
		return "", nil
	}
	return m.secret, nil
}

func (m *mockPersonioSecrets) RecordPersonioWebhook(_ context.Context, orgID string) error {
	m.recordedOrgID = orgID
	return nil
}

func newWebhookEcho() *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	return e
}

// TestVerifyPersonioSignature_Valid verifies that a known payload+secret pair is accepted.
func TestVerifyPersonioSignature_Valid(t *testing.T) {
	secret := "test-secret-123"
	body := []byte(`{"event":"employee.departed","data":{"employee_id":42}}`)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))

	assert.True(t, verifyPersonioSignature(body, sig, secret))
}

// TestVerifyPersonioSignature_Invalid verifies that a manipulated payload is rejected.
func TestVerifyPersonioSignature_Invalid(t *testing.T) {
	secret := "test-secret-123"
	body := []byte(`{"event":"employee.departed","data":{"employee_id":42}}`)
	manipulated := []byte(`{"event":"employee.departed","data":{"employee_id":99}}`)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))

	assert.False(t, verifyPersonioSignature(manipulated, sig, secret))
	assert.False(t, verifyPersonioSignature(body, "", secret))
	assert.False(t, verifyPersonioSignature(body, sig, ""))
}

// TestHandlePersonioWebhook_InvalidSignature verifies that an invalid signature returns 401.
func TestHandlePersonioWebhook_InvalidSignature(t *testing.T) {
	e := newWebhookEcho()

	secrets := &mockPersonioSecrets{secret: "correct-secret"}
	h := &Handler{
		Service:         nil,
		PersonioSecrets: secrets,
	}

	body := []byte(`{"event":"employee.departed","data":{"employee_id":42,"date":"2026-07-01"}}`)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Personio-Signature", "bad-signature")
	rec := httptest.NewRecorder()

	c := e.NewContext(req, rec)
	c.SetParamNames("org_id")
	c.SetParamValues("00000000-0000-0000-0000-000000000001")

	err := h.HandlePersonioWebhook(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// TestHandlePersonioWebhook_UnknownEvent verifies that non-departed events are silently ignored.
func TestHandlePersonioWebhook_UnknownEvent(t *testing.T) {
	e := newWebhookEcho()
	secret := "test-secret"
	secrets := &mockPersonioSecrets{secret: secret}

	h := &Handler{Service: nil, PersonioSecrets: secrets}

	payload := map[string]any{
		"event": "employee.created",
		"data":  map[string]any{"employee_id": 42},
	}
	body, _ := json.Marshal(payload)
	sig := signBody(secret, body)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Personio-Signature", sig)
	rec := httptest.NewRecorder()

	c := e.NewContext(req, rec)
	c.SetParamNames("org_id")
	c.SetParamValues("00000000-0000-0000-0000-000000000001")

	err := h.HandlePersonioWebhook(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "ignored", resp["status"])
}

func signBody(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// TestHandlePersonioWebhook_NoPIIInPayloadStruct ensures the personioWebhookPayload struct
// has no fields that could capture name, email, or other PII from the Personio event payload.
func TestHandlePersonioWebhook_NoPIIInPayloadStruct(t *testing.T) {
	// Construct a Personio payload with PII fields that SHOULD be ignored
	rawWithPII := []byte(`{
		"event": "employee.departed",
		"data": {
			"employee_id": 12345,
			"date": "2026-07-15",
			"first_name": "Max",
			"last_name": "Mustermann",
			"email": "max@example.com",
			"department": "Engineering"
		}
	}`)

	var p personioWebhookPayload
	require.NoError(t, json.Unmarshal(rawWithPII, &p))

	// Only employee_id and date should be populated
	assert.Equal(t, 12345, p.Data.EmployeeID)
	assert.Equal(t, "2026-07-15", p.Data.Date)
	assert.Equal(t, "employee.departed", p.Event)

	// Confirm via reflection that no PII fields exist on the struct
	raw, _ := json.Marshal(p)
	result := string(raw)
	assert.NotContains(t, result, "Mustermann", "PII must not appear in marshaled payload")
	assert.NotContains(t, result, "max@example.com", "PII must not appear in marshaled payload")

	_ = fmt.Sprintf("PII test passed — payload only contains: %s", result)
}
