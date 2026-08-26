//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/auth"
	"github.com/matharnica/vakt/internal/modules/vaktcomply"
	"github.com/matharnica/vakt/internal/modules/vakthr"
)

// R1-14c-05 — Ein abgeschlossener Offboarding-Lauf muss den Zugang wirklich entziehen.
//
// Der Defekt: fireCompletionEvidence schrieb für einen vollständig abgearbeiteten
// Offboarding-Lauf — inklusive des Pflichtschritts off-1 "Alle IT-Zugänge
// widerrufen (AD, VPN, SaaS)" — eine ck_evidence-Zeile mit Status 'approved',
// während das Plattform-Konto derselben Person voll nutzbar blieb: GET /auth/me
// antwortete 200, die refresh_sessions lebten weiter, die org_members-Zeile stand,
// und der Mitarbeiterstatus blieb auf 'offboarding'. Der Entzugs-Code funktionierte;
// von hier aus rief ihn nur nie jemand auf.
//
// Diese Tests prüfen den ZUSTAND danach, nicht einen Aufruf: der Zugriffstoken
// läuft durch die echte auth.AuthMiddleware. Ein Aufrufzähler wäre zu schwach —
// genau dieser Defekt sah auf Aufruf-Ebene korrekt aus.

// offboardingItems mirrors the standard offboarding checklist (demoseed).
var offboardingItems = []vakthr.ChecklistItem{
	{ID: "off-1", Label: "Alle IT-Zugänge widerrufen (AD, VPN, SaaS)", Required: true},
	{ID: "off-2", Label: "Laptop und Hardware zurückgegeben", Required: true},
	{ID: "off-3", Label: "Übergabe der Aufgaben abgeschlossen", Required: true},
}

// deadRedisClient points at a closed port so every Redis call errors. That forces
// checkPwVersion onto its durable PostgreSQL fallback, which is what we want to
// exercise: pw_version in the DB is the source of truth for token validity.
func deadRedisClient() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:        "127.0.0.1:1",
		DialTimeout: 100 * time.Millisecond,
		ReadTimeout: 100 * time.Millisecond,
		MaxRetries:  -1,
	})
}

// tokenAccepted runs a Paseto access token through the real auth.AuthMiddleware
// and reports whether the request got through. This is the "is the account still
// usable?" question, answered by the same code path that answers GET /auth/me.
func tokenAccepted(t *testing.T, mw echo.MiddlewareFunc, token string) (bool, string) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	h := mw(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})
	require.NoError(t, h(c))
	return rec.Code == http.StatusOK, rec.Body.String()
}

