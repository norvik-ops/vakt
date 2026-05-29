// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	"github.com/matharnica/vakt/internal/config"
	"github.com/matharnica/vakt/internal/modules/vaktprivacy"
	"github.com/matharnica/vakt/internal/modules/vaktcomply"
	"github.com/matharnica/vakt/internal/services/alerting"
)

// handleAVVExpiryCheck handles vaktprivacy:avv_expiry_check jobs.
// Marks overdue AVVs as expired and fires avv.expired alerts per org.
// Uses errgroup with limit 5 to process orgs in parallel.
func handleAVVExpiryCheck(cfg *config.Config, pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		svc := vaktprivacy.NewService(pool, asynq.RedisClientOpt{})
		if err := svc.CheckAVVExpiry(ctx); err != nil {
			log.Error().Err(err).Msg("avv expiry check failed")
			return err
		}

		// Fire avv.expired alert once per org that has newly-expired AVVs,
		// rate-limited to once per 24 hours via notification_alert_state.
		if cfg == nil || cfg.SecretKey == "" {
			return nil
		}
		rows, queryErr := pool.Query(ctx, `
			SELECT DISTINCT a.org_id::text
			FROM po_avvs a
			WHERE a.status = 'expired'
			  AND NOT EXISTS (
			    SELECT 1 FROM notification_alert_state s
			    WHERE s.org_id = a.org_id
			      AND s.event_type = $1
			      AND s.last_fired_at > NOW() - INTERVAL '24 hours'
			  )
		`, alerting.EventAVVExpired)
		if queryErr != nil {
			log.Error().Err(queryErr).Msg("avv_expiry_check: query failed")
			return nil
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

		alertSvc := alerting.NewService(pool, workerKey(cfg, "vakt-alert-v1"), alerting.SMTPConfig{Host: cfg.SMTPHost, Port: cfg.SMTPPort, User: cfg.SMTPUser, Pass: cfg.SMTPPass, From: cfg.SMTPFrom})

		g, gCtx := errgroup.WithContext(ctx)
		sem := make(chan struct{}, 5)
		for _, orgID := range orgIDs {
			orgID := orgID
			sem <- struct{}{}
			g.Go(func() error {
				defer func() { <-sem }()
				alertSvc.Fire(gCtx, orgID, alerting.EventAVVExpired, map[string]any{
					"message": "One or more AVVs have expired and must be renewed.",
				})
				_, _ = pool.Exec(gCtx, `
					INSERT INTO notification_alert_state (org_id, event_type, last_fired_at)
					VALUES ($1::uuid, $2, NOW())
					ON CONFLICT (org_id, event_type) DO UPDATE SET last_fired_at = NOW()
				`, orgID, alerting.EventAVVExpired)
				return nil
			})
		}
		_ = g.Wait()
		return nil
	}
}

// handleBreachIncidentCreate creates a linked SecVitals incident when a breach is reported.
// This is the integration point between SecPrivacy (breach) and SecVitals (incident register).
func handleBreachIncidentCreate(cfg *config.Config, pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var payload vaktprivacy.BreachIncidentPayload
		if err := json.Unmarshal(t.Payload(), &payload); err != nil {
			return fmt.Errorf("parse breach_incident payload: %w", err)
		}

		repo := vaktcomply.NewRepository(pool)
		breachID := payload.BreachID
		input := vaktcomply.CreateIncidentInput{
			Title:           "[Datenschutzverletzung] " + payload.Title,
			Description:     payload.Description,
			Severity:        "critical",
			DiscoveredAt:    payload.DiscoveredAt,
			AffectedSystems: []string{},
			BreachID:        &breachID,
		}

		incident, err := repo.CreateIncident(ctx, payload.OrgID, input, nil)
		if err != nil {
			log.Error().Err(err).Str("breach_id", payload.BreachID).Msg("failed to create incident from breach")
			return fmt.Errorf("create incident from breach: %w", err)
		}

		log.Info().
			Str("breach_id", payload.BreachID).
			Str("incident_id", incident.ID).
			Str("org_id", payload.OrgID).
			Msg("vaktprivacy→vaktcomply: incident created from breach")
		return nil
	}
}

// handleDSROverdueCheck fires alerting events for DSR requests past their due date.
// Uses errgroup with limit 5 to process orgs in parallel.
func handleDSROverdueCheck(cfg *config.Config, pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		if cfg == nil || cfg.SecretKey == "" {
			return nil
		}

		rows, err := pool.Query(ctx, `
			SELECT DISTINCT d.org_id::text
			FROM po_dsr d
			WHERE d.status NOT IN ('completed','rejected')
			  AND d.due_date < NOW()
			  AND NOT EXISTS (
			    SELECT 1 FROM notification_alert_state s
			    WHERE s.org_id = d.org_id
			      AND s.event_type = $1
			      AND s.last_fired_at > NOW() - INTERVAL '24 hours'
			  )
		`, alerting.EventDSROverdue)
		if err != nil {
			log.Error().Err(err).Msg("dsr_overdue_check: query failed")
			return nil
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

		alertSvc := alerting.NewService(pool, workerKey(cfg, "vakt-alert-v1"), alerting.SMTPConfig{Host: cfg.SMTPHost, Port: cfg.SMTPPort, User: cfg.SMTPUser, Pass: cfg.SMTPPass, From: cfg.SMTPFrom})

		g, gCtx := errgroup.WithContext(ctx)
		sem := make(chan struct{}, 5)
		for _, orgID := range orgIDs {
			orgID := orgID
			sem <- struct{}{}
			g.Go(func() error {
				defer func() { <-sem }()
				alertSvc.Fire(gCtx, orgID, alerting.EventDSROverdue, map[string]any{
					"message": "One or more DSR requests have exceeded their due date.",
				})
				_, _ = pool.Exec(gCtx, `
					INSERT INTO notification_alert_state (org_id, event_type, last_fired_at)
					VALUES ($1::uuid, $2, NOW())
					ON CONFLICT (org_id, event_type) DO UPDATE SET last_fired_at = NOW()
				`, orgID, alerting.EventDSROverdue)
				return nil
			})
		}
		_ = g.Wait()
		return nil
	}
}
