// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// bsiModelingJoinCols is the column list for queries that JOIN vb_assets and ck_controls.
const bsiModelingJoinCols = `
	bm.id::text, bm.org_id::text, bm.asset_id::text, bm.control_id::text,
	bm.priority, bm.justification_for_exclusion, bm.check_status,
	bm.interview_notes, bm.site_visit_notes,
	COALESCE(vb.name, '') AS asset_name,
	COALESCE(c.title, '') AS control_title,
	COALESCE(c.framework_id::text, '') AS framework_id,
	bm.created_by::text, bm.created_at, bm.updated_at`

// scanBSIModelingEntry scans one row from a JOIN query into a BSIModelingEntry.
func scanBSIModelingEntry(row pgx.Row) (BSIModelingEntry, error) {
	var e BSIModelingEntry
	var checkStatus pgtype.Text
	var createdAt, updatedAt pgtype.Timestamptz

	err := row.Scan(
		&e.ID,
		&e.OrgID,
		&e.AssetID,
		&e.ControlID,
		&e.Priority,
		&e.JustificationForExclusion,
		&checkStatus,
		&e.InterviewNotes,
		&e.SiteVisitNotes,
		&e.AssetName,
		&e.ControlTitle,
		&e.FrameworkID,
		&e.CreatedBy,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return BSIModelingEntry{}, err
	}

	if checkStatus.Valid {
		v := checkStatus.String
		e.CheckStatus = &v
	}
	e.CreatedAt = ckTsToTime(createdAt)
	e.UpdatedAt = ckTsToTime(updatedAt)
	return e, nil
}

// fetchBSIModelingByID performs the JOIN SELECT for a single row identified by id and orgID.
func (r *Repository) fetchBSIModelingByID(ctx context.Context, orgID, id string) (BSIModelingEntry, error) {
	row := r.db.QueryRow(ctx, `
		SELECT `+bsiModelingJoinCols+`
		FROM ck_bsi_modeling bm
		LEFT JOIN vb_assets vb ON vb.id = bm.asset_id
		LEFT JOIN ck_controls c ON c.id = bm.control_id
		WHERE bm.id = $1::uuid AND bm.org_id = $2::uuid`,
		id, orgID,
	)
	return scanBSIModelingEntry(row)
}

// CreateBSIModeling inserts a new BSI modeling entry.
func (r *Repository) CreateBSIModeling(ctx context.Context, orgID, userID string, in CreateBSIModelingInput) (BSIModelingEntry, error) {
	var checkStatus pgtype.Text
	if in.CheckStatus != nil && *in.CheckStatus != "" {
		checkStatus = pgtype.Text{String: *in.CheckStatus, Valid: true}
	}

	var insertedID string
	err := r.db.QueryRow(ctx, `
		INSERT INTO ck_bsi_modeling (
			org_id, asset_id, control_id, priority,
			justification_for_exclusion, check_status,
			interview_notes, site_visit_notes, created_by
		) VALUES (
			$1::uuid, $2::uuid, $3::uuid, $4,
			$5, $6,
			$7, $8, $9::uuid
		)
		RETURNING id::text`,
		orgID, in.AssetID, in.ControlID, in.Priority,
		in.JustificationForExclusion, checkStatus,
		in.InterviewNotes, in.SiteVisitNotes, userID,
	).Scan(&insertedID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return BSIModelingEntry{}, fmt.Errorf("mapping already exists for this asset and control")
		}
		return BSIModelingEntry{}, fmt.Errorf("create bsi modeling: %w", err)
	}

	e, err := r.fetchBSIModelingByID(ctx, orgID, insertedID)
	if err != nil {
		return BSIModelingEntry{}, fmt.Errorf("fetch created bsi modeling: %w", err)
	}
	return e, nil
}

