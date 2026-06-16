// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	"github.com/matharnica/vakt/internal/config"
	"github.com/matharnica/vakt/internal/modules/vaktcomply"
	"github.com/matharnica/vakt/internal/modules/vaktscan"
	"github.com/matharnica/vakt/internal/services/crossevidence"
	"github.com/matharnica/vakt/internal/shared/controltests"
	"github.com/matharnica/vakt/internal/shared/notify"
	"github.com/matharnica/vakt/internal/shared/platform/events"
)

// handleAutoEvidence creates SecVitals evidence entries for patch-management
// controls when a SecPulse finding is resolved.
func handleAutoEvidence(cfg *config.Config, pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var payload vaktscan.AutoEvidencePayload
		if err := json.Unmarshal(t.Payload(), &payload); err != nil {
			return fmt.Errorf("parse auto_evidence payload: %w", err)
		}

		repo := vaktcomply.NewRepository(pool)
		controls, err := repo.FindPatchControls(ctx, payload.OrgID)
		if err != nil || len(controls) == 0 {
			// No patch controls found — not an error, nothing to do.
			return nil
		}

		title := fmt.Sprintf("Auto-collected: Patch verified — %s", payload.Title)
		if payload.CVE != "" {
			title = fmt.Sprintf("Auto-collected: %s patched", payload.CVE)
		}

		collectorData, _ := json.Marshal(map[string]string{
			"finding_id": payload.FindingID,
			"cve":        payload.CVE,
			"source":     "vaktscan",
		})

		for _, ctrl := range controls {
			if _, evidErr := repo.AddCollectorEvidence(ctx, payload.OrgID, ctrl.ID, "", "automated", title, collectorData); evidErr != nil {
				log.Warn().
					Err(evidErr).
					Str("control_id", ctrl.ID).
					Str("finding_id", payload.FindingID).
					Msg("auto_evidence: failed to add evidence for control")
			}
		}

		log.Info().
			Str("finding_id", payload.FindingID).
			Str("org_id", payload.OrgID).
			Int("controls_updated", len(controls)).
			Msg("vaktscan→vaktcomply: auto-evidence created for resolved finding")

		return nil
	}
}

// handleRecordEvidence records cross-module compliance evidence in SecVitals.
// Triggered by vaktaware (training), vaktprivacy (DSR), and vaktvault (rotation) events.
func handleRecordEvidence(pool *pgxpool.Pool) asynq.HandlerFunc {
	// keywords per source module → relevant SecVitals control domains
	sourceKeywords := map[string][]string{
		"vaktaware":   {"training", "awareness", "schulung", "bewusstsein"},
		"vaktprivacy": {"datenschutz", "privacy", "dsar", "betroffene"},
		"vaktvault":   {"access", "password", "secret", "rotation", "credential"},
		"vaktscan":    {"kryptographie", "zertifikat", "tls", "certificate", "pki"},
	}
	return func(ctx context.Context, t *asynq.Task) error {
		var payload crossevidence.EvidencePayload
		if err := json.Unmarshal(t.Payload(), &payload); err != nil {
			return fmt.Errorf("parse evidence payload: %w", err)
		}

		// S88-8: scanner finding-created events use the dedicated, idempotent
		// Scan→Comply bridge (maps to A.8.8/A.8.9 via ck_scan_evidence_map) rather
		// than the generic keyword path, so re-scans never duplicate evidence.
		if payload.ResourceType == events.ResourceTypeFindingCreated {
			svc := vaktcomply.NewService(pool)
			n, err := svc.RecordScanFindingEvidence(ctx, payload.OrgID, payload.ResourceID, payload.Title)
			if err != nil {
				log.Warn().Err(err).Str("org_id", payload.OrgID).Msg("crossevidence: scan bridge failed")
				return nil
			}
			log.Info().Str("org_id", payload.OrgID).Int("controls_updated", n).Msg("crossevidence: scan finding evidence recorded")
			return nil
		}

		keywords := sourceKeywords[payload.Source]
		if len(keywords) == 0 {
			return nil
		}

		repo := vaktcomply.NewRepository(pool)
		controls, err := repo.FindControlsByKeywords(ctx, payload.OrgID, keywords)
		if err != nil || len(controls) == 0 {
			log.Info().
				Str("org_id", payload.OrgID).
				Str("source", payload.Source).
				Msg("crossevidence: no matching controls found")
			return nil
		}

		collectorData, _ := json.Marshal(map[string]string{
			"source":        payload.Source,
			"resource_type": payload.ResourceType,
			"resource_id":   payload.ResourceID,
		})

		for _, ctrl := range controls {
			if _, evidErr := repo.AddCollectorEvidence(
				ctx, payload.OrgID, ctrl.ID, "", "automated",
				payload.Title, collectorData,
			); evidErr != nil {
				log.Warn().
					Err(evidErr).
					Str("control_id", ctrl.ID).
					Str("source", payload.Source).
					Msg("crossevidence: add evidence failed")
			}
		}

		log.Info().
			Str("org_id", payload.OrgID).
			Str("source", payload.Source).
			Str("resource_type", payload.ResourceType).
			Int("controls_updated", len(controls)).
			Msg("crossevidence: evidence recorded")
		return nil
	}
}

