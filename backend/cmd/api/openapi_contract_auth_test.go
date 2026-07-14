// S128-4 (OA-08): authenticated contract cases.
//
// The existing contract test (openapi_contract_test.go) hits every endpoint
// WITHOUT a token, so almost all of its cases validate a 401 body. That proves
// the path exists in the spec and that the error shape is right — it proves
// nothing about the shape of the data the endpoint actually returns, which is the
// half the frontend generates its types from.
//
// The gap is not theoretical. GET /vaktcomply/controls/{id}/changelog returned
// {"changelog": [...]} while the frontend expected a bare array, and .map() threw
// in the browser. A 401-only contract test cannot see that. This one can: it
// seeds an org and an Admin, mints a real Paseto token the same way the server
// derives its key, and validates the 200 body against the spec.
//
// It needs a migrated database and Redis — which CI's backend job already has
// (VAKT_DB_URL / VAKT_REDIS_URL). Without them it skips rather than fails: a
// developer running `go test ./...` on a laptop should not be forced to run
// Postgres to build.
package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/matharnica/vakt/internal/auth"
	"github.com/matharnica/vakt/internal/shared/apidocs"
	sharedcrypto "github.com/matharnica/vakt/internal/shared/crypto"
)

// authContractCases are read endpoints whose 200 body must match the spec.
//
// One per module surface, each a list endpoint that answers for a brand-new org
// (empty, but well-formed). An empty list is enough: the contract under test is
// the SHAPE — array vs. wrapper object, field names, nullability — and that is
// exactly what drifted. Adding a case here is cheap; add one whenever a response
// shape is worth freezing.
var authContractCases = []contractCase{
	{name: "auth_vaktscan_assets", method: http.MethodGet,
		realPath: "/api/v1/vaktscan/assets", specPath: "/api/v1/vaktscan/assets"},
	{name: "auth_vaktscan_findings", method: http.MethodGet,
		realPath: "/api/v1/vaktscan/findings", specPath: "/api/v1/vaktscan/findings"},
	{name: "auth_vaktcomply_frameworks", method: http.MethodGet,
		realPath: "/api/v1/vaktcomply/frameworks", specPath: "/api/v1/vaktcomply/frameworks"},
	{name: "auth_vaktcomply_policies", method: http.MethodGet,
		realPath: "/api/v1/vaktcomply/policies", specPath: "/api/v1/vaktcomply/policies"},
	{name: "auth_vaktcomply_policy_templates", method: http.MethodGet,
		realPath: "/api/v1/vaktcomply/policy-templates", specPath: "/api/v1/vaktcomply/policy-templates"},
	{name: "auth_vaktvault_projects", method: http.MethodGet,
		realPath: "/api/v1/vaktvault/projects", specPath: "/api/v1/vaktvault/projects"},
	// NOTE: vaktaware/campaigns and /api-keys are deliberately absent. They are
	// Pro-gated and answer 402 on a Community licence — correct behaviour, but it
	// means they can never produce a 200 body here. A case that cannot pass is not
	// a stricter test, only a broken one.
	{name: "auth_vaktprivacy_vvt", method: http.MethodGet,
		realPath: "/api/v1/vaktprivacy/vvt", specPath: "/api/v1/vaktprivacy/vvt"},
	{name: "auth_vaktprivacy_dsr_portal_settings", method: http.MethodGet,
		realPath: "/api/v1/vaktprivacy/dsr-portal-settings", specPath: "/api/v1/vaktprivacy/dsr-portal-settings"},
	{name: "auth_vakthr_employees", method: http.MethodGet,
		realPath: "/api/v1/vakthr/employees", specPath: "/api/v1/vakthr/employees"},
	{name: "auth_vaktcomply_capas", method: http.MethodGet,
		realPath: "/api/v1/vaktcomply/capas", specPath: "/api/v1/vaktcomply/capas"},
	{name: "auth_vaktprivacy_breaches", method: http.MethodGet,
		realPath: "/api/v1/vaktprivacy/breaches", specPath: "/api/v1/vaktprivacy/breaches"},
}

