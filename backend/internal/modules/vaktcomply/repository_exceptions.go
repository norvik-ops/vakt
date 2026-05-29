package vaktcomply

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/matharnica/vakt/internal/db"
)

func exceptionFromCk(r db.CkControlExceptions) ControlException {
	return ControlException{
		ID:           r.ID,
		OrgID:        r.OrgID,
		ControlID:    r.ControlID,
		Title:        r.Title,
		Reason:       r.Reason,
		RiskAccepted: r.RiskAccepted,
		ApprovedBy:   r.ApprovedBy.String,
		ExpiresAt:    ckTsToTimePtr(r.ExpiresAt),
		Status:       r.Status,
		CreatedBy:    r.CreatedBy.String,
		CreatedAt:    ckTsToTime(r.CreatedAt),
		UpdatedAt:    ckTsToTime(r.UpdatedAt),
	}
}

// ListControlExceptions returns all exceptions for a given control within an organisation.
func (r *Repository) ListControlExceptions(ctx context.Context, orgID, controlID string) ([]ControlException, error) {
	rows, err := r.q.ListCKControlExceptions(ctx, db.ListCKControlExceptionsParams{OrgID: orgID, ControlID: controlID})
	if err != nil {
		return nil, fmt.Errorf("list control exceptions: %w", err)
	}
	out := make([]ControlException, 0, len(rows))
	for _, row := range rows {
		out = append(out, exceptionFromCk(row))
	}
	return out, nil
}

// ListAllControlExceptions returns all exceptions for an organisation across all controls.
func (r *Repository) ListAllControlExceptions(ctx context.Context, orgID string) ([]ControlException, error) {
	rows, err := r.q.ListAllCKControlExceptions(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list all control exceptions: %w", err)
	}
	out := make([]ControlException, 0, len(rows))
	for _, row := range rows {
		out = append(out, exceptionFromCk(row))
	}
	return out, nil
}

// CreateControlException inserts a new exception record.
func (r *Repository) CreateControlException(ctx context.Context, orgID, controlID string, in CreateControlExceptionInput, createdBy string) (*ControlException, error) {
	row, err := r.q.CreateCKControlException(ctx, db.CreateCKControlExceptionParams{
		OrgID:        orgID,
		ControlID:    controlID,
		Title:        in.Title,
		Reason:       in.Reason,
		RiskAccepted: in.RiskAccepted,
		ApprovedBy:   in.ApprovedBy,
		ExpiresAt:    ckOptTsPtr(in.ExpiresAt),
		CreatedBy:    createdBy,
	})
	if err != nil {
		return nil, fmt.Errorf("create control exception: %w", err)
	}
	e := exceptionFromCk(db.CkControlExceptions(row))
	return &e, nil
}

// UpdateControlException updates an existing exception record.
func (r *Repository) UpdateControlException(ctx context.Context, orgID, id string, in UpdateControlExceptionInput) (*ControlException, error) {
	// approved_by tri-state: empty string explicitly clears; nil-equivalent
	// would be a separate flag. The original code used `$6` directly and
	// relied on the wrapper passing the right semantic — wir nehmen das
	// gleiche Verhalten an (leerer String == NULL setzen, Wert == setzen).
	var approvedBy pgtype.Text
	approvedBy.String = in.ApprovedBy
	approvedBy.Valid = true // immer setzen — der CASE in SQL macht NULLIF auf ""
	row, err := r.q.UpdateCKControlException(ctx, db.UpdateCKControlExceptionParams{
		ID:           id,
		OrgID:        orgID,
		Title:        ckOptText(in.Title),
		Reason:       ckOptText(in.Reason),
		RiskAccepted: ckOptText(in.RiskAccepted),
		ApprovedBy:   approvedBy,
		ExpiresAt:    ckOptTsPtr(in.ExpiresAt),
		Status:       ckOptText(in.Status),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update control exception: %w", err)
	}
	e := exceptionFromCk(db.CkControlExceptions(row))
	return &e, nil
}

// DeleteControlException removes an exception record.
func (r *Repository) DeleteControlException(ctx context.Context, orgID, id string) error {
	n, err := r.q.DeleteCKControlException(ctx, db.DeleteCKControlExceptionParams{ID: id, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("delete control exception: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
