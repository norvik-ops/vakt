//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/modules/vaktaware"
)

// Diese Datei prueft eine einzige Behauptung in vier Varianten: die Zahlen, die
// eine Kampagne meldet, muessen mit dem uebereinstimmen, was die Maschine
// tatsaechlich getan hat. Sie fliessen als Nachweis nach Vakt Comply, und eine
// falsche Null ist dort ein Nachweis, der etwas Unwahres behauptet — plausibel
// unwahr, was schlimmer ist als offensichtlich unwahr.
//
// L2-01 kein Versand, trotzdem "versendet"
// L2-03 Kampagne bleibt nach jedem Sendefehler auf "laeuft"
// L3-05 Wiederanlauf schickt jedem die Mail ein zweites Mal
// L1-06 Bounce-Unterdrueckung war toter Code

func awareTestService(t *testing.T, pool *pgxpool.Pool, mailer vaktaware.MailSender) *vaktaware.Service {
	t.Helper()
	return vaktaware.NewService(pool, vaktaware.SMTPConfig{
		Host: "smtp.test", Port: "25", From: "noreply@acme.test", AppURL: "https://vakt.acme.test",
	}).WithMailSender(mailer)
}

func awareTestUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool, email string) string {
	t.Helper()
	var userID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email) VALUES ($1) RETURNING id::text`, email).Scan(&userID))
	return userID
}

func campaignStatus(t *testing.T, ctx context.Context, pool *pgxpool.Pool, orgID, campaignID string) string {
	t.Helper()
	var status string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT status FROM sr_campaigns WHERE org_id = $1::uuid AND id = $2::uuid`,
		orgID, campaignID).Scan(&status))
	return status
}

// TestVaktaware_NothingDelivered_IsNotReportedAsSent (L2-01, L2-03) is the run
// that gave the finding its name: the mail server is unreachable, not one
// message leaves the machine, and the campaign used to report „Versendet: N,
// Klickrate 0 %" — the picture of a workforce that fell for nothing.
func TestVaktaware_NothingDelivered_IsNotReportedAsSent(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	userID := awareTestUser(t, ctx, pool, "aware-dead@acme.test")
	repo := vaktaware.NewRepository(pool)

	// Every recipient is refused, the way an unreachable mail server refuses.
	dead := &fakeMailer{reject: func(string) (error, bool) {
		return errors.New("smtp open: dial tcp 127.0.0.1:1025: connect: connection refused"), false
	}}
	svc := awareTestService(t, pool, dead)

	campaignID, _ := seedCampaign(t, ctx, pool, repo, orgID, userID, true, false)
	err := svc.SendCampaignEmails(ctx, orgID, campaignID)
	require.Error(t, err, "a send in which nothing left the machine must not report success")

	stats, statsErr := svc.GetCampaignStats(ctx, orgID, campaignID)
	require.NoError(t, statsErr)
	assert.Equal(t, 0, stats.EmailsSent,
		"no mail left the machine — emails_sent must be 0, not the number of attempts")
	assert.Equal(t, "failed", campaignStatus(t, ctx, pool, orgID, campaignID),
		"a campaign whose send broke off must not stay on running (L2-03) nor read as completed (L2-01)")

	// The withdrawn tracking rows must really be gone, not just uncounted.
	var sentEvents int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM sr_events WHERE org_id = $1::uuid AND campaign_id = $2::uuid AND type = 'sent'`,
		orgID, campaignID).Scan(&sentEvents))
	assert.Equal(t, 0, sentEvents, "a sent event for a mail that never went out is a lie in the evidence trail")
}

// TestVaktaware_PartialDelivery_CountsOnlyWhatLeft (L2-01) is the second logged
// run: the server accepts one recipient and rejects two. All three used to be
// counted as delivered.
func TestVaktaware_PartialDelivery_CountsOnlyWhatLeft(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	userID := awareTestUser(t, ctx, pool, "aware-partial@acme.test")
	repo := vaktaware.NewRepository(pool)

	group, err := repo.CreateTargetGroup(ctx, orgID, "Alle", "manual")
	require.NoError(t, err)
	for _, e := range []string{"alice@acme.test", "bob@acme.test", "carol@acme.test"} {
		_, err := repo.CreateTarget(ctx, orgID, group.ID, e, "Vor", "Nach", "IT")
		require.NoError(t, err)
	}
	tmpl, err := repo.CreateTemplate(ctx, orgID, userID, vaktaware.CreateTemplateInput{
		Name: "T", Subject: "S", FromName: "F", FromEmail: "f@acme.test",
		HTMLBody: `<html><body><a href="{{.TrackingURL}}">x</a></body></html>`, AttackType: "phishing",
	})
	require.NoError(t, err)
	camp, err := repo.CreateCampaign(ctx, orgID, userID, vaktaware.CreateCampaignInput{
		Name: "Teilzustellung", TemplateID: &tmpl.ID, GroupID: &group.ID,
	})
	require.NoError(t, err)

	mailer := &fakeMailer{reject: func(to string) (error, bool) {
		if to == "carol@acme.test" {
			return nil, false
		}
		return errors.New("smtp RCPT: 501 5.5.4 Syntax error (invalid TO parameter)"), true
	}}
	svc := awareTestService(t, pool, mailer)

	require.NoError(t, svc.SendCampaignEmails(ctx, orgID, camp.ID),
		"one delivery succeeded, so the run is not a total failure")

	stats, err := svc.GetCampaignStats(ctx, orgID, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, 3, stats.TotalTargets)
	assert.Equal(t, 1, stats.EmailsSent, "one mail reached the mail server, not three")

	delivered, err := repo.CountDeliveries(ctx, orgID, camp.ID, "delivered")
	require.NoError(t, err)
	failedCount, err := repo.CountDeliveries(ctx, orgID, camp.ID, "failed")
	require.NoError(t, err)
	assert.Equal(t, 1, delivered)
	assert.Equal(t, 2, failedCount, "the two rejections must be recorded, not only logged")

	// L1-06: a 5xx is the only bounce signal this deployment can observe, and it
	// is now recorded. Before, is_bounced was written by nothing but a test.
	var bounced int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM sr_targets WHERE org_id = $1::uuid AND group_id = $2::uuid AND is_bounced`,
		orgID, group.ID).Scan(&bounced))
	assert.Equal(t, 2, bounced, "a permanently rejected address must not be mailed again next round")
}

