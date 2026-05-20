package secvitals

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/matharnica/vakt/internal/db"
)

// ccmCheckFields is the shared field-container for all CCM-Check Row-Types
// (Create/Get/List/ListDue). Alle Row-Types haben identische Shape; eine
// einzige Mapper-Funktion reicht.
type ccmCheckFields struct {
	ID, OrgID, ControlID, Name, CheckType string
	Config                                json.RawMessage
	IntervalHours                         int32
	LastRunAt                             pgtype.Timestamptz
	LastStatus, LastOutput                pgtype.Text
	Enabled                               bool
	CreatedAt, UpdatedAt                  pgtype.Timestamptz
}

func ccmCheckFromFields(f ccmCheckFields) CCMCheck {
	c := CCMCheck{
		ID:            f.ID,
		OrgID:         f.OrgID,
		ControlID:     f.ControlID,
		Name:          f.Name,
		CheckType:     f.CheckType,
		IntervalHours: int(f.IntervalHours),
		LastRunAt:     ckTsToTimePtr(f.LastRunAt),
		LastStatus:    f.LastStatus.String,
		LastOutput:    f.LastOutput.String,
		Enabled:       f.Enabled,
		CreatedAt:     ckTsToTime(f.CreatedAt),
		UpdatedAt:     ckTsToTime(f.UpdatedAt),
	}
	c.Config = unmarshalCCMConfig(f.Config)
	return c
}

func unmarshalCCMConfig(b []byte) map[string]string {
	m := make(map[string]string)
	if len(b) == 0 {
		return m
	}
	_ = json.Unmarshal(b, &m)
	return m
}

// ListCCMChecks returns all CCM checks for an organisation.
func (r *Repository) ListCCMChecks(ctx context.Context, orgID string) ([]CCMCheck, error) {
	rows, err := r.q.ListCKCCMChecks(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list ccm checks: %w", err)
	}
	out := make([]CCMCheck, 0, len(rows))
	for _, row := range rows {
		out = append(out, ccmCheckFromFields(ccmCheckFields{
			ID: row.ID, OrgID: row.OrgID, ControlID: row.ControlID,
			Name: row.Name, CheckType: row.CheckType,
			Config: row.Config, IntervalHours: row.IntervalHours,
			LastRunAt: row.LastRunAt, LastStatus: row.LastStatus,
			LastOutput: row.LastOutput, Enabled: row.Enabled,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}))
	}
	return out, nil
}

