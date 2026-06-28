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

const pentestSelectCols = `id, org_id, title, scope, pentest_date::text, tester_type,
	tester_name, methodology, findings_critical, findings_high, findings_medium, findings_low,
	status, retest_date::text, notes, created_by, created_at, updated_at`

// scanPentest reads one row from ck_pentests into a Pentest domain model.
func scanPentest(row pgx.Row) (Pentest, error) {
	var p Pentest
	var methodology pgtype.Text
	var retestDate pgtype.Text
	var createdAt, updatedAt pgtype.Timestamptz

	err := row.Scan(
		&p.ID,
		&p.OrgID,
		&p.Title,
		&p.Scope,
		&p.PentestDate,
		&p.TesterType,
		&p.TesterName,
		&methodology,
		&p.FindingsCritical,
		&p.FindingsHigh,
		&p.FindingsMedium,
		&p.FindingsLow,
		&p.Status,
		&retestDate,
		&p.Notes,
		&p.CreatedBy,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return Pentest{}, err
	}

	if methodology.Valid {
		v := methodology.String
		p.Methodology = &v
	}
	if retestDate.Valid {
		v := retestDate.String
		p.RetestDate = &v
	}
	p.CreatedAt = ckTsToTime(createdAt)
	p.UpdatedAt = ckTsToTime(updatedAt)
	return p, nil
}

// CreatePentest inserts a new pentest record for an organisation.
func (r *Repository) CreatePentest(ctx context.Context, orgID, userID string, in CreatePentestInput) (Pentest, error) {
	var methodology pgtype.Text
	if in.Methodology != nil && *in.Methodology != "" {
		methodology = pgtype.Text{String: *in.Methodology, Valid: true}
	}

	row := r.db.QueryRow(ctx, `
		INSERT INTO ck_pentests (
			org_id, title, scope, pentest_date, tester_type, tester_name,
			methodology, findings_critical, findings_high, findings_medium, findings_low,
			notes, created_by
		) VALUES (
			$1::uuid, $2, $3, $4::date, $5, $6,
			$7, $8, $9, $10, $11,
			$12, $13::uuid
		)
		RETURNING `+pentestSelectCols,
		orgID, in.Title, in.Scope, in.PentestDate, in.TesterType, in.TesterName,
		methodology, in.FindingsCritical, in.FindingsHigh, in.FindingsMedium, in.FindingsLow,
		in.Notes, userID,
	)
	p, err := scanPentest(row)
	if err != nil {
		return Pentest{}, fmt.Errorf("create pentest: %w", err)
	}
	return p, nil
}

// GetPentest returns a single pentest by ID within an organisation.
func (r *Repository) GetPentest(ctx context.Context, orgID, id string) (Pentest, error) {
	row := r.db.QueryRow(ctx, `
		SELECT `+pentestSelectCols+`
		FROM ck_pentests
		WHERE id = $1::uuid AND org_id = $2::uuid`,
		id, orgID,
	)
	p, err := scanPentest(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return Pentest{}, fmt.Errorf("pentest not found")
		}
		return Pentest{}, fmt.Errorf("get pentest: %w", err)
	}
	return p, nil
}

// ListPentests returns all pentest records for an organisation, optionally filtered by tester_type.
func (r *Repository) ListPentests(ctx context.Context, orgID string, testerType *string) ([]Pentest, error) {
	var rows pgx.Rows
	var err error

	if testerType != nil && *testerType != "" {
		rows, err = r.db.Query(ctx, `
			SELECT `+pentestSelectCols+`
			FROM ck_pentests
			WHERE org_id = $1::uuid AND tester_type = $2
			ORDER BY pentest_date DESC, created_at DESC`,
			orgID, *testerType,
		)
	} else {
		rows, err = r.db.Query(ctx, `
			SELECT `+pentestSelectCols+`
			FROM ck_pentests
			WHERE org_id = $1::uuid
			ORDER BY pentest_date DESC, created_at DESC`,
			orgID,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("list pentests: %w", err)
	}
	defer rows.Close()

	var out []Pentest
	for rows.Next() {
		p, err := scanPentest(rows)
		if err != nil {
			return nil, fmt.Errorf("scan pentest: %w", err)
		}
		out = append(out, p)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("list pentests rows: %w", rows.Err())
	}
	return out, nil
}

// UpdatePentest updates an existing pentest record.
func (r *Repository) UpdatePentest(ctx context.Context, orgID, id string, in UpdatePentestInput) (Pentest, error) {
	var methodology pgtype.Text
	if in.Methodology != nil && *in.Methodology != "" {
		methodology = pgtype.Text{String: *in.Methodology, Valid: true}
	}

	var retestDate pgtype.Text
	if in.RetestDate != nil && *in.RetestDate != "" {
		retestDate = pgtype.Text{String: *in.RetestDate, Valid: true}
	}

	row := r.db.QueryRow(ctx, `
		UPDATE ck_pentests SET
			title              = $3,
			scope              = $4,
			tester_type        = $5,
			tester_name        = $6,
			methodology        = $7,
			findings_critical  = $8,
			findings_high      = $9,
			findings_medium    = $10,
			findings_low       = $11,
			status             = $12,
			retest_date        = CASE WHEN $13::text IS NULL THEN NULL ELSE $13::text::date END,
			notes              = $14,
			updated_at         = NOW()
		WHERE id = $1::uuid AND org_id = $2::uuid
		RETURNING `+pentestSelectCols,
		id, orgID,
		in.Title, in.Scope, in.TesterType, in.TesterName,
		methodology,
		in.FindingsCritical, in.FindingsHigh, in.FindingsMedium, in.FindingsLow,
		in.Status,
		retestDate,
		in.Notes,
	)
	p, err := scanPentest(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return Pentest{}, fmt.Errorf("pentest not found")
		}
		return Pentest{}, fmt.Errorf("update pentest: %w", err)
	}
	return p, nil
}

// DeletePentest removes a pentest record.
func (r *Repository) DeletePentest(ctx context.Context, orgID, id string) error {
	ct, err := r.db.Exec(ctx, `
		DELETE FROM ck_pentests
		WHERE id = $1::uuid AND org_id = $2::uuid`,
		id, orgID,
	)
	if err != nil {
		return fmt.Errorf("delete pentest: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("pentest not found")
	}
	return nil
}

// GetLastPentest returns the most recent pentest for an organisation, or nil if none exist.
func (r *Repository) GetLastPentest(ctx context.Context, orgID string) (*Pentest, error) {
	row := r.db.QueryRow(ctx, `
		SELECT `+pentestSelectCols+`
		FROM ck_pentests
		WHERE org_id = $1::uuid
		ORDER BY pentest_date DESC, created_at DESC
		LIMIT 1`,
		orgID,
	)
	p, err := scanPentest(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get last pentest: %w", err)
	}
	return &p, nil
}
