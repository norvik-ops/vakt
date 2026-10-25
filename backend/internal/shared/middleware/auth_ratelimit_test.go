// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// memCounter is an in-memory stand-in for incrWithTTL. The window is not
// simulated: every test runs well inside one minute, exactly like the attack
// the limits are meant to stop.
type memCounter struct {
	mu     sync.Mutex
	counts map[string]int64
	err    error
}

func newMemCounter() *memCounter { return &memCounter{counts: map[string]int64{}} }

func (m *memCounter) incr(_ context.Context, key string, _ time.Duration) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return 0, m.err
	}
	m.counts[key]++
	return m.counts[key], nil
}

func (m *memCounter) keys() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.counts))
	for k := range m.counts {
		out = append(out, k)
	}
	return out
}

// authEcho mounts the limiter the way cmd/api/routes.go does: as per-route
// middleware on the credential routes inside /api/v1/auth. The login handler
// binds the body like the real one and echoes the e-mail back, which proves the
// limiter left the body readable.
func authEcho(incr counterFunc) *echo.Echo {
	e := echo.New()
	e.IPExtractor = echo.ExtractIPDirect()
	mw := authRateLimit(incr)
	g := e.Group("/api/v1/auth")
	g.POST("/login", func(c echo.Context) error {
		var body struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := c.Bind(&body); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, body)
	}, mw)
	g.POST("/register", func(c echo.Context) error { return c.NoContent(http.StatusOK) }, mw)
	g.POST("/password-reset/request", func(c echo.Context) error { return c.NoContent(http.StatusOK) }, mw)
	return e
}

