package vaktcomply

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/matharnica/vakt/internal/db"
	"github.com/rs/zerolog/log"
)

// ListAllOrgs returns the IDs of all organisations.
// Used for cross-org seeding on startup.
func (r *Repository) ListAllOrgs(ctx context.Context) ([]string, error) {
	ids, err := r.q.ListAllOrgIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("list all orgs: %w", err)
	}
	return ids, nil
}

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

// CountRecentIncidents returns the number of incidents created at or after `since`.
func (r *Repository) CountRecentIncidents(ctx context.Context, orgID string, since time.Time) (int, error) {
	n, err := r.q.CountCKRecentIncidents(ctx, db.CountCKRecentIncidentsParams{OrgID: orgID, Since: since})
	if err != nil {
		return 0, fmt.Errorf("count recent incidents: %w", err)
	}
	return int(n), nil
}

// CountIncidentsSince returns the number of incidents created at or after `since`.
func (r *Repository) CountIncidentsSince(ctx context.Context, orgID string, since time.Time) (int, error) {
	n, err := r.q.CountCKIncidentsSince(ctx, db.CountCKIncidentsSinceParams{OrgID: orgID, Since: since})
	if err != nil {
		return 0, fmt.Errorf("count incidents since: %w", err)
	}
	return int(n), nil
}

type ChangeLogEntry struct {
	ID        string    `json:"id"`
	ControlID string    `json:"control_id"`
	UserEmail string    `json:"user_email"`
	Field     string    `json:"field"`
	OldValue  string    `json:"old_value"`
	NewValue  string    `json:"new_value"`
	ChangedAt time.Time `json:"changed_at"`
}

// AppendControlChange inserts a change log entry into ck_control_changelog.
// Errors are logged but not returned — a changelog write failure must never
// abort the primary update operation.
func (r *Repository) AppendControlChange(ctx context.Context, orgID, controlID, userID, userEmail, field, oldVal, newVal string) {
	err := r.q.AppendCKControlChange(ctx, db.AppendCKControlChangeParams{
		ControlID: controlID,
		OrgID:     orgID,
		UserID:    ckOptUUIDFromStr(userID),
		UserEmail: ckOptText(userEmail),
		Field:     field,
		OldValue:  ckOptText(oldVal),
		NewValue:  ckOptText(newVal),
	})
	if err != nil {
		log.Error().
			Err(err).
			Str("control_id", controlID).
			Str("field", field).
			Msg("changelog: failed to append control change")
	}
}

// ListControlChanges returns the last 50 field-level changes for a control,
// ordered newest first.
func (r *Repository) ListControlChanges(ctx context.Context, orgID, controlID string) ([]ChangeLogEntry, error) {
	rows, err := r.q.ListCKControlChanges(ctx, db.ListCKControlChangesParams{OrgID: orgID, ControlID: controlID})
	if err != nil {
		return nil, err
	}
	out := make([]ChangeLogEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, ChangeLogEntry{
			ID:        row.ID,
			ControlID: row.ControlID,
			UserEmail: row.UserEmail.String,
			Field:     row.Field,
			OldValue:  row.OldValue.String,
			NewValue:  row.NewValue.String,
			ChangedAt: ckTsToTime(row.ChangedAt),
		})
	}
	return out, nil
}
