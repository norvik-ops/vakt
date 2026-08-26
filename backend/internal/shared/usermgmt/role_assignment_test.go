// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package usermgmt

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── the mapping table, no DB needed ──────────────────────────────────────────

// TestAssignableRoles_pinsTheADR0077Mapping pins every accepted role word to the
// pair of values a role change writes. Two failure modes it catches:
//
//  1. A new role that maps to the wrong cache value. Since ESK-13 the cache
//     authorises nothing, but it is what migration 253 and every INSERT-boundary
//     check derive from, and a mapping that caches InternalAuditor as "admin"
//     would put an 'admin' back into the column D24-1 came out of.
//  2. A role word the validator accepts but the map does not know. That value
//     used to fall through platformRoleName's default and become Viewer, i.e.
//     "grant InternalAuditor" would report success and grant Viewer.
func TestAssignableRoles_pinsTheADR0077Mapping(t *testing.T) {
	want := map[string]roleAssignment{
		"Admin":           {platform: "Admin", simple: "admin"},
		"SecurityAnalyst": {platform: "SecurityAnalyst", simple: "editor"},
		"Viewer":          {platform: "Viewer", simple: "viewer"},
		"AuditorReadOnly": {platform: "AuditorReadOnly", simple: "viewer"},
		"InternalAuditor": {platform: "InternalAuditor", simple: "viewer"},
		"admin":           {platform: "Admin", simple: "admin"},
		"editor":          {platform: "SecurityAnalyst", simple: "editor"},
		"viewer":          {platform: "Viewer", simple: "viewer"},
	}
	assert.Equal(t, want, assignableRoles,
		"the role vocabulary changed — update UpdateRoleInput's oneof tag and openapi.yaml with it")

	// users.role only knows three values (migration 077 / ADR-0077).
	for word, got := range assignableRoles {
		assert.Contains(t, []string{"admin", "editor", "viewer"}, got.simple,
			"%s maps to a users.role value the column does not have", word)
	}

	// Only Admin may produce the admin cache value.
	for word, got := range assignableRoles {
		if got.simple == "admin" {
			assert.Equal(t, "Admin", got.platform, "%s caches as admin without being Admin", word)
		}
	}
}

// TestPlatformRoleName_derivesFromTheSameTable keeps the invitation-accept path
// and the role-change path from drifting into two different mappings.
func TestPlatformRoleName_derivesFromTheSameTable(t *testing.T) {
	for word, assign := range assignableRoles {
		assert.Equal(t, assign.platform, platformRoleName(word), "role %q", word)
	}
	// Unknown values stay least-privilege on the invitation path (see the
	// function's comment for why this differs from UpdateUserRole).
	assert.Equal(t, "Viewer", platformRoleName("something-else"))
}

// ─── authorisation: who may hand out roles ───────────────────────────────────

