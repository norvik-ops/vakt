// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

// Package admin serves the billing panel: quote requests, subscriptions,
// invoices, issued licences and MSP seats.
//
// It lives in the billing service, NOT in the main Vakt API. That is the whole
// point: the billing process is the one that holds VAKT_LICENSE_PRIVATE_KEY, and
// with that key anyone can mint unlimited valid Pro licences — which cannot be
// revoked, because Vakt has no phone-home by design. The main API has ~919 routes;
// a single auth bypass in one of them must not reach that key. So the key lives in a
// process with six public routes and this panel, and nowhere else.
//
// Which is also why the panel refuses to be careless about how it is reached.
package admin

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// Mode decides how the panel is exposed.
//
// The default is the safe one. A panel that ends up publicly reachable without
// anyone deciding it should be is exactly the accident this type exists to make
// impossible: there is no "no auth, but public" state to fall into.
type Mode string

const (
	// ModeLocal binds to 127.0.0.1 only. Reach it with an SSH tunnel:
	//   ssh -L 8099:localhost:8099 <billing-host>   ->   http://localhost:8099
	// Whoever has SSH is already inside; there is no second login to get wrong.
	//
	// Der Host steht hier bewusst nicht: Diese Datei wird in den oeffentlichen
	// Mirror gespiegelt, und der Leak-Guard in scripts/build-public-mirror.sh
	// bricht den Sync ab, wenn ein NorvikOps-Infra-Name darin auftaucht.
	ModeLocal Mode = "local"

	// ModeCloudflareAccess binds publicly and requires a valid Cloudflare Access
	// JWT on every request. Cloudflare authenticates the user at the edge (e-mail
	// code or Google), so an unauthenticated request never reaches this process.
	//
	// The token is verified HERE as well, against Cloudflare's public keys. Edge
	// enforcement alone would be worthless the moment someone finds the origin's
	// IP and talks to it directly.
	ModeCloudflareAccess Mode = "cloudflare-access"
)

// Config is what the panel needs to decide it is allowed to run.
type Config struct {
	Mode Mode

	// CFTeamDomain e.g. "norvikops.cloudflareaccess.com" — where the public keys live.
	CFTeamDomain string
	// CFAudience is the Application Audience (AUD) tag from the Cloudflare Access
	// application. Without it, a token minted for ANY other application in the same
	// Cloudflare account would be accepted here.
	CFAudience string
}

// Validate refuses configurations that would leave the panel open.
func (c Config) Validate() error {
	switch c.Mode {
	case ModeLocal, "":
		return nil
	case ModeCloudflareAccess:
		if c.CFTeamDomain == "" || c.CFAudience == "" {
			return fmt.Errorf(
				"billing admin: mode=cloudflare-access needs VAKT_BILLING_ADMIN_CF_TEAM and " +
					"VAKT_BILLING_ADMIN_CF_AUD. Without the audience, a token issued for a different " +
					"Cloudflare Access application would unlock this panel — and this panel can sign licences")
		}
		return nil
	default:
		return fmt.Errorf("billing admin: unknown mode %q (want %q or %q)", c.Mode, ModeLocal, ModeCloudflareAccess)
	}
}

// Listen returns the address to bind. Local mode is loopback-only — that is the
// enforcement, not a suggestion.
func (c Config) Listen(port string) string {
	if c.Mode == ModeCloudflareAccess {
		return ":" + port
	}
	return "127.0.0.1:" + port
}

// ── Cloudflare Access token verification ─────────────────────────────────────

type jwks struct {
	mu      sync.RWMutex
	keys    map[string]*rsa.PublicKey
	fetched time.Time
	team    string
}

func (j *jwks) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	j.mu.RLock()
	k, ok := j.keys[kid]
	fresh := time.Since(j.fetched) < 1*time.Hour
	j.mu.RUnlock()
	if ok && fresh {
		return k, nil
	}
	if err := j.refresh(ctx); err != nil {
		// A stale key is better than locking the operator out of their own billing
		// panel because Cloudflare had a bad minute — but only if we had one.
		j.mu.RLock()
		defer j.mu.RUnlock()
		if k, ok := j.keys[kid]; ok {
			log.Warn().Err(err).Msg("billing admin: JWKS refresh failed, using cached key")
			return k, nil
		}
		return nil, err
	}
	j.mu.RLock()
	defer j.mu.RUnlock()
	k, ok = j.keys[kid]
	if !ok {
		return nil, fmt.Errorf("billing admin: no Cloudflare key for kid %q", kid)
	}
	return k, nil
}

