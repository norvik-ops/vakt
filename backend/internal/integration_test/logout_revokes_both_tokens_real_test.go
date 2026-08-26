//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"golang.org/x/crypto/bcrypt"

	"github.com/matharnica/vakt/internal/auth"
	"github.com/matharnica/vakt/internal/config"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
)

// R1-W6A-N1 + R1-W7A-N3 — the end-to-end guard for what logging out means.
//
// A session has two halves with very different lifetimes: the Paseto access
// token (1 h) and the refresh session (30 days). Logout was only ever ending the
// first. The second half's revocation was written, reviewed, unit-tested — and
// dead: the handler took its subject from c.Get("user_id"), the route is mounted
// on the PUBLIC auth group with no middleware to fill that in, so the value was
// always empty and RevokeAllSessions ran never. Nothing failed. The response
// said "logged out" and the 30-day session went on issuing access tokens.
//
// That is not reachable by any test without both a real Postgres and a real
// Redis, which is why it survived: the refresh token lives in Redis, the session
// row and pw_version live in Postgres, and the claim under test is that after
// one logout NEITHER token works any more. A mock of either end would have been
// asked the question and would have answered whatever it was told to.
//
// The test deliberately drives the REAL route registration (auth.Register on a
// bare group) rather than calling the handler method. The defect lived in the
// mount, not in the handler body — a test that calls h.Logout(ctx) directly
// cannot see it, and would have stayed green throughout.
func TestLogoutRevokesAccessTokenAndRefreshSession(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	pool, rdb, orgID := bootLogoutStack(ctx, t)
	key := mustKeyIntegration(t)
	svc := auth.NewService(pool, rdb, key)

	email, passphrase := seedLoginUser(ctx, t, pool, orgID, "logout-both")

	// ── Baseline, and the proof that the assertions below are not vacuous ────
	// Session A exists only to show that a refresh token DOES work before a
	// logout. Without it, "refresh fails after logout" would also pass for a
	// refresh token that never worked at all.
	sessionA, err := svc.Login(ctx, email, passphrase, "session-a")
	require.NoError(t, err)
	refreshedA, err := svc.Refresh(ctx, sessionA.RefreshToken)
	require.NoError(t, err, "baseline: a fresh refresh token must mint a new access token")
	require.NotEmpty(t, refreshedA.AccessToken)

	// Session B is the one that gets logged out.
	sessionB, err := svc.Login(ctx, email, passphrase, "session-b")
	require.NoError(t, err)

	probe := newTokenProbe(t, key, pool, rdb)
	require.Equal(t, http.StatusOK, probe(sessionB.AccessToken),
		"baseline: the access token must be accepted before the logout")

	var sessionsBefore int
	require.NoError(t, pool.QueryRow(ctx,
		// orgid-lint: global — scoped by user_id, and deliberately cross-org: logout
		// is "logout everywhere", so counting per org would measure the wrong thing.
		`SELECT count(*) FROM refresh_sessions WHERE user_id = (SELECT id FROM users WHERE email = $1)`,
		email).Scan(&sessionsBefore))
	require.Positive(t, sessionsBefore, "baseline: refresh sessions must exist before the logout")

	// ── The logout ───────────────────────────────────────────────────────────
	rec := postLogoutThroughRealRoute(t, svc, sessionB.AccessToken)
	require.Equal(t, http.StatusOK, rec.Code,
		"a logout against a healthy stack must succeed; body: %s", rec.Body.String())

	// ── Both halves must be dead ─────────────────────────────────────────────
	assert.Equal(t, http.StatusUnauthorized, probe(sessionB.AccessToken),
		"the access token must be rejected after the logout")

	_, refreshErr := svc.Refresh(ctx, sessionB.RefreshToken)
	assert.Error(t, refreshErr,
		"THE defect: the 30-day refresh session survived every logout and kept "+
			"minting access tokens for anyone holding it")

	// And the same for the other session of the same user — logout is "logout
	// everywhere", which is what RevokeAllSessions has always claimed to do.
	_, refreshErrA := svc.Refresh(ctx, refreshedA.RefreshToken)
	assert.Error(t, refreshErrA, "every refresh session of the user must be gone, not just the caller's")

	var sessionsAfter int
	require.NoError(t, pool.QueryRow(ctx,
		// orgid-lint: global — same reason as the baseline count above.
		`SELECT count(*) FROM refresh_sessions WHERE user_id = (SELECT id FROM users WHERE email = $1)`,
		email).Scan(&sessionsAfter))
	assert.Zero(t, sessionsAfter, "refresh_sessions rows must be deleted, not merely orphaned in Redis")

	var pwVersion int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT pw_version FROM users WHERE email = $1`, email).Scan(&pwVersion))
	assert.Positive(t, pwVersion,
		"pw_version must be bumped — it is the only thing that kills the stateless "+
			"access tokens already handed out")
}

// TestExpiredAccessTokenCanStillLogOut is the baseline this whole design is
// bent around, so it is asserted rather than assumed.
//
// The route is public precisely so that a session whose access token has run out
// can still end itself, and that is not an edge case: the token lives an hour and
// the refresh session thirty days, so anyone coming back to an idle tab has an
// expired token over a live session. If expiry blocked subject resolution, the
// long-lived half would survive exactly the logout that matters most — the fix
// would have moved the defect rather than closed it.
func TestExpiredAccessTokenCanStillLogOut(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	pool, rdb, orgID := bootLogoutStack(ctx, t)
	key := mustKeyIntegration(t)
	svc := auth.NewService(pool, rdb, key)

	email, passphrase := seedLoginUser(ctx, t, pool, orgID, "logout-expired")
	session, err := svc.Login(ctx, email, passphrase, "idle-tab")
	require.NoError(t, err)

	var userID string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT id::text FROM users WHERE email = $1`, email).Scan(&userID))

	// The same session, one hour later: an authentic token that is past its
	// expiry, which is what the browser would actually present.
	expired, err := auth.IssueAccessTokenWithTTL(key,
		auth.Claims{UserID: userID, OrgID: orgID, Roles: []string{"Admin"}}, -time.Hour)
	require.NoError(t, err)

	probe := newTokenProbe(t, key, pool, rdb)
	require.Equal(t, http.StatusUnauthorized, probe(expired),
		"precondition: the expired token must not open any door")

	rec := postLogoutThroughRealRoute(t, svc, expired)
	require.Equal(t, http.StatusOK, rec.Code,
		"an expired token must still be able to end its own session; body: %s", rec.Body.String())

	_, refreshErr := svc.Refresh(ctx, session.RefreshToken)
	assert.Error(t, refreshErr,
		"and the long-lived half must go with it — this is the case the public mount exists for")
}

