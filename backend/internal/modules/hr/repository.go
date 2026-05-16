package hr

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles HR data access against PostgreSQL.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new HR repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// --- Employees ---

// ListEmployees returns all employees for an organisation, ordered newest first.
func (r *Repository) ListEmployees(ctx context.Context, orgID string) ([]Employee, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text, first_name, last_name, email,
		       COALESCE(department,''), COALESCE(role,''),
		       to_char(start_date,'YYYY-MM-DD'), to_char(end_date,'YYYY-MM-DD'),
		       status, COALESCE(notes,''), created_at, updated_at
		FROM hr_employees
		WHERE org_id = $1::uuid
		ORDER BY created_at DESC`, orgID)
	if err != nil {
		return nil, fmt.Errorf("list employees: %w", err)
	}
	defer rows.Close()

	var employees []Employee
	for rows.Next() {
		var e Employee
		var startDate, endDate *string
		if err := rows.Scan(
			&e.ID, &e.OrgID, &e.FirstName, &e.LastName, &e.Email,
			&e.Department, &e.Role,
			&startDate, &endDate,
			&e.Status, &e.Notes, &e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan employee: %w", err)
		}
		if startDate != nil && *startDate != "" {
			e.StartDate = startDate
		}
		if endDate != nil && *endDate != "" {
			e.EndDate = endDate
		}
		employees = append(employees, e)
	}
	return employees, rows.Err()
}

// GetEmployee returns a single employee by org and ID.
func (r *Repository) GetEmployee(ctx context.Context, orgID, id string) (*Employee, error) {
	var e Employee
	var startDate, endDate *string
	err := r.db.QueryRow(ctx, `
		SELECT id::text, org_id::text, first_name, last_name, email,
		       COALESCE(department,''), COALESCE(role,''),
		       to_char(start_date,'YYYY-MM-DD'), to_char(end_date,'YYYY-MM-DD'),
		       status, COALESCE(notes,''), created_at, updated_at
		FROM hr_employees
		WHERE org_id = $1::uuid AND id = $2::uuid`, orgID, id,
	).Scan(
		&e.ID, &e.OrgID, &e.FirstName, &e.LastName, &e.Email,
		&e.Department, &e.Role,
		&startDate, &endDate,
		&e.Status, &e.Notes, &e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get employee: %w", err)
	}
	if startDate != nil && *startDate != "" {
		e.StartDate = startDate
	}
	if endDate != nil && *endDate != "" {
		e.EndDate = endDate
	}
	return &e, nil
}

// CreateEmployee inserts a new employee and returns the persisted record.
func (r *Repository) CreateEmployee(ctx context.Context, orgID string, in CreateEmployeeInput) (*Employee, error) {
	var startDate *string
	if in.StartDate != "" {
		startDate = &in.StartDate
	}
	var e Employee
	var sd, ed *string
	err := r.db.QueryRow(ctx, `
		INSERT INTO hr_employees (org_id, first_name, last_name, email, department, role, start_date, notes)
		VALUES ($1::uuid, $2, $3, $4, $5, $6, $7::date, $8)
		RETURNING id::text, org_id::text, first_name, last_name, email,
		          COALESCE(department,''), COALESCE(role,''),
		          to_char(start_date,'YYYY-MM-DD'), to_char(end_date,'YYYY-MM-DD'),
		          status, COALESCE(notes,''), created_at, updated_at`,
		orgID, in.FirstName, in.LastName, in.Email,
		in.Department, in.Role, startDate, in.Notes,
	).Scan(
		&e.ID, &e.OrgID, &e.FirstName, &e.LastName, &e.Email,
		&e.Department, &e.Role,
		&sd, &ed,
		&e.Status, &e.Notes, &e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create employee: %w", err)
	}
	if sd != nil && *sd != "" {
		e.StartDate = sd
	}
	if ed != nil && *ed != "" {
		e.EndDate = ed
	}
	return &e, nil
}

// UpdateEmployee updates an existing employee record.
func (r *Repository) UpdateEmployee(ctx context.Context, orgID, id string, in UpdateEmployeeInput) (*Employee, error) {
	var endDate *string
	if in.EndDate != "" {
		endDate = &in.EndDate
	}
	var e Employee
	var sd, ed *string
	err := r.db.QueryRow(ctx, `
		UPDATE hr_employees
		SET first_name = $3, last_name = $4, department = $5, role = $6,
		    end_date = $7::date, status = $8, notes = $9, updated_at = now()
		WHERE org_id = $1::uuid AND id = $2::uuid
		RETURNING id::text, org_id::text, first_name, last_name, email,
		          COALESCE(department,''), COALESCE(role,''),
		          to_char(start_date,'YYYY-MM-DD'), to_char(end_date,'YYYY-MM-DD'),
		          status, COALESCE(notes,''), created_at, updated_at`,
		orgID, id,
		in.FirstName, in.LastName, in.Department, in.Role,
		endDate, in.Status, in.Notes,
	).Scan(
		&e.ID, &e.OrgID, &e.FirstName, &e.LastName, &e.Email,
		&e.Department, &e.Role,
		&sd, &ed,
		&e.Status, &e.Notes, &e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("update employee: %w", err)
	}
	if sd != nil && *sd != "" {
		e.StartDate = sd
	}
	if ed != nil && *ed != "" {
		e.EndDate = ed
	}
	return &e, nil
}

// DeleteEmployee removes an employee record.
func (r *Repository) DeleteEmployee(ctx context.Context, orgID, id string) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM hr_employees WHERE org_id = $1::uuid AND id = $2::uuid`, orgID, id)
	if err != nil {
		return fmt.Errorf("delete employee: %w", err)
	}
	return nil
}

