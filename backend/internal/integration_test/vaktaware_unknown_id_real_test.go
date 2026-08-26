//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/license"
	"github.com/matharnica/vakt/internal/modules/vaktaware"
)

var awareParamSeg = regexp.MustCompile(`:[a-zA-Z_]+`)

// TestVaktaware_UnknownButValidID_IsNeverA500 (R1-13-M01).
//
// Der bestehende Wächter cmd/api/uuid_param_coverage_test.go probt jede
// parametrisierte Route mit „not-a-uuid" und verlangt kein 500. Er war grün,
// während GET /vaktaware/campaigns/<gültige-unbekannte-uuid>/stats für JEDEN
// Aufrufer 500 PG_ERROR lieferte — eine wohlgeformte, aber unbekannte UUID kommt
// am UUID-Wächter vorbei, erreicht die Abfrage, findet nichts, und der Handler
// bildet jeden Fehler auf 500 ab. Ein Kaputt-Param-Sweep allein konnte das nicht
// finden; es brauchte die Probe mit gültiger-aber-unbekannter ID.
//
// Der Test kennt die Liste der Handler nicht und darf sie nicht kennen: er läuft
// über den echten Registrierungsbaum und prüft die Invariante listenfrei. Eine
// neue Route mit demselben Fehler fällt hier auf, ohne dass jemand eine Liste
// pflegt.
func TestVaktaware_UnknownButValidID_IsNeverA500(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	_ = ctx

	svc := vaktaware.NewService(pool, vaktaware.SMTPConfig{
		Host: "smtp.test", Port: "25", From: "noreply@acme.test", AppURL: "https://vakt.acme.test",
	})
	h := vaktaware.NewHandler(svc)

	e := echo.New()
	// Session + Pro-Lizenz stellen, sonst antwortet jede Route 401/402 und der
	// Test würde seine eigene Abwesenheit von Fehlern messen.
	g := e.Group("/vaktaware", func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("org_id", orgID)
			c.Set("user_id", uuid.New().String())
			c.Set("roles", []string{"Admin"})
			c.Set("license", &license.License{Tier: "pro", Features: []string{license.FeatureSecReflex}})
			return next(c)
		}
	})
	vaktaware.Register(g, h)

	unknown := uuid.New().String()
	var checked int
	for _, r := range e.Routes() {
		if !strings.Contains(r.Path, ":") {
			continue
		}
		if strings.Contains(r.Name, "glob..func") {
			continue // Echos eigener 404-Fallback
		}
		path := awareParamSeg.ReplaceAllString(r.Path, unknown)
		body := strings.NewReader(`{}`)
		req := httptest.NewRequest(r.Method, path, body)
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		checked++

		assert.NotEqual(t, http.StatusInternalServerError, rec.Code,
			"%s %s auf eine gueltige, aber unbekannte UUID: 500 heisst, ein fehlendes "+
				"Objekt ist als Serverfehler durchgereicht worden — Antwort: %s",
			r.Method, r.Path, rec.Body.String())
	}
	require.Greater(t, checked, 10,
		"zu wenige parametrisierte Routen geprueft (%d) — der Baum ist leer oder das Filter zu scharf", checked)
	t.Logf("%d parametrisierte vaktaware-Routen mit gueltiger unbekannter UUID beprobt", checked)
}

// TestVaktaware_CampaignStats_UnknownID ist die Fundstelle aus dem Befund,
// einzeln und mit der erwarteten Antwort statt nur „kein 500".
func TestVaktaware_CampaignStats_UnknownID(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()

	svc := vaktaware.NewService(pool, vaktaware.SMTPConfig{Host: "smtp.test", Port: "25"})
	h := vaktaware.NewHandler(svc)
	e := echo.New()
	g := e.Group("/vaktaware", func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("org_id", orgID)
			c.Set("roles", []string{"Admin"})
			c.Set("license", &license.License{Tier: "pro", Features: []string{license.FeatureSecReflex}})
			return next(c)
		}
	})
	vaktaware.Register(g, h)

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/vaktaware/campaigns/"+uuid.New().String()+"/stats", nil))

	assert.Equal(t, http.StatusNotFound, rec.Code,
		"eine Kampagne, die es nicht gibt, ist ein 404 — Antwort: %s", rec.Body.String())
	assert.Contains(t, rec.Body.String(), "NOT_FOUND")
}
