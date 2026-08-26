// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/auth"
)

// TestW5ARoleGapsAgainstRealChain ist die Abnahme zu R1-W5A-N1/N2 gegen den
// ECHTEN Router aus setupEcho() — nicht gegen den Nachbau in internal/rbaccov.
//
// Warum beides existiert, und warum keins das andere ersetzt:
//
//	internal/rbaccov baut sich seinen Router selbst zusammen und entscheidet
//	dabei, welche Middleware er mountet. Er belegt damit, dass die
//	Rollenprüfung IN der Register-Funktion steht — aber nicht, dass die Route
//	in der Produktion hinter derselben Kette hängt. Ein verschobener Mount in
//	cmd/api/routes.go färbt ihn nicht rot (das steht dort seit S132 als
//	bekannte Grenze im Kopf der Datei). Dieser Test hier fährt den echten
//	Baum inklusive Auth, CSRF, MFA, Modulprüfung, UUID-Guard und Rate-Limit.
//
// Geprüft wird der FEHLERCODE, nicht der Status. In dieser Kette antworten
// mehrere Schichten mit 403 — CSRF ohne Double-Submit-Paar
// (CSRF_HEADER_MISSING), die Modulprüfung (MODULE_ACCESS_DENIED), das
// Auditor-Portal. Eine Statusprüfung allein wäre deshalb auch dann grün, wenn
// die Rollenprüfung ersatzlos entfernt würde und bloß eine vorgelagerte Schicht
// ablehnt: der Test hätte bestanden, ohne das Gate je berührt zu haben. Genau
// deshalb schickt der Test die Schreibrouten MIT gültigen CSRF-Zugangsdaten und
// fordert `AUTH_INSUFFICIENT_ROLE` — diesen Code setzt ausschließlich
// auth.RequireRole.
//
// Die Nicht-Vakuität sichert die Admin-Gegenprobe unten: ein Baum, der jeden
// ablehnt, bestünde die Viewer-Zusicherung und fiele dort durch.
func TestW5ARoleGapsAgainstRealChain(t *testing.T) {
	dbURL := os.Getenv("VAKT_DB_URL")
	secret := os.Getenv("VAKT_SECRET_KEY")
	if dbURL == "" || secret == "" || os.Getenv("VAKT_REDIS_URL") == "" {
		t.Skip("needs VAKT_DB_URL + VAKT_REDIS_URL + VAKT_SECRET_KEY (CI sets all three)")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err, "connect")
	defer pool.Close()

	adminTok := seedUserWithRole(ctx, t, pool, secret, "Admin", "admin")
	e, _ := setupEcho(ctx, testConfig())

	// Wohlgeformt, damit ValidateUUIDParams die Anfrage durchlässt und sie die
	// Rollenprüfung überhaupt erreicht — ein kaputtes Segment gäbe 400 und der
	// Test prüfte eine Schicht zu früh.
	const dummyID = "00000000-0000-0000-0000-000000000000"

	routes := []struct {
		method string
		path   string
		damage string
	}{
		// R1-W5A-N1 — NIS2-Assistent.
		{http.MethodPost, "/api/v1/vaktcomply/nis2-assessment/migrate-from-anonymous",
			"org-weites UPDATE ck_controls SET manual_status"},
		{http.MethodPost, "/api/v1/vaktcomply/reassess",
			"Org-Run anlegen und die 90-Tage-Sperre verbrennen"},
		{http.MethodPost, "/api/v1/vaktcomply/reassess/" + dummyID + "/answer",
			"Antworten und Score des Org-Runs schreiben"},
		{http.MethodPost, "/api/v1/vaktcomply/nis2-assessment/multi/start",
			"Run anlegen, Pro-Entitlement der Org verbrauchen"},
		{http.MethodPost, "/api/v1/vaktcomply/nis2-assessment/multi/" + dummyID + "/answer",
			"Antworten eines Multi-Framework-Runs schreiben"},

		// R1-W5A-N2 — Benachrichtigungen.
		{http.MethodPost, "/api/v1/dashboard/notifications/read-all",
			"Ungelesen-Status für ALLE Nutzer der Org zurücksetzen"},
		{http.MethodPost, "/api/v1/dashboard/notifications/" + dummyID + "/read",
			"org-weit geteilte Benachrichtigung als gelesen markieren"},
	}

	// Alle drei Nur-Lese-Rollen, nicht nur Viewer: eine Rollenliste, die
	// versehentlich nur Viewer ausschließt, wäre bei einer Viewer-Probe grün.
	readOnly := []struct{ platformRole, usersRole string }{
		{"Viewer", "viewer"},
		{"AuditorReadOnly", "viewer"},
		{"InternalAuditor", "viewer"},
	}

	for _, r := range routes {
		r := r
		t.Run(r.method+" "+r.path, func(t *testing.T) {
			for _, ro := range readOnly {
				tok := seedUserWithRole(ctx, t, pool, secret, ro.platformRole, ro.usersRole)
				rec := doRBACReq(e, r.method, r.path, tok)

				require.Equal(t, http.StatusForbidden, rec.Code,
					"%s darf %s nicht erreichen — ohne Gate: %s. Body: %s",
					ro.platformRole, r.path, r.damage, rec.Body.String())

				var body auth.InsufficientRoleResponse
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body),
					"403-Body ist kein Rollen-Fehler: %s", rec.Body.String())
				assert.Equal(t, "AUTH_INSUFFICIENT_ROLE", body.Code,
					"%s wurde auf %s abgelehnt, aber nicht von der Rollenprüfung "+
						"(Code %q, Body %s) — eine andere Schicht war zuerst dran, das "+
						"Gate selbst bleibt damit unbelegt",
					ro.platformRole, r.path, body.Code, rec.Body.String())
			}

			// Nicht-Vakuität: ein Admin darf an der Rollenprüfung nicht scheitern.
			// Er darf durchaus 402 (Lizenz), 400 oder 404 bekommen — nur eben kein
			// AUTH_INSUFFICIENT_ROLE.
			aRec := doRBACReq(e, r.method, r.path, adminTok)
			if aRec.Code == http.StatusForbidden {
				var body auth.InsufficientRoleResponse
				_ = json.Unmarshal(aRec.Body.Bytes(), &body)
				assert.NotEqual(t, "AUTH_INSUFFICIENT_ROLE", body.Code,
					"Admin wird auf %s von der Rollenprüfung abgewiesen — die "+
						"Ablehnungen oben belegen dann nichts. Body: %s",
					r.path, aRec.Body.String())
			}
		})
	}
}