// handleEvidenceExpiryAlert sends per-evidence in-app notifications for evidence
// expiring within 30 days that has not yet been notified (expiry_notified_at IS NULL).
// Runs daily at 09:00 UTC. Uses errgroup with limit 5 to process orgs in parallel.
func handleEvidenceExpiryAlert(pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		rows, err := pool.Query(ctx, `SELECT id::text FROM organizations`)
		if err != nil {
			return fmt.Errorf("evidence_expiry_alert: list orgs: %w", err)
		}
		defer rows.Close()

		var orgIDs []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				continue
			}
			orgIDs = append(orgIDs, id)
		}
		if err := rows.Err(); err != nil {
			return err
		}

		repo := vaktcomply.NewRepository(pool)
		threshold := time.Now().UTC().AddDate(0, 0, 30)

		g, gCtx := errgroup.WithContext(ctx)
		sem := make(chan struct{}, 5)
		for _, orgID := range orgIDs {
			orgID := orgID
			sem <- struct{}{}
			g.Go(func() error {
				defer func() { <-sem }()
				items, err := repo.GetUnnotifiedExpiringEvidence(gCtx, orgID, threshold)
				if err != nil || len(items) == 0 {
					return nil
				}
				// Send one in-app notification per evidence item for actionable granularity.
				notifiedIDs := make([]string, 0, len(items))
				for _, item := range items {
					dateStr := item.ExpiresAt.Format("02.01.2006")
					msg := fmt.Sprintf(
						"Evidence für Control '%s' läuft am %s ab und muss erneuert werden.",
						item.ControlTitle, dateStr,
					)
					notify.Send(gCtx, pool, orgID, "Nachweis läuft ab", msg, "warning", "vaktcomply")
					notifiedIDs = append(notifiedIDs, item.ID)
				}
				// Mark all notified items so we do not re-notify on subsequent runs.
				if markErr := repo.MarkEvidenceExpiryNotified(gCtx, notifiedIDs); markErr != nil {
					log.Error().Err(markErr).Str("org_id", orgID).Msg("evidence_expiry_alert: mark notified")
				}
				log.Info().Str("org_id", orgID).Int("count", len(notifiedIDs)).Msg("evidence_expiry_alert: sent")
				return nil
			})
		}
		return g.Wait()
	}
}

// handleIncidentDeadlineCheck iterates all organisations and fires in-app notifications
// for any DORA/NIS2 incident deadline that is overdue and has not yet been reported.
// Uses errgroup with limit 5 for parallel org processing.
func handleIncidentDeadlineCheck(pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		rows, err := pool.Query(ctx, `SELECT id::text FROM organizations`)
		if err != nil {
			return fmt.Errorf("incident_deadline_check: list orgs: %w", err)
		}
		defer rows.Close()

		var orgIDs []string
		for rows.Next() {
			var orgID string
			if err := rows.Scan(&orgID); err != nil {
				continue
			}
			orgIDs = append(orgIDs, orgID)
		}
		if err := rows.Err(); err != nil {
			return err
		}

		svc := vaktcomply.NewService(pool)

		g, gCtx := errgroup.WithContext(ctx)
		sem := make(chan struct{}, 5)
		for _, orgID := range orgIDs {
			orgID := orgID
			sem <- struct{}{}
			g.Go(func() error {
				defer func() { <-sem }()
				if err := svc.CheckOverdueDeadlines(gCtx, orgID); err != nil {
					log.Error().Err(err).Str("org_id", orgID).Msg("incident_deadline_check: failed")
				}
				return nil
			})
		}
		return g.Wait()
	}
}

