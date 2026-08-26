//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"net/http"
	"net/http/httptest"
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

// TestVaktaware_BrokenReference_IsRejectedNotSwallowed (R1-14-D05).
//
// Die Umwandlung optionaler UUID-Verweise warf den Parse-Fehler weg
// (`_ = u.Scan(s)`). Ein Tippfehler in group_id wurde damit zu NULL: die
// Kampagne kam mit 201 CREATED zurueck — ohne Zielgruppe. Der Nutzer sah Erfolg,
// die Kampagne war leer. Gemessen vor dem Fix: CreateCampaign mit
// group_id="nicht-eine-uuid" liefert err=nil und eine Kampagne mit
// group_id=NULL, template_id=NULL.
func TestVaktaware_BrokenReference_IsRejectedNotSwallowed(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	userID := awareTestUser(t, ctx, pool, "aware-badref@acme.test")
	repo := vaktaware.NewRepository(pool)
	typo := "nicht-eine-uuid"

	// Repository-Ebene: der Verweis darf nicht still verschwinden.
	_, err := repo.CreateCampaign(ctx, orgID, userID, vaktaware.CreateCampaignInput{
		Name: "Tippfehler", GroupID: &typo,
	})
	require.Error(t, err, "eine kaputte group_id darf keine leere Kampagne anlegen")
	assert.ErrorIs(t, err, vaktaware.ErrInvalidID)

	_, err = repo.CreateCampaign(ctx, orgID, userID, vaktaware.CreateCampaignInput{
		Name: "Tippfehler", TemplateID: &typo,
	})
	require.Error(t, err, "eine kaputte template_id darf keine Kampagne ohne Vorlage anlegen")

	_, err = repo.CreateEnrollmentRule(ctx, orgID, vaktaware.CreateEnrollmentRuleInput{
		Name: "Regel", TriggerType: "new_employee", TargetCampaignID: &typo,
	})
	require.Error(t, err, "dieselbe Klasse in der Anmelderegel")

	err = repo.CreateTrackingEvent(ctx, orgID, uuid.New().String(), &typo, "IT", "tok", "sent", "", "")
	require.Error(t, err, "dieselbe Klasse im Tracking-Ereignis")

	_, err = repo.UpsertAssignment(ctx, orgID, uuid.New().String(), &typo, "IT", time.Now())
	require.Error(t, err, "dieselbe Klasse in der Schulungszuweisung")

	// Der gueltige Fall muss weiter durchgehen — sonst haette der Fix nur die
	// Richtung des Fehlers getauscht.
	group, err := repo.CreateTargetGroup(ctx, orgID, "Alle", "manual")
	require.NoError(t, err)
	camp, err := repo.CreateCampaign(ctx, orgID, userID, vaktaware.CreateCampaignInput{
		Name: "In Ordnung", GroupID: &group.ID,
	})
	require.NoError(t, err)
	require.NotNil(t, camp.GroupID)
	assert.Equal(t, group.ID, *camp.GroupID)

	// Und ohne Verweis bleibt es weiter erlaubt (das Feld ist optional).
	_, err = repo.CreateCampaign(ctx, orgID, userID, vaktaware.CreateCampaignInput{Name: "Ohne Gruppe"})
	require.NoError(t, err)
}

// TestVaktaware_BrokenReference_IsA400 prueft dieselbe Sache an der
// Programmierschnittstelle: der Nutzer muss den Fehler zu sehen bekommen.
func TestVaktaware_BrokenReference_IsA400(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()

	// Ein ECHTER Nutzer: mit einer erfundenen created_by-UUID scheitert das
	// INSERT am Fremdschluessel, und der Test wuerde den Fremdschluessel messen
	// statt den Befund.
	userID := awareTestUser(t, context.Background(), pool, "aware-badref-api@acme.test")

	svc := vaktaware.NewService(pool, vaktaware.SMTPConfig{Host: "smtp.test", Port: "25"})
	h := vaktaware.NewHandler(svc)
	e := echo.New()
	g := e.Group("/vaktaware", func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("org_id", orgID)
			c.Set("user_id", userID)
			c.Set("roles", []string{"Admin"})
			c.Set("license", &license.License{Tier: "pro", Features: []string{license.FeatureSecReflex}})
			return next(c)
		}
	})
	vaktaware.Register(g, h)

	req := httptest.NewRequest(http.MethodPost, "/vaktaware/campaigns",
		strings.NewReader(`{"name":"Tippfehler","from_name":"IT","from_email":"it@acme.test",`+
			`"subject":"Betreff","group_id":"nicht-eine-uuid"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code,
		"eine kaputte Referenz muss abgelehnt werden, nicht als 201 mit stillem NULL durchgehen — Antwort: %s",
		rec.Body.String())

	var created int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM sr_campaigns WHERE org_id = $1::uuid`, orgID).Scan(&created))
	assert.Equal(t, 0, created, "die abgelehnte Anfrage darf keine leere Kampagne hinterlassen")
}
