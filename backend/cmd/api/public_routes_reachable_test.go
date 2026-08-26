// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Dieses Gate (S127-5, "G10") ist das Gegenstueck zu rbaccov: Das prueft, dass
// eine Schreibroute ohne Rolle 403 gibt. Es prueft NIEMAND, dass eine bewusst
// OEFFENTLICHE Route ohne Token erreichbar bleibt — und genau in diese Richtung
// fiel Sprint 127.
//
// Der Defekt damals: Fuenf Routen trugen im Code den Kommentar "public, no auth"
// und hingen trotzdem unter `protected`. Aufgerufen werden sie per Definition von
// Leuten ohne Token — der Tracking-Pixel vom Mail-Client des Phishing-Ziels, der
// Klick-Link von dessen Browser, der Share-Link vom externen Empfaenger. Alle
// antworteten 401. Damit war das Kernfeature von Vakt Aware seit Einfuehrung
// funktionslos, und die daraus gespeiste Klickrate meldete strukturell 0 % — ein
// Nachweis, der etwas Falsches behauptet, und zwar plausibel falsch.
//
// DER MOUNT-PUNKT WAR DER BUG. Deshalb ist der einzige zulaessige Baum fuer
// dieses Gate der aus setupEcho(), also die ECHTE Registrierung in
// cmd/api/routes.go.
//
// Bis Codeaudit v5c (R1-SA12-D06 / R1-SA10-04) tat es genau das nicht: Es baute
// ein eigenes echo.New(), rief RegisterPublic selbst auf und las routes.go nie.
// Ein Rueckfall auf `protected.Group(...)` — also die Wiederkehr des Bugs, den
// dieses Gate verhindern soll — waere gruen geblieben, denn der Nachbau haette
// weiterhin ohne Auth-Middleware gemountet. Das Gate pruefte seinen eigenen
// Nachbau. Dieselbe Lehre steht seit S131 im Nachbarn
// uuid_param_coverage_test.go und galt hier trotzdem nicht.
//
// Drei Zusicherungen tragen das Gate:
//
//  1. Jede kuratierte Route MUSS im echten Baum registriert sein. Ohne diesen
//     Schritt bestuende eine geloeschte oder umbenannte Route vakuos: Ein 404 ist
//     kein 401, die Statuspruefung allein waere zufrieden.
//  2. Ohne Authorization-Header darf keine von ihnen von AuthMiddleware oder
//     CSRF-Guard abgewiesen werden (geprueft am Fehlercode, nicht am nackten
//     Status — siehe authRejectMarker).
//  3. Gegenprobe: Eine geschuetzte Route MUSS ohne Token 401 mit genau diesem
//     Fehlercode antworten, und eine geschuetzte Schreibroute ohne CSRF-Header
//     403 mit jenem. Faellt eine der Middlewares aus oder wird ein Code
//     umbenannt, ist Punkt 2 fuer den ganzen Baum trivial erfuellt — das Gate
//     waere gruen, waehrend die API offen steht.
type publicRoute struct {
	method string // HTTP-Verb
	path   string // Echo-Routen-Template, exakt wie registriert (mit :param)
	probe  string // konkreter Pfad fuer den Request
	why    string // wer ruft das ohne Token auf, und warum muss es so sein
}

