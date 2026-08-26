// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Dieser Test prueft die ABDECKUNG von ValidateUUIDParams, nicht sein Verhalten.
//
// Der Unterschied hat gebissen: uuid_param_test.go beweist, dass der Guard eine
// kaputte UUID ablehnt — und war gruen, waehrend vier Routen ueberhaupt nicht
// hinter ihm hingen (admin, alerting, integrations haengen direkt an `protected`,
// nie an einer Modul-Gruppe, wo der Guard urspruenglich montiert war). Ein
// Verhaltenstest sagt "der Guard funktioniert"; er sagt nichts darueber, wo er
// montiert ist — und genau dort war der Bug.
//
// Vier Entwurfsentscheidungen, alle gegen genau diese Wiederholung:
//
//  1. Der Baum kommt aus setupEcho(), also aus der ECHTEN Registrierung in
//     routes.go. Ein nachgebauter echo.New() (wie in rbaccov) traegt den Guard
//     gar nicht, weil er am Mount-Punkt sitzt und nicht in den Paket-Registern —
//     ein solcher Test wuerde seinen eigenen Nachbau pruefen.
//
//  2. Der Test kennt nonUUIDParamNames NICHT und darf es nicht. Fragte er die
//     Liste ab, wuerde eine neue Route mit einem unbekannten Param-Namen
//     (`:widget_id`) uebersprungen: Test gruen, Route ungeschuetzt — dieselbe
//     Teilmengen-Luecke, nur eine Ebene hoeher. Stattdessen prueft er die
//     Invariante listenfrei: KEINE Route darf auf einen kaputten Pfad-Param mit
//     500 antworten, egal wie der Param heisst. Ein 404/400/200 ist in Ordnung;
//     500 heisst, der Wert ist ungeprueft bis in einen ::uuid-Cast durchgereicht
//     worden.
//
//  3. "Kein 500" ist von JEDEM Kurzschluss vor dem Handler erfuellt — das war
//     bis Codeaudit v5c (R1-SA10-05) die Luecke im Gate selbst. Eine Route, die
//     mit 401/402/403 antwortet, bevor der Guard oder der Handler den Wert je
//     gesehen hat, hat NICHTS bewiesen: sie bestand vakuos. Gemessen sind das 22
//     von 424 Routen, davon 12 mit einem echten UUID-Param (8 SCIM, 4 Auditor) —
//     und ausgerechnet die haengen an `api` statt an `protected`, tragen den
//     Guard also gar nicht. Das Gate war blind genau dort, wo der Guard fehlt.
//     Solche Routen zaehlen jetzt nicht mehr als "geprueft", sondern landen
//     namentlich im Topf `unproven`, der gegen knownUnprovenRoutes abgeglichen
//     wird: ein NEUER Kurzschluss ist rot, ein WEGGEFALLENER ebenso — sonst
//     bleibt eine Verbesserung unbemerkt und die Liste verrottet zu einer
//     Sammlung von Behauptungen ueber einen Zustand, den es nicht mehr gibt.
//
//  4. Das zweite Binary wird nicht verschwiegen. cmd/billing baut drei eigene
//     echo.New()-Baeume und ist von setupEcho() aus per Konstruktion unsichtbar;
//     seine 15 parametrisierten Routen hat dieses Gate nie beschossen und konnte
//     es nie. Statt sie stillschweigend wegzulassen (dieselbe Klasse wie ein
//     Gate, das Eingaben ueberspringt, die es nicht parsen kann, und trotzdem
//     "OK" meldet), zaehlt und benennt sie TestBillingBinaryParamRoutesAreAnAcknowledgedGap
//     weiter unten.
var paramSeg = regexp.MustCompile(`:[a-zA-Z_]+`)

// junkParamValue ist syntaktisch keine UUID, aber ein voellig normales Pfad-Segment.
// Kein Slash, keine Sonderzeichen: Der Test soll den ::uuid-Cast treffen, nicht das
// Routing oder einen URL-Parser.
const junkParamValue = "not-a-uuid"

