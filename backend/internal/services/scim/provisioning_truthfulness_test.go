// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package scim

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/license"
)

// Three findings, one theme: SCIM answered SUCCESS for a change it did not make.
//
// That shape is worse than an error. Vakt HR's promise is audit-ready evidence
// that access provisioning and revocation happened; an IdP that is told "204,
// deprovisioned" writes exactly that into its own log, and the evidence then
// certifies a lock-out that never occurred.
//
//   - R1-14-D07  PATCH with a body that carries no Operations answers 200 and
//     changes nothing. Entra ID's "disable user" is a PATCH.
//   - R1-14cA-12 DELETE answers 204 but leaves is_active TRUE when the account
//     was not SCIM-provisioned (predicate AND scim_provisioned = TRUE).
//   - R1-14-D08  emails[] is read and thrown away; users.email receives the
//     userName, so every Vakt mail goes to the login name.

func newSCIMTestServer(t *testing.T) (*echo.Echo, string, string) {
	t.Helper()
	pool := scimTestDB(t)
	orgID := seedSCIMOrg(t, pool)
	token := seedSCIMToken(t, pool, orgID)

	e := echo.New()
	g := e.Group("/scim/v2", func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("license", &license.License{
				Tier:     "pro",
				Features: []string{license.FeatureSCIMProvisioning},
			})
			return next(c)
		}
	})
	Register(g, pool)
	return e, token, orgID
}

// R1-14-D07 — a PATCH that changes nothing must not answer 200.
func TestSCIMPatchUser_bodyWithoutOperationsIsRejected(t *testing.T) {
	pool := scimTestDB(t)
	e, token, _ := newSCIMTestServer(t)

	suffix := fmt.Sprintf("%d", os.Getpid())
	email := "patch-noop-" + suffix + "@example.com"
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE email = $1`, email) })

	rec := doSCIM(t, e, http.MethodPost, "/scim/v2/Users", token, MediaTypeSCIMJSON, map[string]any{
		"schemas":  []string{schemaUser},
		"userName": email,
		"active":   true,
	})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var created scimUserResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))

	// The shape an IdP sends when its PATCH template is misconfigured: a User
	// resource instead of a PatchOp document. "active": false is right there in
	// the body, and it used to be answered with 200 and ignored.
	rec = doSCIM(t, e, http.MethodPatch, "/scim/v2/Users/"+created.ID, token, MediaTypeSCIMJSON, map[string]any{
		"schemas": []string{schemaUser},
		"active":  false,
	})
	assert.Equal(t, http.StatusBadRequest, rec.Code,
		"a PATCH without Operations changed nothing but reported success: %s", rec.Body.String())

	var stillActive bool
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT is_active FROM users WHERE email = $1`, email).Scan(&stillActive))
	assert.True(t, stillActive, "precondition: the account is in fact still active")
}

// The same hole one level down: Operations is present but every op names a path
// this implementation does not support. Nothing is applied — and 200 would say
// otherwise.
func TestSCIMPatchUser_onlyUnsupportedOperationsIsRejected(t *testing.T) {
	pool := scimTestDB(t)
	e, token, _ := newSCIMTestServer(t)

	suffix := fmt.Sprintf("%d", os.Getpid())
	email := "patch-unknown-" + suffix + "@example.com"
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE email = $1`, email) })

	rec := doSCIM(t, e, http.MethodPost, "/scim/v2/Users", token, MediaTypeSCIMJSON, map[string]any{
		"schemas": []string{schemaUser}, "userName": email, "active": true,
	})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var created scimUserResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))

	rec = doSCIM(t, e, http.MethodPatch, "/scim/v2/Users/"+created.ID, token, MediaTypeSCIMJSON, map[string]any{
		"schemas": []string{schemaPatchOp},
		"Operations": []map[string]any{
			{"op": "replace", "path": "urn:ietf:params:scim:schemas:extension:enterprise:2.0:User:department", "value": "Sales"},
		},
	})
	assert.Equal(t, http.StatusBadRequest, rec.Code,
		"a PATCH whose every operation was skipped reported success: %s", rec.Body.String())
}

// A supported PATCH still has to work — the guard must not swallow the real path.
func TestSCIMPatchUser_supportedOperationStillApplies(t *testing.T) {
	pool := scimTestDB(t)
	e, token, _ := newSCIMTestServer(t)

	suffix := fmt.Sprintf("%d", os.Getpid())
	email := "patch-ok-" + suffix + "@example.com"
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE email = $1`, email) })

	rec := doSCIM(t, e, http.MethodPost, "/scim/v2/Users", token, MediaTypeSCIMJSON, map[string]any{
		"schemas": []string{schemaUser}, "userName": email, "active": true,
	})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var created scimUserResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))

	rec = doSCIM(t, e, http.MethodPatch, "/scim/v2/Users/"+created.ID, token, MediaTypeSCIMJSON, map[string]any{
		"schemas":    []string{schemaPatchOp},
		"Operations": []map[string]any{{"op": "replace", "path": "active", "value": false}},
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var active bool
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT is_active FROM users WHERE email = $1`, email).Scan(&active))
	assert.False(t, active, "the deactivation an IdP actually sends must take effect")
}

// R1-14cA-12 — DELETE reported 204 for an account it left active.
//
// The UPDATE carried "AND scim_provisioned = TRUE", so an account an admin had
// created in the app stayed usable while the IdP recorded a successful
// deprovisioning. Provenance is not a reason to ignore an offboarding.
func TestSCIMDeleteUser_deactivatesALocallyCreatedAccount(t *testing.T) {
	pool := scimTestDB(t)
	e, token, orgID := newSCIMTestServer(t)
	ctx := context.Background()

	suffix := fmt.Sprintf("%d", os.Getpid())
	email := "local-created-" + suffix + "@example.com"
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM users WHERE email = $1`, email) })

	// An account created in the app, never touched by SCIM.
	var userID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO users (email, display_name, is_active, role, scim_provisioned)
		VALUES ($1, 'Local Created', TRUE, 'viewer', FALSE) RETURNING id::text`, email).Scan(&userID))
	_, err := pool.Exec(ctx, `
		INSERT INTO org_members (org_id, user_id, role_id)
		SELECT $1::uuid, $2::uuid, id FROM roles WHERE name = 'Viewer'`, orgID, userID)
	require.NoError(t, err)

	rec := doSCIM(t, e, http.MethodDelete, "/scim/v2/Users/"+userID, token, MediaTypeSCIMJSON, nil)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())

	var active bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT is_active FROM users WHERE id = $1::uuid`, userID).Scan(&active))
	assert.False(t, active,
		"SCIM answered 204 for a deprovisioning it did not perform — the account stayed open")
}

