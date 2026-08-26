// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package rbaccov_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/auth"
)

// R1-W5A-N1 / N2: die sieben Schreibrouten, die bis zu diesem Commit keine
// Rollenprüfung trugen. Sie stehen hier NAMENTLICH, nicht als Ergebnis einer
// Aufzählung — ein Sweep über echo.Routes() (TestSharedSurfaceDenyByDefault)
// deckt dieselbe Fläche ab, sagt aber im Fehlerfall nicht, welcher der
// ursprünglichen Befunde zurückgedreht wurde.
//
// Jeder Eintrag nennt, was ein Viewer ohne das Gate anrichten konnte.
var w5aUngatedWriteRoutes = []struct {
	method string
	path   string
	damage string
}{
	// N1 — NIS2-Assistent (internal/shared/nis2wizard).
	{http.MethodPost, "/vaktcomply/nis2-assessment/migrate-from-anonymous",
		"AutoMapToControls überschreibt manual_status ALLER NIS2-Controls der Org"},
	{http.MethodPost, "/vaktcomply/reassess",
		"legt einen Org-Assessment-Run an und verbrennt die 90-Tage-Sperre"},
	{http.MethodPost, "/vaktcomply/reassess/probe/answer",
		"schreibt Antworten und Score des Org-Runs"},
	{http.MethodPost, "/vaktcomply/nis2-assessment/multi/start",
		"legt einen Run an und verbraucht das Pro-Entitlement der Org"},
	{http.MethodPost, "/vaktcomply/nis2-assessment/multi/probe/answer",
		"schreibt Antworten eines Multi-Framework-Runs"},

	// N2 — Benachrichtigungen (internal/shared/dashboard).
	{http.MethodPost, "/dashboard/notifications/read-all",
		"setzt den Ungelesen-Status für ALLE Nutzer der Org zurück"},
	{http.MethodPost, "/dashboard/notifications/probe/read",
		"markiert eine org-weit geteilte Benachrichtigung als gelesen"},
}

// TestW5AUngatedWritesRejectReadOnlyRoles ist die Regressionsprobe zu
// R1-W5A-N1/N2.
//
// Geprüft wird der FEHLERCODE, nicht nur der Status. Das ist der Punkt, an dem
// diese Probe sich von einer naiven 403-Prüfung unterscheidet: unter der echten
// Kette antworten CSRF (CSRF_HEADER_MISSING), die Modulprüfung und das
// Auditor-Portal ebenfalls mit 403. Eine Probe, die nur den Status liest, wäre
// also auch dann grün, wenn die Rollenprüfung ersatzlos entfernt würde und
// lediglich eine vorgelagerte Schicht ablehnt — sie würde das Falsche messen
// und es „bestanden" nennen. AUTH_INSUFFICIENT_ROLE kann nur aus
// auth.RequireRole kommen.
//
// Probiert werden alle drei Nur-Lese-Rollen, nicht nur Viewer: AuditorReadOnly
// und InternalAuditor sind eigenständige Rollen (auth/middleware.go), und eine
// Rollenliste, die versehentlich nur Viewer ausschließt, wäre bei einer
// Viewer-Probe grün — die K2-Lektion aus MA-01/02/03.
func TestW5AUngatedWritesRejectReadOnlyRoles(t *testing.T) {
	key, err := auth.GenerateSymmetricKey(testHexKey)
	require.NoError(t, err)

	e := buildSharedRouter(t)

	readOnlyRoles := []string{"Viewer", "AuditorReadOnly", "InternalAuditor"}

	for _, rt := range w5aUngatedWriteRoutes {
		rt := rt
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			for _, role := range readOnlyRoles {
				rec := w5aRequest(t, e, rt.method, rt.path, issueTok(t, key, role))

				require.Equal(t, http.StatusForbidden, rec.Code,
					"%s darf %s %s nicht erreichen — ohne Gate: %s. Body: %s",
					role, rt.method, rt.path, rt.damage, rec.Body.String())

				var body auth.InsufficientRoleResponse
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body),
					"403-Body von %s %s ist kein Rollen-Fehler: %s",
					rt.method, rt.path, rec.Body.String())
				assert.Equal(t, "AUTH_INSUFFICIENT_ROLE", body.Code,
					"%s wurde auf %s %s abgelehnt, aber NICHT von der Rollenprüfung "+
						"(Code %q). Eine andere Schicht hat hier 403 gesagt — das Gate "+
						"selbst ist damit unbelegt.",
					role, rt.method, rt.path, body.Code)
				assert.Equal(t, []string{"Admin", "SecurityAnalyst"}, body.RequiredRoles,
					"%s %s nennt eine andere Schreibrolle als erwartet", rt.method, rt.path)
			}

			// Gegenprobe auf Nicht-Vakuität: die berechtigten Rollen kommen durch
			// die Rollenprüfung. Ohne sie wäre die Probe oben auch dann grün, wenn
			// die Route für JEDEN 403 gäbe.
			for _, role := range []string{"Admin", "SecurityAnalyst"} {
				rec := w5aRequest(t, e, rt.method, rt.path, issueTok(t, key, role))
				if rec.Code != http.StatusForbidden {
					continue
				}
				var body auth.InsufficientRoleResponse
				_ = json.Unmarshal(rec.Body.Bytes(), &body)
				assert.NotEqual(t, "AUTH_INSUFFICIENT_ROLE", body.Code,
					"%s wird von der Rollenprüfung auf %s %s abgewiesen — die Ablehnungen "+
						"oben belegen dann nichts", role, rt.method, rt.path)
			}
		})
	}
}