// shortCircuitCodes sind die Antworten, aus denen NICHTS ueber den Guard folgt:
// Der Request ist abgewiesen worden, bevor der kaputte Wert einen ::uuid-Cast
// erreichen konnte.
//
// Warum genau diese drei, und warum 403 dazugehoert: Auf dem echten Baum laeuft
// ValidateUUIDParams als Gruppen-Middleware von `protected` und damit VOR CSRF,
// MFA, Lizenz- und Rollenpruefung (cmd/api/routes.go — `protected := api.Group(...)`,
// danach `protected.Use(...)`). Eine Route hinter dem Guard antwortet auf einen
// kaputten UUID-Param deshalb IMMER mit 400, nie mit 403. Sieht das Gate ein 403,
// ist entweder der Guard nicht montiert, oder alle Params der Route stehen auf der
// Denylist und CSRF hat uebernommen — in beiden Faellen ist ueber den ::uuid-Cast
// nichts bewiesen.
//
// Die Einordnung ist bewusst KONSERVATIV und kann nicht unterscheiden, ob ein 401
// aus einer Middleware oder aus dem Handler selbst kommt (Auditor-Accept und der
// Personio-Webhook pruefen ihr Token im Handler). Ein zu Unrecht als "unbewiesen"
// gefuehrter Eintrag kostet eine Zeile Liste; ein zu Unrecht als "bewiesen"
// gezaehlter Eintrag ist genau der Fehler, den dieses Gate nicht mehr machen soll.
var shortCircuitCodes = map[int]bool{
	http.StatusUnauthorized:    true, // 401 — fremde Auth (SCIM-Token, Auditor-Session, Webhook-HMAC)
	http.StatusPaymentRequired: true, // 402 — Feature-Gate (Pro) vor allem anderen
	http.StatusForbidden:       true, // 403 — CSRF / Rolle / MFA, laeuft also NACH dem Guard
}

// Zwei Gruende, die sich wiederholen — als Konstante, damit jede einzelne
// Log-Zeile fuer sich lesbar bleibt. Ein "dito" im sortierten Protokoll steht
// neben irgendeinem Nachbarn, nicht neben dem gemeinten.
const (
	scimNoGuard    = "SCIM haengt an `api`, nicht an `protected` — kein ValidateUUIDParams; 402 Feature-Gate vor der Messung"
	auditorNoGuard = "Auditor-Portal haengt an `api` mit AuditorAuth — kein ValidateUUIDParams; 401 ohne Auditor-Session"
)

