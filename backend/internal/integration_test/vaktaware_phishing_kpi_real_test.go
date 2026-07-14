//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/modules/vaktaware"
	"github.com/matharnica/vakt/internal/modules/vaktcomply/reporting"
)

// TestPhishingClickRate_ReachesComplyKPI (S126) follows a click all the way from
// the recipient's browser into the Vakt Comply KPI that an auditor reads.
//
// This is the assertion the whole module exists to support, and it has never
// held. Four independent breaks sat in this one chain, each hiding the next:
//
//  1. every tracking route sat behind auth, so the recipient — who by definition
//     has no token — got a 401 (fixed S127);
//  2. the click link was built on the open-pixel path (fixed S127-2);
//  3. the tracking token was never persisted, so the click was rejected as
//     "invalid tracking token" (fixed here, migration 242);
//  4. click events were written with a nil target, and this KPI counted DISTINCT
//     (campaign, target) while skipping NULL targets — so it returned 0 % even
//     with a click in the table (fixed here).
//
// Each of the first two fixes was necessary and neither was sufficient, which is
// exactly how a feature stays broken through two sprints of fixing it. A rate of
// 0 % is not a missing number, it is a wrong one: it is indistinguishable from a
// workforce that no phishing mail could fool, and it goes into an audit file as
// evidence for ISO 27001 A.6.3 / NIS2 Art. 21(2)(g).
//
// So the test asserts on the number an auditor would actually see, not on any
// intermediate step. Two recipients, one clicks: 50 %.
func TestPhishingClickRate_ReachesComplyKPI(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	var userID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email) VALUES ('aware-kpi@acme.test') RETURNING id::text`).Scan(&userID))

	// Before anything happened, the KPI is unknown — not 0 %. A dashboard that
	// shows 0 % for "we never ran a campaign" is lying just as much as one that
	// shows 0 % for "nobody clicked because nothing was measured".
	assert.Nil(t, reporting.CalculateKPIsForOrg(ctx, pool, orgID).PhishingClickRate,
		"no completed campaign yet — the KPI must be unknown, not zero")

	mailer := &fakeMailer{}
	svc := vaktaware.NewService(pool, vaktaware.SMTPConfig{
		Host: "smtp.test", Port: "25", From: "noreply@acme.test", AppURL: "https://vakt.acme.test",
	}).WithMailSender(mailer)
	repo := vaktaware.NewRepository(pool)

	// seedCampaign makes two targets and bounces one. Bounce the bounce: this KPI
	// divides by everyone the campaign was aimed at, so we want both reachable.
	campaignID, _ := seedCampaign(t, ctx, pool, repo, orgID, userID, true, false)
	_, err := pool.Exec(ctx, `UPDATE sr_targets SET is_bounced = false WHERE org_id = $1::uuid`, orgID)
	require.NoError(t, err)

	require.NoError(t, svc.SendCampaignEmails(ctx, orgID, campaignID))
	require.Len(t, mailer.sent, 2, "both targets get the mail")

	// Exactly one of the two falls for it, and clicks twice (people do).
	m := clickTokenRe.FindStringSubmatch(string(mailer.sent[0].Body))
	require.Len(t, m, 2)
	_, err = svc.RecordEvent(ctx, m[1], "click", "203.0.113.7", "Mozilla/5.0")
	require.NoError(t, err)
	_, err = svc.RecordEvent(ctx, m[1], "click", "203.0.113.7", "Mozilla/5.0")
	require.NoError(t, err)

	// SendCampaignEmails marks the campaign completed; the KPI only counts
	// completed campaigns.
	snap := reporting.CalculateKPIsForOrg(ctx, pool, orgID)
	require.NotNil(t, snap.PhishingClickRate, "a completed campaign with recipients must produce a rate")
	assert.InDelta(t, 50.0, *snap.PhishingClickRate, 0.01,
		"one of two recipients clicked — clicking twice must not make it 100%%")

	stats, err := svc.GetCampaignStats(ctx, orgID, campaignID)
	require.NoError(t, err)
	assert.True(t, stats.TrackingMeasured, "a campaign sent after migration 242 records its tracking")
}

// TestPhishingClickRate_IgnoresUnmeasuredLegacyCampaigns is the follow-up to the
// tracking fix: what happens to the campaigns that ran while it was broken.
//
// They stored no tracking tokens, so every click they received was rejected and
// nothing was recorded. Their zeroes are the ABSENCE of a measurement, and the
// danger is that they look exactly like one. Left in the KPI's denominator, a
// legacy campaign with 100 recipients drags the org's click rate towards zero with
// a hundred people whose behaviour nobody ever observed — an audit number that is
// wrong in the flattering direction, which is the worst direction to be wrong in.
//
// So the KPI counts only campaigns that actually recorded their sends, and the API
// marks the others `tracking_measured: false` so the UI can say so out loud instead
// of drawing a confident 0% bar.
func TestPhishingClickRate_IgnoresUnmeasuredLegacyCampaigns(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	var userID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email) VALUES ('aware-legacy@acme.test') RETURNING id::text`).Scan(&userID))

	mailer := &fakeMailer{}
	svc := vaktaware.NewService(pool, vaktaware.SMTPConfig{
		Host: "smtp.test", Port: "25", From: "noreply@acme.test", AppURL: "https://vakt.acme.test",
	}).WithMailSender(mailer)
	repo := vaktaware.NewRepository(pool)

	// A campaign from before the fix: completed, recipients, and not one `sent`
	// event to its name — precisely what every pre-242 campaign looks like.
	legacyID, _ := seedCampaign(t, ctx, pool, repo, orgID, userID, true, false)
	_, err := pool.Exec(ctx,
		`UPDATE sr_campaigns SET status = 'completed', completed_at = NOW()
		  WHERE org_id = $1::uuid AND id = $2::uuid`, orgID, legacyID)
	require.NoError(t, err)

	legacyStats, err := svc.GetCampaignStats(ctx, orgID, legacyID)
	require.NoError(t, err)
	assert.False(t, legacyStats.TrackingMeasured,
		"a campaign with no sent events measured nothing and must admit it")
	assert.Zero(t, legacyStats.EmailsSent)

	// With nothing but the unmeasured campaign, the KPI must be UNKNOWN — not 0%.
	// A dashboard showing 0% here would be claiming that nobody fell for a phishing
	// mail, when the truth is that nobody looked.
	assert.Nil(t, reporting.CalculateKPIsForOrg(ctx, pool, orgID).PhishingClickRate,
		"an org whose only completed campaign was never measured has no click rate — not a rate of zero")

	// Now a real campaign runs: two recipients, one clicks. The legacy campaign's
	// two recipients must not dilute that into 25%.
	goodID, _ := seedCampaign(t, ctx, pool, repo, orgID, userID, true, false)
	_, err = pool.Exec(ctx, `UPDATE sr_targets SET is_bounced = false WHERE org_id = $1::uuid`, orgID)
	require.NoError(t, err)
	require.NoError(t, svc.SendCampaignEmails(ctx, orgID, goodID))

	m := clickTokenRe.FindStringSubmatch(string(mailer.sent[0].Body))
	require.Len(t, m, 2)
	_, err = svc.RecordEvent(ctx, m[1], "click", "203.0.113.7", "Mozilla/5.0")
	require.NoError(t, err)

	snap := reporting.CalculateKPIsForOrg(ctx, pool, orgID)
	require.NotNil(t, snap.PhishingClickRate)
	assert.InDelta(t, 50.0, *snap.PhishingClickRate, 0.01,
		"one of the two MEASURED recipients clicked — the unmeasured campaign must not be in the denominator")
}
