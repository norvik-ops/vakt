// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	"github.com/matharnica/vakt/internal/config"
	"github.com/matharnica/vakt/internal/modules/vaktcomply"
	"github.com/matharnica/vakt/internal/shared/controltests"
	"github.com/matharnica/vakt/internal/shared/notify"
)

// handleEvidenceExpiryAlert sends per-evidence in-app notifications for evidence
// expiring within 30 days that has not yet been notified (expiry_notified_at IS NULL).
// Runs daily at 09:00 UTC. Uses errgroup with limit 5 to process orgs in parallel.
func handleEvidenceExpiryAlert(pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		orgIDs, err := nonDemoOrgIDs(ctx, pool)
		if err != nil {
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
					// R1-W4A-N1: nur ein bestaetigter Versand kommt in
					// notifiedIDs. Die Auswahlabfrage ist
					// GetUnnotifiedExpiringEvidence — ein Nachweis, der ohne
					// zugestellte Meldung als benachrichtigt markiert wird,
					// taucht nie wieder auf, und der Ablauf des Nachweises
					// bleibt dauerhaft unbemerkt.
					if err := notify.Send(gCtx, pool, orgID, "Nachweis läuft ab", msg, "warning", "vaktcomply"); err != nil {
						log.Error().Err(err).Str("org_id", orgID).Str("evidence_id", item.ID).
							Msg("evidence_expiry_alert: Meldung NICHT zugestellt — Nachweis bleibt unmarkiert und wird beim naechsten Lauf erneut versucht")
						continue
					}
					notifiedIDs = append(notifiedIDs, item.ID)
				}
				// Mark all notified items so we do not re-notify on subsequent runs.
				if markErr := repo.MarkEvidenceExpiryNotified(gCtx, notifiedIDs); markErr != nil {
					log.Error().Err(markErr).Str("org_id", orgID).Msg("evidence_expiry_alert: mark notified")
				}
				log.Info().Str("org_id", orgID).
					Int("count", len(notifiedIDs)).Int("found", len(items)).
					Msg("evidence_expiry_alert: sent")
				return nil
			})
		}
		return g.Wait()
	}
}

// handleIncidentDeadlineCheck iterates all organisations and fires in-app notifications
// for any DORA/NIS2 incident deadline that is overdue and has not yet been reported.
// Uses errgroup with limit 5 for parallel org processing.
func handleIncidentDeadlineCheck(cfg *config.Config, pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		orgIDs, err := nonDemoOrgIDs(ctx, pool)
		if err != nil {
			return err
		}

		svc := newComplyService(cfg, pool)

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
func handleCertExpiryCheck(cfg *config.Config, pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		orgs, err := nonDemoOrgs(ctx, pool)
		if err != nil {
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
				if err != nil {
					log.Error().Err(err).Str("org_id", o.id).
						Msg("cert_expiry_check: Abfrage fehlgeschlagen — es wird NICHT gewarnt")
					return nil
				}
				if len(items) == 0 {
					return nil
				}

				// L3-02: die Abfrage hat keine untere Grenze, liefert also auch
				// laengst abgelaufene Zertifikate. Der Text sagte fuer alle
				// "laufen in den naechsten 30 Tagen ab" — ein vor drei Jahren
				// abgelaufenes Zertifikat meldete sich damit taeglich als
				// bevorstehend. Abgelaufen und bald ablaufend sind zwei
				// verschiedene Aussagen und werden getrennt gemeldet.
				today := time.Now().UTC().Truncate(24 * time.Hour)
				var expired, upcoming int
				for _, it := range items {
					if it.CertExpiryDate.Before(today) {
						expired++
					} else {
						upcoming++
					}
				}
				// R1-W4A-N1: Hier gibt es keine Marke, aber es gab ein
				// bedingungsloses „sent" im Log. Zugestellt wird gezaehlt,
				// nicht behauptet.
				delivered := 0
				if upcoming > 0 {
					msg := fmt.Sprintf("%d Lieferanten-Zertifikate laufen in den nächsten 30 Tagen ab.", upcoming)
					if err := notify.Send(gCtx, pool, o.id, "Lieferanten-Zertifikate laufen ab", msg, "warning", "vaktcomply"); err != nil {
						log.Error().Err(err).Str("org_id", o.id).Msg("cert_expiry_check: Meldung über ablaufende Zertifikate NICHT zugestellt")
					} else {
						delivered++
					}
				}
				if expired > 0 {
					msg := fmt.Sprintf("%d Lieferanten-Zertifikate sind bereits abgelaufen.", expired)
					if err := notify.Send(gCtx, pool, o.id, "Lieferanten-Zertifikate abgelaufen", msg, "warning", "vaktcomply"); err != nil {
						log.Error().Err(err).Str("org_id", o.id).Msg("cert_expiry_check: Meldung über abgelaufene Zertifikate NICHT zugestellt")
					} else {
						delivered++
					}
				}
				log.Info().Str("org_id", o.id).
					Int("expiring", upcoming).Int("expired", expired).Int("delivered", delivered).
					Msg("cert_expiry_check: done")
				return nil
			})
		}
		return g.Wait()
	}
}

// handleCCMRunDue runs all enabled CCM checks that are past their interval.
func handleCCMRunDue(cfg *config.Config, pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		svc := newComplyService(cfg, pool)
		if err := svc.RunDueCCMChecks(ctx); err != nil {
			log.Error().Err(err).Msg("ccm_run_due: failed")
			return err
		}
		return nil
	}
}

// handleScoreSnapshot records daily compliance score snapshots for all organisations.
// The snapshots power the trend chart on the dashboard.
func handleScoreSnapshot(cfg *config.Config, pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		svc := newComplyService(cfg, pool)
		if err := svc.RecordScoreSnapshotForAllOrgs(ctx); err != nil {
			log.Error().Err(err).Msg("score_snapshot: failed")
			return err
		}
		log.Info().Msg("score_snapshot: completed")
		return nil
	}
}

func handleControlTestCheck(cfg *config.Config, pool *pgxpool.Pool) asynq.HandlerFunc {
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
func handleDORADeadlineStatus(cfg *config.Config, pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		orgIDs, err := nonDemoOrgIDs(ctx, pool)
		if err != nil {
			return err
		}

		svc := newComplyService(cfg, pool)
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
func handleNIS2ObligationCheck(cfg *config.Config, pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		orgIDs, err := nonDemoOrgIDs(ctx, pool)
		if err != nil {
			return err
		}

		svc := newComplyService(cfg, pool)
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
		orgIDs, err := nonDemoOrgIDs(ctx, pool)
		if err != nil {
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
