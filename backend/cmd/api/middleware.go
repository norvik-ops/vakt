// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package main

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"

	"github.com/matharnica/vakt/internal/auth"
	"github.com/matharnica/vakt/internal/config"
	"github.com/matharnica/vakt/internal/license"
	"github.com/matharnica/vakt/internal/shared/demo"
	sharedmw "github.com/matharnica/vakt/internal/shared/middleware"
)

// applyMiddleware registers the global middleware chain on the Echo instance.
// It runs before any routes are registered and covers request-id, OTel spans,
// security headers, structured logging, CORS, body limits, request timeouts,
// the demo guard, and per-request license context injection.
func applyMiddleware(e *echo.Echo, cfg *config.Config, log zerolog.Logger, licInst *license.Instance) {
	// X-Request-ID — applied first so every subsequent log entry can reference it.
	e.Use(sharedmw.RequestID())

	// OpenTelemetry HTTP instrumentation — wraps every request in a span when
	// telemetry.Init() configured an exporter. No-op when OTEL_EXPORTER_OTLP_ENDPOINT
	// is unset (still safe to register; the global tracer provider is the noop one).
	e.Use(otelecho.Middleware("vakt-api",
		otelecho.WithSkipper(func(c echo.Context) bool {
			// Don't span on /metrics (Prometheus polls every 30s — would dominate
			// the trace volume) or on /health (likewise scraped by Zabbix).
			p := c.Request().URL.Path
			return p == "/metrics" || p == "/health"
		}),
	))

	// Trace ID — unique per request, emitted as X-Trace-ID response header and
	// enriched into the zerolog context for structured log correlation.
	e.Use(auth.TraceMiddleware())

	// style-src-elem 'self': only external stylesheets (<link>, <style> blocks) from same origin.
	// style-src-attr 'unsafe-inline': inline style= attributes allowed — required by Radix UI
	// which sets CSS custom properties (--radix-*) via element.style.setProperty() at runtime.
	// Splitting elem/attr is meaningfully safer than a blanket 'unsafe-inline' on style-src:
	// inline attributes cannot inject <style> blocks or @import rules, severely limiting CSS
	// exfiltration attack surface. Nonce-based CSP would be cleaner but requires Vite integration.
	e.Use(middleware.SecureWithConfig(middleware.SecureConfig{
		XSSProtection:         "0",
		ContentTypeNosniff:    "nosniff",
		XFrameOptions:         "DENY",
		HSTSMaxAge:            31536000,
		ContentSecurityPolicy: "default-src 'self'; script-src 'self'; style-src-elem 'self'; style-src-attr 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; object-src 'none'; base-uri 'self'",
	}))
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			c.Response().Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
			c.Response().Header().Set("Cross-Origin-Opener-Policy", "same-origin")
			// Cross-Origin-Resource-Policy: prevent other origins from loading
			// our JSON/asset responses via <img>/<script>/fetch — completes the
			// COOP/CORP/COEP triple that gives modern Spectre-class isolation.
			// API responses are same-origin only by design; no third-party
			// consumer exists.
			c.Response().Header().Set("Cross-Origin-Resource-Policy", "same-origin")
			return next(c)
		}
	})
	e.Use(requestLogger(log))
	e.Use(middleware.Recover())
	e.Use(middleware.GzipWithConfig(middleware.GzipConfig{
		Level: 5,
		Skipper: func(c echo.Context) bool {
			p := c.Request().URL.Path
			return p == "/metrics" || p == "/health"
		},
	}))
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     cfg.CORSOrigins,
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowHeaders:     []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization, "X-Request-ID"},
		ExposeHeaders:    []string{"X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset", "X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           86400,
	}))
	// S87-2 (F-10): `*` + AllowCredentials:true is a real risk in production.
	// Demo instances may use `*` (no session cookies that matter); a non-demo
	// (production) instance must fail closed so a misconfiguration can't ship.
	if insecureWildcardCORS(cfg.CORSOrigins, cfg.DemoSeed) {
		log.Fatal().Msg("CORS is configured to allow all origins (*) with credentials in non-demo mode — refusing to start. Set VAKT_CORS_ORIGINS to an explicit origin list (e.g. https://vakt.example.com)")
	} else if len(cfg.CORSOrigins) == 1 && cfg.CORSOrigins[0] == "*" {
		log.Warn().Msg("CORS allows all origins (*) with credentials — acceptable only for the public demo; set VAKT_CORS_ORIGINS for production")
	}
	e.Use(middleware.BodyLimit("10MB"))
	e.Use(middleware.ContextTimeoutWithConfig(middleware.ContextTimeoutConfig{
		Timeout: 30 * time.Second,
		// LLM-backed routes (/…/ai/…) legitimately run up to VAKT_AI_REPORT_TIMEOUT
		// (default 120s) and stream; the 30s global timeout cancelled their request
		// context and killed every AI report at 30s (R-H09/S131-F4). The AI client
		// enforces its own timeout, so skipping the global one here is safe.
		Skipper: func(c echo.Context) bool {
			return strings.Contains(c.Path(), "/ai/")
		},
		ErrorHandler: func(err error, c echo.Context) error {
			if err != nil && errors.Is(err, context.DeadlineExceeded) {
				return c.JSON(http.StatusServiceUnavailable, map[string]string{
					"error": "request timeout",
					"code":  "REQUEST_TIMEOUT",
				})
			}
			return err
		},
	}))
	e.Use(demo.Guard(cfg.DemoSeed))

	// Instance.Middleware reads the holder on every request. The previous closure
	// captured the startup licence by value, so a key activated at runtime was
	// invisible to every route that is not behind license.DBMiddleware.
	e.Use(licInst.Middleware())
}

