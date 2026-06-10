// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/shared/notify"
)

// NIS2ReportabilityCheck holds the three NIS2 Art.23 meldepflicht criteria.
type NIS2ReportabilityCheck struct {
	CausesSignificantDisruption bool `json:"causes_significant_disruption"`
	AffectsThirdParties         bool `json:"affects_third_parties"`
	CausesFinancialDamage       bool `json:"causes_financial_damage"`
}

// IsReportable returns true if any criterion is satisfied.
func (c NIS2ReportabilityCheck) IsReportable() bool {
	return c.CausesSignificantDisruption || c.AffectsThirdParties || c.CausesFinancialDamage
}

// NIS2ReportInput holds form data for a single reporting stage.
type NIS2ReportInput struct {
	AffectedServices      string  `json:"affected_services"`
	InitialAssessment     string  `json:"initial_assessment"`
	RootCause             string  `json:"root_cause"`
	AffectedUsersEstimate *int    `json:"affected_users_estimate,omitempty"`
	MeasuresTaken         string  `json:"measures_taken"`
	EstimatedRecovery     *string `json:"estimated_recovery,omitempty"`
	FullRootCauseAnalysis string  `json:"full_root_cause_analysis"`
	PermanentMeasures     string  `json:"permanent_measures"`
	EffectivenessEvidence string  `json:"effectiveness_evidence"`
}

// NIS2ReportStatus is returned by GET /incidents/{id}/nis2-status.
type NIS2ReportStatus struct {
	IsReportable    bool              `json:"is_reportable"`
	ReportingStage  string            `json:"reporting_stage"`
	DetectedAt      *time.Time        `json:"detected_at,omitempty"`
	Deadlines       NIS2Deadlines     `json:"deadlines"`
	CompletedStages []string          `json:"completed_stages"`
	Reports         []NIS2StageReport `json:"reports"`
}

// NIS2Deadlines holds all three deadline timestamps.
type NIS2Deadlines struct {
	EarlyWarning *time.Time `json:"early_warning,omitempty"`
	FullReport   *time.Time `json:"full_report,omitempty"`
	FinalReport  *time.Time `json:"final_report,omitempty"`
}

// NIS2StageReport is a summary of a submitted report stage.
type NIS2StageReport struct {
	ID          string     `json:"id"`
	Stage       string     `json:"stage"`
	SubmittedAt *time.Time `json:"submitted_at,omitempty"`
	PDFPath     string     `json:"pdf_path,omitempty"`
}

// AuthorityContact represents an entry in the DACH authority directory.
type AuthorityContact struct {
	ID            string  `json:"id"`
	OrgID         *string `json:"org_id,omitempty"`
	Country       string  `json:"country"`
	Sector        string  `json:"sector,omitempty"`
	AuthorityName string  `json:"authority_name"`
	ReportURL     string  `json:"report_url,omitempty"`
	Email         string  `json:"email,omitempty"`
	Phone         string  `json:"phone,omitempty"`
	Notes         string  `json:"notes,omitempty"`
	IsPrimary     bool    `json:"is_primary"`
	IsBuiltin     bool    `json:"is_builtin"`
	CreatedAt     string  `json:"created_at"`
}

// MarkIncidentReportable marks an incident as NIS2-meldepflichtig and sets the three deadlines.
func (s *Service) MarkIncidentReportable(ctx context.Context, orgID string, incidentID uuid.UUID, detectedAt time.Time, check NIS2ReportabilityCheck) error {
	earlyWarning := detectedAt.Add(24 * time.Hour)
	fullReport := detectedAt.Add(72 * time.Hour)
	finalReport := detectedAt.Add(30 * 24 * time.Hour)

	if err := s.repo.SetNIS2Reportable(ctx, orgID, incidentID.String(), detectedAt, earlyWarning, fullReport, finalReport, check.IsReportable()); err != nil {
		return fmt.Errorf("set nis2 reportable: %w", err)
	}
	log.Info().
		Str("org_id", orgID).
		Str("incident_id", incidentID.String()).
		Bool("is_reportable", check.IsReportable()).
		Msg("nis2 reportability assessed")
	return nil
}

// SubmitNIS2Stage saves report content for a stage and marks it submitted.
func (s *Service) SubmitNIS2Stage(ctx context.Context, orgID, incidentID, userID, stage string, input NIS2ReportInput) (*NIS2StageReport, error) {
	if stage != "early_warning" && stage != "full_report" && stage != "final_report" {
		return nil, fmt.Errorf("invalid stage: %s", stage)
	}

	report, err := s.repo.UpsertNIS2Report(ctx, orgID, incidentID, userID, stage, input)
	if err != nil {
		return nil, fmt.Errorf("upsert nis2 report: %w", err)
	}

	if err := s.repo.UpdateNIS2Stage(ctx, orgID, incidentID, stage); err != nil {
		log.Warn().Err(err).Str("stage", stage).Msg("update nis2 reporting_stage")
	}

	return report, nil
}

