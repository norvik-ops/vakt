// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/smtp"
	"regexp"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/config"
	"github.com/matharnica/vakt/internal/modules/vaktaware"
	"github.com/matharnica/vakt/internal/modules/vaktcomply"
	"github.com/matharnica/vakt/internal/modules/vaktcomply/risk"
)

// taskControlOwnerReminder is the Asynq task name for the daily control-owner reminder.
const taskControlOwnerReminder = "vaktcomply:control_owner_reminder"

// reEmail matches a basic e-mail address to decide whether to send a reminder.
var reEmail = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// handleSendCampaign handles vaktaware:send_campaign jobs.
func handleSendCampaign(cfg *config.Config, pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var payload struct {
			CampaignID string `json:"campaign_id"`
			OrgID      string `json:"org_id"`
		}
		if err := json.Unmarshal(t.Payload(), &payload); err != nil {
			return fmt.Errorf("parse send_campaign payload: %w", err)
		}

		smtpCfg := vaktaware.SMTPConfig{}
		if cfg != nil {
			smtpCfg.Host = cfg.SMTPHost
			smtpCfg.Port = cfg.SMTPPort
			smtpCfg.User = cfg.SMTPUser
			smtpCfg.Pass = cfg.SMTPPass
			smtpCfg.From = cfg.SMTPFrom
			smtpCfg.AppURL = cfg.FrontendURL
		}

		svc := vaktaware.NewService(pool, smtpCfg)
		if err := svc.SendCampaignEmails(ctx, payload.OrgID, payload.CampaignID); err != nil {
			return err
		}

		// After delivery + completion, check if click rate warrants a risk sync.
		// Betriebsrat-mode campaigns are always excluded (privacy constraint).
		repo := vaktaware.NewRepository(pool)
		campaign, camErr := repo.GetCampaign(ctx, payload.OrgID, payload.CampaignID)
		if camErr != nil {
			log.Error().Err(camErr).Str("campaign_id", payload.CampaignID).
				Msg("awareness_risk_sync: failed to load campaign after send")
			return nil
		}
		if !campaign.BetriebsratMode {
			stats, statErr := repo.GetCampaignStats(ctx, payload.OrgID, payload.CampaignID)
			if statErr == nil && stats.TotalTargets >= 5 && stats.ClickRate > 15.0 {
				syncAwarenessRisk(ctx, pool, payload.OrgID, campaign.Name, payload.CampaignID, stats.ClickRate)
			}
		}

		return nil
	}
}

// awarenessRiskCategory is the stable category key used to find/upsert the persistent risk.
const awarenessRiskCategory = "Awareness / Human Risk"

// awarenessRiskLikelihood maps click rate percentage to likelihood (1–5).
func awarenessRiskLikelihood(clickRate float64) int {
	switch {
	case clickRate <= 20.0:
		return 2
	case clickRate <= 40.0:
		return 3
	default:
		return 4
	}
}

