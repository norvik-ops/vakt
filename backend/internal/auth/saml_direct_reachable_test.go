// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/auth"
	"github.com/matharnica/vakt/internal/config"
	"github.com/matharnica/vakt/internal/license"
	sharedcrypto "github.com/matharnica/vakt/internal/shared/crypto"
)

// R1-10-V04 / R1-21-A03 / R1-07-B03 — the direct SAML SP path was unreachable.
//
// Three findings, one defect chain, two independent halves:
//
//  1. Handler.WithDB ("required for direct SAML SP") had NO caller. h.db stayed
//     nil, LoadOrgSAMLConfig answers "unconfigured" for a nil pool, so every
//     request fell through to the Casdoor proxy no matter what stood in
//     org_saml_configs.
//  2. The three SAML SP handlers read c.Get("org_id"). They are mounted public —
//     and they have to be, an IdP POSTs the assertion without a Vakt session —
//     so that value is never set. GET /auth/saml/initiate answered 400
//     AUTH_SAML_NO_ORG to everyone; measured live in R1-07-B03.
//
// The test drives the REAL registration (auth.Register), so it also fails if
// the routes move somewhere the org can no longer be resolved.

const samlTestMasterKey = "d7463ee089bc65fac0efe91ee13b88413e256de2151228eeebee4787e5d276f7"

func TestSAMLInitiate_reachesTheOrgSAMLConfig(t *testing.T) {
	pool := authTestDB(t)

	orgID := seedInstanceOrg(t, pool)
	seedDirectSAMLConfig(t, pool, orgID)

	e, _ := mountPublicAuthRoutes(t, pool)

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/auth/saml/initiate", nil))

	require.Equal(t, http.StatusOK, rec.Code,
		"the direct SAML SP path is unreachable: %s", rec.Body.String())

	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Contains(t, body["redirect_url"], "https://idp.example.test/sso",
		"the AuthnRequest must be addressed to the IdP from the org's own SAML config")

	// The replay binding from ADR-0036 has to come with it: the request ID is
	// signed into a single-use cookie and matched against InResponseTo in the ACS.
	var found bool
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == "saml_req_id" {
			found = true
			assert.True(t, ck.HttpOnly)
		}
	}
	assert.True(t, found, "no saml_req_id cookie — the ACS would accept any signed assertion (ADR-0036)")
}

// The SP metadata a customer hands to their IdP must be the SP's own, built
// from org_saml_configs — not the Casdoor fallback, which answers a JSON error
// under an application/xml content type (R1-07-B04).
func TestSAMLMetadata_servesTheDirectSPDescriptor(t *testing.T) {
	pool := authTestDB(t)

	orgID := seedInstanceOrg(t, pool)
	seedDirectSAMLConfig(t, pool, orgID)

	e, _ := mountPublicAuthRoutes(t, pool)

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/auth/saml/metadata", nil))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/xml")
	assert.Contains(t, rec.Body.String(), "EntityDescriptor",
		"the response is not SP metadata — the direct SP path was skipped")
	assert.Contains(t, rec.Body.String(), "urn:vakt:test:sp",
		"the metadata must carry the entity ID from the org's own SAML config")
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// mountPublicAuthRoutes registers the real auth routes on a public group, the
// way cmd/api does, with a Pro license in the context so the SAML feature gate
// lets the request through.
func mountPublicAuthRoutes(t *testing.T, pool *pgxpool.Pool) (*echo.Echo, *auth.Handler) {
	t.Helper()
	cfg := &config.Config{SecretKey: samlTestMasterKey, FrontendURL: "http://localhost:5173"}
	h := auth.NewHandler(auth.NewService(pool, nil, mustKey(t)), cfg)

	e := echo.New()
	g := e.Group("/api/v1/auth", func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("license", &license.License{Tier: "pro", Features: []string{"saml_auth", "sso"}})
			return next(c)
		}
	})
	auth.Register(g, h)
	return e, h
}

// seedDirectSAMLConfig writes an enabled org_saml_configs row with a real SP
// keypair and a minimal IdP descriptor, in the encrypted form the loader expects.
func seedDirectSAMLConfig(t *testing.T, pool *pgxpool.Pool, orgID string) {
	t.Helper()
	ctx := context.Background()

	certPEM, keyPEM, err := auth.GenerateSAMLCert(orgID)
	require.NoError(t, err)

	master, err := hex.DecodeString(samlTestMasterKey)
	require.NoError(t, err)
	samlKey, err := sharedcrypto.DeriveServiceKey(master, "vakt-saml-v1")
	require.NoError(t, err)
	keyEnc, err := sharedcrypto.Encrypt(samlKey, []byte(keyPEM))
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO org_saml_configs (org_id, entity_id, acs_url, idp_metadata, cert_pem, key_pem, enabled, jit_provisioning)
		VALUES ($1::uuid, 'urn:vakt:test:sp', 'https://vakt.example.test/api/v1/auth/saml/acs', $2, $3, $4, TRUE, TRUE)
		ON CONFLICT (org_id) DO UPDATE SET
			entity_id = EXCLUDED.entity_id, acs_url = EXCLUDED.acs_url,
			idp_metadata = EXCLUDED.idp_metadata, cert_pem = EXCLUDED.cert_pem,
			key_pem = EXCLUDED.key_pem, enabled = TRUE, jit_provisioning = TRUE`,
		orgID, idpMetadataXML(t), certPEM, keyEnc)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM org_saml_configs WHERE org_id = $1::uuid`, orgID)
	})
}

// idpMetadataXML is a minimal but valid IdP EntityDescriptor: one
// IDPSSODescriptor with a redirect-binding SingleSignOnService and a signing
// certificate, which is all crewjam/saml needs to build an AuthnRequest.
func idpMetadataXML(t *testing.T) string {
	t.Helper()
	certPEM, _, err := auth.GenerateSAMLCert("idp")
	require.NoError(t, err)
	// Strip the PEM armour — X509Certificate carries bare base64.
	var b64 string
	for _, line := range splitLines(certPEM) {
		if line == "" || line[0] == '-' {
			continue
		}
		b64 += line
	}
	return fmt.Sprintf(`<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="https://idp.example.test/metadata">
  <IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <KeyDescriptor use="signing">
      <KeyInfo xmlns="http://www.w3.org/2000/09/xmldsig#">
        <X509Data><X509Certificate>%s</X509Certificate></X509Data>
      </KeyInfo>
    </KeyDescriptor>
    <SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://idp.example.test/sso"/>
    <SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://idp.example.test/sso"/>
  </IDPSSODescriptor>
</EntityDescriptor>`, b64)
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}
