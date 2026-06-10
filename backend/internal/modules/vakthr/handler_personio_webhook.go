// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vakthr

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// PersonioSecretProvider abstracts reading the Personio webhook secret from the
// cloud integrations config so vakthr does not directly import the cloud package.
type PersonioSecretProvider interface {
	GetDecryptedPersonioSecret(ctx context.Context, orgID string) (string, error)
	RecordPersonioWebhook(ctx context.Context, orgID string) error
}

// personioWebhookPayload contains only the fields Vakt persists — name, email and
// other PII are intentionally absent to prevent accidental logging or persistence.
type personioWebhookPayload struct {
	Event string `json:"event"`
	Data  struct {
		EmployeeID int    `json:"employee_id"`
		Date       string `json:"date"` // "YYYY-MM-DD"
	} `json:"data"`
}

// HandlePersonioWebhook processes incoming Personio employee.departed webhooks.
// Route: POST /api/v1/vakthr/webhooks/personio/:org_id  (no auth middleware — HMAC only)
func (h *Handler) HandlePersonioWebhook(c echo.Context) error {
	orgID := c.Param("org_id")
	if orgID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "org_id required"})
	}

	// Read raw body before binding (needed for HMAC verification)
	rawBody, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "cannot read body"})
	}

	if h.PersonioSecrets == nil {
		log.Warn().Str("org_id", orgID).Msg("personio_webhook: no secret provider configured")
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "personio integration not configured"})
	}

	secret, err := h.PersonioSecrets.GetDecryptedPersonioSecret(c.Request().Context(), orgID)
	if err != nil || secret == "" {
		log.Warn().Str("org_id", orgID).Msg("personio_webhook: no secret configured, rejecting")
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "personio integration not configured"})
	}

	signature := c.Request().Header.Get("X-Personio-Signature")
	if !verifyPersonioSignature(rawBody, signature, secret) {
		log.Warn().Str("org_id", orgID).Msg("personio_webhook: invalid HMAC signature")
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid signature"})
	}

	var payload personioWebhookPayload
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
	}

	if payload.Event != "employee.departed" {
		return c.JSON(http.StatusOK, map[string]string{"status": "ignored"})
	}

	if payload.Data.EmployeeID == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "employee_id required"})
	}

	departureDate := time.Now().UTC()
	if payload.Data.Date != "" {
		if t, parseErr := time.Parse("2006-01-02", payload.Data.Date); parseErr == nil {
			departureDate = t
		}
	}

	if err := h.Service.TriggerPersonioOffboarding(c.Request().Context(), orgID,
		payload.Data.EmployeeID, departureDate); err != nil {
		log.Error().Err(err).Str("org_id", orgID).Int("personio_id", payload.Data.EmployeeID).
			Msg("personio_webhook: trigger offboarding failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "offboarding trigger failed"})
	}

	_ = h.PersonioSecrets.RecordPersonioWebhook(c.Request().Context(), orgID)

	log.Info().Str("org_id", orgID).Int("personio_id", payload.Data.EmployeeID).
		Msg("personio_webhook: offboarding checklist started")
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// verifyPersonioSignature verifies the HMAC-SHA256 signature sent by Personio.
func verifyPersonioSignature(body []byte, signature, secret string) bool {
	if signature == "" || secret == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}
