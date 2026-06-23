// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth

import (
	"github.com/labstack/echo/v4"

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
func RegisterAdminRoutes(g *echo.Group, h *Handler) {
	admin := g.Group("/admin", RequireRole("Admin"))
	admin.POST("/users/:email/password-reset-token", h.AdminGeneratePasswordResetToken)
}
