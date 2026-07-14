//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/modules/vaktaware"
)

// fakeMailer is the hand-written MailSender fake (house pattern: narrow
// interface, no mock library — see internal/services/scim). It captures what
// SendCampaignEmails would have put on the wire, which is the only part of the
// campaign flow a test cannot otherwise observe.
type fakeMailer struct {
	sent []vaktaware.OutboundMail
}

func (f *fakeMailer) Send(_ context.Context, msgs []vaktaware.OutboundMail) (int, error) {
	f.sent = append(f.sent, msgs...)
	return len(msgs), nil
}

// clickTokenRe pulls the tracking token back out of the rendered mail body, the
// same way the recipient's browser would: by following the link.
var clickTokenRe = regexp.MustCompile(`/api/v1/vaktaware/t/([0-9a-f-]{36})`)

// seedCampaign builds a complete, sendable campaign: a group with two targets
// (one of them bounced), a template whose body carries the tracking link, and a
// campaign wired to both.
func seedCampaign(t *testing.T, ctx context.Context, pool *pgxpool.Pool, repo *vaktaware.Repository, orgID, userID string, trackOpens, betriebsrat bool) (campaignID, groupID string) {
	t.Helper()

	group, err := repo.CreateTargetGroup(ctx, orgID, "Sales", "manual")
	require.NoError(t, err)

	_, err = repo.CreateTarget(ctx, orgID, group.ID, "alice@acme.test", "Alice", "Ant", "Sales")
	require.NoError(t, err)
	bounced, err := repo.CreateTarget(ctx, orgID, group.ID, "bob@acme.test", "Bob", "Bee", "Sales")
	require.NoError(t, err)
	// A bounced address must be skipped: mailing it again is how a simulation
	// gets the sending domain blacklisted.
	_, err = pool.Exec(ctx, `UPDATE sr_targets SET is_bounced = true WHERE org_id = $1::uuid AND id = $2::uuid`, orgID, bounced.ID)
	require.NoError(t, err)

	tmpl, err := repo.CreateTemplate(ctx, orgID, userID, vaktaware.CreateTemplateInput{
		Name:       "Paket-Zustellung",
		Subject:    "Ihre Sendung wartet",
		FromName:   "DHL",
		FromEmail:  "noreply@dhl.test",
		HTMLBody:   `<html><body><p>Hallo {{.FirstName}}</p><a href="{{.TrackingURL}}">Sendung verfolgen</a></body></html>`,
		AttackType: "phishing",
	})
	require.NoError(t, err)

	camp, err := repo.CreateCampaign(ctx, orgID, userID, vaktaware.CreateCampaignInput{
		Name:            "Q3 Phishing",
		TemplateID:      &tmpl.ID,
		GroupID:         &group.ID,
		FromName:        "DHL",
		FromEmail:       "noreply@dhl.test",
		Subject:         "Ihre Sendung wartet",
		TrackOpens:      trackOpens,
		BetriebsratMode: betriebsrat,
	})
	require.NoError(t, err)

	return camp.ID, group.ID
}

