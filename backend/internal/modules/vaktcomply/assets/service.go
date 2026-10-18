// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package assets

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// csvMaxRows caps the extended CSV import per request.
const csvMaxRows = 500

// Service holds the asset inventory business logic.
type Service struct {
	db   *pgxpool.Pool
	repo *Repository
}

// NewService creates a new asset service.
func NewService(pool *pgxpool.Pool) *Service {
	return &Service{db: pool, repo: NewRepository(pool)}
}

// CreateAsset validates and creates a new asset.
func (s *Service) CreateAsset(ctx context.Context, orgID string, input CreateAssetInput) (*Asset, error) {
	if input.Criticality == "" {
		input.Criticality = "medium"
	}
	return s.repo.CreateAsset(ctx, orgID, input)
}

// ListAssets returns a paginated list of assets for the org.
func (s *Service) ListAssets(ctx context.Context, orgID string, page, limit int, tag string) ([]Asset, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 500 {
		limit = 25
	}
	return s.repo.ListAssets(ctx, orgID, page, limit, tag)
}

// GetAsset fetches a single asset by ID.
func (s *Service) GetAsset(ctx context.Context, orgID, assetID string) (*Asset, error) {
	return s.repo.GetAsset(ctx, orgID, assetID)
}

// UpdateAsset applies a partial update.
func (s *Service) UpdateAsset(ctx context.Context, orgID, assetID string, input UpdateAssetInput) (*Asset, error) {
	return s.repo.UpdateAsset(ctx, orgID, assetID, input)
}

// GetAssetProtectionNeedID returns the protection_need_id soft-link for an asset, or nil if unlinked.
func (s *Service) GetAssetProtectionNeedID(ctx context.Context, orgID, assetID string) (*string, error) {
	return s.repo.GetAssetProtectionNeedID(ctx, orgID, assetID)
}

// GetClassificationSummary returns asset counts grouped by classification level (S67-3).
func (s *Service) GetClassificationSummary(ctx context.Context, orgID string) (*ClassificationSummary, error) {
	return s.repo.GetClassificationSummary(ctx, orgID)
}

// DeleteAsset soft-deletes an asset.
func (s *Service) DeleteAsset(ctx context.Context, orgID, assetID string) error {
	return s.repo.SoftDeleteAsset(ctx, orgID, assetID)
}

// ImportAssetsCSV parses a CSV reader and bulk-inserts assets.
// Returns (inserted, errored, errorMessages, error).
// The CSV must have a header row: name,type,criticality,tags,external_url
// Tags are comma-separated within a quoted field.
func (s *Service) ImportAssetsCSV(ctx context.Context, orgID string, r io.Reader) (int, int, []string, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true

	header, err := reader.Read()
	if err != nil {
		return 0, 0, nil, fmt.Errorf("read CSV header: %w", err)
	}

	// Build column index map for flexible column ordering.
	colIdx := make(map[string]int, len(header))
	for i, h := range header {
		colIdx[strings.ToLower(strings.TrimSpace(h))] = i
	}

	required := []string{"name", "type"}
	for _, col := range required {
		if _, ok := colIdx[col]; !ok {
			return 0, 0, nil, fmt.Errorf("CSV missing required column %q", col)
		}
	}

	var rows []CSVAssetRow
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, 0, nil, fmt.Errorf("read CSV row: %w", err)
		}

		row := CSVAssetRow{}

		if i, ok := colIdx["name"]; ok && i < len(record) {
			row.Name = strings.TrimSpace(record[i])
		}
		if i, ok := colIdx["type"]; ok && i < len(record) {
			row.Type = strings.TrimSpace(record[i])
		}
		if i, ok := colIdx["criticality"]; ok && i < len(record) {
			row.Criticality = strings.TrimSpace(record[i])
		}
		if row.Criticality == "" {
			row.Criticality = "medium"
		}
		if i, ok := colIdx["tags"]; ok && i < len(record) {
			raw := strings.TrimSpace(record[i])
			if raw != "" {
				for _, t := range strings.Split(raw, ",") {
					if tag := strings.TrimSpace(t); tag != "" {
						row.Tags = append(row.Tags, tag)
					}
				}
			}
		}
		if i, ok := colIdx["external_url"]; ok && i < len(record) {
			row.ExternalURL = strings.TrimSpace(record[i])
		}

		rows = append(rows, row)
	}

	if len(rows) == 0 {
		return 0, 0, nil, fmt.Errorf("CSV file contains no data rows")
	}

	inserted, errored, errs := s.repo.BulkCreateAssets(ctx, orgID, rows)
	return inserted, errored, errs, nil
}

// ImportAssetsCSVExtended reads the extended asset CSV format
// name,type,ip,owner,criticality and returns (imported, skipped, errors).
func (s *Service) ImportAssetsCSVExtended(ctx context.Context, orgID string, r io.Reader) (int, int, []string, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true

	header, err := reader.Read()
	if err != nil {
		return 0, 0, nil, fmt.Errorf("read CSV header: %w", err)
	}

	colIdx := make(map[string]int, len(header))
	for i, h := range header {
		colIdx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	if _, ok := colIdx["name"]; !ok {
		return 0, 0, nil, fmt.Errorf("CSV missing required column \"name\"")
	}

	col := func(record []string, name string) string {
		if i, ok := colIdx[name]; ok && i < len(record) {
			return strings.TrimSpace(record[i])
		}
		return ""
	}

	var rows []CSVAssetRow
	var skipped int
	var errs []string
	rowNum := 0

	for {
		record, readErr := reader.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return 0, skipped, errs, fmt.Errorf("read CSV row: %w", readErr)
		}

		rowNum++
		if rowNum > csvMaxRows {
			skipped++
			errs = append(errs, fmt.Sprintf("row %d: exceeded maximum of %d rows per import", rowNum, csvMaxRows))
			continue
		}

		name := col(record, "name")
		if name == "" {
			skipped++
			errs = append(errs, fmt.Sprintf("row %d: name is empty, skipped", rowNum))
			continue
		}

		assetType := col(record, "type")
		if assetType == "" {
			assetType = "server"
		}

		criticality := col(record, "criticality")
		if criticality == "" {
			criticality = "medium"
		}

		// ip is stored as external_url (there is no dedicated ip column in vb_assets).
		ip := col(record, "ip")

		rows = append(rows, CSVAssetRow{
			Name:        name,
			Type:        assetType,
			Criticality: criticality,
			ExternalURL: ip,
		})
	}

	if len(rows) == 0 {
		return 0, skipped, errs, nil
	}

	inserted, errored, bulkErrs := s.repo.BulkCreateAssets(ctx, orgID, rows)
	errs = append(errs, bulkErrs...)
	return inserted, skipped + errored, errs, nil
}
