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
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/modules/vaktscan"
)

// R1-36b-SC06: recurring scan schedules (vb_scan_schedules) had no executor and
// therefore never ran. This file adds one, following the scheduledreports
// ProcessDue pattern: a per-minute cron selects every active schedule whose
// next_run has come due, enqueues the matching scanner task, and advances
// next_run from the schedule's cron expression.

// dueScanSchedule is one active, due schedule joined with the asset fields the
// scanner payload needs (name + external target URL).
type dueScanSchedule struct {
	ID        string
	OrgID     string
	AssetID   string
	Scanner   string
	CronExpr  string
	AssetName string
	TargetURL string
}

// scanScheduleStore is the DB seam for the executor. The production impl uses
// raw SQL against pgx; tests provide a fake so ProcessDueScanSchedules can be
// exercised without a live database.
type scanScheduleStore interface {
	listDueScanSchedules(ctx context.Context) ([]dueScanSchedule, error)
	createScanRow(ctx context.Context, orgID, assetID, scanner string) (scanID string, err error)
	markScanScheduleRun(ctx context.Context, id string, nextRun time.Time) error
}

// scanScheduleEnqueuer abstracts enqueuing a scan task so the executor can be
// tested without a real asynq client.
type scanScheduleEnqueuer interface {
	enqueueScan(taskType string, payload []byte) error
}

// computeNextScanRun parses a 5-field standard cron expression and returns the
// next activation strictly after `from`. A malformed expression yields an error
// so the caller skips the schedule instead of enqueuing a scan it could never
// advance past — which would re-fire the scan on every tick.
func computeNextScanRun(cronExpr string, from time.Time) (time.Time, error) {
	sched, err := cron.ParseStandard(cronExpr)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse cron %q: %w", cronExpr, err)
	}
	return sched.Next(from), nil
}

// scanTaskTypeForScanner maps a scanner name to its Asynq task type. Mirrors the
// unexported taskTypeForScanner in the vaktscan service, using the exported
// task-type constants. Returns ok=false for an unknown scanner.
func scanTaskTypeForScanner(scanner string) (string, bool) {
	switch scanner {
	case "trivy":
		return vaktscan.TaskScanTrivy, true
	case "nuclei":
		return vaktscan.TaskScanNuclei, true
	case "openvas":
		return vaktscan.TaskScanOpenVAS, true
	default:
		return "", false
	}
}

// ProcessDueScanSchedules enqueues a scan for every due schedule and advances
// its next_run from the cron expression. One failing schedule never aborts the
// others: every per-schedule error is logged and the loop continues.
//
// Ordering per schedule is deliberate:
//  1. Compute next_run first. A malformed cron means we can never advance
//     next_run, so we must not enqueue — otherwise the schedule stays due and
//     re-fires every minute.
//  2. Create the vb_scans row (the scanner updates it by scan_id, exactly like
//     the manual TriggerScan path).
//  3. Enqueue. On enqueue failure we do NOT advance next_run, so the next tick
//     retries. The created scan row stays 'pending' — same outcome as a manual
//     scan whose enqueue failed.
func ProcessDueScanSchedules(ctx context.Context, store scanScheduleStore, enq scanScheduleEnqueuer) error {
	due, err := store.listDueScanSchedules(ctx)
	if err != nil {
		return fmt.Errorf("list due scan schedules: %w", err)
	}

	now := time.Now().UTC()
	for _, s := range due {
		taskType, ok := scanTaskTypeForScanner(s.Scanner)
		if !ok {
			log.Error().Str("schedule_id", s.ID).Str("scanner", s.Scanner).
				Msg("scan_schedule: unknown scanner, skipping")
			continue
		}

		nextRun, nrErr := computeNextScanRun(s.CronExpr, now)
		if nrErr != nil {
			log.Error().Err(nrErr).Str("schedule_id", s.ID).
				Msg("scan_schedule: invalid cron expression, skipping")
			continue
		}

		scanID, csErr := store.createScanRow(ctx, s.OrgID, s.AssetID, s.Scanner)
		if csErr != nil {
			log.Error().Err(csErr).Str("schedule_id", s.ID).
				Msg("scan_schedule: create scan row failed, skipping")
			continue
		}

		payloadBytes, mErr := json.Marshal(vaktscan.ScanPayload{
			ScanID:    scanID,
			OrgID:     s.OrgID,
			AssetID:   s.AssetID,
			AssetName: s.AssetName,
			Scanner:   s.Scanner,
			TargetURL: s.TargetURL,
		})
		if mErr != nil {
			log.Error().Err(mErr).Str("schedule_id", s.ID).
				Msg("scan_schedule: marshal payload failed, skipping")
			continue
		}

		if eErr := enq.enqueueScan(taskType, payloadBytes); eErr != nil {
			log.Error().Err(eErr).Str("schedule_id", s.ID).Str("scan_id", scanID).
				Msg("scan_schedule: enqueue failed, will retry next tick")
			continue
		}

		if mrErr := store.markScanScheduleRun(ctx, s.ID, nextRun); mrErr != nil {
			log.Error().Err(mrErr).Str("schedule_id", s.ID).
				Msg("scan_schedule: advance next_run failed")
		}
	}
	return nil
}

