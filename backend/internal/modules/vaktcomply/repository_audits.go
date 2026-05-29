package vaktcomply

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/matharnica/vakt/internal/db"
)

// --- Internal Audit Records (FR-CK15) ---

// auditRecordFields is shared between Create/Get/List/Update Row types.
type auditRecordFields struct {
	ID, OrgID, Title, Scope, Auditor, Status, Findings, Recommendations string
	AuditDate                                                           pgtype.Date
	CreatedAt, UpdatedAt                                                pgtype.Timestamptz
}

func auditRecordFromFields(f auditRecordFields) AuditRecord {
	rec := AuditRecord{
		ID:              f.ID,
		OrgID:           f.OrgID,
		Title:           f.Title,
		Scope:           f.Scope,
		Auditor:         f.Auditor,
		Status:          f.Status,
		Findings:        f.Findings,
		Recommendations: f.Recommendations,
		CreatedAt:       ckTsToTime(f.CreatedAt),
		UpdatedAt:       ckTsToTime(f.UpdatedAt),
	}
	if f.AuditDate.Valid {
		rec.AuditDate = f.AuditDate.Time
	}
	return rec
}

func (r *Repository) ListAuditRecords(ctx context.Context, orgID string) ([]AuditRecord, error) {
	rows, err := r.q.ListCKAuditRecords(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list audit records: %w", err)
	}
	out := make([]AuditRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, auditRecordFromFields(auditRecordFields{
			ID: row.ID, OrgID: row.OrgID, Title: row.Title, Scope: row.Scope,
			Auditor: row.Auditor, AuditDate: row.AuditDate, Status: row.Status,
			Findings: row.Findings, Recommendations: row.Recommendations,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}))
	}
	return out, nil
}