func post(t *testing.T, e *echo.Echo, path, ip, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.RemoteAddr = ip + ":40000"
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func loginBody(email string) string {
	return fmt.Sprintf(`{"email":%q,"password":"irrelevant-password"}`, email)
}

func TestAuthRateLimitConstants(t *testing.T) {
	// R1-KW-4 decision: login 5/min per (IP, e-mail) + 30/min per IP;
	// register and password reset stay at 5/min per IP (S45-5).
	assert.Equal(t, 5, authRLLimit)
	assert.Equal(t, 5, loginPairLimit)
	assert.Equal(t, 30, loginIPLimit)
	assert.Equal(t, time.Minute, authRLWindow)
}

func TestLoginRateLimit_sixthLoginOfSamePairIs429(t *testing.T) {
	e := authEcho(newMemCounter().incr)
	for i := 1; i <= 5; i++ {
		rec := post(t, e, "/api/v1/auth/login", "203.0.113.7", loginBody("alice@example.com"))
		require.Equal(t, http.StatusOK, rec.Code, "login %d", i)
	}
	rec := post(t, e, "/api/v1/auth/login", "203.0.113.7", loginBody("alice@example.com"))
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.Contains(t, rec.Body.String(), "AUTH_RATE_LIMIT")
}

func TestLoginRateLimit_otherUserBehindSameIPKeepsLoggingIn(t *testing.T) {
	e := authEcho(newMemCounter().incr)
	const nat = "198.51.100.20"
	for i := 0; i < 6; i++ {
		post(t, e, "/api/v1/auth/login", nat, loginBody("alice@example.com"))
	}
	// Alice is limited, but Bob behind the same office NAT is not.
	for i := 1; i <= 5; i++ {
		rec := post(t, e, "/api/v1/auth/login", nat, loginBody("bob@example.com"))
		assert.Equal(t, http.StatusOK, rec.Code, "bob login %d", i)
	}
	// Further colleagues, one login each, up to the IP cap of 30 in total.
	// So far: 6 (alice) + 5 (bob) = 11.
	for i := 12; i <= loginIPLimit; i++ {
		rec := post(t, e, "/api/v1/auth/login", nat, loginBody(fmt.Sprintf("user%d@example.com", i)))
		assert.Equal(t, http.StatusOK, rec.Code, "login %d of the IP", i)
	}
	rec := post(t, e, "/api/v1/auth/login", nat, loginBody("late@example.com"))
	assert.Equal(t, http.StatusTooManyRequests, rec.Code, "login 31 of the IP hits the IP cap")
}

func TestLoginRateLimit_thirtyFirstLoginOverManyEmailsIs429(t *testing.T) {
	e := authEcho(newMemCounter().incr)
	for i := 1; i <= loginIPLimit; i++ {
		rec := post(t, e, "/api/v1/auth/login", "192.0.2.50", loginBody(fmt.Sprintf("victim%d@example.com", i)))
		require.Equal(t, http.StatusOK, rec.Code, "login %d", i)
	}
	rec := post(t, e, "/api/v1/auth/login", "192.0.2.50", loginBody("victim31@example.com"))
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)

	// A different IP is not affected by that cap.
	rec = post(t, e, "/api/v1/auth/login", "192.0.2.51", loginBody("victim31@example.com"))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestLoginRateLimit_emailCaseAndWhitespaceCountAsSameAccount(t *testing.T) {
	e := authEcho(newMemCounter().incr)
	variants := []string{"alice@example.com", "Alice@Example.com", " ALICE@EXAMPLE.COM", "alice@example.com ", "aLiCe@example.COM"}
	for i, v := range variants {
		rec := post(t, e, "/api/v1/auth/login", "203.0.113.9", loginBody(v))
		require.Equal(t, http.StatusOK, rec.Code, "variant %d %q", i, v)
	}
	rec := post(t, e, "/api/v1/auth/login", "203.0.113.9", loginBody("ALICE@example.com"))
	assert.Equal(t, http.StatusTooManyRequests, rec.Code, "the sixth spelling of one address must hit the pair limit")
}

func TestLoginRateLimit_bodyStaysReadableForHandler(t *testing.T) {
	e := authEcho(newMemCounter().incr)
	rec := post(t, e, "/api/v1/auth/login", "203.0.113.10", `{"email":"Carol@Example.com","password":"secret-value-123"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	var got struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	// The handler sees the original bytes — not the normalised e-mail.
	assert.Equal(t, "Carol@Example.com", got.Email)
	assert.Equal(t, "secret-value-123", got.Password)
}

func TestLoginRateLimit_oversizedBodyStaysReadableAndFallsBack(t *testing.T) {
	var seen int
	e := echo.New()
	e.IPExtractor = echo.ExtractIPDirect()
	counter := newMemCounter()
	e.POST("/api/v1/auth/login", func(c echo.Context) error {
		b, err := io.ReadAll(c.Request().Body)
		require.NoError(t, err)
		seen = len(b)
		return c.NoContent(http.StatusOK)
	}, authRateLimit(counter.incr))

	big := `{"email":"dave@example.com","pad":"` + strings.Repeat("x", loginBodyPeek) + `"}`
	rec := post(t, e, "/api/v1/auth/login", "203.0.113.11", big)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, len(big), seen, "the handler must receive the complete body, including the part beyond the peek")
	assert.Contains(t, counter.keys(), "auth_rl:login_noemail:203.0.113.11")
}

// Without a readable e-mail the login counts like before R1-KW-4 (5/min per
// IP). Otherwise a form-encoded body — which Echo's binder accepts but the
// limiter does not parse — would get the IP cap of 30 per account.
func TestLoginRateLimit_unreadableEmailCountsConservatively(t *testing.T) {
	e := authEcho(newMemCounter().incr)
	for i := 1; i <= loginNoEmailLimit; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
			strings.NewReader("email=alice%40example.com&password=x"))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
		req.RemoteAddr = "203.0.113.12:40000"
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, "form login %d", i)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
		strings.NewReader("email=alice%40example.com&password=x"))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	req.RemoteAddr = "203.0.113.12:40000"
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)

	for _, body := range []string{``, `{}`, `{"email":""}`, `{"email":"no-at-sign"}`, `not json`} {
		assert.False(t, func() bool {
			c := echo.New().NewContext(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)), httptest.NewRecorder())
			_, ok := loginEmailKey(c)
			return ok
		}(), "body %q must not yield an e-mail key", body)
	}
}

func TestLoginRateLimit_emailNeverInClearTextInKeys(t *testing.T) {
	counter := newMemCounter()
	e := authEcho(counter.incr)
	post(t, e, "/api/v1/auth/login", "203.0.113.13", loginBody("Secret.Person@Example.com"))
	for _, k := range counter.keys() {
		assert.NotContains(t, strings.ToLower(k), "secret.person", "key %q leaks the e-mail", k)
		assert.NotContains(t, strings.ToLower(k), "example.com", "key %q leaks the e-mail", k)
	}
	assert.Contains(t, counter.keys(), "auth_rl:login_ip:203.0.113.13")
}

func TestAuthRateLimit_registerAndResetStayAtFivePerIP(t *testing.T) {
	for _, path := range []string{"/api/v1/auth/register", "/api/v1/auth/password-reset/request"} {
		t.Run(path, func(t *testing.T) {
			e := authEcho(newMemCounter().incr)
			for i := 1; i <= 5; i++ {
				// Different e-mails do not buy extra attempts here.
				rec := post(t, e, path, "203.0.113.14", loginBody(fmt.Sprintf("u%d@example.com", i)))
				require.Equal(t, http.StatusOK, rec.Code, "request %d", i)
			}
			rec := post(t, e, path, "203.0.113.14", loginBody("u6@example.com"))
			assert.Equal(t, http.StatusTooManyRequests, rec.Code)
		})
	}
}

// Fail-open on a Redis error is unchanged — for login and for the other routes.
func TestAuthRateLimit_failsOpenOnCounterError(t *testing.T) {
	counter := newMemCounter()
	counter.err = fmt.Errorf("redis: connection refused")
	e := authEcho(counter.incr)
	for i := 0; i < 40; i++ {
		require.Equal(t, http.StatusOK, post(t, e, "/api/v1/auth/login", "203.0.113.15", loginBody("a@example.com")).Code)
		require.Equal(t, http.StatusOK, post(t, e, "/api/v1/auth/register", "203.0.113.15", loginBody("a@example.com")).Code)
	}
}

// The same with a real go-redis client whose server is unreachable: this is the
// production wiring (AuthRateLimit → incrWithTTL), not the in-memory stand-in.
func TestAuthRateLimit_failsOpenWithUnreachableRedis(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", MaxRetries: -1, DialTimeout: 100 * time.Millisecond})
	defer func() { _ = rdb.Close() }()

	e := echo.New()
	e.IPExtractor = echo.ExtractIPDirect()
	e.POST("/api/v1/auth/login", func(c echo.Context) error { return c.NoContent(http.StatusOK) }, AuthRateLimit(rdb))
	for i := 0; i < 7; i++ {
		assert.Equal(t, http.StatusOK, post(t, e, "/api/v1/auth/login", "203.0.113.16", loginBody("a@example.com")).Code)
	}
}

// SkipLogin keeps the auth group's general IP limiter off POST /auth/login and
// on every other auth route.
func TestSkipLogin_onlyLoginBypassesTheWrappedLimiter(t *testing.T) {
	var hits int
	counting := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			hits++
			return next(c)
		}
	}
	e := echo.New()
	g := e.Group("/api/v1/auth", SkipLogin(counting))
	ok := func(c echo.Context) error { return c.NoContent(http.StatusOK) }
	g.POST("/login", ok)
	g.POST("/refresh", ok)
	g.POST("/register", ok)
	g.GET("/login", ok) // not the password login

	post(t, e, "/api/v1/auth/login", "203.0.113.17", loginBody("a@example.com"))
	assert.Equal(t, 0, hits, "POST /auth/login must skip the group limiter")

	post(t, e, "/api/v1/auth/refresh", "203.0.113.17", `{}`)
	post(t, e, "/api/v1/auth/register", "203.0.113.17", `{}`)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/auth/login", nil))
	assert.Equal(t, 3, hits, "every other auth route keeps the group limiter")
}

func postCT(t *testing.T, e *echo.Echo, ip, contentType, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, contentType)
	req.RemoteAddr = ip + ":40000"
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

// Subreview PR #179: the login handler binds by Content-Type (c.Bind), the
// limiter used to parse every body as JSON. A body that is valid JSON with a
// fresh e-mail AND valid XML carrying the victim's credentials, sent as
// application/xml, got a new (IP, e-mail) budget per request while the handler
// tried the victim's password. Non-JSON bodies must land in the no-e-mail bucket.
func TestLoginRateLimit_jsonXMLPolyglotCountsInNoEmailBucket(t *testing.T) {
	e := authEcho(newMemCounter().incr)
	poly := func(i int) string {
		return fmt.Sprintf(`{"email":"random%d@x.de","p":"<a><Email>victim@x.de</Email><Password>secretpass1</Password></a>"}`, i)
	}
	for i := 1; i <= loginNoEmailLimit; i++ {
		rec := postCT(t, e, "203.0.113.20", echo.MIMEApplicationXML, poly(i))
		require.NotEqual(t, http.StatusTooManyRequests, rec.Code, "attempt %d", i)
	}
	rec := postCT(t, e, "203.0.113.20", echo.MIMEApplicationXML, poly(99))
	assert.Equal(t, http.StatusTooManyRequests, rec.Code, "6th XML-polyglot attempt must hit the 5/min no-e-mail limit")
}

func TestLoginRateLimit_jsonWithCharsetStillCountsPerPair(t *testing.T) {
	counter := newMemCounter()
	e := authEcho(counter.incr)
	rec := postCT(t, e, "203.0.113.21", "application/json; charset=utf-8", loginBody("alice@example.com"))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.NotContains(t, counter.keys(), "auth_rl:login_noemail:203.0.113.21")
	var pair int
	for _, k := range counter.keys() {
		if strings.HasPrefix(k, "auth_rl:login:203.0.113.21:") {
			pair++
		}
	}
	assert.Equal(t, 1, pair, "JSON with charset parameter must use the (IP, e-mail) bucket")
}

func TestLoginRateLimit_formBodyCountsInNoEmailBucket(t *testing.T) {
	counter := newMemCounter()
	e := authEcho(counter.incr)
	postCT(t, e, "203.0.113.22", echo.MIMEApplicationForm, "email=alice%40example.com&password=x")
	assert.Contains(t, counter.keys(), "auth_rl:login_noemail:203.0.113.22")
}