// TestVaktaware_TemporaryRejection_DoesNotBounce guards the other half of L1-06:
// a 4xx is „not right now", and marking that address as bounced would exclude a
// perfectly good employee from every future simulation.
func TestVaktaware_TemporaryRejection_DoesNotBounce(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	userID := awareTestUser(t, ctx, pool, "aware-temp@acme.test")
	repo := vaktaware.NewRepository(pool)

	mailer := &fakeMailer{reject: func(to string) (error, bool) {
		if to == "alice@acme.test" {
			return errors.New("smtp RCPT: 451 4.7.1 greylisted, try again later"), false
		}
		return nil, false
	}}
	svc := awareTestService(t, pool, mailer)
	campaignID, groupID := seedCampaign(t, ctx, pool, repo, orgID, userID, false, false)
	_ = svc.SendCampaignEmails(ctx, orgID, campaignID)

	var bounced int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM sr_targets
		  WHERE org_id = $1::uuid AND group_id = $2::uuid AND email = 'alice@acme.test' AND is_bounced`,
		orgID, groupID).Scan(&bounced))
	assert.Equal(t, 0, bounced, "a temporary rejection is not a bounce")
}

// TestVaktaware_HardError_MarksCampaignFailed (L2-03) covers the second, entirely
// independent trigger from the finding: a campaign without a template. It was
// creatable, launchable, and then sat on „running" forever while Asynq retried
// 25 times and archived the task in silence.
func TestVaktaware_HardError_MarksCampaignFailed(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	userID := awareTestUser(t, ctx, pool, "aware-notmpl@acme.test")
	repo := vaktaware.NewRepository(pool)
	svc := awareTestService(t, pool, &fakeMailer{})

	camp, err := repo.CreateCampaign(ctx, orgID, userID, vaktaware.CreateCampaignInput{Name: "ohne Vorlage"})
	require.NoError(t, err)
	require.NoError(t, repo.UpdateCampaignStatus(ctx, orgID, camp.ID, "running"))

	err = svc.SendCampaignEmails(ctx, orgID, camp.ID)
	require.Error(t, err)
	assert.Equal(t, "failed", campaignStatus(t, ctx, pool, orgID, camp.ID),
		"the user must be able to see that this campaign is over and why")
}

// TestVaktaware_Restart_DoesNotMailAnyoneTwice (L3-05) is the finding's own
// scenario: the send task runs a second time — because the worker died mid-run,
// because SetCampaignCompleted failed and Asynq redelivered, or because someone
// clicked „Starten" twice.
func TestVaktaware_Restart_DoesNotMailAnyoneTwice(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	userID := awareTestUser(t, ctx, pool, "aware-restart@acme.test")
	repo := vaktaware.NewRepository(pool)
	mailer := &fakeMailer{}
	svc := awareTestService(t, pool, mailer)

	campaignID, _ := seedCampaign(t, ctx, pool, repo, orgID, userID, true, false)
	require.NoError(t, svc.SendCampaignEmails(ctx, orgID, campaignID))
	require.Len(t, mailer.sent, 1, "one of the two targets is bounced, so one mail goes out")

	// Second run with the campaign still on running — exactly what a redelivered
	// task sees after the worker died before the final status write.
	require.NoError(t, repo.UpdateCampaignStatus(ctx, orgID, campaignID, "running"))
	require.NoError(t, svc.SendCampaignEmails(ctx, orgID, campaignID))
	assert.Len(t, mailer.sent, 1, "the restart must not mail the same employee a second time")

	// Third run, this time with the status guard doing the work.
	require.NoError(t, svc.SendCampaignEmails(ctx, orgID, campaignID))
	assert.Len(t, mailer.sent, 1, "a completed campaign must not go out again")

	stats, err := svc.GetCampaignStats(ctx, orgID, campaignID)
	require.NoError(t, err)
	assert.Equal(t, 1, stats.EmailsSent,
		"a doubled delivery count halves the click rate and makes the campaign look better than it was")
}

// TestVaktaware_BetriebsratMode_RestartIsAlsoGuarded: the delivery ledger exists
// because sr_events carries no target_id under §87 BetrVG. If the guard only
// worked outside that mode, it would protect exactly the campaigns that run
// WITHOUT works-council involvement — the wrong half.
func TestVaktaware_BetriebsratMode_RestartIsAlsoGuarded(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	userID := awareTestUser(t, ctx, pool, "aware-br-restart@acme.test")
	repo := vaktaware.NewRepository(pool)
	mailer := &fakeMailer{}
	svc := awareTestService(t, pool, mailer)

	campaignID, _ := seedCampaign(t, ctx, pool, repo, orgID, userID, false, true /* betriebsrat */)
	require.NoError(t, svc.SendCampaignEmails(ctx, orgID, campaignID))
	require.Len(t, mailer.sent, 1)

	require.NoError(t, repo.UpdateCampaignStatus(ctx, orgID, campaignID, "running"))
	require.NoError(t, svc.SendCampaignEmails(ctx, orgID, campaignID))
	assert.Len(t, mailer.sent, 1, "the restart guard must work under betriebsrat_mode too")

	// And it must not have bought that by storing the join key it is forbidden
	// to store.
	var withTarget int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM sr_events WHERE org_id = $1::uuid AND campaign_id = $2::uuid AND target_id IS NOT NULL`,
		orgID, campaignID).Scan(&withTarget))
	assert.Equal(t, 0, withTarget, "betriebsrat_mode must still keep the click-to-person join out of sr_events")
}

