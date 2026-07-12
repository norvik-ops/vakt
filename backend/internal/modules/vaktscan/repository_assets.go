// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktscan

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/db"
)

// enrichEnvironments fetches the environment column for a slice of assets via a
// single IN-query and patches the slice in place. Called after every sqlc-based
// asset read because the environment column was added after sqlc generation.
func (r *Repository) enrichEnvironments(ctx context.Context, assets []Asset) {
	if len(assets) == 0 {
		return
	}
	ids := make([]string, len(assets))
	for i, a := range assets {
		ids[i] = a.ID
	}
	// orgid-lint: global — IDs come from a prior org-scoped query; update by PK is safe
	rows, err := r.db.Query(ctx,
		`SELECT id, environment FROM vb_assets WHERE id = ANY($1)`, ids)
	if err != nil {
		return
	}
	defer rows.Close()
	envMap := make(map[string]string, len(assets))
	for rows.Next() {
		var id, env string
		if rows.Scan(&id, &env) == nil {
			envMap[id] = env
		}
	}
	for i := range assets {
		if env, ok := envMap[assets[i].ID]; ok {
			assets[i].Environment = env
		}
	}
}

// GetClassificationSummary returns asset counts grouped by classification level.
func (r *Repository) GetClassificationSummary(ctx context.Context, orgID string) (*ClassificationSummary, error) {
	rows, err := r.db.Query(ctx,
		`SELECT COALESCE(classification, 'unclassified') AS cls, COUNT(*) AS cnt
		   FROM vb_assets
		  WHERE org_id = $1::uuid AND (is_deleted IS NULL OR is_deleted = false)
		  GROUP BY cls`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("get classification summary: %w", err)
	}
	defer rows.Close()

	levels := map[string]int{"public": 0, "internal": 0, "confidential": 0, "restricted": 0}
	unclassified := 0
	total := 0
	for rows.Next() {
		var cls string
		var cnt int
		if err := rows.Scan(&cls, &cnt); err != nil {
			return nil, fmt.Errorf("scan classification row: %w", err)
		}
		total += cnt
		if cls == "unclassified" || cls == "" {
			unclassified += cnt
		} else {
			levels[cls] += cnt
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("classification summary rows: %w", err)
	}

	classified := total - unclassified
	return &ClassificationSummary{
		TotalCount:        total,
		ClassifiedCount:   classified,
		ByLevel:           levels,
		UnclassifiedCount: unclassified,
	}, nil
}

// CreateAsset inserts a new asset row and returns the created record.
func (r *Repository) CreateAsset(ctx context.Context, orgID string, input CreateAssetInput) (*Asset, error) {
	tags := input.Tags
	if tags == nil {
		tags = []string{}
	}
	row, err := r.q.CreateSPAsset(ctx, db.CreateSPAssetParams{
		OrgID:       orgID,
		Name:        input.Name,
		Type:        input.Type,
		Criticality: input.Criticality,
		Tags:        tags,
		OwnerID:     spOptUUID(input.OwnerID),
		ExternalUrl: spOptText(input.ExternalURL),
	})
	if err != nil {
		return nil, fmt.Errorf("insert asset: %w", err)
	}
	env := input.Environment
	if env == "" {
		env = "prod"
	}
	// orgid-lint: global — UPDATE by PK row.ID from the RETURNING clause of the INSERT just above
	if _, execErr := r.db.Exec(ctx,
		`UPDATE vb_assets SET environment=$1 WHERE id=$2`, env, row.ID); execErr != nil {
		log.Warn().Err(execErr).Str("asset_id", row.ID).Msg("could not set environment on new asset")
	}
	cls := input.Classification
	if cls == "" {
		cls = "internal"
	}
	// orgid-lint: global — UPDATE by PK row.ID from the RETURNING clause of the INSERT just above
	if _, execErr := r.db.Exec(ctx,
		`UPDATE vb_assets SET classification=$1 WHERE id=$2`, cls, row.ID); execErr != nil {
		log.Warn().Err(execErr).Str("asset_id", row.ID).Msg("could not set classification on new asset")
	}
	a := assetFromFields(assetFields{
		ID: row.ID, OrgID: row.OrgID, Name: row.Name, Type: row.Type,
		Criticality: row.Criticality, Environment: env, Tags: row.Tags,
		OwnerID: row.OwnerID, ExternalUrl: row.ExternalUrl,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	a.Classification = cls
	return &a, nil
}

// ListAssets returns a paginated list of non-deleted assets for an org.
// An optional tag filter restricts results to assets containing that tag.
func (r *Repository) ListAssets(ctx context.Context, orgID string, page, limit int, tag string) ([]Asset, int, error) {
	var tagParam pgtype.Text
	if tag != "" {
		tagParam = pgtype.Text{String: tag, Valid: true}
	}
	total, err := r.q.CountSPAssets(ctx, db.CountSPAssetsParams{
		OrgID: orgID,
		Tag:   tagParam,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("count assets: %w", err)
	}
	offset := (page - 1) * limit
	rows, err := r.q.ListSPAssets(ctx, db.ListSPAssetsParams{
		OrgID:  orgID,
		Tag:    tagParam,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("query assets: %w", err)
	}
	out := make([]Asset, 0, len(rows))
	for _, row := range rows {
		out = append(out, assetFromFields(assetFields{
			ID: row.ID, OrgID: row.OrgID, Name: row.Name, Type: row.Type,
			Criticality: row.Criticality, Classification: row.Classification, Tags: row.Tags,
			OwnerID: row.OwnerID, ExternalUrl: row.ExternalUrl,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}))
	}
	r.enrichEnvironments(ctx, out)
	return out, int(total), nil
}

// GetAsset fetches a single non-deleted asset by ID within the org.
func (r *Repository) GetAsset(ctx context.Context, orgID, assetID string) (*Asset, error) {
	row, err := r.q.GetSPAsset(ctx, db.GetSPAssetParams{ID: assetID, OrgID: orgID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("asset not found: %w", err)
		}
		return nil, fmt.Errorf("get asset: %w", err)
	}
	tmp := []Asset{assetFromFields(assetFields{
		ID: row.ID, OrgID: row.OrgID, Name: row.Name, Type: row.Type,
		Criticality: row.Criticality, Classification: row.Classification, Tags: row.Tags,
		OwnerID: row.OwnerID, ExternalUrl: row.ExternalUrl,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})}
	r.enrichEnvironments(ctx, tmp)
	return &tmp[0], nil
}

// GetAssetByName fetches the first non-deleted asset matching name (case-insensitive) within the org.
// Returns nil, nil when no asset matches.
func (r *Repository) GetAssetByName(ctx context.Context, orgID, name string) (*Asset, error) {
	row, err := r.q.GetSPAssetByName(ctx, db.GetSPAssetByNameParams{OrgID: orgID, Column2: name})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get asset by name: %w", err)
	}
	tmp := []Asset{assetFromFields(assetFields{
		ID: row.ID, OrgID: row.OrgID, Name: row.Name, Type: row.Type,
		Criticality: row.Criticality, Classification: row.Classification, Tags: row.Tags,
		OwnerID: row.OwnerID, ExternalUrl: row.ExternalUrl,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})}
	r.enrichEnvironments(ctx, tmp)
	return &tmp[0], nil
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
// Read-merge-write because sqlc cannot generate dynamic SET clauses (ADR-0005).
func (r *Repository) UpdateAsset(ctx context.Context, orgID, assetID string, input UpdateAssetInput) (*Asset, error) {
	cur, err := r.GetAsset(ctx, orgID, assetID)
	if err != nil {
		return nil, err
	}

	params := db.UpdateSPAssetParams{
		ID:          assetID,
		OrgID:       orgID,
		Name:        cur.Name,
		Type:        cur.Type,
		Criticality: cur.Criticality,
		Tags:        cur.Tags,
		OwnerID:     spOptUUID(cur.OwnerID),
		ExternalUrl: spOptText(derefStrPtr(cur.ExternalURL)),
	}
	if input.Name != nil {
		params.Name = *input.Name
	}
	if input.Type != nil {
		params.Type = *input.Type
	}
	if input.Criticality != nil {
		params.Criticality = *input.Criticality
	}
	if input.Tags != nil {
		params.Tags = input.Tags
	}
	if input.OwnerID != nil {
		params.OwnerID = spOptUUID(input.OwnerID)
	}
	if input.ExternalURL != nil {
		params.ExternalUrl = spOptText(*input.ExternalURL)
	}

	row, err := r.q.UpdateSPAsset(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("update asset: %w", err)
	}
	newEnv := cur.Environment
	if input.Environment != nil {
		newEnv = *input.Environment
		if _, execErr := r.db.Exec(ctx,
			`UPDATE vb_assets SET environment=$1 WHERE id=$2 AND org_id=$3`,
			newEnv, assetID, orgID); execErr != nil {
			log.Warn().Err(execErr).Str("asset_id", assetID).Msg("could not update environment")
		}
	}
	newCls := cur.Classification
	if input.Classification != nil {
		newCls = *input.Classification
		if _, execErr := r.db.Exec(ctx,
			`UPDATE vb_assets SET classification=$1 WHERE id=$2 AND org_id=$3`,
			newCls, assetID, orgID); execErr != nil {
			log.Warn().Err(execErr).Str("asset_id", assetID).Msg("could not update classification")
		}
	}
	a := assetFromFields(assetFields{
		ID: row.ID, OrgID: row.OrgID, Name: row.Name, Type: row.Type,
		Criticality: row.Criticality, Environment: newEnv, Tags: row.Tags,
		OwnerID: row.OwnerID, ExternalUrl: row.ExternalUrl,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	a.Classification = newCls
	return &a, nil
}

// SoftDeleteAsset marks an asset as deleted (is_deleted = TRUE).
func (r *Repository) SoftDeleteAsset(ctx context.Context, orgID, assetID string) error {
	n, err := r.q.SoftDeleteSPAsset(ctx, db.SoftDeleteSPAssetParams{ID: assetID, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("soft delete asset: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("asset not found")
	}
	return nil
}

// GetAssetProtectionNeedID returns the protection_need_id soft-link for an asset, or nil if unlinked.
func (r *Repository) GetAssetProtectionNeedID(ctx context.Context, orgID, assetID string) (*string, error) {
	var pnaID *string
	// S121: vb_assets has no deleted_at column — soft-delete is the is_deleted
	// boolean. The old `deleted_at IS NULL` 500'd for every asset (SQLSTATE 42703),
	// found by the live route sweep.
	err := r.db.QueryRow(ctx,
		`SELECT protection_need_id FROM vb_assets
		 WHERE id = $1::uuid AND org_id = $2::uuid AND is_deleted = FALSE`,
		assetID, orgID,
	).Scan(&pnaID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get asset protection need id: %w", err)
	}
	return pnaID, nil
}

// derefStrPtr returns "" for a nil *string.
func derefStrPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// GetSLAConfig fetches the SLA configuration for an org; returns defaults if absent.
func (r *Repository) GetSLAConfig(ctx context.Context, orgID string) (*SLAConfig, error) {
	row, err := r.q.GetSPSLAConfig(ctx, orgID)
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
	return &SLAConfig{
		OrgID:        row.OrgID,
		CriticalDays: int(row.CriticalDays),
		HighDays:     int(row.HighDays),
		MediumDays:   int(row.MediumDays),
		LowDays:      int(row.LowDays),
	}, nil
}

// UpsertSLAConfig inserts or updates the SLA config for an org.
func (r *Repository) UpsertSLAConfig(ctx context.Context, orgID string, input SLAConfig) error {
	err := r.q.UpsertSPSLAConfig(ctx, db.UpsertSPSLAConfigParams{
		OrgID:        orgID,
		CriticalDays: int32(input.CriticalDays),
		HighDays:     int32(input.HighDays),
		MediumDays:   int32(input.MediumDays),
		LowDays:      int32(input.LowDays),
	})
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
	rows, err := r.q.GetSPSLADashboard(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("sla dashboard query: %w", err)
	}
	result := make([]slaDashboardRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, slaDashboardRow{
			AssetID:      row.AssetID,
			AssetName:    row.AssetName,
			FindingID:    row.FindingID,
			FindingTitle: row.FindingTitle,
			Severity:     row.Severity,
			Status:       row.Status,
			DaysOpen:     int(row.DaysOpen),
		})
	}
	return result, nil
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
		_ = tx.Rollback(ctx) // no-op when Commit succeeded
	}()
	qtx := r.q.WithTx(tx)

	for i, row := range rows {
		tags := row.Tags
		if tags == nil {
			tags = []string{}
		}
		_, scanErr := qtx.CreateSPAsset(ctx, db.CreateSPAssetParams{
			OrgID:       orgID,
			Name:        row.Name,
			Type:        row.Type,
			Criticality: row.Criticality,
			Tags:        tags,
			OwnerID:     pgtype.UUID{}, // bulk import has no owner
			ExternalUrl: spOptText(row.ExternalURL),
		})
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
