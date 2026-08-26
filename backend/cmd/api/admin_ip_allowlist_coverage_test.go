// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Dieses Gate prueft die ABDECKUNG der env-basierten Admin-IP-Allowlist
// (VAKT_ADMIN_ALLOWED_IPS), nicht ihr Verhalten.
//
// Der Defekt, den es festhaelt (Codeaudit v5b, R1-10-V01 / ESK-8), war live
// gemessen: mit gesetzter env-Allowlist und einer nicht gelisteten Client-IP
// blockten 48 von 59 /admin-Routen. Die 11 durchgelassenen waren ausgerechnet
// die schwersten — Rollenwechsel, MFA-Break-Glass, Passwort-Reset-Token.
// Ursache war nicht ein kaputter Guard, sondern VIER Mount-Punkte statt einem:
// wer eine neue /admin-Untergruppe anlegt, erbt den Guard nicht. Die parallele
// Org-Allowlist (OrgIPAllowlist) zeigt die Loesung — EIN Mount auf `protected`
// mit Pfad-Bedingung deckte schon damals 59 von 59.
//
// Drei Entwurfsentscheidungen, alle gegen genau diese Wiederholung:
//
//  1. Der Baum kommt aus setupEcho(), also aus der ECHTEN Registrierung in
//     routes.go. Ein nachgebauter echo.New() traegt den Guard gar nicht — er
//     sitzt am Mount-Punkt, nicht im Paket-Register. Ein solcher Test wuerde
//     seinen eigenen Nachbau pruefen.
//
//  2. KEIN Routen-Literal, keine gepflegte Liste. Eine Liste driftet — das IST
//     die Ursache dieses Defekts, nur eine Ebene hoeher. Der Kandidatensatz
//     ergibt sich rein syntaktisch aus dem Pfad (ein Segment heisst "admin"),
//     und dieses Kriterium ist bewusst NICHT die Produktions-Praedikatsfunktion
//     IsAdminPath: fragte der Test sie ab, wuerde eine Verengung von IsAdminPath
//     Routen aus dem Nenner werfen und das Gate bliebe gruen, waehrend der Guard
//     schrumpft.
//
//  3. Der Nenner wird gezaehlt UND eingefordert. Ein leerer Baum (setupEcho
//     liefert ohne DB nur /health) wuerde sonst als "sauber" durchgehen —
//     Stille ist kein Beleg.

// adminAllowlistTestCIDR ist das Sperrnetz fuer den Lauf. httptest.NewRequest
// setzt RemoteAddr auf 192.0.2.1:1234 (TEST-NET-1, RFC 5737) — bewusst
// ausserhalb dieses Netzes, damit jeder Request als "nicht gelistet" gilt.
const adminAllowlistTestCIDR = "10.99.99.0/24"

// adminAllowlistParamSeg ersetzt Pfad-Params. Der Wert ist eine gueltige UUID:
// Der Test soll die Allowlist messen, nicht ValidateUUIDParams. Mit einem
// kaputten Segment wuerde ein 400 der UUID-Pruefung als "Allowlist-Luecke"
// gemeldet, sobald jemand die Reihenfolge der beiden Guards aendert — ein
// Fehlalarm auf eine voellig zulaessige Konfiguration.
var adminAllowlistParamSeg = regexp.MustCompile(`:[a-zA-Z_]+`)

const adminAllowlistParamValue = "00000000-0000-0000-0000-000000000000"

// isAdminRoutePath ist die testeigene, absichtlich naive Definition von
// "das ist eine Admin-Route": irgendein Pfadsegment heisst exakt "admin".
// Bewusst unabhaengig von sharedmw.IsAdminPath (siehe Punkt 2 oben).
func isAdminRoutePath(p string) bool {
	for _, seg := range strings.Split(p, "/") {
		if seg == "admin" {
			return true
		}
	}
	return false
}

