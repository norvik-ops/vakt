// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package secpulse

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// Repository handles VulnBoard data access via pgx.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new VulnBoard repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// CreateAsset inserts a new asset row and returns the created record.
func (r *Repository) CreateAsset(ctx context.Context, orgID string, input CreateAssetInput) (*Asset, error) {
	tags := input.Tags
	if tags == nil {
		tags = []string{}
	}

	var a Asset
	err := r.db.QueryRow(ctx, `
		INSERT INTO vb_assets (org_id, name, type, criticality, tags, owner_id, external_url)
		VALUES ($1::uuid, $2, $3, $4, $5, $6::uuid, $7)
		RETURNING id::text, org_id::text, name, type, criticality, tags,
		          owner_id::text, external_url, created_at, updated_at`,
		orgID, input.Name, input.Type, input.Criticality, tags, input.OwnerID, input.ExternalURL,
	).Scan(
		&a.ID, &a.OrgID, &a.Name, &a.Type, &a.Criticality, &a.Tags,
		&a.OwnerID, &a.ExternalURL, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert asset: %w", err)
	}
	return &a, nil
}

// ListAssets returns a paginated list of non-deleted assets for an org.
// An optional tag filter restricts results to assets containing that tag.
func (r *Repository) ListAssets(ctx context.Context, orgID string, page, limit int, tag string) ([]Asset, int, error) {
	args := []interface{}{orgID}
	where := "org_id = $1::uuid AND is_deleted = FALSE"
	argN := 2

	if tag != "" {
		where += fmt.Sprintf(" AND $%d = ANY(tags)", argN)
		args = append(args, tag)
		argN++
	}

	var total int
	if err := r.db.QueryRow(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM vb_assets WHERE %s", where), args...,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count assets: %w", err)
	}

	offset := (page - 1) * limit
	dataArgs := append(args, limit, offset)
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT id::text, org_id::text, name, type, criticality, tags,
		       owner_id::text, external_url, created_at, updated_at
		FROM vb_assets
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`, where, argN, argN+1), dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query assets: %w", err)
	}
	defer rows.Close()

	var assets []Asset
	for rows.Next() {
		var a Asset
		if err := rows.Scan(
			&a.ID, &a.OrgID, &a.Name, &a.Type, &a.Criticality, &a.Tags,
			&a.OwnerID, &a.ExternalURL, &a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan asset row: %w", err)
		}
		assets = append(assets, a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate asset rows: %w", err)
	}
	return assets, total, nil
}

// GetAsset fetches a single non-deleted asset by ID within the org.
func (r *Repository) GetAsset(ctx context.Context, orgID, assetID string) (*Asset, error) {
	var a Asset
	err := r.db.QueryRow(ctx, `
		SELECT id::text, org_id::text, name, type, criticality, tags,
		       owner_id::text, external_url, created_at, updated_at
		FROM vb_assets
		WHERE id = $1::uuid AND org_id = $2::uuid AND is_deleted = FALSE`,
		assetID, orgID,
	).Scan(
		&a.ID, &a.OrgID, &a.Name, &a.Type, &a.Criticality, &a.Tags,
		&a.OwnerID, &a.ExternalURL, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get asset: %w", err)
	}
	return &a, nil
}

// GetAssetByName fetches the first non-deleted asset matching name (case-insensitive) within the org.
// Returns nil, nil when no asset matches.
func (r *Repository) GetAssetByName(ctx context.Context, orgID, name string) (*Asset, error) {
	var a Asset
	err := r.db.QueryRow(ctx, `
		SELECT id::text, org_id::text, name, type, criticality, tags,
		       owner_id::text, external_url, created_at, updated_at
		FROM vb_assets
		WHERE org_id = $1::uuid AND LOWER(name) = LOWER($2) AND is_deleted = FALSE
		LIMIT 1`,
		orgID, name,
	).Scan(
		&a.ID, &a.OrgID, &a.Name, &a.Type, &a.Criticality, &a.Tags,
		&a.OwnerID, &a.ExternalURL, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get asset by name: %w", err)
	}
	return &a, nil
}

// ResolveAssetRef resolves an asset reference (UUID or name) to an asset ID.
// It first tries to treat ref as a UUID and look up by ID, then falls back to
// a case-insensitive name lookup.  Returns an error when no asset is found.
func (r *Repository) ResolveAssetRef(ctx context.Context, orgID, ref string) (string, error) {
	if ref == "" {
		return "", fmt.Errorf("asset reference is empty")
	}

	// Try ID lookup first (valid UUIDs are 36 chars).
	if len(ref) == 36 {
		if a, err := r.GetAsset(ctx, orgID, ref); err == nil {
			return a.ID, nil
		}
	}

	// Fall back to name lookup.
	a, err := r.GetAssetByName(ctx, orgID, ref)
	if err != nil {
		return "", fmt.Errorf("resolve asset %q: %w", ref, err)
	}
	if a == nil {
		return "", fmt.Errorf("asset %q not found", ref)
	}
	return a.ID, nil
}

// UpdateAsset applies a partial update to an asset.  Only non-nil fields are changed.
func (r *Repository) UpdateAsset(ctx context.Context, orgID, assetID string, input UpdateAssetInput) (*Asset, error) {
	// Build SET clause dynamically.
	setClauses := []string{"updated_at = NOW()"}
	args := []interface{}{}
	argN := 1

	if input.Name != nil {
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", argN))
		args = append(args, *input.Name)
		argN++
	}
	if input.Type != nil {
		setClauses = append(setClauses, fmt.Sprintf("type = $%d", argN))
		args = append(args, *input.Type)
		argN++
	}
	if input.Criticality != nil {
		setClauses = append(setClauses, fmt.Sprintf("criticality = $%d", argN))
		args = append(args, *input.Criticality)
		argN++
	}
	if input.Tags != nil {
		setClauses = append(setClauses, fmt.Sprintf("tags = $%d", argN))
		args = append(args, input.Tags)
		argN++
	}
	if input.OwnerID != nil {
		setClauses = append(setClauses, fmt.Sprintf("owner_id = $%d::uuid", argN))
		args = append(args, *input.OwnerID)
		argN++
	}
	if input.ExternalURL != nil {
		setClauses = append(setClauses, fmt.Sprintf("external_url = $%d", argN))
		args = append(args, *input.ExternalURL)
		argN++
	}

	args = append(args, assetID, orgID)
	query := fmt.Sprintf(`
		UPDATE vb_assets
		SET %s
		WHERE id = $%d::uuid AND org_id = $%d::uuid AND is_deleted = FALSE
		RETURNING id::text, org_id::text, name, type, criticality, tags,
		          owner_id::text, external_url, created_at, updated_at`,
		strings.Join(setClauses, ", "), argN, argN+1)

	var a Asset
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&a.ID, &a.OrgID, &a.Name, &a.Type, &a.Criticality, &a.Tags,
		&a.OwnerID, &a.ExternalURL, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("update asset: %w", err)
	}
	return &a, nil
}

// SoftDeleteAsset marks an asset as deleted (is_deleted = TRUE).
func (r *Repository) SoftDeleteAsset(ctx context.Context, orgID, assetID string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE vb_assets SET is_deleted = TRUE, updated_at = NOW()
		WHERE id = $1::uuid AND org_id = $2::uuid AND is_deleted = FALSE`,
		assetID, orgID)
	if err != nil {
		return fmt.Errorf("soft delete asset: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("asset not found")
	}
	return nil
}