// TestOffboardingRunCompletion_RevokesPlatformAccess is the headline regression
// guard: complete every step of a standard offboarding run and the account must
// be unusable afterwards.
//
// Rolling the fix back (removing the guardCompletionForType call from
// CompleteStep) makes this test fail on the very first access assertion — the
// token is still accepted — not merely on a bookkeeping detail.
func TestOffboardingRunCompletion_RevokesPlatformAccess(t *testing.T) {
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx := context.Background()

	const email = "leaving@acme.test"
	key := mustKeyIntegration(t)
	mw := auth.AuthMiddleware(key, pool, deadRedisClient())

	// A platform user: org member, live refresh session, live API key.
	var userID, roleID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email, display_name, is_active) VALUES ($1, 'Leaving', TRUE)
		 RETURNING id::text`, email).Scan(&userID))
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT id::text FROM roles ORDER BY name LIMIT 1`).Scan(&roleID))
	_, err := pool.Exec(ctx,
		`INSERT INTO org_members (org_id, user_id, role_id) VALUES ($1::uuid, $2::uuid, $3::uuid)`,
		orgID, userID, roleID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO refresh_sessions (user_id, org_id, token_hash, expires_at)
		 VALUES ($1::uuid, $2::uuid, 'run-off-hash', NOW() + INTERVAL '30 days')`, userID, orgID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO api_keys (org_id, created_by, name, key_hash, key_prefix)
		 VALUES ($1::uuid, $2::uuid, 'ci', 'run-off-keyhash', 'sk_test')`, orgID, userID)
	require.NoError(t, err)

	token, err := auth.IssueAccessToken(key, auth.Claims{
		UserID: userID, OrgID: orgID, Roles: []string{"Admin"}, PwVersion: 0,
	})
	require.NoError(t, err)

	// Baseline: the account works before offboarding. Without this the test could
	// pass vacuously against a token that was never valid in the first place.
	ok, body := tokenAccepted(t, mw, token)
	require.True(t, ok, "precondition: the account must be usable before offboarding, got %s", body)

	// HR service wired exactly as cmd/api does it, including the real evidence writer.
	authSvc := auth.NewService(pool, nil, key)
	hr := vakthr.NewServiceFromPool(pool).
		WithSessionRevoker(authSvc).
		WithEvidenceWriter(vaktcomply.NewHREvidenceWriter(pool))
	actor := vakthr.Actor{OrgID: orgID, UserID: userID, UserEmail: "admin@acme.test"}

	emp, err := hr.CreateEmployee(ctx, actor, vakthr.CreateEmployeeInput{
		FirstName: "Lea", LastName: "Ving", Email: email,
	})
	require.NoError(t, err)
	_, err = hr.CreateChecklist(ctx, actor, vakthr.CreateChecklistInput{
		Type: "offboarding", Name: "Standard-Offboarding", Items: offboardingItems,
	})
	require.NoError(t, err)

	run, err := hr.StartOffboarding(ctx, actor, emp.ID)
	require.NoError(t, err)
	require.NotEqual(t, "completed", run.Status)

	// Work the run: every step, in order, exactly as an admin ticks the boxes.
	for _, item := range offboardingItems {
		run, err = hr.CompleteStep(ctx, actor, run.ID, item.ID, "admin@acme.test")
		require.NoError(t, err, "step %s must complete", item.ID)
	}
	require.Equal(t, "completed", run.Status, "all required steps done → run completed")

	// ── The claim the evidence makes must now be true. ──────────────────────────

	// 1. The access token is dead. This is the assertion the old code failed.
	ok, body = tokenAccepted(t, mw, token)
	assert.False(t, ok,
		"the offboarded employee's access token is still accepted — the completed run "+
			"claims the IT access was revoked while the account stays usable; body=%s", body)
	assert.Contains(t, body, "AUTH_SESSION_INVALIDATED",
		"the token must be rejected because pw_version moved, not for an unrelated reason")

	// 2. Refresh sessions gone — no way to mint a fresh access token.
	// orgid-lint: global — scoped by user_id (global users table); asserts the RevokeAllSessions contract
	var sessions int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM refresh_sessions WHERE user_id = $1::uuid`, userID).Scan(&sessions))
	assert.Equal(t, 0, sessions, "completing the run must delete the refresh sessions")

	// 3. Org membership gone — org-scoped RBAC no longer grants anything.
	var members int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM org_members WHERE user_id = $1::uuid AND org_id = $2::uuid`,
		userID, orgID).Scan(&members))
	assert.Equal(t, 0, members, "completing the run must remove the org membership")

	// 4. API keys revoked — a long-lived sk_ key must not outlive the session cut.
	var liveKeys int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM api_keys WHERE org_id = $1::uuid AND created_by = $2::uuid
		   AND revoked_at IS NULL`, orgID, userID).Scan(&liveKeys))
	assert.Equal(t, 0, liveKeys, "completing the run must revoke the employee's API keys")

	// 5. The HR record agrees with reality instead of staying on 'offboarding'.
	var empStatus string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT status FROM hr_employees WHERE org_id = $1::uuid AND id = $2::uuid`,
		orgID, emp.ID).Scan(&empStatus))
	assert.Equal(t, "terminated", empStatus,
		"a completed offboarding run must leave the employee terminated, not still offboarding")

	// 6. And the evidence exists exactly once, so the fix did not cost the feature.
	var evidence int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM ck_evidence WHERE org_id = $1::uuid
		   AND source = 'hr_checklist_completed' AND status = 'approved'`, orgID).Scan(&evidence))
	assert.Equal(t, 1, evidence, "the completed run must still produce exactly one approved evidence row")
}

// TestOffboardingRunCompletion_IsIdempotent covers repetition, Asynq retries and
// double-clicks: a second completion must neither fail nor duplicate anything.
func TestOffboardingRunCompletion_IsIdempotent(t *testing.T) {
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx := context.Background()

	const email = "twice@acme.test"
	key := mustKeyIntegration(t)

	var userID, roleID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email, display_name, is_active) VALUES ($1, 'Twice', TRUE)
		 RETURNING id::text`, email).Scan(&userID))
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT id::text FROM roles ORDER BY name LIMIT 1`).Scan(&roleID))
	_, err := pool.Exec(ctx,
		`INSERT INTO org_members (org_id, user_id, role_id) VALUES ($1::uuid, $2::uuid, $3::uuid)`,
		orgID, userID, roleID)
	require.NoError(t, err)

	authSvc := auth.NewService(pool, nil, key)
	hr := vakthr.NewServiceFromPool(pool).
		WithSessionRevoker(authSvc).
		WithEvidenceWriter(vaktcomply.NewHREvidenceWriter(pool))
	actor := vakthr.Actor{OrgID: orgID, UserID: userID, UserEmail: "admin@acme.test"}

	emp, err := hr.CreateEmployee(ctx, actor, vakthr.CreateEmployeeInput{
		FirstName: "Tina", LastName: "Wice", Email: email,
	})
	require.NoError(t, err)
	_, err = hr.CreateChecklist(ctx, actor, vakthr.CreateChecklistInput{
		Type: "offboarding", Name: "Standard-Offboarding", Items: offboardingItems,
	})
	require.NoError(t, err)

	run, err := hr.StartOffboarding(ctx, actor, emp.ID)
	require.NoError(t, err)
	for _, item := range offboardingItems {
		run, err = hr.CompleteStep(ctx, actor, run.ID, item.ID, "admin@acme.test")
		require.NoError(t, err)
	}
	require.Equal(t, "completed", run.Status)

	// Repeat every completion path a retry could take.
	for _, item := range offboardingItems {
		_, err = hr.CompleteStep(ctx, actor, run.ID, item.ID, "admin@acme.test")
		require.NoError(t, err, "re-completing step %s must not fail", item.ID)
	}
	_, err = hr.UpdateChecklistRun(ctx, actor, run.ID, vakthr.UpdateChecklistRunInput{
		CompletedItems: []string{"off-1", "off-2", "off-3"}, Status: "completed",
	})
	require.NoError(t, err, "re-closing an already-closed run must not fail")

	var evidence int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM ck_evidence WHERE org_id = $1::uuid
		   AND source = 'hr_checklist_completed'`, orgID).Scan(&evidence))
	assert.Equal(t, 1, evidence, "repeated completion must not duplicate the evidence row")

	var members int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM org_members WHERE user_id = $1::uuid AND org_id = $2::uuid`,
		userID, orgID).Scan(&members))
	assert.Equal(t, 0, members, "org membership stays removed after a repeated completion")
}

// TestOffboardingRunCompletion_NoEvidenceWhenRevocationFails is the truth
// requirement: if the revocation does not fully happen, no approved evidence may
// be published and the run must stay open.
//
// The failure is injected without mocks by leaving the SessionRevoker unwired
// while a platform account exists. That is a real, reachable degradation: only
// the refresh rows can be deleted, pw_version cannot be bumped, so the access
// token would survive. Two thirds of a revocation is not a revocation.
func TestOffboardingRunCompletion_NoEvidenceWhenRevocationFails(t *testing.T) {
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx := context.Background()

	const email = "halfway@acme.test"

	var userID, roleID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email, display_name, is_active) VALUES ($1, 'Half', TRUE)
		 RETURNING id::text`, email).Scan(&userID))
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT id::text FROM roles ORDER BY name LIMIT 1`).Scan(&roleID))
	_, err := pool.Exec(ctx,
		`INSERT INTO org_members (org_id, user_id, role_id) VALUES ($1::uuid, $2::uuid, $3::uuid)`,
		orgID, userID, roleID)
	require.NoError(t, err)

	// Deliberately no WithSessionRevoker.
	hr := vakthr.NewServiceFromPool(pool).
		WithEvidenceWriter(vaktcomply.NewHREvidenceWriter(pool))
	actor := vakthr.Actor{OrgID: orgID, UserID: userID, UserEmail: "admin@acme.test"}

	emp, err := hr.CreateEmployee(ctx, actor, vakthr.CreateEmployeeInput{
		FirstName: "Hal", LastName: "Fway", Email: email,
	})
	require.NoError(t, err)
	_, err = hr.CreateChecklist(ctx, actor, vakthr.CreateChecklistInput{
		Type: "offboarding", Name: "Standard-Offboarding", Items: offboardingItems,
	})
	require.NoError(t, err)

	run, err := hr.StartOffboarding(ctx, actor, emp.ID)
	require.NoError(t, err)
	runID := run.ID
	for i, item := range offboardingItems {
		updated, sErr := hr.CompleteStep(ctx, actor, runID, item.ID, "admin@acme.test")
		if i < len(offboardingItems)-1 {
			require.NoError(t, sErr, "step %s is not the closing one", item.ID)
			require.NotEqual(t, "completed", updated.Status)
			continue
		}
		// The closing step must be refused, because closing would publish a claim.
		require.Error(t, sErr, "the closing step must fail while the revocation cannot complete")
		assert.Contains(t, sErr.Error(), "access revocation failed")
	}

	// The step tick itself is still recorded — the step WAS done, only the run
	// must not close on a claim that did not come true.
	events, err := hr.ListRunEvents(ctx, orgID, runID)
	require.NoError(t, err)
	assert.Len(t, events, len(offboardingItems),
		"every ticked step stays recorded even though the run could not be closed")

	// The run stays open, so it is visibly unfinished instead of falsely closed.
	reloaded, err := hr.GetChecklistRun(ctx, orgID, runID)
	require.NoError(t, err)
	assert.NotEqual(t, "completed", reloaded.Status,
		"a run whose revocation failed must stay open")

	// No evidence at all — an evidence row without the deed is the defect itself.
	var evidence int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM ck_evidence WHERE org_id = $1::uuid
		   AND source = 'hr_checklist_completed'`, orgID).Scan(&evidence))
	assert.Equal(t, 0, evidence,
		"no completion evidence may be written when the access revocation did not fully happen")

	// The employee is not stamped terminated either.
	var empStatus string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT status FROM hr_employees WHERE org_id = $1::uuid AND id = $2::uuid`,
		orgID, emp.ID).Scan(&empStatus))
	assert.Equal(t, "offboarding", empStatus)

	// The membership row survives so a retry can still resolve the user and bump
	// pw_version. Deleting it here would make every retry report success while the
	// access token silently stayed valid for the rest of its TTL.
	var members int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM org_members WHERE user_id = $1::uuid AND org_id = $2::uuid`,
		userID, orgID).Scan(&members))
	assert.Equal(t, 1, members,
		"a failed revocation must keep the membership row, otherwise the retry cannot finish the job")
}

