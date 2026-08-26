package vakthr

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/shared/audit"
	"github.com/matharnica/vakt/internal/shared/logsafe"
	"github.com/matharnica/vakt/internal/shared/platform/events"
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
	repo           *Repository
	db             *pgxpool.Pool
	evidence       EvidenceWriter
	accessReview   AccessReviewTrigger
	onboarding     EmployeeOnboardingTrigger
	sessionRevoker SessionRevoker
}

// SessionRevoker fully revokes a platform user's access: it deletes the refresh
// sessions AND bumps pw_version so the stateless Paseto access token is rejected
// on the next request. Satisfied by *auth.Service.RevokeAllSessions.
//
// S131-C1 (R-H21): without this, HR offboarding deleted the refresh sessions but
// left the terminated employee's access token valid for up to the 1h TTL — vakthr's
// core promise ("audit-ready evidence that access revocation occurred") was false
// for that window.
type SessionRevoker interface {
	RevokeAllSessions(ctx context.Context, userID string) error
}

// audit is the single point where the HR service writes audit-log entries.
// Best-effort: a failure here is logged inside audit.Write but never aborts
// the calling operation.
func (s *Service) audit(ctx context.Context, actor Actor, action, resourceType, resourceID, resourceName string) {
	audit.Write(ctx, s.db, audit.WriteEntry{
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
	return &Service{repo: repo, db: repo.db, evidence: NoopEvidenceWriter(),
		accessReview: &NoopAccessReviewTrigger{}, onboarding: &NoopEmployeeOnboardingTrigger{}}
}

// NewServiceFromPool is a convenience constructor that creates the repository internally.
func NewServiceFromPool(db *pgxpool.Pool) *Service {
	repo := NewRepository(db)
	return &Service{repo: repo, db: db, evidence: NoopEvidenceWriter(),
		accessReview: &NoopAccessReviewTrigger{}, onboarding: &NoopEmployeeOnboardingTrigger{}}
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

// WithSessionRevoker injects the auth session revoker used on termination so the
// offboarded user's access token dies immediately (pw_version bump), not just
// their refresh sessions. When nil, revokeUserAccess falls back to the
// refresh-only repo query (degraded but non-fatal).
func (s *Service) WithSessionRevoker(sr SessionRevoker) *Service {
	s.sessionRevoker = sr
	return s
}

// WithEmployeeOnboardingTrigger injects the trigger fired when a new employee
// is created. When nil, the noop is used and no module reacts to an entry.
func (s *Service) WithEmployeeOnboardingTrigger(t EmployeeOnboardingTrigger) *Service {
	if t == nil {
		s.onboarding = &NoopEmployeeOnboardingTrigger{}
	} else {
		s.onboarding = t
	}
	return s
}

// WithAccessReviewTrigger injects the access-review trigger used when offboarding runs complete.
func (s *Service) WithAccessReviewTrigger(t AccessReviewTrigger) *Service {
	if t == nil {
		s.accessReview = &NoopAccessReviewTrigger{}
	} else {
		s.accessReview = t
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

	// R1-SA25-01: publish the entry so vaktaware can auto-enrol the new employee
	// into the campaigns its active `new_employee` rules point at. This is THE
	// producer of that event — CreateEmployee is the single chokepoint through
	// which every employee record is created (handler.go:125 is its only caller;
	// the sqlc-generated db.CreateEmployee has none).
	//
	// Best-effort, like revokeUserAccess and the offboarding access review: the
	// employee row is already committed and must not be rolled back because a
	// training enrolment failed. But the error is LOGGED AT ERROR, never
	// discarded — a silent enrolment failure is exactly what made BSI ORP.3
	// report an induction that never happened.
	if s.onboarding != nil {
		if obErr := s.onboarding.TriggerNewEmployeeEnrollment(ctx, NewEmployeeInput{
			OrgID:      actor.OrgID,
			EmployeeID: emp.ID,
		}); obErr != nil {
			log.Error().Err(obErr).
				Str("org_id", actor.OrgID).
				Str("employee_id", emp.ID).
				Msg("hr: trigger new-employee auto-enrollment")
		}
	}
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

// Termination paths that revoke platform access (pw_version bump + session
// delete via revokeUserAccess): UpdateEmployee(status=terminated),
// UpdateContractor(status=terminated) and — since R1-14c-05 — the completion of
// an offboarding checklist run (guardCompletionForType → enforceOffboardingRevocation).
//
// S131-C1 — gefunden, bewusst NICHT gebaut (Produktentscheidung, P13): the
// Personio `employee.departed` webhook (TriggerPersonioOffboarding) and
// StartOffboarding set status to 'offboarding', NOT 'terminated' — they start the
// offboarding *process* (a checklist for the admin), which is not the same as an
// immediate access cut. Whether an automated Personio departure should revoke
// access instantly (security-first) or only when offboarding completes to
// 'terminated' (process-first) is a product decision, not a wiring bug.
//
// R1-14c-05 resolves that open question in the process-first direction: the END
// of the offboarding process is exactly the moment process-first says the cut
// belongs, and it is the moment at which we publish a compliance record claiming
// it happened. Starting offboarding still only sets 'offboarding'.

// revokeUserAccess revokes all active sessions and API keys for the platform user
// matching the given email within the org. Errors are logged but do not fail the call —
// the HR record update is already committed and must not be rolled back due to a
// transient auth-DB issue. Use revokeUserAccessChecked wherever the outcome has
// to be known (i.e. wherever a compliance claim depends on it).
func (s *Service) revokeUserAccess(ctx context.Context, orgID, email string) {
	if err := s.revokeUserAccessChecked(ctx, orgID, email); err != nil {
		log.Error().Err(err).Str("email_redacted", logsafe.RedactEmail(email)).
			Msg("hr: revoke platform access")
	}
}

// revokeUserAccessChecked performs the full platform-access revocation for the
// given employee email inside the org and REPORTS what failed. It is the single
// implementation; revokeUserAccess is the fire-and-forget wrapper for callers
// whose HR-record write is already committed and must not be rolled back.
//
// What "revoking access" comprises, in this exact order:
//
//  1. Resolve the platform user through org_members. This MUST run before step 3,
//     because step 3 deletes the very row the lookup joins through.
//  2. RevokeAllSessions(userID) — deletes every refresh_sessions row of that user
//     AND bumps users.pw_version, so the already-minted, stateless Paseto access
//     token is rejected on the next request instead of living out its ≤1h TTL.
//     Without a wired revoker only the refresh rows can be deleted; the access
//     token would survive. Half a revocation is not a revocation, so that case is
//     reported as an error rather than silently accepted.
//  3. DisableUser — DELETE FROM org_members, which removes the org-scoped RBAC
//     grant. The global account and memberships in other orgs stay untouched
//     (Vakt is multi-org at the user level).
//  4. RevokeUserAPIKeys — revokes every active API key of that user in the org,
//     so a long-lived sk_ key cannot outlive the session revocation.
//
// An employee without a platform account (an HR-only record — the common case)
// is NOT a failure: the lookup returns found=false, every statement matches zero
// rows, and the function returns nil. There was nothing to revoke.
//
// Every sub-step is attempted even after an earlier one failed, so the returned
// error names all of them instead of stopping at the first.
func (s *Service) revokeUserAccessChecked(ctx context.Context, orgID, email string) error {
	if email == "" {
		// An HR record without an email cannot be matched to a platform account.
		return nil
	}
	redacted := logsafe.RedactEmail(email)
	var errs []error

	userID, found, err := s.repo.PlatformUserIDByEmail(ctx, orgID, email)
	if err != nil {
		log.Error().Err(err).Str("email_redacted", redacted).Msg("hr: resolve platform user on termination")
		errs = append(errs, fmt.Errorf("resolve platform user: %w", err))
	}

	tokensDead := true
	switch {
	case found && s.sessionRevoker != nil:
		if rErr := s.sessionRevoker.RevokeAllSessions(ctx, userID); rErr != nil {
			log.Error().Err(rErr).Str("email_redacted", redacted).Msg("hr: revoke sessions+token on termination")
			errs = append(errs, fmt.Errorf("revoke sessions and access token: %w", rErr))
			tokensDead = false
		}
	default:
		// No platform account for this email, or no revoker wired: fall back to
		// the refresh-only delete so at least the refresh tokens are gone.
		if rErr := s.repo.RevokeUserSessions(ctx, orgID, email); rErr != nil {
			log.Error().Err(rErr).Str("email_redacted", redacted).Msg("hr: revoke sessions on termination")
			errs = append(errs, fmt.Errorf("revoke refresh sessions: %w", rErr))
			tokensDead = false
		}
		if found && s.sessionRevoker == nil {
			// A platform account exists but nothing can bump pw_version, so the
			// access token stays usable for the rest of its TTL. Reported, not
			// swallowed — otherwise a misconfigured wiring would publish a
			// revocation claim that is only two thirds true.
			errs = append(errs, errors.New("no session revoker wired: access token cannot be invalidated"))
			tokensDead = false
		}
	}

	// API keys are keyed on users.email, independent of the membership row, and
	// the UPDATE is idempotent — always worth doing, even after a failure above.
	if err := s.repo.RevokeUserAPIKeys(ctx, orgID, email); err != nil {
		log.Error().Err(err).Str("email_redacted", redacted).Msg("hr: revoke api keys on termination")
		errs = append(errs, fmt.Errorf("revoke api keys: %w", err))
	}

	// Removing the org membership comes LAST and only once the tokens are dead.
	// That row is the sole handle PlatformUserIDByEmail joins through: deleting it
	// after a failed token revocation would make every later retry resolve
	// found=false and report success without ever bumping pw_version — the access
	// token would then stay valid for the rest of its TTL, permanently and
	// silently. Keeping the row costs nothing here (the token is valid either way)
	// and keeps the retry able to finish the job.
	if tokensDead {
		if err := s.repo.DisableUser(ctx, orgID, email); err != nil {
			log.Error().Err(err).Str("email_redacted", redacted).Msg("hr: disable user on termination")
			errs = append(errs, fmt.Errorf("remove org membership: %w", err))
		}
	}

	return errors.Join(errs...)
}

// enforceOffboardingRevocation carries out everything a completed offboarding run
// claims about the employee: the platform access is revoked (see
// revokeUserAccessChecked for the exact list) and the HR record moves to
// 'terminated'. Returns an error as soon as any part of it did not happen.
//
// It is idempotent: every statement it triggers is a DELETE/UPDATE that matches
// zero rows the second time, and re-bumping pw_version merely invalidates
// already-invalid tokens. A retry after a partial failure completes the rest.
func (s *Service) enforceOffboardingRevocation(ctx context.Context, orgID, employeeID string) error {
	emp, err := s.repo.GetEmployee(ctx, orgID, employeeID)
	if err != nil {
		return fmt.Errorf("load employee for access revocation: %w", err)
	}
	if err := s.revokeUserAccessChecked(ctx, orgID, emp.Email); err != nil {
		return err
	}
	// The HR record must agree with reality; leaving it on 'offboarding' would
	// keep the employee listed as still-in-process after the run closed.
	if err := s.repo.SetEmployeeStatus(ctx, orgID, employeeID, "terminated"); err != nil {
		return fmt.Errorf("set employee status to terminated: %w", err)
	}
	return nil
}

// guardCompletionForType is the single gate through which a checklist run may
// transition to "completed". For an offboarding run it first performs the access
// revocation the resulting compliance record will claim; if that fails, it
// returns an error and the caller must leave the run open.
//
// R1-14c-05: before this gate existed, completing an offboarding run — including
// the mandatory step "Alle IT-Zugänge widerrufen (AD, VPN, SaaS)" — wrote an
// `approved` ck_evidence row while the person's platform account stayed fully
// usable: GET /auth/me answered 200, the refresh sessions lived on, the
// org_members row stood, and the employee record stayed on 'offboarding'. The
// revocation code worked; nothing ever called it from here. That is worse than a
// missing feature: it is a released audit record asserting something false, and
// plausibly false, so nobody goes looking.
//
// Why the whole run and not the single step off-1: a step id is free text chosen
// per organisation (the demo seed's "off-1" is one org's convention, not a
// contract) and ChecklistItem carries no machine-readable "revokes access" flag,
// so hanging the cut on a step would mean string-matching seed data. The
// checklist TYPE is a first-class column (hr_checklists.type) and already gates
// the offboarding-specific evidence. Above all, the false claim is produced at
// run completion, so the deed has to be a precondition of exactly that write.
// A per-step trigger stays the finer instrument and would be the right follow-up
// once checklist items can declare an effect — that needs a schema change.
func (s *Service) guardCompletionForType(ctx context.Context, actor Actor, run *ChecklistRun, checklistType string) error {
	if checklistType != "offboarding" {
		return nil
	}
	if err := s.enforceOffboardingRevocation(ctx, actor.OrgID, run.EmployeeID); err != nil {
		// No evidence is written on this path: an evidence row without the deed is
		// the defect itself. The attempt and its failure are recorded in the audit
		// log and the error log, and the run stays open so it is visibly unfinished.
		log.Error().Err(err).Str("run_id", run.ID).Str("employee_id", run.EmployeeID).
			Msg("hr: offboarding access revocation failed — run left open, no completion evidence written")
		s.audit(ctx, actor, "offboarding_revoke_failed", "hr/checklist_run", run.ID, run.EmployeeID)
		return fmt.Errorf("offboarding run not completed, access revocation failed: %w", err)
	}
	s.audit(ctx, actor, "offboarding_access_revoked", "hr/checklist_run", run.ID, run.EmployeeID)
	return nil
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

// GetChecklist returns a single checklist template by ID, scoped to the org.
// S121-C4 (C7): the checklist-run page fetches the template to render its items,
// but no single-template read route existed — only the list. This exposes the
// already-present repository.GetChecklist through the service/handler layers.
func (s *Service) GetChecklist(ctx context.Context, orgID, id string) (*Checklist, error) {
	return s.repo.GetChecklist(ctx, orgID, id)
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
//
// R1-14c-05: closing an offboarding run is gated on the access revocation it
// claims (guardCompletionForType). Evidence fires on the TRANSITION, not on the
// end state — a repeated PUT with status=completed used to write a second
// ck_evidence row for the same run.
func (s *Service) UpdateChecklistRun(ctx context.Context, actor Actor, id string, in UpdateChecklistRunInput) (*ChecklistRun, error) {
	alreadyCompleted := false
	if in.Status == "completed" {
		cur, err := s.repo.GetChecklistRun(ctx, actor.OrgID, id)
		if err != nil {
			return nil, fmt.Errorf("get checklist run: %w", err)
		}
		alreadyCompleted = cur.Status == "completed"
		if !alreadyCompleted {
			checklist, err := s.repo.GetChecklist(ctx, actor.OrgID, cur.ChecklistID)
			if err != nil {
				return nil, fmt.Errorf("get checklist for run: %w", err)
			}
			if err := s.guardCompletionForType(ctx, actor, cur, checklist.Type); err != nil {
				return nil, err
			}
		}
	}

	run, err := s.repo.UpdateChecklistRun(ctx, actor.OrgID, id, in)
	if err != nil {
		return nil, err
	}
	s.audit(ctx, actor, "update", "hr/checklist_run", run.ID, run.EmployeeID)
	if run.Status == "completed" && !alreadyCompleted {
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

	// R1-14c-05: an offboarding run may only close after the access revocation it
	// claims has actually happened. On failure the tick is still persisted (the
	// step WAS done) but the run stays open, so the state is honest and re-posting
	// the same step retries the revocation.
	if status == "completed" && run.Status != "completed" {
		if err := s.guardCompletionForType(ctx, actor, run, checklist.Type); err != nil {
			if _, uErr := s.repo.UpdateChecklistRun(ctx, actor.OrgID, runID, UpdateChecklistRunInput{
				CompletedItems: run.CompletedItems,
				Status:         run.Status,
			}); uErr != nil {
				log.Error().Err(uErr).Str("run_id", runID).Msg("hr: persist step tick after failed revocation")
			}
			return nil, err
		}
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
//
// PRECONDITION (R1-14c-05): both callers run guardCompletionForType before they
// let a run reach "completed", so for an offboarding run the access revocation
// this evidence attests to has already succeeded. Do not call this function from
// a new path without passing that gate first — the evidence row lands with
// status 'approved' and would otherwise assert something that never happened.
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
	err = s.evidence.WriteChecklistCompletion(ctx, events.ChecklistCompletionEvidence{
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

	if checklist.Type == "offboarding" {
		if arErr := s.accessReview.TriggerOffboardingReview(ctx, OffboardingReviewInput{
			OrgID:       run.OrgID,
			RunID:       run.ID,
			Department:  emp.Department,
			CompletedAt: completedAt,
		}); arErr != nil {
			log.Error().Err(arErr).Str("run_id", run.ID).Msg("hr: trigger offboarding access review")
		}

		// Personio-specific evidence: calculate elapsed time since departure
		personioID, departureDate, pfErr := s.repo.GetEmployeePersonioFields(ctx, run.OrgID, run.EmployeeID)
		if pfErr == nil && personioID > 0 && !departureDate.IsZero() {
			elapsed := completedAt.Sub(departureDate).Hours()
			status := "ok"
			if elapsed > 24 {
				status = "warning"
			}
			evIn := events.PersonioOffboardingEvidence{
				OrgID:              run.OrgID,
				PersonioEmployeeID: personioID,
				RunID:              run.ID,
				CompletedAt:        completedAt,
				DepartureDate:      departureDate,
				ElapsedHours:       elapsed,
				Status:             status,
			}
			if evErr := s.evidence.WritePersonioOffboardingEvidence(ctx, evIn); evErr != nil {
				log.Error().Err(evErr).Str("run_id", run.ID).Msg("hr: write personio offboarding evidence")
			}
		}
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

// ListEmployeesCursor returns employees using keyset pagination.
func (s *Service) ListEmployeesCursor(ctx context.Context, orgID string, cursorID string, cursorTS time.Time, limit int) ([]Employee, error) {
	return s.repo.ListEmployeesCursor(ctx, orgID, cursorID, cursorTS, limit)
}
