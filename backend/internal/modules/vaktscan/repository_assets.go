// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktscan

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

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