// syncAwarenessRisk upserts a persistent "Awareness / Human Risk" risk and creates a
// per-campaign CAPA. Privacy guard: only aggregated stats reach this function (no names/emails).
func syncAwarenessRisk(ctx context.Context, pool *pgxpool.Pool, orgID, campaignName, campaignID string, clickRate float64) {
	complyRepo := vaktcomply.NewRepository(pool)
	riskRepo := risk.NewRepository(pool)

	// Upsert: find existing persistent risk for this org/category.
	risks, err := riskRepo.ListRisks(ctx, orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("awareness_risk_sync: list risks failed")
		return
	}

	likelihood := awarenessRiskLikelihood(clickRate)
	var riskID string

	for _, r := range risks {
		if r.Category == awarenessRiskCategory {
			riskID = r.ID
			_, updateErr := riskRepo.UpdateRisk(ctx, orgID, r.ID, vaktcomply.UpdateRiskInput{
				Title:       r.Title,
				Description: r.Description,
				Category:    r.Category,
				Likelihood:  likelihood,
				Impact:      r.Impact,
				Owner:       r.Owner,
				Status:      r.Status,
				Treatment:   r.Treatment,
			})
			if updateErr != nil {
				log.Error().Err(updateErr).Str("risk_id", r.ID).Msg("awareness_risk_sync: update risk likelihood failed")
				return
			}
			break
		}
	}

	if riskID == "" {
		// No persistent risk yet — create one.
		newRisk, createErr := riskRepo.CreateRisk(ctx, orgID, vaktcomply.CreateRiskInput{
			Title:       "Awareness-Risiko (Phishing-Simulationen)",
			Description: "Persistentes Risiko aus internen Phishing-Simulationen. Likelihood wird automatisch nach jeder Kampagne aktualisiert.",
			Category:    awarenessRiskCategory,
			Likelihood:  likelihood,
			Impact:      3,
			Treatment:   "mitigate",
		})
		if createErr != nil {
			log.Error().Err(createErr).Str("org_id", orgID).Msg("awareness_risk_sync: create risk failed")
			return
		}
		riskID = newRisk.ID
	}

	// Per-campaign CAPA for historical traceability (never floods the risk register).
	dueDate := time.Now().UTC().Add(30 * 24 * time.Hour).Format(time.RFC3339)
	_, capaErr := complyRepo.CreateCAPA(ctx, orgID, vaktcomply.CreateCAPAInput{
		SourceType:  "risk",
		SourceID:    riskID,
		Title:       fmt.Sprintf("Hohe Klickrate: %s — %.1f%%", campaignName, clickRate),
		Description: fmt.Sprintf("Kampagne \"%s\" (ID: %s) erzielte eine Klickrate von %.1f%%. Überprüfen Sie die Ergebnisse und passen Sie das Schulungskonzept an.", campaignName, campaignID, clickRate),
		DueDate:     &dueDate,
		Priority:    "medium",
	})
	if capaErr != nil {
		log.Error().Err(capaErr).Str("campaign_id", campaignID).Msg("awareness_risk_sync: create CAPA failed")
		return
	}

	log.Info().
		Str("org_id", orgID).
		Str("campaign_id", campaignID).
		Str("risk_id", riskID).
		Int("likelihood", likelihood).
		Float64("click_rate", clickRate).
		Msg("vaktaware→vaktcomply: awareness risk synced from campaign")
}

// handleTrainingReminder handles vaktaware:training_reminder jobs.
// It queries members who have not completed any training in the last 14 days
// and sends them a reminder email via the configured SMTP server.
func handleTrainingReminder(cfg *config.Config, pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var payload struct {
			OrgID string `json:"org_id"`
		}
		if err := json.Unmarshal(t.Payload(), &payload); err != nil {
			return fmt.Errorf("parse training_reminder payload: %w", err)
		}

		// Query targets that have an overdue assignment and have not completed
		// any training in the last 14 days.
		type reminderTarget struct {
			Email     string
			FirstName string
		}

		rows, err := pool.Query(ctx, `
			SELECT DISTINCT t.email, t.first_name
			FROM sr_targets t
			JOIN sr_assignments a ON a.target_id = t.id AND a.org_id = $1
			WHERE t.org_id = $1
			  AND t.is_bounced = false
			  AND NOT EXISTS (
			    SELECT 1 FROM sr_completions c
			    WHERE c.assignment_id = a.id
			      AND c.completed_at >= NOW() - INTERVAL '14 days'
			  )
		`, payload.OrgID)
		if err != nil {
			return fmt.Errorf("training_reminder: query targets: %w", err)
		}
		defer rows.Close()

		var targets []reminderTarget
		for rows.Next() {
			var rt reminderTarget
			if err := rows.Scan(&rt.Email, &rt.FirstName); err != nil {
				continue
			}
			targets = append(targets, rt)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("training_reminder: scan rows: %w", err)
		}

		if len(targets) == 0 {
			log.Info().Str("org_id", payload.OrgID).Msg("training_reminder: no pending reminders")
			return nil
		}

		if cfg == nil || cfg.SMTPHost == "" {
			log.Warn().Str("org_id", payload.OrgID).
				Int("targets", len(targets)).
				Msg("training_reminder: SMTP not configured, skipping send")
			return nil
		}

		smtpCfg := vaktaware.SMTPConfig{
			Host:   cfg.SMTPHost,
			Port:   cfg.SMTPPort,
			User:   cfg.SMTPUser,
			Pass:   cfg.SMTPPass,
			From:   cfg.SMTPFrom,
			AppURL: cfg.FrontendURL,
		}
		svc := vaktaware.NewService(pool, smtpCfg)

		sent := 0
		for _, target := range targets {
			if err := svc.SendTrainingReminderEmail(ctx, payload.OrgID, target.Email, target.FirstName); err != nil {
				log.Warn().Err(err).
					Str("org_id", payload.OrgID).
					Str("email", target.Email).
					Msg("training_reminder: send failed")
				continue
			}
			sent++
		}

		log.Info().
			Str("org_id", payload.OrgID).
			Int("sent", sent).
			Int("total", len(targets)).
			Msg("training_reminder: reminders dispatched")

		return nil
	}
}