// publicRoutes ist die kuratierte Liste der Routen, die ohne Token erreichbar
// sein MUESSEN. Sie steht bewusst im Testcode und nicht in einer externen Datei:
// Sie ist eine Behauptung ueber das Produktverhalten ("ein Fremder ohne Konto
// muss das aufrufen koennen"), keine Konfiguration — und sie soll im selben Diff
// begruendet werden, in dem eine Route hinzukommt.
//
// Nicht enthalten und bewusst nicht: /health, /auth/login und die
// Setup-/NIS2-Wizard-Routen sind ebenfalls unauthentifiziert, aber ihre
// Erreichbarkeit ist durch eigene Tests (health_test.go, openapi_contract_test.go)
// abgedeckt. Diese Liste fuehrt die Routen, deren Aufrufer PER DEFINITION kein
// Konto in dieser Instanz hat: Phishing-Ziel, Share-Empfaenger, Lieferant,
// Auditor, betroffene Person, Fremdsystem.
var publicRoutes = []publicRoute{
	// ── Vakt Aware: der Bug von S127 selbst ──────────────────────────────────
	{http.MethodGet, "/api/v1/vaktaware/track/:token", "/api/v1/vaktaware/track/sometoken",
		"Open-Pixel — laedt der Mail-Client des Phishing-Ziels, niemals ein eingeloggter Nutzer"},
	{http.MethodGet, "/api/v1/vaktaware/t/:token", "/api/v1/vaktaware/t/sometoken",
		"Klick-Link — oeffnet der Browser des Ziels"},
	{http.MethodPost, "/api/v1/vaktaware/t/:token/submit", "/api/v1/vaktaware/t/sometoken/submit",
		"Formular-Abgabe der Landing-Page; kein Session-Cookie, also auch kein CSRF-Token"},
	{http.MethodPost, "/api/v1/vaktaware/phish-report", "/api/v1/vaktaware/phish-report",
		"Melde-Button/Weiterleitung aus dem Mail-Client"},

	// ── Vakt Vault: Share-Link ───────────────────────────────────────────────
	{http.MethodGet, "/api/v1/vaktvault/share/:token", "/api/v1/vaktvault/share/sometoken",
		"externer Empfaenger eines Secret-Share-Links, hat kein Konto"},

	// ── Trust Center: oeffentliche Seite ─────────────────────────────────────
	{http.MethodGet, "/api/v1/trust/:slug", "/api/v1/trust/some-org-slug",
		"oeffentliche Trust-Center-Seite; muss unter /api/v1 liegen, weil Caddy nur /api/* proxyt (S131-D4)"},

	// ── Vakt Comply: Auditor- und Lieferantenportal, Richtlinien-Bestaetigung ─
	{http.MethodGet, "/api/v1/vaktcomply/auditor/:token", "/api/v1/vaktcomply/auditor/sometoken",
		"externer Auditor mit Magic-Link"},
	{http.MethodGet, "/api/v1/vaktcomply/auditor/:token/export", "/api/v1/vaktcomply/auditor/sometoken/export",
		"externer Auditor laedt das Evidenzpaket"},
	{http.MethodGet, "/api/v1/vaktcomply/supplier/:token", "/api/v1/vaktcomply/supplier/sometoken",
		"Lieferant beantwortet einen Fragebogen, ohne Konto"},
	{http.MethodPost, "/api/v1/vaktcomply/supplier/:token/save", "/api/v1/vaktcomply/supplier/sometoken/save",
		"Zwischenspeichern durch den Lieferanten"},
	{http.MethodPost, "/api/v1/vaktcomply/supplier/:token/submit", "/api/v1/vaktcomply/supplier/sometoken/submit",
		"Abgabe durch den Lieferanten"},
	{http.MethodPost, "/api/v1/vaktcomply/supplier/:token/upload", "/api/v1/vaktcomply/supplier/sometoken/upload",
		"Nachweis-Upload durch den Lieferanten"},
	{http.MethodGet, "/api/v1/policy-accept/:token", "/api/v1/policy-accept/sometoken",
		"Mitarbeiter bestaetigt eine Richtlinie per Mail-Link, ggf. ohne Vakt-Konto"},
	{http.MethodPost, "/api/v1/policy-accept/:token", "/api/v1/policy-accept/sometoken",
		"dieselbe Bestaetigung als Schreibvorgang — darf deshalb auch kein CSRF verlangen"},

	// ── Vakt Privacy: DSR-Portal fuer betroffene Personen ────────────────────
	{http.MethodGet, "/api/v1/vaktprivacy/dsr-portal/status/:token", "/api/v1/vaktprivacy/dsr-portal/status/sometoken",
		"betroffene Person fragt den Stand ihres Auskunftsersuchens ab"},
	{http.MethodGet, "/api/v1/vaktprivacy/dsr-portal/:slug/info", "/api/v1/vaktprivacy/dsr-portal/some-org/info",
		"oeffentliche Portalseite der Organisation"},
	{http.MethodPost, "/api/v1/vaktprivacy/dsr-portal/:slug/submit", "/api/v1/vaktprivacy/dsr-portal/some-org/submit",
		"betroffene Person stellt ein Ersuchen nach Art. 15 ff."},

	// ── Auditor-Einladung ────────────────────────────────────────────────────
	{http.MethodPost, "/api/v1/auditor/accept/:token", "/api/v1/auditor/accept/sometoken",
		"externer Auditor loest seine Einladung ein — der Vorgang, der ihm ueberhaupt erst eine Session gibt"},

	// ── Vakt HR: eingehender Personio-Webhook ────────────────────────────────
	{http.MethodPost, "/api/v1/vakthr/webhooks/personio/:org_id", "/api/v1/vakthr/webhooks/personio/00000000-0000-0000-0000-000000000000",
		"Fremdsystem authentifiziert per HMAC-Signatur, nicht per Paseto — hinter `protected` waere es tot"},
}

