package vaktaware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/shared/events"
	"github.com/matharnica/vakt/internal/shared/queuemetrics"
)

const TaskAutoEnrollment = "aware:auto_enrollment"

// Trigger types — the value set of sr_enrollment_rules.trigger_type
// (migration 173: CHECK (trigger_type IN ('new_employee','phishing_click'))).
// It answers WHY a rule fires.
const (
	TriggerNewEmployee   = "new_employee"
	TriggerPhishingClick = "phishing_click"
)

// Enrolment sources — the value set of sr_campaign_enrollments.source
// (migration 173: CHECK (source IN ('manual','auto_new_employee','auto_phishing_click'))).
// It answers HOW an enrolment came about, and that is a DIFFERENT question:
// `manual` records a human decision and has no counterpart among the trigger
// types at all.
const (
	SourceManual            = "manual"
	SourceAutoNewEmployee   = "auto_new_employee"
	SourceAutoPhishingClick = "auto_phishing_click"
)

// enrollmentSourceByTrigger maps a rule's trigger type onto the enrolment
// source it produces. Total over the trigger-type value set by construction —
// see enrollmentSourceFor for why the lookup must be able to fail.
//
// R1-35-01: the two value sets are disjoint. `HandleAutoEnrollment` used to
// pass the trigger type straight into the `source` column, so EVERY automatic
// enrolment INSERT was rejected with SQLSTATE 23514 (check constraint
// sr_campaign_enrollments_source_check). The error was logged and swallowed,
// so the feature reported success and wrote nothing — since it was written.
//
// Reconciled in code rather than by widening the CHECK constraint: the target
// set's `manual` vs `auto_*` split carries information that the trigger-type
// set does not have (was this enrolment a human decision or a rule?), and
// collapsing the two sets into one would destroy it. No migration needed.
var enrollmentSourceByTrigger = map[string]string{
	TriggerNewEmployee:   SourceAutoNewEmployee,
	TriggerPhishingClick: SourceAutoPhishingClick,
}

// enrollmentSourceFor translates a trigger type into the enrolment source to
// store, and FAILS on anything it does not know.
//
// The failure case is the point. A silent fallback (or a `"auto_" + trigger`
// string concatenation) would push an unvetted value at the CHECK constraint
// again and reproduce the original defect for the next trigger type someone
// adds. Failing here moves the detection from "INSERT rejected at runtime and
// swallowed" to "refused at the seam, with the offending value named".
func enrollmentSourceFor(triggerType string) (string, error) {
	source, ok := enrollmentSourceByTrigger[triggerType]
	if !ok {
		return "", fmt.Errorf("unknown enrollment trigger type %q: no matching sr_campaign_enrollments.source value", triggerType)
	}
	return source, nil
}

// AutoEnrollmentPayload is the Asynq task payload for auto-enrollment jobs.
type AutoEnrollmentPayload struct {
	OrgID       string `json:"org_id"`
	TriggerType string `json:"trigger_type"`
	EmployeeID  string `json:"employee_id,omitempty"`
	CampaignID  string `json:"campaign_id,omitempty"`
}

// ListEnrollmentRules returns all enrollment rules for the org.
func (s *Service) ListEnrollmentRules(ctx context.Context, orgID string) ([]EnrollmentRule, error) {
	return s.repo.ListEnrollmentRules(ctx, orgID)
}

// CreateEnrollmentRule creates a new auto-enrollment rule.
func (s *Service) CreateEnrollmentRule(ctx context.Context, orgID string, input CreateEnrollmentRuleInput) (*EnrollmentRule, error) {
	return s.repo.CreateEnrollmentRule(ctx, orgID, input)
}

// UpdateEnrollmentRuleActive toggles the is_active flag on an enrollment rule.
func (s *Service) UpdateEnrollmentRuleActive(ctx context.Context, orgID, ruleID string, active bool) error {
	return s.repo.UpdateEnrollmentRuleActive(ctx, orgID, ruleID, active)
}

