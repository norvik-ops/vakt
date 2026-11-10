// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"

	"github.com/crewjam/saml"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/config"
)

// R1-SEC-01 — Kontoübernahme über die Org-Grenze per Direct-SAML.
//
// Die SAML-Bibliothek prüft den Namespace der NameID nicht (GO-2026-6909). Der
// Direct-SAML-Login suchte das Konto aber zuerst per
// `WHERE oidc_subject = NameID` — ohne Provider und ohne Org. Ein IdP, den eine
// Organisation B für sich eingerichtet hat, konnte damit eine NameID senden,
// die dem Subject eines Kontos aus Organisation A entspricht, und war als
// dieses Konto angemeldet. Derselbe Weg galt für den E-Mail-Abgleich: E-Mail
// eines Kontos aus A im Attribut, und das Konto wurde verknüpft.
//
// Diese Tests treiben den echten Login-Abschluss (completeSAMLLogin) mit einer
// handgebauten, als „geprüft" geltenden Assertion gegen eine echte Datenbank.

var samlTakeoverSeq atomic.Int64

func samlTakeoverDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dbURL := os.Getenv("VAKT_DB_URL")
	if dbURL == "" {
		t.Skip("VAKT_DB_URL not set — der Direct-SAML-Übernahmetest braucht eine migrierte Postgres")
	}
	pool, err := pgxpool.New(context.Background(), dbURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func samlTakeoverTag() string {
	return fmt.Sprintf("%d-%d", os.Getpid(), samlTakeoverSeq.Add(1))
}

func seedSAMLOrg(t *testing.T, pool *pgxpool.Pool, tag string) string {
	t.Helper()
	var orgID string
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO organizations (name, slug) VALUES ($1, $1) RETURNING id::text`,
		"samltakeover-"+tag,
	).Scan(&orgID))
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM organizations WHERE id = $1::uuid`, orgID)
	})
	return orgID
}

// seedSSOUser legt ein bestehendes SSO-Konto mit Subject/Provider in orgID an.
func seedSSOUser(t *testing.T, pool *pgxpool.Pool, orgID, email, subject, provider string) string {
	t.Helper()
	ctx := context.Background()
	var userID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO users (email, display_name, oidc_subject, oidc_provider, is_active, role)
		VALUES ($1, $1, $2, $3, TRUE, 'viewer') RETURNING id::text`,
		email, subject, provider,
	).Scan(&userID))
	_, err := pool.Exec(ctx, `
		INSERT INTO org_members (org_id, user_id, role_id)
		VALUES ($1::uuid, $2::uuid, (SELECT id FROM roles WHERE name = 'Admin'))`,
		orgID, userID)
	require.NoError(t, err)
	dropUserByEmail(t, pool, email)
	return userID
}

func dropUserByEmail(t *testing.T, pool *pgxpool.Pool, email string) {
	t.Helper()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE email = $1`, email)
	})
}

func samlAssertion(nameID, email string) *saml.Assertion {
	a := &saml.Assertion{}
	if nameID != "" {
		a.Subject = &saml.Subject{NameID: &saml.NameID{Value: nameID}}
	}
	if email != "" {
		a.AttributeStatements = []saml.AttributeStatement{{Attributes: []saml.Attribute{
			{Name: "email", Values: []saml.AttributeValue{{Value: email}}},
		}}}
	}
	return a
}

// runSAMLLogin schließt einen Direct-SAML-Login für orgID ab und gibt Status
// und (bei 200) die angemeldete Nutzer-ID zurück. Ein Panic wird zum Testfehler.
func runSAMLLogin(t *testing.T, pool *pgxpool.Pool, orgID string, a *saml.Assertion) (code int, location, userID string) {
	t.Helper()
	svc := NewService(pool, nil, mustInternalKey(t))
	h := NewHandler(svc, &config.Config{})
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/saml/acs", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("completeSAMLLogin panicked: %v", r)
			}
		}()
		require.NoError(t, h.completeSAMLLogin(c, orgID, true, a))
	}()

	if rec.Code == http.StatusOK {
		var resp AuthResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		// resp.User ist auf diesem Weg leer; wer angemeldet ist, steht in der
		// Session-Zeile, die der Token-Ausgabe gehört.
		require.NotEmpty(t, resp.SessionID, "Login 200 ohne Session — Nutzer nicht bestimmbar")
		// orgid-lint: global — Testabfrage, per Primärschlüssel auf die eben ausgegebene Session eingegrenzt.
		require.NoError(t, pool.QueryRow(context.Background(),
			`SELECT user_id::text FROM refresh_sessions WHERE id = $1::uuid`, resp.SessionID,
		).Scan(&userID))
	}
	return rec.Code, rec.Header().Get("Location"), userID
}

