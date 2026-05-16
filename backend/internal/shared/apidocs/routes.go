package apidocs

import "github.com/labstack/echo/v4"

// Register mounts the API documentation endpoints on the Echo instance.
// No authentication is required so these routes are added directly to the
// root Echo instance, not to an authenticated group.
//
// Intentional design: exposing the OpenAPI spec without auth is standard
// practice for self-hosted products and is safe here because:
//   - The schema itself contains no sensitive data (no keys, no org data).
//   - Every actual API endpoint requires a Paseto Bearer token; this is
//     documented in the spec via the BearerAuth securityScheme and the
//     top-level security requirement in the generated YAML.
//   - Operators who wish to restrict access can place Nginx auth_basic or
//     network-level controls in front of /api/docs and /api/v1/openapi.yaml.
//
// Routes registered:
//
//	GET /api/docs            — Swagger UI HTML page
//	GET /api/v1/openapi.yaml — OpenAPI 3.0.3 YAML spec (includes BearerAuth securityScheme)
func Register(e *echo.Echo) {
	e.GET("/api/docs", ServeSwaggerUI)
	e.GET("/api/v1/openapi.yaml", ServeOpenAPISpec)
}
