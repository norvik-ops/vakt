// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package lexware

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/shared/logsafe"
)

// ParseOrigins liest eine komma-getrennte Ursprungsliste.
//
// Leere Eintraege und Leerraum fallen weg, damit ein versehentliches
// "a, ,b" nicht einen leeren String als erlaubten Ursprung eintraegt —
// der wuerde auf nichts passen, aber die Liste laenger aussehen lassen,
// als sie ist.
func ParseOrigins(raw string) []string {
	out := make([]string, 0, 2)
	for _, part := range strings.Split(raw, ",") {
		if o := strings.TrimSpace(part); o != "" {
			out = append(out, o)
		}
	}
	return out
}

// SalesCORS erlaubt genau einer Sorte fremder Seite, diesen Endpunkt aus dem
// Browser aufzurufen: der eigenen Marketing-Seite mit dem Bestellformular.
//
// Warum es das ueberhaupt braucht: Das Formular auf vakt.norvikops.de POSTet
// JSON an api.norvikops.de. Zwei Hosts, also schickt der Browser vorher eine
// Preflight-Anfrage (OPTIONS) und liefert die eigentliche Bestellung nur ab,
// wenn die Antwort den eigenen Ursprung freigibt. Bis 2026-08-07 tat sie das
// nicht: Echos Router beantwortet ein unregistriertes OPTIONS selbst mit 204
// und einem Allow-Header (router.go, optionsMethodHandler) — syntaktisch eine
// Antwort, aber ohne ein einziges Access-Control-Feld. Der Browser brach ab,
// die Anfrage erreichte den Handler nie, und der Interessent las "Das hat
// nicht geklappt.".
//
// Bewusst NICHT "*": Dieser Prozess stellt Rechnungen und signiert
// Lizenzschluessel. Eine Freigabe fuer jede Seite im Netz waere hier kein
// Komfort, sondern eine Einladung, den Rechnungslauf von beliebigen Seiten
// aus anzustossen. Die Liste kommt deshalb aus der Konfiguration
// (VAKT_BILLING_CORS_ORIGINS) und steht nicht im Code.
//
// Bewusst OHNE Zugangsdaten (AllowCredentials bleibt false): Das Formular
// schickt keinen `credentials`-Modus mit, der Browser sendet also ohnehin
// keine Cookies. Und dieser Dienst kennt gar keine Sitzungscookies — der
// Freigabe-Link und das Kundenportal weisen sich ueber ein Token im Pfad aus.
// Was niemand braucht, wird auch nicht erlaubt.
func SalesCORS(allowed []string) echo.MiddlewareFunc {
	set := make(map[string]struct{}, len(allowed))
	for _, o := range allowed {
		set[o] = struct{}{}
	}

	// Eine leere Liste heisst KEINE Freigabe — und das muss man hier
	// ausdruecklich hinschreiben.
	//
	// Echos CORS-Middleware ersetzt eine leere AllowOrigins-Liste stillschweigend
	// durch DefaultCORSConfig, und deren Wert ist ["*"] (middleware/cors.go:141
	// in v4.15.2, am Quelltext geprueft). Wer also die Ursprungsliste nicht setzt
	// oder sich vertippt, bekaeme nicht "niemand darf", sondern "jeder darf" —
	// ausgerechnet auf dem Endpunkt, der den Rechnungslauf anstoesst, und ohne
	// dass irgendetwas auffiele. Genau umgekehrt zur Erwartung.
	//
	// Deshalb wird die Middleware bei leerer Liste gar nicht erst eingehaengt:
	// Der Endpunkt verhaelt sich dann wie vor dieser Aenderung — keine
	// Access-Control-Koepfe, der Browser verwirft die Bestellung —, aber jeder
	// Versuch wird protokolliert und gezaehlt. Kaputt und laut ist besser als
	// offen und still.
	if len(allowed) == 0 {
		return func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				logRejectedOrigin(c, set)
				return next(c)
			}
		}
	}

	cors := middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: allowed,
		// Nur was das Formular tut. GET/PUT/DELETE haben auf diesem Endpunkt
		// nichts zu suchen, also stehen sie auch nicht in der Freigabe.
		AllowMethods: []string{http.MethodPost, http.MethodOptions},
		// Content-Type ist der einzige Kopf, den das Formular setzt — und
		// zugleich der Grund fuer den Preflight: mit application/json ist die
		// Anfrage nicht mehr "einfach" im Sinne von CORS.
		AllowHeaders:     []string{echo.HeaderContentType},
		AllowCredentials: false,
		MaxAge:           86400,
	})

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		inner := cors(next)
		return func(c echo.Context) error {
			logRejectedOrigin(c, set)
			return inner(c)
		}
	}
}

// logRejectedOrigin macht einen Fehler sichtbar, der sonst spurlos ist.
//
// Ein im Browser geblockter Preflight erzeugt bei uns NICHTS: kein Handler
// laeuft, keine Zeile entsteht, keine Kennzahl bewegt sich. Genau daran ist
// dieser Defekt so lange unbemerkt geblieben — die einzige Person, die ihn
// sehen konnte, war der Interessent, und der hat eine Fehlermeldung gelesen,
// die nach seinem eigenen Fehler aussah.
//
// Laeuft VOR Echos CORS-Middleware, denn die beantwortet einen Preflight
// selbst und ruft next() nicht mehr auf — von dahinter waere der
// interessanteste Fall unsichtbar.
//
// Der Ursprungs-Kopf kommt vom Aufrufer und ist nicht vertrauenswuerdig: erst
// durch logsafe, dann ins Log, sonst schreibt uns der naechste Bot
// Zeilenumbrueche in die Logdatei.
func logRejectedOrigin(c echo.Context, allowed map[string]struct{}) {
	origin := c.Request().Header.Get(echo.HeaderOrigin)
	if origin == "" {
		return // kein Browser — Lexware, eine Kundeninstanz, eine Health-Probe
	}
	if _, ok := allowed[origin]; ok {
		return
	}
	recordCORSRejected()
	log.Warn().
		Str("origin", logsafe.SanitizeField(origin, 200)).
		Str("method", c.Request().Method).
		Str("path", c.Request().URL.Path).
		Msg("billing: CORS — Ursprung nicht freigegeben, der Browser wird die Anfrage verwerfen (VAKT_BILLING_CORS_ORIGINS pruefen)")
}
