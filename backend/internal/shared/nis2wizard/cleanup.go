package nis2wizard

import (
	"context"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// Sprint 22 / S22-12: täglicher Cleanup für abgelaufene anonyme NIS2-
// Wizard-Runs. Default-TTL pro Run ist 7 Tage (in StartRun gesetzt) —
// dieser Job räumt nach Ablauf auf.

const TaskCleanupAnonymousRuns = "nis2:cleanup_anonymous_runs"

// NewCleanupAnonymousRunsTask erstellt den täglichen Cleanup-Task mit
// 23h-Uniqueness-Lock.
func NewCleanupAnonymousRunsTask() *asynq.Task {
	return asynq.NewTask(TaskCleanupAnonymousRuns, nil, asynq.Unique(23*time.Hour))
}

// CleanupAnonymousRuns löscht alle Wizard-Runs, deren expires_at in der
// Vergangenheit liegt.
func CleanupAnonymousRuns(ctx context.Context, pool *pgxpool.Pool) error {
	tag, err := pool.Exec(ctx, `
		DELETE FROM nis2_anonymous_runs
		WHERE expires_at < NOW()
	`)
	if err != nil {
		return err
	}
	log.Info().Int64("deleted", tag.RowsAffected()).Msg("nis2: anonymous runs cleanup complete")
	return nil
}
