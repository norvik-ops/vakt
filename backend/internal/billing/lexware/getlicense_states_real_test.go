// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package lexware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/billing/licensing"
)

// Diese Datei beweist gegen ECHTES Postgres, was GetLicense pro Abo-Zustand
// antwortet — also genau die Frage, die eine Testrechnung im echten Mandanten
// beantworten sollte.
//
// Warum nicht die Rechnung: Für einen echten "unpaid"-Zustand müsste die Rechnung
// FINALISIERT werden (ein Entwurf erzeugt keine Subscription). Das verbraucht eine
// fortlaufende Nummer unter der eigenen Steuernummer und ist nur per Storno
// zurückzuholen — live_probe_test.go hält dazu selbst fest: "das ist eine
// Entscheidung, die ein Test nicht treffen darf". Der Zustand, auf den es ankommt,
// steht ohnehin in der Datenbank, nicht in Lexware: Die Query filtert auf
// `s.status = 'paid'`. Also wird hier der Zustand gesetzt und die Antwort gemessen.
//
// Gegen die migrierte Test-DB:
//
//	VAKT_DB_URL=postgres://vakt:vakt@127.0.0.1:15497/vakt_test?sslmode=disable \
//	  go test -run TestGetLicenseStates -v ./internal/billing/lexware/

func statesPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dbURL := os.Getenv("VAKT_DB_URL")
	if dbURL == "" {
		t.Skip("VAKT_DB_URL not set — needs a migrated Postgres (set in CI)")
	}
	pool, err := pgxpool.New(context.Background(), dbURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

// seedSubscription legt ein Abo + eine Lizenz an und gibt den Renewal-Token zurück.
// Räumt sich selbst wieder ab, damit die Test-DB zwischen Läufen sauber bleibt.
func seedSubscription(t *testing.T, pool *pgxpool.Pool, status string, cancelled, revoked bool) string {
	t.Helper()
	ctx := context.Background()

	var subID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO billing_quote_requests (company_name, email, approval_token_hash, status, cancelled_at)
		VALUES ('ZZZ Teststand', 'zzz-test@example.invalid', 'not-a-real-hash', $1,
		        CASE WHEN $2::bool THEN NOW() ELSE NULL END)
		RETURNING id::text`, status, cancelled).Scan(&subID))

	var token string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO billing_licenses (subscription_id, org_name, license_key, expires_at, renewal_token, revoked_at)
		VALUES ($1::uuid, 'ZZZ Teststand', 'dummy-key-not-signed', NOW() + INTERVAL '10 days', gen_random_uuid(),
		        CASE WHEN $2::bool THEN NOW() ELSE NULL END)
		RETURNING renewal_token::text`, subID, revoked).Scan(&token))

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM billing_licenses WHERE subscription_id = $1::uuid`, subID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM billing_quote_requests WHERE id = $1::uuid`, subID)
	})
	return token
}

func callGetLicense(t *testing.T, pool *pgxpool.Pool, token string) (int, string) {
	t.Helper()
	h := NewHandler(pool, nil, nil, licensing.SMTPConfig{}, "https://billing.test", "ops@example.invalid")
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/billing/license", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	require.NoError(t, h.GetLicense(e.NewContext(req, rec)))
	return rec.Code, rec.Body.String()
}

// TestGetLicenseStates_RefusalIsIdenticalForEveryReason ist der Kern: Der Server
// beantwortet unbezahlt, gekündigt, widerrufen und unbekannt mit DERSELBEN Antwort.
// Genau deshalb bringt eine echte unbezahlte Rechnung keine Erkenntnis, die ein
// erfundener Token nicht auch liefert — und deshalb darf die Instanz aus dem 404
// nur "Erneuerung klappt nicht" ableiten, nie einen Grund.
func TestGetLicenseStates_RefusalIsIdenticalForEveryReason(t *testing.T) {
	pool := statesPool(t)

	cases := []struct {
		name      string
		status    string
		cancelled bool
		revoked   bool
	}{
		// 'approved' ist der Zustand, den eine echte, noch nicht beglichene Rechnung
		// erzeugt: Angebot angenommen, Rechnung raus, 45-Tage-Key liegt bei —
		// settle() setzt erst beim Zahlungseingang auf 'paid'. Genau diesen Zustand
		// hätte eine Testrechnung im Mandanten hergestellt, hier ohne Beleg.
		{"offene Rechnung (approved, noch nicht bezahlt)", "approved", false, false},
		{"Anfrage noch nicht freigegeben", "requested", false, false},
		{"Anfrage abgelehnt", "rejected", false, false},
		{"gekündigt", "paid", true, false},
		{"Platz widerrufen", "paid", false, true},
	}

	var bodies []string
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			token := seedSubscription(t, pool, tc.status, tc.cancelled, tc.revoked)
			code, body := callGetLicense(t, pool, token)
			assert.Equal(t, http.StatusNotFound, code, "%s muss abgelehnt werden", tc.name)
			bodies = append(bodies, fmt.Sprintf("%d %s", code, body))
		})
	}

	// Unbekannter Token — der Fall, den autorefresh_live_test.go gegen den echten
	// Server fährt. Muss byte-identisch zu den vier oben sein.
	code, body := callGetLicense(t, pool, "00000000-0000-0000-0000-0000000000ff")
	unknown := fmt.Sprintf("%d %s", code, body)
	assert.Equal(t, http.StatusNotFound, code)

	for i, b := range bodies {
		assert.Equal(t, unknown, b,
			"Fall %q muss von einem unbekannten Token ununterscheidbar sein — sonst wird der Endpoint zum Orakel fürs Token-Raten", cases[i].name)
	}
}

// TestGetLicenseStates_MissingToken belegt die einzige Antwort, die sich vom 404
// unterscheiden DARF: gar kein Token ist ein Aufrufer-Fehler, kein Abo-Zustand.
func TestGetLicenseStates_MissingToken(t *testing.T) {
	pool := statesPool(t)
	code, _ := callGetLicense(t, pool, "")
	assert.Equal(t, http.StatusUnauthorized, code)
}

// TestGetLicenseStates_PaidIsNotRefused ist die Gegenprobe, ohne die alles oben
// vakuum wäre: Gäbe der Endpoint IMMER 404, würden die Tests ebenfalls grün.
// Ein bezahltes, nicht gekündigtes, nicht widerrufenes Abo muss also am 404-Zweig
// vorbeikommen.
func TestGetLicenseStates_PaidIsNotRefused(t *testing.T) {
	pool := statesPool(t)
	token := seedSubscription(t, pool, "paid", false, false)

	code, body := callGetLicense(t, pool, token)
	assert.NotEqual(t, http.StatusNotFound, code,
		"ein bezahltes Abo darf NICHT wie ein unbezahltes abgelehnt werden — sonst prüfen die Tests oben nichts. Body: %s", body)
}
