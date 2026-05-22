// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles data access for cloud integrations.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new cloud integration repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// GetConfig returns the raw config JSONB for a provider (org-scoped).
// Returns nil, nil if no row exists yet.
func (r *Repository) GetConfig(ctx context.Context, orgID, provider string) ([]byte, error) {
	var raw []byte
	err := r.db.QueryRow(ctx, `
		SELECT config FROM cloud_integrations
		WHERE org_id = $1::uuid AND provider = $2`,
		orgID, provider,
	).Scan(&raw)
	if err != nil {
		// pgx returns pgx.ErrNoRows — return nil, nil for "not configured"
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("get cloud config: %w", err)
	}
	return raw, nil
}

// UpsertConfig saves (insert or update) the config JSONB for a provider.
func (r *Repository) UpsertConfig(ctx context.Context, orgID, provider string, config map[string]any) error {
	raw, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshal cloud config: %w", err)
	}
	_, err = r.db.Exec(ctx, `
		INSERT INTO cloud_integrations (org_id, provider, config)
		VALUES ($1::uuid, $2, $3::jsonb)
		ON CONFLICT (org_id, provider)
		DO UPDATE SET config = EXCLUDED.config`,
		orgID, provider, raw,
	)
	if err != nil {
		return fmt.Errorf("upsert cloud config: %w", err)
	}
	return nil
}

// GetIntegration returns the full integration row (no config payload).
func (r *Repository) GetIntegration(ctx context.Context, orgID, provider string) (*CloudIntegration, error) {
	var ci CloudIntegration
	err := r.db.QueryRow(ctx, `
		SELECT id::text, org_id::text, provider, enabled, last_sync_at, last_sync_status, last_sync_error, created_at
		FROM cloud_integrations
		WHERE org_id = $1::uuid AND provider = $2`,
		orgID, provider,
	).Scan(&ci.ID, &ci.OrgID, &ci.Provider, &ci.Enabled, &ci.LastSyncAt, &ci.LastSyncStatus, &ci.LastSyncError, &ci.CreatedAt)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("get cloud integration: %w", err)
	}
	return &ci, nil
}

// ListEnabled returns all enabled cloud integrations (for scheduled sync).
func (r *Repository) ListEnabled(ctx context.Context) ([]CloudIntegration, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text, provider, enabled, last_sync_at, last_sync_status, last_sync_error, created_at
		FROM cloud_integrations
		WHERE enabled = true`)
	if err != nil {
		return nil, fmt.Errorf("list enabled cloud integrations: %w", err)
	}
	defer rows.Close()

	var out []CloudIntegration
	for rows.Next() {
		var ci CloudIntegration
		if err := rows.Scan(&ci.ID, &ci.OrgID, &ci.Provider, &ci.Enabled, &ci.LastSyncAt, &ci.LastSyncStatus, &ci.LastSyncError, &ci.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan cloud integration: %w", err)
		}
		out = append(out, ci)
	}
	return out, rows.Err()
}

// UpdateSyncResult records the outcome of a sync run.
func (r *Repository) UpdateSyncResult(ctx context.Context, orgID, provider, status string, syncErr error) error {
	var errStr *string
	if syncErr != nil {
		s := syncErr.Error()
		errStr = &s
	}
	now := time.Now().UTC()
	_, err := r.db.Exec(ctx, `
		UPDATE cloud_integrations
		SET last_sync_at = $1, last_sync_status = $2, last_sync_error = $3
		WHERE org_id = $4::uuid AND provider = $5`,
		now, status, errStr, orgID, provider,
	)
	return err
}

// CountEvidence returns the number of evidence items with source = "aws-collector" or "azure-collector"
// for a given org.
func (r *Repository) CountEvidence(ctx context.Context, orgID, source string) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM ck_evidence
		WHERE org_id = $1::uuid AND source = $2`,
		orgID, source,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count cloud evidence: %w", err)
	}
	return count, nil
}

// RecentEvidence returns the most recent evidence items (up to limit) for a given org and source.
func (r *Repository) RecentEvidence(ctx context.Context, orgID, source string, limit int) ([]EvidenceItem, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, title, COALESCE(description, ''), source, created_at
		FROM ck_evidence
		WHERE org_id = $1::uuid AND source = $2
		ORDER BY created_at DESC
		LIMIT $3`,
		orgID, source, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("recent cloud evidence: %w", err)
	}
	defer rows.Close()

	var out []EvidenceItem
	for rows.Next() {
		var e EvidenceItem
		if err := rows.Scan(&e.ID, &e.Title, &e.Description, &e.Source, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan evidence: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// EvidenceItem is a lightweight evidence summary for status responses.
type EvidenceItem struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Source      string    `json:"source"`
	CreatedAt   time.Time `json:"created_at"`
}
