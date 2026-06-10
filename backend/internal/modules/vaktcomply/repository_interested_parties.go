// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// ListInterestedParties returns all entries for the org.
func (r *Repository) ListInterestedParties(ctx context.Context, orgID string) ([]InterestedParty, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, org_id, name, category,
		       COALESCE(requirements,''), COALESCE(concerns,''),
		       to_char(review_date,'YYYY-MM-DD'), is_system_default,
		       created_at, updated_at
		FROM ck_interested_parties WHERE org_id = $1 ORDER BY name`,
		orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	today := time.Now().UTC().Format("2006-01-02")
	var parties []InterestedParty
	for rows.Next() {
		var p InterestedParty
		var reviewDate pgtype.Text
		var createdAt, updatedAt time.Time
		if err := rows.Scan(
			&p.ID, &p.OrgID, &p.Name, &p.Category,
			&p.Requirements, &p.Concerns, &reviewDate,
			&p.IsSystemDefault, &createdAt, &updatedAt,
		); err != nil {
			return nil, err
		}
		p.CreatedAt = createdAt.Format(time.RFC3339)
		p.UpdatedAt = updatedAt.Format(time.RFC3339)
		if reviewDate.Valid {
			rd := reviewDate.String
			p.ReviewDate = &rd
			p.ReviewOverdue = rd < today
		}
		parties = append(parties, p)
	}
	return parties, rows.Err()
}

// CreateInterestedParty inserts a new entry.
func (r *Repository) CreateInterestedParty(ctx context.Context, orgID string, in CreateInterestedPartyInput, isDefault bool) (*InterestedParty, error) {
	var reviewDate pgtype.Date
	if in.ReviewDate != nil && *in.ReviewDate != "" {
		if err := reviewDate.Scan(*in.ReviewDate); err != nil {
			return nil, err
		}
	}
	var p InterestedParty
	var reviewDateOut pgtype.Text
	var createdAt, updatedAt time.Time
	err := r.db.QueryRow(ctx, `
		INSERT INTO ck_interested_parties (org_id, name, category, requirements, concerns, review_date, is_system_default)
		VALUES ($1, $2, $3, NULLIF($4,''), NULLIF($5,''), NULLIF($6::date, NULL), $7)
		RETURNING id, org_id, name, category,
		          COALESCE(requirements,''), COALESCE(concerns,''),
		          to_char(review_date,'YYYY-MM-DD'), is_system_default, created_at, updated_at`,
		orgID, in.Name, in.Category, in.Requirements, in.Concerns, reviewDate, isDefault,
	).Scan(&p.ID, &p.OrgID, &p.Name, &p.Category, &p.Requirements, &p.Concerns,
		&reviewDateOut, &p.IsSystemDefault, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	p.CreatedAt = createdAt.Format(time.RFC3339)
	p.UpdatedAt = updatedAt.Format(time.RFC3339)
	if reviewDateOut.Valid {
		rd := reviewDateOut.String
		p.ReviewDate = &rd
	}
	return &p, nil
}

// UpdateInterestedParty modifies an existing entry.
func (r *Repository) UpdateInterestedParty(ctx context.Context, orgID, id string, in CreateInterestedPartyInput) (*InterestedParty, error) {
	var reviewDate pgtype.Date
	if in.ReviewDate != nil && *in.ReviewDate != "" {
		if err := reviewDate.Scan(*in.ReviewDate); err != nil {
			return nil, err
		}
	}
	var p InterestedParty
	var reviewDateOut pgtype.Text
	var createdAt, updatedAt time.Time
	err := r.db.QueryRow(ctx, `
		UPDATE ck_interested_parties SET
			name         = $1,
			category     = $2,
			requirements = NULLIF($3,''),
			concerns     = NULLIF($4,''),
			review_date  = NULLIF($5::date, NULL),
			updated_at   = NOW()
		WHERE org_id = $6 AND id = $7
		RETURNING id, org_id, name, category,
		          COALESCE(requirements,''), COALESCE(concerns,''),
		          to_char(review_date,'YYYY-MM-DD'), is_system_default, created_at, updated_at`,
		in.Name, in.Category, in.Requirements, in.Concerns, reviewDate, orgID, id,
	).Scan(&p.ID, &p.OrgID, &p.Name, &p.Category, &p.Requirements, &p.Concerns,
		&reviewDateOut, &p.IsSystemDefault, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	p.CreatedAt = createdAt.Format(time.RFC3339)
	p.UpdatedAt = updatedAt.Format(time.RFC3339)
	if reviewDateOut.Valid {
		rd := reviewDateOut.String
		p.ReviewDate = &rd
	}
	return &p, nil
}

// DeleteInterestedParty removes an entry by ID.
func (r *Repository) DeleteInterestedParty(ctx context.Context, orgID, id string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM ck_interested_parties WHERE org_id = $1 AND id = $2`, orgID, id)
	return err
}

// CountInterestedParties returns the total count for the org.
func (r *Repository) CountInterestedParties(ctx context.Context, orgID string) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM ck_interested_parties WHERE org_id = $1`, orgID).Scan(&count)
	return count, err
}

// CheckClause42Fulfilled returns true if ≥3 entries have requirements set.
func (r *Repository) CheckClause42Fulfilled(ctx context.Context, orgID string) (bool, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM ck_interested_parties
		WHERE org_id = $1 AND requirements IS NOT NULL AND requirements != ''`, orgID,
	).Scan(&count)
	return count >= 3, err
}
