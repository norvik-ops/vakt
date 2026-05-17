// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth

import (
	"context"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// TaskCleanupPasswordResetTokens is the Asynq task type for the daily
// password-reset token cleanup job.
const TaskCleanupPasswordResetTokens = "auth:cleanup_password_reset_tokens"

// NewCleanupPasswordResetTokensTask creates the daily cleanup task with a 23h
// uniqueness lock to prevent duplicate runs when multiple worker instances are
// running.
func NewCleanupPasswordResetTokensTask() *asynq.Task {
	return asynq.NewTask(TaskCleanupPasswordResetTokens, nil, asynq.Unique(23*time.Hour))
}

// CleanupPasswordResetTokens deletes:
//   - expired tokens (expires_at < NOW())
//   - used tokens older than 7 days (used_at < NOW() - INTERVAL '7 days')
//
// This keeps the password_reset_tokens table lean and avoids unbounded growth.
func CleanupPasswordResetTokens(ctx context.Context, pool *pgxpool.Pool) error {
	tag, err := pool.Exec(ctx, `
		DELETE FROM password_reset_tokens
		WHERE expires_at < NOW()
		   OR (used_at IS NOT NULL AND used_at < NOW() - INTERVAL '7 days')
	`)
	if err != nil {
		return err
	}
	log.Info().Int64("deleted", tag.RowsAffected()).Msg("auth: password reset token cleanup complete")
	return nil
}
