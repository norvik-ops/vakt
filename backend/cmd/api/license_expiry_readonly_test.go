//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	tcwait "github.com/testcontainers/testcontainers-go/wait"

	"github.com/matharnica/vakt/internal/auth"
	"github.com/matharnica/vakt/internal/license"
	sharedcrypto "github.com/matharnica/vakt/internal/shared/crypto"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
)

// Bildfixierungen wie internal/integration_test/images_test.go — bewusst
// dieselben Digests, damit dieser Test dieselbe Postgres-/Redis-Version faehrt
// wie der Rest der Integrationsschicht.
const (
	expiryImagePostgres = "postgres:16.14-alpine@sha256:57c72fd2a128e416c7fcc499958864df5301e940bca0a56f58fddf30ffc07777"
	expiryImageRedis    = "redis:7.4.10-alpine@sha256:e7723ff73d963f5cc6d9c4643ea3d989527a402a319239054e9472a7fb9219a2"
)

func expiryMigrationsDir(t *testing.T) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller failed")
	// here = .../backend/cmd/api/license_expiry_readonly_test.go → ../../db/migrations
	return filepath.Join(filepath.Dir(here), "..", "..", "db", "migrations")
}

// startExpiryStack bringt Postgres (migriert) und Redis hoch und legt die
// Verbindungsdaten so in die Umgebung, dass setupEcho() sie beim Bauen des
// echten Baums liest.
//
// Warum Testcontainers und nicht VAKT_DB_URL aus dem Job: Dieser Test hing
// vorher an drei Env-Variablen, die NUR der Unit-Job setzt. Mit dem
// Integrations-Tag waere er dort nicht mehr kompiliert und im Integrations-Job
// (rein testcontainers-basiert, kein Service-Container) haette er sich still
// weggeskippt — ein Test, den kein Job ausfuehrt. Genau diese Falle ist in
// ci.yml als bekannt offener Punkt fuer die drei lexware-Dateien beschrieben;
// sie hier zu wiederholen waere derselbe Fehlschluss in neuer Verkleidung.
func startExpiryStack(t *testing.T) (dsn, redisURL string) {
	t.Helper()
	ctx := context.Background()

	pgC, err := postgres.Run(ctx,
		expiryImagePostgres,
		postgres.WithDatabase("vakt_test"),
		postgres.WithUsername("vakt"),
		postgres.WithPassword("vakt"),
		postgres.WithSQLDriver("pgx"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		if strings.Contains(err.Error(), "permission denied") ||
			strings.Contains(err.Error(), "Cannot connect to the Docker daemon") {
			t.Skipf("integration: Docker unavailable (%v)", err)
		}
		t.Fatalf("postgres container: %v", err)
	}
	t.Cleanup(func() { _ = pgC.Terminate(ctx) })

	dsn, err = pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	require.NoError(t, shareddb.RunMigrations(dsn, expiryMigrationsDir(t)))

	rC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        expiryImageRedis,
			ExposedPorts: []string{"6379/tcp"},
			WaitingFor:   tcwait.ForListeningPort("6379/tcp").WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("redis container: %v", err)
	}
	t.Cleanup(func() { _ = rC.Terminate(ctx) })

	rHost, err := rC.Host(ctx)
	require.NoError(t, err)
	rPort, err := rC.MappedPort(ctx, "6379/tcp")
	require.NoError(t, err)
	redisURL = "redis://" + rHost + ":" + rPort.Port()

	// setupEcho()/config.Load() lesen die Umgebung — vor dem Bauen setzen.
	t.Setenv("VAKT_DB_URL", dsn)
	t.Setenv("VAKT_REDIS_URL", redisURL)
	t.Setenv("VAKT_SECRET_KEY", "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2")
	return dsn, redisURL
}

