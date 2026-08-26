// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth

// AUTH-001 / R1-W6A-N1 / R1-W7A-N3 — what logout owes the caller.
//
// Security contract: a logout that answers 200 has revoked BOTH halves of the
// session — the access token (deny-list) and every refresh session of that user
// (refresh_sessions rows, Redis keys, pw_version bump). A logout that could not
// do that says so, rather than answering 200 over a session that is still live.
//
// ── Why this file was rewritten ─────────────────────────────────────────────
//
// It used to open with almost exactly that sentence and then check none of it.
// Five tests, all green, none of which called Logout: two compared
// refreshRedisKey() against a "refresh:"+hash literal, one asserted that a
// dead Redis client returns an error (a property of go-redis, not of this
// codebase), one asserted that RevokeAllSessions errors on a nil pool — and one
// declared `const expectedSQL = "DELETE FROM refresh_sessions …"` inside the
// test and then made assertions about that constant. That last one is the
// clearest case: it would have stayed green if the production query had been
// deleted outright, because the string it inspected was its own.
//
// Meanwhile the defect the header describes sat one call away and untouched:
// Logout read its subject from c.Get("user_id"), the route is mounted without
// auth middleware, so the value was always empty and RevokeAllSessions ran
// never. Five green tests over the exact hole they claimed to guard.
//
// What is kept: the key-format agreement between the storage and the revocation
// path is a genuine invariant with a genuine failure mode (revocation deletes
// keys nobody stored, silently). It is one test now, not three.
//
// What is added: the wiring itself, which is the part that was missing. The
// full end-to-end proof — log in, log out, then find BOTH tokens dead against a
// real Postgres and a real Redis — needs both of those and lives in
// internal/integration_test/logout_revokes_both_tokens_real_test.go.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aidanwoods.dev/go-paseto"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLogout_RevokeAllSessions_KeyFormat verifies that the Redis keys deleted
// by RevokeAllSessions match the "refresh:<sha256>" format that issueTokenPair
// uses when storing refresh tokens.
//
// A mismatch (different prefix, different hash encoding) would mean revocation
// deletes keys nobody ever wrote while the real ones survive — and it would
// look identical to a working revocation from the outside.
func TestLogout_RevokeAllSessions_KeyFormat(t *testing.T) {
	rawToken := "aabbccdd1122334455667788aabbccdd"

	storeKey := refreshRedisKey(rawToken)         // what issueTokenPair writes
	revokeKey := "refresh:" + sha256Hex(rawToken) // what RevokeAllSessions deletes

	assert.Equal(t, storeKey, revokeKey,
		"revocation key must match the storage key used by issueTokenPair; "+
			"a mismatch means stolen refresh tokens survive a logout")
	assert.Len(t, sha256Hex(rawToken), 64,
		"token_hash must be a 64-char hex-encoded SHA-256 digest")
}

// newLogoutTestHandler builds a Handler whose Service carries a real Paseto key
// but no database — deliberately. A nil pool makes RevokeAllSessions fail, and
// the point of these tests is what Logout DOES with that failure. Before the
// fix it did nothing at all, because it never got that far.
func newLogoutTestHandler(t *testing.T) (*Handler, paseto.V4SymmetricKey) {
	t.Helper()
	key := paseto.NewV4SymmetricKey()
	return &Handler{service: &Service{key: key, redis: dialFailingRedis(t)}}, key
}

func postLogout(t *testing.T, h *Handler, token string) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	// No auth middleware — this is the production mount (api.Group("/auth", …)),
	// and reproducing it is the whole point: with middleware in the chain the
	// bug under test cannot occur.
	require.NoError(t, h.Logout(e.NewContext(req, rec)))
	return rec
}

// TestLogout_ReachesSessionRevocationWithoutAuthMiddleware is the regression
// guard for R1-W6A-N1. It pins the wiring, not the outcome: on a mount with no
// auth middleware, Logout must still identify the subject and attempt the
// session revocation.
//
// The proof that it got there is the failure itself. The Service has a nil DB,
// so RevokeAllSessions cannot succeed, so a handler that reached it must answer
// LOGOUT_REVOCATION_FAILED. The old handler skipped the whole block (empty
// user_id) and answered 200 — which is exactly what this test would have caught.
func TestLogout_ReachesSessionRevocationWithoutAuthMiddleware(t *testing.T) {
	h, key := newLogoutTestHandler(t)
	token, err := IssueAccessToken(key, Claims{
		UserID: "11111111-1111-1111-1111-111111111111",
		OrgID:  "22222222-2222-2222-2222-222222222222",
		Roles:  []string{"Admin"},
	})
	require.NoError(t, err)

	rec := postLogout(t, h, token)

	require.NotEqual(t, http.StatusOK, rec.Code,
		"a logout that could revoke neither the token nor the sessions must not answer 200")
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)

	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "LOGOUT_REVOCATION_FAILED", body["code"],
		"the session revocation must be attempted on the public mount — an empty "+
			"subject used to skip it silently and answer 'logged out'")
}