// DeleteEnrollmentRule removes an enrollment rule.
func (s *Service) DeleteEnrollmentRule(ctx context.Context, orgID, ruleID string) error {
	return s.repo.DeleteEnrollmentRule(ctx, orgID, ruleID)
}

// EnqueueAutoEnrollment enqueues an auto-enrollment Asynq task.
func (s *Service) EnqueueAutoEnrollment(ctx context.Context, payload AutoEnrollmentPayload) error {
	if s.asynqClient == nil {
		return nil
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal auto-enrollment payload: %w", err)
	}
	task := asynq.NewTask(TaskAutoEnrollment, data)
	if _, err = s.asynqClient.EnqueueContext(ctx, task, asynq.Queue(Queue)); err != nil {
		queuemetrics.RecordError(Queue)
	}
	return err
}

// HandleAutoEnrollment processes an auto-enrollment task: for each active rule
// matching the trigger type, it enrolls the employee into the target campaign
// unless they are already enrolled.
func (s *Service) HandleAutoEnrollment(ctx context.Context, payload AutoEnrollmentPayload) error {
	// Translate ONCE, before touching the database: an unknown trigger type is
	// a defect in the caller, not a per-rule problem, and must not be reported
	// as "0 rules matched".
	source, err := enrollmentSourceFor(payload.TriggerType)
	if err != nil {
		return err
	}
	if payload.EmployeeID == "" {
		return fmt.Errorf("auto-enrollment for org %s: employee id is empty", payload.OrgID)
	}

	rules, err := s.repo.ListEnrollmentRules(ctx, payload.OrgID)
	if err != nil {
		return fmt.Errorf("list enrollment rules: %w", err)
	}

	var (
		failures []error
		matched  int
		enrolled int
	)
	for _, rule := range rules {
		if !rule.IsActive || rule.TriggerType != payload.TriggerType {
			continue
		}
		if rule.TargetCampaignID == nil {
			continue
		}
		matched++
		created, err := s.enrollIfNotAlready(ctx, payload.OrgID, *rule.TargetCampaignID, payload.EmployeeID, source)
		if err != nil {
			// Still logged with the rule id — but ALSO collected, so the caller
			// learns that the work did not happen.
			log.Error().Err(err).Str("rule_id", rule.ID).Str("org_id", payload.OrgID).
				Msg("auto-enrollment failed")
			failures = append(failures, fmt.Errorf("rule %s: %w", rule.ID, err))
			continue
		}
		if created {
			enrolled++
		}
	}

	// R1-35-01 (second half): returning nil after a failed INSERT reports
	// success for work that did not happen — the Asynq task is acknowledged,
	// nothing is retried, and no alert fires. The rejected INSERTs above were
	// invisible for exactly this reason. Aggregate and hand the failure up.
	if len(failures) > 0 {
		return fmt.Errorf("auto-enrollment org=%s trigger=%s: %d of %d rules failed: %w",
			payload.OrgID, payload.TriggerType, len(failures), matched, errors.Join(failures...))
	}

	log.Debug().Str("org_id", payload.OrgID).Str("trigger", payload.TriggerType).
		Int("rules_matched", matched).Int("enrolled", enrolled).
		Msg("auto-enrollment: completed")
	return nil
}

// enrollIfNotAlready links an employee to a campaign unless the combination
// already exists. It reports whether a row was created, so the caller can tell
// "enrolled" apart from "was already enrolled" instead of counting both as
// success and learning nothing.
//
// `source` is an already-translated sr_campaign_enrollments.source value, never
// a trigger type — see enrollmentSourceFor.
func (s *Service) enrollIfNotAlready(ctx context.Context, orgID, campaignID, employeeID, source string) (bool, error) {
	already, err := s.repo.IsEnrolledInCampaign(ctx, orgID, campaignID, employeeID)
	if err != nil {
		return false, fmt.Errorf("check enrollment: %w", err)
	}
	if already {
		return false, nil
	}
	if err := s.repo.CreateCampaignEnrollment(ctx, orgID, campaignID, employeeID, source); err != nil {
		return false, err
	}
	return true, nil
}