// TestLogoutDoesNotClaimSuccessWhenRevocationFails is the error direction.
//
// R1-W7A-N3: RevokeToken returned a hard nil and the handler answered 200 no
// matter what, so a revocation that reached nothing at all was reported to the
// user as a completed logout. The frontend reads this contract directly —
// TopBar.tsx treats >=500 as "revocation unconfirmed" and warns the user, while
// 4xx stays silent — so a failed revocation has to land in the 5xx bucket or the
// warning never fires.
//
// The failure here is real, not injected into the code under test: Postgres is
// healthy and Redis is not. That is the nastiest shape, because it looks like
// success from every angle except the return value — the durable pw_version bump
// lands in Postgres while the Redis mirror keeps the OLD number, and
// checkPwVersion reads Redis first. A reachable Redis holding a stale value
// answers "matches" and waves the supposedly revoked token straight through.
func TestLogoutDoesNotClaimSuccessWhenRevocationFails(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	pool, rdb, orgID := bootLogoutStack(ctx, t)
	key := mustKeyIntegration(t)
	healthy := auth.NewService(pool, rdb, key)

	email, passphrase := seedLoginUser(ctx, t, pool, orgID, "logout-fails")
	session, err := healthy.Login(ctx, email, passphrase, "doomed")
	require.NoError(t, err)

	// Same Postgres, unreachable Redis. Port 1 is reserved and never bound, so
	// the OS refuses the connection immediately (same technique as the unit
	// tests' dialFailingRedis) — a genuine outage, nothing stubbed.
	deadRedis := redis.NewClient(&redis.Options{
		Addr:          "127.0.0.1:1",
		DialTimeout:   100 * time.Millisecond,
		ReadTimeout:   100 * time.Millisecond,
		MaxRetries:    -1,
		DialerRetries: 1,
	})
	defer deadRedis.Close()
	broken := auth.NewService(pool, deadRedis, key)

	rec := postLogoutThroughRealRoute(t, broken, session.AccessToken)

	require.NotEqual(t, http.StatusOK, rec.Code,
		"a logout that could not complete the revocation must not answer 200; body: %s", rec.Body.String())
	assert.GreaterOrEqual(t, rec.Code, 500,
		"the frontend only warns the user on >=500 — a 4xx here would be silently swallowed")

	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "LOGOUT_REVOCATION_FAILED", body["code"])

	// The local teardown still happens: a browser that asked to leave should stop
	// presenting the token even when the server could not revoke it.
	cookies := strings.Join(rec.Header().Values("Set-Cookie"), " | ")
	assert.Contains(t, cookies, "access_token=",
		"the cookie must be cleared even on the failure path")
}