// TestLogout_ExpiredTokenStillIdentifiesItsSubject covers the case the public
// mount exists for. The access token lives an hour, the refresh session thirty
// days, so the ordinary logout — someone returning to an idle tab — presents an
// EXPIRED token over a live session. If expiry blocked subject resolution, the
// long-lived session would survive precisely the logout that matters most.
func TestLogout_ExpiredTokenStillIdentifiesItsSubject(t *testing.T) {
	key := paseto.NewV4SymmetricKey()
	const userID = "33333333-3333-3333-3333-333333333333"

	expired, err := IssueAccessTokenWithTTL(key, Claims{UserID: userID, OrgID: "org"}, -time.Hour)
	require.NoError(t, err)

	// The strict parser refuses it — that is correct and must stay correct.
	_, strictErr := ParseAccessToken(key, expired)
	require.Error(t, strictErr, "an expired token must never authorise anything")

	// The revocation parser still names the subject whose sessions to destroy.
	subject, err := ParseTokenSubjectForRevocation(key, expired)
	require.NoError(t, err, "an expired token must still be able to end its own session")
	assert.Equal(t, userID, subject)
}

// TestLogout_ForgedTokenIsRefusedNotConfirmed guards the other edge: a token
// this server did not mint names no session, so there is nothing to revoke.
// Answering "logged out" would claim work that never happened.
func TestLogout_ForgedTokenIsRefusedNotConfirmed(t *testing.T) {
	h, _ := newLogoutTestHandler(t)

	// Authentic-looking, minted under a different key.
	foreign, err := IssueAccessToken(paseto.NewV4SymmetricKey(), Claims{
		UserID: "44444444-4444-4444-4444-444444444444", OrgID: "org",
	})
	require.NoError(t, err)

	rec := postLogout(t, h, foreign)

	assert.Equal(t, http.StatusUnauthorized, rec.Code,
		"a token from a foreign key must not be answered as a completed logout")
	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "AUTH_INVALID_TOKEN", body["code"])
}

// TestLogout_ClearsCookiesEvenWhenRevocationFails pins the ordering trap:
// SetCookie only stages a header, and the first c.JSON flushes them. Clearing
// the cookies in a defer would compile, read as careful, and drop both
// Set-Cookie lines on every response.
//
// The local teardown is deliberately unconditional — a browser that asked to
// leave should stop presenting the token even when the server could not revoke
// it. The 503 carries the rest of the truth.
func TestLogout_ClearsCookiesEvenWhenRevocationFails(t *testing.T) {
	h, key := newLogoutTestHandler(t)
	token, err := IssueAccessToken(key, Claims{
		UserID: "55555555-5555-5555-5555-555555555555", OrgID: "org",
	})
	require.NoError(t, err)

	rec := postLogout(t, h, token)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)

	cookies := strings.Join(rec.Header().Values("Set-Cookie"), " | ")
	assert.Contains(t, cookies, "access_token=",
		"the access_token cookie must be expired even on the failure path")
	assert.True(t,
		strings.Contains(strings.ToLower(cookies), "max-age=0"),
		"cookie must be expired immediately; got: %s", cookies)
}

// TestRevokeToken_ErrorsWhenNoSinkAcceptsIt is the regression guard for
// R1-W7A-N3. RevokeToken writes to two sinks and used to `return nil`
// unconditionally, so the logout handler could not distinguish a persisted
// revocation from a lost one — and answered 200 for both.
//
// Here neither sink can accept it: Redis is unreachable and no PG fallback is
// wired. The token stays valid for the rest of its hour, and that must be
// reported, not logged.
func TestRevokeToken_ErrorsWhenNoSinkAcceptsIt(t *testing.T) {
	svc := &Service{redis: dialFailingRedis(t), denyFall: nil}

	err := svc.RevokeToken(context.Background(), "some-raw-token")

	require.Error(t, err, "a revocation that reached neither Redis nor the PG fallback is not a success")
	assert.Contains(t, err.Error(), "no sink accepted",
		"the error must say what actually failed, not just that something did")
}

// TestRevokeAllSessions_NilDBReturnsError keeps the one useful assertion from
// the old file: a degenerate wiring must produce an error, not a panic. It
// matters more now, because the Logout handler turns that error into a 503
// instead of discarding it.
func TestRevokeAllSessions_NilDBReturnsError(t *testing.T) {
	svc := &Service{redis: nil, db: nil}

	err := svc.RevokeAllSessions(context.Background(), "00000000-0000-0000-0000-000000000001")

	require.Error(t, err, "nil DB must return an error")
	assert.Contains(t, err.Error(), "revoke sessions",
		"error message must identify the failing operation")
}