// --- production wiring ---------------------------------------------------------

// pgScanScheduleStore is the raw-SQL implementation of scanScheduleStore.
type pgScanScheduleStore struct {
	db *pgxpool.Pool
}

func (p pgScanScheduleStore) listDueScanSchedules(ctx context.Context) ([]dueScanSchedule, error) {
	rows, err := p.db.Query(ctx, `
		SELECT s.id::text, s.org_id::text, s.asset_id::text, s.scanner, s.cron_expr,
		       a.name, COALESCE(a.external_url, '')
		FROM vb_scan_schedules s
		JOIN vb_assets a ON a.id = s.asset_id
		WHERE s.is_active
		  AND (s.next_run IS NULL OR s.next_run <= NOW())
		  AND a.is_deleted = FALSE
		ORDER BY s.next_run ASC NULLS FIRST`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []dueScanSchedule
	for rows.Next() {
		var d dueScanSchedule
		if err := rows.Scan(&d.ID, &d.OrgID, &d.AssetID, &d.Scanner, &d.CronExpr,
			&d.AssetName, &d.TargetURL); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (p pgScanScheduleStore) createScanRow(ctx context.Context, orgID, assetID, scanner string) (string, error) {
	var scanID string
	err := p.db.QueryRow(ctx, `
		INSERT INTO vb_scans (org_id, asset_id, scanner, status)
		VALUES ($1::uuid, $2::uuid, $3, 'pending')
		RETURNING id::text`,
		orgID, assetID, scanner).Scan(&scanID)
	if err != nil {
		return "", fmt.Errorf("insert scan row: %w", err)
	}
	return scanID, nil
}

func (p pgScanScheduleStore) markScanScheduleRun(ctx context.Context, id string, nextRun time.Time) error {
	// orgid-lint: global — UPDATE by PK; id is the already-fetched due schedule's own id
	_, err := p.db.Exec(ctx, `
		UPDATE vb_scan_schedules
		SET last_run = NOW(), next_run = $2
		WHERE id = $1::uuid`,
		id, nextRun)
	return err
}

// asynqScanEnqueuer adapts the package-level EnqueueScanTask to the enqueuer seam.
type asynqScanEnqueuer struct {
	client *asynq.Client
}

func (a asynqScanEnqueuer) enqueueScan(taskType string, payload []byte) error {
	return EnqueueScanTask(a.client, taskType, payload)
}

// handleProcessDueScanSchedules is the Asynq cron handler that runs the executor.
func handleProcessDueScanSchedules(pool *pgxpool.Pool, client *asynq.Client) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		store := pgScanScheduleStore{db: pool}
		enq := asynqScanEnqueuer{client: client}
		if err := ProcessDueScanSchedules(ctx, store, enq); err != nil {
			log.Error().Err(err).Msg("scan_schedule: process_due failed")
			return err
		}
		return nil
	}
}
