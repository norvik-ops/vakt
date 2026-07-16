// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package license

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// getLicenseJSON drives Handler.Get through a real Echo context and decodes the
// wire response, so the assertions see the actual JSON field names rather than
// the Go struct.
func getLicenseJSON(t *testing.T, h *Handler) map[string]any {
	t.Helper()
	e := echo.New()
	rec := httptest.NewRecorder()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/license", nil), rec)
	require.NoError(t, h.Get(c))
	require.Equal(t, http.StatusOK, rec.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body
}

// keyExpiringIn signs a key whose expiry sits `d` from now, issued 90 days before
// that expiry. The 90-day lifetime is what production issues, so renewWindow()
// (lifetime/4, capped at 30d) yields the real ~22-day window rather than a number
// invented for the test.
func keyExpiringIn(t *testing.T, priv *ecdsa.PrivateKey, d time.Duration, token string) string {
	t.Helper()
	exp := time.Now().Add(d)
	iat := exp.Add(-90 * 24 * time.Hour)
	e := exp.Unix()
	return makeTestKey(t, priv, payload{
		Tier:         "pro",
		Features:     []string{"audit_pdf"},
		Org:          "ACME",
		IssuedAt:     iat.Unix(),
		Exp:          &e,
		RenewalToken: token,
	})
}

func refresherFor(t *testing.T, key, serverURL string) (*AutoRefresher, *Handler) {
	t.Helper()
	lic := Load(key, false)
	require.NotNil(t, lic)
	h := NewHandler(lic)
	return NewAutoRefresher("", serverURL, true, h, nil, nil), h
}

// TestRenewalFailing_NotDue_StaysFalse: a key nowhere near expiry is not renewed
// and therefore nothing is failing. The flag must never nag a healthy instance.
func TestRenewalFailing_NotDue_StaysFalse(t *testing.T) {
	priv, restore := setupTestKeys(t)
	defer restore()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("instance must not call out while the key is far from expiry")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// 80 days left on a 90-day key → far outside the ~22-day window.
	r, h := refresherFor(t, keyExpiringIn(t, priv, 80*24*time.Hour, "tok"), srv.URL)
	r.check(context.Background())

	assert.False(t, h.RenewalFailing(), "not due → nothing can be failing")
}

// TestRenewalFailing_ServerRefuses: the invoice is open, the server hands back no
// newer key. This is the case ADR-0052 accepts (the key lapses) — but the admin
// has to be told, which is exactly what the flag is for.
func TestRenewalFailing_ServerRefuses(t *testing.T) {
	priv, restore := setupTestKeys(t)
	defer restore()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
	}))
	defer srv.Close()

	// 10 days left → inside the ~22-day window → the refresher calls and is refused.
	r, h := refresherFor(t, keyExpiringIn(t, priv, 10*24*time.Hour, "tok"), srv.URL)
	r.check(context.Background())

	assert.True(t, h.RenewalFailing(), "server refused → renewal is failing")
}

// TestRenewalFailing_NoRenewalToken: an old key without an embedded token cannot
// renew itself at all. Silence here is the worst case — the admin must paste a key
// by hand and has no way to know it.
func TestRenewalFailing_NoRenewalToken(t *testing.T) {
	priv, restore := setupTestKeys(t)
	defer restore()

	r, h := refresherFor(t, keyExpiringIn(t, priv, 10*24*time.Hour, ""), "http://127.0.0.1:1")
	r.check(context.Background())

	assert.True(t, h.RenewalFailing(), "expiring with no renewal token → failing")
}

// TestRenewalFailing_SuccessClearsFlag: a granted renewal returns a later expiry;
// the flag must drop back to false so the warning disappears on its own.
func TestRenewalFailing_SuccessClearsFlag(t *testing.T) {
	priv, restore := setupTestKeys(t)
	defer restore()

	fresh := keyExpiringIn(t, priv, 89*24*time.Hour, "tok")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"key":"` + fresh + `"}`))
	}))
	defer srv.Close()

	r, h := refresherFor(t, keyExpiringIn(t, priv, 10*24*time.Hour, "tok"), srv.URL)

	// Start from a failed attempt so the test proves the flag CLEARS, not merely
	// that it was never set.
	h.setRenewalFailing(true)
	r.check(context.Background())

	assert.False(t, h.RenewalFailing(), "a granted renewal must clear the flag")
}

// TestLicenseResponse_ExposesRenewalFailing pins the field into the /license
// contract: the banner is driven by it, so a silent rename would put the UI back
// to saying nothing while the licence lapses.
func TestLicenseResponse_ExposesRenewalFailing(t *testing.T) {
	priv, restore := setupTestKeys(t)
	defer restore()

	h := NewHandler(Load(keyExpiringIn(t, priv, 10*24*time.Hour, "tok"), false)).WithAutoRenewal()
	h.setRenewalFailing(true)

	body := getLicenseJSON(t, h)
	assert.Equal(t, true, body["renewal_failing"], "renewal_failing must reach the client")
	assert.Equal(t, true, body["auto_renewal_enabled"])
}
