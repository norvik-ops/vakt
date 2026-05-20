package apidocs

import "embed"

// staticFiles holds the vendored swagger-ui-dist assets (CSS + JS).
// These files are committed to the repository so that the API documentation
// UI is fully self-contained and works without any external CDN access.
//
// To upgrade swagger-ui-dist, run:
//
//	make swagger-dist VERSION=5.x.y
//
//go:embed dist/swagger-ui.css dist/swagger-ui-bundle.js
var staticFiles embed.FS

// specFile holds the canonical OpenAPI 3.0 spec, embedded at build time from
// docs/api/openapi.yaml. This is the single source of truth — the same file
// that lives in version control is what the running server serves. There is
// no separate hand-rolled Go fallback, no drift between "documented" and
// "actually served" endpoints.
//
// The spec lives in docs/api/ (not internal/shared/apidocs/) because it is
// product documentation that ships with the repo for offline review and
// SDK generation — embedding it from there keeps a single canonical copy.
//
//go:embed openapi.yaml
var specFile embed.FS
