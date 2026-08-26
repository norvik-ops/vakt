//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import "github.com/labstack/echo/v4"

// HIER STAND DAS G10-GATE ("oeffentliche Routen sind ohne Token erreichbar").
// Es ist nach cmd/api/public_routes_reachable_test.go umgezogen — Codeaudit v5c,
// R1-SA12-D06 / R1-SA10-04.
//
// Grund: Es baute ein eigenes echo.New() und rief RegisterPublic selbst auf.
// Damit pruefte es seinen eigenen Nachbau und las cmd/api/routes.go nie. Der
// Defekt, gegen den es gebaut wurde (S127), WAR aber der Mount-Punkt: fuenf
// Routen trugen den Kommentar "public, no auth" und hingen unter `protected`.
// Ein Rueckfall genau dorthin waere hier gruen geblieben, weil der Nachbau
// weiterhin ohne Auth-Middleware mountete.
//
// Der echte Baum kommt aus setupEcho(), und setupEcho() liegt in `package main`
// von cmd/api — von hier aus nicht importierbar. Deshalb der Umzug, nicht bloss
// eine Reparatur an Ort und Stelle.
//
// Diese Datei bleibt nur wegen des Helfers darunter bestehen, den ein anderer
// Test dieses Pakets nutzt.

// passThroughMW is a no-op echo.MiddlewareFunc for wiring RegisterPublic in
// tests: the per-route rate limiter (R-H15/S131-C2) is production-only, a test
// just needs the routes registered.
//
// Genutzt von vaktaware_e2e_mailpit_real_test.go. Dort geht es um den
// Mail-Durchstich (Kampagne → Mailpit → Tracking-Handler), nicht um den
// Mount-Punkt — fuer diesen Zweck ist ein selbst gebauter Baum richtig.
var passThroughMW echo.MiddlewareFunc = func(next echo.HandlerFunc) echo.HandlerFunc { return next }