// insecureWildcardCORS reports whether the CORS configuration is a wildcard
// origin (`*`) in non-demo (production) mode. The main CORS block sets
// AllowCredentials:true, so `*` + credentials must never ship in production
// (S87-2, F-10). Demo instances are exempt — they have no session cookies that
// matter and the public demo intentionally accepts any origin.
func insecureWildcardCORS(origins []string, demoMode bool) bool {
	if demoMode {
		return false
	}
	return len(origins) == 1 && origins[0] == "*"
}

// requestLogger is the structured access log. It is a named function rather than
// an inline closure so the redaction below can be tested through the real
// middleware, not through a rebuilt copy of it — a test that reassembles the
// chain by hand tests a configuration that does not exist.
func requestLogger(log zerolog.Logger) echo.MiddlewareFunc {
	return middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogMethod:  true,
		LogURI:     true,
		LogStatus:  true,
		LogLatency: true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			// c.Path() is the matched route template ("/supplier/:token/save"),
			// not the concrete URI. Echo's ServeHTTP runs the router before the
			// e.Use chain, so it is already populated here.
			log.Info().
				Str("method", v.Method).
				Str("uri", redactURI(c.Path(), v.URI)).
				Int("status", v.Status).
				Dur("latency", v.Latency).
				Msg("request")
			return nil
		},
	})
}

// sensitiveQueryKeys are query parameters whose values must never reach a log.
//
// Query strings end up in the access log, and from there in Loki on another host.
// A one-click approval link cannot avoid carrying its token in the URL — a mail
// client will not POST — so the token is redacted on the way out instead.
//
// This was not theoretical: the billing approval token was hashed in the database
// precisely so a leaked backup could not be used to approve invoices, and then the
// access log printed the plaintext token on every click, undoing all of it.
var sensitiveQueryKeys = map[string]bool{
	"token":        true,
	"access_token": true,
	"api_key":      true,
	"apikey":       true,
	"key":          true,
	"secret":       true,
	"password":     true,
	"code":         true, // OAuth/OIDC authorization codes
	"state":        true,
}