// ── helpers ────────────────────────────────────────────────────────────────

// postLogoutThroughRealRoute drives POST /api/v1/auth/logout through
// auth.Register on a bare group — i.e. the production mount, with no auth
// middleware. Reproducing the mount is the point: the defect was that the
// handler expected a context value only middleware writes, and there is none
// here. Calling h.Logout directly would supply the same empty context and
// still miss it, because it would also skip the registration under test.
func postLogoutThroughRealRoute(t *testing.T, svc *auth.Service, accessToken string) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	auth.Register(e.Group("/api/v1/auth"), auth.NewHandler(svc, &config.Config{}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

// newTokenProbe returns a func that presents a token to the real
// PasetoMiddleware chain (deny-list check + parse + pw_version check) and
// reports the status. This is the only honest way to ask "is this token still
// good?" — the three ways it can die live in three different places.
func newTokenProbe(t *testing.T, key auth.SymmetricKey, pool *pgxpool.Pool, rdb *redis.Client) func(string) int {
	t.Helper()
	e := echo.New()
	g := e.Group("/probe", auth.PasetoMiddleware(key, pool, rdb))
	g.GET("", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	return func(token string) int {
		req := httptest.NewRequest(http.MethodGet, "/probe", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec.Code
	}
}

// seedLoginUser creates an active user with an org membership and returns its
// e-mail plus the passphrase Login will accept. The passphrase is generated at
// run time and never leaves this process — there is no credential literal in
// this file to leak or to trip a secret scanner.
func seedLoginUser(ctx context.Context, t *testing.T, pool *pgxpool.Pool, orgID, label string) (string, string) {
	t.Helper()
	buf := make([]byte, 16)
	_, err := rand.Read(buf)
	require.NoError(t, err)
	passphrase := hex.EncodeToString(buf)

	// MinCost: this test measures revocation, not bcrypt.
	hash, err := bcrypt.GenerateFromPassword([]byte(passphrase), bcrypt.MinCost)
	require.NoError(t, err)

	email := label + "@logout.test"
	var userID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, display_name, is_active)
		VALUES ($1, $2, $3, TRUE)
		RETURNING id::text`, email, string(hash), label).Scan(&userID))
	require.NoError(t, ensureMember(ctx, pool, userID, orgID))
	return email, passphrase
}

// bootLogoutStack starts Postgres + Redis, runs every migration and seeds one
// org. Both containers are torn down via t.Cleanup.
func bootLogoutStack(ctx context.Context, t *testing.T) (*pgxpool.Pool, *redis.Client, string) {
	t.Helper()

	pgC, err := postgres.Run(ctx,
		imagePostgres,
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
	t.Cleanup(func() { _ = pgC.Terminate(context.Background()) })

	redisC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        imageRedis,
			ExposedPorts: []string{"6379/tcp"},
			WaitingFor:   wait.ForListeningPort("6379/tcp").WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = redisC.Terminate(context.Background()) })

	rHost, err := redisC.Host(ctx)
	require.NoError(t, err)
	rPort, err := redisC.MappedPort(ctx, "6379/tcp")
	require.NoError(t, err)
	rdb := redis.NewClient(&redis.Options{Addr: rHost + ":" + rPort.Port()})
	t.Cleanup(func() { _ = rdb.Close() })

	dsn, err := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	require.NoError(t, shareddb.RunMigrations(dsn, migrationsDir(t)))

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	var orgID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO organizations (name, slug) VALUES ('LogoutOrg', 'logout-org')
		RETURNING id::text`).Scan(&orgID))

	return pool, rdb, orgID
}
