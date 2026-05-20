// Package apidocs provides Swagger UI and OpenAPI spec endpoints.
//
// The OpenAPI spec is embedded at build time from openapi.yaml in this package.
// All static assets (swagger-ui CSS and JS) are also embedded so that no
// external CDN is required — this satisfies the Vakt self-hosted principle:
// nothing leaves the customer's own infrastructure, and the UI works fully
// offline.
package apidocs

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// ServeSwaggerUI delivers a Swagger UI HTML page. All CSS and JS are served
// from self-hosted paths (/api/docs/swagger-ui.css and
// /api/docs/swagger-ui-bundle.js) that are backed by the embedded dist/ files.
// No unpkg.com or other CDN is referenced.
func ServeSwaggerUI(c echo.Context) error {
	html := `<!DOCTYPE html>
<html>
<head>
  <title>Vakt API Docs</title>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <link rel="stylesheet" type="text/css" href="/api/docs/swagger-ui.css">
</head>
<body>
<div id="swagger-ui"></div>
<script src="/api/docs/swagger-ui-bundle.js"></script>
<script>
window.onload = function() {
  SwaggerUIBundle({
    url: "/api/v1/openapi.yaml",
    dom_id: '#swagger-ui',
    presets: [SwaggerUIBundle.presets.apis, SwaggerUIBundle.presets.standalone],
    layout: "BaseLayout",
    deepLinking: true,
    displayRequestDuration: true
  });
};
</script>
</body>
</html>`
	return c.HTML(http.StatusOK, html)
}

// ServeSwaggerCSS serves the vendored swagger-ui CSS from the embedded dist/ directory.
func ServeSwaggerCSS(c echo.Context) error {
	data, err := staticFiles.ReadFile("dist/swagger-ui.css")
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "swagger-ui.css not found")
	}
	c.Response().Header().Set("Content-Type", "text/css; charset=utf-8")
	c.Response().Header().Set("Cache-Control", "public, max-age=86400")
	return c.Blob(http.StatusOK, "text/css; charset=utf-8", data)
}

// ServeSwaggerJS serves the vendored swagger-ui-bundle JS from the embedded dist/ directory.
func ServeSwaggerJS(c echo.Context) error {
	data, err := staticFiles.ReadFile("dist/swagger-ui-bundle.js")
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "swagger-ui-bundle.js not found")
	}
	return c.Blob(http.StatusOK, "application/javascript; charset=utf-8", data)
}

// ServeOpenAPISpec delivers the canonical OpenAPI 3.0 YAML specification.
// The spec is embedded at build time from internal/shared/apidocs/openapi.yaml
// — there is no second source of truth. Adding a new endpoint requires editing
// that file; the CI check (cmd/openapi-verify) blocks merges that introduce
// undocumented routes.
func ServeOpenAPISpec(c echo.Context) error {
	data, err := specFile.ReadFile("openapi.yaml")
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "openapi spec not found")
	}
	c.Response().Header().Set("Content-Type", "application/yaml")
	c.Response().Header().Set("Cache-Control", "public, max-age=300")
	return c.Blob(http.StatusOK, "application/yaml", data)
}

// SpecBytes returns the raw embedded OpenAPI spec.
// Used by the openapi-verify CI tool and by tests that need to inspect the spec.
func SpecBytes() ([]byte, error) {
	return specFile.ReadFile("openapi.yaml")
}
