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
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoginHandler_AttackerCannotLockOutVictim is the real S121-F4 (F1-Auth)
// regression: it drives the actual POST /auth/login handler, because that is
// where the account-DoS lived. A service-level test would be vacuous here — the
// per-(IP, email) counter was always IP-scoped; the bug was that the handler
// *additionally* consulted a pure per-email counter and locked the account for
// everyone after 5 failures from anywhere.
//
// No seeded user is required: the lockout checks run BEFORE the credential check,
// so a locked account answers 429 regardless of the password. That lets us assert
// the exact property we care about — after an attacker floods failures for the
// victim's address, a request for that same address from a DIFFERENT IP must come
// back 401 (credentials rejected), NOT 429 (account locked).
func TestLoginHandler_AttackerCannotLockOutVictim(t *testing.T) {
	dbURL := os.Getenv("VAKT_DB_URL")
	redisURL := os.Getenv("VAKT_REDIS_URL")
	if dbURL == "" || redisURL == "" {
		t.Skip("VAKT_DB_URL and VAKT_REDIS_URL required — this drives the real login handler (set in CI)")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err)
	defer pool.Close()

	opt, err := redis.ParseURL(redisURL)
	require.NoError(t, err)
	rdb := redis.NewClient(opt)
	defer func() { _ = rdb.Close() }()
	require.NoError(t, rdb.Ping(ctx).Err())

	key, err := GenerateSymmetricKey(
		"0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
	require.NoError(t, err)

	svc := NewService(pool, rdb, key)
	h := NewHandler(svc, nil)

	e := echo.New()
	e.POST("/auth/login", h.Login)

	const (
		attackerIP  = "203.0.113.9"
		victimIP    = "198.51.100.7"
		victimEmail = "s121-dos-handler-victim@example.org"
	)

	// Start clean and don't leak counters into other tests.
	keys := []string{
		loginIPEmailFailKey(attackerIP, victimEmail),
		loginIPEmailFailKey(victimIP, victimEmail),
		loginIPFailKey(attackerIP),
		loginIPFailKey(victimIP),
	}
	_ = rdb.Del(ctx, keys...).Err()
	t.Cleanup(func() { _ = rdb.Del(context.Background(), keys...).Err() })

	login := func(ip string) *httptest.ResponseRecorder {
		body := fmt.Sprintf(`{"email":%q,"password":"definitely-the-wrong-password"}`, victimEmail)
		req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		req.Header.Set(echo.HeaderXRealIP, ip) // drive c.RealIP()
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}

	codeOf := func(rec *httptest.ResponseRecorder) string {
		var out map[string]string
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return out["code"]
	}

	// The attacker floods failures for the victim's address — well past the old
	// threshold of 5 and past the (IP, email) threshold of 10.
	var attackerFinal *httptest.ResponseRecorder
	for i := 0; i < ipEmailLockoutFailMax+3; i++ {
		attackerFinal = login(attackerIP)
	}

	// The attacker locks themselves out of that account: brute-force is still stopped.
	assert.Equal(t, http.StatusTooManyRequests, attackerFinal.Code,
		"the attacking IP must end up locked out of the targeted account")
	assert.Equal(t, "ACCOUNT_LOCKED", codeOf(attackerFinal))

	// The victim, from their own IP, must still reach the credential check.
	victimRec := login(victimIP)
	assert.NotEqual(t, http.StatusTooManyRequests, victimRec.Code,
		"ACCOUNT DoS REGRESSION: an attacker locked the victim out of their own account "+
			"from a different IP. The login lockout must be keyed on (IP, email) only — "+
			"no IP-agnostic per-email lockout.")
	assert.Equal(t, http.StatusUnauthorized, victimRec.Code,
		"the victim's request must be judged on its credentials (401), not blocked (429)")
	assert.Equal(t, "AUTH_INVALID_CREDENTIALS", codeOf(victimRec))
}