// handleControlOwnerReminder queries all controls whose due_date (from ck_tasks) is in
// exactly 7 days, whose status is neither implemented nor not_applicable, and whose
// soa_responsible looks like a valid e-mail address, then sends a plain-HTML reminder.
func handleControlOwnerReminder(cfg *config.Config, pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		if cfg == nil || cfg.SMTPHost == "" {
			log.Info().Msg("control_owner_reminder: SMTP not configured, skipping")
			return nil
		}

		// Query controls with a task due in exactly 7 days that are not yet done.
		type reminderRow struct {
			OrgID       string
			ControlID   string
			ControlDBID string
			Title       string
			Responsible string
			DueDate     time.Time
		}

		rows, err := pool.Query(ctx, `
			SELECT
			    c.org_id::text,
			    c.control_id,
			    c.id::text,
			    c.title,
			    COALESCE(c.soa_responsible, '') AS responsible,
			    t.due_date::timestamptz
			FROM ck_controls c
			JOIN ck_tasks t ON t.entity_id = c.id
			                AND t.entity_type = 'control'
			                AND t.org_id = c.org_id
			WHERE t.due_date = CURRENT_DATE + INTERVAL '7 days'
			  AND t.status NOT IN ('done', 'closed')
			  AND COALESCE(c.manual_status, '') NOT IN ('implemented', 'not_applicable')
			  AND c.not_applicable = false
			  AND COALESCE(c.soa_responsible, '') <> ''
		`)
		if err != nil {
			return fmt.Errorf("control_owner_reminder: query: %w", err)
		}
		defer rows.Close()

		var reminders []reminderRow
		for rows.Next() {
			var r reminderRow
			if err := rows.Scan(&r.OrgID, &r.ControlID, &r.ControlDBID, &r.Title, &r.Responsible, &r.DueDate); err != nil {
				log.Warn().Err(err).Msg("control_owner_reminder: scan row")
				continue
			}
			reminders = append(reminders, r)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("control_owner_reminder: rows error: %w", err)
		}

		if len(reminders) == 0 {
			log.Info().Msg("control_owner_reminder: no controls due in 7 days")
			return nil
		}

		smtpAddr := cfg.SMTPHost + ":" + cfg.SMTPPort
		if smtpAddr == ":" {
			smtpAddr = "localhost:25"
		}
		var smtpAuth smtp.Auth
		if cfg.SMTPUser != "" && cfg.SMTPPass != "" {
			smtpAuth = smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPHost)
		}

		frontendURL := cfg.FrontendURL
		if frontendURL == "" {
			frontendURL = "https://sec.norvikops.de"
		}

		sent := 0
		for _, r := range reminders {
			if !reEmail.MatchString(r.Responsible) {
				log.Debug().
					Str("control_id", r.ControlDBID).
					Str("responsible", r.Responsible).
					Msg("control_owner_reminder: not a valid e-mail, skipping")
				continue
			}

			subject := fmt.Sprintf("Erinnerung: Control %s fällig in 7 Tagen", r.ControlID)
			link := fmt.Sprintf("%s/vaktcomply/controls/%s", frontendURL, r.ControlDBID)
			dueDateStr := r.DueDate.Format("02.01.2006")

			var buf bytes.Buffer
			buf.WriteString(`<!DOCTYPE html><html><body style="font-family:sans-serif;color:#1a202c;">`)
			buf.WriteString(`<h2 style="color:#2b6cb0;">Vakt — Control-Erinnerung</h2>`)
			buf.WriteString(`<p>Das folgende Control ist in <strong>7 Tagen</strong> fällig:</p>`)
			buf.WriteString(`<table border="0" cellpadding="6"><tbody>`)
			fmt.Fprintf(&buf, `<tr><td><strong>Control:</strong></td><td>%s — %s</td></tr>`, r.ControlID, r.Title)
			fmt.Fprintf(&buf, `<tr><td><strong>Fälligkeitsdatum:</strong></td><td>%s</td></tr>`, dueDateStr)
			fmt.Fprintf(&buf, `<tr><td><strong>Link:</strong></td><td><a href="%s">Control öffnen</a></td></tr>`, link)
			buf.WriteString(`</tbody></table>`)
			buf.WriteString(`<p style="color:#718096;font-size:0.85em;">Diese E-Mail wurde automatisch von Vakt versandt.</p>`)
			buf.WriteString(`</body></html>`)

			headers := fmt.Sprintf(
				"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n",
				cfg.SMTPFrom, r.Responsible, subject,
			)
			msg := []byte(headers + buf.String())

			if sendErr := smtp.SendMail(smtpAddr, smtpAuth, cfg.SMTPFrom, []string{r.Responsible}, msg); sendErr != nil {
				log.Warn().
					Err(sendErr).
					Str("control_id", r.ControlDBID).
					Str("to", r.Responsible).
					Msg("control_owner_reminder: send failed")
				continue
			}
			sent++
			log.Info().
				Str("control_id", r.ControlDBID).
				Str("to", r.Responsible).
				Msg("control_owner_reminder: sent")
		}

		log.Info().
			Int("sent", sent).
			Int("total", len(reminders)).
			Msg("control_owner_reminder: completed")
		return nil
	}
}

