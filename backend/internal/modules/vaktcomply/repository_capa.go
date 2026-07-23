package vaktcomply

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/matharnica/vakt/internal/db"
)

// --- CAPA (Corrective and Preventive Actions) ---

// capaCols is the canonical column projection for a CAPA read. It includes the
// NC/effectiveness fields (Migration 163) that the local sqlc `CkCapas` model
// never regenerated — S131-G3/D27-02: these were written by the NC-workflow
// endpoint but no read selected them, so the FE badges/edit-form (nc_classification,
// effectiveness_*, immediate_containment, similar_ncs_*) were silently always empty
// after a save. Read via raw queries here rather than mutating the shared generated
// model + 7 generated queries (a scan-order slip there is worse than the missing data).
const capaCols = `id, org_id, source_type, source_id, title, description,
	root_cause, action_plan, assignee_email, due_date, priority,
	status, verification_note, closed_at, created_at, updated_at,
	nc_classification, immediate_containment, similar_ncs_assessed, similar_ncs_notes,
	effectiveness_check_date, effectiveness_confirmed, effectiveness_checked_at,
	effectiveness_checked_by::text, effectiveness_evidence`

// capaRow scans the capaCols projection. Field order MUST match capaCols exactly.
type capaRow struct {
	ID, OrgID, SourceType, SourceID, Title, Description string
	RootCause, ActionPlan, AssigneeEmail                string
	DueDate                                             pgtype.Date
	Priority, Status, VerificationNote                  string
	ClosedAt, CreatedAt, UpdatedAt                      pgtype.Timestamptz
	NCClassification                                    pgtype.Text
	ImmediateContainment                                string
	SimilarNCsAssessed                                  pgtype.Bool
	SimilarNCsNotes                                     string
	EffectivenessCheckDate                              pgtype.Date
	EffectivenessConfirmed                              pgtype.Bool
	EffectivenessCheckedAt                              pgtype.Timestamptz
	EffectivenessCheckedBy                              pgtype.Text
	EffectivenessEvidence                               string
}

// scanCapa scans one capaCols row (from QueryRow or the current Rows position —
// pgx.Rows satisfies pgx.Row) into the domain CAPA. Single source of the 25-field
// scan so the column order lives in exactly one place next to capaCols.
func scanCapa(row pgx.Row) (CAPA, error) {
	var f capaRow
	if err := row.Scan(
		&f.ID, &f.OrgID, &f.SourceType, &f.SourceID, &f.Title, &f.Description,
		&f.RootCause, &f.ActionPlan, &f.AssigneeEmail, &f.DueDate, &f.Priority,
		&f.Status, &f.VerificationNote, &f.ClosedAt, &f.CreatedAt, &f.UpdatedAt,
		&f.NCClassification, &f.ImmediateContainment, &f.SimilarNCsAssessed, &f.SimilarNCsNotes,
		&f.EffectivenessCheckDate, &f.EffectivenessConfirmed, &f.EffectivenessCheckedAt,
		&f.EffectivenessCheckedBy, &f.EffectivenessEvidence,
	); err != nil {
		return CAPA{}, err
	}
	return capaFromRow(f), nil
}

func capaFromRow(f capaRow) CAPA {
	return CAPA{
		ID:                     f.ID,
		OrgID:                  f.OrgID,
		SourceType:             f.SourceType,
		SourceID:               f.SourceID,
		Title:                  f.Title,
		Description:            f.Description,
		RootCause:              f.RootCause,
		ActionPlan:             f.ActionPlan,
		AssigneeEmail:          f.AssigneeEmail,
		DueDate:                ckDateToTimePtr(f.DueDate),
		Priority:               f.Priority,
		Status:                 f.Status,
		VerificationNote:       f.VerificationNote,
		ClosedAt:               ckTsToTimePtr(f.ClosedAt),
		CreatedAt:              ckTsToTime(f.CreatedAt),
		UpdatedAt:              ckTsToTime(f.UpdatedAt),
		NCClassification:       ckTextPtr(f.NCClassification),
		ImmediateContainment:   f.ImmediateContainment,
		SimilarNCsAssessed:     ckBoolPtr(f.SimilarNCsAssessed),
		SimilarNCsNotes:        f.SimilarNCsNotes,
		EffectivenessCheckDate: ckDatePtrYMD(f.EffectivenessCheckDate),
		EffectivenessConfirmed: ckBoolPtr(f.EffectivenessConfirmed),
		EffectivenessCheckedAt: ckTsToTimePtr(f.EffectivenessCheckedAt),
		EffectivenessCheckedBy: ckTextPtr(f.EffectivenessCheckedBy),
		EffectivenessEvidence:  f.EffectivenessEvidence,
	}
}

