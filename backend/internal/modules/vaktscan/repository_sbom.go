// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktscan

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/db"
)

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
	defer func() { _ = tx.Rollback(ctx) }() // no-op when Commit succeeded
	qtx := r.q.WithTx(tx)

	sbomID, err := qtx.CreateSPSBOM(ctx, db.CreateSPSBOMParams{
		OrgID:          orgID,
		AssetID:        assetID,
		Format:         doc.BOMFormat,
		SpecVersion:    doc.SpecVersion,
		Document:       docJSON,
		ComponentCount: int32(len(doc.Components)),
	})
	if err != nil {
		return "", fmt.Errorf("insert vb_sboms: %w", err)
	}

	for _, comp := range doc.Components {
		if err := qtx.InsertSPComponent(ctx, db.InsertSPComponentParams{
			OrgID:   orgID,
			SbomID:  sbomID,
			Name:    comp.Name,
			Version: comp.Version,
			Purl:    spOptText(comp.PURL),
		}); err != nil {
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
	row, err := r.q.GetLatestSPSBOM(ctx, db.GetLatestSPSBOMParams{OrgID: orgID, AssetID: assetID})
	if err != nil {
		return nil, fmt.Errorf("get latest SBOM: %w", err)
	}
	return &SBOMSummary{
		ID:             row.ID,
		AssetID:        row.AssetID,
		Format:         row.Format,
		ComponentCount: int(row.ComponentCount),
		CreatedAt:      spTsToTime(row.CreatedAt),
	}, nil
}

// ListComponentsWithEOL returns paginated components for an org, optionally filtered to EOL-only.
// page is 1-based; up to 500 rows per page.
func (r *Repository) ListComponentsWithEOL(ctx context.Context, orgID string, eolOnly bool, page int) ([]ComponentSummary, error) {
	if page < 1 {
		page = 1
	}
	const limit = 500
	offset := (page - 1) * limit

	if eolOnly {
		rows, err := r.q.ListSPComponentsEOL(ctx, db.ListSPComponentsEOLParams{
			OrgID:  orgID,
			Limit:  int32(limit),
			Offset: int32(offset),
		})
		if err != nil {
			return nil, fmt.Errorf("list components: %w", err)
		}
		out := make([]ComponentSummary, 0, len(rows))
		for _, c := range rows {
			out = append(out, ComponentSummary{
				ID: c.ID, Name: c.Name, Version: c.Version,
				PURL: c.Purl.String, EOLStatus: c.EolStatus,
				EOLDate: dateToStringPtr(c.EolDate), AssetID: c.AssetID,
			})
		}
		return out, nil
	}
	rows, err := r.q.ListSPComponentsAll(ctx, db.ListSPComponentsAllParams{
		OrgID:  orgID,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("list components: %w", err)
	}
	out := make([]ComponentSummary, 0, len(rows))
	for _, c := range rows {
		out = append(out, ComponentSummary{
			ID: c.ID, Name: c.Name, Version: c.Version,
			PURL: c.Purl.String, EOLStatus: c.EolStatus,
			EOLDate: dateToStringPtr(c.EolDate), AssetID: c.AssetID,
		})
	}
	return out, nil
}

// listComponentsBySBOM returns the raw component rows for a given sbom_id (used by EOLChecker).
func (r *Repository) listComponentsBySBOM(ctx context.Context, sbomID string) ([]componentRow, error) {
	rows, err := r.q.ListSPComponentsBySBOM(ctx, sbomID)
	if err != nil {
		return nil, fmt.Errorf("list components by SBOM: %w", err)
	}
	out := make([]componentRow, 0, len(rows))
	for _, c := range rows {
		out = append(out, componentRow{ID: c.ID, Name: c.Name, Version: c.Version})
	}
	return out, nil
}

// upsertEOLCache inserts or updates a cache row for the (product, cycle) pair.
func (r *Repository) upsertEOLCache(ctx context.Context, product, cycle string, payload []byte) error {
	err := r.q.UpsertSPEOLCache(ctx, db.UpsertSPEOLCacheParams{
		Product: product,
		Cycle:   cycle,
		Payload: payload,
	})
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
	args := make([]any, 0, len(pairs)*2)
	placeholders := make([]string, 0, len(pairs))
	for i, p := range pairs {
		a := i * 2
		placeholders = append(placeholders, fmt.Sprintf("($%d, $%d)", a+1, a+2))
		args = append(args, p[0], p[1])
	}

	// orgid-lint: global — vb_eol_cache is a shared product lifecycle cache with no org data
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

	return r.q.BatchUpdateSPComponentEOL(ctx, db.BatchUpdateSPComponentEOLParams{
		Ids:      ids,
		Statuses: statuses,
		Dates:    dates,
	})
}

// ListFindingsCursor returns findings using keyset pagination.
// Fetch limit+1 rows so callers can detect HasMore; the caller strips the extra row.
func (r *Repository) ListFindingsCursor(ctx context.Context, orgID string, filter FindingFilter, cursorID string, cursorTS time.Time, limit int) ([]Finding, error) {
	const baseQuery = `
		SELECT id, org_id, asset_id, scan_id, cve_id,
		       title, description, severity,
		       cvss_score, epss_score, epss_percentile, risk_score,
		       status, scanner, raw_id, sources, template_id,
		       assigned_to, justification,
		       reopen_count, occurrence_count,
		       last_seen_at, sla_due_at, created_at, updated_at
		FROM vb_findings
		WHERE org_id = $1`

	args := []any{orgID}
	q := baseQuery
	n := 2

	if filter.Severity != "" {
		args = append(args, filter.Severity)
		q += fmt.Sprintf(" AND severity = $%d", n)
		n++
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		q += fmt.Sprintf(" AND status = $%d", n)
		n++
	}
	if !cursorTS.IsZero() && cursorID != "" {
		args = append(args, cursorTS, cursorID)
		q += fmt.Sprintf(" AND (created_at < $%d OR (created_at = $%d AND id::text < $%d))", n, n, n+1)
		n += 2
	}
	args = append(args, int32(limit+1))
	q += fmt.Sprintf(" ORDER BY created_at DESC, id DESC LIMIT $%d", n)

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list findings cursor: %w", err)
	}
	defer rows.Close()

	var out []Finding
	for rows.Next() {
		var f vbFindingRow
		if err := rows.Scan(
			&f.ID, &f.OrgID, &f.AssetID, &f.ScanID, &f.CVEID,
			&f.Title, &f.Description, &f.Severity,
			&f.CVSSScore, &f.EPSSScore, &f.EPSSPercentile, &f.RiskScore,
			&f.Status, &f.Scanner, &f.RawID, &f.Sources, &f.TemplateID,
			&f.AssignedTo, &f.Justification,
			&f.ReopenCount, &f.OccurrenceCount,
			&f.LastSeenAt, &f.SLADueAt, &f.CreatedAt, &f.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan finding cursor row: %w", err)
		}
		out = append(out, findingFromRow(f))
	}
	return out, rows.Err()
}

// vbFindingRow is a scan target for raw vb_findings queries.
type vbFindingRow struct {
	ID              pgtype.UUID
	OrgID           string
	AssetID         pgtype.UUID
	ScanID          pgtype.UUID
	CVEID           pgtype.Text
	Title           string
	Description     pgtype.Text
	Severity        string
	CVSSScore       pgtype.Float8
	EPSSScore       pgtype.Float8
	EPSSPercentile  pgtype.Float8
	RiskScore       pgtype.Float8
	Status          string
	Scanner         string
	RawID           pgtype.Text
	Sources         []string
	TemplateID      pgtype.Text
	AssignedTo      pgtype.UUID
	Justification   pgtype.Text
	ReopenCount     int32
	OccurrenceCount int32
	LastSeenAt      pgtype.Timestamptz
	SLADueAt        pgtype.Timestamptz
	CreatedAt       pgtype.Timestamptz
	UpdatedAt       pgtype.Timestamptz
}

func findingFromRow(r vbFindingRow) Finding {
	f := Finding{
		ID:              r.ID.String(),
		OrgID:           r.OrgID,
		AssetID:         r.AssetID.String(),
		Severity:        r.Severity,
		Status:          r.Status,
		Scanner:         r.Scanner,
		Title:           r.Title,
		ReopenCount:     int(r.ReopenCount),
		OccurrenceCount: int(r.OccurrenceCount),
		CreatedAt:       spTsToTime(r.CreatedAt),
		UpdatedAt:       spTsToTime(r.UpdatedAt),
		LastSeenAt:      spTsToTime(r.LastSeenAt),
	}
	if r.Description.Valid {
		f.Description = r.Description.String
	}
	if r.CVEID.Valid {
		s := r.CVEID.String
		f.CVEID = &s
	}
	if r.ScanID.Valid {
		s := r.ScanID.String()
		f.ScanID = &s
	}
	if r.CVSSScore.Valid {
		v := r.CVSSScore.Float64
		f.CVSSScore = &v
	}
	if r.EPSSScore.Valid {
		v := r.EPSSScore.Float64
		f.EPSSScore = &v
	}
	if r.EPSSPercentile.Valid {
		v := r.EPSSPercentile.Float64
		f.EPSSPercentile = &v
	}
	if r.RiskScore.Valid {
		v := r.RiskScore.Float64
		f.RiskScore = &v
	}
	if r.RawID.Valid {
		f.RawID = r.RawID.String
	}
	if r.Sources != nil {
		f.Sources = r.Sources
	}
	if r.TemplateID.Valid {
		f.TemplateID = r.TemplateID.String
	}
	if r.AssignedTo.Valid {
		s := r.AssignedTo.String()
		f.AssignedTo = &s
	}
	if r.Justification.Valid {
		f.Justification = r.Justification.String
	}
	if r.SLADueAt.Valid {
		t := r.SLADueAt.Time
		f.SLADueAt = &t
	}
	return f
}
