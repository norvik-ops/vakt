//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/modules/vaktaware"
)

// fromHeaderRe pulls the From line back out of the assembled MIME message.
var fromHeaderRe = regexp.MustCompile(`(?m)^From: (.*) <(.*)>\r?$`)

// TestVaktaware_EveryPresetIsSendable (R1-27-V01) drives every one of the 50
// shipped presets through the REAL send path — template row in Postgres,
// campaign row, SendCampaignEmails, assembled MIME message — and asserts a mail
// actually came out of it.
//
// The package-level TestEveryPresetRenders checks the same contract against the
// renderer alone. This one is the claim a customer cares about, and it is a
// different claim: it includes the database round trip through sr_templates
// (which is where the preset lives once someone picks it in the UI) and the org
// lookup behind {{company}}. Both existed as separate failure points; neither was
// covered by a test that had ever seen a real preset.
func TestVaktaware_EveryPresetIsSendable(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// The org name is what {{company}} resolves to; bootPostgresWithOrg seeds
	// "AuditOrg". A name with a space and an ampersand is the realistic case and
	// the one that breaks a spoofed sender address.
	_, err := pool.Exec(ctx, `UPDATE organizations SET name = 'Muster & Co GmbH' WHERE id = $1::uuid`, orgID)
	require.NoError(t, err)

	var userID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email) VALUES ('aware-presets@acme.test') RETURNING id::text`).Scan(&userID))

	mailer := &fakeMailer{}
	svc := vaktaware.NewService(pool, vaktaware.SMTPConfig{
		Host: "smtp.test", Port: "25", From: "noreply@acme.test", AppURL: "https://vakt.acme.test",
	}).WithMailSender(mailer)
	repo := vaktaware.NewRepository(pool)

	group, err := repo.CreateTargetGroup(ctx, orgID, "Alle", "manual")
	require.NoError(t, err)
	_, err = repo.CreateTarget(ctx, orgID, group.ID, "ada@kunde.test", "Ada", "Lovelace", "IT")
	require.NoError(t, err)

	presets := svc.GetPresetTemplates()
	require.NotEmpty(t, presets, "no presets shipped — the library is gone, not the renderer")

	for _, p := range presets {
		t.Run(p.ID, func(t *testing.T) {
			before := len(mailer.sent)

			// Exactly what the UI does when an admin picks a preset: copy it into
			// an org-owned template row.
			tmpl, err := repo.CreateTemplate(ctx, orgID, userID, vaktaware.CreateTemplateInput{
				Name:       p.Name,
				Subject:    p.Subject,
				FromName:   p.FromName,
				FromEmail:  p.FromEmail,
				HTMLBody:   p.HTMLBody,
				AttackType: p.AttackType,
			})
			require.NoError(t, err)

			// Campaign without its own subject/sender: the preset's values are the
			// ones that must reach the wire.
			camp, err := repo.CreateCampaign(ctx, orgID, userID, vaktaware.CreateCampaignInput{
				Name:       "Kampagne " + p.ID,
				TemplateID: &tmpl.ID,
				GroupID:    &group.ID,
				TrackOpens: true,
			})
			require.NoError(t, err)

			require.NoError(t, svc.SendCampaignEmails(ctx, orgID, camp.ID),
				"preset %s could not be sent", p.ID)
			require.Len(t, mailer.sent, before+1, "preset %s produced no mail", p.ID)

			msg := string(mailer.sent[before].Body)
			assert.NotContains(t, msg, "{{", "unresolved placeholder in the delivered mail:\n%s", msg)
			assert.Contains(t, msg, "Ada", "the recipient's name must be rendered")

			from := fromHeaderRe.FindStringSubmatch(msg)
			require.Len(t, from, 3, "no From header in the assembled message:\n%s", msg)
			assert.Regexp(t, `^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`, from[2],
				"sender address must be one the mail server accepts")

			// track_opens=true: exactly one open pixel, not two.
			assert.Equal(t, 1, strings.Count(msg, "/api/v1/vaktaware/track/"),
				"open pixel must appear exactly once")
		})
	}
}

// TestVaktaware_SeededTemplateSpellingIsSendable covers the templates that
// cmd/seed writes ({{TRACK_URL}}): every instance that ran the seed has two of
// them in sr_templates, and until now a campaign on one of them failed to parse.
func TestVaktaware_SeededTemplateSpellingIsSendable(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	var userID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email) VALUES ('aware-seed@acme.test') RETURNING id::text`).Scan(&userID))

	mailer := &fakeMailer{}
	svc := vaktaware.NewService(pool, vaktaware.SMTPConfig{
		Host: "smtp.test", Port: "25", From: "noreply@acme.test", AppURL: "https://vakt.acme.test",
	}).WithMailSender(mailer)
	repo := vaktaware.NewRepository(pool)

	group, err := repo.CreateTargetGroup(ctx, orgID, "Alle", "manual")
	require.NoError(t, err)
	_, err = repo.CreateTarget(ctx, orgID, group.ID, "ada@kunde.test", "Ada", "Lovelace", "IT")
	require.NoError(t, err)

	tmpl, err := repo.CreateTemplate(ctx, orgID, userID, vaktaware.CreateTemplateInput{
		Name:       "IT-Support Passwort-Reset",
		Subject:    "Dringend: Ihr Passwort läuft heute ab",
		FromName:   "IT-Support",
		FromEmail:  "it-support@company-internal.com",
		HTMLBody:   `<p>Ihr Passwort läuft in 24 Stunden ab. Klicken Sie <a href="{{TRACK_URL}}">hier</a>.</p>`,
		AttackType: "phishing",
	})
	require.NoError(t, err)

	camp, err := repo.CreateCampaign(ctx, orgID, userID, vaktaware.CreateCampaignInput{
		Name: "Seed-Kampagne", TemplateID: &tmpl.ID, GroupID: &group.ID,
	})
	require.NoError(t, err)

	require.NoError(t, svc.SendCampaignEmails(ctx, orgID, camp.ID))
	require.Len(t, mailer.sent, 1)
	msg := string(mailer.sent[0].Body)
	assert.NotContains(t, msg, "{{TRACK_URL}}")
	assert.Regexp(t, `/api/v1/vaktaware/t/[0-9a-f-]{36}`, msg,
		"the seeded template's link must resolve to a tracking URL")
}