// TestAllowlistEntriesStillMatchARegisteredRoute hält beide Ausnahmelisten
// dieses Pakets an eine echte Route gebunden.
//
// Anlass ist R1-W5A-N2, aber der Befund ist allgemeiner: eine Ausnahme wird
// einmal begründet und danach nicht wieder angefasst. Bleibt sie stehen,
// nachdem die Route umbenannt oder entfernt wurde, ist sie eine Vorab-Erlaubnis
// für einen Pfad, den heute niemand prüft — und der nächste Handler, der
// zufällig so heißt, kommt ungegated durch, ohne dass ein Test rot wird.
// Dieselbe Disziplin setzen scripts/check_routes.py (check_allowlist_evidence)
// und scripts/lint-orgid-queries.py bereits durch; hier fehlte sie.
//
// Geprüft wird in BEIDE Richtungen, und die zweite ist die wichtigere:
//
//	(a) Die Route existiert noch. Eine Ausnahme ohne Route ist eine
//	    Vorab-Erlaubnis für einen Pfad, den niemand mehr prüft.
//	(b) Die Ausnahme wird noch GEBRAUCHT — eine Nur-Lese-Rolle darf auf dieser
//	    Route nicht ohnehin schon von der Rollenprüfung abgewiesen werden.
//
// (b) ist der Fall, an dem R1-W5A-N2 tatsächlich hing: die beiden
// Notification-Routen existierten die ganze Zeit, ihre Einträge waren also nach
// (a) unauffällig. Sobald eine allow-gelistete Route ein Gate bekommt, wird ihr
// Eintrag zu totem Gewicht — und totes Gewicht in einer Ausnahmeliste ist genau
// das, was beim nächsten Durchsehen als „schon geprüft" gelesen wird. Dieselbe
// Gegenrichtung erzwingt ci.yml für KNOWN_UNCOVERED: eine Ausnahme, die
// inzwischen mitläuft, MUSS die Liste schrumpfen lassen.
//
// Was der Test NICHT kann: die Begründung lesen. Ob „persistiert nichts"
// stimmt, muss ein Mensch am Handler-Rumpf prüfen — der Test hält die Liste nur
// so klein, dass das zumutbar bleibt.
func TestAllowlistEntriesStillMatchARegisteredRoute(t *testing.T) {
	key, err := auth.GenerateSymmetricKey(testHexKey)
	require.NoError(t, err)
	viewerTok := issueTok(t, key, "Viewer")

	lists := []struct {
		name    string
		entries map[string]bool
		router  *echo.Echo
	}{
		{"viewerWriteAllow", viewerWriteAllow, buildSharedRouter(t)},
		{"aiViewerWriteAllow", aiViewerWriteAllow, buildAIRouter(t)},
	}

	for _, l := range lists {
		l := l
		t.Run(l.name, func(t *testing.T) {
			registered := map[string]string{} // "METHOD /pfad" → Pfad mit :params
			for _, r := range l.router.Routes() {
				registered[r.Method+" "+r.Path] = r.Path
			}
			require.NotEmpty(t, registered,
				"%s: der Router hat keine Route registriert — die Prüfung unten wäre "+
					"vakuos und würde jede Ausnahme durchwinken", l.name)
			require.NotEmpty(t, l.entries,
				"%s ist leer — dann gehört die Liste gelöscht, nicht leer gepflegt", l.name)

			for entry := range l.entries {
				path, ok := registered[entry]
				if !assert.True(t, ok,
					"%s enthält %q, aber diese Route ist nicht (mehr) registriert. "+
						"Eine Ausnahme ohne Route erlaubt im Voraus etwas, das niemand "+
						"mehr prüft — Eintrag streichen.", l.name, entry) {
					continue
				}

				method := entry[:strings.Index(entry, " ")]
				reqPath := strings.NewReplacer(
					":id", "00000000-0000-0000-0000-000000000000",
					":run_id", "00000000-0000-0000-0000-000000000000",
					":token", "probe", ":name", "probe").Replace(path)

				rec := w5aRequest(t, l.router, method, reqPath, viewerTok)
				if rec.Code != http.StatusForbidden {
					continue // Ausnahme wird gebraucht: der Viewer kommt durch.
				}
				var body auth.InsufficientRoleResponse
				_ = json.Unmarshal(rec.Body.Bytes(), &body)
				assert.NotEqual(t, "AUTH_INSUFFICIENT_ROLE", body.Code,
					"%s führt %q als Ausnahme, aber die Rollenprüfung weist einen Viewer "+
						"dort ohnehin ab. Der Eintrag ist tot und gehört gestrichen — "+
						"eine überflüssige Ausnahme liest sich beim nächsten Durchsehen "+
						"wie eine geprüfte.", l.name, entry)
			}
			t.Logf("%s: %d Ausnahmen, alle an eine registrierte und ungegatete Route gebunden (von %d Routen)",
				l.name, len(l.entries), len(registered))
		})
	}
}

func w5aRequest(t *testing.T, e *echo.Echo, method, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}
