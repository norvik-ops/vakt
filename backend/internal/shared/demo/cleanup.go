package demo

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

const TaskCleanupEphemeralOrgs = "demo:cleanup_ephemeral_orgs"

// NewCleanupTask returns an asynq task for cleaning up expired ephemeral demo orgs.
func NewCleanupTask() *asynq.Task {
	return asynq.NewTask(TaskCleanupEphemeralOrgs, nil)
}

// HandleCleanup deletes ephemeral demo orgs older than 4 hours.
// Ephemeral orgs have slugs matching 'demo-%'. The shared 'demo' org is not affected.
func HandleCleanup(ctx context.Context, db *pgxpool.Pool) error {
	tag, err := db.Exec(ctx, `
		DELETE FROM organizations
		WHERE slug LIKE 'demo-%'
		  AND created_at < NOW() - INTERVAL '4 hours'
	`)
	if err != nil {
		return fmt.Errorf("demo cleanup: %w", err)
	}
	if tag.RowsAffected() > 0 {
		log.Info().Int64("deleted", tag.RowsAffected()).Msg("demo cleanup: ephemeral orgs deleted")
	}
	return nil
}
