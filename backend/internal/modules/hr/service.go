package hr

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// Service handles HR business logic.
type Service struct {
	repo *Repository
	db   *pgxpool.Pool
}

// NewService creates a new HR service backed by the given DB pool.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo, db: repo.db}
}

// NewServiceFromPool is a convenience constructor that creates the repository internally.
func NewServiceFromPool(db *pgxpool.Pool) *Service {
	return &Service{repo: NewRepository(db), db: db}
}

// --- Employees ---

// ListEmployees returns all employees for an organisation.
// Always returns a non-nil slice so the JSON response is [] rather than null.
func (s *Service) ListEmployees(ctx context.Context, orgID string) ([]Employee, error) {
	employees, err := s.repo.ListEmployees(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list employees: %w", err)
	}
	if employees == nil {
		employees = []Employee{}
	}
	return employees, nil
}

// GetEmployee returns a single employee by org and ID.
func (s *Service) GetEmployee(ctx context.Context, orgID, id string) (*Employee, error) {
	return s.repo.GetEmployee(ctx, orgID, id)
}

// CreateEmployee validates and creates a new employee record.
func (s *Service) CreateEmployee(ctx context.Context, orgID string, in CreateEmployeeInput) (*Employee, error) {
	return s.repo.CreateEmployee(ctx, orgID, in)
}

// UpdateEmployee updates an existing employee record.
// When status transitions to "terminated", the corresponding platform user's
// sessions and API keys are revoked immediately to fulfil the SecHR compliance promise.
func (s *Service) UpdateEmployee(ctx context.Context, orgID, id string, in UpdateEmployeeInput) (*Employee, error) {
	emp, err := s.repo.UpdateEmployee(ctx, orgID, id, in)
	if err != nil {
		return nil, err
	}
	if in.Status == "terminated" {
		s.revokeUserAccess(ctx, orgID, emp.Email)
	}
	return emp, nil
}

// revokeUserAccess revokes all active sessions and API keys for the platform user
// matching the given email within the org. Errors are logged but do not fail the call —
// the HR record update is already committed and must not be rolled back due to a
// transient auth-DB issue.
func (s *Service) revokeUserAccess(ctx context.Context, orgID, email string) {
	if _, err := s.db.Exec(ctx,
		`UPDATE sessions SET revoked_at = NOW()
		 FROM users
		 WHERE sessions.user_id = users.id
		   AND users.org_id    = $1::uuid
		   AND users.email     = $2
		   AND sessions.revoked_at IS NULL`,
		orgID, email,
	); err != nil {
		log.Error().Err(err).Str("email", email).Msg("hr: revoke sessions on termination")
	}
	if _, err := s.db.Exec(ctx,
		`UPDATE users SET status = 'disabled'
		 WHERE org_id = $1::uuid AND email = $2`,
		orgID, email,
	); err != nil {
		log.Error().Err(err).Str("email", email).Msg("hr: disable user on termination")
	}
	if _, err := s.db.Exec(ctx,
		`UPDATE api_keys SET revoked_at = NOW()
		 FROM users
		 WHERE api_keys.created_by = users.id
		   AND users.org_id        = $1::uuid
		   AND users.email         = $2
		   AND api_keys.revoked_at IS NULL`,
		orgID, email,
	); err != nil {
		log.Error().Err(err).Str("email", email).Msg("hr: revoke api keys on termination")
	}
}

// DeleteEmployee removes an employee record.
func (s *Service) DeleteEmployee(ctx context.Context, orgID, id string) error {
	return s.repo.DeleteEmployee(ctx, orgID, id)
}

// ListEmployeesPaged returns a page of employees plus the total count.
func (s *Service) ListEmployeesPaged(ctx context.Context, orgID string, offset, limit int) ([]Employee, int, error) {
	employees, total, err := s.repo.ListEmployeesPaged(ctx, orgID, offset, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("list employees paged: %w", err)
	}
	if employees == nil {
		employees = []Employee{}
	}
	return employees, total, nil
}

// --- Checklists ---

// ListChecklists returns all checklist templates for an organisation.
// Always returns a non-nil slice.
func (s *Service) ListChecklists(ctx context.Context, orgID string) ([]Checklist, error) {
	checklists, err := s.repo.ListChecklists(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list checklists: %w", err)
	}
	if checklists == nil {
		checklists = []Checklist{}
	}
	return checklists, nil
}

// CreateChecklist creates a new checklist template.
func (s *Service) CreateChecklist(ctx context.Context, orgID string, in CreateChecklistInput) (*Checklist, error) {
	return s.repo.CreateChecklist(ctx, orgID, in)
}

// DeleteChecklist removes a checklist template.
func (s *Service) DeleteChecklist(ctx context.Context, orgID, id string) error {
	return s.repo.DeleteChecklist(ctx, orgID, id)
}

// --- Checklist Runs ---

// StartChecklistRun starts a new checklist run for an employee.
func (s *Service) StartChecklistRun(ctx context.Context, orgID string, in StartChecklistRunInput) (*ChecklistRun, error) {
	return s.repo.StartChecklistRun(ctx, orgID, in)
}

// GetChecklistRun returns a single checklist run.
func (s *Service) GetChecklistRun(ctx context.Context, orgID, id string) (*ChecklistRun, error) {
	return s.repo.GetChecklistRun(ctx, orgID, id)
}

// ListChecklistRuns returns all checklist runs for a specific employee.
// Always returns a non-nil slice.
func (s *Service) ListChecklistRuns(ctx context.Context, orgID, employeeID string) ([]ChecklistRun, error) {
	runs, err := s.repo.ListChecklistRuns(ctx, orgID, employeeID)
	if err != nil {
		return nil, fmt.Errorf("list checklist runs: %w", err)
	}
	if runs == nil {
		runs = []ChecklistRun{}
	}
	return runs, nil
}

// UpdateChecklistRun updates the progress of a checklist run.
func (s *Service) UpdateChecklistRun(ctx context.Context, orgID, id string, in UpdateChecklistRunInput) (*ChecklistRun, error) {
	return s.repo.UpdateChecklistRun(ctx, orgID, id, in)
}

// StartOnboarding finds the first onboarding checklist for the organisation and starts
// a run for the given employee. Returns an error if no onboarding checklist exists.
func (s *Service) StartOnboarding(ctx context.Context, orgID, employeeID string) (*ChecklistRun, error) {
	checklist, err := s.repo.FirstOnboardingChecklist(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("find onboarding checklist: %w", err)
	}
	if checklist == nil {
		return nil, errors.New("no onboarding checklist found for organisation")
	}
	return s.repo.StartChecklistRun(ctx, orgID, StartChecklistRunInput{
		EmployeeID:  employeeID,
		ChecklistID: checklist.ID,
	})
}
