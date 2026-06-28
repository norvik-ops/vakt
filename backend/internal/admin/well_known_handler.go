// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package admin

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// securityTXT is the static content returned at /.well-known/security.txt
// per RFC 9116 (S45-6).
const securityTXT = `Contact: mailto:security@norvikops.de
Expires: 2027-05-24T00:00:00.000Z
Policy: https://github.com/norvik-ops/vatk/blob/main/SECURITY.md
Preferred-Languages: de, en
Canonical: https://vakt.norvikops.de/.well-known/security.txt
`

// HandleSecurityTXT serves the static security.txt file at
// GET /.well-known/security.txt (no authentication required).
func HandleSecurityTXT(c echo.Context) error {
	c.Response().Header().Set(echo.HeaderContentType, "text/plain; charset=utf-8")
	return c.String(http.StatusOK, securityTXT)
}