// TestVaktaware_CampaignSend_TrackingRoundTrip (S126) drives the entire core
// flow of Vakt Aware end to end against real Postgres: send a campaign, take the
// tracking link out of the delivered mail exactly as the recipient's browser
// would, click it, and check that the click was counted.
//
// Nothing has ever tested this. SendCampaignEmails dialled net/smtp inline, so
// it was unreachable from any test, and the module's concrete *Repository made a
// service-level test impossible anyway. That is not an academic gap: this exact
// flow has now broken three times, each time structurally, each time found by a
// human clicking through a live stack (S127: every tracking route sat behind
// auth and 401'd for the recipient who has no token; S127-2/D5: the click link
// was built on the open-pixel path).
//
// The round trip is the assertion that matters. A phishing simulation whose
// clicks are not recorded reports a 0% click rate — which is indistinguishable
// from a perfectly trained workforce, flows into Vakt Comply as evidence for
// ISO 27001 A.6.3 / NIS2 Art. 21(2)(g), and is therefore a plausible-looking lie
// in an audit file.
func TestVaktaware_CampaignSend_TrackingRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	var userID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email) VALUES ('aware-flow@acme.test') RETURNING id::text`).Scan(&userID))

	mailer := &fakeMailer{}
	svc := vaktaware.NewService(pool, vaktaware.SMTPConfig{
		Host:   "smtp.test",
		Port:   "25",
		From:   "noreply@acme.test",
		AppURL: "https://vakt.acme.test",
	}).WithMailSender(mailer)
	repo := vaktaware.NewRepository(pool)

	campaignID, _ := seedCampaign(t, ctx, pool, repo, orgID, userID, true, false)

	require.NoError(t, svc.SendCampaignEmails(ctx, orgID, campaignID))

	// One mail, to the one target that has not bounced.
	require.Len(t, mailer.sent, 1, "the bounced target must be skipped, the other must be mailed")
	mail := mailer.sent[0]
	assert.Equal(t, "alice@acme.test", mail.To)

	body := string(mail.Body)
	assert.Contains(t, body, "Hallo Alice", "the template must be rendered with the target's data")
	assert.Contains(t, body, "/api/v1/vaktaware/track/", "track_opens=true must embed the open pixel")

	// Follow the link out of the mail, exactly as the recipient would.
	m := clickTokenRe.FindStringSubmatch(body)
	require.Len(t, m, 2, "the mail must carry a click link on /t/ (the open pixel lives on /track/) — body:\n%s", body)
	token := m[1]

	// The click. This is the moment the whole feature exists for.
	_, err := svc.RecordEvent(ctx, token, "click", "203.0.113.7", "Mozilla/5.0")
	require.NoError(t, err, "the token minted at send time must resolve — a recipient who clicks must be counted")

	stats, err := svc.GetCampaignStats(ctx, orgID, campaignID)
	require.NoError(t, err)
	assert.Equal(t, 2, stats.TotalTargets)
	assert.Equal(t, 1, stats.Clicks, "the click must be recorded")
	assert.InDelta(t, 50.0, stats.ClickRate, 0.01, "1 of 2 targets clicked")
	assert.Equal(t, 1, stats.EmailsSent, "one mail went out — a campaign that reports 0 sent cannot be audited")
}

// TestVaktaware_BetriebsratMode_StoresNoPII (S126) proves the §87 BetrVG /
// DSGVO Art. 22 invariant at the layer that actually persists: with
// betriebsrat_mode on, the IP and user agent of the employee who clicked must
// not reach the database.
//
// The pure function anonymizeForBetriebsrat has a unit test. That test proves a
// function returns empty strings — not that the row written to sr_events is
// clean. Those are different claims, and only the second one is the promise made
// to a works council.
func TestVaktaware_BetriebsratMode_StoresNoPII(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	var userID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email) VALUES ('aware-br@acme.test') RETURNING id::text`).Scan(&userID))

	mailer := &fakeMailer{}
	svc := vaktaware.NewService(pool, vaktaware.SMTPConfig{
		Host: "smtp.test", Port: "25", From: "noreply@acme.test", AppURL: "https://vakt.acme.test",
	}).WithMailSender(mailer)
	repo := vaktaware.NewRepository(pool)

	campaignID, _ := seedCampaign(t, ctx, pool, repo, orgID, userID, false, true /* betriebsrat */)
	require.NoError(t, svc.SendCampaignEmails(ctx, orgID, campaignID))
	require.Len(t, mailer.sent, 1)

	m := clickTokenRe.FindStringSubmatch(string(mailer.sent[0].Body))
	require.Len(t, m, 2)

	_, err := svc.RecordEvent(ctx, m[1], "click", "203.0.113.7", "Mozilla/5.0 (secret-laptop)")
	require.NoError(t, err)

	var ip, ua *string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT ip_address, user_agent FROM sr_events
		  WHERE org_id = $1::uuid AND campaign_id = $2::uuid AND type = 'click'`,
		orgID, campaignID).Scan(&ip, &ua))

	assert.True(t, ip == nil || *ip == "", "betriebsrat_mode must not persist the clicker's IP, got %v", ip)
	assert.True(t, ua == nil || *ua == "", "betriebsrat_mode must not persist the clicker's user agent, got %v", ua)

	// The click itself is still counted — anonymisation must not cost the metric.
	stats, err := svc.GetCampaignStats(ctx, orgID, campaignID)
	require.NoError(t, err)
	assert.Equal(t, 1, stats.Clicks)
}
