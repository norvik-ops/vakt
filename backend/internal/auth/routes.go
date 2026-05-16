// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth

import (
	"github.com/labstack/echo/v4"

	"github.com/sechealth-app/sechealth/internal/license"
)

// Register mounts the auth routes onto the given echo Group.
func Register(g *echo.Group, h *Handler) {
	g.POST("/register", h.Register)
	g.POST("/login", h.Login)
	g.POST("/refresh", h.Refresh)
	g.POST("/logout", h.Logout)

	// OIDC (OAuth2 via Casdoor sidecar) — SSO Pro feature
	g.POST("/oidc/callback", h.OIDCCallback, license.Require(license.FeatureSSO))

	// SAML (proxied through Casdoor) — SSO Pro feature
	g.GET("/saml/metadata", h.SAMLMetadata, license.Require(license.FeatureSSO))
	g.POST("/saml/callback", h.SAMLCallback, license.Require(license.FeatureSSO))
	g.POST("/saml/acs", h.SAMLACS, license.Require(license.FeatureSSO))
}
