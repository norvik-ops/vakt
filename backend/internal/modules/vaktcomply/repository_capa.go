package vaktcomply

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/matharnica/vakt/internal/db"
)

// --- CAPA (Corrective and Preventive Actions) ---

// capaFromCkCapas maps the sqlc Table-Row to the domain CAPA type.
func capaFromCkCapas(r db.CkCapas) CAPA {
	return CAPA{
		ID:               r.ID,
		OrgID:            r.OrgID,
		SourceType:       r.SourceType,
		SourceID:         r.SourceID,
		Title:            r.Title,
		Description:      r.Description,
		RootCause:        r.RootCause,
		ActionPlan:       r.ActionPlan,
		AssigneeEmail:    r.AssigneeEmail,
		DueDate:          ckDateToTimePtr(r.DueDate),
		Priority:         r.Priority,
		Status:           r.Status,
		VerificationNote: r.VerificationNote,
		ClosedAt:         ckTsToTimePtr(r.ClosedAt),
		CreatedAt:        ckTsToTime(r.CreatedAt),
		UpdatedAt:        ckTsToTime(r.UpdatedAt),
	}
}

// ListCAPAs returns CAPAs for an organisation, optionally filtered by status.
func (r *Repository) ListCAPAs(ctx context.Context, orgID string, statusFilter string) ([]CAPA, error) {
	rows, err := r.q.ListCKCAPAs(ctx, db.ListCKCAPAsParams{
		OrgID:  orgID,
		Status: ckOptText(statusFilter),
	})
	if err != nil {
		return nil, fmt.Errorf("list capas: %w", err)
	}
	out := make([]CAPA, 0, len(rows))
	for _, row := range rows {
		out = append(out, capaFromCkCapas(row))
	}
	return out, nil
}

// ListCAPAsForSource returns CAPAs linked to a specific source (audit/incident/risk).
func (r *Repository) ListCAPAsForSource(ctx context.Context, orgID, sourceType, sourceID string) ([]CAPA, error) {
	rows, err := r.q.ListCKCAPAsForSource(ctx, db.ListCKCAPAsForSourceParams{
		OrgID:      orgID,
		SourceType: sourceType,
		SourceID:   sourceID,
	})
	if err != nil {
		return nil, fmt.Errorf("list capas for source: %w", err)
	}
	out := make([]CAPA, 0, len(rows))
	for _, row := range rows {
		out = append(out, capaFromCkCapas(row))
	}
	return out, nil
}

// GetCAPA returns a single CAPA by ID within an organisation.
func (r *Repository) GetCAPA(ctx context.Context, orgID, capaID string) (CAPA, error) {
	row, err := r.q.GetCKCAPA(ctx, db.GetCKCAPAParams{ID: capaID, OrgID: orgID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CAPA{}, ErrNotFound
		}
		return CAPA{}, fmt.Errorf("get capa: %w", err)
	}
	return capaFromCkCapas(row), nil
}

// CreateCAPA inserts a new CAPA record.
func (r *Repository) CreateCAPA(ctx context.Context, orgID string, in CreateCAPAInput) (CAPA, error) {
	priority := in.Priority
	if priority == "" {
		priority = "medium"
	}
	row, err := r.q.CreateCKCAPA(ctx, db.CreateCKCAPAParams{
		OrgID:         orgID,
		SourceType:    in.SourceType,
		SourceID:      in.SourceID,
		Title:         in.Title,
		Description:   in.Description,
		AssigneeEmail: in.AssigneeEmail,
		DueDate:       ckOptDatePtr(in.DueDate),
		Priority:      priority,
	})
	if err != nil {
		return CAPA{}, fmt.Errorf("create capa: %w", err)
	}
	return capaFromCkCapas(row), nil
}

// UpdateCAPA applies partial updates to a CAPA using COALESCE.
// When status transitions to 'closed', closed_at is set to NOW().
func (r *Repository) UpdateCAPA(ctx context.Context, orgID, capaID string, in UpdateCAPAInput) (CAPA, error) {
	row, err := r.q.UpdateCKCAPA(ctx, db.UpdateCKCAPAParams{
		ID:               capaID,
		OrgID:            orgID,
		Title:            optTextStrPtr(in.Title),
		Description:      optTextStrPtr(in.Description),
		RootCause:        optTextStrPtr(in.RootCause),
		ActionPlan:       optTextStrPtr(in.ActionPlan),
		AssigneeEmail:    optTextStrPtr(in.AssigneeEmail),
		DueDate:          ckOptDatePtr(in.DueDate),
		Priority:         optTextStrPtr(in.Priority),
		Status:           optTextStrPtr(in.Status),
		VerificationNote: optTextStrPtr(in.VerificationNote),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CAPA{}, ErrNotFound
		}
		return CAPA{}, fmt.Errorf("update capa: %w", err)
	}
	return capaFromCkCapas(row), nil
}

// DeleteCAPA removes a CAPA record.
func (r *Repository) DeleteCAPA(ctx context.Context, orgID, capaID string) error {
	n, err := r.q.DeleteCKCAPA(ctx, db.DeleteCKCAPAParams{ID: capaID, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("delete capa: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListCAPAsPaged returns a page of CAPAs plus the total count.
func (r *Repository) ListCAPAsPaged(ctx context.Context, orgID string, statusFilter string, offset, limit int) ([]CAPA, int, error) {
	statusArg := ckOptText(statusFilter)
	total, err := r.q.CountCKCAPAs(ctx, db.CountCKCAPAsParams{OrgID: orgID, Status: statusArg})
	if err != nil {
		return nil, 0, fmt.Errorf("count capas: %w", err)
	}
	rows, err := r.q.ListCKCAPAsPaged(ctx, db.ListCKCAPAsPagedParams{
		OrgID:  orgID,
		Status: statusArg,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list capas paged: %w", err)
	}
	capas := make([]CAPA, 0, len(rows))
	for _, row := range rows {
		capas = append(capas, capaFromCkCapas(row))
	}
	return capas, int(total), nil
}

// BulkUpdateCAPAStatus sets status for all CAPAs in ids that belong to the org.
// Behavior unchanged from original embedded query but jetzt setzt der Query
// auch closed_at = NOW() bei Übergang in 'closed' (Audit-Trail-Konsistenz mit
// UpdateCAPA).
func (r *Repository) BulkUpdateCAPAStatus(ctx context.Context, orgID string, ids []string, status string) error {
	_, err := r.q.BulkUpdateCKCAPAStatus(ctx, db.BulkUpdateCKCAPAStatusParams{
		OrgID:  orgID,
		Status: status,
		Ids:    ids,
	})
	if err != nil {
		return fmt.Errorf("bulk update capa status: %w", err)
	}
	return nil
}