// --- Checklists ---

// ListChecklists returns all checklist templates for an organisation.
func (r *Repository) ListChecklists(ctx context.Context, orgID string) ([]Checklist, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text, type, name, items, created_at, updated_at
		FROM hr_checklists
		WHERE org_id = $1::uuid
		ORDER BY created_at DESC`, orgID)
	if err != nil {
		return nil, fmt.Errorf("list checklists: %w", err)
	}
	defer rows.Close()

	var checklists []Checklist
	for rows.Next() {
		var c Checklist
		var itemsRaw []byte
		if err := rows.Scan(&c.ID, &c.OrgID, &c.Type, &c.Name, &itemsRaw, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan checklist: %w", err)
		}
		if err := json.Unmarshal(itemsRaw, &c.Items); err != nil {
			c.Items = []ChecklistItem{}
		}
		checklists = append(checklists, c)
	}
	return checklists, rows.Err()
}

// CreateChecklist inserts a new checklist template and returns the persisted record.
func (r *Repository) CreateChecklist(ctx context.Context, orgID string, in CreateChecklistInput) (*Checklist, error) {
	if in.Items == nil {
		in.Items = []ChecklistItem{}
	}
	itemsJSON, err := json.Marshal(in.Items)
	if err != nil {
		return nil, fmt.Errorf("marshal checklist items: %w", err)
	}
	var c Checklist
	var itemsRaw []byte
	err = r.db.QueryRow(ctx, `
		INSERT INTO hr_checklists (org_id, type, name, items)
		VALUES ($1::uuid, $2, $3, $4)
		RETURNING id::text, org_id::text, type, name, items, created_at, updated_at`,
		orgID, in.Type, in.Name, itemsJSON,
	).Scan(&c.ID, &c.OrgID, &c.Type, &c.Name, &itemsRaw, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create checklist: %w", err)
	}
	if err := json.Unmarshal(itemsRaw, &c.Items); err != nil {
		c.Items = []ChecklistItem{}
	}
	return &c, nil
}

// DeleteChecklist removes a checklist template.
func (r *Repository) DeleteChecklist(ctx context.Context, orgID, id string) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM hr_checklists WHERE org_id = $1::uuid AND id = $2::uuid`, orgID, id)
	if err != nil {
		return fmt.Errorf("delete checklist: %w", err)
	}
	return nil
}

// --- Checklist Runs ---

// StartChecklistRun creates a new checklist run for an employee.
func (r *Repository) StartChecklistRun(ctx context.Context, orgID string, in StartChecklistRunInput) (*ChecklistRun, error) {
	var run ChecklistRun
	var completedRaw []byte
	err := r.db.QueryRow(ctx, `
		INSERT INTO hr_checklist_runs (org_id, employee_id, checklist_id)
		VALUES ($1::uuid, $2::uuid, $3::uuid)
		RETURNING id::text, org_id::text, employee_id::text, checklist_id::text,
		          status, completed_items, started_at, completed_at, created_at, updated_at`,
		orgID, in.EmployeeID, in.ChecklistID,
	).Scan(
		&run.ID, &run.OrgID, &run.EmployeeID, &run.ChecklistID,
		&run.Status, &completedRaw, &run.StartedAt, &run.CompletedAt,
		&run.CreatedAt, &run.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("start checklist run: %w", err)
	}
	if err := json.Unmarshal(completedRaw, &run.CompletedItems); err != nil {
		run.CompletedItems = []string{}
	}
	return &run, nil
}

// GetChecklistRun returns a single checklist run by org and ID.
func (r *Repository) GetChecklistRun(ctx context.Context, orgID, id string) (*ChecklistRun, error) {
	var run ChecklistRun
	var completedRaw []byte
	err := r.db.QueryRow(ctx, `
		SELECT id::text, org_id::text, employee_id::text, checklist_id::text,
		       status, completed_items, started_at, completed_at, created_at, updated_at
		FROM hr_checklist_runs
		WHERE org_id = $1::uuid AND id = $2::uuid`, orgID, id,
	).Scan(
		&run.ID, &run.OrgID, &run.EmployeeID, &run.ChecklistID,
		&run.Status, &completedRaw, &run.StartedAt, &run.CompletedAt,
		&run.CreatedAt, &run.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get checklist run: %w", err)
	}
	if err := json.Unmarshal(completedRaw, &run.CompletedItems); err != nil {
		run.CompletedItems = []string{}
	}
	return &run, nil
}