// TestVaktaware_StartedAt_IsRecorded covers the side finding noted under L2-03:
// sr_campaigns.started_at was written by no code path at all, so even a
// successful campaign reported „gestartet: —" in a field the frontend renders.
func TestVaktaware_StartedAt_IsRecorded(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	userID := awareTestUser(t, ctx, pool, "aware-startedat@acme.test")
	repo := vaktaware.NewRepository(pool)
	svc := awareTestService(t, pool, &fakeMailer{})

	campaignID, _ := seedCampaign(t, ctx, pool, repo, orgID, userID, false, false)
	require.NoError(t, svc.SendCampaignEmails(ctx, orgID, campaignID))

	var startedAt *time.Time
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT started_at FROM sr_campaigns WHERE org_id = $1::uuid AND id = $2::uuid`,
		orgID, campaignID).Scan(&startedAt))
	require.NotNil(t, startedAt, "started_at must be stamped when the send begins")
}

// TestVaktaware_LaunchGuards (L2-05, L3-05) covers the two entry conditions of a
// launch: the SMTP guard that never fired in the shipped configuration, and the
// status guard that lets a running campaign be started a second time.
func TestVaktaware_LaunchGuards(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	userID := awareTestUser(t, ctx, pool, "aware-launch@acme.test")
	repo := vaktaware.NewRepository(pool)

	campaignID, _ := seedCampaign(t, ctx, pool, repo, orgID, userID, false, false)

	// L2-05: "localhost" is the SHIPPED default (internal/config/config.go), and
	// the same process reports smtp_configured:false at startup. A guard that
	// only rejects the empty string never fires on an untouched installation.
	unconfigured := vaktaware.NewService(pool, vaktaware.SMTPConfig{
		Host: "localhost", Port: "1025", From: "noreply@vakt.local", AppURL: "http://localhost:8080",
	}).WithMailSender(&fakeMailer{})
	require.Error(t, unconfigured.LaunchCampaign(ctx, orgID, campaignID),
		"launching without a configured mail server must be refused, not accepted and then fail in the worker")
	assert.Equal(t, "draft", campaignStatus(t, ctx, pool, orgID, campaignID),
		"a refused launch must not leave the campaign on running")

	// L3-05, human path: the second click on „Starten".
	svc := awareTestService(t, pool, &fakeMailer{})
	require.NoError(t, svc.LaunchCampaign(ctx, orgID, campaignID))
	assert.Equal(t, "running", campaignStatus(t, ctx, pool, orgID, campaignID))
	require.Error(t, svc.LaunchCampaign(ctx, orgID, campaignID),
		"a running campaign must not be launchable again")

	// A failed campaign, on the other hand, is exactly the one an admin has to be
	// able to restart after fixing whatever broke.
	require.NoError(t, repo.UpdateCampaignStatus(ctx, orgID, campaignID, "failed"))
	require.NoError(t, svc.LaunchCampaign(ctx, orgID, campaignID),
		"a failed campaign must be relaunchable — otherwise the new status is a dead end")
}

// TestVaktaware_TransientOutage_RetryStillDelivers deckt den Fall ab, den die
// Wiederanlaufsperre selbst hätte kaputtmachen können: der Mailserver ist beim
// ersten Lauf nicht erreichbar, beim Wiederholungslauf schon.
//
// Ohne den 'failed'-Zweig in ClaimDelivery fände der zweite Lauf jede Zielperson
// beansprucht, verschickte nichts — und die Kampagne stünde am Ende auf
// „abgeschlossen" mit null Mails. Genau die falsche Null, gegen die diese Datei
// geschrieben ist, nur eine Ebene später.
func TestVaktaware_TransientOutage_RetryStillDelivers(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	userID := awareTestUser(t, ctx, pool, "aware-outage@acme.test")
	repo := vaktaware.NewRepository(pool)

	down := true
	mailer := &fakeMailer{reject: func(string) (error, bool) {
		if down {
			return errors.New("smtp open: dial tcp 127.0.0.1:1025: connect: connection refused"), false
		}
		return nil, false
	}}
	svc := awareTestService(t, pool, mailer)

	campaignID, _ := seedCampaign(t, ctx, pool, repo, orgID, userID, false, false)
	require.Error(t, svc.SendCampaignEmails(ctx, orgID, campaignID), "erster Lauf: Mailserver weg")
	require.Empty(t, mailer.sent)
	require.Equal(t, "failed", campaignStatus(t, ctx, pool, orgID, campaignID))

	// Der Wiederholungslauf, den Asynq nach dem Fehler zustellt.
	down = false
	require.NoError(t, svc.SendCampaignEmails(ctx, orgID, campaignID))
	assert.Len(t, mailer.sent, 1, "der Wiederholungslauf muss die Mail nachholen, nicht ueberspringen")
	assert.Equal(t, "completed", campaignStatus(t, ctx, pool, orgID, campaignID))

	stats, err := svc.GetCampaignStats(ctx, orgID, campaignID)
	require.NoError(t, err)
	assert.Equal(t, 1, stats.EmailsSent)

	// Und ein DRITTER Lauf darf niemanden erneut anschreiben.
	require.NoError(t, repo.UpdateCampaignStatus(ctx, orgID, campaignID, "running"))
	require.NoError(t, svc.SendCampaignEmails(ctx, orgID, campaignID))
	assert.Len(t, mailer.sent, 1, "die zugestellte Mail darf nicht ein zweites Mal rausgehen")
}