// protectedProbe ist die Gegenprobe zu Zusicherung 3. Ohne sie waere ein
// kompletter Ausfall der Auth-Middleware fuer dieses Gate ein Erfolg.
const protectedProbe = "/api/v1/auth/me"

// csrfProbe ist eine geschuetzte Schreibroute; sie dient nur dazu, den
// CSRF-Markercode festzunageln. Der Guard antwortet, bevor der Handler laeuft —
// die Probe hat deshalb keine Nebenwirkung. Bewusst NICHT /auth/logout: das
// haengt an `authGroup`, nicht an `protected`, und antwortet 200 (nachgemessen).
const csrfProbe = "/api/v1/onboarding/dismiss"

// Die Fehlercodes, mit denen die beiden Guards eine Anfrage abweisen. Auf sie
// wird geprueft, NICHT auf den nackten Statuscode — und das ist eine Korrektur
// am Gate selbst: Die Handler von `auditor/accept/:token` und dem
// Personio-Webhook antworten SELBST mit 401 ("ungueltige Einladung", "Signatur
// fehlt"). Das ist die richtige Antwort einer erreichbaren Route. Ein Gate, das
// auf `!= 401` prueft, faerbt genau die zwei Routen rot, die ihr Token korrekt
// im Handler pruefen — und wer das "behebt", indem er sie aus der Liste nimmt,
// hat die Abdeckung verkleinert statt einen Fehler gefunden.
//
// Umgekehrt ist der Marker praezise: OHNE Authorization-Header antwortet
// AuthMiddleware IMMER mit AUTH_MISSING_TOKEN, egal welche Route. Steht er in
// der Antwort, haengt die Route hinter Auth — das IST der S127-Fehler.
//
// Damit die Marker nicht stillschweigend veralten (jemand benennt den Code um,
// das Gate prueft danach auf eine Zeichenkette, die nie mehr vorkommt, und ist
// fuer immer gruen), nagelt TestPublicRoutesReachableWithoutToken beide mit
// einer echten Probe gegen eine geschuetzte Route fest.
const (
	authRejectMarker = "AUTH_MISSING_TOKEN"
	csrfRejectMarker = "CSRF_"
)

var publicParamSeg = regexp.MustCompile(`:[a-zA-Z_]+`)

// probeIP gibt jeder Probe eine EIGENE Client-IP (TEST-NET-3, RFC 5737).
//
// Die oeffentlichen Routen sind IP-ratenbegrenzt (vaktaware_track: 10/min/IP,
// vaktvault_share und portal: 30/min/IP). Kaemen alle Proben von derselben
// Adresse — httptest.NewRequest setzt fest 192.0.2.1 —, teilten sie sich einen
// Eimer, und der Test waere ab einer bestimmten Listenlaenge oder beim zweiten
// Lauf innerhalb einer Minute rot mit 429. Das waere kein Fund, sondern ein
// Defekt am Gate: Ein Gate, das bei gesundem Repo rot wird, wird abgeschaltet
// statt gefixt.
//
// Getrennte IPs sind ausserdem die ehrlichere Nachbildung: Jedes Phishing-Ziel,
// jeder Share-Empfaenger sitzt hinter einer eigenen Adresse.
func probeIP(n int) string {
	return fmt.Sprintf("203.0.113.%d:41000", n%250+1)
}

