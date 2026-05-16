package secvitals

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ListCCMChecks returns all CCM checks for an organisation.
func (r *Repository) ListCCMChecks(ctx context.Context, orgID string) ([]CCMCheck, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text, control_id::text, name, check_type::text,
		       config, interval_hours, last_run_at, last_status, last_output,
		       enabled, created_at, updated_at
		FROM ck_ccm_checks
		WHERE org_id = $1::uuid
		ORDER BY created_at ASC`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list ccm checks: %w", err)
	}
	defer rows.Close()

	var checks []CCMCheck
	for rows.Next() {
		c, err := scanCCMCheck(rows)
		if err != nil {
			return nil, err
		}
		checks = append(checks, c)
	}
	return checks, rows.Err()
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

	var c CCMCheck
	var configBytes []byte
	var lastStatus, lastOutput *string

	err = r.db.QueryRow(ctx, `
		INSERT INTO ck_ccm_checks (org_id, control_id, name, check_type, config, interval_hours)
		VALUES ($1::uuid, $2::uuid, $3, $4::ck_check_type, $5::jsonb, $6)
		RETURNING id::text, org_id::text, control_id::text, name, check_type::text,
		          config, interval_hours, last_run_at, last_status, last_output,
		          enabled, created_at, updated_at`,
		orgID, in.ControlID, in.Name, in.CheckType, configJSON, intervalHours,
	).Scan(
		&c.ID, &c.OrgID, &c.ControlID, &c.Name, &c.CheckType,
		&configBytes, &c.IntervalHours, &c.LastRunAt, &lastStatus, &lastOutput,
		&c.Enabled, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create ccm check: %w", err)
	}

	if lastStatus != nil {
		c.LastStatus = *lastStatus
	}
	if lastOutput != nil {
		c.LastOutput = *lastOutput
	}
	c.Config = unmarshalConfig(configBytes)
	return &c, nil
}

// GetCCMCheck returns a single CCM check by ID scoped to org.
func (r *Repository) GetCCMCheck(ctx context.Context, orgID, id string) (*CCMCheck, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id::text, org_id::text, control_id::text, name, check_type::text,
		       config, interval_hours, last_run_at, last_status, last_output,
		       enabled, created_at, updated_at
		FROM ck_ccm_checks
		WHERE id = $1::uuid AND org_id = $2::uuid`,
		id, orgID,
	)
	c, err := scanCCMCheckRow(row)
	if err != nil {
		return nil, fmt.Errorf("get ccm check: %w", err)
	}
	return &c, nil
}

// DeleteCCMCheck removes a CCM check by ID scoped to org.
func (r *Repository) DeleteCCMCheck(ctx context.Context, orgID, id string) error {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM ck_ccm_checks WHERE id = $1::uuid AND org_id = $2::uuid`,
		id, orgID,
	)
	if err != nil {
		return fmt.Errorf("delete ccm check: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("ccm check not found")
	}
	return nil
}

// UpdateCCMCheckEnabled toggles the enabled flag on a CCM check.
func (r *Repository) UpdateCCMCheckEnabled(ctx context.Context, id string, enabled bool) error {
	_, err := r.db.Exec(ctx,
		`UPDATE ck_ccm_checks SET enabled = $1, updated_at = now() WHERE id = $2::uuid`,
		enabled, id,
	)
	if err != nil {
		return fmt.Errorf("toggle ccm check: %w", err)
	}
	return nil
}

// SaveCCMResult inserts a new CCM result row.
func (r *Repository) SaveCCMResult(ctx context.Context, checkID, status, output string) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO ck_ccm_results (check_id, status, output) VALUES ($1::uuid, $2, $3)`,
		checkID, status, output,
	)
	if err != nil {
		return fmt.Errorf("save ccm result: %w", err)
	}
	return nil
}

