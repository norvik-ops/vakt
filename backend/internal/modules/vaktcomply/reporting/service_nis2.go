// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package reporting

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/shared/notify"
)

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
	inc, err := s.repo.GetNIS2Incident(ctx, orgID, incidentID)
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

// CheckNIS2StagingDeadlines checks all open NIS2 incidents for upcoming deadlines.
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

func (s *Service) sendNIS2DeadlineNotification(ctx context.Context, orgID string, inc NIS2IncidentRow, stageName string, deadline time.Time) {
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
