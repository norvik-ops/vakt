package vaktcomply

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func scanISMSScope(row pgx.Row) (ISMSScope, error) {
	var s ISMSScope
	var createdAt, updatedAt pgtype.Timestamptz
	var approvedAt pgtype.Timestamptz
	var approvedBy pgtype.Text
	var exclusionsRaw []byte

	err := row.Scan(
		&s.ID,
		&s.OrgID,
		&s.Version,
		&s.Status,
		&s.ScopeDefinition,
		&exclusionsRaw,
		&s.OutsourcingDependencies,
		&s.ChangeNote,
		&approvedBy,
		&approvedAt,
		&s.CreatedBy,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return ISMSScope{}, err
	}

	if exclusionsRaw != nil {
		s.Exclusions = json.RawMessage(exclusionsRaw)
	} else {
		s.Exclusions = json.RawMessage("[]")
	}
	if approvedBy.Valid {
		v := approvedBy.String
		s.ApprovedBy = &v
	}
	s.ApprovedAt = ckTsToTimePtr(approvedAt)
	s.CreatedAt = ckTsToTime(createdAt)
	s.UpdatedAt = ckTsToTime(updatedAt)
	return s, nil
}

const ismsScopeSelectCols = `id, org_id, version, status, scope_definition, exclusions,
	outsourcing_dependencies, change_note, approved_by, approved_at, created_by, created_at, updated_at`

// CreateOrVersionISMSScope inserts a new scope document. If one already exists for
// the org, the new record gets version = max(existing) + 1.
func (r *Repository) CreateOrVersionISMSScope(ctx context.Context, orgID, userID string, in CreateISMSScopeInput) (ISMSScope, error) {
	exclusions := in.Exclusions
	if exclusions == nil {
		exclusions = json.RawMessage("[]")
	}

	row := r.db.QueryRow(ctx, `
		INSERT INTO ck_isms_scope (
			org_id, version, status, scope_definition, exclusions,
			outsourcing_dependencies, change_note, created_by
		)
		VALUES (
			$1::uuid,
			COALESCE((SELECT MAX(version) FROM ck_isms_scope WHERE org_id = $1::uuid), 0) + 1,
			'draft',
			$2, $3::jsonb, $4, $5,
			$6::uuid
		)
		RETURNING `+ismsScopeSelectCols,
		orgID,
		in.ScopeDefinition,
		exclusions,
		in.OutsourcingDependencies,
		in.ChangeNote,
		userID,
	)
	scope, err := scanISMSScope(row)
	if err != nil {
		return ISMSScope{}, fmt.Errorf("create isms scope: %w", err)
	}
	return scope, nil
}

// GetCurrentISMSScope returns the latest version for the given org.
func (r *Repository) GetCurrentISMSScope(ctx context.Context, orgID string) (ISMSScope, error) {
	row := r.db.QueryRow(ctx, `
		SELECT `+ismsScopeSelectCols+`
		FROM ck_isms_scope
		WHERE org_id = $1::uuid
		ORDER BY version DESC
		LIMIT 1`,
		orgID,
	)
	scope, err := scanISMSScope(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return ISMSScope{}, fmt.Errorf("isms scope not found: %w", ErrNotFound)
		}
		return ISMSScope{}, fmt.Errorf("get current isms scope: %w", err)
	}
	return scope, nil
}

// ListISMSScopeVersions returns all versions for the given org, newest first.
func (r *Repository) ListISMSScopeVersions(ctx context.Context, orgID string) ([]ISMSScope, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+ismsScopeSelectCols+`
		FROM ck_isms_scope
		WHERE org_id = $1::uuid
		ORDER BY version DESC`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list isms scope versions: %w", err)
	}
	defer rows.Close()

	var out []ISMSScope
	for rows.Next() {
		scope, err := scanISMSScope(rows)
		if err != nil {
			return nil, fmt.Errorf("scan isms scope: %w", err)
		}
		out = append(out, scope)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("list isms scope versions rows: %w", rows.Err())
	}
	return out, nil
}

// ApproveISMSScope sets status='approved' and records the approver.
func (r *Repository) ApproveISMSScope(ctx context.Context, orgID, id, approverID string) (ISMSScope, error) {
	row := r.db.QueryRow(ctx, `
		UPDATE ck_isms_scope
		SET status = 'approved', approved_by = $1::uuid, approved_at = NOW(), updated_at = NOW()
		WHERE id = $2::uuid AND org_id = $3::uuid
		RETURNING `+ismsScopeSelectCols,
		approverID, id, orgID,
	)
	scope, err := scanISMSScope(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return ISMSScope{}, fmt.Errorf("isms scope not found: %w", ErrNotFound)
		}
		return ISMSScope{}, fmt.Errorf("approve isms scope: %w", err)
	}
	return scope, nil
}