// GetNIS2Status returns the full NIS2 reporting status for an incident.
func (s *Service) GetNIS2Status(ctx context.Context, orgID, incidentID string) (*NIS2ReportStatus, error) {
	inc, err := s.repo.GetIncident(ctx, orgID, incidentID)
	if err != nil {
		return nil, fmt.Errorf("get incident: %w", err)
	}

	reports, err := s.repo.ListNIS2Reports(ctx, orgID, incidentID)
	if err != nil {
		return nil, fmt.Errorf("list nis2 reports: %w", err)
	}

	var completed []string
	for _, r := range reports {
		if r.SubmittedAt != nil {
			completed = append(completed, r.Stage)
		}
	}
	if completed == nil {
		completed = []string{}
	}

	stage := "none"
	if inc.NIS2ReportingStage != nil {
		stage = *inc.NIS2ReportingStage
	}

	return &NIS2ReportStatus{
		IsReportable:   inc.NIS2Reportable != nil && *inc.NIS2Reportable,
		ReportingStage: stage,
		DetectedAt:     inc.NIS2DetectedAt,
		Deadlines: NIS2Deadlines{
			EarlyWarning: inc.NIS2EarlyWarningDue,
			FullReport:   inc.NIS2FullReportDue,
			FinalReport:  inc.NIS2FinalReportDue,
		},
		CompletedStages: completed,
		Reports:         reports,
	}, nil
}

// CheckNIS2StagingDeadlines checks all open NIS2-meldepflichtige incidents for upcoming deadlines
// and sends notifications when a deadline is within 2 hours.
func (s *Service) CheckNIS2StagingDeadlines(ctx context.Context, orgID string) error {
	incidents, err := s.repo.ListNIS2OpenIncidents(ctx, orgID)
	if err != nil {
		return fmt.Errorf("list nis2 open incidents: %w", err)
	}

	now := time.Now().UTC()
	warn := now.Add(2 * time.Hour)

	for _, inc := range incidents {
		if inc.NIS2EarlyWarningDue != nil && !inc.NIS2EarlyWarningDue.IsZero() &&
			inc.NIS2EarlyWarningSubmittedAt == nil &&
			inc.NIS2EarlyWarningDue.Before(warn) {
			s.sendNIS2DeadlineNotification(ctx, orgID, inc, "Frühwarnung", *inc.NIS2EarlyWarningDue)
		}
		if inc.NIS2FullReportDue != nil && !inc.NIS2FullReportDue.IsZero() &&
			inc.NIS2FullReportSubmittedAt == nil &&
			inc.NIS2FullReportDue.Before(warn) {
			s.sendNIS2DeadlineNotification(ctx, orgID, inc, "72h-Meldung", *inc.NIS2FullReportDue)
		}
		if inc.NIS2FinalReportDue != nil && !inc.NIS2FinalReportDue.IsZero() &&
			inc.NIS2FinalReportSubmittedAt == nil &&
			inc.NIS2FinalReportDue.Before(warn) {
			s.sendNIS2DeadlineNotification(ctx, orgID, inc, "30-Tage-Abschlussbericht", *inc.NIS2FinalReportDue)
		}
	}
	return nil
}

func (s *Service) sendNIS2DeadlineNotification(ctx context.Context, orgID string, inc Incident, stageName string, deadline time.Time) {
	remaining := time.Until(deadline)
	title := "NIS2-Meldepflicht: Deadline naht"
	body := fmt.Sprintf("Vorfall \"%s\" — %s läuft in %.0f Minuten ab (Frist: %s)",
		inc.Title, stageName, remaining.Minutes(), deadline.Format("02.01.2006 15:04 UTC"))
	notify.Send(ctx, s.db, orgID, title, body, "nis2_deadline", "vaktcomply")
	log.Info().Str("incident_id", inc.ID).Str("stage", stageName).Msg("nis2 deadline notification sent")
}

// ListAuthorityContacts returns authority contacts for the given org (including built-ins).
func (s *Service) ListAuthorityContacts(ctx context.Context, orgID string) ([]AuthorityContact, error) {
	contacts, err := s.repo.ListAuthorityContacts(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list authority contacts: %w", err)
	}
	if contacts == nil {
		contacts = []AuthorityContact{}
	}
	return contacts, nil
}

// CreateAuthorityContact creates a custom authority contact for an org.
func (s *Service) CreateAuthorityContact(ctx context.Context, orgID string, in AuthorityContact) (*AuthorityContact, error) {
	in.OrgID = &orgID
	in.IsBuiltin = false
	contact, err := s.repo.CreateAuthorityContact(ctx, orgID, in)
	if err != nil {
		return nil, fmt.Errorf("create authority contact: %w", err)
	}
	return contact, nil
}
