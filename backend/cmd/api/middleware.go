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
func applyMiddleware(e *echo.Echo, cfg *config.Config, log zerolog.Logger, lic *license.License) {
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
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogMethod:  true,
		LogURI:     true,
		LogStatus:  true,
		LogLatency: true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			log.Info().
				Str("method", v.Method).
				Str("uri", redactQuery(v.URI)).
				Int("status", v.Status).
				Dur("latency", v.Latency).
				Msg("request")
			return nil
		},
	}))
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

	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("license", lic)
			return next(c)
		}
	})
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

// redactQuery replaces the values of sensitive query parameters with "***",
// keeping the rest of the URI intact so the logs stay useful for debugging.
func redactQuery(uri string) string {
	i := strings.IndexByte(uri, '?')
	if i < 0 {
		return uri
	}
	path, query := uri[:i], uri[i+1:]

	parts := strings.Split(query, "&")
	for j, p := range parts {
		k, _, found := strings.Cut(p, "=")
		if found && sensitiveQueryKeys[strings.ToLower(k)] {
			parts[j] = k + "=***"
		}
	}
	return path + "?" + strings.Join(parts, "&")
}
