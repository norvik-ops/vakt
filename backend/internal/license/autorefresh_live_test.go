// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

//go:build billinglive

// Diese Datei spricht mit dem ECHTEN Billing-Server (api.norvikops.de).
//
// Anders als `lexwarelive` legt sie NICHTS an und kostet nichts: Sie schickt einen
// GET mit einem erfundenen Renewal-Token. Das UPDATE hinter dem Endpoint matcht auf
// `renewal_token = $1` und trifft damit keine Zeile — kein Schreibzugriff, kein
// Buchungsbeleg, kein Kontakt. Trotzdem laeuft sie hinter einem Build-Tag, den weder
// CI noch `go test ./...` je setzt: ein Test, der das Internet braucht, darf keinen
// gruenen Lauf rot faerben, wenn nur die Leitung klemmt.
//
//	go test -tags=billinglive -run TestLive -v ./internal/license/
//
// Warum ein erfundener Token denselben Fall testet wie eine offene Rechnung:
// GetLicense in internal/billing/lexware/handler.go beantwortet "revoked, cancelled,
// unpaid, unknown token" bewusst ALLE mit demselben 404 — jede Unterscheidung waere
// ein Orakel zum Token-Raten. Die Instanz sieht also in allen vier Faellen exakt
// dieselben Bytes und nimmt exakt denselben Pfad. Eine echte Nullrechnung im
// Mandanten wuerde denselben 404 erzeugen, aber einen echten Beleg hinterlassen.
package license

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const liveBillingURL = "https://api.norvikops.de"

// TestLive_RenewalRefusedByRealServer ist die Abnahme, die der Unit-Test nicht
// leisten kann: Der httptest-Server dort antwortet mit dem 402, den ICH mir
// ausgedacht habe. Hier antwortet der echte Server mit dem, was er wirklich
// schickt — und die Instanz muss daraus `renewal_failing` machen.
//
// Faellt die Antwortform des Servers je um (etwa 200 mit leerem Key statt 404),
// wird dieser Test rot, waehrend der Unit-Test weiter gruen bliebe.
func TestLive_RenewalRefusedByRealServer(t *testing.T) {
	if os.Getenv("VAKT_LIVE_BILLING") == "" {
		t.Skip("VAKT_LIVE_BILLING not set — this test talks to api.norvikops.de")
	}
	priv, restore := setupTestKeys(t)
	defer restore()

	// 10 Tage Rest auf einem 90-Tage-Key -> innerhalb des Renew-Fensters
	// (Lebensdauer/4 ~ 22 Tage), die Erneuerung ist also faellig.
	exp := time.Now().Add(10 * 24 * time.Hour)
	e := exp.Unix()
	key := makeTestKey(t, priv, payload{
		Tier:         "pro",
		Features:     []string{"audit_pdf"},
		Org:          "Live Probe",
		IssuedAt:     exp.Add(-90 * 24 * time.Hour).Unix(),
		Exp:          &e,
		RenewalToken: "00000000-0000-0000-0000-0000000000ff", // existiert nicht -> 404
	})

	lic := Load(key, false)
	require.NotNil(t, lic)
	require.True(t, lic.IsPro(), "der Testschluessel muss als Pro geladen werden")

	h := NewHandler(lic).WithAutoRenewal()
	r := NewAutoRefresher("", liveBillingURL, true, h, nil, nil)

	require.True(t, r.due(), "10 Tage Rest muessen im Renew-Fenster liegen")
	// Ohne diese Zusicherung waere der Test vakuum: check() setzt das Flag AUCH,
	// wenn gar kein Renewal-Token im Key steckt — dann haette es nie den Server
	// gefragt und der Test waere trotzdem gruen. Nur mit Token laeuft er bis
	// fetchKey durch, und genau dessen Antwort will diese Datei pruefen.
	require.NotEmpty(t, r.currentToken(), "der Key muss einen Renewal-Token tragen, sonst prueft der Test das Netz gar nicht")

	// Zuerst die Antwort selbst pruefen, nicht nur ihre Folge. Sonst waere der Test
	// nicht zu unterscheiden von "Leitung tot": ein Netzfehler ist ebenfalls ein
	// Fehlschlag und wuerde renewal_failing genauso setzen. Nur der Statuscode im
	// Fehlertext belegt, dass der echte Server geantwortet und abgelehnt hat.
	_, fetchErr := r.fetchKey(context.Background())
	require.Error(t, fetchErr, "ein unbekannter Token darf keinen Key liefern")
	require.Contains(t, fetchErr.Error(), "404",
		"der echte Server muss mit 404 ablehnen (revoked/cancelled/unpaid/unbekannt sind bewusst ununterscheidbar) — ein Netzfehler haette einen anderen Text")

	r.check(context.Background())

	assert.True(t, h.RenewalFailing(),
		"der echte Server lehnt einen unbekannten Token ab — die Instanz muss das als fehlschlagende Erneuerung fuehren")

	// Der Schluessel selbst bleibt gueltig: eine abgelehnte Erneuerung darf einen
	// zahlenden Kunden nie sofort abschalten (ADR-0052).
	assert.False(t, h.RenewalFailing() && lic.Expired, "ein abgelehnter Renewal darf den laufenden Key nicht entwerten")
	assert.True(t, lic.IsPro(), "der laufende Key bleibt bis zum Ablauf Pro")
}