func (r *Repository) GetAuditRecord(ctx context.Context, orgID, id string) (*AuditRecord, error) {
	row, err := r.q.GetCKAuditRecord(ctx, db.GetCKAuditRecordParams{ID: id, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("get audit record: %w", err)
	}
	rec := auditRecordFromFields(auditRecordFields{
		ID: row.ID, OrgID: row.OrgID, Title: row.Title, Scope: row.Scope,
		Auditor: row.Auditor, AuditDate: row.AuditDate, Status: row.Status,
		Findings: row.Findings, Recommendations: row.Recommendations,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	return &rec, nil
}

func (r *Repository) UpdateAuditRecord(ctx context.Context, orgID, id string, in UpdateAuditRecordInput) (*AuditRecord, error) {
	row, err := r.q.UpdateCKAuditRecord(ctx, db.UpdateCKAuditRecordParams{
		ID:              id,
		OrgID:           orgID,
		Title:           in.Title,
		Scope:           in.Scope,
		Auditor:         in.Auditor,
		AuditDate:       pgtype.Date{Time: in.AuditDate, Valid: true},
		Status:          in.Status,
		Findings:        in.Findings,
		Recommendations: in.Recommendations,
	})
	if err != nil {
		return nil, fmt.Errorf("update audit record: %w", err)
	}
	rec := auditRecordFromFields(auditRecordFields{
		ID: row.ID, OrgID: row.OrgID, Title: row.Title, Scope: row.Scope,
		Auditor: row.Auditor, AuditDate: row.AuditDate, Status: row.Status,
		Findings: row.Findings, Recommendations: row.Recommendations,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	return &rec, nil
}

func (r *Repository) CreateAuditRecord(ctx context.Context, orgID string, in CreateAuditRecordInput) (*AuditRecord, error) {
	row, err := r.q.CreateCKAuditRecord(ctx, db.CreateCKAuditRecordParams{
		OrgID:           orgID,
		Title:           in.Title,
		Scope:           in.Scope,
		Auditor:         in.Auditor,
		AuditDate:       pgtype.Date{Time: in.AuditDate, Valid: true},
		Findings:        in.Findings,
		Recommendations: in.Recommendations,
	})
	if err != nil {
		return nil, fmt.Errorf("create audit record: %w", err)
	}
	rec := auditRecordFromFields(auditRecordFields{
		ID: row.ID, OrgID: row.OrgID, Title: row.Title, Scope: row.Scope,
		Auditor: row.Auditor, AuditDate: row.AuditDate, Status: row.Status,
		Findings: row.Findings, Recommendations: row.Recommendations,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	return &rec, nil
}

// --- Score History ---

// InsertScoreSnapshot inserts a compliance score snapshot for an organisation.
// frameworkID is optional (pass empty string for the org-wide snapshot).
func (r *Repository) InsertScoreSnapshot(ctx context.Context, orgID string, frameworkID *string, score float64, total, implemented int) error {
	var fwID pgtype.UUID
	if frameworkID != nil && *frameworkID != "" {
		fwID = ckOptUUIDFromStr(*frameworkID)
	}
	return r.q.InsertCKScoreSnapshot(ctx, db.InsertCKScoreSnapshotParams{
		OrgID:               orgID,
		FrameworkID:         fwID,
		Score:               float64PtrToNumericCK(&score),
		ControlsTotal:       int32(total),
		ControlsImplemented: int32(implemented),
	})
}

// float64PtrToNumericCK is the vaktcomply-local copy of the vaktscan helper.
func float64PtrToNumericCK(f *float64) pgtype.Numeric {
	if f == nil {
		return pgtype.Numeric{}
	}
	var n pgtype.Numeric
	if err := n.Scan(strconv.FormatFloat(*f, 'f', -1, 64)); err != nil {
		return pgtype.Numeric{}
	}
	return n
}

// ScoreHistoryEntry is a single data point for the score trend chart.
type ScoreHistoryEntry struct {
	Date                string  `json:"date"`
	Score               float64 `json:"score"`
	ControlsTotal       int     `json:"controls_total"`
	ControlsImplemented int     `json:"controls_implemented"`
}

// GetScoreHistory returns aggregated daily score history for an organisation.
// framework_id is nil to query the org-wide score. Days is the look-back window.
func (r *Repository) GetScoreHistory(ctx context.Context, orgID string, days int) ([]ScoreHistoryEntry, error) {
	rows, err := r.q.GetCKScoreHistory(ctx, db.GetCKScoreHistoryParams{
		OrgID: orgID,
		Days:  int32(days),
	})
	if err != nil {
		return nil, fmt.Errorf("get score history: %w", err)
	}
	out := make([]ScoreHistoryEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, ScoreHistoryEntry{
			Date:                row.Date,
			Score:               row.Score,
			ControlsTotal:       int(row.ControlsTotal),
			ControlsImplemented: int(row.ControlsImplemented),
		})
	}
	return out, nil
}

// --- Board Report + Executive Summary (s26-sqlc-vitals-4) ---

// BoardReportComplianceScoreRow is a single framework's control counts for the board report score.
type BoardReportComplianceScoreRow struct {
	Total       int
	Implemented int
}

// GetBoardReportComplianceScoreRows returns per-framework control counts for computing the weighted compliance score.
func (r *Repository) GetBoardReportComplianceScoreRows(ctx context.Context, orgID string) ([]BoardReportComplianceScoreRow, error) {
	rows, err := r.q.GetBoardReportComplianceScoreRows(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("board report compliance score: %w", err)
	}
	out := make([]BoardReportComplianceScoreRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, BoardReportComplianceScoreRow{
			Total:       int(row.Total),
			Implemented: int(row.Implemented),
		})
	}
	return out, nil
}

// GetPreviousScore returns the most recent compliance score snapshot before today (for board report delta).
// Returns 0 and no error when no prior snapshot exists.
func (r *Repository) GetPreviousScore(ctx context.Context, orgID string) (int, error) {
	score, err := r.q.GetCKPreviousScore(ctx, orgID)
	if err != nil {
		return 0, err
	}
	return int(score), nil
}

// ListActiveOrgIDs returns IDs of all non-deleted organisations.
func (r *Repository) ListActiveOrgIDs(ctx context.Context) ([]string, error) {
	ids, err := r.q.ListActiveOrgIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active org ids: %w", err)
	}
	return ids, nil
}

// ExecutiveFrameworkScoreRow holds name + control counts for the executive summary.
type ExecutiveFrameworkScoreRow struct {
	Name        string
	Total       int
	Implemented int
}

// GetExecutiveFrameworkScores returns per-framework name + control counts for the executive summary.
func (r *Repository) GetExecutiveFrameworkScores(ctx context.Context, orgID string) ([]ExecutiveFrameworkScoreRow, error) {
	rows, err := r.q.GetExecutiveFrameworkScores(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("executive framework scores: %w", err)
	}
	out := make([]ExecutiveFrameworkScoreRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, ExecutiveFrameworkScoreRow{
			Name:        row.Name,
			Total:       int(row.Total),
			Implemented: int(row.Implemented),
		})
	}
	return out, nil
}

// ExecutiveTopRiskRow holds title, score and severity for the top-5 risks.
type ExecutiveTopRiskRow struct {
	Title    string
	Score    int
	Severity string
}

// GetExecutiveTopRisks returns the top-5 open risks by score for the executive summary.
func (r *Repository) GetExecutiveTopRisks(ctx context.Context, orgID string) ([]ExecutiveTopRiskRow, error) {
	rows, err := r.q.GetExecutiveTopRisks(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("executive top risks: %w", err)
	}
	out := make([]ExecutiveTopRiskRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, ExecutiveTopRiskRow{
			Title:    row.Title,
			Score:    int(row.Score),
			Severity: row.Severity,
		})
	}
	return out, nil
}

// CountClosedControlsSince returns the number of controls set to 'implemented' since `since`.
func (r *Repository) CountClosedControlsSince(ctx context.Context, orgID string, since time.Time) (int, error) {
	n, err := r.q.CountCKClosedControlsSince(ctx, db.CountCKClosedControlsSinceParams{OrgID: orgID, Since: since})
	if err != nil {
		return 0, fmt.Errorf("count closed controls since: %w", err)
	}
	return int(n), nil
}

// CountResolvedFindingsSince returns the number of findings set to 'resolved' since `since`.
func (r *Repository) CountResolvedFindingsSince(ctx context.Context, orgID string, since time.Time) (int, error) {
	n, err := r.q.CountSPResolvedFindingsSince(ctx, db.CountSPResolvedFindingsSinceParams{OrgID: orgID, Since: since})
	if err != nil {
		return 0, fmt.Errorf("count resolved findings since: %w", err)
	}
	return int(n), nil
}