// ckTextPtr renders a NULLable pgtype.Text as *string, nil when NULL or empty.
func ckTextPtr(t pgtype.Text) *string {
	if !t.Valid || t.String == "" {
		return nil
	}
	s := t.String
	return &s
}

// ckBoolPtr renders a NULLable pgtype.Bool as *bool.
func ckBoolPtr(b pgtype.Bool) *bool {
	if !b.Valid {
		return nil
	}
	v := b.Bool
	return &v
}

// ckDatePtrYMD renders a NULLable pgtype.Date as *string in YYYY-MM-DD, nil when NULL.
func ckDatePtrYMD(d pgtype.Date) *string {
	if !d.Valid {
		return nil
	}
	s := d.Time.Format("2006-01-02")
	return &s
}

// ListCAPAs returns CAPAs for an organisation, optionally filtered by status.
func (r *Repository) ListCAPAs(ctx context.Context, orgID string, statusFilter string) ([]CAPA, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+capaCols+` FROM ck_capas
		 WHERE org_id = $1 AND ($2::text IS NULL OR status = $2::text)
		 ORDER BY created_at DESC`,
		orgID, ckOptText(statusFilter))
	if err != nil {
		return nil, fmt.Errorf("list capas: %w", err)
	}
	defer rows.Close()
	out := []CAPA{}
	for rows.Next() {
		c, err := scanCapa(rows)
		if err != nil {
			return nil, fmt.Errorf("scan capa row: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ListCAPAsForSource returns CAPAs linked to a specific source (audit/incident/risk).
func (r *Repository) ListCAPAsForSource(ctx context.Context, orgID, sourceType, sourceID string) ([]CAPA, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+capaCols+` FROM ck_capas
		 WHERE org_id = $1 AND source_type = $2 AND source_id = $3
		 ORDER BY created_at DESC`,
		orgID, sourceType, sourceID)
	if err != nil {
		return nil, fmt.Errorf("list capas for source: %w", err)
	}
	defer rows.Close()
	out := []CAPA{}
	for rows.Next() {
		c, err := scanCapa(rows)
		if err != nil {
			return nil, fmt.Errorf("scan capa row: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetCAPA returns a single CAPA by ID within an organisation.
func (r *Repository) GetCAPA(ctx context.Context, orgID, capaID string) (CAPA, error) {
	c, err := scanCapa(r.db.QueryRow(ctx,
		`SELECT `+capaCols+` FROM ck_capas WHERE id = $1 AND org_id = $2`,
		capaID, orgID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CAPA{}, ErrNotFound
		}
		return CAPA{}, fmt.Errorf("get capa: %w", err)
	}
	return c, nil
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
	// Re-read via the NC-aware projection so the returned CAPA carries the same
	// shape as a subsequent GET (the generated RETURNING omits the NC fields).
	return r.GetCAPA(ctx, orgID, row.ID)
}

// UpdateCAPA applies partial updates to a CAPA using COALESCE.
// When status transitions to 'closed', closed_at is set to NOW().
func (r *Repository) UpdateCAPA(ctx context.Context, orgID, capaID string, in UpdateCAPAInput) (CAPA, error) {
	_, err := r.q.UpdateCKCAPA(ctx, db.UpdateCKCAPAParams{
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
	// NC-aware re-read: a generic update must not wipe the NC badges from the
	// returned row (the generated RETURNING omits those columns).
	return r.GetCAPA(ctx, orgID, capaID)
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
	rows, err := r.db.Query(ctx,
		`SELECT `+capaCols+` FROM ck_capas
		 WHERE org_id = $1 AND ($2::text IS NULL OR status = $2::text)
		 ORDER BY created_at DESC LIMIT $3 OFFSET $4`,
		orgID, statusArg, int32(limit), int32(offset))
	if err != nil {
		return nil, 0, fmt.Errorf("list capas paged: %w", err)
	}
	defer rows.Close()
	capas := []CAPA{}
	for rows.Next() {
		c, err := scanCapa(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan capa row: %w", err)
		}
		capas = append(capas, c)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
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