// handleCertExpiryCheck sends in-app notifications for supplier certificates expiring within 30 days.
// Runs daily at 07:00 UTC. Uses errgroup with limit 5 for parallel org processing.
func handleCertExpiryCheck(pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		rows, err := pool.Query(ctx, `SELECT id::text, name FROM organizations`)
		if err != nil {
			return fmt.Errorf("cert_expiry_check: list orgs: %w", err)
		}
		defer rows.Close()

		type orgRow struct {
			id   string
			name string
		}
		var orgs []orgRow
		for rows.Next() {
			var o orgRow
			if err := rows.Scan(&o.id, &o.name); err != nil {
				continue
			}
			orgs = append(orgs, o)
		}
		if err := rows.Err(); err != nil {
			return err
		}

		repo := vaktcomply.NewRepository(pool)
		threshold := time.Now().UTC().AddDate(0, 0, 30)

		g, gCtx := errgroup.WithContext(ctx)
		sem := make(chan struct{}, 5)
		for _, o := range orgs {
			o := o
			sem <- struct{}{}
			g.Go(func() error {
				defer func() { <-sem }()
				items, err := repo.FindExpiringCerts(gCtx, o.id, threshold)
				if err != nil || len(items) == 0 {
					return nil
				}
				msg := fmt.Sprintf("%d Lieferanten-Zertifikate laufen in den nächsten 30 Tagen ab.", len(items))
				notify.Send(gCtx, pool, o.id, "Lieferanten-Zertifikate laufen ab", msg, "warning", "vaktcomply")
				log.Info().Str("org_id", o.id).Int("count", len(items)).Msg("cert_expiry_check: sent")
				return nil
			})
		}
		return g.Wait()
	}
}

// handleCCMRunDue runs all enabled CCM checks that are past their interval.
func handleCCMRunDue(pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		svc := vaktcomply.NewService(pool)
		if err := svc.RunDueCCMChecks(ctx); err != nil {
			log.Error().Err(err).Msg("ccm_run_due: failed")
			return err
		}
		return nil
	}
}

// handleScoreSnapshot records daily compliance score snapshots for all organisations.
// The snapshots power the trend chart on the dashboard.
func handleScoreSnapshot(pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		svc := vaktcomply.NewService(pool)
		if err := svc.RecordScoreSnapshotForAllOrgs(ctx); err != nil {
			log.Error().Err(err).Msg("score_snapshot: failed")
			return err
		}
		log.Info().Msg("score_snapshot: completed")
		return nil
	}
}

func handleControlTestCheck(pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		if err := controltests.CheckOverdueControlTests(ctx, pool); err != nil {
			log.Error().Err(err).Msg("control_test_check: failed")
			return err
		}
		return nil
	}
}

// handleDORADeadlineStatus computes and persists the DORA Ampel-Status for all
// IKT-DORA incidents across all orgs. Runs every 5 minutes (S37-4).
func handleDORADeadlineStatus(pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		// Exclude ephemeral demo orgs (slug LIKE 'demo-%'): they have no DORA incidents
		// and this task runs every 5 minutes — processing thousands of short-lived demo
		// orgs would dominate DB load on the public demo server.
		rows, err := pool.Query(ctx, `SELECT id::text FROM organizations WHERE slug NOT LIKE 'demo-%'`)
		if err != nil {
			return fmt.Errorf("dora_deadline_status: list orgs: %w", err)
		}
		defer rows.Close()

		var orgIDs []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				continue
			}
			orgIDs = append(orgIDs, id)
		}

		svc := vaktcomply.NewService(pool)
		g, gCtx := errgroup.WithContext(ctx)
		sem := make(chan struct{}, 5)
		for _, orgID := range orgIDs {
			orgID := orgID
			sem <- struct{}{}
			g.Go(func() error {
				defer func() { <-sem }()
				if err := svc.UpdateDORADeadlineStatus(gCtx, orgID); err != nil {
					log.Error().Err(err).Str("org_id", orgID).Msg("dora_deadline_status: update failed")
				}
				return nil
			})
		}
		return g.Wait()
	}
}

// handleNIS2ObligationCheck iterates all organisations and fires in-app/email notifications
// for NIS2 incidents where the classify-reporting wizard has set obligation = "probably".
// Runs daily. S39-2.
func handleNIS2ObligationCheck(pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		rows, err := pool.Query(ctx, `SELECT id::text FROM organizations`)
		if err != nil {
			return fmt.Errorf("nis2_obligation_check: list orgs: %w", err)
		}
		defer rows.Close()

		var orgIDs []string
		for rows.Next() {
			var orgID string
			if err := rows.Scan(&orgID); err != nil {
				continue
			}
			orgIDs = append(orgIDs, orgID)
		}
		if err := rows.Err(); err != nil {
			return err
		}

		svc := vaktcomply.NewService(pool)
		g, gCtx := errgroup.WithContext(ctx)
		sem := make(chan struct{}, 5)
		for _, orgID := range orgIDs {
			orgID := orgID
			sem <- struct{}{}
			g.Go(func() error {
				defer func() { <-sem }()
				if err := svc.CheckNIS2ObligationDeadlines(gCtx, orgID); err != nil {
					log.Error().Err(err).Str("org_id", orgID).Msg("nis2_obligation_check: failed")
				}
				return nil
			})
		}
		return g.Wait()
	}
}

