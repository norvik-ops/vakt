// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package license

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// captureLog leitet den globalen zerolog-Logger für die Dauer des Tests in
// einen Puffer um.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	old := log.Logger
	log.Logger = zerolog.New(&buf)
	t.Cleanup(func() { log.Logger = old })
	return &buf
}

// signRaw signiert beliebige Payload-Bytes, auch solche, die kein JSON sind.
func signRaw(t *testing.T, priv *ecdsa.PrivateKey, raw []byte) string {
	t.Helper()
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256([]byte(encoded))
	r, s, err := ecdsa.Sign(rand.Reader, priv, hash[:])
	if err != nil {
		t.Fatal(err)
	}
	sig := make([]byte, 64)
	rb, sb := r.Bytes(), s.Bytes()
	copy(sig[32-len(rb):32], rb)
	copy(sig[64-len(sb):], sb)
	return encoded + "." + base64.RawURLEncoding.EncodeToString(sig)
}

// Ein abgelehnter oder abgelaufener Schlüssel muss mit Grund im Log stehen,
// der Schlüssel selbst (Payload, Signatur, Renewal-Token) nie.
func TestLoad_RejectionIsLoggedWithoutKey(t *testing.T) {
	priv, restore := setupTestKeys(t)
	defer restore()

	const secretToken = "renewal-secret-do-not-log"
	past := int64(1600000000) // 2020
	future := int64(1830211200)

	cases := []struct {
		name       string
		key        string
		wantReason string
		wantExtra  string
	}{
		{
			name:       "Signatur ungültig",
			key:        "eyJ0aWVyIjoicHJvIn0.AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
			wantReason: "license signature verification failed",
		},
		{
			name:       "Format ungültig",
			key:        "notakey",
			wantReason: "invalid license key format",
		},
		{
			name:       "Base64 kaputt",
			key:        "%%%%.%%%%",
			wantReason: "license key could not be decoded",
		},
		{
			name:       "Payload kein JSON",
			key:        signRaw(t, priv, []byte("not-json-"+secretToken)),
			wantReason: "license key could not be decoded",
		},
		{
			name: "unbekannter Tier",
			key: makeTestKey(t, priv, payload{
				Tier: "enterprise", Org: "Acme", IssuedAt: 1778923388, Exp: &future, RenewalToken: secretToken,
			}),
			wantReason: "license tier is not sold by this build",
			wantExtra:  `"tier":"enterprise"`,
		},
		{
			name: "abgelaufen",
			key: makeTestKey(t, priv, payload{
				Tier: "pro", Org: "Acme", IssuedAt: 1500000000, Exp: &past, RenewalToken: secretToken,
			}),
			wantReason: "license has expired",
			wantExtra:  `"expires_at":"2020-09-13T12:26:40Z"`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf := captureLog(t)
			Load(tc.key, false)
			out := buf.String()

			if !strings.Contains(out, `"level":"warn"`) {
				t.Fatalf("keine Warnzeile geschrieben: %q", out)
			}
			if !strings.Contains(out, `"reason":"`+tc.wantReason+`"`) {
				t.Fatalf("Grund %q fehlt im Log: %s", tc.wantReason, out)
			}
			if tc.wantExtra != "" && !strings.Contains(out, tc.wantExtra) {
				t.Fatalf("%s fehlt im Log: %s", tc.wantExtra, out)
			}
			if strings.Contains(out, secretToken) {
				t.Fatalf("Renewal-Token steht im Log: %s", out)
			}
			for _, part := range strings.SplitN(tc.key, ".", 2) {
				if len(part) >= 8 && strings.Contains(out, part) {
					t.Fatalf("Teil des Schlüssels steht im Log: %s", out)
				}
			}
		})
	}
}

// Gegenprobe: Ein gültiger Schlüssel und ein leerer Schlüssel schreiben keine
// Warnung — sonst wäre die Warnung Rauschen.
func TestLoad_ValidOrEmptyKeyLogsNoWarning(t *testing.T) {
	priv, restore := setupTestKeys(t)
	defer restore()
	future := int64(1830211200)
	key := makeTestKey(t, priv, payload{Tier: "pro", Org: "Acme", IssuedAt: 1778923388, Exp: &future})

	for name, k := range map[string]string{"gültig": key, "leer": ""} {
		buf := captureLog(t)
		Load(k, false)
		if strings.Contains(buf.String(), `"level":"warn"`) {
			t.Fatalf("%s: unerwartete Warnung: %s", name, buf.String())
		}
	}
}

// assertReasonOnly prüft, dass die Warnzeile den festen Grund trägt und weder
// den rohen Fehler (Feld "error") noch Teile des Schlüssels.
func assertReasonOnly(t *testing.T, out, wantReason, key string) {
	t.Helper()
	if !strings.Contains(out, `"reason":"`+wantReason+`"`) {
		t.Fatalf("Grund %q fehlt im Log: %s", wantReason, out)
	}
	if strings.Contains(out, `"error":`) {
		t.Fatalf("roher parse()-Fehler steht im Log: %s", out)
	}
	for _, part := range strings.SplitN(key, ".", 2) {
		if len(part) >= 8 && strings.Contains(out, part) {
			t.Fatalf("Teil des Schlüssels steht im Log: %s", out)
		}
	}
}

// Aktivierung und Auto-Refresh loggen einen abgelehnten Schlüssel wie Load():
// nur mit festem Grund, nie mit dem rohen Fehler aus parse().
func TestActivate_RejectionLogsReasonOnly(t *testing.T) {
	priv, restore := setupTestKeys(t)
	defer restore()
	key := signRaw(t, priv, []byte("not-json-renewal-secret-do-not-log"))

	buf := captureLog(t)
	e := echo.New()
	body, err := json.Marshal(map[string]string{"key": key})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/license/activate", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	if err := NewHandler(communityLicense()).Activate(e.NewContext(req, rec)); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("Status %d, erwartet 422", rec.Code)
	}
	assertReasonOnly(t, buf.String(), "license key could not be decoded", key)
}

func TestAutoRefresh_InvalidKeyLogsReasonOnly(t *testing.T) {
	priv, restore := setupTestKeys(t)
	defer restore()
	bad := signRaw(t, priv, []byte("not-json-renewal-secret-do-not-log"))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"key": bad})
	}))
	defer srv.Close()

	r, h := refresherFor(t, keyExpiringIn(t, priv, 10*24*time.Hour, "tok"), srv.URL)
	buf := captureLog(t)
	r.check(context.Background())
	if !h.RenewalFailing() {
		t.Fatal("ungültiger Schlüssel vom Server muss als fehlgeschlagene Erneuerung gelten")
	}
	assertReasonOnly(t, buf.String(), "license key could not be decoded", bad)
}

// Ein kaputter Public Key im Build ist kein kaputter Kundenschlüssel: Der Grund
// muss das sagen, statt "could not be decoded".
func TestLoad_BrokenBuildKeyHasOwnReason(t *testing.T) {
	old := publicKeyPEM
	publicKeyPEM = "-----BEGIN PUBLIC KEY-----\nAAAA\n-----END PUBLIC KEY-----\n"
	defer func() { publicKeyPEM = old }()

	buf := captureLog(t)
	key := "eyJ0aWVyIjoicHJvIn0.AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	if lic := Load(key, false); lic.Tier != "community" {
		t.Fatalf("Tier %q, erwartet community", lic.Tier)
	}
	assertReasonOnly(t, buf.String(), "license public key of this build is invalid", key)
}
