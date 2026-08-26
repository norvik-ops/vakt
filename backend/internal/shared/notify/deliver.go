// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package notify

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// DeliverHandler returns the Asynq handler for NotificationJobType. It is the
// consumer half of Service.Notify: for years the enqueue side existed with no
// registered handler, so every "notifications:deliver" task hit "handler not
// found", retried 25 times, then archived — and the notifications row stayed
// 'pending' forever. Register this in the worker (cmd/worker/main.go) so Slack,
// Teams, email, and webhook notifications actually go out.
//
// On success the notifications row advances to status='sent' (sent_at=NOW()).
// On a delivery failure the handler advances the row to status='failed'
// (failed_at=NOW(), retry_count+1) AND returns the error, so Asynq retries the
// task — a delivery failure must be loud, never a silent `return nil`. The
// worker has no central ErrorHandler/IsFailure (R1-19-W09), so returning the
// error is what makes Asynq treat the attempt as a failure.
func DeliverHandler(pool *pgxpool.Pool, senders map[Channel]Sender) asynq.HandlerFunc {
	return func(ctx context.Context, task *asynq.Task) error {
		var env deliveryEnvelope
		if err := json.Unmarshal(task.Payload(), &env); err != nil {
			// A malformed payload cannot be delivered and retrying will not fix
			// it. SkipRetry stops the retry loop; still an error so it is logged
			// as a failure, not swallowed.
			return fmt.Errorf("notify deliver: bad payload: %w: %w", err, asynq.SkipRetry)
		}

		sender, ok := senders[env.Message.Channel]
		if !ok {
			markFailed(ctx, pool, env.NotificationID, env.Message.OrgID)
			return fmt.Errorf("notify deliver: no sender for channel %q", env.Message.Channel)
		}

		if err := sender.Send(ctx, env.Message); err != nil {
			markFailed(ctx, pool, env.NotificationID, env.Message.OrgID)
			log.Error().Err(err).
				Str("org_id", env.Message.OrgID).
				Str("channel", string(env.Message.Channel)).
				Str("notification_id", env.NotificationID).
				Msg("notification delivery failed")
			// Loud failure: return the error so Asynq retries the task.
			return fmt.Errorf("notify deliver over %s: %w", env.Message.Channel, err)
		}

		markSent(ctx, pool, env.NotificationID, env.Message.OrgID)
		return nil
	}
}

// markSent advances a delivered notification to status='sent'. The org_id is in
// the WHERE clause as tenant-isolation defense-in-depth (ADR-0042: app-layer
// org_id scoping is the only isolation mechanism) even though id is a globally
// unique UUID PK. A no-op id (empty) is tolerated so a task enqueued before this
// envelope shape shipped still delivers without a spurious error. DB write errors
// are logged, not returned: the notification WAS delivered, and failing the task
// would re-send it.
func markSent(ctx context.Context, pool *pgxpool.Pool, id, orgID string) {
	if pool == nil || id == "" {
		return
	}
	ct, err := pool.Exec(ctx,
		`UPDATE notifications SET status = 'sent', sent_at = NOW()
		  WHERE id = $1::uuid AND org_id = $2::uuid`,
		id, orgID,
	)
	if err != nil {
		log.Error().Err(err).Str("notification_id", id).Msg("notify: mark sent failed")
		return
	}
	if ct.RowsAffected() == 0 {
		// The notification we just delivered has no matching row: it was deleted
		// (org purge, retention) or the org_id no longer matches. Surface it —
		// a delivered-but-untracked notification is a real inconsistency.
		log.Warn().Str("notification_id", id).Str("org_id", orgID).
			Msg("notify: mark sent affected 0 rows — notification row missing")
	}
}

// markFailed records the failed attempt on the notification row. The org_id is in
// the WHERE clause for the same tenant-isolation defense-in-depth as markSent.
// Errors are logged, not returned — the delivery error (the real failure) is what
// the handler returns to trigger the retry.
func markFailed(ctx context.Context, pool *pgxpool.Pool, id, orgID string) {
	if pool == nil || id == "" {
		return
	}
	ct, err := pool.Exec(ctx,
		`UPDATE notifications
		    SET status = 'failed', failed_at = NOW(), retry_count = retry_count + 1
		  WHERE id = $1::uuid AND org_id = $2::uuid`,
		id, orgID,
	)
	if err != nil {
		log.Error().Err(err).Str("notification_id", id).Msg("notify: mark failed failed")
		return
	}
	if ct.RowsAffected() == 0 {
		// No matching row for a notification we are actively failing/retrying:
		// the row was deleted or its org_id changed under us. Surface it.
		log.Warn().Str("notification_id", id).Str("org_id", orgID).
			Msg("notify: mark failed affected 0 rows — notification row missing")
	}
}