// GetSLAConfig fetches the SLA configuration for an org; returns defaults if absent.
func (r *Repository) GetSLAConfig(ctx context.Context, orgID string) (*SLAConfig, error) {
	var cfg SLAConfig
	err := r.db.QueryRow(ctx, `
		SELECT org_id::text, critical_days, high_days, medium_days, low_days
		FROM vb_sla_config
		WHERE org_id = $1::uuid`, orgID).Scan(
		&cfg.OrgID, &cfg.CriticalDays, &cfg.HighDays, &cfg.MediumDays, &cfg.LowDays,
	)
	if err != nil {
		// No row → return defaults
		return &SLAConfig{
			OrgID:        orgID,
			CriticalDays: 7,
			HighDays:     30,
			MediumDays:   90,
			LowDays:      180,
		}, nil
	}
	return &cfg, nil
}

// UpsertSLAConfig inserts or updates the SLA config for an org.
func (r *Repository) UpsertSLAConfig(ctx context.Context, orgID string, input SLAConfig) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO vb_sla_config (org_id, critical_days, high_days, medium_days, low_days, updated_at)
		VALUES ($1::uuid, $2, $3, $4, $5, NOW())
		ON CONFLICT (org_id) DO UPDATE
		  SET critical_days = EXCLUDED.critical_days,
		      high_days     = EXCLUDED.high_days,
		      medium_days   = EXCLUDED.medium_days,
		      low_days      = EXCLUDED.low_days,
		      updated_at    = NOW()`,
		orgID, input.CriticalDays, input.HighDays, input.MediumDays, input.LowDays,
	)
	if err != nil {
		return fmt.Errorf("upsert sla config: %w", err)
	}
	return nil
}

// slaDashboardRow is a raw DB row from the SLA dashboard query (no SLA logic applied).
type slaDashboardRow struct {
	AssetID      string
	AssetName    string
	FindingID    string
	FindingTitle string
	Severity     string
	Status       string
	DaysOpen     int
}

// GetSLADashboard returns up to 100 open findings with their age in days for the given org.
func (r *Repository) GetSLADashboard(ctx context.Context, orgID string) ([]slaDashboardRow, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
		    a.id::text,
		    a.name,
		    f.id::text,
		    f.title,
		    f.severity,
		    f.status,
		    EXTRACT(DAY FROM now() - f.created_at)::int AS days_open
		FROM vb_findings f
		JOIN vb_assets a ON a.id = f.asset_id
		WHERE f.org_id = $1::uuid
		  AND f.status NOT IN ('resolved', 'false_positive')
		ORDER BY f.severity DESC, days_open DESC
		LIMIT 100`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("sla dashboard query: %w", err)
	}
	defer rows.Close()

	var result []slaDashboardRow
	for rows.Next() {
		var row slaDashboardRow
		if err := rows.Scan(&row.AssetID, &row.AssetName, &row.FindingID, &row.FindingTitle, &row.Severity, &row.Status, &row.DaysOpen); err != nil {
			return nil, fmt.Errorf("scan sla dashboard row: %w", err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// BulkCreateAssets inserts multiple assets in a single transaction and returns
// the number inserted, the number of errors, and a slice of error messages.
func (r *Repository) BulkCreateAssets(ctx context.Context, orgID string, rows []CSVAssetRow) (int, int, []string) {
	var inserted, errored int
	var errs []string

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, len(rows), []string{fmt.Sprintf("begin transaction: %s", err)}
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	for i, row := range rows {
		tags := row.Tags
		if tags == nil {
			tags = []string{}
		}

		var id string
		scanErr := tx.QueryRow(ctx, `
			INSERT INTO vb_assets (org_id, name, type, criticality, tags, external_url, updated_at)
			VALUES ($1::uuid, $2, $3, $4, $5, $6, $7)
			RETURNING id::text`,
			orgID, row.Name, row.Type, row.Criticality, tags, row.ExternalURL, time.Now(),
		).Scan(&id)
		if scanErr != nil {
			errored++
			errs = append(errs, fmt.Sprintf("row %d (%q): %s", i+1, row.Name, scanErr))
		} else {
			inserted++
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return 0, len(rows), []string{fmt.Sprintf("commit: %s", err)}
	}
	return inserted, errored, errs
}

// ---------------------------------------------------------------------------
// Scans
// ---------------------------------------------------------------------------

// CreateScan inserts a new scan record and returns it.
func (r *Repository) CreateScan(ctx context.Context, orgID string, input CreateScanInput, assetID string) (*Scan, error) {
	var s Scan
	err := r.db.QueryRow(ctx, `
		INSERT INTO vb_scans (org_id, asset_id, scanner, status, target_url, target_ip)
		VALUES ($1::uuid, $2::uuid, $3, 'pending', $4, $5)
		RETURNING id::text, org_id::text, asset_id::text, scanner, status,
		          COALESCE(target_url,''), COALESCE(target_ip,''),
		          COALESCE(error_message,''), finding_count,
		          duration_ms, started_at, completed_at, created_at`,
		orgID, assetID, input.Scanner, input.TargetURL, input.TargetIP,
	).Scan(
		&s.ID, &s.OrgID, &s.AssetID, &s.Scanner, &s.Status,
		&s.TargetURL, &s.TargetIP, &s.ErrorMessage, &s.FindingCount,
		&s.DurationMs, &s.StartedAt, &s.CompletedAt, &s.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert scan: %w", err)
	}
	return &s, nil
}

// GetScan fetches a scan by ID within the org.
func (r *Repository) GetScan(ctx context.Context, orgID, scanID string) (*Scan, error) {
	var s Scan
	err := r.db.QueryRow(ctx, `
		SELECT id::text, org_id::text, asset_id::text, scanner, status,
		       COALESCE(target_url,''), COALESCE(target_ip,''),
		       COALESCE(error_message,''), finding_count,
		       duration_ms, started_at, completed_at, created_at
		FROM vb_scans
		WHERE id = $1::uuid AND org_id = $2::uuid`,
		scanID, orgID,
	).Scan(
		&s.ID, &s.OrgID, &s.AssetID, &s.Scanner, &s.Status,
		&s.TargetURL, &s.TargetIP, &s.ErrorMessage, &s.FindingCount,
		&s.DurationMs, &s.StartedAt, &s.CompletedAt, &s.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get scan: %w", err)
	}
	return &s, nil
}

// UpdateScanStatus updates scan status and optional fields.
func (r *Repository) UpdateScanStatus(ctx context.Context, scanID, status string, opts ...ScanUpdateOpt) error {
	o := &scanUpdateOptions{}
	for _, opt := range opts {
		opt(o)
	}

	setClauses := []string{"status = $1"}
	args := []interface{}{status}
	argN := 2

	if o.errorMessage != nil {
		setClauses = append(setClauses, fmt.Sprintf("error_message = $%d", argN))
		args = append(args, *o.errorMessage)
		argN++
	}
	if o.findingCount != nil {
		setClauses = append(setClauses, fmt.Sprintf("finding_count = $%d", argN))
		args = append(args, *o.findingCount)
		argN++
	}
	if o.durationMs != nil {
		setClauses = append(setClauses, fmt.Sprintf("duration_ms = $%d", argN))
		args = append(args, *o.durationMs)
		argN++
	}
	if o.startedAt != nil {
		setClauses = append(setClauses, fmt.Sprintf("started_at = $%d", argN))
		args = append(args, *o.startedAt)
		argN++
	}
	if o.completedAt != nil {
		setClauses = append(setClauses, fmt.Sprintf("completed_at = $%d", argN))
		args = append(args, *o.completedAt)
		argN++
	}

	args = append(args, scanID)
	query := fmt.Sprintf("UPDATE vb_scans SET %s WHERE id = $%d::uuid",
		strings.Join(setClauses, ", "), argN)

	_, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update scan status: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Findings
// ---------------------------------------------------------------------------

// UpsertFinding inserts a finding or deduplicates if the same CVE / template
// was already found on the asset. Returns the upserted finding.
func (r *Repository) UpsertFinding(ctx context.Context, orgID string, f Finding) (*Finding, error) {
	sources := f.Sources
	if sources == nil {
		sources = []string{}
	}

	// Deduplicate on (org_id, asset_id, cve_id) for CVE findings or
	// (org_id, asset_id, scanner, template_id) for non-CVE findings.
	var existing Finding
	var scanErr error
	if f.CVEID != nil && *f.CVEID != "" {
		scanErr = r.db.QueryRow(ctx, `
			SELECT id::text, status, reopen_count, occurrence_count, sources
			FROM vb_findings
			WHERE org_id = $1::uuid AND asset_id = $2::uuid AND cve_id = $3
			LIMIT 1`,
			orgID, f.AssetID, *f.CVEID,
		).Scan(&existing.ID, &existing.Status, &existing.ReopenCount,
			&existing.OccurrenceCount, &existing.Sources)
	} else if f.TemplateID != "" {
		scanErr = r.db.QueryRow(ctx, `
			SELECT id::text, status, reopen_count, occurrence_count, sources
			FROM vb_findings
			WHERE org_id = $1::uuid AND asset_id = $2::uuid
			  AND scanner = $3 AND template_id = $4
			LIMIT 1`,
			orgID, f.AssetID, f.Scanner, f.TemplateID,
		).Scan(&existing.ID, &existing.Status, &existing.ReopenCount,
			&existing.OccurrenceCount, &existing.Sources)
	} else {
		scanErr = fmt.Errorf("no match key")
	}

	if scanErr == nil {
		// Existing record: update occurrence info.
		newStatus := existing.Status
		newReopenCount := existing.ReopenCount
		if existing.Status == "resolved" || existing.Status == "false_positive" {
			newStatus = "open"
			newReopenCount++
		}

		// Merge sources.
		srcSet := make(map[string]struct{})
		for _, s := range existing.Sources {
			srcSet[s] = struct{}{}
		}
		for _, s := range sources {
			srcSet[s] = struct{}{}
		}
		mergedSources := make([]string, 0, len(srcSet))
		for s := range srcSet {
			mergedSources = append(mergedSources, s)
		}

		var updated Finding
		err := r.db.QueryRow(ctx, `
			UPDATE vb_findings
			SET last_seen_at     = NOW(),
			    occurrence_count = occurrence_count + 1,
			    status           = $1,
			    reopen_count     = $2,
			    sources          = $3,
			    updated_at       = NOW()
			WHERE id = $4::uuid
			RETURNING id::text, org_id::text, asset_id::text,
			          scan_id::text, cve_id,
			          title, COALESCE(description,''), severity,
			          cvss_score, epss_score, epss_percentile, risk_score,
			          status, scanner, COALESCE(raw_id,''), sources,
			          COALESCE(template_id,''), assigned_to::text, COALESCE(justification,''),
			          reopen_count, occurrence_count,
			          last_seen_at, sla_due_at, created_at, updated_at`,
			newStatus, newReopenCount, mergedSources, existing.ID,
		).Scan(
			&updated.ID, &updated.OrgID, &updated.AssetID,
			&updated.ScanID, &updated.CVEID,
			&updated.Title, &updated.Description, &updated.Severity,
			&updated.CVSSScore, &updated.EPSSScore, &updated.EPSSPercentile, &updated.RiskScore,
			&updated.Status, &updated.Scanner, &updated.RawID, &updated.Sources,
			&updated.TemplateID, &updated.AssignedTo, &updated.Justification,
			&updated.ReopenCount, &updated.OccurrenceCount,
			&updated.LastSeenAt, &updated.SLADueAt, &updated.CreatedAt, &updated.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("update existing finding: %w", err)
		}
		return &updated, nil
	}

	// New finding: insert.
	var inserted Finding
	err := r.db.QueryRow(ctx, `
		INSERT INTO vb_findings
		  (org_id, asset_id, scan_id, cve_id, title, description, severity,
		   cvss_score, epss_score, epss_percentile, risk_score,
		   status, scanner, raw_id, sources, template_id,
		   assigned_to, justification, reopen_count, occurrence_count, last_seen_at)
		VALUES
		  ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7,
		   $8, $9, $10, $11,
		   $12, $13, $14, $15, $16,
		   $17::uuid, $18, 0, 1, NOW())
		RETURNING id::text, org_id::text, asset_id::text,
		          scan_id::text, cve_id,
		          title, COALESCE(description,''), severity,
		          cvss_score, epss_score, epss_percentile, risk_score,
		          status, scanner, COALESCE(raw_id,''), sources,
		          COALESCE(template_id,''), assigned_to::text, COALESCE(justification,''),
		          reopen_count, occurrence_count,
		          last_seen_at, sla_due_at, created_at, updated_at`,
		orgID, f.AssetID, f.ScanID, f.CVEID, f.Title, f.Description, f.Severity,
		f.CVSSScore, f.EPSSScore, f.EPSSPercentile, f.RiskScore,
		f.Status, f.Scanner, f.RawID, sources, f.TemplateID,
		f.AssignedTo, f.Justification,
	).Scan(
		&inserted.ID, &inserted.OrgID, &inserted.AssetID,
		&inserted.ScanID, &inserted.CVEID,
		&inserted.Title, &inserted.Description, &inserted.Severity,
		&inserted.CVSSScore, &inserted.EPSSScore, &inserted.EPSSPercentile, &inserted.RiskScore,
		&inserted.Status, &inserted.Scanner, &inserted.RawID, &inserted.Sources,
		&inserted.TemplateID, &inserted.AssignedTo, &inserted.Justification,
		&inserted.ReopenCount, &inserted.OccurrenceCount,
		&inserted.LastSeenAt, &inserted.SLADueAt, &inserted.CreatedAt, &inserted.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert finding: %w", err)
	}
	return &inserted, nil
}

// ListFindings returns findings for an org matching the given filter.
func (r *Repository) ListFindings(ctx context.Context, orgID string, filter FindingFilter) ([]Finding, error) {
	args := []interface{}{orgID}
	where := "org_id = $1::uuid"
	argN := 2

	if filter.Severity != "" {
		where += fmt.Sprintf(" AND severity = $%d", argN)
		args = append(args, filter.Severity)
		argN++
	}
	if filter.Status != "" {
		where += fmt.Sprintf(" AND status = $%d", argN)
		args = append(args, filter.Status)
		argN++
	}
	if filter.AssetID != "" {
		where += fmt.Sprintf(" AND asset_id = $%d::uuid", argN)
		args = append(args, filter.AssetID)
		argN++
	}

	orderBy := "risk_score DESC NULLS LAST"
	if filter.SortBy == "created_at" {
		orderBy = "created_at DESC"
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	limit := filter.Limit
	if limit < 1 || limit > 500 {
		limit = 25
	}
	offset := (page - 1) * limit

	args = append(args, limit, offset)
	query := fmt.Sprintf(`
		SELECT id::text, org_id::text, asset_id::text,
		       scan_id::text, cve_id,
		       title, COALESCE(description,''), severity,
		       cvss_score, epss_score, epss_percentile, risk_score,
		       status, scanner, COALESCE(raw_id,''), sources,
		       COALESCE(template_id,''), assigned_to::text, COALESCE(justification,''),
		       reopen_count, occurrence_count,
		       last_seen_at, sla_due_at, created_at, updated_at
		FROM vb_findings
		WHERE %s
		ORDER BY %s
		LIMIT $%d OFFSET $%d`, where, orderBy, argN, argN+1)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list findings: %w", err)
	}
	defer rows.Close()

	var findings []Finding
	for rows.Next() {
		var f Finding
		if err := rows.Scan(
			&f.ID, &f.OrgID, &f.AssetID,
			&f.ScanID, &f.CVEID,
			&f.Title, &f.Description, &f.Severity,
			&f.CVSSScore, &f.EPSSScore, &f.EPSSPercentile, &f.RiskScore,
			&f.Status, &f.Scanner, &f.RawID, &f.Sources,
			&f.TemplateID, &f.AssignedTo, &f.Justification,
			&f.ReopenCount, &f.OccurrenceCount,
			&f.LastSeenAt, &f.SLADueAt, &f.CreatedAt, &f.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan finding row: %w", err)
		}
		findings = append(findings, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate finding rows: %w", err)
	}
	return findings, nil
}

// CountFindings returns the total number of findings matching the filter (ignoring page/limit).
func (r *Repository) CountFindings(ctx context.Context, orgID string, filter FindingFilter) (int, error) {
	args := []interface{}{orgID}
	where := "org_id = $1::uuid"
	argN := 2

	if filter.Severity != "" {
		where += fmt.Sprintf(" AND severity = $%d", argN)
		args = append(args, filter.Severity)
		argN++
	}
	if filter.Status != "" {
		where += fmt.Sprintf(" AND status = $%d", argN)
		args = append(args, filter.Status)
		argN++
	}
	if filter.AssetID != "" {
		where += fmt.Sprintf(" AND asset_id = $%d::uuid", argN)
		args = append(args, filter.AssetID)
	}

	var total int
	if err := r.db.QueryRow(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM vb_findings WHERE %s", where), args...,
	).Scan(&total); err != nil {
		return 0, fmt.Errorf("count findings: %w", err)
	}
	return total, nil
}

// GetFinding fetches a single finding by ID within the org.
func (r *Repository) GetFinding(ctx context.Context, orgID, findingID string) (*Finding, error) {
	var f Finding
	err := r.db.QueryRow(ctx, `
		SELECT id::text, org_id::text, asset_id::text,
		       scan_id::text, cve_id,
		       title, COALESCE(description,''), severity,
		       cvss_score, epss_score, epss_percentile, risk_score,
		       status, scanner, COALESCE(raw_id,''), sources,
		       COALESCE(template_id,''), assigned_to::text, COALESCE(justification,''),
		       reopen_count, occurrence_count,
		       last_seen_at, sla_due_at, created_at, updated_at
		FROM vb_findings
		WHERE id = $1::uuid AND org_id = $2::uuid`,
		findingID, orgID,
	).Scan(
		&f.ID, &f.OrgID, &f.AssetID,
		&f.ScanID, &f.CVEID,
		&f.Title, &f.Description, &f.Severity,
		&f.CVSSScore, &f.EPSSScore, &f.EPSSPercentile, &f.RiskScore,
		&f.Status, &f.Scanner, &f.RawID, &f.Sources,
		&f.TemplateID, &f.AssignedTo, &f.Justification,
		&f.ReopenCount, &f.OccurrenceCount,
		&f.LastSeenAt, &f.SLADueAt, &f.CreatedAt, &f.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get finding: %w", err)
	}
	return &f, nil
}

// UpdateFinding applies a partial update to a finding.
func (r *Repository) UpdateFinding(ctx context.Context, orgID, findingID string, input UpdateFindingInput) (*Finding, error) {
	setClauses := []string{"updated_at = NOW()"}
	args := []interface{}{}
	argN := 1

	if input.Status != nil {
		setClauses = append(setClauses, fmt.Sprintf("status = $%d", argN))
		args = append(args, *input.Status)
		argN++
	}
	if input.AssignedTo != nil {
		setClauses = append(setClauses, fmt.Sprintf("assigned_to = $%d::uuid", argN))
		args = append(args, *input.AssignedTo)
		argN++
	}
	if input.Justification != nil {
		setClauses = append(setClauses, fmt.Sprintf("justification = $%d", argN))
		args = append(args, *input.Justification)
		argN++
	}
	if input.Severity != nil {
		setClauses = append(setClauses, fmt.Sprintf("severity = $%d", argN))
		args = append(args, *input.Severity)
		argN++
	}

	args = append(args, findingID, orgID)
	query := fmt.Sprintf(`
		UPDATE vb_findings
		SET %s
		WHERE id = $%d::uuid AND org_id = $%d::uuid
		RETURNING id::text, org_id::text, asset_id::text,
		          scan_id::text, cve_id,
		          title, COALESCE(description,''), severity,
		          cvss_score, epss_score, epss_percentile, risk_score,
		          status, scanner, COALESCE(raw_id,''), sources,
		          COALESCE(template_id,''), assigned_to::text, COALESCE(justification,''),
		          reopen_count, occurrence_count,
		          last_seen_at, sla_due_at, created_at, updated_at`,
		strings.Join(setClauses, ", "), argN, argN+1)

	var f Finding
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&f.ID, &f.OrgID, &f.AssetID,
		&f.ScanID, &f.CVEID,
		&f.Title, &f.Description, &f.Severity,
		&f.CVSSScore, &f.EPSSScore, &f.EPSSPercentile, &f.RiskScore,
		&f.Status, &f.Scanner, &f.RawID, &f.Sources,
		&f.TemplateID, &f.AssignedTo, &f.Justification,
		&f.ReopenCount, &f.OccurrenceCount,
		&f.LastSeenAt, &f.SLADueAt, &f.CreatedAt, &f.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("update finding: %w", err)
	}
	return &f, nil
}

// BulkUpdateFindings applies a bulk status/assignee update; returns the number of affected rows.
func (r *Repository) BulkUpdateFindings(ctx context.Context, orgID string, input BulkFindingInput) (int, error) {
	if len(input.IDs) == 0 {
		return 0, nil
	}

	setClauses := []string{"updated_at = NOW()"}
	args := []interface{}{}
	argN := 1

	if input.Status != nil {
		setClauses = append(setClauses, fmt.Sprintf("status = $%d", argN))
		args = append(args, *input.Status)
		argN++
	}
	if input.AssignedTo != nil {
		setClauses = append(setClauses, fmt.Sprintf("assigned_to = $%d::uuid", argN))
		args = append(args, *input.AssignedTo)
		argN++
	}

	// Build UUID array placeholder.
	idPlaceholders := make([]string, len(input.IDs))
	for i, id := range input.IDs {
		idPlaceholders[i] = fmt.Sprintf("$%d::uuid", argN)
		args = append(args, id)
		argN++
	}

	args = append(args, orgID)
	query := fmt.Sprintf(`
		UPDATE vb_findings
		SET %s
		WHERE id IN (%s) AND org_id = $%d::uuid`,
		strings.Join(setClauses, ", "),
		strings.Join(idPlaceholders, ", "),
		argN)

	tag, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("bulk update findings: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// ---------------------------------------------------------------------------
// Suppression Rules
// ---------------------------------------------------------------------------

// CreateSuppressionRule inserts a new suppression rule.
func (r *Repository) CreateSuppressionRule(ctx context.Context, orgID, userID string, input CreateSuppressionInput) (*SuppressionRule, error) {
	var rule SuppressionRule
	err := r.db.QueryRow(ctx, `
		INSERT INTO vb_finding_suppressions (org_id, cve_id, asset_tag, reason, created_by)
		VALUES ($1::uuid, $2, $3, $4, $5::uuid)
		RETURNING id::text, org_id::text, cve_id, asset_tag, reason,
		          created_by::text, match_count, created_at`,
		orgID, input.CVEID, input.AssetTag, input.Reason, userID,
	).Scan(
		&rule.ID, &rule.OrgID, &rule.CVEID, &rule.AssetTag, &rule.Reason,
		&rule.CreatedBy, &rule.MatchCount, &rule.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert suppression rule: %w", err)
	}
	return &rule, nil
}

// ListSuppressionRules returns all suppression rules for an org.
func (r *Repository) ListSuppressionRules(ctx context.Context, orgID string) ([]SuppressionRule, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text, cve_id, asset_tag, reason,
		       created_by::text, match_count, created_at
		FROM vb_finding_suppressions
		WHERE org_id = $1::uuid
		ORDER BY created_at DESC`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list suppression rules: %w", err)
	}
	defer rows.Close()

	var rules []SuppressionRule
	for rows.Next() {
		var rule SuppressionRule
		if err := rows.Scan(
			&rule.ID, &rule.OrgID, &rule.CVEID, &rule.AssetTag, &rule.Reason,
			&rule.CreatedBy, &rule.MatchCount, &rule.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan suppression row: %w", err)
		}
		rules = append(rules, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate suppression rows: %w", err)
	}
	return rules, nil
}

// DeleteSuppressionRule deletes a suppression rule by ID within the org.
func (r *Repository) DeleteSuppressionRule(ctx context.Context, orgID, ruleID string) error {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM vb_finding_suppressions
		WHERE id = $1::uuid AND org_id = $2::uuid`,
		ruleID, orgID,
	)
	if err != nil {
		return fmt.Errorf("delete suppression rule: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("suppression rule not found")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Scan Schedules
// ---------------------------------------------------------------------------

// CreateScanSchedule inserts a new scan schedule for an asset.
func (r *Repository) CreateScanSchedule(ctx context.Context, orgID, assetID string, input CreateScanScheduleInput) (*ScanSchedule, error) {
	var s ScanSchedule
	err := r.db.QueryRow(ctx, `
		INSERT INTO vb_scan_schedules (org_id, asset_id, scanner, cron_expr, is_active)
		VALUES ($1::uuid, $2::uuid, $3, $4, TRUE)
		RETURNING id::text, org_id::text, asset_id::text, scanner, cron_expr,
		          is_active, last_run, next_run, created_at`,
		orgID, assetID, input.Scanner, input.CronExpr,
	).Scan(
		&s.ID, &s.OrgID, &s.AssetID, &s.Scanner, &s.CronExpr,
		&s.IsActive, &s.LastRun, &s.NextRun, &s.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert scan schedule: %w", err)
	}
	return &s, nil
}

// ListScanSchedules returns all scan schedules for an asset.
func (r *Repository) ListScanSchedules(ctx context.Context, orgID, assetID string) ([]ScanSchedule, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text, asset_id::text, scanner, cron_expr,
		       is_active, last_run, next_run, created_at
		FROM vb_scan_schedules
		WHERE org_id = $1::uuid AND asset_id = $2::uuid
		ORDER BY created_at DESC`,
		orgID, assetID,
	)
	if err != nil {
		return nil, fmt.Errorf("list scan schedules: %w", err)
	}
	defer rows.Close()

	var schedules []ScanSchedule
	for rows.Next() {
		var s ScanSchedule
		if err := rows.Scan(
			&s.ID, &s.OrgID, &s.AssetID, &s.Scanner, &s.CronExpr,
			&s.IsActive, &s.LastRun, &s.NextRun, &s.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan schedule row: %w", err)
		}
		schedules = append(schedules, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate schedule rows: %w", err)
	}
	return schedules, nil
}

// DeleteScanSchedule removes a scan schedule by ID within the org.
func (r *Repository) DeleteScanSchedule(ctx context.Context, orgID, scheduleID string) error {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM vb_scan_schedules
		WHERE id = $1::uuid AND org_id = $2::uuid`,
		scheduleID, orgID,
	)
	if err != nil {
		return fmt.Errorf("delete scan schedule: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("scan schedule not found")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Risk Trend
// ---------------------------------------------------------------------------

// GetRiskTrend returns daily aggregated risk data over the last N days.
func (r *Repository) GetRiskTrend(ctx context.Context, orgID string, days int) ([]RiskTrendPoint, error) {
	if days <= 0 {
		days = 30
	}
	rows, err := r.db.Query(ctx, `
		SELECT
		    TO_CHAR(d::date, 'YYYY-MM-DD') AS date,
		    COALESCE(SUM(f.risk_score), 0)::float8 AS total_risk_score,
		    COUNT(f.id)                             AS open_count,
		    COUNT(f.id) FILTER (WHERE f.severity = 'critical') AS critical_count
		FROM generate_series(
		    (NOW() - make_interval(days => $2::int))::date,
		    NOW()::date,
		    '1 day'::interval
		) AS d
		LEFT JOIN vb_findings f
		    ON f.org_id = $1::uuid
		   AND f.status = 'open'
		   AND f.created_at < (d::date + INTERVAL '1 day')
		GROUP BY d
		ORDER BY d`,
		orgID, days,
	)
	if err != nil {
		return nil, fmt.Errorf("get risk trend: %w", err)
	}
	defer rows.Close()

	var points []RiskTrendPoint
	for rows.Next() {
		var p RiskTrendPoint
		if err := rows.Scan(&p.Date, &p.TotalRiskScore, &p.OpenCount, &p.CriticalCount); err != nil {
			return nil, fmt.Errorf("scan risk trend row: %w", err)
		}
		points = append(points, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate risk trend rows: %w", err)
	}
	return points, nil
}

// ---------------------------------------------------------------------------
// Reports
// ---------------------------------------------------------------------------

// CreateReport inserts a new report record.
func (r *Repository) CreateReport(ctx context.Context, orgID, userID string, scope ReportScope) (*Report, error) {
	scopeJSON, err := json.Marshal(scope)
	if err != nil {
		return nil, fmt.Errorf("marshal report scope: %w", err)
	}

	var rpt Report
	err = r.db.QueryRow(ctx, `
		INSERT INTO vb_reports (org_id, generated_by, scope, status)
		VALUES ($1::uuid, $2::uuid, $3::jsonb, 'pending')
		RETURNING id::text, org_id::text, generated_by::text, scope,
		          COALESCE(file_path,''), status, expires_at, created_at`,
		orgID, userID, scopeJSON,
	).Scan(
		&rpt.ID, &rpt.OrgID, &rpt.GeneratedBy, &rpt.Scope,
		&rpt.FilePath, &rpt.Status, &rpt.ExpiresAt, &rpt.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert report: %w", err)
	}
	rpt.Title = rpt.Scope.Title
	return &rpt, nil
}

// GetReport fetches a report by ID within the org.
func (r *Repository) GetReport(ctx context.Context, orgID, reportID string) (*Report, error) {
	var rpt Report
	err := r.db.QueryRow(ctx, `
		SELECT id::text, org_id::text, generated_by::text, scope,
		       COALESCE(file_path,''), status, expires_at, created_at
		FROM vb_reports
		WHERE id = $1::uuid AND org_id = $2::uuid`,
		reportID, orgID,
	).Scan(
		&rpt.ID, &rpt.OrgID, &rpt.GeneratedBy, &rpt.Scope,
		&rpt.FilePath, &rpt.Status, &rpt.ExpiresAt, &rpt.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get report: %w", err)
	}
	rpt.Title = rpt.Scope.Title
	return &rpt, nil
}

// ListReports returns reports for an org, newest first (metadata only — no PDF blob).
func (r *Repository) ListReports(ctx context.Context, orgID string) ([]Report, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text, generated_by::text, scope,
		       COALESCE(file_path,''), status, expires_at, created_at
		FROM vb_reports
		WHERE org_id = $1::uuid
		ORDER BY created_at DESC
		LIMIT 100`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list reports: %w", err)
	}
	defer rows.Close()

	var reports []Report
	for rows.Next() {
		var rpt Report
		if err := rows.Scan(
			&rpt.ID, &rpt.OrgID, &rpt.GeneratedBy, &rpt.Scope,
			&rpt.FilePath, &rpt.Status, &rpt.ExpiresAt, &rpt.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan report row: %w", err)
		}
		rpt.Title = rpt.Scope.Title
		reports = append(reports, rpt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate report rows: %w", err)
	}
	return reports, nil
}

// UpsertFindingByRawID inserts a finding or updates it on conflict of
// (org_id, raw_id, scanner). This is used for import operations (SARIF, CycloneDX, CSV).
func (r *Repository) UpsertFindingByRawID(ctx context.Context, orgID string, f Finding) (*Finding, error) {
	sources := f.Sources
	if sources == nil {
		sources = []string{}
	}

	var inserted Finding
	err := r.db.QueryRow(ctx, `
		INSERT INTO vb_findings
		  (org_id, asset_id, cve_id, title, description, severity,
		   cvss_score, status, scanner, raw_id, sources, sla_due_at,
		   reopen_count, occurrence_count, last_seen_at)
		VALUES
		  ($1::uuid, $2::uuid, $3, $4, $5, $6,
		   $7, $8, $9, $10, $11, $12,
		   0, 1, NOW())
		ON CONFLICT (org_id, raw_id, scanner) DO UPDATE
		  SET title            = EXCLUDED.title,
		      description      = EXCLUDED.description,
		      severity         = EXCLUDED.severity,
		      cvss_score       = EXCLUDED.cvss_score,
		      sla_due_at       = EXCLUDED.sla_due_at,
		      occurrence_count = vb_findings.occurrence_count + 1,
		      last_seen_at     = NOW(),
		      updated_at       = NOW()
		RETURNING id::text, org_id::text, asset_id::text,
		          scan_id::text, cve_id,
		          title, COALESCE(description,''), severity,
		          cvss_score, epss_score, epss_percentile, risk_score,
		          status, scanner, COALESCE(raw_id,''), sources,
		          COALESCE(template_id,''), assigned_to::text, COALESCE(justification,''),
		          reopen_count, occurrence_count,
		          last_seen_at, sla_due_at, created_at, updated_at`,
		orgID, f.AssetID, f.CVEID, f.Title, f.Description, f.Severity,
		f.CVSSScore, f.Status, f.Scanner, f.RawID, sources, f.SLADueAt,
	).Scan(
		&inserted.ID, &inserted.OrgID, &inserted.AssetID,
		&inserted.ScanID, &inserted.CVEID,
		&inserted.Title, &inserted.Description, &inserted.Severity,
		&inserted.CVSSScore, &inserted.EPSSScore, &inserted.EPSSPercentile, &inserted.RiskScore,
		&inserted.Status, &inserted.Scanner, &inserted.RawID, &inserted.Sources,
		&inserted.TemplateID, &inserted.AssignedTo, &inserted.Justification,
		&inserted.ReopenCount, &inserted.OccurrenceCount,
		&inserted.LastSeenAt, &inserted.SLADueAt, &inserted.CreatedAt, &inserted.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("upsert finding by raw_id: %w", err)
	}
	return &inserted, nil
}

// UpdateReport updates a report's file path, status, and expiry.
func (r *Repository) UpdateReport(ctx context.Context, reportID, filePath, status string, expiresAt *time.Time) error {
	_, err := r.db.Exec(ctx, `
		UPDATE vb_reports
		SET file_path = $1, status = $2, expires_at = $3
		WHERE id = $4::uuid`,
		filePath, status, expiresAt, reportID,
	)
	if err != nil {
		return fmt.Errorf("update report: %w", err)
	}
	return nil
}

// StoreReportContent saves a generated PDF and marks the report completed.
func (r *Repository) StoreReportContent(ctx context.Context, reportID string, content []byte, expiresAt time.Time) error {
	_, err := r.db.Exec(ctx, `
		UPDATE vb_reports
		SET content = $1, status = 'completed', expires_at = $2, file_path = ''
		WHERE id = $3::uuid`,
		content, expiresAt, reportID,
	)
	if err != nil {
		return fmt.Errorf("store report content: %w", err)
	}
	return nil
}

// GetReportContent returns the raw PDF bytes and title for a completed report.
func (r *Repository) GetReportContent(ctx context.Context, orgID, reportID string) ([]byte, string, error) {
	var content []byte
	var scope ReportScope
	err := r.db.QueryRow(ctx, `
		SELECT content, scope FROM vb_reports
		WHERE id = $1::uuid AND org_id = $2::uuid AND status = 'completed'`,
		reportID, orgID,
	).Scan(&content, &scope)
	if err != nil {
		return nil, "", fmt.Errorf("get report content: %w", err)
	}
	title := scope.Title
	if title == "" {
		title = "report"
	}
	return content, title, nil
}

// ---------------------------------------------------------------------------
// SBOM & EOL
// ---------------------------------------------------------------------------

// CreateSBOM inserts a new SBOM record and its components in a single transaction.
// Returns the newly created SBOM's UUID as a string.
func (r *Repository) CreateSBOM(ctx context.Context, orgID, assetID string, doc SBOMDocument) (string, error) {
	docJSON, err := json.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("marshal SBOM document: %w", err)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var sbomID string
	err = tx.QueryRow(ctx, `
		INSERT INTO vb_sboms (org_id, asset_id, format, spec_version, document, component_count)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6)
		RETURNING id::text`,
		orgID, assetID,
		doc.BOMFormat, doc.SpecVersion,
		docJSON, len(doc.Components),
	).Scan(&sbomID)
	if err != nil {
		return "", fmt.Errorf("insert vb_sboms: %w", err)
	}

	for _, comp := range doc.Components {
		purl := comp.PURL
		_, err := tx.Exec(ctx, `
			INSERT INTO vb_components (org_id, sbom_id, name, version, purl)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5)
			ON CONFLICT (sbom_id, name, version) DO NOTHING`,
			orgID, sbomID, comp.Name, comp.Version, purl,
		)
		if err != nil {
			return "", fmt.Errorf("insert vb_components (%s %s): %w", comp.Name, comp.Version, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit SBOM tx: %w", err)
	}
	return sbomID, nil
}

// GetLatestSBOM returns the most recently created SBOM summary for the given asset.
func (r *Repository) GetLatestSBOM(ctx context.Context, orgID, assetID string) (*SBOMSummary, error) {
	var s SBOMSummary
	err := r.db.QueryRow(ctx, `
		SELECT id::text, asset_id::text, format, component_count, created_at
		FROM vb_sboms
		WHERE org_id = $1::uuid AND asset_id = $2::uuid
		ORDER BY created_at DESC
		LIMIT 1`,
		orgID, assetID,
	).Scan(&s.ID, &s.AssetID, &s.Format, &s.ComponentCount, &s.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get latest SBOM: %w", err)
	}
	return &s, nil
}

// ListComponentsWithEOL returns paginated components for an org, optionally filtered to EOL-only.
// page is 1-based; up to 500 rows per page.
func (r *Repository) ListComponentsWithEOL(ctx context.Context, orgID string, eolOnly bool, page int) ([]ComponentSummary, error) {
	if page < 1 {
		page = 1
	}
	const limit = 500
	offset := (page - 1) * limit

	query := `
		SELECT c.id::text, c.name, c.version, COALESCE(c.purl,''),
		       c.eol_status,
		       CASE WHEN c.eol_date IS NOT NULL THEN c.eol_date::text ELSE NULL END,
		       s.asset_id::text
		FROM vb_components c
		JOIN vb_sboms s ON s.id = c.sbom_id
		WHERE c.org_id = $1::uuid`
	if eolOnly {
		query += " AND c.eol_status = 'eol'"
	}
	query += " ORDER BY c.name, c.version LIMIT $2 OFFSET $3"

	rows, err := r.db.Query(ctx, query, orgID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list components: %w", err)
	}
	defer rows.Close()

	var result []ComponentSummary
	for rows.Next() {
		var comp ComponentSummary
		if err := rows.Scan(
			&comp.ID, &comp.Name, &comp.Version, &comp.PURL,
			&comp.EOLStatus, &comp.EOLDate, &comp.AssetID,
		); err != nil {
			return nil, fmt.Errorf("scan component: %w", err)
		}
		result = append(result, comp)
	}
	return result, rows.Err()
}

// listComponentsBySBOM returns the raw component rows for a given sbom_id (used by EOLChecker).
func (r *Repository) listComponentsBySBOM(ctx context.Context, sbomID string) ([]componentRow, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, name, version
		FROM vb_components
		WHERE sbom_id = $1::uuid`,
		sbomID,
	)
	if err != nil {
		return nil, fmt.Errorf("list components by SBOM: %w", err)
	}
	defer rows.Close()

	var result []componentRow
	for rows.Next() {
		var c componentRow
		if err := rows.Scan(&c.ID, &c.Name, &c.Version); err != nil {
			return nil, fmt.Errorf("scan component row: %w", err)
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

// updateComponentEOL sets the eol_status and eol_date for a component.
func (r *Repository) updateComponentEOL(ctx context.Context, componentID, eolStatus string, eolDate *string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE vb_components
		SET eol_status = $1,
		    eol_date = $2::date,
		    eol_checked_at = NOW()
		WHERE id = $3::uuid`,
		eolStatus, eolDate, componentID,
	)
	if err != nil {
		return fmt.Errorf("update component EOL: %w", err)
	}
	return nil
}

// getEOLCache returns the cached payload for a (product, cycle) pair along with when it was fetched.
func (r *Repository) getEOLCache(ctx context.Context, product, cycle string) ([]byte, time.Time, error) {
	var payload []byte
	var fetchedAt time.Time
	err := r.db.QueryRow(ctx, `
		SELECT payload, fetched_at
		FROM vb_eol_cache
		WHERE product = $1 AND cycle = $2`,
		product, cycle,
	).Scan(&payload, &fetchedAt)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("get EOL cache: %w", err)
	}
	return payload, fetchedAt, nil
}

// upsertEOLCache inserts or updates a cache row for the (product, cycle) pair.
func (r *Repository) upsertEOLCache(ctx context.Context, product, cycle string, payload []byte) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO vb_eol_cache (product, cycle, payload, fetched_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (product, cycle) DO UPDATE
		  SET payload = EXCLUDED.payload,
		      fetched_at = EXCLUDED.fetched_at`,
		product, cycle, payload,
	)
	if err != nil {
		return fmt.Errorf("upsert EOL cache: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Batch finding upsert
// ---------------------------------------------------------------------------

// BatchUpsertFindings inserts or deduplicates multiple findings in a single
// pgx.Batch round-trip. Each finding is upserted using the same logic as
// UpsertFinding but without returning the full row, to minimise wire overhead.
// Errors for individual rows are logged but do not abort the batch; the number
// of successfully processed rows is returned.
func (r *Repository) BatchUpsertFindings(ctx context.Context, orgID string, findings []Finding) (int, error) {
	if len(findings) == 0 {
		return 0, nil
	}

	batch := &pgx.Batch{}
	for _, f := range findings {
		sources := f.Sources
		if sources == nil {
			sources = []string{}
		}

		if f.CVEID != nil && *f.CVEID != "" {
			// CVE-keyed upsert: merge on (org_id, asset_id, cve_id).
			batch.Queue(`
				INSERT INTO vb_findings
				  (org_id, asset_id, scan_id, cve_id, title, description, severity,
				   cvss_score, epss_score, epss_percentile, risk_score,
				   status, scanner, raw_id, sources, template_id,
				   assigned_to, justification, reopen_count, occurrence_count, last_seen_at)
				VALUES
				  ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7,
				   $8, $9, $10, $11,
				   $12, $13, $14, $15, $16,
				   $17::uuid, $18, 0, 1, NOW())
				ON CONFLICT (org_id, asset_id, cve_id) WHERE cve_id IS NOT NULL DO UPDATE
				  SET last_seen_at     = NOW(),
				      occurrence_count = vb_findings.occurrence_count + 1,
				      status           = CASE
				                          WHEN vb_findings.status IN ('resolved','false_positive') THEN 'open'
				                          ELSE vb_findings.status
				                        END,
				      reopen_count     = CASE
				                          WHEN vb_findings.status IN ('resolved','false_positive') THEN vb_findings.reopen_count + 1
				                          ELSE vb_findings.reopen_count
				                        END,
				      sources          = (SELECT ARRAY(SELECT DISTINCT unnest(vb_findings.sources || EXCLUDED.sources))),
				      updated_at       = NOW()`,
				orgID, f.AssetID, f.ScanID, f.CVEID, f.Title, f.Description, f.Severity,
				f.CVSSScore, f.EPSSScore, f.EPSSPercentile, f.RiskScore,
				f.Status, f.Scanner, f.RawID, sources, f.TemplateID,
				f.AssignedTo, f.Justification,
			)
		} else if f.TemplateID != "" {
			// Template-keyed upsert: merge on (org_id, asset_id, scanner, template_id).
			batch.Queue(`
				INSERT INTO vb_findings
				  (org_id, asset_id, scan_id, cve_id, title, description, severity,
				   cvss_score, epss_score, epss_percentile, risk_score,
				   status, scanner, raw_id, sources, template_id,
				   assigned_to, justification, reopen_count, occurrence_count, last_seen_at)
				VALUES
				  ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7,
				   $8, $9, $10, $11,
				   $12, $13, $14, $15, $16,
				   $17::uuid, $18, 0, 1, NOW())
				ON CONFLICT (org_id, asset_id, scanner, template_id) WHERE template_id IS NOT NULL DO UPDATE
				  SET last_seen_at     = NOW(),
				      occurrence_count = vb_findings.occurrence_count + 1,
				      status           = CASE
				                          WHEN vb_findings.status IN ('resolved','false_positive') THEN 'open'
				                          ELSE vb_findings.status
				                        END,
				      reopen_count     = CASE
				                          WHEN vb_findings.status IN ('resolved','false_positive') THEN vb_findings.reopen_count + 1
				                          ELSE vb_findings.reopen_count
				                        END,
				      sources          = (SELECT ARRAY(SELECT DISTINCT unnest(vb_findings.sources || EXCLUDED.sources))),
				      updated_at       = NOW()`,
				orgID, f.AssetID, f.ScanID, f.CVEID, f.Title, f.Description, f.Severity,
				f.CVSSScore, f.EPSSScore, f.EPSSPercentile, f.RiskScore,
				f.Status, f.Scanner, f.RawID, sources, f.TemplateID,
				f.AssignedTo, f.Justification,
			)
		} else {
			// No dedup key: plain insert.
			batch.Queue(`
				INSERT INTO vb_findings
				  (org_id, asset_id, scan_id, cve_id, title, description, severity,
				   cvss_score, epss_score, epss_percentile, risk_score,
				   status, scanner, raw_id, sources, template_id,
				   assigned_to, justification, reopen_count, occurrence_count, last_seen_at)
				VALUES
				  ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7,
				   $8, $9, $10, $11,
				   $12, $13, $14, $15, $16,
				   $17::uuid, $18, 0, 1, NOW())`,
				orgID, f.AssetID, f.ScanID, f.CVEID, f.Title, f.Description, f.Severity,
				f.CVSSScore, f.EPSSScore, f.EPSSPercentile, f.RiskScore,
				f.Status, f.Scanner, f.RawID, sources, f.TemplateID,
				f.AssignedTo, f.Justification,
			)
		}
	}

	br := r.db.SendBatch(ctx, batch)
	defer br.Close()

	var count int
	for i := range findings {
		if _, err := br.Exec(); err != nil {
			log.Error().Err(err).Int("index", i).Msg("batch upsert finding: row failed")
		} else {
			count++
		}
	}
	return count, nil
}

// ---------------------------------------------------------------------------
// EOL batch helpers
// ---------------------------------------------------------------------------

// eolCacheRow is an internal struct for batch cache lookups.
type eolCacheRow struct {
	payload   []byte
	fetchedAt time.Time
}

// batchGetEOLCache loads all cache entries for the given (product, cycle) pairs
// in a single query. Returns a map keyed by [product, cycle].
func (r *Repository) batchGetEOLCache(ctx context.Context, pairs [][2]string) (map[[2]string]eolCacheRow, error) {
	result := make(map[[2]string]eolCacheRow, len(pairs))
	if len(pairs) == 0 {
		return result, nil
	}

	// Build WHERE clause: (product, cycle) IN (($1,$2), ($3,$4), ...)
	args := make([]interface{}, 0, len(pairs)*2)
	placeholders := make([]string, 0, len(pairs))
	for i, p := range pairs {
		a := i * 2
		placeholders = append(placeholders, fmt.Sprintf("($%d, $%d)", a+1, a+2))
		args = append(args, p[0], p[1])
	}

	query := fmt.Sprintf(`
		SELECT product, cycle, payload, fetched_at
		FROM vb_eol_cache
		WHERE (product, cycle) IN (%s)`, strings.Join(placeholders, ", "))

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("batch get EOL cache: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var product, cycle string
		var row eolCacheRow
		if err := rows.Scan(&product, &cycle, &row.payload, &row.fetchedAt); err != nil {
			continue
		}
		result[[2]string{product, cycle}] = row
	}
	return result, rows.Err()
}

// batchUpdateComponentEOL updates eol_status and eol_date for multiple components
// in a single statement using an unnest-based approach.
func (r *Repository) batchUpdateComponentEOL(ctx context.Context, results []eolResult) error {
	if len(results) == 0 {
		return nil
	}

	ids := make([]string, len(results))
	statuses := make([]string, len(results))
	dates := make([]*string, len(results))
	for i, res := range results {
		ids[i] = res.componentID
		statuses[i] = res.eolStatus
		dates[i] = res.eolDate
	}

	_, err := r.db.Exec(ctx, `
		UPDATE vb_components AS c
		SET eol_status    = v.status,
		    eol_date      = v.eol_date::date,
		    eol_checked_at = NOW()
		FROM (
		    SELECT
		        UNNEST($1::uuid[])   AS id,
		        UNNEST($2::text[])   AS status,
		        UNNEST($3::text[])   AS eol_date
		) AS v
		WHERE c.id = v.id`,
		ids, statuses, dates,
	)
	if err != nil {
		return fmt.Errorf("batch update component EOL: %w", err)
	}
	return nil
}