func mustInternalKey(t *testing.T) SymmetricKey {
	t.Helper()
	key, err := GenerateSymmetricKey("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
	require.NoError(t, err)
	return key
}

func TestDirectSAML_NameIDOfForeignAccountDoesNotLogIn(t *testing.T) {
	pool := samlTakeoverDB(t)
	for _, provider := range []string{"keycloak", "saml"} {
		t.Run("Opfer mit Provider "+provider, func(t *testing.T) {
			tag := samlTakeoverTag()
			orgA := seedSAMLOrg(t, pool, "a-"+tag)
			orgB := seedSAMLOrg(t, pool, "b-"+tag)
			victimSub := "victim-sub-" + tag
			victim := seedSSOUser(t, pool, orgA, "victim-"+tag+"@a.example", victimSub, provider)

			attackerEmail := "attacker-" + tag + "@b.example"
			dropUserByEmail(t, pool, attackerEmail)
			code, _, got := runSAMLLogin(t, pool, orgB, samlAssertion(victimSub, attackerEmail))

			require.NotEqual(t, victim, got,
				"IdP von Org B hat per NameID das Konto aus Org A übernommen (status %d)", code)
			// Ein eigenes, getrenntes Konto in Org B ist richtig (H1: kein Aussperren);
			// das Konto aus Org A bleibt unberührt.
			require.Equal(t, http.StatusOK, code)
			var subj, prov string
			require.NoError(t, pool.QueryRow(context.Background(),
				`SELECT oidc_subject, oidc_provider FROM users WHERE id = $1::uuid`, victim,
			).Scan(&subj, &prov))
			require.Equal(t, victimSub, subj, "Org B hat das Subject des Kontos aus Org A umgeschrieben")
			require.Equal(t, provider, prov)
		})
	}
}

func TestDirectSAML_EmailOfForeignOrgAccountDoesNotLogIn(t *testing.T) {
	pool := samlTakeoverDB(t)
	tag := samlTakeoverTag()
	orgA := seedSAMLOrg(t, pool, "a-"+tag)
	orgB := seedSAMLOrg(t, pool, "b-"+tag)
	victimEmail := "victim-mail-" + tag + "@a.example"
	victim := seedSSOUser(t, pool, orgA, victimEmail, "victim-mail-sub-"+tag, "keycloak")

	code, _, got := runSAMLLogin(t, pool, orgB, samlAssertion("fresh-nameid-"+tag, victimEmail))
	require.NotEqual(t, victim, got,
		"IdP von Org B hat per E-Mail-Attribut das Konto aus Org A übernommen (status %d)", code)
	require.Equal(t, http.StatusFound, code, "Login mit der E-Mail eines fremden Org-Kontos muss abgelehnt werden")
}

// Gegenprobe: Ohne sie bliebe eine Implementierung grün, die jeden SAML-Login
// ablehnt. Das eigene Konto in der eigenen Org muss weiter funktionieren.
func TestDirectSAML_OwnOrgAccountStillLogsIn(t *testing.T) {
	pool := samlTakeoverDB(t)
	tag := samlTakeoverTag()
	orgB := seedSAMLOrg(t, pool, "b-"+tag)
	sub := "own-sub-" + tag
	own := seedSSOUser(t, pool, orgB, "own-"+tag+"@b.example", sub, "saml")

	code, loc, got := runSAMLLogin(t, pool, orgB, samlAssertion(sub, "own-"+tag+"@b.example"))
	require.Equal(t, http.StatusOK, code, "Redirect: %s", loc)
	require.Equal(t, own, got)
}

func TestDirectSAML_MissingSubjectDoesNotPanic(t *testing.T) {
	pool := samlTakeoverDB(t)
	tag := samlTakeoverTag()
	orgB := seedSAMLOrg(t, pool, "b-"+tag)
	email := "nosubject-" + tag + "@b.example"
	dropUserByEmail(t, pool, email)

	code, loc, _ := runSAMLLogin(t, pool, orgB, samlAssertion("", email))
	require.Equal(t, http.StatusFound, code)
	require.Equal(t, "/login?error=saml_assertion_invalid", loc)
}

// H1 (Subreview 2): Org B darf ein Subject nicht für Org A „besetzen". Vor dem
// org-qualifizierten Subject lehnte der Login von Org A ab, sobald irgendein
// Konto die NameID schon trug — Org B konnte so jeden künftigen Nutzer von A
// per Direct-SAML aussperren.
func TestDirectSAML_ForeignOrgCannotLockOutSubject(t *testing.T) {
	pool := samlTakeoverDB(t)
	tag := samlTakeoverTag()
	orgA := seedSAMLOrg(t, pool, "a-"+tag)
	orgB := seedSAMLOrg(t, pool, "b-"+tag)
	nameID := "shared-nameid-" + tag
	bEmail, aEmail := "squat-"+tag+"@b.example", "legit-"+tag+"@a.example"
	dropUserByEmail(t, pool, bEmail)
	dropUserByEmail(t, pool, aEmail)

	code, loc, squatter := runSAMLLogin(t, pool, orgB, samlAssertion(nameID, bEmail))
	require.Equal(t, http.StatusOK, code, "Org B legt ihr eigenes Konto an (Redirect %s)", loc)

	code, loc, legit := runSAMLLogin(t, pool, orgA, samlAssertion(nameID, aEmail))
	require.Equal(t, http.StatusOK, code, "Nutzer von Org A ist ausgesperrt (Redirect %s)", loc)
	require.NotEqual(t, squatter, legit, "Org A darf nicht im Konto von Org B landen")

	// Und beide bleiben getrennt anmeldbar.
	_, _, again := runSAMLLogin(t, pool, orgB, samlAssertion(nameID, bEmail))
	require.Equal(t, squatter, again)
}

// H1, E-Mail-Hälfte — dokumentiert eine Grenze, keinen Fix: users.email ist
// instanzweit eindeutig (001_core_schema). Hat Org B ein Konto mit dieser
// E-Mail, kann Org A es weder übernehmen noch ein zweites anlegen. Gepinnt wird,
// dass das als generischer Redirect endet — kein 500, kein fremdes Konto.
func TestDirectSAML_EmailTakenInForeignOrgFailsGenerically(t *testing.T) {
	pool := samlTakeoverDB(t)
	tag := samlTakeoverTag()
	orgA := seedSAMLOrg(t, pool, "a-"+tag)
	orgB := seedSAMLOrg(t, pool, "b-"+tag)
	email := "contested-" + tag + "@example.com"
	dropUserByEmail(t, pool, email)

	code, _, bUser := runSAMLLogin(t, pool, orgB, samlAssertion("b-nameid-"+tag, email))
	require.Equal(t, http.StatusOK, code)

	code, loc, got := runSAMLLogin(t, pool, orgA, samlAssertion("a-nameid-"+tag, email))
	require.NotEqual(t, bUser, got)
	require.Equal(t, http.StatusFound, code)
	require.Equal(t, "/login?error=saml_provision_failed", loc)
}

// H2 (Subreview 2): Ein Direct-SAML-IdP wählt die NameID frei. Wählt er das
// Casdoor-Subject eines Nutzers, der sich noch nie angemeldet hat, darf dessen
// späterer Casdoor-Login (targetOrgID "") nicht im Konto des IdP landen.
func TestCasdoorLogin_DoesNotLandInDirectSAMLAccount(t *testing.T) {
	pool := samlTakeoverDB(t)
	tag := samlTakeoverTag()
	orgB := seedSAMLOrg(t, pool, "b-"+tag)
	casdoorSub := "casdoor-sub-" + tag
	attackerEmail, victimEmail := "plant-"+tag+"@b.example", "casdoor-victim-"+tag+"@example.com"
	dropUserByEmail(t, pool, attackerEmail)
	dropUserByEmail(t, pool, victimEmail)

	code, loc, planted := runSAMLLogin(t, pool, orgB, samlAssertion(casdoorSub, attackerEmail))
	require.Equal(t, http.StatusOK, code, "Redirect %s", loc)

	svc := NewService(pool, nil, mustInternalKey(t))
	got, _, _, err := svc.provisionOIDCUser(context.Background(), casdoorSub, "keycloak", victimEmail, "Victim", "", "", true, true)
	require.NoError(t, err)
	require.NotEqual(t, planted, got, "Casdoor-Login ist im per Direct-SAML angelegten Konto gelandet")
}

// Subreview 3: Konten, die der alte Code per Direct-SAML anlegte, tragen die
// rohe NameID mit Provider "saml" — dieselbe Form wie Casdoor-SAML-Konten. Ohne
// Datenänderung trifft ein Casdoor-Login mit gleichem Subject sie weiter.
// Migration 270 schreibt sie auf die org-qualifizierte Form um; dieser Test
// fährt sie auf geseedeten Altzeilen.
func TestLegacyDirectSAMLRow_MigratedAndNotHitByCasdoor(t *testing.T) {
	pool := samlTakeoverDB(t)
	ctx := context.Background()
	up := readMigration(t, "270_direct_saml_subject_org_qualified.up.sql")
	down := readMigration(t, "270_direct_saml_subject_org_qualified.down.sql")

	for _, marker := range []string{"org_saml_configs", "login_history"} {
		t.Run("Altzeile erkannt über "+marker, func(t *testing.T) {
			tag := samlTakeoverTag()
			orgB := seedSAMLOrg(t, pool, "b-"+tag)
			orgC := seedSAMLOrg(t, pool, "c-"+tag)
			nameID := "legacy-nameid-" + tag
			legacyEmail := "legacy-" + tag + "@b.example"
			legacy := seedSSOUser(t, pool, orgB, legacyEmail, nameID, "saml")
			switch marker {
			case "org_saml_configs":
				_, err := pool.Exec(ctx, `
					INSERT INTO org_saml_configs (org_id, entity_id, acs_url, idp_metadata, cert_pem, key_pem)
					VALUES ($1::uuid, 'e', 'a', 'm', 'c', '\x00')`, orgB)
				require.NoError(t, err)
			case "login_history":
				// orgid-lint: global — Testdaten, org_id wird gesetzt.
				_, err := pool.Exec(ctx, `
					INSERT INTO login_history (org_id, user_id, source, result)
					VALUES ($1::uuid, $2::uuid, 'saml_direct', 'ok')`, orgB, legacy)
				require.NoError(t, err)
			}
			// Kontrolle: ein Casdoor-SAML-Konto einer Org ohne Direct-SAML bleibt, wie es ist.
			casdoorSub := "casdoor-saml-sub-" + tag
			controlEmail := "control-" + tag + "@c.example"
			control := seedSSOUser(t, pool, orgC, controlEmail, casdoorSub, "saml")

			_, err := pool.Exec(ctx, up)
			require.NoError(t, err)

			svc := NewService(pool, nil, mustInternalKey(t))
			strangerEmail := "stranger-" + tag + "@example.com"
			dropUserByEmail(t, pool, strangerEmail)
			got, _, _, err := svc.provisionOIDCUser(ctx, nameID, "keycloak", strangerEmail, "S", "", "", true, true)
			require.NoError(t, err)
			require.NotEqual(t, legacy, got, "Casdoor-Login hat die Direct-SAML-Altzeile getroffen")

			code, loc, again := runSAMLLogin(t, pool, orgB, samlAssertion(nameID, legacyEmail))
			require.Equal(t, http.StatusOK, code, "Redirect %s", loc)
			require.Equal(t, legacy, again, "der Direct-SAML-Nutzer der Altzeile muss sein Konto behalten")

			ctrl, _, _, err := svc.provisionOIDCUser(ctx, casdoorSub, "saml", controlEmail, "C", "", "", true, true)
			require.NoError(t, err)
			require.Equal(t, control, ctrl, "Casdoor-SAML-Konto ohne Direct-SAML-Bezug wurde fälschlich umgeschrieben")

			// Umkehrbar: down stellt die alte Form her, up ist danach wieder anwendbar.
			_, err = pool.Exec(ctx, down)
			require.NoError(t, err)
			var subj, prov string
			require.NoError(t, pool.QueryRow(ctx,
				`SELECT oidc_subject, oidc_provider FROM users WHERE id = $1::uuid`, legacy).Scan(&subj, &prov))
			require.Equal(t, nameID, subj)
			require.Equal(t, "saml", prov)
			_, err = pool.Exec(ctx, up)
			require.NoError(t, err)
		})
	}
}

func readMigration(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("../../db/migrations/" + name)
	require.NoError(t, err)
	return string(b)
}
