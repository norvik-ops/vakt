// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Dieser Test prueft die ABDECKUNG von ValidateUUIDParams, nicht sein Verhalten.
//
// Der Unterschied hat gebissen: uuid_param_test.go beweist, dass der Guard eine
// kaputte UUID ablehnt — und war gruen, waehrend vier Routen ueberhaupt nicht
// hinter ihm hingen (admin, alerting, integrations haengen direkt an `protected`,
// nie an einer Modul-Gruppe, wo der Guard urspruenglich montiert war). Ein
// Verhaltenstest sagt "der Guard funktioniert"; er sagt nichts darueber, wo er
// montiert ist — und genau dort war der Bug.
//
// Zwei Entwurfsentscheidungen, beide gegen genau diese Wiederholung:
//
//  1. Der Baum kommt aus setupEcho(), also aus der ECHTEN Registrierung in
//     routes.go. Ein nachgebauter echo.New() (wie in rbaccov) traegt den Guard
//     gar nicht, weil er am Mount-Punkt sitzt und nicht in den Paket-Registern —
//     ein solcher Test wuerde seinen eigenen Nachbau pruefen.
//
//  2. Der Test kennt uuidParamNames NICHT und darf es nicht. Fragte er die Liste
//     ab, wuerde eine neue Route mit einem unbekannten Param-Namen (`:widget_id`)
//     uebersprungen: Test gruen, Route ungeschuetzt — dieselbe Teilmengen-Luecke,
//     nur eine Ebene hoeher. Stattdessen prueft er die Invariante listenfrei:
//     KEINE Route darf auf einen kaputten Pfad-Param mit 500 antworten, egal wie
//     der Param heisst. Ein 404/400/200 ist in Ordnung; 500 heisst, der Wert ist
//     ungeprueft bis in einen ::uuid-Cast durchgereicht worden.
var paramSeg = regexp.MustCompile(`:[a-zA-Z_]+`)

// junkParamValue ist syntaktisch keine UUID, aber ein voellig normales Pfad-Segment.
// Kein Slash, keine Sonderzeichen: Der Test soll den ::uuid-Cast treffen, nicht das
// Routing oder einen URL-Parser.
const junkParamValue = "not-a-uuid"

func TestUUIDParamGuardCoversEveryParameterisedRoute(t *testing.T) {
	dbURL := os.Getenv("VAKT_DB_URL")
	secret := os.Getenv("VAKT_SECRET_KEY")
	if dbURL == "" || secret == "" || os.Getenv("VAKT_REDIS_URL") == "" {
		t.Skip("needs VAKT_DB_URL + VAKT_REDIS_URL + VAKT_SECRET_KEY (CI sets all three)")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err, "connect")
	defer pool.Close()

	token := seedOrgAndToken(ctx, t, pool, secret)
	e, _ := setupEcho(ctx, testConfig())

	var checked, skipped int
	var skippedPaths []string

	for _, r := range e.Routes() {
		if !strings.Contains(r.Path, ":") {
			continue // keine Pfad-Params, nichts zu casten
		}
		// Echo registriert den 404/405-Fallback selbst; der hat keine echte Route.
		if strings.Contains(r.Name, "glob..func") && r.Path == "/*" {
			skipped++
			skippedPaths = append(skippedPaths, r.Method+" "+r.Path)
			continue
		}

		reqPath := paramSeg.ReplaceAllString(r.Path, junkParamValue)
		key := r.Method + " " + r.Path

		t.Run(key, func(t *testing.T) {
			req := httptest.NewRequest(r.Method, reqPath, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			assert.NotEqual(t, http.StatusInternalServerError, rec.Code,
				"%s antwortet auf einen kaputten Pfad-Param mit 500 — der Wert erreicht ungeprueft einen ::uuid-Cast "+
					"(SQLSTATE 22P02). Entweder fehlt der Param-Name in uuidParamNames "+
					"(internal/shared/middleware/uuid_param.go), oder die Route haengt nicht hinter "+
					"ValidateUUIDParams. Body: %s", key, strings.TrimSpace(rec.Body.String()))
		})
		checked++
	}

	t.Logf("checked=%d routes with path params, skipped=%d %v", checked, skipped, skippedPaths)

	// Ohne diese Zusicherung waere der Test selbst die naechste Ausgabe desselben
	// Fehlers: Wenn setupEcho() je einen leeren Baum liefert oder das Param-Muster
	// nicht mehr passt, liefe die Schleife durch, faende null Routen und meldete
	// gruen. Stille ist kein Beleg.
	require.Greater(t, checked, 100,
		"der Test hat fast keine parametrisierten Routen gesehen — das ist ein Defekt AM TEST, nicht ein sauberes Ergebnis")
}
