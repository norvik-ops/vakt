package vakthr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/matharnica/vakt/internal/db"
)

// Repository handles HR data access via sqlc-generated queries.
type Repository struct {
	db *pgxpool.Pool
	q  *db.Queries
}

// NewRepository creates a new HR repository backed by the given pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{db: pool, q: db.New(pool)}
}

// --- type conversion helpers ---

func optText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

func textToString(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

func optDate(s string) (pgtype.Date, error) {
	if s == "" {
		return pgtype.Date{}, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return pgtype.Date{}, fmt.Errorf("parse date %q: %w", s, err)
	}
	return pgtype.Date{Time: t, Valid: true}, nil
}

func dateToString(d pgtype.Date) *string {
	if !d.Valid {
		return nil
	}
	s := d.Time.Format("2006-01-02")
	return &s
}

func tsToTime(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}

func tsToTimePtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	tt := t.Time
	return &tt
}

func optTimestamptz(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

// --- mapping helpers (db row → domain model) ---

func employeeFromRow(r db.HrEmployees) Employee {
	return Employee{
		ID:         r.ID,
		OrgID:      r.OrgID,
		FirstName:  r.FirstName,
		LastName:   r.LastName,
		Email:      r.Email,
		Department: textToString(r.Department),
		Role:       textToString(r.Role),
		StartDate:  dateToString(r.StartDate),
		EndDate:    dateToString(r.EndDate),
		Status:     r.Status,
		Notes:      textToString(r.Notes),
		CreatedAt:  tsToTime(r.CreatedAt),
		UpdatedAt:  tsToTime(r.UpdatedAt),
	}
}

func checklistFromRow(r db.HrChecklists) Checklist {
	items := []ChecklistItem{}
	if len(r.Items) > 0 {
		_ = json.Unmarshal(r.Items, &items)
	}
	return Checklist{
		ID:        r.ID,
		OrgID:     r.OrgID,
		Type:      r.Type,
		Name:      r.Name,
		Items:     items,
		CreatedAt: tsToTime(r.CreatedAt),
		UpdatedAt: tsToTime(r.UpdatedAt),
	}
}

func runFromRow(r db.HrChecklistRuns) ChecklistRun {
	completed := []string{}
	if len(r.CompletedItems) > 0 {
		_ = json.Unmarshal(r.CompletedItems, &completed)
	}
	return ChecklistRun{
		ID:             r.ID,
		OrgID:          r.OrgID,
		EmployeeID:     r.EmployeeID,
		ChecklistID:    r.ChecklistID,
		Status:         r.Status,
		CompletedItems: completed,
		StartedAt:      tsToTime(r.StartedAt),
		CompletedAt:    tsToTimePtr(r.CompletedAt),
		CreatedAt:      tsToTime(r.CreatedAt),
		UpdatedAt:      tsToTime(r.UpdatedAt),
	}
}

func runEventFromRow(r db.HrRunEvents) RunEvent {
	return RunEvent{
		ID:          r.ID,
		RunID:       r.RunID,
		OrgID:       r.OrgID,
		StepID:      r.StepID,
		CompletedBy: r.CompletedBy,
		CompletedAt: tsToTime(r.CompletedAt),
	}
}

// --- Employees ---

func (r *Repository) ListEmployees(ctx context.Context, orgID string) ([]Employee, error) {
	rows, err := r.q.ListHREmployees(ctx, db.ListHREmployeesParams{
		OrgID: orgID, Limit: 1000, Offset: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("list employees: %w", err)
	}
	out := make([]Employee, 0, len(rows))
	for _, row := range rows {
		out = append(out, employeeFromRow(row))
	}
	return out, nil
}

func (r *Repository) ListEmployeesPaged(ctx context.Context, orgID string, offset, limit int) ([]Employee, int, error) {
	total, err := r.q.CountHREmployees(ctx, orgID)
	if err != nil {
		return nil, 0, fmt.Errorf("count employees: %w", err)
	}
	rows, err := r.q.ListHREmployees(ctx, db.ListHREmployeesParams{
		OrgID: orgID, Limit: int32(limit), Offset: int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list employees paged: %w", err)
	}
	out := make([]Employee, 0, len(rows))
	for _, row := range rows {
		out = append(out, employeeFromRow(row))
	}
	return out, int(total), nil
}

func (r *Repository) GetEmployee(ctx context.Context, orgID, id string) (*Employee, error) {
	row, err := r.q.GetHREmployee(ctx, db.GetHREmployeeParams{OrgID: orgID, ID: id})
	if err != nil {
		return nil, fmt.Errorf("get employee: %w", err)
	}
	e := employeeFromRow(row)
	return &e, nil
}

func (r *Repository) CreateEmployee(ctx context.Context, orgID string, in CreateEmployeeInput) (*Employee, error) {
	startDate, err := optDate(in.StartDate)
	if err != nil {
		return nil, err
	}
	row, err := r.q.CreateHREmployee(ctx, db.CreateHREmployeeParams{
		OrgID:      orgID,
		FirstName:  in.FirstName,
		LastName:   in.LastName,
		Email:      in.Email,
		Department: optText(in.Department),
		Role:       optText(in.Role),
		StartDate:  startDate,
		Notes:      optText(in.Notes),
	})
	if err != nil {
		return nil, fmt.Errorf("create employee: %w", err)
	}
	e := employeeFromRow(row)
	return &e, nil
}

func (r *Repository) UpdateEmployee(ctx context.Context, orgID, id string, in UpdateEmployeeInput) (*Employee, error) {
	endDate, err := optDate(in.EndDate)
	if err != nil {
		return nil, err
	}
	row, err := r.q.UpdateHREmployee(ctx, db.UpdateHREmployeeParams{
		OrgID:      orgID,
		ID:         id,
		FirstName:  in.FirstName,
		LastName:   in.LastName,
		Department: optText(in.Department),
		Role:       optText(in.Role),
		EndDate:    endDate,
		Status:     in.Status,
		Notes:      optText(in.Notes),
	})
	if err != nil {
		return nil, fmt.Errorf("update employee: %w", err)
	}
	e := employeeFromRow(row)
	return &e, nil
}

func (r *Repository) SetEmployeeStatus(ctx context.Context, orgID, id, status string) error {
	return r.q.SetHREmployeeStatus(ctx, db.SetHREmployeeStatusParams{
		OrgID: orgID, ID: id, Status: status,
	})
}

func (r *Repository) DeleteEmployee(ctx context.Context, orgID, id string) error {
	return r.q.DeleteHREmployee(ctx, db.DeleteHREmployeeParams{OrgID: orgID, ID: id})
}

// --- Checklists ---

func (r *Repository) ListChecklists(ctx context.Context, orgID string) ([]Checklist, error) {
	rows, err := r.q.ListHRChecklists(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list checklists: %w", err)
	}
	out := make([]Checklist, 0, len(rows))
	for _, row := range rows {
		out = append(out, checklistFromRow(row))
	}
	return out, nil
}

func (r *Repository) GetChecklist(ctx context.Context, orgID, id string) (*Checklist, error) {
	row, err := r.q.GetHRChecklist(ctx, db.GetHRChecklistParams{OrgID: orgID, ID: id})
	if err != nil {
		return nil, fmt.Errorf("get checklist: %w", err)
	}
	c := checklistFromRow(row)
	return &c, nil
}

func (r *Repository) CreateChecklist(ctx context.Context, orgID string, in CreateChecklistInput) (*Checklist, error) {
	if in.Items == nil {
		in.Items = []ChecklistItem{}
	}
	itemsJSON, err := json.Marshal(in.Items)
	if err != nil {
		return nil, fmt.Errorf("marshal checklist items: %w", err)
	}
	row, err := r.q.CreateHRChecklist(ctx, db.CreateHRChecklistParams{
		OrgID: orgID, Type: in.Type, Name: in.Name, Items: itemsJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("create checklist: %w", err)
	}
	c := checklistFromRow(row)
	return &c, nil
}

func (r *Repository) DeleteChecklist(ctx context.Context, orgID, id string) error {
	return r.q.DeleteHRChecklist(ctx, db.DeleteHRChecklistParams{OrgID: orgID, ID: id})
}

// FirstChecklistByType returns the oldest checklist of the given type ('onboarding'|'offboarding')
// for an organisation. Returns nil, nil if none exists.
func (r *Repository) FirstChecklistByType(ctx context.Context, orgID, checklistType string) (*Checklist, error) {
	row, err := r.q.FirstHRChecklistByType(ctx, db.FirstHRChecklistByTypeParams{
		OrgID: orgID, Type: checklistType,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("first checklist by type %s: %w", checklistType, err)
	}
	c := checklistFromRow(row)
	return &c, nil
}

// FirstOnboardingChecklist returns the first onboarding checklist for an organisation, or nil.
func (r *Repository) FirstOnboardingChecklist(ctx context.Context, orgID string) (*Checklist, error) {
	return r.FirstChecklistByType(ctx, orgID, "onboarding")
}

// FirstOffboardingChecklist returns the first offboarding checklist for an organisation, or nil.
func (r *Repository) FirstOffboardingChecklist(ctx context.Context, orgID string) (*Checklist, error) {
	return r.FirstChecklistByType(ctx, orgID, "offboarding")
}

// --- Checklist Runs ---

func (r *Repository) StartChecklistRun(ctx context.Context, orgID string, in StartChecklistRunInput) (*ChecklistRun, error) {
	row, err := r.q.StartHRChecklistRun(ctx, db.StartHRChecklistRunParams{
		OrgID: orgID, EmployeeID: in.EmployeeID, ChecklistID: in.ChecklistID,
	})
	if err != nil {
		return nil, fmt.Errorf("start checklist run: %w", err)
	}
	run := runFromRow(row)
	return &run, nil
}

func (r *Repository) GetChecklistRun(ctx context.Context, orgID, id string) (*ChecklistRun, error) {
	row, err := r.q.GetHRChecklistRun(ctx, db.GetHRChecklistRunParams{OrgID: orgID, ID: id})
	if err != nil {
		return nil, fmt.Errorf("get checklist run: %w", err)
	}
	run := runFromRow(row)
	return &run, nil
}

func (r *Repository) ListChecklistRuns(ctx context.Context, orgID, employeeID string) ([]ChecklistRun, error) {
	rows, err := r.q.ListHRChecklistRuns(ctx, db.ListHRChecklistRunsParams{
		OrgID: orgID, EmployeeID: employeeID,
	})
	if err != nil {
		return nil, fmt.Errorf("list checklist runs: %w", err)
	}
	out := make([]ChecklistRun, 0, len(rows))
	for _, row := range rows {
		out = append(out, runFromRow(row))
	}
	return out, nil
}

func (r *Repository) UpdateChecklistRun(ctx context.Context, orgID, id string, in UpdateChecklistRunInput) (*ChecklistRun, error) {
	if in.CompletedItems == nil {
		in.CompletedItems = []string{}
	}
	completedJSON, err := json.Marshal(in.CompletedItems)
	if err != nil {
		return nil, fmt.Errorf("marshal completed items: %w", err)
	}

	var completedAt *time.Time
	if in.Status == "completed" {
		now := time.Now().UTC()
		completedAt = &now
	}

	row, err := r.q.UpdateHRChecklistRun(ctx, db.UpdateHRChecklistRunParams{
		OrgID:          orgID,
		ID:             id,
		CompletedItems: completedJSON,
		Status:         in.Status,
		CompletedAt:    optTimestamptz(completedAt),
	})
	if err != nil {
		return nil, fmt.Errorf("update checklist run: %w", err)
	}
	run := runFromRow(row)
	return &run, nil
}

// --- Run Events ---

func (r *Repository) InsertRunEvent(ctx context.Context, runID, orgID, stepID, completedBy string) error {
	return r.q.InsertHRRunEvent(ctx, db.InsertHRRunEventParams{
		RunID: runID, OrgID: orgID, StepID: stepID, CompletedBy: completedBy,
	})
}

func (r *Repository) ListRunEvents(ctx context.Context, orgID, runID string) ([]RunEvent, error) {
	rows, err := r.q.ListHRRunEvents(ctx, db.ListHRRunEventsParams{
		OrgID: orgID, RunID: runID,
	})
	if err != nil {
		return nil, fmt.Errorf("list run events: %w", err)
	}
	out := make([]RunEvent, 0, len(rows))
	for _, row := range rows {
		out = append(out, runEventFromRow(row))
	}
	return out, nil
}

// RevokeUserSessions revokes all active sessions for the platform user
// matching the given email within the org.
func (r *Repository) RevokeUserSessions(ctx context.Context, orgID, email string) error {
	return r.q.HRRevokeUserSessions(ctx, db.HRRevokeUserSessionsParams{OrgID: orgID, Email: email})
}

// DisableUser sets the platform user's status to 'disabled'.
func (r *Repository) DisableUser(ctx context.Context, orgID, email string) error {
	return r.q.HRDisableUser(ctx, db.HRDisableUserParams{OrgID: orgID, Email: email})
}

// RevokeUserAPIKeys revokes all active API keys for the platform user
// matching the given email within the org.
func (r *Repository) RevokeUserAPIKeys(ctx context.Context, orgID, email string) error {
	return r.q.HRRevokeUserAPIKeys(ctx, db.HRRevokeUserAPIKeysParams{OrgID: orgID, Email: email})
}

// ListEmployeesCursor returns employees for orgID using keyset pagination on (created_at DESC, id DESC).
func (r *Repository) ListEmployeesCursor(ctx context.Context, orgID string, cursorID string, cursorTS time.Time, limit int) ([]Employee, error) {
	args := []any{orgID}
	q := `SELECT id, org_id, first_name, last_name, email, department, role,
	             start_date, end_date, status, notes, created_at, updated_at
	      FROM hr_employees
	      WHERE org_id = $1`
	if !cursorTS.IsZero() {
		q += ` AND (created_at < $2 OR (created_at = $2 AND id::text < $3))`
		args = append(args, cursorTS, cursorID)
	}
	q += ` ORDER BY created_at DESC, id DESC LIMIT $` + strconv.Itoa(len(args)+1)
	args = append(args, limit+1)
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list employees cursor: %w", err)
	}
	defer rows.Close()
	var out []Employee
	for rows.Next() {
		var id, orgID, firstName, lastName, email, status string
		var department, role, notes pgtype.Text
		var startDate, endDate pgtype.Date
		var createdAt, updatedAt pgtype.Timestamptz
		if err := rows.Scan(&id, &orgID, &firstName, &lastName, &email, &department, &role,
			&startDate, &endDate, &status, &notes, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan employee cursor row: %w", err)
		}
		out = append(out, Employee{
			ID:         id,
			OrgID:      orgID,
			FirstName:  firstName,
			LastName:   lastName,
			Email:      email,
			Department: textToString(department),
			Role:       textToString(role),
			StartDate:  dateToString(startDate),
			EndDate:    dateToString(endDate),
			Status:     status,
			Notes:      textToString(notes),
			CreatedAt:  tsToTime(createdAt),
			UpdatedAt:  tsToTime(updatedAt),
		})
	}
	return out, rows.Err()
}

// GetEmployeePersonioFields returns personio_employee_id and departure_date for an employee.
// Returns (0, zero, nil) if not set.
func (r *Repository) GetEmployeePersonioFields(ctx context.Context, orgID, employeeID string) (personioID int, departureDate time.Time, err error) {
	var pID pgtype.Int4
	var dd pgtype.Date
	err = r.db.QueryRow(ctx, `
		SELECT personio_employee_id, departure_date
		FROM hr_employees
		WHERE org_id = $1::uuid AND id = $2::uuid`,
		orgID, employeeID,
	).Scan(&pID, &dd)
	if err != nil {
		return 0, time.Time{}, err
	}
	if pID.Valid {
		personioID = int(pID.Int32)
	}
	if dd.Valid {
		departureDate = dd.Time
	}
	return personioID, departureDate, nil
}

// UpsertEmployeeByPersonioID inserts or updates an hr_employees row for the given
// Personio employee ID. Returns the Vakt employee UUID, whether a new row was created,
// and any error. Only personio_employee_id and departure_date are stored — no PII.
func (r *Repository) UpsertEmployeeByPersonioID(ctx context.Context, orgID string, personioEmployeeID int, departureDate time.Time) (employeeID string, created bool, err error) {
	// Try to find existing employee
	err = r.db.QueryRow(ctx, `
		SELECT id::text FROM hr_employees
		WHERE org_id = $1::uuid AND personio_employee_id = $2`,
		orgID, personioEmployeeID,
	).Scan(&employeeID)

	if err == nil {
		// Found — update departure_date
		_, err = r.db.Exec(ctx, `
			UPDATE hr_employees
			SET departure_date = $1, updated_at = NOW()
			WHERE org_id = $2::uuid AND personio_employee_id = $3`,
			departureDate.Format("2006-01-02"), orgID, personioEmployeeID,
		)
		return employeeID, false, err
	}

	// Not found — create placeholder (no name or email)
	err = r.db.QueryRow(ctx, `
		INSERT INTO hr_employees
			(org_id, first_name, last_name, email, status, personio_employee_id, departure_date)
		VALUES
			($1::uuid, '', '', '', 'offboarding', $2, $3)
		RETURNING id::text`,
		orgID, personioEmployeeID, departureDate.Format("2006-01-02"),
	).Scan(&employeeID)
	if err != nil {
		return "", false, fmt.Errorf("create placeholder employee for personio_id %d: %w", personioEmployeeID, err)
	}
	return employeeID, true, nil
}

// --- S69-4: JML Mover Workflow ---

func (r *Repository) CreateMoverEvent(ctx context.Context, orgID, initiatedBy string, in CreateMoverEventInput, effectiveDate, dueDate time.Time) (*MoverEvent, error) {
	var initiatedByP *string
	if initiatedBy != "" {
		initiatedByP = &initiatedBy
	}
	var fromDept, fromTitle *string
	if in.FromDepartment != "" {
		fromDept = &in.FromDepartment
	}
	if in.FromJobTitle != "" {
		fromTitle = &in.FromJobTitle
	}

	var ev MoverEvent
	var completedAt *time.Time
	var checklistRunID, initiatedByOut *string
	err := r.db.QueryRow(ctx, `
		INSERT INTO hr_mover_events
			(org_id, employee_id, from_department, from_job_title, to_department, to_job_title,
			 effective_date, initiated_by, due_date)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8::uuid, $9)
		RETURNING id::text, org_id::text, employee_id::text,
		          COALESCE(from_department,''), COALESCE(from_job_title,''),
		          to_department, to_job_title,
		          effective_date, initiated_by::text, checklist_run_id::text,
		          status, due_date, completed_at, created_at`,
		orgID, in.EmployeeID, fromDept, fromTitle, in.ToDepartment, in.ToJobTitle,
		effectiveDate, initiatedByP, dueDate,
	).Scan(
		&ev.ID, &ev.OrgID, &ev.EmployeeID,
		&ev.FromDepartment, &ev.FromJobTitle,
		&ev.ToDepartment, &ev.ToJobTitle,
		&ev.EffectiveDate, &initiatedByOut, &checklistRunID,
		&ev.Status, &ev.DueDate, &completedAt, &ev.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create mover event: %w", err)
	}
	ev.InitiatedBy = initiatedByOut
	ev.ChecklistRunID = checklistRunID
	ev.CompletedAt = completedAt
	return &ev, nil
}

func (r *Repository) ListMoverEvents(ctx context.Context, orgID string) ([]MoverEvent, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text, employee_id::text,
		       COALESCE(from_department,''), COALESCE(from_job_title,''),
		       to_department, to_job_title,
		       effective_date, initiated_by::text, checklist_run_id::text,
		       status, due_date, completed_at, created_at
		FROM hr_mover_events
		WHERE org_id = $1::uuid
		ORDER BY created_at DESC`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list mover events: %w", err)
	}
	defer rows.Close()

	var out []MoverEvent
	for rows.Next() {
		var ev MoverEvent
		var completedAt *time.Time
		var initiated, checklist *string
		if err := rows.Scan(
			&ev.ID, &ev.OrgID, &ev.EmployeeID,
			&ev.FromDepartment, &ev.FromJobTitle,
			&ev.ToDepartment, &ev.ToJobTitle,
			&ev.EffectiveDate, &initiated, &checklist,
			&ev.Status, &ev.DueDate, &completedAt, &ev.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan mover event: %w", err)
		}
		ev.InitiatedBy = initiated
		ev.ChecklistRunID = checklist
		ev.CompletedAt = completedAt
		out = append(out, ev)
	}
	return out, rows.Err()
}

func (r *Repository) GetMoverEvent(ctx context.Context, orgID, id string) (*MoverEvent, error) {
	var ev MoverEvent
	var completedAt *time.Time
	var initiated, checklist *string
	err := r.db.QueryRow(ctx, `
		SELECT id::text, org_id::text, employee_id::text,
		       COALESCE(from_department,''), COALESCE(from_job_title,''),
		       to_department, to_job_title,
		       effective_date, initiated_by::text, checklist_run_id::text,
		       status, due_date, completed_at, created_at
		FROM hr_mover_events
		WHERE id = $1::uuid AND org_id = $2::uuid`,
		id, orgID,
	).Scan(
		&ev.ID, &ev.OrgID, &ev.EmployeeID,
		&ev.FromDepartment, &ev.FromJobTitle,
		&ev.ToDepartment, &ev.ToJobTitle,
		&ev.EffectiveDate, &initiated, &checklist,
		&ev.Status, &ev.DueDate, &completedAt, &ev.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get mover event: %w", err)
	}
	ev.InitiatedBy = initiated
	ev.ChecklistRunID = checklist
	ev.CompletedAt = completedAt
	return &ev, nil
}

func (r *Repository) UpdateMoverEventStatus(ctx context.Context, orgID, id, status string) (*MoverEvent, error) {
	var completedAt *time.Time
	if status == "completed" {
		t := time.Now()
		completedAt = &t
	}
	var ev MoverEvent
	var initiated, checklist *string
	err := r.db.QueryRow(ctx, `
		UPDATE hr_mover_events
		SET status = $1, completed_at = $2
		WHERE id = $3::uuid AND org_id = $4::uuid
		RETURNING id::text, org_id::text, employee_id::text,
		          COALESCE(from_department,''), COALESCE(from_job_title,''),
		          to_department, to_job_title,
		          effective_date, initiated_by::text, checklist_run_id::text,
		          status, due_date, completed_at, created_at`,
		status, completedAt, id, orgID,
	).Scan(
		&ev.ID, &ev.OrgID, &ev.EmployeeID,
		&ev.FromDepartment, &ev.FromJobTitle,
		&ev.ToDepartment, &ev.ToJobTitle,
		&ev.EffectiveDate, &initiated, &checklist,
		&ev.Status, &ev.DueDate, &completedAt, &ev.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("update mover event status: %w", err)
	}
	ev.InitiatedBy = initiated
	ev.ChecklistRunID = checklist
	ev.CompletedAt = completedAt
	return &ev, nil
}

func (r *Repository) ListMoverTemplates(ctx context.Context, orgID string) ([]MoverTemplate, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text, name,
		       COALESCE(from_role_hint,''), COALESCE(to_role_hint,''),
		       is_default, created_at
		FROM hr_mover_templates WHERE org_id = $1::uuid ORDER BY name`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list mover templates: %w", err)
	}
	defer rows.Close()

	var out []MoverTemplate
	for rows.Next() {
		var t MoverTemplate
		if err := rows.Scan(&t.ID, &t.OrgID, &t.Name, &t.FromRoleHint, &t.ToRoleHint, &t.IsDefault, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan mover template: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *Repository) CreateMoverTemplate(ctx context.Context, orgID, name, fromRoleHint, toRoleHint string, isDefault bool) (string, error) {
	var id string
	err := r.db.QueryRow(ctx, `
		INSERT INTO hr_mover_templates (org_id, name, from_role_hint, to_role_hint, is_default)
		VALUES ($1::uuid, $2, NULLIF($3,''), NULLIF($4,''), $5)
		RETURNING id::text`,
		orgID, name, fromRoleHint, toRoleHint, isDefault,
	).Scan(&id)
	return id, err
}

func (r *Repository) CreateMoverTemplateItem(ctx context.Context, templateID, section, title, description, responsibleRole string, sortOrder int) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO hr_mover_template_items (template_id, section, title, description, responsible_role, sort_order)
		VALUES ($1::uuid, $2, $3, NULLIF($4,''), NULLIF($5,''), $6)`,
		templateID, section, title, description, responsibleRole, sortOrder,
	)
	return err
}

func (r *Repository) ListMoverTemplateItems(ctx context.Context, templateID string) ([]MoverTemplateItem, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, template_id::text, section, title,
		       COALESCE(description,''), COALESCE(responsible_role,''), sort_order
		FROM hr_mover_template_items WHERE template_id = $1::uuid ORDER BY section, sort_order`,
		templateID,
	)
	if err != nil {
		return nil, fmt.Errorf("list mover template items: %w", err)
	}
	defer rows.Close()

	var out []MoverTemplateItem
	for rows.Next() {
		var item MoverTemplateItem
		if err := rows.Scan(&item.ID, &item.TemplateID, &item.Section, &item.Title, &item.Description, &item.ResponsibleRole, &item.SortOrder); err != nil {
			return nil, fmt.Errorf("scan mover template item: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