// sensitivePathParamParts are substrings that mark a *path* parameter as
// carrying a secret rather than an identifier.
//
// Matching on a substring of the parameter NAME — not on a list of concrete
// paths — is deliberate. An enumeration of paths is stale the moment somebody
// adds a route, which is exactly the subset trap this project has walked into
// repeatedly. The route template is the truth, and it is available for free at
// log time, so a future /reset/:reset_token or /invite/:share_token is covered
// on the day it is written, with nobody having to remember this file.
//
// Measured against the actual route surface (16 routes carry :token; the full
// parameter inventory is :id, :project_id, :env_id, :slug, :key, :code, … ),
// three query keys are deliberately NOT listed here even though they stay
// sensitive as query parameters:
//   - "key"   — /…/secrets/:key is the secret's NAME (DATABASE_URL), not its value
//   - "code"  — /physical-templates/:code is an ISO 27001 A.7.x template code
//   - "state" — no path parameter uses it; as a path segment it would be an ID
//
// Masking those would cost real debuggability and protect nothing. Over-masking
// is not the safe direction: logs nobody can read stop being used.
var sensitivePathParamParts = []string{"token", "secret", "password", "apikey"}

// sensitivePathParam reports whether a route parameter name denotes a secret.
func sensitivePathParam(name string) bool {
	n := strings.ToLower(name)
	n = strings.ReplaceAll(n, "_", "")
	n = strings.ReplaceAll(n, "-", "")
	for _, part := range sensitivePathParamParts {
		if strings.Contains(n, part) {
			return true
		}
	}
	return false
}

// redactPathParams masks the path segments that the matched route declares as
// secret-bearing parameters. routePath is Echo's route template; path is the
// concrete request path (no query string).
//
// Known, bounded gap: when no route matched, the template is the catch-all
// ("/api/v1/*") and no parameter names are known, so the path is left intact.
// Masking every segment of an unmatched path instead would destroy the live-404
// signal this project uses to find unwired handlers — and a token sent to a URL
// that does not exist was never accepted by any handler.
func redactPathParams(routePath, path string) string {
	if routePath == "" || !strings.ContainsRune(routePath, ':') {
		return path
	}
	// RequestURI is normally origin-form ("/a/b"). Anything else (absolute-form
	// from a proxy) would not align with the template segment by segment.
	if !strings.HasPrefix(path, "/") {
		return path
	}

	tmpl := strings.Split(strings.TrimPrefix(routePath, "/"), "/")
	segs := strings.Split(strings.TrimPrefix(path, "/"), "/")

	changed := false
	for i, t := range tmpl {
		if i >= len(segs) {
			break
		}
		if strings.HasPrefix(t, "*") {
			break // catch-all swallows the rest; nothing is named beyond here
		}
		if strings.HasPrefix(t, ":") && sensitivePathParam(t[1:]) {
			segs[i] = "***"
			changed = true
		}
	}
	if !changed {
		return path
	}
	return "/" + strings.Join(segs, "/")
}

// redactQueryString masks the values of sensitive query parameters, keeping the
// harmless ones readable so the logs stay useful for debugging.
func redactQueryString(query string) string {
	if query == "" {
		return query
	}
	parts := strings.Split(query, "&")
	for j, p := range parts {
		k, _, found := strings.Cut(p, "=")
		if found && sensitiveQueryKeys[strings.ToLower(k)] {
			parts[j] = k + "=***"
		}
	}
	return strings.Join(parts, "&")
}

// redactURI masks secrets in both halves of the logged URI: the query string
// and the path.
//
// The path half was the hole. redactQuery only ever touched the query string and
// returned early when the URI had no "?", so the 16 public routes that carry the
// raw token in a path segment (auditor, supplier, share, policy-accept,
// DSR portal, phishing tracking, billing portal) logged it verbatim. Those tokens
// are stored as SHA-256 hashes precisely so a log or backup leak is worthless —
// and the logs ship to Loki on another host, so anyone who could read Loki could
// replay an auditor, supplier or DSR link inside its validity window.
func redactURI(routePath, uri string) string {
	path, sep, query := uri, "", ""
	if i := strings.IndexByte(uri, '?'); i >= 0 {
		path, sep, query = uri[:i], "?", uri[i+1:]
	}
	return redactPathParams(routePath, path) + sep + redactQueryString(query)
}

// redactQuery masks sensitive query parameters only. Retained as the
// query-string half of redactURI.
func redactQuery(uri string) string {
	return redactURI("", uri)
}