// Dieser Test schickt eine ABGELAUFENE Pro-Lizenz durch den echten Baum aus
// setupEcho() und prueft je Route namentlich, dass Lesen durchkommt und
// Schreiben nicht.
//
// Warum nicht als Unit-Test: Der Defekt (R1-17-01) war nie ein falscher
// Rueckgabewert, sondern eine falsche VERTEILUNG. features.Require und
// license.Require beantworteten dieselbe Frage verschieden, und 164 von 167
// Route-Gates hingen an der Variante ohne den Ablauf-Sonderfall. Ein Unit-Test
// auf einem einzelnen Gate ist beide Male gruen — er sagt "das Gate tut, was es
// tut", nie "an den Routen haengt das richtige Gate". Genau diese Unterscheidung
// steht schon in uuid_param_coverage_test.go, und sie war hier dieselbe.
//
// Wie die abgelaufene Lizenz auf den Request kommt — und warum ohne Signatur:
// Einen echten abgelaufenen Pro-Schluessel zu erzeugen hiesse, mit dem privaten
// Norvik-Schluessel zu signieren; der liegt nicht in dieser Umgebung, und der
// Vertrauensanker dafuer gehoert auch nicht in einen Test (in `package license`
// tauschen die Tests dafuer publicKeyPEM aus — von `package main` aus ist das
// unerreichbar, weil unexportiert). Der Test benutzt stattdessen den
// Redis-Cache-Pfad von license.DBMiddleware: Der Cache-Eintrag wird ohne
// Signaturpruefung deserialisiert (middleware.go, cacheToLicense), traegt das
// Feld `expired` und ist derselbe *License, den der Produktionspfad danach auf
// den Context legt. Der Test faelscht also keinen Schluessel, er setzt den
// Lizenz-ZUSTAND — und genau den nimmt die Middleware entgegen, erzeugen tut sie
// ihn nicht.
//
// Die Kopplung an die Cache-JSON-Form ist Absicht und faellt in die richtige
// Richtung: Aendert jemand die Feldnamen, deserialisiert der Eintrag zu einer
// leeren Lizenz, Phase B sieht ueberall 402 statt 200 und der Test wird ROT. Er
// kann nicht still auf "nichts gemessen" zurueckfallen.