func (j *jwks) refresh(ctx context.Context) error {
	url := "https://" + j.team + "/cdn-cgi/access/certs"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return fmt.Errorf("fetch cloudflare access certs: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch cloudflare access certs: status %d", resp.StatusCode)
	}

	var doc struct {
		Keys []struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return fmt.Errorf("decode cloudflare access certs: %w", err)
	}

	out := make(map[string]*rsa.PublicKey, len(doc.Keys))
	for _, k := range doc.Keys {
		if k.Kty != "RSA" {
			continue
		}
		nb, err := base64.RawURLEncoding.DecodeString(k.N)
		if err != nil {
			continue
		}
		eb, err := base64.RawURLEncoding.DecodeString(k.E)
		if err != nil {
			continue
		}
		out[k.Kid] = &rsa.PublicKey{
			N: new(big.Int).SetBytes(nb),
			E: int(new(big.Int).SetBytes(eb).Int64()),
		}
	}
	if len(out) == 0 {
		return fmt.Errorf("cloudflare access certs: no usable RSA keys")
	}

	j.mu.Lock()
	j.keys, j.fetched = out, time.Now()
	j.mu.Unlock()
	return nil
}

// RequireCloudflareAccess verifies the Access JWT on every request.
//
// In local mode it is a no-op: the listener is already bound to loopback, so the
// only way in is an SSH tunnel, and SSH is the authentication.
func RequireCloudflareAccess(cfg Config) echo.MiddlewareFunc {
	if cfg.Mode != ModeCloudflareAccess {
		return func(next echo.HandlerFunc) echo.HandlerFunc { return next }
	}
	set := &jwks{team: cfg.CFTeamDomain, keys: map[string]*rsa.PublicKey{}}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			raw := c.Request().Header.Get("Cf-Access-Jwt-Assertion")
			if raw == "" {
				// Also accept the cookie Cloudflare sets, so a plain browser
				// navigation works without the header being forwarded.
				if ck, err := c.Cookie("CF_Authorization"); err == nil {
					raw = ck.Value
				}
			}
			if raw == "" {
				log.Warn().Str("ip", c.RealIP()).Msg("billing admin: request without Cloudflare Access token")
				return c.String(http.StatusForbidden, "Kein gültiger Zugang.")
			}

			claims := jwt.MapClaims{}
			tok, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
				if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
					// Without this, an attacker could hand us a token signed with
					// "alg": "none", or an HMAC computed over the public key, and we
					// would happily accept it. This is THE classic JWT hole.
					return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
				}
				kid, _ := t.Header["kid"].(string)
				return set.key(c.Request().Context(), kid)
			})
			if err != nil || !tok.Valid {
				log.Warn().Err(err).Str("ip", c.RealIP()).Msg("billing admin: invalid Cloudflare Access token")
				return c.String(http.StatusForbidden, "Kein gültiger Zugang.")
			}

			// Signature valid is not the same as "meant for us". Both checks are
			// mandatory (the `true` argument), so a token WITHOUT an aud or iss claim
			// is rejected rather than waved through.
			//
			// Audience: Cloudflare signs every application in the account with the same
			// keys. Skip this and a token minted for some OTHER app of yours — a staging
			// site, a Grafana — would unlock a panel that can sign licences.
			//
			// Issuer: pins the token to your Access team, not somebody else's.
			if !claims.VerifyAudience(cfg.CFAudience, true) {
				log.Warn().Str("ip", c.RealIP()).Msg("billing admin: Access token is for a different application")
				return c.String(http.StatusForbidden, "Kein gültiger Zugang.")
			}
			if !claims.VerifyIssuer("https://"+cfg.CFTeamDomain, true) {
				log.Warn().Str("ip", c.RealIP()).Msg("billing admin: Access token from a different team")
				return c.String(http.StatusForbidden, "Kein gültiger Zugang.")
			}
			// Expiry is checked by ParseWithClaims; VerifyExpiresAt(.., true) additionally
			// rejects a token that carries no exp at all.
			if !claims.VerifyExpiresAt(time.Now().Unix(), true) {
				return c.String(http.StatusForbidden, "Kein gültiger Zugang.")
			}

			if email, ok := claims["email"].(string); ok {
				c.Set("admin_email", email)
			}
			return next(c)
		}
	}
}

// requestEmail is who is looking at the panel — logged with every mutation, so
// "who cancelled that subscription" has an answer.
func requestEmail(c echo.Context) string {
	if v, ok := c.Get("admin_email").(string); ok && v != "" {
		return v
	}
	return "local (ssh tunnel)"
}

// csrfToken / requireCSRF: the panel mutates state (cancel a subscription, sign a
// licence). Behind Cloudflare Access the browser holds a cookie, and a cookie is
// sent on cross-site POSTs too — so an attacker's page could make the operator's
// browser cancel a subscription. A double-submit token closes that.
func requireCSRF(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if c.Request().Method != http.MethodPost {
			return next(c)
		}
		ck, err := c.Cookie(csrfCookie)
		if err != nil || ck.Value == "" || !strings.EqualFold(ck.Value, c.FormValue("csrf")) {
			return c.String(http.StatusForbidden, "CSRF-Token fehlt oder passt nicht. Seite neu laden.")
		}
		return next(c)
	}
}

const csrfCookie = "vakt_billing_csrf"