// handleAutoEnrollment processes aware:auto_enrollment jobs.
// For each active enrollment rule matching the trigger type, it enrolls the
// employee into the target campaign unless they are already enrolled.
func handleAutoEnrollment(pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var payload vaktaware.AutoEnrollmentPayload
		if err := json.Unmarshal(t.Payload(), &payload); err != nil {
			return fmt.Errorf("parse auto_enrollment payload: %w", err)
		}
		svc := vaktaware.NewService(pool, vaktaware.SMTPConfig{})
		if err := svc.HandleAutoEnrollment(ctx, payload); err != nil {
			return fmt.Errorf("handle auto-enrollment org=%s: %w", payload.OrgID, err)
		}
		log.Info().
			Str("org_id", payload.OrgID).
			Str("trigger", payload.TriggerType).
			Str("employee_id", payload.EmployeeID).
			Msg("auto_enrollment: processed")
		return nil
	}
}

// handleORP3EvidenceSync processes aware:orp3_evidence_sync jobs.
// It evaluates BSI ORP.3 compliance for the org and writes the result as evidence.
func handleORP3EvidenceSync(pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var payload struct {
			OrgID string `json:"org_id"`
		}
		if err := json.Unmarshal(t.Payload(), &payload); err != nil {
			return fmt.Errorf("parse orp3_evidence_sync payload: %w", err)
		}
		svc := vaktaware.NewService(pool, vaktaware.SMTPConfig{})
		if err := svc.RunORP3EvidenceSync(ctx, payload.OrgID); err != nil {
			return fmt.Errorf("orp3_evidence_sync org=%s: %w", payload.OrgID, err)
		}
		log.Info().Str("org_id", payload.OrgID).Msg("orp3_evidence_sync: completed")
		return nil
	}
}
