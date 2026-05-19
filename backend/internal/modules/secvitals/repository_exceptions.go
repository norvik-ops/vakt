package secvitals

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ListControlExceptions returns all exceptions for a given control within an organisation.
func (r *Repository) ListControlExceptions(ctx context.Context, orgID, controlID string) ([]ControlException, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text, control_id::text,
		       title, reason, risk_accepted,
		       COALESCE(approved_by, ''), expires_at, status,
		       COALESCE(created_by, ''), created_at, updated_at
		FROM ck_control_exceptions
		WHERE org_id = $1::uuid AND control_id = $2::uuid
		ORDER BY created_at DESC`,
		orgID, controlID,
	)
	if err != nil {
		return nil, fmt.Errorf("list control exceptions: %w", err)
	}
	defer rows.Close()

	var exceptions []ControlException
	for rows.Next() {
		var e ControlException
		if err := rows.Scan(
			&e.ID, &e.OrgID, &e.ControlID,
			&e.Title, &e.Reason, &e.RiskAccepted,
			&e.ApprovedBy, &e.ExpiresAt, &e.Status,
			&e.CreatedBy, &e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan control exception: %w", err)
		}
		exceptions = append(exceptions, e)
	}
	return exceptions, rows.Err()
}

// ListAllControlExceptions returns all exceptions for an organisation across all controls.
func (r *Repository) ListAllControlExceptions(ctx context.Context, orgID string) ([]ControlException, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text, control_id::text,
		       title, reason, risk_accepted,
		       COALESCE(approved_by, ''), expires_at, status,
		       COALESCE(created_by, ''), created_at, updated_at
		FROM ck_control_exceptions
		WHERE org_id = $1::uuid
		ORDER BY created_at DESC`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list all control exceptions: %w", err)
	}
	defer rows.Close()

	var exceptions []ControlException
	for rows.Next() {
		var e ControlException
		if err := rows.Scan(
			&e.ID, &e.OrgID, &e.ControlID,
			&e.Title, &e.Reason, &e.RiskAccepted,
			&e.ApprovedBy, &e.ExpiresAt, &e.Status,
			&e.CreatedBy, &e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan control exception: %w", err)
		}
		exceptions = append(exceptions, e)
	}
	return exceptions, rows.Err()
}

// CreateControlException inserts a new exception record.
func (r *Repository) CreateControlException(ctx context.Context, orgID, controlID string, in CreateControlExceptionInput, createdBy string) (*ControlException, error) {
	var e ControlException
	err := r.db.QueryRow(ctx, `
		INSERT INTO ck_control_exceptions
		       (org_id, control_id, title, reason, risk_accepted, approved_by, expires_at, status, created_by)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, NULLIF($6,''), $7, 'active', NULLIF($8,''))
		RETURNING id::text, org_id::text, control_id::text,
		          title, reason, risk_accepted,
		          COALESCE(approved_by, ''), expires_at, status,
		          COALESCE(created_by, ''), created_at, updated_at`,
		orgID, controlID, in.Title, in.Reason, in.RiskAccepted,
		in.ApprovedBy, in.ExpiresAt, createdBy,
	).Scan(
		&e.ID, &e.OrgID, &e.ControlID,
		&e.Title, &e.Reason, &e.RiskAccepted,
		&e.ApprovedBy, &e.ExpiresAt, &e.Status,
		&e.CreatedBy, &e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create control exception: %w", err)
	}
	return &e, nil
}

// UpdateControlException updates an existing exception record.
func (r *Repository) UpdateControlException(ctx context.Context, orgID, id string, in UpdateControlExceptionInput) (*ControlException, error) {
	var e ControlException
	err := r.db.QueryRow(ctx, `
		UPDATE ck_control_exceptions
		SET title        = COALESCE(NULLIF($3,''), title),
		    reason       = COALESCE(NULLIF($4,''), reason),
		    risk_accepted = COALESCE(NULLIF($5,''), risk_accepted),
		    approved_by  = CASE WHEN $6::text IS NOT NULL THEN NULLIF($6,'') ELSE approved_by END,
		    expires_at   = COALESCE($7, expires_at),
		    status       = COALESCE(NULLIF($8,''), status),
		    updated_at   = now()
		WHERE id = $1::uuid AND org_id = $2::uuid
		RETURNING id::text, org_id::text, control_id::text,
		          title, reason, risk_accepted,
		          COALESCE(approved_by, ''), expires_at, status,
		          COALESCE(created_by, ''), created_at, updated_at`,
		id, orgID, in.Title, in.Reason, in.RiskAccepted,
		in.ApprovedBy, in.ExpiresAt, in.Status,
	).Scan(
		&e.ID, &e.OrgID, &e.ControlID,
		&e.Title, &e.Reason, &e.RiskAccepted,
		&e.ApprovedBy, &e.ExpiresAt, &e.Status,
		&e.CreatedBy, &e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update control exception: %w", err)
	}
	return &e, nil
}

// DeleteControlException removes an exception record.
func (r *Repository) DeleteControlException(ctx context.Context, orgID, id string) error {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM ck_control_exceptions
		WHERE id = $1::uuid AND org_id = $2::uuid`,
		id, orgID,
	)
	if err != nil {
		return fmt.Errorf("delete control exception: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