// EnrollmentTrigger is vaktaware's implementation of the shared
// events.EmployeeOnboardingTrigger: it reacts to an HR entry by enrolling the
// new employee into the campaigns the org's active `new_employee` rules point at.
//
// R1-SA25-01 — why this is SYNCHRONOUS rather than an Asynq enqueue:
//
//  1. It matches the cross-module shape this codebase already uses for the
//     sibling event (vaktcomply's HRAccessReviewTrigger, fired in-process from
//     vakthr on offboarding completion).
//  2. EnqueueAutoEnrollment returns nil when no Asynq client is configured. A
//     service built without one — which is exactly how the worker builds
//     vaktaware — would therefore report success and enqueue nothing. Routing
//     the entry through the queue would have rebuilt the silent-no-op the
//     original defect consisted of, one layer down.
//  3. The work is one SELECT plus a small INSERT per matching rule. There is no
//     latency argument for deferring it, and doing it in-process makes the
//     whole chain verifiable end to end: create an employee, then read the row.
//
// The queue path (EnqueueAutoEnrollment + the worker's aware:auto_enrollment
// handler) stays for the phishing_click trigger, where the producer is a public
// tracking endpoint and deferring the work IS the right call.
type EnrollmentTrigger struct {
	svc *Service
}

// NewEnrollmentTrigger wires the vaktaware service as the subscriber for HR
// entry events.
//
// It returns the INTERFACE, not *EnrollmentTrigger, on purpose. Callers store
// the result in an events.EmployeeOnboardingTrigger variable, and a nil
// *EnrollmentTrigger stored in an interface is not a nil interface — the
// `if t == nil` guard in WithEmployeeOnboardingTrigger would wave it through
// and every employee creation would panic on the first call. Handing back a
// real noop instead of a typed nil makes that unrepresentable.
func NewEnrollmentTrigger(svc *Service) events.EmployeeOnboardingTrigger {
	if svc == nil {
		return &events.NoopEmployeeOnboardingTrigger{}
	}
	return &EnrollmentTrigger{svc: svc}
}

// TriggerNewEmployeeEnrollment enrols the new employee via the same code path
// the queued task uses, so the two triggers cannot drift apart.
func (t *EnrollmentTrigger) TriggerNewEmployeeEnrollment(ctx context.Context, in events.NewEmployeeInput) error {
	return t.svc.HandleAutoEnrollment(ctx, AutoEnrollmentPayload{
		OrgID:       in.OrgID,
		TriggerType: TriggerNewEmployee,
		EmployeeID:  in.EmployeeID,
	})
}

// enrollmentRuleFromRow maps raw pgx scan results into an EnrollmentRule.
func enrollmentRuleFromRow(id, orgID, name, triggerType string, campaignID pgtype.UUID, isActive bool, createdAt, updatedAt pgtype.Timestamptz) EnrollmentRule {
	r := EnrollmentRule{
		ID:          id,
		OrgID:       orgID,
		Name:        name,
		TriggerType: triggerType,
		IsActive:    isActive,
		CreatedAt:   tsToTime(createdAt),
		UpdatedAt:   tsToTime(updatedAt),
	}
	if campaignID.Valid {
		s := campaignID.String()
		r.TargetCampaignID = &s
	}
	return r
}

// ── Campaign enrollment ───────────────────────────────────────────────────

// CampaignEnrollment tracks auto-enrollment of an employee into a campaign.
type CampaignEnrollment struct {
	ID         string    `json:"id"`
	OrgID      string    `json:"org_id"`
	CampaignID string    `json:"campaign_id"`
	EmployeeID string    `json:"employee_id"`
	Source     string    `json:"source"`
	CreatedAt  time.Time `json:"created_at"`
}
