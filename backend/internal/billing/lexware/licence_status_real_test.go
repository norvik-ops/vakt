// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package lexware

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// LicenceStatus hat fuenf Zustaende und erreichte vier.
//
// Es rief Entitlement, und Entitlement filtert gekuendigte Abos absichtlich weg
// (die Signier-Grenze DARF fuer ein gekuendigtes Abo nicht antworten). Fuer jedes
// gekuendigte Abo kam damit pgx.ErrNoRows zurueck, LicenceStatus brach VOR seinem
// eigenen `case cancelled != nil` ab, beide Konsumenten schluckten den Fehler
// ("if ... err == nil") und die Templates rendern fuer einen leeren Status eine
// leere rote Plakette. Ein Kunde, der gekuendigt hat, stand im Panel und im
// MSP-Portal mit einem LEEREN Statusfeld.
//
// Gegen die migrierte Test-DB (VAKT_DB_URL), siehe renewal_token_real_test.go.

func TestLicenceStatusReachesCancelled(t *testing.T) {
	pool := statesPool(t)
	ctx := context.Background()

	periodEnd := time.Now().Add(20 * 24 * time.Hour)
	subID, token := seedPaidLicence(t, pool, "year", periodEnd, time.Now().Add(20*24*time.Hour))

	// Gegenprobe ZUERST, damit der Test nicht gruen waere, wenn LicenceStatus
	// einfach immer "gekuendigt" saegte: solange nicht gekuendigt ist, muss
	// "bezahlt" herauskommen.
	status, limit, err := LicenceStatus(ctx, pool, token)
	require.NoError(t, err)
	require.Equal(t, StatusPaid, status, "ein bezahltes, nicht gekuendigtes Abo ist 'bezahlt'")
	require.False(t, limit.IsZero())

	_, err = pool.Exec(ctx, `UPDATE billing_quote_requests SET cancelled_at = NOW() WHERE id = $1::uuid`, subID)
	require.NoError(t, err)

	status, limit, err = LicenceStatus(ctx, pool, token)
	require.NoError(t, err,
		"LicenceStatus bricht fuer ein gekuendigtes Abo ab — Panel und MSP-Portal zeigen dann ein leeres Statusfeld")
	assert.Equal(t, StatusCancelled, status,
		"StatusCancelled ist unerreichbar: gekuendigte Abos bekommen im Panel eine leere rote Plakette statt 'gekuendigt'")
	assert.WithinDuration(t, entitlementOf(periodEnd, 30), limit, time.Minute,
		"auch ein gekuendigtes Abo muss das Datum nennen, bis zu dem das Bezahlte reicht — genau das verspricht StatusCancelled")
}

