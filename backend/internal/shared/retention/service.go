package retention

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// RunRetention deletes data that has exceeded the configured retention periods
// for the given organisation.  A retention of 0 for any category means
// "disabled" — that category is skipped.
func RunRetention(ctx context.Context, db *pgxpool.Pool, orgID string) error {
	cfg, err := GetConfig(ctx, db, orgID)
	if err != nil {
		return fmt.Errorf("retention: get config for %s: %w", orgID, err)
	}

	if cfg.AuditLogDays > 0 {
		tag, err := db.Exec(ctx, `
			DELETE FROM audit_log
			WHERE  org_id = $1::uuid
			  AND  created_at < NOW() - ($2::text || ' days')::INTERVAL`,
			orgID, fmt.Sprint(cfg.AuditLogDays),
		)
		if err != nil {
			log.Error().Err(err).Str("org_id", orgID).Msg("retention: delete audit_log")
		} else {
			log.Info().Str("org_id", orgID).Int64("deleted", tag.RowsAffected()).Msg("retention: audit_log pruned")
		}
	}

	if cfg.FindingsResolvedDays > 0 {
		tag, err := db.Exec(ctx, `
			DELETE FROM vb_findings
			WHERE  org_id = $1::uuid
			  AND  status IN ('resolved','false_positive')
			  AND  updated_at < NOW() - ($2::text || ' days')::INTERVAL`,
			orgID, fmt.Sprint(cfg.FindingsResolvedDays),
		)
		if err != nil {
			log.Error().Err(err).Str("org_id", orgID).Msg("retention: delete vb_findings")
		} else {
			log.Info().Str("org_id", orgID).Int64("deleted", tag.RowsAffected()).Msg("retention: vb_findings pruned")
		}
	}

	if cfg.NotificationsDays > 0 {
		tag, err := db.Exec(ctx, `
			DELETE FROM user_notifications
			WHERE  org_id = $1::uuid
			  AND  read = true
			  AND  created_at < NOW() - ($2::text || ' days')::INTERVAL`,
			orgID, fmt.Sprint(cfg.NotificationsDays),
		)
		if err != nil {
			log.Error().Err(err).Str("org_id", orgID).Msg("retention: delete user_notifications")
		} else {
			log.Info().Str("org_id", orgID).Int64("deleted", tag.RowsAffected()).Msg("retention: user_notifications pruned")
		}
	}

	return nil
}

// RunRetentionAllOrgs iterates over all orgs that have a retention_config row
// and calls RunRetention for each.
func RunRetentionAllOrgs(ctx context.Context, db *pgxpool.Pool) error {
	rows, err := db.Query(ctx, `SELECT org_id::text FROM retention_config`)
	if err != nil {
		return fmt.Errorf("retention: list orgs: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var orgID string
		if err := rows.Scan(&orgID); err != nil {
			log.Error().Err(err).Msg("retention: scan org_id")
			continue
		}
		if err := RunRetention(ctx, db, orgID); err != nil {
			log.Error().Err(err).Str("org_id", orgID).Msg("retention: run failed")
		}
	}
	return rows.Err()
}
