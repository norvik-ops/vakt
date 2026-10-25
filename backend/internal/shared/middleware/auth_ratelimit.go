// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

const (
	// authRLLimit is the maximum number of attempts per IP per window on the
	// credential routes other than login (register, password reset).
	// S45-5: 5 req/min per IP.
	authRLLimit = 5
	// authRLWindow is the rolling window over which all auth limits apply.
	authRLWindow = time.Minute

	// loginPairLimit is the login limit per (IP, e-mail) pair (R1-KW-4). A
	// brute force against one account from one address stops at the 6th try,
	// while colleagues behind the same office NAT keep logging in.
	loginPairLimit = 5
	// loginIPLimit caps logins per IP across all e-mail addresses (R1-KW-4), so
	// spraying one password over many accounts does not get 5 tries per account.
	loginIPLimit = 30
	// loginNoEmailLimit applies to login requests whose e-mail cannot be read
	// from the body (see loginRateLimit).
	loginNoEmailLimit = 5

	// loginBodyPeek bounds how much of the login body the limiter reads to find
	// the e-mail. A login body is two short strings; anything longer is not a
	// login a browser sends and falls back to the conservative bucket.
	loginBodyPeek = 8 << 10
)

// counterFunc increments a windowed counter and returns its new value. The
// production implementation is Redis (incrWithTTL); tests pass an in-memory one.
type counterFunc func(ctx context.Context, key string, ttl time.Duration) (int64, error)

// AuthRateLimit returns the Redis-backed limiter for the credential routes.
//
//   - POST …/auth/login (R1-KW-4): 5 per minute per (IP, e-mail) pair plus 30
//     per minute per IP across all e-mails. The per-IP-only limit locked a
//     whole office behind one NAT out from the 6th login of the minute.
//   - every other route it is mounted on (register, password reset): 5 per
//     minute per IP, as before (S45-5).
//
// Unchanged: fails open when Redis is unavailable, and the failure-based
// lockout in the login handler ((IP, e-mail) after 10 failures,
// VAKT_RATELIMIT_IP_MAX per IP) is a separate, additional layer.
//
// Redis keys: "auth_rl:<ip>" (register/reset), "auth_rl:login_ip:<ip>",
// "auth_rl:login:<ip>:<sha256(e-mail)>" and "auth_rl:login_noemail:<ip>".
// The e-mail never appears in clear text in a key or a log line.
func AuthRateLimit(rdb *redis.Client) echo.MiddlewareFunc {
	return authRateLimit(func(ctx context.Context, key string, ttl time.Duration) (int64, error) {
		return incrWithTTL(ctx, rdb, key, ttl)
	})
}

func authRateLimit(incr counterFunc) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if IsLoginRequest(c) {
				return loginRateLimit(c, next, incr)
			}
			ip := c.RealIP()
			key := fmt.Sprintf("auth_rl:%s", ip)
			count, err := incr(c.Request().Context(), key, authRLWindow)
			if err != nil {
				// Fail open: if Redis is unavailable we do not block legitimate users.
				return next(c)
			}
			if count > authRLLimit {
				return authRateLimited(c)
			}
			return next(c)
		}
	}
}

// loginRateLimit applies the two login limits. Both counters are incremented
// on every attempt, so a request rejected by one still counts against the
// other.
func loginRateLimit(c echo.Context, next echo.HandlerFunc, incr counterFunc) error {
	ctx := c.Request().Context()
	ip := c.RealIP()

	ipCount, err := incr(ctx, "auth_rl:login_ip:"+ip, authRLWindow)
	if err != nil {
		return next(c) // fail open, as for every auth limiter
	}

	// Without a readable e-mail the request counts like before R1-KW-4: 5 per
	// minute per IP, in its own bucket. Every body that is not application/json
	// (form, XML, …) lands here: Echo's binder reads the body according to the
	// Content-Type, and only for JSON is the e-mail the limiter would read the
	// one the handler binds. Deliberately not "IP cap only": that would give an
	// attacker who re-encodes the login 30 tries per account instead of 5.
	// Browsers send JSON, so real users never land here.
	pairKey := "auth_rl:login_noemail:" + ip
	pairLimit := int64(loginNoEmailLimit)
	if emailKey, ok := loginEmailKey(c); ok {
		pairKey = "auth_rl:login:" + ip + ":" + emailKey
		pairLimit = loginPairLimit
	}
	pairCount, err := incr(ctx, pairKey, authRLWindow)
	if err != nil {
		return next(c)
	}

	if ipCount > loginIPLimit || pairCount > pairLimit {
		return authRateLimited(c)
	}
	return next(c)
}