// knownUnprovenRoutes ist die begruendete, vollstaendige Liste der Routen, ueber
// die dieses Gate heute nichts beweisen kann. Sie ist KEINE Ausnahme vom Guard,
// sondern die ausgewiesene Blindstelle des Gates — der Nenner, den es NICHT
// abdeckt.
//
// Zwei Klassen, und die erste ist eine echte Luecke, keine Messluecke:
//
//   - LUECKE: SCIM und das Auditor-Portal mounten auf `api`, nicht auf
//     `protected` (cmd/api/routes.go: `scimSvc.Register(api.Group("/scim/v2"), …)`,
//     `vaktcomply.RegisterAuditor(api.Group("/auditor/vaktcomply", …))`). Sie
//     tragen ValidateUUIDParams NICHT. Ein Paseto-Admin-Token kommt an ihrer
//     eigenen Auth bzw. am Feature-Gate nicht vorbei, die Probe endet bei
//     401/402 — wer aber einen gueltigen SCIM-Token hat, reicht `not-a-uuid` bis
//     in den Handler. Das zu schliessen heisst, den Mount-Punkt in
//     cmd/api/routes.go zu aendern; das ist bewusst NICHT Teil dieses Tests.
//
//   - MESSLUECKE: Routen, deren Params allesamt auf der Denylist stehen
//     (`:name`, `:email`, `:type`, `:code`, `:control_ref`, `:severity`,
//     `:token`). Der Guard laesst den Wert absichtlich passieren, danach
//     antwortet CSRF/Feature-Gate. Der Handler wird nicht erreicht, ueber ihn
//     ist nichts bewiesen.
//
// Wer eine Route hier eintraegt, nennt den Grund. Wer eine Route verbessert,
// traegt sie aus — das Gate erzwingt beides.
var knownUnprovenRoutes = map[string]string{
	// ── LUECKE: eigener Auth-Pfad, kein ValidateUUIDParams (12 Routen) ────────
	"GET /api/v1/scim/v2/Users/:id":                            scimNoGuard,
	"PUT /api/v1/scim/v2/Users/:id":                            scimNoGuard,
	"PATCH /api/v1/scim/v2/Users/:id":                          scimNoGuard,
	"DELETE /api/v1/scim/v2/Users/:id":                         scimNoGuard,
	"GET /api/v1/scim/v2/Groups/:id":                           scimNoGuard,
	"PUT /api/v1/scim/v2/Groups/:id":                           scimNoGuard,
	"PATCH /api/v1/scim/v2/Groups/:id":                         scimNoGuard,
	"DELETE /api/v1/scim/v2/Groups/:id":                        scimNoGuard,
	"GET /api/v1/auditor/vaktcomply/frameworks/:id":            auditorNoGuard,
	"GET /api/v1/auditor/vaktcomply/frameworks/:id/controls":   auditorNoGuard,
	"GET /api/v1/auditor/vaktcomply/frameworks/:id/report.pdf": auditorNoGuard,
	"GET /api/v1/auditor/vaktcomply/frameworks/:id/soa.pdf":    auditorNoGuard,

	// ── MESSLUECKE: Token-/Signaturpruefung im Handler ────────────────────────
	"POST /api/v1/auditor/accept/:token":            "public, `:token` auf der Denylist; der Handler selbst antwortet 401 auf ein unbekanntes Token",
	"POST /api/v1/vakthr/webhooks/personio/:org_id": "public Webhook, HMAC-Signaturpruefung antwortet 401 vor jedem Cast",

	// ── MESSLUECKE: nur Denylist-Params, CSRF/Feature-Gate uebernimmt ─────────
	"DELETE /api/v1/admin/accounts/:email/unlock":            "`:email` auf der Denylist → Guard laesst passieren, CSRF antwortet 403",
	"POST /api/v1/admin/users/:email/password-reset-token":   "`:email` auf der Denylist → Guard laesst passieren, CSRF antwortet 403",
	"POST /api/v1/vaktcomply/frameworks/:name/enable":        "`:name` auf der Denylist → CSRF 403",
	"POST /api/v1/vaktcomply/physical-templates/:code/apply": "`:code` auf der Denylist → CSRF 403",
	"PUT /api/v1/vaktcomply/soa/entries/:control_ref":        "`:control_ref` auf der Denylist → CSRF 403",
	"PUT /api/v1/vaktscan/sla-policies/:severity":            "`:severity` auf der Denylist → CSRF 403",
	"GET /api/v1/vaktcomply/bsi/reports/:type":               "`:type` auf der Denylist; BSI-Feature-Gate antwortet 402 vor dem Handler",
	"GET /api/v1/vaktcomply/bsi/reports/:type/preview":       "`:type` auf der Denylist; BSI-Feature-Gate antwortet 402 vor dem Handler",
}

