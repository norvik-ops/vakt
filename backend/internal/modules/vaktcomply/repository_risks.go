package vaktcomply

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/matharnica/vakt/internal/db"
)

// --- Risk Assessment (FR-CK12) ---

func (r *Repository) ListRisks(ctx context.Context, orgID string) ([]Risk, error) {
	rows, err := r.q.ListCKRisks(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list risks: %w", err)
	}
	out := make([]Risk, 0, len(rows))
	for _, row := range rows {
		out = append(out, riskFromFields(riskFields{
			ID: row.ID, OrgID: row.OrgID, Title: row.Title,
			Description: row.Description, Category: row.Category,
			Likelihood: row.Likelihood, Impact: row.Impact, RiskScore: row.RiskScore,
			Owner: row.Owner, Status: row.Status, Treatment: row.Treatment,
			TreatmentNotes:  row.TreatmentNotes,
			TreatmentOption: row.TreatmentOption, TreatmentPlan: row.TreatmentPlan,
			TreatmentOwner: row.TreatmentOwner, TreatmentDueDate: row.TreatmentDueDate,
			TreatmentStatus:    row.TreatmentStatus,
			ResidualLikelihood: row.ResidualLikelihood, ResidualImpact: row.ResidualImpact,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}))
	}
	return out, nil
}