// TestOnboardingRunCompletion_DoesNotRevokeAccess guards the other direction:
// the gate must not over-reach. A finished onboarding run must leave the freshly
// provisioned account alone.
func TestOnboardingRunCompletion_DoesNotRevokeAccess(t *testing.T) {
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx := context.Background()

	const email = "joining@acme.test"
	key := mustKeyIntegration(t)

	var userID, roleID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email, display_name, is_active) VALUES ($1, 'Join', TRUE)
		 RETURNING id::text`, email).Scan(&userID))
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT id::text FROM roles ORDER BY name LIMIT 1`).Scan(&roleID))
	_, err := pool.Exec(ctx,
		`INSERT INTO org_members (org_id, user_id, role_id) VALUES ($1::uuid, $2::uuid, $3::uuid)`,
		orgID, userID, roleID)
	require.NoError(t, err)

	authSvc := auth.NewService(pool, nil, key)
	hr := vakthr.NewServiceFromPool(pool).
		WithSessionRevoker(authSvc).
		WithEvidenceWriter(vaktcomply.NewHREvidenceWriter(pool))
	actor := vakthr.Actor{OrgID: orgID, UserID: userID, UserEmail: "admin@acme.test"}

	emp, err := hr.CreateEmployee(ctx, actor, vakthr.CreateEmployeeInput{
		FirstName: "Jo", LastName: "Ining", Email: email,
	})
	require.NoError(t, err)
	onItems := []vakthr.ChecklistItem{
		{ID: "ob-1", Label: "IT-Zugang einrichten", Required: true},
		{ID: "ob-2", Label: "Sicherheitsunterweisung", Required: true},
	}
	_, err = hr.CreateChecklist(ctx, actor, vakthr.CreateChecklistInput{
		Type: "onboarding", Name: "Standard-Onboarding", Items: onItems,
	})
	require.NoError(t, err)

	run, err := hr.StartOnboarding(ctx, actor, emp.ID)
	require.NoError(t, err)
	for _, item := range onItems {
		run, err = hr.CompleteStep(ctx, actor, run.ID, item.ID, "admin@acme.test")
		require.NoError(t, err)
	}
	require.Equal(t, "completed", run.Status)

	var members int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM org_members WHERE user_id = $1::uuid AND org_id = $2::uuid`,
		userID, orgID).Scan(&members))
	assert.Equal(t, 1, members, "a completed onboarding run must not revoke anything")

	var pwVersion int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT pw_version FROM users WHERE id = $1::uuid`, userID).Scan(&pwVersion))
	assert.EqualValues(t, 0, pwVersion, "onboarding must not invalidate the new employee's token")

	var empStatus string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT status FROM hr_employees WHERE org_id = $1::uuid AND id = $2::uuid`,
		orgID, emp.ID).Scan(&empStatus))
	assert.Equal(t, "active", empStatus, "onboarding must not terminate the employee")
}
