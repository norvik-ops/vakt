// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package lexware

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog/log"
)

// Register mounts the direct-sale endpoints.
//
// All three are PUBLIC by necessity and each is protected differently:
//
//   - quote-request  : anyone may ask for a quote. Rate-limited + honeypot; it
//     only writes a row and mails the seller. It cannot create an invoice.
//   - approve        : guarded by a 32-byte token checked against a stored hash.
//   - lexware/webhook: Lexware has no static source IP. The payload is treated as
//     an untrusted hint and every claim in it is re-checked against the Lexware
//     API before a key is issued (see Handler.Webhook).
//
// Mounted on a group WITHOUT AuthMiddleware on purpose. A customer's browser and
// Lexware's servers have no Vakt session — the "commented as public, mounted on
// protected" mistake (S127) is exactly what this comment exists to prevent.
//
// corsOrigins gibt die Ursprünge frei, die das Bestellformular aus einem Browser
// aufrufen dürfen. Leer heißt: keine Freigabe. Siehe cors.go.
func Register(g *echo.Group, h *Handler, corsOrigins []string) {
	quoteLimiter := middleware.RateLimiterWithConfig(middleware.RateLimiterConfig{
		Store: middleware.NewRateLimiterMemoryStoreWithConfig(
			middleware.RateLimiterMemoryStoreConfig{Rate: 0.05, Burst: 3, ExpiresIn: 1 * time.Hour},
		),
	})

	// CORS haengt an GENAU DIESEM Endpunkt, nicht an der Gruppe.
	//
	// Von den neun Routen dieses Listeners wird nur diese eine aus dem Browser
	// einer anderen Domain aufgerufen (Bestellformular auf vakt.norvikops.de →
	// api.norvikops.de). Alles andere braucht es nicht und bekommt es deshalb
	// nicht: Der Freigabe-Link ist eine normale Seitennavigation aus einer Mail
	// (Navigationen kennen kein CORS), der Lexware-Webhook und die
	// Lizenzerneuerung sind Server-zu-Server ohne Browser, und das Kundenportal
	// liefert sein HTML und nimmt sein Formular unter demselben Host entgegen
	// (lizenz.norvikops.de) — also gleicher Ursprung.
	//
	// Die eigene OPTIONS-Route ist notwendig, nicht Zierde: Echos Router
	// beantwortet ein nicht registriertes OPTIONS mit einem selbst erzeugten
	// Handler (router.go, optionsMethodHandler) — 204 plus Allow-Header, und
	// Route-Middleware der POST-Route laeuft dabei NICHT. Genau das war der
	// Defekt: eine Antwort ohne ein einziges Access-Control-Feld.
	//
	// Reihenfolge: CORS VOR dem Rate-Limit. Sonst traegt eine 429 keinen
	// Access-Control-Allow-Origin, der Browser verwirft sie, und aus einem
	// "zu viele Anfragen, bitte gleich nochmal" wird wieder das nichtssagende
	// "Das hat nicht geklappt.".
	cors := SalesCORS(corsOrigins)
	g.POST("/billing/quote-request", h.RequestQuote, cors, quoteLimiter)
	g.OPTIONS("/billing/quote-request", func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	}, cors)
	g.GET("/billing/quote-request/:id/approve", h.Approve)
	g.POST("/billing/lexware/webhook", h.Webhook)

	// The endpoint behind VAKT_LICENSE_TOKEN: a customer's instance polls it once
	// a day and swaps in the key it gets back, so a renewal needs no manual step.
	// Guarded by the renewal token in the Authorization header, plus a rate limit —
	// 60/h with a small burst is far above one poll a day and far below anything
	// useful for guessing a UUID.
	renewLimiter := middleware.RateLimiterWithConfig(middleware.RateLimiterConfig{
		Store: middleware.NewRateLimiterMemoryStoreWithConfig(
			middleware.RateLimiterMemoryStoreConfig{Rate: 60.0 / 3600.0, Burst: 5, ExpiresIn: 10 * time.Minute},
		),
	})
	g.GET("/billing/license", h.GetLicense, renewLimiter)

	// Lexware probes the callback URL with HEAD before it accepts a subscription.
	// Without this the probe fell through to the catch-all and answered 401 —
	// harmless today (the subscription registered anyway), but Lexware deletes
	// subscriptions whose callback looks dead. A webhook that quietly stops firing
	// means paid invoices stop issuing licences, and nobody finds out until a
	// customer writes in. Answer the probe explicitly instead of hoping.
	g.HEAD("/billing/lexware/webhook", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	log.Info().Strs("cors_origins", corsOrigins).
		Msg("billing: direct-sale routes registered (quote-request, approve, lexware webhook, license refresh)")
}

// EnsureWebhook registers the payment.changed subscription with Lexware at boot.
//
// Re-registering on every boot is deliberate: rotating the Lexware API key
// deletes every subscription created with the old one, and the key expires after
// 24 months. Without this, webhooks would go quiet on rotation day and licences
// would silently stop being issued after payment — the kind of failure nobody
// notices until a paying customer writes in.
func EnsureWebhook(c *Client, callbackURL string) {
	if !c.Enabled() || callbackURL == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := c.EnsureEventSubscription(ctx, callbackURL); err != nil {
		log.Error().Err(err).Str("callback", callbackURL).
			Msg("billing: could not register Lexware payment webhook — payments will NOT auto-issue licences")
		return
	}
	log.Info().Str("callback", callbackURL).Msg("billing: Lexware payment.changed webhook registered")
}
