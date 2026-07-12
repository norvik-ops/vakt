package vaktaware

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/shared/queuemetrics"
)

const TaskAutoEnrollment = "aware:auto_enrollment"

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
	rules, err := s.repo.ListEnrollmentRules(ctx, payload.OrgID)
	if err != nil {
		return fmt.Errorf("list enrollment rules: %w", err)
	}
	for _, rule := range rules {
		if !rule.IsActive || rule.TriggerType != payload.TriggerType {
			continue
		}
		if rule.TargetCampaignID == nil {
			continue
		}
		if err := s.enrollIfNotAlready(ctx, payload.OrgID, *rule.TargetCampaignID, payload.EmployeeID, payload.TriggerType); err != nil {
			log.Warn().Err(err).Str("rule_id", rule.ID).Msg("auto-enrollment failed")
		}
	}
	return nil
}

// enrollIfNotAlready links an employee (by campaign-level note) to a campaign
// unless the combination already exists.
func (s *Service) enrollIfNotAlready(ctx context.Context, orgID, campaignID, employeeID, source string) error {
	already, err := s.repo.IsEnrolledInCampaign(ctx, orgID, campaignID, employeeID)
	if err != nil {
		return fmt.Errorf("check enrollment: %w", err)
	}
	if already {
		return nil
	}
	return s.repo.CreateCampaignEnrollment(ctx, orgID, campaignID, employeeID, source)
}

// HandleNewEmployeeEnrollment is called by the HR event subscriber when a new
// employee is created. It enqueues an auto-enrollment job for the new employee.
func (s *Service) HandleNewEmployeeEnrollment(ctx context.Context, orgID, employeeID string) {
	if err := s.EnqueueAutoEnrollment(ctx, AutoEnrollmentPayload{
		OrgID:       orgID,
		TriggerType: "new_employee",
		EmployeeID:  employeeID,
	}); err != nil {
		log.Error().Err(err).Str("org_id", orgID).Str("employee_id", employeeID).
			Msg("failed to enqueue new-employee auto-enrollment")
	}
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
