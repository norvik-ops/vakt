package hr

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/shared/auditlog"
)

// Actor identifies who is performing a state-changing operation and from where.
// The service uses this to write audit-log entries — see docs/dev/service-pattern.md
// (ADR pattern P2-19): keeping audit-write in the service makes the audit
// trail intact for non-HTTP callers (workers, future CLI, SDK).
type Actor struct {
	OrgID     string
	UserID    string
	UserEmail string
	IPAddress string
}

// Service handles HR business logic.
type Service struct {
	repo     *Repository
	db       *pgxpool.Pool
	evidence EvidenceWriter
}

// audit is the single point where the HR service writes audit-log entries.
// Best-effort: a failure here is logged inside auditlog.Log but never aborts
// the calling operation.
func (s *Service) audit(ctx context.Context, actor Actor, action, resourceType, resourceID, resourceName string) {
	auditlog.Log(ctx, s.db, auditlog.Entry{
		OrgID:        actor.OrgID,
		UserID:       actor.UserID,
		UserEmail:    actor.UserEmail,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		ResourceName: resourceName,
		IPAddress:    actor.IPAddress,
	})
}

// NewService creates a new HR service backed by the given repository.
// The evidence writer defaults to a noop; use WithEvidenceWriter to inject the real one.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo, db: repo.db, evidence: NoopEvidenceWriter()}
}

// NewServiceFromPool is a convenience constructor that creates the repository internally.
func NewServiceFromPool(db *pgxpool.Pool) *Service {
	repo := NewRepository(db)
	return &Service{repo: repo, db: db, evidence: NoopEvidenceWriter()}
}

// WithEvidenceWriter injects the evidence writer used when checklist runs complete.
func (s *Service) WithEvidenceWriter(w EvidenceWriter) *Service {
	if w == nil {
		s.evidence = NoopEvidenceWriter()
	} else {
		s.evidence = w
	}
	return s
}

// --- Employees ---

// ListEmployees returns all employees for an organisation.
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

// CreateEmployee validates and creates a new employee record. Writes a
// "create" audit-log entry attributed to the caller.
func (s *Service) CreateEmployee(ctx context.Context, actor Actor, in CreateEmployeeInput) (*Employee, error) {
	emp, err := s.repo.CreateEmployee(ctx, actor.OrgID, in)
	if err != nil {
		return nil, err
	}
	s.audit(ctx, actor, "create", "hr/employee", emp.ID, emp.FirstName+" "+emp.LastName)
	return emp, nil
}

