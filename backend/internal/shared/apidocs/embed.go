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
