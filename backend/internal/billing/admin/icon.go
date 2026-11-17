// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package admin

import (
	_ "embed"
	"net/http"

	"github.com/labstack/echo/v4"
)

// appleTouchIcon is the Vakt "V" (same file as frontend/public/apple-touch-icon.png).
// iOS uses it when the panel is saved to the home screen as a web app.
//
//go:embed static/apple-touch-icon.png
var appleTouchIcon []byte

// AppleTouchIcon serves the home-screen icon. It sits outside the CSRF/Access group on
// purpose: it is a public brand image, and iOS fetches it in a separate request.
// Cloudflare Access in front of the panel still applies at the edge.
func AppleTouchIcon(c echo.Context) error {
	c.Response().Header().Set("Cache-Control", "public, max-age=86400")
	return c.Blob(http.StatusOK, "image/png", appleTouchIcon)
}