// TestRequireAdmin_pinsWhoMayAssignRoles is the authorisation gate for a
// privileged path. Assigning roles is the one call that can turn any member into
// an Admin, and this repository has a documented privilege-escalation history on
// exactly this surface (D24-1: OIDC/SCIM-provisioned users reached requireAdmin
// because users.role defaulted to 'admin' — the reason it no longer authorises).
//
// It runs the real requireAdmin middleware against a real database, once per
// role, so a middleware that stops reading the role fails here. The Admin row is
// the control: without it a middleware that rejects everyone would look like a
// passing security test. The fixtures here are consistent; the drifted ones,
// where the two columns disagree, are in
// TestRoleSourceOfTruth_guardAndGateReadTheSameColumn.
func TestRequireAdmin_pinsWhoMayAssignRoles(t *testing.T) {
	pool, orgID := testDB(t)

	// Every value users.role can hold, plus the platform roles that cache as
	// "viewer" — an InternalAuditor must NOT be able to hand out roles, or the
	// segregation of duties ADR-0055 establishes would be self-serving.
	cases := []struct {
		platform   string
		wantAccess bool
	}{
		{"Admin", true},
		{"SecurityAnalyst", false},
		{"Viewer", false},
		{"AuditorReadOnly", false},
		{"InternalAuditor", false},
	}

	for _, tc := range cases {
		t.Run(tc.platform, func(t *testing.T) {
			userID := seedMember(t, pool, orgID, tc.platform)

			e := echo.New()
			rec := httptest.NewRecorder()
			c := e.NewContext(httptest.NewRequest(http.MethodPatch, "/", nil), rec)
			c.Set("user_id", userID)
			c.Set("org_id", orgID)

			reached := false
			handler := func(c echo.Context) error {
				reached = true
				return c.NoContent(http.StatusNoContent)
			}
			require.NoError(t, requireAdmin(pool)(handler)(c))

			if tc.wantAccess {
				assert.True(t, reached, "%s must reach the role-assignment handler", tc.platform)
				assert.Equal(t, http.StatusNoContent, rec.Code)
				return
			}
			assert.False(t, reached, "%s must NOT reach the role-assignment handler", tc.platform)
			assert.Equal(t, http.StatusForbidden, rec.Code)
			assert.Contains(t, rec.Body.String(), "AUTH_INSUFFICIENT_ROLE")
			assert.Contains(t, rec.Body.String(), "Admin", "the 403 must name the role it wants")
		})
	}

	t.Run("no user_id in context", func(t *testing.T) {
		e := echo.New()
		rec := httptest.NewRecorder()
		c := e.NewContext(httptest.NewRequest(http.MethodPatch, "/", nil), rec)

		reached := false
		require.NoError(t, requireAdmin(pool)(func(echo.Context) error {
			reached = true
			return nil
		})(c))
		assert.False(t, reached)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

// ─── the role change itself ──────────────────────────────────────────────────

// TestUpdateUserRole_writesTheAuthoritativeRole is the ESK-13 regression test.
//
// Measured live before the fix, against a real instance: PATCH
// /admin/users/:id/role with {"role":"InternalAuditor"} answered 422 (the value
// was not in the vocabulary), and {"role":"admin"} answered 204 while writing
// only users.role — org_members.role_id stayed Viewer and the member's next
// login still carried ["Viewer"]. The reverse was worse: demoting an Admin to
// "viewer" returned 204, the team list showed "Betrachter", and a fresh login
// still carried ["Admin"] over the whole platform. org_members.role_id had nine
// INSERT sites in non-test code (seven outside cmd/seed and demoseed) and no
// UPDATE site at all.
//
// Non-vacuity: the org_members assertions fail against the old implementation
// (verified by running this test against it, not assumed) — that is the whole
// content of the fix.
func TestUpdateUserRole_writesTheAuthoritativeRole(t *testing.T) {
	pool, orgID := testDB(t)
	ctx := context.Background()
	svc := NewService(pool, SMTPConfig{}, "")

	// A second Admin so the last-admin guard never masks what is being tested.
	seedMember(t, pool, orgID, "Admin")

	t.Run("grants InternalAuditor — the role that had no UI path", func(t *testing.T) {
		userID := seedMember(t, pool, orgID, "Viewer")
		require.NoError(t, svc.UpdateUserRole(ctx, orgID, userID, "InternalAuditor"))

		platform, simple := readRoles(t, pool, orgID, userID)
		assert.Equal(t, "InternalAuditor", platform,
			"org_members carries the role the token claim and every RequireRole guard read")
		assert.Equal(t, "viewer", simple,
			"an internal auditor is not an admin on the requireAdmin surface")
	})

	t.Run("promotion reaches the authorisation role, not just the cache", func(t *testing.T) {
		userID := seedMember(t, pool, orgID, "Viewer")
		require.NoError(t, svc.UpdateUserRole(ctx, orgID, userID, "Admin"))

		platform, simple := readRoles(t, pool, orgID, userID)
		assert.Equal(t, "Admin", platform)
		assert.Equal(t, "admin", simple)
	})

	t.Run("demotion actually removes the Admin claim", func(t *testing.T) {
		userID := seedMember(t, pool, orgID, "Admin")
		require.NoError(t, svc.UpdateUserRole(ctx, orgID, userID, "Viewer"))

		platform, simple := readRoles(t, pool, orgID, userID)
		assert.Equal(t, "Viewer", platform,
			"before the fix this stayed Admin — the demoted user kept full Admin claims")
		assert.Equal(t, "viewer", simple)
	})

	t.Run("legacy lowercase words still work", func(t *testing.T) {
		userID := seedMember(t, pool, orgID, "Viewer")
		require.NoError(t, svc.UpdateUserRole(ctx, orgID, userID, "editor"))

		platform, simple := readRoles(t, pool, orgID, userID)
		assert.Equal(t, "SecurityAnalyst", platform)
		assert.Equal(t, "editor", simple)
	})

	t.Run("unknown role fails closed, changing nothing", func(t *testing.T) {
		userID := seedMember(t, pool, orgID, "Viewer")
		require.Error(t, svc.UpdateUserRole(ctx, orgID, userID, "Superuser"))

		platform, simple := readRoles(t, pool, orgID, userID)
		assert.Equal(t, "Viewer", platform)
		assert.Equal(t, "viewer", simple)
	})

	t.Run("a member of another org cannot be re-roled", func(t *testing.T) {
		otherOrg := seedOrg(t, pool, "esk13-other")
		userID := seedMember(t, pool, otherOrg, "Viewer")

		require.Error(t, svc.UpdateUserRole(ctx, orgID, userID, "Admin"),
			"an admin must not be able to re-role a user outside their organisation")
		platform, _ := readRoles(t, pool, otherOrg, userID)
		assert.Equal(t, "Viewer", platform)
	})

	t.Run("both columns move together or not at all", func(t *testing.T) {
		// The transaction boundary: the guard, the list and the token must never
		// disagree, which is the split-brain ADR-0077 exists to prevent.
		userID := seedMember(t, pool, orgID, "Viewer")
		require.NoError(t, svc.UpdateUserRole(ctx, orgID, userID, "AuditorReadOnly"))

		users, err := svc.ListUsers(ctx, orgID)
		require.NoError(t, err)
		var found bool
		for _, u := range users {
			if u.ID == userID {
				found = true
				assert.Equal(t, "AuditorReadOnly", u.PlatformRole,
					"the team list must show the authoritative role, not the lossy cache")
				assert.Equal(t, "viewer", u.Role)
			}
		}
		assert.True(t, found, "the re-roled member must still be listed")
	})
}

// TestUpdateUserRole_lastAdminGuardCountsTheAuthoritativeRole makes sure the fix
// did not open a lock-out: with org_members now really changing, a guard that
// counted the users.role cache could clear the demotion of the only Admin.
func TestUpdateUserRole_lastAdminGuardCountsTheAuthoritativeRole(t *testing.T) {
	pool, _ := testDB(t)
	ctx := context.Background()
	svc := NewService(pool, SMTPConfig{}, "")

	orgID := seedOrg(t, pool, "esk13-single-admin")
	adminID := seedMember(t, pool, orgID, "Admin")

	require.Error(t, svc.UpdateUserRole(ctx, orgID, adminID, "Viewer"),
		"the only Admin must not be demotable")
	platform, _ := readRoles(t, pool, orgID, adminID)
	assert.Equal(t, "Admin", platform)

	// With a second Admin the demotion is allowed again — the control that keeps
	// the assertion above from passing because the guard blocks everything.
	seedMember(t, pool, orgID, "Admin")
	require.NoError(t, svc.UpdateUserRole(ctx, orgID, adminID, "Viewer"))
	platform, _ = readRoles(t, pool, orgID, adminID)
	assert.Equal(t, "Viewer", platform)
}

// ─── one column, or the guard and the gate drift ─────────────────────────────

// TestRoleSourceOfTruth_guardAndGateReadTheSameColumn is the REV-ESK13 §2.1/§2.2
// regression test. Every fixture here has the two role columns deliberately
// DISAGREEING, because that is the state migration 249 could not reach and every
// role change made before ESK-13 produced:
//
//	§2.1  org_members = Admin,  users.role = 'viewer'   ("phantom admin")
//	§2.2  org_members = Viewer, users.role = 'admin'    ("cache admin")
//
// Against a guard that counts org_members while requireAdmin authorises over
// users.role, the first shape locks an organisation out of its own user
// management (the last-admin guard clears the demotion of the only member who
// still passes the gate) and the second hands a demoted member the whole
// role-assignment surface. Both are measured below, so both go red if the two
// ever read different columns again.
//
// Non-vacuity of every subtest was established by reverting the fix and watching
// it fail — see the commit message for the exit codes.
func TestRoleSourceOfTruth_guardAndGateReadTheSameColumn(t *testing.T) {
	pool, _ := testDB(t)
	ctx := context.Background()
	svc := NewService(pool, SMTPConfig{}, "")

	// §2.2 — the cache is not a credential. A member whose org_members row says
	// Viewer carries Viewer in every token claim; letting them through here would
	// let them PATCH their own org_members row to Admin, i.e. mint the claims the
	// operator had already taken away. That is the D24-1 shape through a new door.
	t.Run("a cache-only admin does not reach the role-assignment surface", func(t *testing.T) {
		orgID := seedOrg(t, pool, "esk13-cache-admin")
		seedMember(t, pool, orgID, "Admin") // control: a real admin exists
		driftedID := seedDriftedMember(t, pool, orgID, "Viewer", "admin")

		reached, code := callRequireAdmin(t, pool, orgID, driftedID)
		assert.False(t, reached, "users.role='admin' with org_members=Viewer must not authorise")
		assert.Equal(t, http.StatusForbidden, code)
	})

	// §2.1 — the authoritative role IS the credential. This member holds Admin in
	// org_members, so their token claim is ["Admin"] and auth.RequireRole("Admin")
	// admits them on every other module. Rejecting them only here is what leaves
	// an org with nobody able to repair the roles.
	t.Run("a phantom admin keeps the surface their claims already give them", func(t *testing.T) {
		orgID := seedOrg(t, pool, "esk13-phantom-admin")
		phantomID := seedDriftedMember(t, pool, orgID, "Admin", "viewer")

		reached, code := callRequireAdmin(t, pool, orgID, phantomID)
		assert.True(t, reached, "org_members=Admin must authorise regardless of the cache")
		assert.Equal(t, http.StatusNoContent, code)
	})

	// The lock-out itself, end to end: the guard permits this demotion (two Admins
	// in org_members), so the ONLY thing that keeps the org usable afterwards is
	// that the phantom still passes the gate. Counting one column and authorising
	// over the other measured 0 remaining admins here (REV-ESK13 §2.1).
	t.Run("after a permitted demotion the org still has a working admin", func(t *testing.T) {
		orgID := seedOrg(t, pool, "esk13-lockout")
		realAdminID := seedMember(t, pool, orgID, "Admin")
		phantomID := seedDriftedMember(t, pool, orgID, "Admin", "viewer")

		require.NoError(t, svc.UpdateUserRole(ctx, orgID, realAdminID, "Viewer"),
			"two Admins in org_members — the guard has no reason to block")

		working := 0
		for _, id := range []string{realAdminID, phantomID} {
			if reached, _ := callRequireAdmin(t, pool, orgID, id); reached {
				working++
			}
		}
		assert.Equal(t, 1, working,
			"the organisation must not be left without anyone who can assign roles")
	})

	// The other half of the same invariant: the guard has to protect the member
	// the gate lets in. A guard counting users.role sees zero admins here, decides
	// the target is not an admin, and lets the demotion through — which is the
	// last one that could have repaired anything.
	t.Run("the last admin is protected even when the cache disagrees", func(t *testing.T) {
		orgID := seedOrg(t, pool, "esk13-last-phantom")
		phantomID := seedDriftedMember(t, pool, orgID, "Admin", "viewer")
		seedMember(t, pool, orgID, "Viewer")

		require.Error(t, svc.UpdateUserRole(ctx, orgID, phantomID, "Viewer"),
			"the only member who passes requireAdmin must not be demotable")
		platform, _ := readRoles(t, pool, orgID, phantomID)
		assert.Equal(t, "Admin", platform)

		// Control, so the assertion above cannot pass because the guard blocks
		// everything: with a second authoritative Admin the demotion is allowed.
		seedMember(t, pool, orgID, "Admin")
		require.NoError(t, svc.UpdateUserRole(ctx, orgID, phantomID, "Viewer"))
		platform, _ = readRoles(t, pool, orgID, phantomID)
		assert.Equal(t, "Viewer", platform)
	})

	// RemoveUser shares the guard, and before ESK-13 it was the cheaper way to
	// reach zero authoritative admins: the old guard only protected a target whose
	// users.role said 'admin', so a phantom admin could simply be deleted.
	t.Run("removing the last admin is blocked on the same column", func(t *testing.T) {
		orgID := seedOrg(t, pool, "esk13-remove-phantom")
		phantomID := seedDriftedMember(t, pool, orgID, "Admin", "viewer")
		seedMember(t, pool, orgID, "Viewer")

		require.Error(t, svc.RemoveUser(ctx, orgID, phantomID),
			"the only authoritative Admin must not be removable")
		platform, _ := readRoles(t, pool, orgID, phantomID)
		assert.Equal(t, "Admin", platform)
	})
}

// TestUpdateUserRole_lastAdminAnswerSaysWhatWasRefused pins the 400 body for the
// guard. Live before this change the operator got
// `{"error":"Rolle konnte nicht aktualisiert werden"}` — true but useless: it does
// not say the last-admin rule fired, so the obvious next step is an UPDATE in the
// database. That is the same complaint ESK-13 raises about a bare "forbidden".
//
// The masking of every OTHER service error stays, and the second subtest is what
// keeps this from becoming a licence to leak pgx errors to clients.
func TestUpdateUserRole_lastAdminAnswerSaysWhatWasRefused(t *testing.T) {
	pool, _ := testDB(t)
	svc := NewService(pool, SMTPConfig{}, "")
	h := newHandler(svc)

	orgID := seedOrg(t, pool, "esk13-last-admin-msg")
	adminID := seedMember(t, pool, orgID, "Admin")

	patch := func(targetID, role string) *httptest.ResponseRecorder {
		e := echo.New()
		req := httptest.NewRequest(http.MethodPatch, "/", strings.NewReader(`{"role":"`+role+`"}`))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues(targetID)
		c.Set("org_id", orgID)
		c.Set("user_id", adminID)
		require.NoError(t, h.UpdateUserRole(c))
		return rec
	}

	t.Run("names the rule that refused", func(t *testing.T) {
		rec := patch(adminID, "Viewer")
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "USERMGMT_LAST_ADMIN")
		assert.Contains(t, rec.Body.String(), "Admin",
			"the answer must say which role the organisation would lose")
	})

	t.Run("every other failure stays masked", func(t *testing.T) {
		rec := patch("00000000-0000-0000-0000-000000000000", "Viewer")
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "USERMGMT_ROLE_UPDATE_FAILED")
		assert.NotContains(t, rec.Body.String(), "SQLSTATE",
			"internal errors must not reach the client")
		assert.NotContains(t, rec.Body.String(), "org_members")
	})
}

// TestEnsureNotLastAdmin_waitsForACompetingDemotion covers the TOCTOU the guard
// had while it ran on the pool: two demotions could both read "2 admins" before
// either wrote, and both be let through, leaving the org with none.
//
// It drives the interleaving instead of hoping for it. Simply running two
// UpdateUserRole calls from two goroutines does NOT show the defect — REV-ESK13
// tried five times and so did I, and the unlocked version won every race. A test
// that only wins by luck proves nothing, so this one asserts the mechanism: while
// a competing transaction holds the demotion open, the second guard must still be
// waiting. Without FOR UPDATE OF om it answers from its own snapshot — two admins,
// go ahead — and the t.Fatal below fires.
func TestEnsureNotLastAdmin_waitsForACompetingDemotion(t *testing.T) {
	pool, _ := testDB(t)
	ctx := context.Background()
	svc := NewService(pool, SMTPConfig{}, "")

	orgID := seedOrg(t, pool, "esk13-toctou")
	first := seedMember(t, pool, orgID, "Admin")
	second := seedMember(t, pool, orgID, "Admin")

	var viewerRoleID string
	require.NoError(t, pool.QueryRow(ctx, `SELECT id::text FROM roles WHERE name = 'Viewer'`).Scan(&viewerRoleID))

	// Transaction 1 asks the guard (two Admins, permitted) and performs its
	// demotion, but does not commit yet.
	tx1, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx1.Rollback(ctx) }()
	require.NoError(t, svc.ensureNotLastAdmin(ctx, tx1, orgID, first))
	_, err = tx1.Exec(ctx,
		`UPDATE org_members SET role_id = $1::uuid WHERE org_id = $2::uuid AND user_id = $3::uuid`,
		viewerRoleID, orgID, first)
	require.NoError(t, err)

	// Transaction 2 asks the same question about the other Admin, concurrently.
	answer := make(chan error, 1)
	go func() {
		tx2, err := pool.Begin(ctx)
		if err != nil {
			answer <- err
			return
		}
		defer func() { _ = tx2.Rollback(ctx) }()
		answer <- svc.ensureNotLastAdmin(ctx, tx2, orgID, second)
	}()

	select {
	case err := <-answer:
		t.Fatalf("the guard answered (%v) while a competing demotion was still open — "+
			"both would have been let through, leaving the organisation without an Admin", err)
	case <-time.After(500 * time.Millisecond):
		// Still blocked on the row lock, which is the point.
	}

	require.NoError(t, tx1.Commit(ctx))
	require.ErrorIs(t, <-answer, ErrLastAdmin,
		"once the first demotion is visible, the second must be refused")

	var admins int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM org_members om JOIN roles r ON r.id = om.role_id
		WHERE om.org_id = $1::uuid AND r.name = 'Admin'`, orgID).Scan(&admins))
	assert.Equal(t, 1, admins, "the organisation must keep an Admin")
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// testDB returns a pool against the migrated test database plus a fresh org, or
// skips. CI sets VAKT_DB_URL for this package.
func testDB(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	dbURL := os.Getenv("VAKT_DB_URL")
	if dbURL == "" {
		t.Skip("VAKT_DB_URL not set — role-assignment tests need a migrated Postgres (CI sets it)")
	}
	pool, err := pgxpool.New(context.Background(), dbURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	return pool, seedOrg(t, pool, "esk13")
}

var orgSeq int

func seedOrg(t *testing.T, pool *pgxpool.Pool, prefix string) string {
	t.Helper()
	orgSeq++
	slug := fmt.Sprintf("%s-%d-%d", prefix, os.Getpid(), orgSeq)

	var orgID string
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO organizations (name, slug) VALUES ($1, $1) RETURNING id::text`, slug,
	).Scan(&orgID))
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id IN (SELECT user_id FROM org_members WHERE org_id = $1::uuid)`, orgID)
		_, _ = pool.Exec(ctx, `DELETE FROM organizations WHERE id = $1::uuid`, orgID)
	})
	return orgID
}

// seedMember creates a user in orgID holding the given platform role, with
// users.role set to the matching cache value — i.e. a member in the consistent
// state ADR-0077 requires at every insert boundary.
func seedMember(t *testing.T, pool *pgxpool.Pool, orgID, platformRole string) string {
	t.Helper()
	assign, ok := assignableRoles[platformRole]
	require.True(t, ok, "unknown seed role %q", platformRole)
	return seedDriftedMember(t, pool, orgID, platformRole, assign.simple)
}

// seedDriftedMember creates a member whose authoritative role (org_members) and
// cache (users.role) deliberately DISAGREE. Consistent fixtures cannot tell the
// two columns apart, which is exactly why the first version of the last-admin
// test stayed green when the guard was pointed at the wrong one (REV-ESK13 §2.1).
//
// The state is not synthetic: every role change made before ESK-13 wrote only
// users.role, so a promotion left (Viewer, 'admin') and a demotion left
// (Admin, 'viewer'). Migration 249 reached only the first shape and only where
// no org granted Admin.
func seedDriftedMember(t *testing.T, pool *pgxpool.Pool, orgID, platformRole, cacheRole string) string {
	t.Helper()
	ctx := context.Background()

	orgSeq++
	email := fmt.Sprintf("esk13-%s-%d-%d@example.test", platformRole, os.Getpid(), orgSeq)

	var userID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, display_name, role)
		 VALUES ($1, 'x', $2, $3) RETURNING id::text`,
		email, platformRole, cacheRole,
	).Scan(&userID))

	var roleID string
	require.NoError(t, pool.QueryRow(ctx, `SELECT id::text FROM roles WHERE name = $1`, platformRole).Scan(&roleID),
		"role %q missing from the roles table — migration 202 seeds InternalAuditor", platformRole)

	_, err := pool.Exec(ctx,
		`INSERT INTO org_members (org_id, user_id, role_id) VALUES ($1::uuid, $2::uuid, $3::uuid)`,
		orgID, userID, roleID)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1::uuid`, userID)
	})
	return userID
}

// callRequireAdmin runs the real requireAdmin middleware against the real
// database for one member and reports whether the handler behind it was reached,
// plus the status code. Asserting on the middleware rather than on a copy of its
// query is the point: a test that re-implements the lookup cannot notice when
// the middleware starts reading a different column.
func callRequireAdmin(t *testing.T, pool *pgxpool.Pool, orgID, userID string) (bool, int) {
	t.Helper()
	e := echo.New()
	rec := httptest.NewRecorder()
	c := e.NewContext(httptest.NewRequest(http.MethodPatch, "/", nil), rec)
	c.Set("user_id", userID)
	c.Set("org_id", orgID)

	reached := false
	require.NoError(t, requireAdmin(pool)(func(c echo.Context) error {
		reached = true
		return c.NoContent(http.StatusNoContent)
	})(c))
	return reached, rec.Code
}

// readRoles returns (org_members role name, users.role) for a member.
func readRoles(t *testing.T, pool *pgxpool.Pool, orgID, userID string) (string, string) {
	t.Helper()
	var platform, simple string
	require.NoError(t, pool.QueryRow(context.Background(), `
		SELECT r.name, u.role
		FROM org_members om
		JOIN roles r ON r.id = om.role_id
		JOIN users u ON u.id = om.user_id
		WHERE om.org_id = $1::uuid AND om.user_id = $2::uuid`,
		orgID, userID,
	).Scan(&platform, &simple))
	return platform, simple
}
