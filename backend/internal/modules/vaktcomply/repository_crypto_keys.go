// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func scanCryptoKey(rows pgx.Rows) (CryptoKey, error) {
	var k CryptoKey
	var keyLength pgtype.Int4
	var location, notes pgtype.Text
	var rotInterv pgtype.Int4
	var lastRot, nextDue, expiry pgtype.Date
	var updatedAt pgtype.Timestamptz
	var createdAt pgtype.Timestamptz

	if err := rows.Scan(
		&k.ID, &k.OrgID, &k.Name, &k.KeyType, &k.Algorithm,
		&keyLength, &k.Purpose, &location,
		&rotInterv, &lastRot, &nextDue, &expiry,
		&k.IsWeakAlgorithm, &notes,
		&createdAt, &updatedAt,
	); err != nil {
		return k, err
	}
	if keyLength.Valid {
		v := int(keyLength.Int32)
		k.KeyLength = &v
	}
	if location.Valid {
		k.Location = location.String
	}
	if notes.Valid {
		k.Notes = notes.String
	}
	if rotInterv.Valid {
		v := int(rotInterv.Int32)
		k.RotationIntervalDays = &v
	}
	if lastRot.Valid {
		s := lastRot.Time.Format("2006-01-02")
		k.LastRotationDate = &s
	}
	if nextDue.Valid {
		s := nextDue.Time.Format("2006-01-02")
		k.NextRotationDue = &s
	}
	if expiry.Valid {
		s := expiry.Time.Format("2006-01-02")
		k.ExpiryDate = &s
	}
	k.CreatedAt = ckTsToTime(createdAt)
	k.UpdatedAt = ckTsToTime(updatedAt)
	return k, nil
}

// ListCryptoKeys returns all crypto keys for the org.
func (r *Repository) ListCryptoKeys(ctx context.Context, orgID string) ([]CryptoKey, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id::text, org_id::text, name, key_type, algorithm,
		        key_length, purpose, location,
		        rotation_interval_days, last_rotation_date, next_rotation_due, expiry_date,
		        is_weak_algorithm, notes,
		        created_at, updated_at
		   FROM ck_crypto_keys
		  WHERE org_id = $1::uuid
		  ORDER BY name`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list crypto keys: %w", err)
	}
	defer rows.Close()

	var out []CryptoKey
	for rows.Next() {
		k, err := scanCryptoKey(rows)
		if err != nil {
			return nil, fmt.Errorf("scan crypto key: %w", err)
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// GetCryptoKey returns a single crypto key.
func (r *Repository) GetCryptoKey(ctx context.Context, orgID, keyID string) (*CryptoKey, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id::text, org_id::text, name, key_type, algorithm,
		        key_length, purpose, location,
		        rotation_interval_days, last_rotation_date, next_rotation_due, expiry_date,
		        is_weak_algorithm, notes,
		        created_at, updated_at
		   FROM ck_crypto_keys
		  WHERE id = $1::uuid AND org_id = $2::uuid`,
		keyID, orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("get crypto key: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, pgx.ErrNoRows
	}
	k, err := scanCryptoKey(rows)
	if err != nil {
		return nil, fmt.Errorf("scan crypto key: %w", err)
	}
	return &k, nil
}

// CreateCryptoKey inserts a new crypto key record.
func (r *Repository) CreateCryptoKey(ctx context.Context, orgID string, in CreateCryptoKeyInput, isWeak bool, nextDue *string) (*CryptoKey, error) {
	var id string
	var createdAt, updatedAt pgtype.Timestamptz

	err := r.db.QueryRow(ctx,
		`INSERT INTO ck_crypto_keys
		    (org_id, name, key_type, algorithm, key_length, purpose, location,
		     rotation_interval_days, last_rotation_date, next_rotation_due, expiry_date,
		     is_weak_algorithm, notes)
		 VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9::date, $10::date, $11::date, $12, $13)
		 RETURNING id::text, created_at, updated_at`,
		orgID, in.Name, in.KeyType, in.Algorithm, in.KeyLength, in.Purpose, in.Location,
		in.RotationIntervalDays, in.LastRotationDate, nextDue, in.ExpiryDate,
		isWeak, in.Notes,
	).Scan(&id, &createdAt, &updatedAt)
	if err != nil {
		return nil, fmt.Errorf("create crypto key: %w", err)
	}

	return &CryptoKey{
		ID:                   id,
		OrgID:                orgID,
		Name:                 in.Name,
		KeyType:              in.KeyType,
		Algorithm:            in.Algorithm,
		KeyLength:            in.KeyLength,
		Purpose:              in.Purpose,
		Location:             in.Location,
		RotationIntervalDays: in.RotationIntervalDays,
		LastRotationDate:     in.LastRotationDate,
		NextRotationDue:      nextDue,
		ExpiryDate:           in.ExpiryDate,
		IsWeakAlgorithm:      isWeak,
		Notes:                in.Notes,
		CreatedAt:            ckTsToTime(createdAt),
		UpdatedAt:            ckTsToTime(updatedAt),
	}, nil
}

// RecordKeyRotation records a rotation event and updates dates.
func (r *Repository) RecordKeyRotation(ctx context.Context, orgID, keyID, today string, nextDue *string) (*CryptoKey, error) {
	_, err := r.db.Exec(ctx,
		`UPDATE ck_crypto_keys
		    SET last_rotation_date = $3::date,
		        next_rotation_due  = $4::date,
		        updated_at         = NOW()
		  WHERE id = $1::uuid AND org_id = $2::uuid`,
		keyID, orgID, today, nextDue,
	)
	if err != nil {
		return nil, fmt.Errorf("record key rotation: %w", err)
	}
	return r.GetCryptoKey(ctx, orgID, keyID)
}

// DeleteCryptoKey removes a crypto key record.
func (r *Repository) DeleteCryptoKey(ctx context.Context, orgID, keyID string) error {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM ck_crypto_keys WHERE id = $1::uuid AND org_id = $2::uuid`,
		keyID, orgID,
	)
	if err != nil {
		return fmt.Errorf("delete crypto key: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// CountCryptoKeyRotationStats returns rotation counts for evidence auto-generation.
func (r *Repository) CountCryptoKeyRotationStats(ctx context.Context, orgID string) (total, overdue int, err error) {
	row := r.db.QueryRow(ctx,
		`SELECT
		    COUNT(*) AS total,
		    COUNT(*) FILTER (
		        WHERE rotation_interval_days IS NOT NULL
		          AND next_rotation_due IS NOT NULL
		          AND next_rotation_due < CURRENT_DATE
		    ) AS overdue
		   FROM ck_crypto_keys WHERE org_id = $1::uuid`,
		orgID,
	)
	if err := row.Scan(&total, &overdue); err != nil {
		return 0, 0, fmt.Errorf("count crypto key rotation stats: %w", err)
	}
	return total, overdue, nil
}

var _ = time.Now // keep import