// The offboarding must not be able to strip the customer of their last admin.
// An IdP sync that removes the final Admin membership leaves an organisation
// nobody can administer — the state W5-OPS-01 describes, created on purpose by
// a routine sync.
func TestSCIMDeleteUser_refusesToRemoveTheLastAdmin(t *testing.T) {
	pool := scimTestDB(t)
	e, token, orgID := newSCIMTestServer(t)
	ctx := context.Background()

	suffix := fmt.Sprintf("%d", os.Getpid())
	email := "last-admin-" + suffix + "@example.com"
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM users WHERE email = $1`, email) })

	var userID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO users (email, display_name, is_active, role)
		VALUES ($1, 'Last Admin', TRUE, 'admin') RETURNING id::text`, email).Scan(&userID))
	_, err := pool.Exec(ctx, `
		INSERT INTO org_members (org_id, user_id, role_id)
		SELECT $1::uuid, $2::uuid, id FROM roles WHERE name = 'Admin'`, orgID, userID)
	require.NoError(t, err)

	rec := doSCIM(t, e, http.MethodDelete, "/scim/v2/Users/"+userID, token, MediaTypeSCIMJSON, nil)
	assert.Equal(t, http.StatusConflict, rec.Code,
		"the last admin was deprovisioned — the organisation is now unadministrable: %s", rec.Body.String())

	var active bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT is_active FROM users WHERE id = $1::uuid`, userID).Scan(&active))
	assert.True(t, active, "the refused deprovisioning must not have taken partial effect")

	var members int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM org_members WHERE org_id = $1::uuid AND user_id = $2::uuid`,
		orgID, userID).Scan(&members))
	assert.Equal(t, 1, members, "the membership was removed despite the refusal")
}

// R1-14-D08 — emails[] was read and dropped; users.email got the userName.
//
// An IdP that keeps login name and mail address apart (Entra ID's
// userPrincipalName vs. mail is the common case) provisioned a users row whose
// email column is a login name. Every Vakt mail — password reset, digest,
// campaign — then goes to that string.
func TestSCIMCreateUser_storesTheMailAddressNotTheLoginName(t *testing.T) {
	pool := scimTestDB(t)
	e, token, _ := newSCIMTestServer(t)
	ctx := context.Background()

	suffix := fmt.Sprintf("%d", os.Getpid())
	userName := "svc-account-" + suffix
	mail := "vorname.nachname-" + suffix + "@example.com"
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE email = $1 OR email = $2`, mail, userName)
	})

	rec := doSCIM(t, e, http.MethodPost, "/scim/v2/Users", token, MediaTypeSCIMJSON, map[string]any{
		"schemas":     []string{schemaUser},
		"userName":    userName,
		"displayName": "Vorname Nachname",
		"emails": []map[string]any{
			{"value": mail, "type": "work", "primary": true},
		},
		"active": true,
	})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var stored string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT email FROM users WHERE scim_user_name = $1`, userName).Scan(&stored))
	assert.Equal(t, mail, stored,
		"users.email carries the login name — every mail to this employee goes to %q", userName)

	// The userName the IdP sent has to come back unchanged, or the IdP treats
	// the account as missing and provisions a second one on the next sync.
	var created scimUserResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	assert.Equal(t, userName, created.UserName)
	require.NotEmpty(t, created.Emails)
	assert.Equal(t, mail, created.Emails[0].Value)
}
