package retention

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles persistence of retention_config rows.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository constructs a Repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// GetConfig returns the retention config for the given org.
// If no row exists yet the default values from the DB schema are returned.
func GetConfig(ctx context.Context, db *pgxpool.Pool, orgID string) (*RetentionConfig, error) {
	row := db.QueryRow(ctx, `
		SELECT org_id::text,
		       audit_log_days, findings_resolved_days,
		       notifications_days, scan_history_days,
		       digest_enabled, digest_hour, updated_at
		FROM   retention_config
		WHERE  org_id = $1::uuid`,
		orgID,
	)

	var cfg RetentionConfig
	err := row.Scan(
		&cfg.OrgID,
		&cfg.AuditLogDays,
		&cfg.FindingsResolvedDays,
		&cfg.NotificationsDays,
		&cfg.ScanHistoryDays,
		&cfg.DigestEnabled,
		&cfg.DigestHour,
		&cfg.UpdatedAt,
	)
	if err != nil {
		// Return defaults when no config row exists yet.
		return &RetentionConfig{
			OrgID:                orgID,
			AuditLogDays:         365,
			FindingsResolvedDays: 180,
			NotificationsDays:    90,
			ScanHistoryDays:      365,
			DigestEnabled:        false,
			DigestHour:           8,
		}, nil
	}
	return &cfg, nil
}

// UpsertConfig creates or replaces the retention config for an org.
func UpsertConfig(ctx context.Context, db *pgxpool.Pool, orgID string, cfg RetentionConfig) (*RetentionConfig, error) {
	row := db.QueryRow(ctx, `
		INSERT INTO retention_config
		    (org_id, audit_log_days, findings_resolved_days,
		     notifications_days, scan_history_days,
		     digest_enabled, digest_hour, updated_at)
		VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, now())
		ON CONFLICT (org_id) DO UPDATE SET
		    audit_log_days         = EXCLUDED.audit_log_days,
		    findings_resolved_days = EXCLUDED.findings_resolved_days,
		    notifications_days     = EXCLUDED.notifications_days,
		    scan_history_days      = EXCLUDED.scan_history_days,
		    digest_enabled         = EXCLUDED.digest_enabled,
		    digest_hour            = EXCLUDED.digest_hour,
		    updated_at             = now()
		RETURNING org_id::text,
		          audit_log_days, findings_resolved_days,
		          notifications_days, scan_history_days,
		          digest_enabled, digest_hour, updated_at`,
		orgID,
		cfg.AuditLogDays,
		cfg.FindingsResolvedDays,
		cfg.NotificationsDays,
		cfg.ScanHistoryDays,
		cfg.DigestEnabled,
		cfg.DigestHour,
	)

	var out RetentionConfig
	if err := row.Scan(
		&out.OrgID,
		&out.AuditLogDays,
		&out.FindingsResolvedDays,
		&out.NotificationsDays,
		&out.ScanHistoryDays,
		&out.DigestEnabled,
		&out.DigestHour,
		&out.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("upsert retention config: %w", err)
	}
	return &out, nil
}