// TestOpenAPIContractAuthenticated validates real 200 bodies against the spec.
func TestOpenAPIContractAuthenticated(t *testing.T) {
	dbURL := os.Getenv("VAKT_DB_URL")
	secret := os.Getenv("VAKT_SECRET_KEY")
	if dbURL == "" || secret == "" || os.Getenv("VAKT_REDIS_URL") == "" {
		t.Skip("needs VAKT_DB_URL + VAKT_REDIS_URL + VAKT_SECRET_KEY (CI sets all three)")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	token := seedOrgAndToken(ctx, t, pool, secret)

	specBytes, err := apidocs.SpecBytes()
	if err != nil {
		t.Fatalf("read embedded spec: %v", err)
	}
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(specBytes)
	if err != nil {
		t.Fatalf("parse spec: %v", err)
	}
	router, err := gorillamux.NewRouter(doc)
	if err != nil {
		t.Fatalf("build router: %v", err)
	}

	e, _ := setupEcho(ctx, testConfig())

	for _, tc := range authContractCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.realPath, bytes.NewReader(nil))
			req.Header.Set("Authorization", "Bearer "+token)

			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200 with a valid Admin token, got %d — body: %s\n"+
					"(a 401 here means the token/seed is wrong; a 403 means the route is "+
					"gated beyond Admin; either way the case cannot validate a 200 body)",
					rec.Code, rec.Body.String())
			}

			matchReq, _ := http.NewRequest(tc.method, tc.specPath, nil)
			route, _, err := router.FindRoute(matchReq)
			if err != nil {
				t.Fatalf("no OpenAPI operation matches %s %s — the spec is missing this endpoint",
					tc.method, tc.specPath)
			}

			vRes := &openapi3filter.ResponseValidationInput{
				RequestValidationInput: &openapi3filter.RequestValidationInput{
					Request: matchReq,
					Route:   route,
				},
				Status: rec.Code,
				Header: rec.Header(),
				Body:   noCloseBuffer{Reader: bytes.NewReader(rec.Body.Bytes())},
			}
			vRes.SetBodyBytes(rec.Body.Bytes())

			if err := openapi3filter.ValidateResponse(ctx, vRes); err != nil {
				t.Errorf("200 body does not match the spec: %v\n\nbody: %s",
					err, rec.Body.String())
			}
		})
	}
}

// seedOrgAndToken creates an org + Admin user and mints an access token for them,
// deriving the Paseto key exactly the way the server does (HKDF over the master
// key with the `vakt-paseto-v1` purpose) — if that derivation ever changes, this
// test stops authenticating and says so, which is the correct outcome.
func seedOrgAndToken(ctx context.Context, t *testing.T, pool *pgxpool.Pool, secret string) string {
	t.Helper()

	var orgID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ('Contract Test', 'contract-test-'||substr(md5(random()::text),1,8))
		 RETURNING id::text`).Scan(&orgID); err != nil {
		t.Fatalf("seed org: %v", err)
	}

	var userID string
	var pwVersion int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email) VALUES ('contract-'||substr(md5(random()::text),1,8)||'@test.local')
		 RETURNING id::text, pw_version`).Scan(&userID, &pwVersion); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	// Membership is its own table — a user is not "in" an org by carrying its id,
	// and the module-permission middleware reads the membership, not the token.
	if _, err := pool.Exec(ctx,
		`INSERT INTO org_members (org_id, user_id, role_id)
		 SELECT $1::uuid, $2::uuid, id FROM roles WHERE name = 'Admin' LIMIT 1`,
		orgID, userID); err != nil {
		t.Fatalf("seed membership: %v", err)
	}

	raw, err := hex.DecodeString(secret)
	if err != nil {
		t.Fatalf("decode master key: %v", err)
	}
	keyBytes, err := sharedcrypto.DeriveServiceKey(raw, "vakt-paseto-v1")
	if err != nil {
		t.Fatalf("derive paseto key: %v", err)
	}
	key, err := auth.GenerateSymmetricKeyFromBytes(keyBytes)
	if err != nil {
		t.Fatalf("paseto key: %v", err)
	}

	token, err := auth.IssueAccessToken(key, auth.Claims{
		UserID:    userID,
		OrgID:     orgID,
		Roles:     []string{"Admin"},
		PwVersion: pwVersion,
		MFA:       true,
	})
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	return token
}