// UpdateCCMCheckLastRun updates last_run_at, last_status, last_output on a check after execution.
func (r *Repository) UpdateCCMCheckLastRun(ctx context.Context, id, status, output string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE ck_ccm_checks
		SET last_run_at = now(), last_status = $1, last_output = $2, updated_at = now()
		WHERE id = $3::uuid`,
		status, output, id,
	)
	if err != nil {
		return fmt.Errorf("update ccm check last run: %w", err)
	}
	return nil
}

// ListDueCCMChecks returns all enabled checks that are due to run.
// A check is due when last_run_at IS NULL or last_run_at + interval_hours < now().
func (r *Repository) ListDueCCMChecks(ctx context.Context) ([]CCMCheck, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text, control_id::text, name, check_type::text,
		       config, interval_hours, last_run_at, last_status, last_output,
		       enabled, created_at, updated_at
		FROM ck_ccm_checks
		WHERE enabled = true
		  AND (last_run_at IS NULL
		       OR last_run_at + (interval_hours * interval '1 hour') < now())
		ORDER BY last_run_at ASC NULLS FIRST`,
	)
	if err != nil {
		return nil, fmt.Errorf("list due ccm checks: %w", err)
	}
	defer rows.Close()

	var checks []CCMCheck
	for rows.Next() {
		c, err := scanCCMCheck(rows)
		if err != nil {
			return nil, err
		}
		checks = append(checks, c)
	}
	return checks, rows.Err()
}

// ListCCMResults returns the most recent results for a given check.
func (r *Repository) ListCCMResults(ctx context.Context, checkID string, limit int) ([]CCMResult, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, check_id::text, status, output, ran_at
		FROM ck_ccm_results
		WHERE check_id = $1::uuid
		ORDER BY ran_at DESC
		LIMIT $2`,
		checkID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list ccm results: %w", err)
	}
	defer rows.Close()

	var results []CCMResult
	for rows.Next() {
		var res CCMResult
		var output *string
		if err := rows.Scan(&res.ID, &res.CheckID, &res.Status, &output, &res.RanAt); err != nil {
			return nil, fmt.Errorf("scan ccm result: %w", err)
		}
		if output != nil {
			res.Output = *output
		}
		results = append(results, res)
	}
	return results, rows.Err()
}

// --- helpers ---

// scanCCMCheck scans a CCMCheck from a pgx.Rows cursor.
func scanCCMCheck(rows pgx.Rows) (CCMCheck, error) {
	var c CCMCheck
	var configBytes []byte
	var lastStatus, lastOutput *string

	if err := rows.Scan(
		&c.ID, &c.OrgID, &c.ControlID, &c.Name, &c.CheckType,
		&configBytes, &c.IntervalHours, &c.LastRunAt, &lastStatus, &lastOutput,
		&c.Enabled, &c.CreatedAt, &c.UpdatedAt,
	); err != nil {
		return c, fmt.Errorf("scan ccm check: %w", err)
	}
	if lastStatus != nil {
		c.LastStatus = *lastStatus
	}
	if lastOutput != nil {
		c.LastOutput = *lastOutput
	}
	c.Config = unmarshalConfig(configBytes)
	return c, nil
}

// scanCCMCheckRow scans a CCMCheck from a pgx.Row (single-row query).
func scanCCMCheckRow(row pgx.Row) (CCMCheck, error) {
	var c CCMCheck
	var configBytes []byte
	var lastStatus, lastOutput *string

	if err := row.Scan(
		&c.ID, &c.OrgID, &c.ControlID, &c.Name, &c.CheckType,
		&configBytes, &c.IntervalHours, &c.LastRunAt, &lastStatus, &lastOutput,
		&c.Enabled, &c.CreatedAt, &c.UpdatedAt,
	); err != nil {
		return c, err
	}
	if lastStatus != nil {
		c.LastStatus = *lastStatus
	}
	if lastOutput != nil {
		c.LastOutput = *lastOutput
	}
	c.Config = unmarshalConfig(configBytes)
	return c, nil
}

// unmarshalConfig deserialises JSONB config bytes into map[string]string.
func unmarshalConfig(b []byte) map[string]string {
	m := make(map[string]string)
	if len(b) == 0 {
		return m
	}
	_ = json.Unmarshal(b, &m)
	return m
}
