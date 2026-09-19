package retention

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// execer is the subset of *pgxpool.Pool that the prune step needs. It exists so
// unit tests can inject a fake that forces a delete to fail and assert the error
// is propagated rather than swallowed. *pgxpool.Pool satisfies it.
type execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// sqlAuditLogSoftDelete is the retention query for audit_log. Exported as a
// package-level constant so unit tests can assert it uses UPDATE (soft-delete)
// rather than DELETE, protecting the SHA-256 hash chain (ADR-0040/ADR-0050).
const sqlAuditLogSoftDelete = `
			UPDATE audit_log
			SET    deleted_at = NOW()
			WHERE  org_id     = $1::uuid
			  AND  created_at < NOW() - ($2::text || ' days')::INTERVAL
			  AND  deleted_at IS NULL`

// RunRetention deletes data that has exceeded the configured retention periods
// for the given organisation.  A retention of 0 for any category means
// "disabled" — that category is skipped.
func RunRetention(ctx context.Context, db *pgxpool.Pool, orgID string) error {
	cfg, err := GetConfig(ctx, db, orgID)
	if err != nil {
		return fmt.Errorf("retention: get config for %s: %w", orgID, err)
	}
	return pruneRetention(ctx, db, cfg, orgID)
}

// pruneRetention runs the per-category deletes for one org. It attempts every
// enabled category even when an earlier one fails, then returns all delete
// errors joined together — so the worker sees a failure instead of "success"
// when a prune did not happen. A category-level failure is still logged.
func pruneRetention(ctx context.Context, db execer, cfg *RetentionConfig, orgID string) error {
	var errs []error

	if cfg.AuditLogDays > 0 {
		// Soft-delete instead of hard-delete to preserve the SHA-256 hash chain
		// (migration 149 / ADR-0040). Hard-deleting a row breaks the prev_hash
		// link for every subsequent row in the same org, causing cmd/audit-verify
		// to report all later rows as tampered. The chain verifier and the writer's
		// SELECT-for-UPDATE tail query intentionally do NOT filter on deleted_at so
		// the chain remains continuous. UI-facing read paths filter deleted_at IS NULL.
		tag, err := db.Exec(ctx, sqlAuditLogSoftDelete,
			orgID, fmt.Sprint(cfg.AuditLogDays),
		)
		if err != nil {
			log.Error().Err(err).Str("org_id", orgID).Msg("retention: soft-delete audit_log")
			errs = append(errs, fmt.Errorf("audit_log: %w", err))
		} else {
			log.Info().Str("org_id", orgID).Int64("soft_deleted", tag.RowsAffected()).Msg("retention: audit_log pruned")
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
			errs = append(errs, fmt.Errorf("vb_findings: %w", err))
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
			errs = append(errs, fmt.Errorf("user_notifications: %w", err))
		} else {
			log.Info().Str("org_id", orgID).Int64("deleted", tag.RowsAffected()).Msg("retention: user_notifications pruned")
		}
	}

	return errors.Join(errs...)
}

// RunRetentionAllOrgs iterates over all orgs that have a retention_config row
// and calls RunRetention for each.
func RunRetentionAllOrgs(ctx context.Context, db *pgxpool.Pool) error {
	rows, err := db.Query(ctx, `SELECT org_id::text FROM retention_config`)
	if err != nil {
		return fmt.Errorf("retention: list orgs: %w", err)
	}
	defer rows.Close()

	var errs []error
	for rows.Next() {
		var orgID string
		if err := rows.Scan(&orgID); err != nil {
			log.Error().Err(err).Msg("retention: scan org_id")
			errs = append(errs, fmt.Errorf("scan org_id: %w", err))
			continue
		}
		// Keep going across orgs — one bad org must not stop the rest — but
		// collect failures so the worker reports a failure instead of success.
		if err := RunRetention(ctx, db, orgID); err != nil {
			log.Error().Err(err).Str("org_id", orgID).Msg("retention: run failed")
			errs = append(errs, fmt.Errorf("org %s: %w", orgID, err))
		}
	}
	if err := rows.Err(); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}