// ListChecklistRuns returns all checklist runs for a specific employee.
func (r *Repository) ListChecklistRuns(ctx context.Context, orgID, employeeID string) ([]ChecklistRun, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text, employee_id::text, checklist_id::text,
		       status, completed_items, started_at, completed_at, created_at, updated_at
		FROM hr_checklist_runs
		WHERE org_id = $1::uuid AND employee_id = $2::uuid
		ORDER BY started_at DESC`, orgID, employeeID)
	if err != nil {
		return nil, fmt.Errorf("list checklist runs: %w", err)
	}
	defer rows.Close()

	var runs []ChecklistRun
	for rows.Next() {
		var run ChecklistRun
		var completedRaw []byte
		if err := rows.Scan(
			&run.ID, &run.OrgID, &run.EmployeeID, &run.ChecklistID,
			&run.Status, &completedRaw, &run.StartedAt, &run.CompletedAt,
			&run.CreatedAt, &run.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan checklist run: %w", err)
		}
		if err := json.Unmarshal(completedRaw, &run.CompletedItems); err != nil {
			run.CompletedItems = []string{}
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

// UpdateChecklistRun updates the progress of a checklist run.
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

	var run ChecklistRun
	var completedRaw []byte
	err = r.db.QueryRow(ctx, `
		UPDATE hr_checklist_runs
		SET completed_items = $3, status = $4, completed_at = $5, updated_at = now()
		WHERE org_id = $1::uuid AND id = $2::uuid
		RETURNING id::text, org_id::text, employee_id::text, checklist_id::text,
		          status, completed_items, started_at, completed_at, created_at, updated_at`,
		orgID, id, completedJSON, in.Status, completedAt,
	).Scan(
		&run.ID, &run.OrgID, &run.EmployeeID, &run.ChecklistID,
		&run.Status, &completedRaw, &run.StartedAt, &run.CompletedAt,
		&run.CreatedAt, &run.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("update checklist run: %w", err)
	}
	if err := json.Unmarshal(completedRaw, &run.CompletedItems); err != nil {
		run.CompletedItems = []string{}
	}
	return &run, nil
}

// FirstOnboardingChecklist returns the first onboarding checklist for an organisation.
// Returns nil, nil if none exists.
func (r *Repository) FirstOnboardingChecklist(ctx context.Context, orgID string) (*Checklist, error) {
	var c Checklist
	var itemsRaw []byte
	err := r.db.QueryRow(ctx, `
		SELECT id::text, org_id::text, type, name, items, created_at, updated_at
		FROM hr_checklists
		WHERE org_id = $1::uuid AND type = 'onboarding'
		ORDER BY created_at ASC
		LIMIT 1`, orgID,
	).Scan(&c.ID, &c.OrgID, &c.Type, &c.Name, &itemsRaw, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, nil //nolint:nilerr // no onboarding checklist is a valid state
	}
	if err := json.Unmarshal(itemsRaw, &c.Items); err != nil {
		c.Items = []ChecklistItem{}
	}
	return &c, nil
}

// ListEmployeesPaged returns a page of employees plus the total count.
func (r *Repository) ListEmployeesPaged(ctx context.Context, orgID string, offset, limit int) ([]Employee, int, error) {
	var total int
	if err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM hr_employees WHERE org_id = $1::uuid`, orgID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count employees: %w", err)
	}

	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text, first_name, last_name, email,
		       COALESCE(department,''), COALESCE(role,''),
		       to_char(start_date,'YYYY-MM-DD'), to_char(end_date,'YYYY-MM-DD'),
		       status, COALESCE(notes,''), created_at, updated_at
		FROM hr_employees
		WHERE org_id = $1::uuid
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`, orgID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list employees paged: %w", err)
	}
	defer rows.Close()

	var employees []Employee
	for rows.Next() {
		var e Employee
		var startDate, endDate *string
		if err := rows.Scan(
			&e.ID, &e.OrgID, &e.FirstName, &e.LastName, &e.Email,
			&e.Department, &e.Role,
			&startDate, &endDate,
			&e.Status, &e.Notes, &e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan employee paged: %w", err)
		}
		if startDate != nil && *startDate != "" {
			e.StartDate = startDate
		}
		if endDate != nil && *endDate != "" {
			e.EndDate = endDate
		}
		employees = append(employees, e)
	}
	return employees, total, rows.Err()
}