// UpdateBSIModeling updates an existing BSI modeling entry and returns the updated row.
func (r *Repository) UpdateBSIModeling(ctx context.Context, orgID, id string, in UpdateBSIModelingInput) (BSIModelingEntry, error) {
	var checkStatus pgtype.Text
	if in.CheckStatus != nil && *in.CheckStatus != "" {
		checkStatus = pgtype.Text{String: *in.CheckStatus, Valid: true}
	}

	ct, err := r.db.Exec(ctx, `
		UPDATE ck_bsi_modeling SET
			priority                    = $3,
			justification_for_exclusion = $4,
			check_status                = $5,
			interview_notes             = $6,
			site_visit_notes            = $7,
			updated_at                  = NOW()
		WHERE id = $1::uuid AND org_id = $2::uuid`,
		id, orgID,
		in.Priority, in.JustificationForExclusion, checkStatus,
		in.InterviewNotes, in.SiteVisitNotes,
	)
	if err != nil {
		return BSIModelingEntry{}, fmt.Errorf("update bsi modeling: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return BSIModelingEntry{}, fmt.Errorf("bsi modeling entry not found")
	}

	e, err := r.fetchBSIModelingByID(ctx, orgID, id)
	if err != nil {
		return BSIModelingEntry{}, fmt.Errorf("fetch updated bsi modeling: %w", err)
	}
	return e, nil
}

// DeleteBSIModeling removes a BSI modeling entry.
func (r *Repository) DeleteBSIModeling(ctx context.Context, orgID, id string) error {
	ct, err := r.db.Exec(ctx, `
		DELETE FROM ck_bsi_modeling
		WHERE id = $1::uuid AND org_id = $2::uuid`,
		id, orgID,
	)
	if err != nil {
		return fmt.Errorf("delete bsi modeling: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("bsi modeling entry not found")
	}
	return nil
}

// listBSIModelingRows is the shared helper for scanning multi-row results.
func (r *Repository) listBSIModelingRows(ctx context.Context, query string, args ...any) ([]BSIModelingEntry, error) {
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []BSIModelingEntry
	for rows.Next() {
		e, err := scanBSIModelingEntry(rows)
		if err != nil {
			return nil, fmt.Errorf("scan bsi modeling row: %w", err)
		}
		out = append(out, e)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("bsi modeling rows: %w", rows.Err())
	}
	return out, nil
}

// ListBSIModelingByAsset returns all BSI modeling entries for a specific asset.
func (r *Repository) ListBSIModelingByAsset(ctx context.Context, orgID, assetID string) ([]BSIModelingEntry, error) {
	entries, err := r.listBSIModelingRows(ctx, `
		SELECT `+bsiModelingJoinCols+`
		FROM ck_bsi_modeling bm
		LEFT JOIN vb_assets vb ON vb.id = bm.asset_id
		LEFT JOIN ck_controls c ON c.id = bm.control_id
		WHERE bm.org_id = $1::uuid AND bm.asset_id = $2::uuid
		ORDER BY vb.name, c.title`,
		orgID, assetID,
	)
	if err != nil {
		return nil, fmt.Errorf("list bsi modeling by asset: %w", err)
	}
	return entries, nil
}

// ListBSIModelingByControl returns all BSI modeling entries for a specific control.
func (r *Repository) ListBSIModelingByControl(ctx context.Context, orgID, controlID string) ([]BSIModelingEntry, error) {
	entries, err := r.listBSIModelingRows(ctx, `
		SELECT `+bsiModelingJoinCols+`
		FROM ck_bsi_modeling bm
		LEFT JOIN vb_assets vb ON vb.id = bm.asset_id
		LEFT JOIN ck_controls c ON c.id = bm.control_id
		WHERE bm.org_id = $1::uuid AND bm.control_id = $2::uuid
		ORDER BY vb.name, c.title`,
		orgID, controlID,
	)
	if err != nil {
		return nil, fmt.Errorf("list bsi modeling by control: %w", err)
	}
	return entries, nil
}

// GetBSIModelingMatrix returns all BSI modeling entries for an organisation.
func (r *Repository) GetBSIModelingMatrix(ctx context.Context, orgID string) ([]BSIModelingEntry, error) {
	entries, err := r.listBSIModelingRows(ctx, `
		SELECT `+bsiModelingJoinCols+`
		FROM ck_bsi_modeling bm
		LEFT JOIN vb_assets vb ON vb.id = bm.asset_id
		LEFT JOIN ck_controls c ON c.id = bm.control_id
		WHERE bm.org_id = $1::uuid
		ORDER BY vb.name, c.title`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("get bsi modeling matrix: %w", err)
	}
	return entries, nil
}

// GetBSIModelingStats returns aggregate check-status counts for an organisation's matrix.
func (r *Repository) GetBSIModelingStats(ctx context.Context, orgID string) (BSIModelingStats, error) {
	var s BSIModelingStats
	err := r.db.QueryRow(ctx, `
		SELECT
			COUNT(*)                                                    AS total,
			COUNT(CASE WHEN check_status = 'yes'            THEN 1 END) AS count_yes,
			COUNT(CASE WHEN check_status = 'partial'        THEN 1 END) AS count_partial,
			COUNT(CASE WHEN check_status = 'no'             THEN 1 END) AS count_no,
			COUNT(CASE WHEN check_status = 'not_applicable' THEN 1 END) AS count_na,
			COUNT(CASE WHEN check_status IS NULL             THEN 1 END) AS count_pending
		FROM ck_bsi_modeling
		WHERE org_id = $1::uuid`,
		orgID,
	).Scan(&s.Total, &s.CountYes, &s.CountPartial, &s.CountNo, &s.CountNA, &s.CountPending)
	if err != nil {
		return BSIModelingStats{}, fmt.Errorf("get bsi modeling stats: %w", err)
	}
	return s, nil
}