func TestEnvIPAllowlistCoversEveryAdminRoute(t *testing.T) {
	dbURL := os.Getenv("VAKT_DB_URL")
	secret := os.Getenv("VAKT_SECRET_KEY")
	if dbURL == "" || secret == "" || os.Getenv("VAKT_REDIS_URL") == "" {
		t.Skip("needs VAKT_DB_URL + VAKT_REDIS_URL + VAKT_SECRET_KEY (CI sets all three)")
	}

	// IPAllowlist() liest os.Getenv EINMAL beim Bau (ip_allowlist.go). Die
	// Variable muss deshalb vor setupEcho() stehen — nach dem Bau gesetzt
	// waere sie wirkungslos und das Gate vakuos gruen.
	t.Setenv("VAKT_ADMIN_ALLOWED_IPS", adminAllowlistTestCIDR)

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err, "connect")
	defer pool.Close()

	// Ein echter Admin-Token. Ohne ihn wuerde AuthMiddleware jeden Request mit
	// 401 beantworten und das Gate saehe nie, ob der IP-Guard haengt — es
	// waere gruen, egal wie kaputt die Montage ist.
	token := seedOrgAndToken(ctx, t, pool, secret)
	e, _ := setupEcho(ctx, testConfig())

	var checked, skipped int
	var skippedPaths []string
	seen := make(map[string]bool)

	for _, r := range e.Routes() {
		if !isAdminRoutePath(r.Path) {
			continue
		}

		// Echo registriert fuer jede Gruppe mit Middleware automatisch
		// RouteNotFound-Eintraege ("echo_route_not_found"). Das ist keine
		// Pseudo-Route zum Ueberspringen, sondern echte Angriffsflaeche: ein
		// unbekannter Pfad unter /admin laeuft durch dieselbe Middleware-Kette.
		// Wir proben ihn als GET statt ihn wegzuwerfen — deshalb skipped=0.
		method := r.Method
		if method == echo.RouteNotFound {
			method = http.MethodGet
		}

		reqPath := adminAllowlistParamSeg.ReplaceAllString(r.Path, adminAllowlistParamValue)
		reqPath = strings.ReplaceAll(reqPath, "*", "unknown-subpath")

		key := method + " " + reqPath
		if seen[key] {
			continue // dieselbe Probe zweimal sagt nichts Neues
		}
		seen[key] = true

		t.Run(r.Method+" "+r.Path, func(t *testing.T) {
			req := httptest.NewRequest(method, reqPath, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			body := strings.TrimSpace(rec.Body.String())
			assert.Equal(t, http.StatusForbidden, rec.Code,
				"%s %s laeuft an der env-Allowlist VORBEI: VAKT_ADMIN_ALLOWED_IPS=%s ist gesetzt, "+
					"die Client-IP liegt nicht darin — trotzdem kein 403. Die Route haengt nicht hinter "+
					"sharedmw.AdminIPAllowlist(). Ein neuer /admin-Mount erbt den Guard nur, wenn er unter "+
					"`protected` haengt (cmd/api/routes.go). Body: %s", r.Method, r.Path, adminAllowlistTestCIDR, body)
			assert.Contains(t, body, "IP_NOT_ALLOWED",
				"%s %s wird zwar mit 403 abgelehnt, aber nicht von der IP-Allowlist — ein anderer Guard "+
					"(Rolle, Feature, MFA) kommt ihr zuvor. Die IP-Pruefung muss die erste sein, sonst "+
					"verrraet die Antwort einer gesperrten IP, welche Routen es gibt. Body: %s", r.Method, r.Path, body)
		})
		checked++
	}

	t.Logf("checked=%d admin routes, skipped=%d %v", checked, skipped, skippedPaths)

	// Ohne diese Zusicherung waere der Test die naechste Ausgabe desselben
	// Fehlers: Liefert setupEcho() je einen leeren Baum (fehlende DB-Env) oder
	// aendert sich das Pfadschema, liefe die Schleife durch, faende null Routen
	// und meldete gruen. Der Live-Nenner des Audits war 59.
	require.Greater(t, checked, 50,
		"das Gate hat fast keine /admin-Routen gesehen — das ist ein Defekt AM GATE, kein sauberes Ergebnis")
	require.Zero(t, skipped, "keine /admin-Route darf ungeprueft bleiben: %v", skippedPaths)

	// Gegenprobe. Der Guard haengt am gesamten `protected`-Baum; das ist nur
	// zulaessig, weil er Nicht-Admin-Pfade durchlaesst. Ohne diese Zeile waere
	// eine Verbreiterung des Scopes hier unsichtbar — das Gate fragt ja nur nach
	// /admin — und jeder Betreiber mit gesetzter VAKT_ADMIN_ALLOWED_IPS bekaeme
	// auf JEDEM API-Aufruf 403. Dieselbe Klasse wie der "*"-Fall in der
	// UUID-Denylist, der jeden 404 zu einem 400 machte.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.NotContains(t, rec.Body.String(), "IP_NOT_ALLOWED",
		"GET /auth/me ist keine Admin-Route und darf von der Admin-IP-Allowlist nicht angefasst werden "+
			"(Status %d) — der Guard ist zu breit montiert", rec.Code)
}
