// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth

import (
	"encoding/hex"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	sharedcrypto "github.com/matharnica/vakt/internal/shared/crypto"
	sharedmw "github.com/matharnica/vakt/internal/shared/middleware"
	"github.com/matharnica/vakt/internal/shared/platform/features"
)

// Register mounts the auth routes onto the given echo Group.
func Register(g *echo.Group, h *Handler) {
	g.POST("/register", h.Register)
	g.POST("/login", h.Login)
	g.POST("/refresh", h.Refresh)
	g.POST("/logout", h.Logout)

	// OIDC (OAuth2 via Casdoor sidecar) — SSO Pro feature
	g.GET("/oidc/initiate", h.OIDCInitiate, features.Require(features.FeatureSSO))
	g.POST("/oidc/callback", h.OIDCCallback, features.Require(features.FeatureSSO))

	// SAML 2.0 SP — Pro feature (moved from CE in S105-4; ADR-0022 superseded)
	// Direct SP (crewjam/saml) when org_saml_configs row exists; falls back to Casdoor proxy.
	g.GET("/saml/metadata", h.SAMLDirectMetadata, features.Require(features.FeatureSAMLAuth))
	g.GET("/saml/initiate", h.SAMLInitiate, features.Require(features.FeatureSAMLAuth))
	g.POST("/saml/callback", h.SAMLCallback, features.Require(features.FeatureSAMLAuth))
	g.POST("/saml/acs", h.SAMLDirectACS, features.Require(features.FeatureSAMLAuth))

	// Password reset — local auth only, no auth middleware required.
	g.POST("/password-reset/request", h.RequestPasswordReset)
	g.POST("/password-reset/confirm", h.ResetPassword)
}

// RegisterAdminRoutes mounts admin-only auth management routes onto g.
// g must already be behind auth middleware; the Admin role check is applied here.
//
// The password-reset-token route issues a credential (a password reset for
// another member of the org), so it carries the same TOTP step-up as the sibling
// user-management write routes (S131-R-H24): when the org enabled
// require_mfa_sensitive_calls, the caller must present X-MFA-Token. The
// middleware is a no-op for orgs that never opted in and skips safe methods.
//
// mfaSensitive is built here from the handler's own dependencies (DB pool +
// TOTP-derived key from the master secret) rather than being threaded through
// cmd/api/routes.go, so this one route can be gated without touching the shared
// wiring file. If the master key cannot be derived (e.g. no secret configured in
// a stripped-down test), the route still mounts with the org-scoping and
// no-token-in-body fixes — those are enforced unconditionally in the handler.
func RegisterAdminRoutes(g *echo.Group, h *Handler) {
	admin := g.Group("/admin", RequireRole("Admin"))

	mw := buildMFASensitive(h)
	if mw == nil {
		// R1-F3W2A-02: this used to mount the route WITHOUT the step-up and carry
		// on with a log.Warn. A guard whose absence is only logged is not a guard —
		// and this is the route ESK-5 identified as the weakest-protected admin
		// action, one that mints a credential for another member of the org.
		//
		// The route stays registered on purpose rather than disappearing: an
		// unmounted route answers 404, which reads as "this version does not have
		// the feature". 503 with a named code says what actually happened and can
		// be acted on.
		log.Error().Msg("auth admin routes: step-up MFA could not be built — password-reset-token is refused, not mounted unguarded")
		admin.POST("/users/:email/password-reset-token", func(c echo.Context) error {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{
				"error": "step-up MFA is not available on this instance — the admin password reset stays closed until it is",
				"code":  "MFA_STEPUP_UNAVAILABLE",
			})
		})
		return
	}
	admin.POST("/users/:email/password-reset-token", h.AdminGeneratePasswordResetToken, mw)
}

// buildMFASensitive constructs the step-up-MFA middleware for the admin auth
// routes from the handler's dependencies. Returns nil when the dependencies to
// enforce it are unavailable (no DB pool or the TOTP key cannot be derived).
func buildMFASensitive(h *Handler) echo.MiddlewareFunc {
	if h == nil || h.service == nil || h.service.db == nil || h.cfg == nil || h.cfg.SecretKey == "" {
		return nil
	}
	rawMasterKey, err := hex.DecodeString(h.cfg.SecretKey)
	if err != nil {
		log.Warn().Err(err).Msg("auth admin routes: cannot decode master key — mfaSensitive not mounted on password-reset-token")
		return nil
	}
	// Must match the derivation used for the shared mfaSensitive mount
	// (cmd/api/routes.go: deriveKey("vakt-totp-v1")) — the TOTP secret is
	// encrypted with this derived key, not the raw master key.
	totpKey, err := sharedcrypto.DeriveServiceKey(rawMasterKey, "vakt-totp-v1")
	if err != nil {
		log.Warn().Err(err).Msg("auth admin routes: cannot derive TOTP key — mfaSensitive not mounted on password-reset-token")
		return nil
	}
	return sharedmw.RequireMFASensitive(h.service.db, totpKey, ValidateTOTP)
}