func TestPublicRoutesReachableWithoutToken(t *testing.T) {
	if os.Getenv("VAKT_DB_URL") == "" || os.Getenv("VAKT_SECRET_KEY") == "" || os.Getenv("VAKT_REDIS_URL") == "" {
		t.Skip("needs VAKT_DB_URL + VAKT_REDIS_URL + VAKT_SECRET_KEY (CI sets all three)")
	}

	// Der ECHTE Baum. Kein echo.New(), kein RegisterPublic-Aufruf von Hand —
	// sonst prueft das Gate wieder seinen eigenen Nachbau und uebersieht genau
	// den Mount-Punkt-Fehler, gegen den es gebaut ist.
	e, _ := setupEcho(context.Background(), testConfig())

	registered := map[string]bool{}
	for _, r := range e.Routes() {
		registered[r.Method+" "+r.Path] = true
	}
	require.Greater(t, len(registered), 500,
		"setupEcho() hat fast keine Routen registriert (%d) — ohne DB/Redis/Secret ist dieser Lauf wertlos; "+
			"das ist ein Defekt AM GATE, kein sauberes Ergebnis", len(registered))

	var checked, missing int
	var missingNames []string

	for i, rt := range publicRoutes {
		key := rt.method + " " + rt.path

		// Zusicherung 1: Die Route MUSS es geben. Ein 404 ist kein 401 — ohne
		// diesen Schritt bestuende eine geloeschte Route vakuos.
		if !registered[key] {
			missing++
			missingNames = append(missingNames, key)
			t.Errorf("%s ist als oeffentliche Route gefuehrt, im echten Baum aus setupEcho() aber NICHT "+
				"registriert. Entweder ist sie weggefallen/umbenannt (dann Eintrag hier anpassen) oder die "+
				"Registrierung in cmd/api/routes.go ist kaputt. Erwartetes Muster: %s — "+
				"Aufrufer: %s", key, rt.path, rt.why)
			continue
		}

		// Der Probe-Pfad muss zum Template passen; sonst probt das Gate etwas
		// anderes, als es zu pruefen behauptet.
		wantSegs := len(strings.Split(rt.path, "/"))
		gotSegs := len(strings.Split(rt.probe, "/"))
		require.Equal(t, wantSegs, gotSegs,
			"Probe-Pfad %s passt nicht zum Template %s — das Gate wuerde eine andere Route treffen", rt.probe, rt.path)
		require.NotContains(t, publicParamSeg.ReplaceAllString(rt.probe, ""), ":",
			"Probe-Pfad %s enthaelt noch einen :param", rt.probe)

		t.Run(key, func(t *testing.T) {
			req := httptest.NewRequest(rt.method, rt.probe, nil) // KEIN Authorization-Header
			req.RemoteAddr = probeIP(i)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			body := strings.TrimSpace(rec.Body.String())
			assert.NotContains(t, body, authRejectMarker,
				"%s wird ohne Token von der AuthMiddleware abgewiesen (Status %d). Diese Route ruft %s auf — "+
					"sie haengt hinter `protected`, gehoert aber auf eine Gruppe OHNE Auth (Muster: "+
					"`api.Group(...)` + RegisterPublic in cmd/api/routes.go, nicht `protected.Group(...)`). "+
					"Genau dieser Mount-Punkt-Fehler ist S127. Body: %s", key, rec.Code, rt.why, body)
			assert.NotContains(t, body, csrfRejectMarker,
				"%s wird ohne Token vom CSRF-Guard abgewiesen (Status %d). Beim Public-Mount faellt die ganze "+
					"protected-Kette weg, und CSRF ist das Glied, das man dabei am leichtesten stehen laesst: "+
					"Der externe Browser hat keinen csrf_token-Cookie. Das ist derselbe Bug, nur umlackiert. "+
					"Aufrufer: %s. Body: %s", key, rec.Code, rt.why, body)
		})
		checked++
	}

	// Nenner + Skip-Zaehler. Dieses Gate ueberspringt per Konstruktion nichts:
	// Was es nicht pruefen kann, ist eine fehlende Route, und die ist rot, nicht
	// still. Die Zahl steht trotzdem im Protokoll — ein Gate, das seinen Nenner
	// nicht nennt, meldet Erfolg fuer Arbeit, die es vielleicht nicht getan hat.
	t.Logf("checked=%d oeffentliche Routen (von %d kuratierten), missing=%d %v, skipped=0 — "+
		"Baum aus setupEcho(): %d registrierte Routen",
		checked, len(publicRoutes), missing, missingNames, len(registered))

	require.Equal(t, len(publicRoutes), checked,
		"nicht jede kuratierte oeffentliche Route wurde geprueft (checked=%d von %d) — fehlende: %v",
		checked, len(publicRoutes), missingNames)
	require.GreaterOrEqual(t, len(publicRoutes), 19,
		"die kuratierte Liste ist geschrumpft — wer eine oeffentliche Route entfernt, entfernt auch ihren "+
			"Nachweis; das ist eine bewusste Entscheidung und keine Nebenwirkung")

	// Zusicherung 3 — Gegenprobe, zwei Teile. Ohne sie waere ein Totalausfall der
	// Auth-Middleware fuer dieses Gate ein Erfolg: dann waere JEDE Route ohne
	// Token erreichbar, und die Marker-Pruefungen oben waeren trivial erfuellt.
	// Sie nagelt zugleich die beiden Markercodes fest — benennt sie jemand um,
	// wird dieser Test rot, statt dass die Pruefungen oben stumm ins Leere greifen.
	req := httptest.NewRequest(http.MethodGet, protectedProbe, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code,
		"GET %s antwortet ohne Token mit %d statt 401 — die Auth-Middleware greift nicht. Damit ist die "+
			"Aussage dieses Gates ('diese Routen sind OHNE Token erreichbar') wertlos, weil es ALLE waeren",
		protectedProbe, rec.Code)
	require.Contains(t, rec.Body.String(), authRejectMarker,
		"die Ablehnung der AuthMiddleware traegt nicht mehr den Code %q (Body: %s) — die Marker-Pruefung "+
			"oben greift damit ins Leere und dieses Gate waere ab sofort dauerhaft gruen. Marker anpassen.",
		authRejectMarker, strings.TrimSpace(rec.Body.String()))

	pool, err := pgxpool.New(context.Background(), os.Getenv("VAKT_DB_URL"))
	require.NoError(t, err, "connect")
	defer pool.Close()
	token := seedOrgAndToken(context.Background(), t, pool, os.Getenv("VAKT_SECRET_KEY"))

	require.True(t, registered[http.MethodPost+" "+csrfProbe],
		"die CSRF-Probe %s existiert nicht mehr — ohne sie ist der CSRF-Marker nicht festgenagelt", csrfProbe)
	req = httptest.NewRequest(http.MethodPost, csrfProbe, nil)
	req.Header.Set("Authorization", "Bearer "+token) // gueltiger Token, aber KEIN CSRF-Header
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code,
		"POST %s mit gueltigem Token, aber ohne CSRF-Header antwortet %d statt 403 — der CSRF-Guard greift "+
			"nicht mehr auf `protected`", csrfProbe, rec.Code)
	require.Contains(t, rec.Body.String(), csrfRejectMarker,
		"die Ablehnung des CSRF-Guards traegt nicht mehr den Code-Praefix %q (Body: %s) — die Marker-Pruefung "+
			"oben greift ins Leere. Marker anpassen.", csrfRejectMarker, strings.TrimSpace(rec.Body.String()))
}

// TestPublicOpenPixelIsNoEnumerationOracle haelt die zweite Haelfte von S127 §3c
// fest: Der Open-Pixel darf ein gueltiges nicht von einem ungueltigen Token
// unterscheiden. Antwortete er nur bei bekanntem Token mit 200, koennte ein
// Fremder Kampagnen-Token durchprobieren.
func TestPublicOpenPixelIsNoEnumerationOracle(t *testing.T) {
	if os.Getenv("VAKT_DB_URL") == "" || os.Getenv("VAKT_SECRET_KEY") == "" || os.Getenv("VAKT_REDIS_URL") == "" {
		t.Skip("needs VAKT_DB_URL + VAKT_REDIS_URL + VAKT_SECRET_KEY (CI sets all three)")
	}
	e, _ := setupEcho(context.Background(), testConfig())

	for i, token := range []string{"definitely-invalid", "00000000-0000-0000-0000-000000000000"} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/vaktaware/track/"+token, nil)
		req.RemoteAddr = probeIP(200 + i)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code,
			"der Open-Pixel muss auch fuer ein unbekanntes Token 200 liefern (Token %q, Status %d) — "+
				"jede Abweichung macht ihn zum Aufzaehlungs-Orakel", token, rec.Code)
	}
}