// UpdateEmployee updates an existing employee record.
// When status transitions to "terminated", the corresponding platform user's
// sessions and API keys are revoked immediately to fulfil the SecHR compliance promise.
func (s *Service) UpdateEmployee(ctx context.Context, actor Actor, id string, in UpdateEmployeeInput) (*Employee, error) {
	emp, err := s.repo.UpdateEmployee(ctx, actor.OrgID, id, in)
	if err != nil {
		return nil, err
	}
	if in.Status == "terminated" {
		s.revokeUserAccess(ctx, actor.OrgID, emp.Email)
	}
	s.audit(ctx, actor, "update", "hr/employee", emp.ID, emp.FirstName+" "+emp.LastName)
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
func (s *Service) DeleteEmployee(ctx context.Context, actor Actor, id string) error {
	if err := s.repo.DeleteEmployee(ctx, actor.OrgID, id); err != nil {
		return err
	}
	s.audit(ctx, actor, "delete", "hr/employee", id, "")
	return nil
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
func (s *Service) CreateChecklist(ctx context.Context, actor Actor, in CreateChecklistInput) (*Checklist, error) {
	cl, err := s.repo.CreateChecklist(ctx, actor.OrgID, in)
	if err != nil {
		return nil, err
	}
	s.audit(ctx, actor, "create", "hr/checklist", cl.ID, cl.Name)
	return cl, nil
}

// DeleteChecklist removes a checklist template.
func (s *Service) DeleteChecklist(ctx context.Context, actor Actor, id string) error {
	if err := s.repo.DeleteChecklist(ctx, actor.OrgID, id); err != nil {
		return err
	}
	s.audit(ctx, actor, "delete", "hr/checklist", id, "")
	return nil
}

// --- Checklist Runs ---

// StartChecklistRun starts a new checklist run for an employee.
func (s *Service) StartChecklistRun(ctx context.Context, actor Actor, in StartChecklistRunInput) (*ChecklistRun, error) {
	run, err := s.repo.StartChecklistRun(ctx, actor.OrgID, in)
	if err != nil {
		return nil, err
	}
	s.audit(ctx, actor, "create", "hr/checklist_run", run.ID, run.EmployeeID)
	return run, nil
}

// GetChecklistRun returns a single checklist run.
func (s *Service) GetChecklistRun(ctx context.Context, orgID, id string) (*ChecklistRun, error) {
	return s.repo.GetChecklistRun(ctx, orgID, id)
}

// ListChecklistRuns returns all checklist runs for a specific employee.
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
// When the run transitions to "completed", a compliance evidence record is written.
func (s *Service) UpdateChecklistRun(ctx context.Context, actor Actor, id string, in UpdateChecklistRunInput) (*ChecklistRun, error) {
	run, err := s.repo.UpdateChecklistRun(ctx, actor.OrgID, id, in)
	if err != nil {
		return nil, err
	}
	s.audit(ctx, actor, "update", "hr/checklist_run", run.ID, run.EmployeeID)
	if run.Status == "completed" {
		s.fireCompletionEvidence(ctx, run)
	}
	return run, nil
}

// StartOnboarding finds the first onboarding checklist for the organisation and starts
// a run for the given employee. Returns an error if no onboarding checklist exists.
func (s *Service) StartOnboarding(ctx context.Context, actor Actor, employeeID string) (*ChecklistRun, error) {
	run, err := s.startTypedRun(ctx, actor.OrgID, employeeID, "onboarding")
	if err != nil {
		return nil, err
	}
	s.audit(ctx, actor, "start_onboarding", "hr/checklist_run", run.ID, employeeID)
	return run, nil
}

// StartOffboarding finds the first offboarding checklist for the organisation, sets the
// employee's status to "offboarding", and starts a run. Returns an error if no offboarding
// checklist exists.
func (s *Service) StartOffboarding(ctx context.Context, actor Actor, employeeID string) (*ChecklistRun, error) {
	run, err := s.startTypedRun(ctx, actor.OrgID, employeeID, "offboarding")
	if err != nil {
		return nil, err
	}
	if err := s.repo.SetEmployeeStatus(ctx, actor.OrgID, employeeID, "offboarding"); err != nil {
		log.Error().Err(err).Str("employee_id", employeeID).Msg("hr: set employee status to offboarding")
	}
	s.audit(ctx, actor, "start_offboarding", "hr/checklist_run", run.ID, employeeID)
	return run, nil
}

func (s *Service) startTypedRun(ctx context.Context, orgID, employeeID, checklistType string) (*ChecklistRun, error) {
	checklist, err := s.repo.FirstChecklistByType(ctx, orgID, checklistType)
	if err != nil {
		return nil, fmt.Errorf("find %s checklist: %w", checklistType, err)
	}
	if checklist == nil {
		return nil, fmt.Errorf("no %s checklist found for organisation", checklistType)
	}
	return s.repo.StartChecklistRun(ctx, orgID, StartChecklistRunInput{
		EmployeeID:  employeeID,
		ChecklistID: checklist.ID,
	})
}

// CompleteStep marks a single step within a checklist run as completed by the given user.
// It is idempotent — re-completing an already-completed step is a no-op. When all required
// steps are completed, the run automatically transitions to "completed" status.
//
// Audit-log: a "complete_step" entry is written for every successful step
// completion (including idempotent re-tries — those are useful audit data:
// they show that someone tried to mark a step done that was already done).
func (s *Service) CompleteStep(ctx context.Context, actor Actor, runID, stepID, completedBy string) (*ChecklistRun, error) {
	if stepID == "" {
		return nil, errors.New("step_id is required")
	}
	run, err := s.repo.GetChecklistRun(ctx, actor.OrgID, runID)
	if err != nil {
		return nil, fmt.Errorf("get checklist run: %w", err)
	}
	if run.Status == "completed" {
		return run, nil
	}
	checklist, err := s.repo.GetChecklist(ctx, actor.OrgID, run.ChecklistID)
	if err != nil {
		return nil, fmt.Errorf("get checklist for run: %w", err)
	}
	if !stepExists(checklist.Items, stepID) {
		return nil, fmt.Errorf("step %q not found in checklist", stepID)
	}

	alreadyDone := contains(run.CompletedItems, stepID)
	if !alreadyDone {
		run.CompletedItems = append(run.CompletedItems, stepID)
		if err := s.repo.InsertRunEvent(ctx, runID, actor.OrgID, stepID, completedBy); err != nil {
			log.Error().Err(err).Str("run_id", runID).Str("step_id", stepID).Msg("hr: insert run event")
		}
	}

	status := run.Status
	if allRequiredCompleted(checklist.Items, run.CompletedItems) {
		status = "completed"
	}

	updated, err := s.repo.UpdateChecklistRun(ctx, actor.OrgID, runID, UpdateChecklistRunInput{
		CompletedItems: run.CompletedItems,
		Status:         status,
	})
	if err != nil {
		return nil, err
	}
	s.audit(ctx, actor, "complete_step", "hr/checklist_run", runID, stepID)
	if updated.Status == "completed" && run.Status != "completed" {
		s.fireCompletionEvidence(ctx, updated)
	}
	return updated, nil
}

// ListRunEvents returns the step-completion audit trail for a given run.
func (s *Service) ListRunEvents(ctx context.Context, orgID, runID string) ([]RunEvent, error) {
	events, err := s.repo.ListRunEvents(ctx, orgID, runID)
	if err != nil {
		return nil, fmt.Errorf("list run events: %w", err)
	}
	if events == nil {
		events = []RunEvent{}
	}
	return events, nil
}

// fireCompletionEvidence writes compliance evidence for a completed run.
// Errors are logged but never propagated — evidence-writing failures must not
// roll back the run-completion (the run is already persisted at this point).
func (s *Service) fireCompletionEvidence(ctx context.Context, run *ChecklistRun) {
	emp, err := s.repo.GetEmployee(ctx, run.OrgID, run.EmployeeID)
	if err != nil {
		log.Error().Err(err).Str("run_id", run.ID).Msg("hr: load employee for evidence")
		return
	}
	checklist, err := s.repo.GetChecklist(ctx, run.OrgID, run.ChecklistID)
	if err != nil {
		log.Error().Err(err).Str("run_id", run.ID).Msg("hr: load checklist for evidence")
		return
	}
	completedAt := time.Now().UTC()
	if run.CompletedAt != nil {
		completedAt = *run.CompletedAt
	}
	err = s.evidence.WriteChecklistCompletion(ctx, ChecklistCompletionEvidence{
		OrgID:         run.OrgID,
		EmployeeName:  emp.FirstName + " " + emp.LastName,
		EmployeeEmail: emp.Email,
		ChecklistName: checklist.Name,
		ChecklistType: checklist.Type,
		RunID:         run.ID,
		CompletedAt:   completedAt,
		StepCount:     len(run.CompletedItems),
	})
	if err != nil {
		log.Error().Err(err).Str("run_id", run.ID).Msg("hr: write checklist completion evidence")
	}
}

func stepExists(items []ChecklistItem, stepID string) bool {
	for _, it := range items {
		if it.ID == stepID {
			return true
		}
	}
	return false
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func allRequiredCompleted(items []ChecklistItem, completed []string) bool {
	done := make(map[string]struct{}, len(completed))
	for _, id := range completed {
		done[id] = struct{}{}
	}
	for _, it := range items {
		if it.Required {
			if _, ok := done[it.ID]; !ok {
				return false
			}
		}
	}
	return true
}
