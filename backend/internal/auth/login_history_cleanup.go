package auth

import (
	"context"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// Sprint 22 / S22-13: Asynq-Periodic-Task für den wöchentlichen
// Login-History-Cleanup. Default-Retention 90 Tage — Customer kann das
// im Worker-Scheduler überschreiben, wenn längere Aufbewahrung Pflicht ist.

const TaskCleanupLoginHistory = "auth:cleanup_login_history"

// NewCleanupLoginHistoryTask erstellt den wöchentlichen Cleanup-Task mit
// 6-Tage-Uniqueness-Lock (verhindert Doppel-Runs bei mehreren Workern).
func NewCleanupLoginHistoryTask() *asynq.Task {
	return asynq.NewTask(TaskCleanupLoginHistory, nil, asynq.Unique(6*24*time.Hour))
}

// CleanupLoginHistory löscht alle Einträge älter als 90 Tage.
func CleanupLoginHistory(ctx context.Context, pool *pgxpool.Pool) error {
	tag, err := pool.Exec(ctx, `
		DELETE FROM login_history
		WHERE ts < NOW() - INTERVAL '90 days'
	`)
	if err != nil {
		return err
	}
	log.Info().Int64("deleted", tag.RowsAffected()).Msg("auth: login history cleanup complete")
	return nil
}