func TestUUIDParamGuardCoversEveryParameterisedRoute(t *testing.T) {
	dbURL := os.Getenv("VAKT_DB_URL")
	secret := os.Getenv("VAKT_SECRET_KEY")
	if dbURL == "" || secret == "" || os.Getenv("VAKT_REDIS_URL") == "" {
		t.Skip("needs VAKT_DB_URL + VAKT_REDIS_URL + VAKT_SECRET_KEY (CI sets all three)")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err, "connect")
	defer pool.Close()

	token := seedOrgAndToken(ctx, t, pool, secret)
	e, _ := setupEcho(ctx, testConfig())

	var checked, proved, skipped int
	var skippedPaths []string
	unproven := map[string]int{}

	for _, r := range e.Routes() {
		if !strings.Contains(r.Path, ":") {
			continue // keine Pfad-Params, nichts zu casten
		}
		// Echo registriert den 404/405-Fallback selbst; der hat keine echte Route.
		if strings.Contains(r.Name, "glob..func") && r.Path == "/*" {
			skipped++
			skippedPaths = append(skippedPaths, r.Method+" "+r.Path)
			continue
		}

		reqPath := paramSeg.ReplaceAllString(r.Path, junkParamValue)
		key := r.Method + " " + r.Path

		var code int
		t.Run(key, func(t *testing.T) {
			req := httptest.NewRequest(r.Method, reqPath, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			code = rec.Code

			assert.NotEqual(t, http.StatusInternalServerError, rec.Code,
				"%s antwortet auf einen kaputten Pfad-Param mit 500 — der Wert erreicht ungeprueft einen ::uuid-Cast "+
					"(SQLSTATE 22P02). Entweder steht der Param-Name faelschlich in nonUUIDParamNames "+
					"(internal/shared/middleware/uuid_param.go), oder die Route haengt nicht hinter "+
					"ValidateUUIDParams. Body: %s", key, strings.TrimSpace(rec.Body.String()))
		})
		checked++
		if shortCircuitCodes[code] {
			unproven[key] = code
		} else {
			proved++
		}
	}

	unprovenKeys := sortedKeys(unproven)
	t.Logf("checked=%d routes with path params, proved=%d, unproven=%d, skipped=%d %v",
		checked, proved, len(unproven), skipped, skippedPaths)
	for _, k := range unprovenKeys {
		t.Logf("  unproven %d  %s  (%s)", unproven[k], k, knownUnprovenRoutes[k])
	}

	// (a) Neue Blindstelle → rot, mit Namen. Ohne diese Zusicherung waere jede
	// kuenftig vor den Guard gezogene Route ein stiller Zugewinn an Nichtwissen:
	// Das Gate haette sie weiterhin als bestanden gezaehlt.
	for _, k := range unprovenKeys {
		if _, known := knownUnprovenRoutes[k]; !known {
			t.Errorf("%s schliesst mit %d kurz, BEVOR der kaputte Pfad-Param einen Handler oder "+
				"ValidateUUIDParams erreicht — ueber diese Route beweist das Gate nichts, sie bestand vakuos. "+
				"Entweder die Route hinter `protected` mounten (dann antwortet sie 400 INVALID_UUID_PARAM), "+
				"oder sie mit Begruendung in knownUnprovenRoutes eintragen.", k, unproven[k])
		}
	}

	// (b) Weggefallene Blindstelle → ebenfalls rot, mit Namen. Eine Liste, die nur
	// waechst, ist eine Sammlung von Behauptungen ueber einen Zustand, den es
	// vielleicht nicht mehr gibt — und der naechste Leser glaubt ihr.
	for k := range knownUnprovenRoutes {
		if _, still := unproven[k]; !still {
			t.Errorf("%s steht in knownUnprovenRoutes, schliesst aber nicht mehr kurz — die Route ist "+
				"jetzt beweisbar (oder es gibt sie nicht mehr). Eintrag entfernen, sonst behauptet die "+
				"Liste eine Blindstelle, die gefixt ist.", k)
		}
	}

	// (c) Nenner. Ein leerer Baum (setupEcho ohne DB liefert nur /health) oder ein
	// nicht mehr passendes Param-Muster liefe stumm durch und meldete gruen.
	// Stille ist kein Beleg.
	require.Greater(t, checked, 400,
		"der Test hat fast keine parametrisierten Routen gesehen — das ist ein Defekt AM TEST, nicht ein sauberes Ergebnis")
	// (d) Und der Nenner der tatsaechlichen BEWEISE, nicht nur der Versuche: Wuerde
	// ein Middleware-Umbau alle Routen vor den Guard kurzschliessen lassen, waere
	// `checked` unveraendert und das Gate gruen — genau die Vakuitaet, gegen die
	// dieser Test gebaut ist.
	require.Greater(t, proved, 380,
		"zu wenige Routen haben den kaputten Param tatsaechlich bis zu Guard oder Handler getragen "+
			"(proved=%d von checked=%d) — das Gate misst gerade fast nichts", proved, checked)
}

// ── cmd/billing: die benannte, gezaehlte Luecke ──────────────────────────────

// billingRouteRE findet Routen-Literale mit Pfad-Param in den Quellen, die das
// billing-Binary registriert. Bewusst eine Quelltext-Probe und kein Nachbau:
// cmd/billing/main.go baut seine drei echo.New()-Baeume inline in main(), es gibt
// keine setupEcho()-artige Funktion, die ein Test aufrufen koennte. Ein hier
// zusammengesteckter Nachbau waere genau der Fehler, den Punkt 1 oben beschreibt
// — er wuerde eine spaeter in main() ergaenzte Middleware nicht tragen und
// entweder falsch gruen oder falsch rot melden.
var billingRouteRE = regexp.MustCompile(`\.(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS)\("([^"]*:[^"]*)"`)

// knownBillingParamRoutes ist der anerkannte Stand der parametrisierten Routen im
// billing-Binary — der Teil der Angriffsflaeche, den das Gate oben per
// Konstruktion nicht beschiessen kann.
//
// Der Eintrag ist eine Zusage, kein Freibrief: Waechst die Liste, hat jemand eine
// Route in ein Binary gelegt, das keinen UUID-Guard traegt, und muss das bewusst
// tun. Schrumpft sie, ist der Eintrag stale.
const (
	billingPortalToken = "`:token` ist ein Hash-Lookup im MSP-Portal, kein uuid-Cast"
	billingAdminPanel  = "Admin-Panel auf dem zweiten Listener (standardmaessig nur Loopback); `:id` wird als uuid gelesen — kein Guard im billing-Binary"
)

var knownBillingParamRoutes = map[string]string{
	"GET /billing/quote-request/:id/approve": "Ein-Klick-Freigabelink aus der Mail; `:id` wird als uuid gelesen — kein Guard im billing-Binary",
	"GET /billing/portal/:token":             billingPortalToken,
	"POST /billing/portal/:token/seat":       billingPortalToken,
	"GET /subscriptions/:id":                 billingAdminPanel,
	"GET /invoices/:id/pdf":                  billingAdminPanel,
	"POST /subscriptions/:id/approve":        billingAdminPanel,
	"POST /subscriptions/:id/notes":          billingAdminPanel,
	"POST /subscriptions/:id/discount":       billingAdminPanel,
	"POST /subscriptions/:id/convert":        billingAdminPanel,
	"POST /subscriptions/:id/resend":         billingAdminPanel,
	"POST /invoices/:id/remind":              billingAdminPanel,
	"POST /subscriptions/:id/cancel":         billingAdminPanel,
	"POST /subscriptions/:id/seat":           billingAdminPanel,
	"POST /subscriptions/:id/portal":         billingAdminPanel,
	"POST /subscriptions/:id/revoke":         billingAdminPanel,
}

// TestBillingBinaryParamRoutesAreAnAcknowledgedGap haelt fest, was das
// UUID-Coverage-Gate NICHT sieht.
//
// cmd/billing ist ein zweiter Prozess mit drei eigenen echo.New()-Baeumen
// (public, admin, metrics). setupEcho() kennt ihn nicht und kann ihn nicht
// kennen; jede seiner parametrisierten Routen ist von dem Gate oben ungeprueft.
// Sie einfach wegzulassen waere dieselbe Klasse wie ein Gate, das nicht parsbare
// Eingaben still ueberspringt und trotzdem "OK" meldet — deshalb wird die Luecke
// hier gezaehlt, benannt und gegen Wachstum verriegelt.
func TestBillingBinaryParamRoutesAreAnAcknowledgedGap(t *testing.T) {
	root, err := findRepoRoot()
	require.NoError(t, err, "repo root")

	found := map[string]string{} // "METHOD /pfad" → datei:zeile
	var scannedFiles int
	for _, dir := range []string{"cmd/billing", "internal/billing"} {
		err := filepath.Walk(filepath.Join(root, dir), func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			src, readErr := os.ReadFile(path) //nolint:gosec // Testpfad aus dem Repo-Root
			if readErr != nil {
				return readErr
			}
			scannedFiles++
			rel, _ := filepath.Rel(root, path)
			for _, line := range strings.Split(string(src), "\n") {
				for _, m := range billingRouteRE.FindAllStringSubmatch(line, -1) {
					found[m[1]+" "+m[2]] = rel
				}
			}
			return nil
		})
		require.NoError(t, err, "walk %s", dir)
	}

	// Nenner ausweisen — ein Scan, der nichts findet, weil sich das
	// Registrierungsmuster geaendert hat, darf nicht als "sauber" durchgehen.
	require.Greater(t, scannedFiles, 10, "der Scan hat fast keine billing-Quelldateien gesehen — Defekt AM GATE")
	t.Logf("cmd/billing: %d Go-Dateien gescannt, %d parametrisierte Routen gefunden — "+
		"alle UNGEPRUEFT von TestUUIDParamGuardCoversEveryParameterisedRoute (eigenes Binary, eigene echo.New()-Baeume)",
		scannedFiles, len(found))
	for _, k := range sortedKeys(found) {
		t.Logf("  ungeprueft  %s  (%s)", k, found[k])
	}

	for _, k := range sortedKeys(found) {
		if _, known := knownBillingParamRoutes[k]; !known {
			t.Errorf("neue parametrisierte Route %s (%s) im billing-Binary. Dieses Binary traegt KEIN "+
				"ValidateUUIDParams, und das API-Coverage-Gate kann es nicht beschiessen (eigener Prozess, "+
				"drei eigene echo.New()). Entweder den Guard dort montieren oder den Eintrag mit Begruendung "+
				"in knownBillingParamRoutes aufnehmen.", k, found[k])
		}
	}
	for k := range knownBillingParamRoutes {
		if _, still := found[k]; !still {
			t.Errorf("%s steht in knownBillingParamRoutes, wurde aber nicht mehr gefunden — Eintrag entfernen "+
				"(oder das Scan-Muster ist kaputt, dann ist es ein Defekt AM GATE).", k)
		}
	}

	// Die Ursache der Luecke selbst festhalten: drei inline in main() gebaute
	// Router. Kommt ein vierter dazu, ist die obige Begruendung unvollstaendig.
	mainSrc, err := os.ReadFile(filepath.Join(root, "cmd/billing/main.go")) //nolint:gosec // Testpfad aus dem Repo-Root
	require.NoError(t, err)
	listeners := strings.Count(string(mainSrc), "echo.New()")
	assert.Equal(t, 3, listeners,
		"cmd/billing/main.go baut %d echo.New()-Baeume statt der dokumentierten 3 — die Luecken-Beschreibung "+
			"dieses Gates ist damit veraltet. Sauberer Ausweg: die Registrierung in eine aufrufbare "+
			"setupBillingEcho()-Funktion ziehen, dann kann ein Test den ECHTEN Baum beschiessen statt ihn "+
			"statisch abzuzaehlen.", listeners)
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