// loginEmailKey reads the e-mail from an application/json login body and returns the hex
// SHA-256 of its normalised form (trimmed, lower-cased), so "Admin@X.de " and
// "admin@x.de" share one budget. The body is put back so the handler can bind
// it afterwards. Returns ok=false when there is no usable e-mail.
func loginEmailKey(c echo.Context) (string, bool) {
	req := c.Request()
	if req.Body == nil || req.Body == http.NoBody {
		return "", false
	}
	// Only JSON: the handler binds by Content-Type (c.Bind). Reading a body as
	// JSON that the handler then binds as XML lets a JSON/XML polyglot carry a
	// fresh e-mail for the limiter and the victim's for the handler — a new
	// (IP, e-mail) budget per request (subreview PR #179).
	if mt, _, err := mime.ParseMediaType(req.Header.Get(echo.HeaderContentType)); err != nil || mt != echo.MIMEApplicationJSON {
		return "", false
	}
	original := req.Body
	peek, err := io.ReadAll(io.LimitReader(original, loginBodyPeek+1))
	// Restore before anything else: the handler must see exactly the bytes it
	// would have seen without this middleware, including any unread rest.
	req.Body = replayBody{Reader: io.MultiReader(bytes.NewReader(peek), original), Closer: original}
	if err != nil || len(peek) > loginBodyPeek {
		return "", false
	}

	var body struct {
		Email string `json:"email"`
	}
	if json.Unmarshal(peek, &body) != nil {
		return "", false
	}
	email := strings.ToLower(strings.TrimSpace(body.Email))
	if email == "" || !strings.Contains(email, "@") {
		return "", false
	}
	sum := sha256.Sum256([]byte(email))
	return hex.EncodeToString(sum[:]), true
}

// replayBody serves the peeked bytes followed by the rest of the original
// body, and closes the original body on Close.
type replayBody struct {
	io.Reader
	io.Closer
}

// IsLoginRequest reports whether c is the password login (POST …/auth/login).
// It matches the registered route path, not the raw URL, so it is only true
// after routing — i.e. inside route or group middleware.
func IsLoginRequest(c echo.Context) bool {
	return c.Request().Method == http.MethodPost && strings.HasSuffix(c.Path(), "/auth/login")
}

// SkipLogin wraps mw so it does not run for POST …/auth/login. The auth
// group's general IP limiter (5/min per IP) uses it: login has its own
// per-(IP, e-mail) limit with a per-IP cap in AuthRateLimit (R1-KW-4), and the
// group limiter would otherwise put the 5-per-IP ceiling for a whole NAT back.
func SkipLogin(mw echo.MiddlewareFunc) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		limited := mw(next)
		return func(c echo.Context) error {
			if IsLoginRequest(c) {
				return next(c)
			}
			return limited(c)
		}
	}
}

func authRateLimited(c echo.Context) error {
	return c.JSON(http.StatusTooManyRequests, map[string]string{
		"error": "Too many attempts",
		"code":  "AUTH_RATE_LIMIT",
	})
}

const (
	// totpRLLimit is the maximum number of TOTP verification attempts allowed
	// per IP per totpRLWindow.  5 attempts per 5 minutes matches the TOTP code
	// rotation period and prevents brute-force sweeps without blocking genuine
	// retries caused by clock skew.
	totpRLLimit = 5
	// totpRLWindow is the rolling window over which totpRLLimit is applied.
	totpRLWindow = 5 * time.Minute
)

// TOTPRateLimit returns an Echo middleware that enforces an IP-based rate limit
// of 5 TOTP verification attempts per 5 minutes using Redis as the backing store.
//
// Redis-backed storage ensures the limit is shared across replicas and survives
// process restarts — unlike the Echo in-memory rate limiter which resets on
// every restart and is not effective behind a load balancer.
//
// Fails open when Redis is unavailable so that a Redis outage does not lock
// users out of 2FA-protected endpoints.
func TOTPRateLimit(rdb *redis.Client) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ip := c.RealIP()
			key := fmt.Sprintf("totp_rl:%s", ip)
			ctx := c.Request().Context()

			count, err := incrWithTTL(ctx, rdb, key, totpRLWindow)
			if err != nil {
				// Fail open: if Redis is unavailable we do not block legitimate users.
				return next(c)
			}

			if count > totpRLLimit {
				return c.JSON(http.StatusTooManyRequests, map[string]string{
					"error": "Too many TOTP attempts",
					"code":  "TOTP_RATE_LIMIT",
				})
			}

			return next(c)
		}
	}
}

// incrWithTTL atomically increments key and, on the first increment, sets its
// expiry to ttl.  Returns the new counter value.
func incrWithTTL(ctx context.Context, rdb *redis.Client, key string, ttl time.Duration) (int64, error) {
	pipe := rdb.Pipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return incr.Val(), nil
}