// handleEvidenceFreshnessCheck runs daily to find controls whose evidence has gone stale
// (all evidence older than 90 days) and creates AI insights for each such control.
// S52-1.
func handleEvidenceFreshnessCheck(cfg *config.Config, pool *pgxpool.Pool) asynq.HandlerFunc {
	const staleThresholdDays = 90
	return func(ctx context.Context, _ *asynq.Task) error {
		rows, err := pool.Query(ctx, `SELECT id::text FROM organizations`)
		if err != nil {
			return fmt.Errorf("evidence_freshness_check: list orgs: %w", err)
		}
		defer rows.Close()

		var orgIDs []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				continue
			}
			orgIDs = append(orgIDs, id)
		}
		if err := rows.Err(); err != nil {
			return err
		}

		repo := vaktcomply.NewRepository(pool)
		totalInsights := 0

		for _, orgID := range orgIDs {
			stale, err := repo.FindStaleEvidenceControls(ctx, orgID, staleThresholdDays)
			if err != nil {
				log.Error().Err(err).Str("org_id", orgID).Msg("evidence_freshness_check: find stale controls failed")
				continue
			}
			for _, ctrl := range stale {
				ctrlID := ctrl.ControlID
				title := "Veraltete Evidence: " + ctrl.ControlTitle
				message := fmt.Sprintf(
					"Der Control \"%s\" hat seit %d Tagen keine neue Evidence erhalten. Bitte aktualisieren Sie die Nachweise, um die Compliance-Dokumentation aktuell zu halten.",
					ctrl.ControlTitle, ctrl.DaysSince,
				)
				if upsertErr := repo.UpsertAIInsight(ctx, orgID, "evidence_stale", title, message, &ctrlID, nil, nil, 2, nil); upsertErr != nil {
					log.Warn().Err(upsertErr).Str("org_id", orgID).Str("control_id", ctrlID).Msg("evidence_freshness_check: upsert insight failed")
					continue
				}
				totalInsights++
			}
		}

		log.Info().
			Int("orgs", len(orgIDs)).
			Int("insights_created", totalInsights).
			Msg("evidence_freshness_check: completed")
		return nil
	}
}

// aiEvidenceSuggestionPayload is the task payload for the AI evidence suggestion job.
type aiEvidenceSuggestionPayload struct {
	FindingID    string `json:"finding_id"`
	OrgID        string `json:"org_id"`
	Severity     string `json:"severity"`
	FindingTitle string `json:"finding_title"`
}

// handleAIEvidenceSuggestion creates AI insights suggesting which controls need evidence
// after a finding is resolved. S52-5.
func handleAIEvidenceSuggestion(cfg *config.Config, pool *pgxpool.Pool) asynq.HandlerFunc {
	// Keyword mapping from severity to relevant control keywords.
	severityKeywords := map[string][]string{
		"critical": {"access_control", "patch", "vulnerability"},
		"high":     {"vulnerability", "patch"},
		"medium":   {"patch", "vulnerability"},
	}
	return func(ctx context.Context, t *asynq.Task) error {
		var payload aiEvidenceSuggestionPayload
		if err := json.Unmarshal(t.Payload(), &payload); err != nil {
			return fmt.Errorf("ai_evidence_suggestion: parse payload: %w", err)
		}

		keywords := severityKeywords[payload.Severity]
		if len(keywords) == 0 {
			// Unknown severity — use generic keywords.
			keywords = []string{"vulnerability", "patch"}
		}

		repo := vaktcomply.NewRepository(pool)
		controls, err := repo.FindControlsByKeywords(ctx, payload.OrgID, keywords)
		if err != nil {
			return fmt.Errorf("ai_evidence_suggestion: find controls: %w", err)
		}

		// Limit to 3 suggestions.
		if len(controls) > 3 {
			controls = controls[:3]
		}

		findingID := payload.FindingID
		for _, ctrl := range controls {
			ctrlID := ctrl.ID
			title := "Evidence-Empfehlung: " + ctrl.Title
			message := fmt.Sprintf(
				"Das Finding \"%s\" (Schweregrad: %s) wurde als behoben markiert. Erwägen Sie, Evidence für den Control \"%s\" zu hinterlegen, um die Behebung zu dokumentieren.",
				payload.FindingTitle, payload.Severity, ctrl.Title,
			)
			if upsertErr := repo.UpsertAIInsight(ctx, payload.OrgID, "evidence_suggestion", title, message, &ctrlID, nil, &findingID, 2, nil); upsertErr != nil {
				log.Warn().Err(upsertErr).Str("org_id", payload.OrgID).Str("control_id", ctrlID).Msg("ai_evidence_suggestion: upsert insight failed")
			}
		}

		log.Info().
			Str("finding_id", payload.FindingID).
			Str("org_id", payload.OrgID).
			Int("suggestions", len(controls)).
			Msg("ai_evidence_suggestion: completed")
		return nil
	}
}