// TestLicenceStatusReachesRevokedOnACancelledSubscription ist die Nebenwirkung
// desselben frueh greifenden Fehlerzweigs: ein gesperrter Platz auf einem
// gekuendigten Abo war ebenfalls nicht darstellbar. 'gesperrt' schlaegt
// 'gekuendigt', weil es die engere Aussage ist.
func TestLicenceStatusReachesRevokedOnACancelledSubscription(t *testing.T) {
	pool := statesPool(t)
	ctx := context.Background()

	subID, token := seedPaidLicence(t, pool, "year", time.Now().Add(20*24*time.Hour), time.Now().Add(20*24*time.Hour))
	_, err := pool.Exec(ctx, `UPDATE billing_quote_requests SET cancelled_at = NOW() WHERE id = $1::uuid`, subID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE billing_licenses SET revoked_at = NOW() WHERE renewal_token = $1::uuid`, token)
	require.NoError(t, err)

	status, _, err := LicenceStatus(ctx, pool, token)
	require.NoError(t, err)
	assert.Equal(t, StatusRevoked, status)
}

// TestEntitlementStillRefusesACancelledSubscription ist der Test, ohne den der
// Fix oben gefaehrlich waere.
//
// Entitlement ist die einzige Signier-Grenze im Geldpfad: sie MUSS fuer ein
// gekuendigtes oder unbezahltes Abo weiter verweigern, sonst haette der Anzeige-Fix
// den Geld-Guard aufgeweicht — ein gekuendigter Kunde bekaeme weiter Schluessel.
// Der Anzeigepfad hat deshalb seinen eigenen Zugriff (paidThrough), und die Grenze
// bleibt, wo sie war.
func TestEntitlementStillRefusesACancelledSubscription(t *testing.T) {
	pool := statesPool(t)
	ctx := context.Background()

	periodEnd := time.Now().Add(20 * 24 * time.Hour)
	subID, _ := seedPaidLicence(t, pool, "year", periodEnd, time.Now().Add(20*24*time.Hour))

	limit, err := Entitlement(ctx, pool, subID)
	require.NoError(t, err, "Gegenprobe: solange bezahlt und nicht gekuendigt, MUSS Entitlement antworten")
	require.WithinDuration(t, entitlementOf(periodEnd, 30), limit, time.Minute)

	for _, tc := range []struct {
		name string
		sql  string
	}{
		{"gekuendigt", `UPDATE billing_quote_requests SET cancelled_at = NOW() WHERE id = $1::uuid`},
		{"nicht bezahlt", `UPDATE billing_quote_requests SET cancelled_at = NULL, status = 'approved' WHERE id = $1::uuid`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := pool.Exec(ctx, tc.sql, subID)
			require.NoError(t, err)

			_, err = Entitlement(ctx, pool, subID)
			require.Error(t, err, "%s: Entitlement darf keinen Anspruch liefern", tc.name)
			assert.ErrorIs(t, err, pgx.ErrNoRows,
				"%s: das Sentinel muss pgx.ErrNoRows bleiben — die Aufrufer unterscheiden es von ErrNothingPaid", tc.name)
		})
	}
}

// TestLicenceStatusBeforeTheFirstPayment pinnt den Zustand, den JEDER Neukunde
// durchlaeuft: Rechnung raus (status='approved', eine offene Rechnung), Zahlung
// noch nicht da, und in der Hand ein 45-Tage-Schluessel, der WIRKLICH funktioniert.
//
// Er ist 'laeuft aus' und nicht 'abgelaufen'. Genau diesen Zustand beschreibt
// StatusExpiring in seinem eigenen Doc-Kommentar ("an invoice is out and unpaid.
// The key still works … the last moment to pick up the phone"), und er war
// unerreichbar: paidThrough liefert vor der ersten Zahlung ErrNothingPaid und
// limit == zero, also griff `limit.IsZero()` vorher und LicenceStatus sagte
// 'abgelaufen' — direkt neben der Spalte "Gueltig bis" mit einem Datum 45 Tage in
// der Zukunft. Falsch und plausibel ist schlimmer als sichtbar leer.
//
// Die Abgrenzung liegt am SCHLUESSEL, nicht an der Zeit: laeuft der 45-Tage-
// Schluessel ohne Zahlungseingang aus, ist 'abgelaufen' dann richtig.
func TestLicenceStatusBeforeTheFirstPayment(t *testing.T) {
	pool := statesPool(t)
	ctx := context.Background()

	subID, token := seedApprovedUnpaid(t, pool, time.Now().Add(45*24*time.Hour))

	status, limit, err := LicenceStatus(ctx, pool, token)
	require.NoError(t, err, "ErrNothingPaid ist kein Abbruchgrund fuer die Anzeige")
	assert.Equal(t, StatusExpiring, status,
		"Rechnung raus, nicht bezahlt, Schluessel laeuft noch 45 Tage — das ist 'laeuft aus'; 'abgelaufen' behauptet neben einem gueltigen Datum das Gegenteil")
	assert.True(t, limit.IsZero(),
		"bezahlt ist nichts, also gibt es kein Datum, bis zu dem das Bezahlte reicht — hier eine Zahl zu erfinden waere dieselbe Sorte Fehler")

	// Gegenprobe 1 — die Aussage haengt am Schluessel: laeuft er aus, ohne dass
	// gezahlt wurde, ist 'abgelaufen' richtig. Ohne diese Probe waere der Zweig
	// von "sagt immer 'laeuft aus'" nicht zu unterscheiden.
	_, err = pool.Exec(ctx,
		`UPDATE billing_licenses SET expires_at = NOW() - INTERVAL '1 day' WHERE renewal_token = $1::uuid`, token)
	require.NoError(t, err)

	status, _, err = LicenceStatus(ctx, pool, token)
	require.NoError(t, err)
	assert.Equal(t, StatusLapsed, status,
		"unbezahlt UND Schluessel abgelaufen: hier ist 'abgelaufen' die Wahrheit")

	// Gegenprobe 2 — die Aussage haengt an der offenen Rechnung: ohne Rechnung gibt
	// es keinen Anlass, jemanden anzurufen, und 'laeuft aus' waere eine Erfindung.
	_, err = pool.Exec(ctx,
		`UPDATE billing_licenses SET expires_at = NOW() + INTERVAL '45 days' WHERE renewal_token = $1::uuid`, token)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `DELETE FROM billing_invoices WHERE subscription_id = $1::uuid`, subID)
	require.NoError(t, err)

	status, _, err = LicenceStatus(ctx, pool, token)
	require.NoError(t, err)
	assert.Equal(t, StatusLapsed, status,
		"kein bezahlter Zeitraum und keine offene Rechnung: nichts laeuft aus, es ist nichts da")
}

// seedApprovedUnpaid legt die Lage nach dem Rechnungsversand nach, genau wie
// ApproveRequest sie schreibt: Abo auf 'approved', eine OFFENE Rechnung, und eine
// Lizenzzeile kind='trial' mit license.TrialExpiry() als Ablauf.
func seedApprovedUnpaid(t *testing.T, pool *pgxpool.Pool, keyExpires time.Time) (subID, token string) {
	t.Helper()
	ctx := context.Background()

	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO billing_quote_requests (company_name, email, approval_token_hash, status, product, interval, approved_at)
		VALUES ('ZZZ Approved Unpaid', 'zzz-approved@example.invalid', 'not-a-real-hash', 'approved', 'pro', 'year', NOW())
		RETURNING id::text`).Scan(&subID))

	t.Cleanup(func() {
		c := context.Background()
		_, _ = pool.Exec(c, `DELETE FROM billing_invoices WHERE subscription_id = $1::uuid`, subID)
		_, _ = pool.Exec(c, `DELETE FROM billing_licenses WHERE subscription_id = $1::uuid`, subID)
		_, _ = pool.Exec(c, `DELETE FROM billing_quote_requests WHERE id = $1::uuid`, subID)
	})

	_, err := pool.Exec(ctx, `
		INSERT INTO billing_invoices (subscription_id, lexware_invoice_id, status,
		                              net_amount_cents, gross_amount_cents, tax_amount_cents,
		                              period_start, period_end)
		VALUES ($1::uuid, 'ZZZ-'||gen_random_uuid()::text, 'open', 29900, 29900, 0,
		        NOW()::date, (NOW() + INTERVAL '365 days')::date)`, subID)
	require.NoError(t, err)

	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO billing_licenses (subscription_id, org_name, license_key, expires_at, kind)
		VALUES ($1::uuid, 'ZZZ Approved Unpaid', 'placeholder', $2, 'trial')
		RETURNING renewal_token::text`, subID, keyExpires).Scan(&token))
	return subID, token
}

// TestPaidThroughReportsNothingPaid haelt die andere Grenze fest: hat der Kunde
// noch keine Periode bezahlt, ist das ErrNothingPaid und NICHT ErrNoRows —
// LicenceStatus laesst genau dieses Sentinel durch, statt daran abzubrechen.
// Ohne offene Rechnung ist die Anzeige dann 'abgelaufen' (siehe
// TestLicenceStatusBeforeTheFirstPayment fuer den Fall MIT offener Rechnung).
func TestPaidThroughReportsNothingPaid(t *testing.T) {
	pool := statesPool(t)
	ctx := context.Background()

	var subID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO billing_quote_requests (company_name, email, approval_token_hash, status, product, interval)
		VALUES ('ZZZ Nothing Paid', 'zzz-nothing@example.invalid', 'not-a-real-hash', 'paid', 'pro', 'year')
		RETURNING id::text`).Scan(&subID))

	var token string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO billing_licenses (subscription_id, org_name, license_key, expires_at, kind)
		VALUES ($1::uuid, 'ZZZ Nothing Paid', 'placeholder', NOW() + INTERVAL '10 days', 'full')
		RETURNING renewal_token::text`, subID).Scan(&token))

	t.Cleanup(func() {
		c := context.Background()
		_, _ = pool.Exec(c, `DELETE FROM billing_licenses WHERE subscription_id = $1::uuid`, subID)
		_, _ = pool.Exec(c, `DELETE FROM billing_quote_requests WHERE id = $1::uuid`, subID)
	})

	_, _, err := paidThrough(ctx, pool, subID)
	assert.ErrorIs(t, err, ErrNothingPaid)

	status, _, err := LicenceStatus(ctx, pool, token)
	require.NoError(t, err, "ErrNothingPaid ist kein Abbruchgrund fuer die Anzeige")
	assert.Equal(t, StatusLapsed, status)
}