// CreateCCMCheck inserts a new CCM check.
func (r *Repository) CreateCCMCheck(ctx context.Context, orgID string, in CreateCCMCheckInput) (*CCMCheck, error) {
	configJSON, err := json.Marshal(in.Config)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}
	intervalHours := in.IntervalHours
	if intervalHours == 0 {
		intervalHours = 24
	}
	row, err := r.q.CreateCKCCMCheck(ctx, db.CreateCKCCMCheckParams{
		OrgID:         orgID,
		ControlID:     in.ControlID,
		Name:          in.Name,
		CheckType:     db.CkCheckType(in.CheckType),
		Config:        configJSON,
		IntervalHours: int32(intervalHours),
	})
	if err != nil {
		return nil, fmt.Errorf("create ccm check: %w", err)
	}
	c := ccmCheckFromFields(ccmCheckFields{
		ID: row.ID, OrgID: row.OrgID, ControlID: row.ControlID,
		Name: row.Name, CheckType: row.CheckType,
		Config: row.Config, IntervalHours: row.IntervalHours,
		LastRunAt: row.LastRunAt, LastStatus: row.LastStatus,
		LastOutput: row.LastOutput, Enabled: row.Enabled,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	return &c, nil
}

// GetCCMCheck returns a single CCM check by ID scoped to org.
func (r *Repository) GetCCMCheck(ctx context.Context, orgID, id string) (*CCMCheck, error) {
	row, err := r.q.GetCKCCMCheck(ctx, db.GetCKCCMCheckParams{ID: id, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("get ccm check: %w", err)
	}
	c := ccmCheckFromFields(ccmCheckFields{
		ID: row.ID, OrgID: row.OrgID, ControlID: row.ControlID,
		Name: row.Name, CheckType: row.CheckType,
		Config: row.Config, IntervalHours: row.IntervalHours,
		LastRunAt: row.LastRunAt, LastStatus: row.LastStatus,
		LastOutput: row.LastOutput, Enabled: row.Enabled,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	return &c, nil
}

// DeleteCCMCheck removes a CCM check by ID scoped to org.
func (r *Repository) DeleteCCMCheck(ctx context.Context, orgID, id string) error {
	n, err := r.q.DeleteCKCCMCheck(ctx, db.DeleteCKCCMCheckParams{ID: id, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("delete ccm check: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("ccm check not found")
	}
	return nil
}

// UpdateCCMCheckEnabled toggles the enabled flag on a CCM check.
func (r *Repository) UpdateCCMCheckEnabled(ctx context.Context, id string, enabled bool) error {
	if err := r.q.UpdateCKCCMCheckEnabled(ctx, db.UpdateCKCCMCheckEnabledParams{
		Enabled: enabled,
		ID:      id,
	}); err != nil {
		return fmt.Errorf("toggle ccm check: %w", err)
	}
	return nil
}

// SaveCCMResult inserts a new CCM result row.
func (r *Repository) SaveCCMResult(ctx context.Context, checkID, status, output string) error {
	if err := r.q.SaveCKCCMResult(ctx, db.SaveCKCCMResultParams{
		CheckID: checkID,
		Status:  status,
		Output:  ckOptText(output),
	}); err != nil {
		return fmt.Errorf("save ccm result: %w", err)
	}
	return nil
}

// UpdateCCMCheckLastRun updates last_run_at, last_status, last_output on a check after execution.
func (r *Repository) UpdateCCMCheckLastRun(ctx context.Context, id, status, output string) error {
	if err := r.q.UpdateCKCCMCheckLastRun(ctx, db.UpdateCKCCMCheckLastRunParams{
		LastStatus: ckOptText(status),
		LastOutput: ckOptText(output),
		ID:         id,
	}); err != nil {
		return fmt.Errorf("update ccm check last run: %w", err)
	}
	return nil
}

// ListDueCCMChecks returns all enabled checks that are due to run.
func (r *Repository) ListDueCCMChecks(ctx context.Context) ([]CCMCheck, error) {
	rows, err := r.q.ListCKDueCCMChecks(ctx)
	if err != nil {
		return nil, fmt.Errorf("list due ccm checks: %w", err)
	}
	out := make([]CCMCheck, 0, len(rows))
	for _, row := range rows {
		out = append(out, ccmCheckFromFields(ccmCheckFields{
			ID: row.ID, OrgID: row.OrgID, ControlID: row.ControlID,
			Name: row.Name, CheckType: row.CheckType,
			Config: row.Config, IntervalHours: row.IntervalHours,
			LastRunAt: row.LastRunAt, LastStatus: row.LastStatus,
			LastOutput: row.LastOutput, Enabled: row.Enabled,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}))
	}
	return out, nil
}

// ListCCMResults returns the most recent results for a given check.
func (r *Repository) ListCCMResults(ctx context.Context, checkID string, limit int) ([]CCMResult, error) {
	rows, err := r.q.ListCKCCMResults(ctx, db.ListCKCCMResultsParams{
		CheckID: checkID,
		Limit:   int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list ccm results: %w", err)
	}
	out := make([]CCMResult, 0, len(rows))
	for _, row := range rows {
		out = append(out, CCMResult{
			ID:      row.ID,
			CheckID: row.CheckID,
			Status:  row.Status,
			Output:  row.Output.String,
			RanAt:   ckTsToTime(row.RanAt),
		})
	}
	return out, nil
}