func (r *Repository) GetRisk(ctx context.Context, orgID, id string) (*Risk, error) {
	row, err := r.q.GetCKRisk(ctx, db.GetCKRiskParams{ID: id, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("get risk: %w", err)
	}
	risk := riskFromFields(riskFields{
		ID: row.ID, OrgID: row.OrgID, Title: row.Title,
		Description: row.Description, Category: row.Category,
		Likelihood: row.Likelihood, Impact: row.Impact, RiskScore: row.RiskScore,
		Owner: row.Owner, Status: row.Status, Treatment: row.Treatment,
		TreatmentNotes:  row.TreatmentNotes,
		TreatmentOption: row.TreatmentOption, TreatmentPlan: row.TreatmentPlan,
		TreatmentOwner: row.TreatmentOwner, TreatmentDueDate: row.TreatmentDueDate,
		TreatmentStatus:    row.TreatmentStatus,
		ResidualLikelihood: row.ResidualLikelihood, ResidualImpact: row.ResidualImpact,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	return &risk, nil
}

func (r *Repository) UpdateRisk(ctx context.Context, orgID, id string, in UpdateRiskInput) (*Risk, error) {
	row, err := r.q.UpdateCKRisk(ctx, db.UpdateCKRiskParams{
		ID:             id,
		OrgID:          orgID,
		Title:          in.Title,
		Description:    in.Description,
		Category:       in.Category,
		Likelihood:     int16(in.Likelihood),
		Impact:         int16(in.Impact),
		Owner:          in.Owner,
		Status:         in.Status,
		Treatment:      in.Treatment,
		TreatmentNotes: in.TreatmentNotes,
	})
	if err != nil {
		return nil, fmt.Errorf("update risk: %w", err)
	}
	risk := riskFromFields(riskFields{
		ID: row.ID, OrgID: row.OrgID, Title: row.Title,
		Description: row.Description, Category: row.Category,
		Likelihood: row.Likelihood, Impact: row.Impact, RiskScore: row.RiskScore,
		Owner: row.Owner, Status: row.Status, Treatment: row.Treatment,
		TreatmentNotes:  row.TreatmentNotes,
		TreatmentOption: row.TreatmentOption, TreatmentPlan: row.TreatmentPlan,
		TreatmentOwner: row.TreatmentOwner, TreatmentDueDate: row.TreatmentDueDate,
		TreatmentStatus:    row.TreatmentStatus,
		ResidualLikelihood: row.ResidualLikelihood, ResidualImpact: row.ResidualImpact,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	return &risk, nil
}

// UpdateRiskTreatment patches only the treatment workflow fields for a risk.
// Tri-state TreatmentDueDate:
//   - nil      → keep existing (read current value first)
//   - *""      → set NULL
//   - *"date"  → set the parsed date
func (r *Repository) UpdateRiskTreatment(ctx context.Context, orgID, id string, in UpdateRiskTreatmentInput) (*Risk, error) {
	var dueDate pgtype.Date
	if in.TreatmentDueDate == nil {
		// keep current: read first
		cur, err := r.GetRisk(ctx, orgID, id)
		if err != nil {
			return nil, fmt.Errorf("read risk for due_date keep: %w", err)
		}
		if cur.TreatmentDueDate != nil {
			dueDate = pgtype.Date{Time: *cur.TreatmentDueDate, Valid: true}
		}
	} else if *in.TreatmentDueDate != "" {
		t, err := time.Parse("2006-01-02", *in.TreatmentDueDate)
		if err != nil {
			return nil, fmt.Errorf("parse treatment_due_date: %w", err)
		}
		dueDate = pgtype.Date{Time: t, Valid: true}
	}
	row, err := r.q.UpdateCKRiskTreatment(ctx, db.UpdateCKRiskTreatmentParams{
		ID:                 id,
		OrgID:              orgID,
		TreatmentOption:    ckOptText(in.TreatmentOption),
		TreatmentPlan:      in.TreatmentPlan,
		TreatmentOwner:     in.TreatmentOwner,
		TreatmentStatus:    in.TreatmentStatus,
		ResidualLikelihood: ckOptIntPtr(in.ResidualLikelihood),
		ResidualImpact:     ckOptIntPtr(in.ResidualImpact),
		TreatmentDueDate:   dueDate,
	})
	if err != nil {
		return nil, fmt.Errorf("update risk treatment: %w", err)
	}
	risk := riskFromFields(riskFields{
		ID: row.ID, OrgID: row.OrgID, Title: row.Title,
		Description: row.Description, Category: row.Category,
		Likelihood: row.Likelihood, Impact: row.Impact, RiskScore: row.RiskScore,
		Owner: row.Owner, Status: row.Status, Treatment: row.Treatment,
		TreatmentNotes:  row.TreatmentNotes,
		TreatmentOption: row.TreatmentOption, TreatmentPlan: row.TreatmentPlan,
		TreatmentOwner: row.TreatmentOwner, TreatmentDueDate: row.TreatmentDueDate,
		TreatmentStatus:    row.TreatmentStatus,
		ResidualLikelihood: row.ResidualLikelihood, ResidualImpact: row.ResidualImpact,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	return &risk, nil
}

func (r *Repository) CreateRisk(ctx context.Context, orgID string, in CreateRiskInput) (*Risk, error) {
	row, err := r.q.CreateCKRisk(ctx, db.CreateCKRiskParams{
		OrgID:          orgID,
		Title:          in.Title,
		Description:    in.Description,
		Category:       in.Category,
		Likelihood:     int16(in.Likelihood),
		Impact:         int16(in.Impact),
		Owner:          in.Owner,
		Treatment:      in.Treatment,
		TreatmentNotes: in.TreatmentNotes,
	})
	if err != nil {
		return nil, fmt.Errorf("create risk: %w", err)
	}
	risk := riskFromFields(riskFields{
		ID: row.ID, OrgID: row.OrgID, Title: row.Title,
		Description: row.Description, Category: row.Category,
		Likelihood: row.Likelihood, Impact: row.Impact, RiskScore: row.RiskScore,
		Owner: row.Owner, Status: row.Status, Treatment: row.Treatment,
		TreatmentNotes:  row.TreatmentNotes,
		TreatmentOption: row.TreatmentOption, TreatmentPlan: row.TreatmentPlan,
		TreatmentOwner: row.TreatmentOwner, TreatmentDueDate: row.TreatmentDueDate,
		TreatmentStatus:    row.TreatmentStatus,
		ResidualLikelihood: row.ResidualLikelihood, ResidualImpact: row.ResidualImpact,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	return &risk, nil
}

func (r *Repository) DeleteRisk(ctx context.Context, orgID, id string) error {
	n, err := r.q.DeleteCKRisk(ctx, db.DeleteCKRiskParams{ID: id, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("delete risk: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("risk not found")
	}
	return nil
}

// ListRisksPaged returns a page of risks plus the total count.
func (r *Repository) ListRisksPaged(ctx context.Context, orgID string, offset, limit int) ([]Risk, int, error) {
	total, err := r.q.CountCKRisks(ctx, orgID)
	if err != nil {
		return nil, 0, fmt.Errorf("count risks: %w", err)
	}
	rows, err := r.q.ListCKRisksPaged(ctx, db.ListCKRisksPagedParams{
		OrgID:  orgID,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list risks paged: %w", err)
	}
	risks := make([]Risk, 0, len(rows))
	for _, row := range rows {
		risks = append(risks, riskFromFields(riskFields{
			ID: row.ID, OrgID: row.OrgID, Title: row.Title,
			Description: row.Description, Category: row.Category,
			Likelihood: row.Likelihood, Impact: row.Impact, RiskScore: row.RiskScore,
			Owner: row.Owner, Status: row.Status, Treatment: row.Treatment,
			TreatmentNotes:  row.TreatmentNotes,
			TreatmentOption: row.TreatmentOption, TreatmentPlan: row.TreatmentPlan,
			TreatmentOwner: row.TreatmentOwner, TreatmentDueDate: row.TreatmentDueDate,
			TreatmentStatus:    row.TreatmentStatus,
			ResidualLikelihood: row.ResidualLikelihood, ResidualImpact: row.ResidualImpact,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}))
	}
	return risks, int(total), nil
}

// ListRisksCursor returns risks using keyset pagination ordered by (created_at DESC, id DESC).
// Returns limit+1 rows so the caller can detect HasMore.
func (r *Repository) ListRisksCursor(ctx context.Context, orgID string, cursorID string, cursorTS time.Time, limit int) ([]Risk, error) {
	const baseQ = `
		SELECT id, org_id, title, description, category,
		       likelihood, impact, risk_score, owner, status, treatment, treatment_notes,
		       treatment_option, treatment_plan, treatment_owner, treatment_due_date,
		       treatment_status, residual_likelihood, residual_impact,
		       inherent_likelihood, inherent_impact,
		       risk_accepted_by, risk_accepted_at, risk_acceptance_justification,
		       created_at, updated_at
		FROM ck_risks
		WHERE org_id = $1`

	args := []any{orgID}
	q := baseQ
	n := 2
	if !cursorTS.IsZero() && cursorID != "" {
		args = append(args, cursorTS, cursorID)
		q += fmt.Sprintf(" AND (created_at < $%d OR (created_at = $%d AND id::text < $%d))", n, n, n+1)
		n += 2
	}
	args = append(args, int32(limit+1))
	q += fmt.Sprintf(" ORDER BY created_at DESC, id DESC LIMIT $%d", n)

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list risks cursor: %w", err)
	}
	defer rows.Close()

	var out []Risk
	for rows.Next() {
		var f riskFields
		if err := rows.Scan(
			&f.ID, &f.OrgID, &f.Title, &f.Description, &f.Category,
			&f.Likelihood, &f.Impact, &f.RiskScore, &f.Owner, &f.Status, &f.Treatment, &f.TreatmentNotes,
			&f.TreatmentOption, &f.TreatmentPlan, &f.TreatmentOwner, &f.TreatmentDueDate,
			&f.TreatmentStatus, &f.ResidualLikelihood, &f.ResidualImpact,
			&f.InherentLikelihood, &f.InherentImpact,
			&f.RiskAcceptedBy, &f.RiskAcceptedAt, &f.RiskAcceptanceJustification,
			&f.CreatedAt, &f.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan risk cursor row: %w", err)
		}
		out = append(out, riskFromFields(f))
	}
	return out, rows.Err()
}

// GetRiskFull fetches a single risk including S61-4 residual/acceptance fields via raw SQL.
func (r *Repository) GetRiskFull(ctx context.Context, orgID, id string) (*Risk, error) {
	var f riskFields
	err := r.db.QueryRow(ctx, `
		SELECT id, org_id, title, description, category,
		       likelihood, impact, risk_score, owner, status, treatment, treatment_notes,
		       treatment_option, treatment_plan, treatment_owner, treatment_due_date,
		       treatment_status, residual_likelihood, residual_impact,
		       inherent_likelihood, inherent_impact,
		       risk_accepted_by, risk_accepted_at, risk_acceptance_justification,
		       created_at, updated_at
		FROM ck_risks
		WHERE id = $1 AND org_id = $2`,
		id, orgID,
	).Scan(
		&f.ID, &f.OrgID, &f.Title, &f.Description, &f.Category,
		&f.Likelihood, &f.Impact, &f.RiskScore, &f.Owner, &f.Status, &f.Treatment, &f.TreatmentNotes,
		&f.TreatmentOption, &f.TreatmentPlan, &f.TreatmentOwner, &f.TreatmentDueDate,
		&f.TreatmentStatus, &f.ResidualLikelihood, &f.ResidualImpact,
		&f.InherentLikelihood, &f.InherentImpact,
		&f.RiskAcceptedBy, &f.RiskAcceptedAt, &f.RiskAcceptanceJustification,
		&f.CreatedAt, &f.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get risk full: %w", err)
	}
	risk := riskFromFields(f)
	return &risk, nil
}