// licenseCacheEntry ist die JSON-Form von license.licenseCache (unexportiert).
// Feldnamen aus internal/license/middleware.go.
type licenseCacheEntry struct {
	Tier      string     `json:"tier"`
	Features  []string   `json:"features"`
	OrgName   string     `json:"org_name"`
	IssuedAt  time.Time  `json:"issued_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	Demo      bool       `json:"demo"`
	Community bool       `json:"community"`
	Expired   bool       `json:"expired"`
}

var expiryParamSeg = regexp.MustCompile(`:[a-zA-Z_]+`)

// nilUUID ist wohlgeformt und existiert nicht. Wohlgeformt MUSS es sein:
// ValidateUUIDParams haengt als Gruppen-Middleware an `protected` und damit VOR
// dem Lizenz-Gate — ein kaputtes Segment wuerde mit 400 antworten, bevor das
// Gate ueberhaupt befragt wird, und der Test haette nichts gemessen.
// Nicht-existent MUSS es sein, damit ein UNgegateter Schreib-Handler, der
// tatsaechlich durchlaeuft, 0 Zeilen trifft.
const nilUUID = "00000000-0000-0000-0000-000000000000"

// notOrgScoped nennt die Praefixe, deren Lizenz NICHT aus license.DBMiddleware
// kommt und die deshalb von diesem Test gar nicht auf "abgelaufen" gestellt
// werden koennen.
//
//   - /scim/v2 haengt an `api` statt an `protected`
//     (cmd/api/routes.go: `scimSvc.Register(api.Group("/scim/v2"), …)`).
//   - /auth/* liegt vor der Anmeldung, es gibt noch keinen org_id-Kontext.
//   - /auditor/* haengt hinter auditor.AuditorAuth und antwortet ohne
//     Auditor-Sitzung immer 401 — mit einem Paseto-Admin-Token ist dort per
//     Konstruktion nichts messbar. Diese Routen werden nicht nur nicht
//     ausgewertet, sie werden gar nicht erst angefragt: Sie haengen an einem
//     IP-Rate-Limit von 30/min (routes.go:504), und das ist ein GETEILTER
//     Eimer. Ein Sweep, der ihn leerraeumt, laesst die vier Auditor-Routen im
//     NACHFOLGENDEN Test (TestUUIDParamGuardCoversEveryParameterisedRoute) mit
//     429 statt 401 antworten — 429 steht dort nicht in shortCircuitCodes, also
//     zaehlt dieser Test sie als "bewiesen", seine Liste schrumpft von 22 auf
//     18 Eintraege und er wird rot. Genau die Fehlerklasse, gegen die er
//     gebaut wurde, nur von aussen ausgeloest.
//
// Beide werden von der Instanz-Lizenz bedient (dem Env-Key bzw. dem adoptierten
// DB-Key), und die ist hier Community. Sie melden folgerichtig 402 mit
// `feature_not_available` — das ist kein Ablauf, sondern eine fehlende Lizenz.
//
// Eine echte Folge davon steht in der Commit-Message: Ist die INSTANZ-Lizenz
// abgelaufen, laesst der Fix zwar GET /auth/saml/initiate wieder durch, aber der
// POST-Callback bleibt 402 — ein reiner SSO-Kunde kaeme sich also weiterhin nicht
// anmelden und haette von der Leserechte-Zusage nichts. Das zu aendern ist eine
// Produktentscheidung (darf Authentifizierung ueberhaupt lizenzgegatet sein?) und
// bewusst nicht Teil dieses Fixes.
var notOrgScoped = []string{"/api/v1/scim/v2/", "/api/v1/auth/", "/api/v1/auditor/"}

func orgScopedLicence(path string) bool {
	for _, p := range notOrgScoped {
		if strings.HasPrefix(path+"/", p) || strings.HasPrefix(path, p) {
			return false
		}
	}
	return true
}

// freedReadsFloor ist der Nicht-Vakuitaets-Anker. Gemessen wurden 120 GET-Routen,
// die unter Community 402 antworten und unter abgelaufener Pro-Lizenz nicht mehr.
// Der Boden liegt bewusst darunter, damit eine einzelne neue oder entfernte
// Pro-Route den Lauf nicht rot faerbt — aber hoch genug, dass ein Fix, der nur
// eine Handvoll Routen erwischt, auffliegt.
const freedReadsFloor = 100

// blockedWritesFloor ist derselbe Anker fuer die Gegenrichtung: So viele
// Schreib-Routen muessen unter abgelaufener Lizenz weiterhin 402 melden.
const blockedWritesFloor = 20

func TestExpiredProLicenceKeepsReadsAndBlocksWritesOnTheRealTree(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	dbURL, redisURL := startExpiryStack(t)
	secret := "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err, "connect postgres")
	defer pool.Close()

	opt, err := redis.ParseURL(redisURL)
	require.NoError(t, err, "parse redis url")
	rdb := redis.NewClient(opt)
	defer func() { _ = rdb.Close() }()

	token, orgID := seedOrgAndTokenWithOrg(ctx, t, pool, secret)
	e, _ := setupEcho(ctx, testConfig())

	var current licenseCacheEntry

	// setLicence schreibt den Lizenz-Zustand in den Cache, den DBMiddleware liest.
	// TTL grosszuegig ueber der Laufzeit einer Phase; jede Phase setzt neu.
	setLicence := func(entry licenseCacheEntry) {
		current = entry
		b, err := json.Marshal(entry)
		require.NoError(t, err, "marshal licence cache entry")
		require.NoError(t, rdb.Set(ctx, "license:"+orgID, b, 10*time.Minute).Err(), "write licence cache")
	}
	defer func() { _ = rdb.Del(ctx, "license:"+orgID).Err() }()

	// reseed holt eine frische Org samt Token und stellt den Lizenz-Zustand
	// darauf wieder her.
	//
	// Notwendig, weil der Schreib-Sweep sich selbst abschiesst: Unter den
	// UNgegateten Schreib-Routen laufen echte Handler, und mindestens einer
	// erhoeht users.pw_version. Ab da antwortet AuthMiddleware auf JEDEN
	// weiteren Request mit 401 AUTH_SESSION_INVALIDATED — im vorigen Lauf hat
	// das 33 Routen als "durchgekommen" gemeldet, die in Wahrheit nie an der
	// Anmeldung vorbeikamen. Ein 401 ist keine Aussage ueber die Lizenz.
	reseed := func() {
		_ = rdb.Del(ctx, "license:"+orgID).Err()
		token, orgID = seedOrgAndTokenWithOrg(ctx, t, pool, secret)
		setLicence(current)
	}

	past := time.Now().Add(-90 * 24 * time.Hour)
	community := licenseCacheEntry{Tier: "community", Features: []string{}, IssuedAt: past, Community: true}
	expiredPro := licenseCacheEntry{
		Tier:      "pro",
		Features:  license.AllFeatures(),
		OrgName:   "Abgelaufene GmbH",
		IssuedAt:  past.Add(-365 * 24 * time.Hour),
		ExpiresAt: &past,
		Expired:   true,
	}
	validPro := licenseCacheEntry{
		Tier:     "pro",
		Features: license.AllFeatures(),
		OrgName:  "Zahlende GmbH",
		IssuedAt: past,
	}

	// Das Org-Rate-Limit (300/min, sharedmw.OrgRateLimitRedis) haengt an
	// `protected` und damit VOR dem Lizenz-Gate. Ein Sweep ueber alle Routen
	// reisst es zwangslaeufig, und ein 429 sagt ueber die Lizenz nichts — im
	// ersten Lauf dieses Tests hat es 35 Schreib-Routen als "durchgekommen"
	// erscheinen lassen, die das Gate nie erreicht hatten. Der Zaehler wird
	// deshalb vor jedem Request zurueckgesetzt: Das Rate-Limit ist nicht der
	// Gegenstand dieser Messung, es steht nur davor.
	// testClientIP ist die RemoteAddr, die httptest jedem Request gibt; alle
	// IP-Limiter (routes.go, "rate:<prefix>:<ip>") buchen darauf.
	const testClientIP = "192.0.2.1"

	clearRateLimits := func() {
		bucket := time.Now().UTC().Format("200601021504")
		_ = rdb.Del(ctx, fmt.Sprintf("vakt:ratelimit:org:%s:%s", orgID, bucket)).Err()
		iter := rdb.Scan(ctx, 0, "rate:*:"+testClientIP, 100).Iterator()
		for iter.Next(ctx) {
			_ = rdb.Del(ctx, iter.Val()).Err()
		}
	}

	// Aufraeumen ist Pflicht, nicht Hoeflichkeit: Die Limiter-Eimer haengen an
	// der IP, und alle Tests dieses Pakets teilen sich dieselbe httptest-IP und
	// dieselbe Redis-Instanz. Ein Sweep, der verbrauchtes Budget stehen laesst,
	// laesst den naechsten Test etwas anderes messen als er glaubt.
	t.Cleanup(clearRateLimits)

	once := func(method, routePath string, withCSRF bool) (int, string) {
		reqPath := expiryParamSeg.ReplaceAllString(routePath, nilUUID)
		req := httptest.NewRequest(method, reqPath, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		if withCSRF {
			// Double-submit: ohne passendes Cookie/Header-Paar antwortet
			// CSRFMiddleware 403, bevor das Lizenz-Gate je befragt wird.
			const csrf = "licence-expiry-test-csrf"
			req.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: csrf})
			req.Header.Set(auth.CSRFHeaderName, csrf)
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec.Code, rec.Body.String()
	}

	probe := func(method, routePath string, withCSRF bool) (int, string) {
		code, body := once(method, routePath, withCSRF)
		if code == http.StatusUnauthorized && strings.Contains(body, "AUTH_SESSION_INVALIDATED") {
			reseed()
			code, body = once(method, routePath, withCSRF)
		}
		if code == http.StatusTooManyRequests {
			clearRateLimits()
			code, body = once(method, routePath, withCSRF)
		}
		return code, body
	}

	type routeKey struct{ method, path string }
	var reads, writes []routeKey
	for _, r := range e.Routes() {
		if strings.Contains(r.Name, "glob..func") && strings.HasSuffix(r.Path, "/*") {
			continue // Echos eigener 404/405-Fallback, keine echte Route
		}
		switch r.Method {
		case http.MethodGet, http.MethodHead:
			reads = append(reads, routeKey{r.Method, r.Path})
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			writes = append(writes, routeKey{r.Method, r.Path})
		}
	}
	require.NotEmpty(t, reads, "no GET routes on the tree — this gate has lost its subject")

	setLicence(community)
	// ── Phase A: Community ────────────────────────────────────────────────────
	// Der Nenner. Was hier 402 meldet, ist Pro-gegatet; alles andere sagt ueber
	// den Fix nichts aus.
	setLicence(community)
	communityReads := map[string]int{}
	for _, r := range reads {
		code, _ := probe(r.method, r.path, false)
		communityReads[r.method+" "+r.path] = code
	}
	// Fuer die Schreib-Routen gibt es bewusst KEINEN Community-Durchlauf. Er
	// wuerde jede ungegatete Schreib-Route ein zweites Mal ausfuehren, ohne eine
	// Frage zu beantworten, die nicht anderswo schaerfer beantwortet waere: Dass
	// der Ablauf-Sonderfall Schreibzugriff niemals oeffnet, ist eine Eigenschaft
	// von license.Allows (nur GET/HEAD erreichen den Zweig) und wird in
	// features/flags_test.go ueber die volle Matrix aus 8 Lizenzzustaenden x 6
	// Methoden geprueft. Auf dem echten Baum wird hier die ANDERE Haelfte
	// gemessen, die kein Unit-Test sehen kann: dass an den Schreib-Routen
	// tatsaechlich das Gate haengt und mit dem richtigen Koerper antwortet.

	// ── Phase B: abgelaufene Pro-Lizenz ───────────────────────────────────────
	setLicence(expiredPro)
	var freed, stillBlocked, unprovenReads []string
	var regressed []string
	for _, r := range reads {
		key := r.method + " " + r.path
		if !orgScopedLicence(r.path) {
			// SCIM/auth: die Lizenz auf dem Context ist die Instanz-Lizenz, die
			// dieser Test nicht auf "abgelaufen" stellen kann. Ueber den
			// Ablauf-Sonderfall ist hier per Konstruktion nichts messbar — also
			// wird auch nichts behauptet.
			if communityReads[key] == http.StatusPaymentRequired {
				unprovenReads = append(unprovenReads, key)
			}
			continue
		}
		code, body := probe(r.method, r.path, false)
		switch {
		case communityReads[key] == http.StatusPaymentRequired && code != http.StatusPaymentRequired:
			freed = append(freed, key)
		case communityReads[key] == http.StatusPaymentRequired:
			stillBlocked = append(stillBlocked, key)
		case code == http.StatusPaymentRequired:
			regressed = append(regressed, fmt.Sprintf("%s (community %d → expired 402, body %s)",
				key, communityReads[key], strings.TrimSpace(body)))
		}
	}

	var blockedWrites, wrongBody, unprovenWrites []string
	for _, r := range writes {
		key := r.method + " " + r.path
		if !orgScopedLicence(r.path) {
			// SCIM/auth: die Lizenz auf dem Context ist die Instanz-Lizenz, die
			// dieser Test nicht auf "abgelaufen" stellen kann. Ein 402 hier ist
			// eine fehlende Lizenz, kein abgelaufener Schluessel.
			unprovenWrites = append(unprovenWrites, key+" (Instanz-Lizenz)")
			continue
		}
		code, body := probe(r.method, r.path, true)
		if code == http.StatusTooManyRequests {
			unprovenWrites = append(unprovenWrites, key+" (429)")
			continue
		}
		if code != http.StatusPaymentRequired {
			continue // nicht lizenzgegatet — sagt ueber den Fix nichts
		}
		blockedWrites = append(blockedWrites, key)
		if !strings.Contains(body, "license_expired") {
			wrongBody = append(wrongBody, key+" → "+strings.TrimSpace(body))
		}
	}

	// ── Phase C: gueltige Pro-Lizenz ──────────────────────────────────────────
	setLicence(validPro)
	var validProBlocked []string
	for _, r := range reads {
		key := r.method + " " + r.path
		// Nur die unter Community gegateten Routen: Auf einer ungegateten Route
		// kann eine gueltige Pro-Lizenz per Konstruktion kein 402 erzeugen, die
		// Anfrage wuerde nichts pruefen und nur Laufzeit und geteiltes
		// Rate-Limit-Budget kosten.
		if !orgScopedLicence(r.path) || communityReads[key] != http.StatusPaymentRequired {
			continue
		}
		if code, _ := probe(r.method, r.path, false); code == http.StatusPaymentRequired {
			validProBlocked = append(validProBlocked, key)
		}
	}

	sort.Strings(freed)
	sort.Strings(stillBlocked)
	t.Logf("reads=%d writes=%d | community-gated reads=%d → freed=%d, still blocked=%d | "+
		"community-gated writes still blocked on expiry=%d",
		len(reads), len(writes), len(freed)+len(stillBlocked), len(freed), len(stillBlocked), len(blockedWrites))
	for _, k := range freed {
		t.Logf("  read freed   %s", k)
	}
	sort.Strings(unprovenReads)
	for _, k := range unprovenReads {
		t.Logf("  read unproven %s  (Instanz-Lizenz, nicht auf abgelaufen stellbar)", k)
	}
	sort.Strings(blockedWrites)
	sort.Strings(unprovenWrites)
	for _, k := range blockedWrites {
		t.Logf("  write blocked %s  (402 license_expired)", k)
	}
	for _, k := range unprovenWrites {
		t.Logf("  write unproven %s", k)
	}

	// (1) Lesen: jede Pro-gegatete GET-Route muss aufgehen, ausser den begruendeten.
	assert.Empty(t, stillBlocked,
		"diese org-gescopten GET-Routen melden auch mit abgelaufener Pro-Lizenz 402 — der Kunde kommt "+
			"an seine eigenen Daten nicht heran, obwohl license.License.Expired ihm genau das zusichert "+
			"(R1-17-01): %v", stillBlocked)

	// (2) Nicht-Vakuitaet: der Fix muss viele Routen betreffen, nicht drei.
	assert.GreaterOrEqual(t, len(freed), freedReadsFloor,
		"nur %d GET-Routen sind durch den Fix wieder lesbar — erwartet >= %d. Entweder greift das "+
			"zusammengefuehrte Gate nicht mehr ueberall, oder dieser Test misst den falschen Baum",
		len(freed), freedReadsFloor)

	// (3) Kein Weg in die andere Richtung: was unter Community offen war, bleibt offen.
	assert.Empty(t, regressed,
		"Routen, die unter Community durchkamen, melden mit abgelaufener Pro-Lizenz 402 — der "+
			"Ablauf-Sonderfall darf niemandem etwas WEGNEHMEN: %v", regressed)

	// (4) Schreiben bleibt zu — das ist die Haelfte, die die Lizenz verkauft.
	assert.GreaterOrEqual(t, len(blockedWrites), blockedWritesFloor,
		"nur %d gegatete Schreib-Routen wurden ueberhaupt bis zum Lizenz-Gate gemessen (erwartet >= %d) — "+
			"vermutlich antwortet eine fruehere Middleware (CSRF, MFA, Rate-Limit) und dieser Teil des "+
			"Tests prueft nichts", len(blockedWrites), blockedWritesFloor)
	assert.Empty(t, wrongBody,
		"abgelaufene Lizenz, Schreibzugriff: der Koerper muss license_expired tragen, nicht das "+
			"generische feature_not_available — der Kunde hat bezahlt und braucht den Verlaengerungs-Hinweis: %v",
		wrongBody)

	// (5) Baseline gueltige Pro-Lizenz: unveraendert offen.
	assert.Empty(t, validProBlocked,
		"eine GUELTIGE Pro-Lizenz bekommt 402 auf Leseruten: %v", validProBlocked)

	// (6) Baseline kostenlose Edition: weiterhin gegatet. Ohne diese Zusicherung
	//     koennte der Fix die Pro-Features schlicht verschenkt haben.
	communityGatedReads := 0
	for _, code := range communityReads {
		if code == http.StatusPaymentRequired {
			communityGatedReads++
		}
	}
	assert.GreaterOrEqual(t, communityGatedReads, freedReadsFloor,
		"die kostenlose Edition sieht nur noch %d gegatete GET-Routen (erwartet >= %d) — der "+
			"Ablauf-Sonderfall darf Pro-Features nicht in die Community-Edition durchreichen",
		communityGatedReads, freedReadsFloor)
}

// seedOrgAndTokenWithOrg ist seedOrgAndToken (openapi_contract_auth_test.go) plus
// der Org-ID: Der Lizenz-Cache-Schluessel ist pro Org, ohne die ID kommt der
// Lizenz-Zustand nicht an den Request.
func seedOrgAndTokenWithOrg(ctx context.Context, t *testing.T, pool *pgxpool.Pool, secret string) (string, string) {
	t.Helper()

	var orgID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ('Licence Expiry Test', 'licence-expiry-'||substr(md5(random()::text),1,8))
		 RETURNING id::text`).Scan(&orgID), "seed org")

	var userID string
	var pwVersion int64
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email) VALUES ('licence-expiry-'||substr(md5(random()::text),1,8)||'@test.local')
		 RETURNING id::text, pw_version`).Scan(&userID, &pwVersion), "seed user")

	_, err := pool.Exec(ctx,
		`INSERT INTO org_members (org_id, user_id, role_id)
		 SELECT $1::uuid, $2::uuid, id FROM roles WHERE name = 'Admin' LIMIT 1`,
		orgID, userID)
	require.NoError(t, err, "seed membership")

	raw, err := hex.DecodeString(secret)
	require.NoError(t, err, "decode master key")
	keyBytes, err := sharedcrypto.DeriveServiceKey(raw, "vakt-paseto-v1")
	require.NoError(t, err, "derive paseto key")
	key, err := auth.GenerateSymmetricKeyFromBytes(keyBytes)
	require.NoError(t, err, "paseto key")

	token, err := auth.IssueAccessToken(key, auth.Claims{
		UserID:    userID,
		OrgID:     orgID,
		Roles:     []string{"Admin"},
		PwVersion: pwVersion,
		MFA:       true,
	})
	require.NoError(t, err, "issue token")
	return token, orgID
}
