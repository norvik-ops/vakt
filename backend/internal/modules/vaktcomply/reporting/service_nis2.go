// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package reporting

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/shared/notify"
)

// ErrNoDiscoveryTime signals that an incident carries no usable discovery
// timestamp, so no NIS2 deadline can be anchored. The caller answers 422 and
// asks for the discovery time — see MarkIncidentReportable for why we refuse
// instead of substituting a value.
var ErrNoDiscoveryTime = errors.New("incident has no usable discovered_at; cannot anchor NIS2 deadlines")

// earliestPlausibleDiscovery guards against a Go zero time (0001-01-01) written
// into the NOT NULL discovered_at column. Anything before this is not a real
// discovery timestamp.
var earliestPlausibleDiscovery = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

// NIS2Assessment is the verdict returned by MarkIncidentReportable: whether the
// incident is meldepflichtig, and which timestamp the deadlines were anchored
// to. DetectedAt is echoed back deliberately — an operator who is shown an
// already expired deadline must be able to see WHICH moment it was counted from.
type NIS2Assessment struct {
	IsReportable bool      `json:"is_reportable"`
	DetectedAt   time.Time `json:"detected_at"`
}

// NIS2Deadlines24_72_1M returns the three NIS2 Art. 23(4) deadlines for a
// discovery timestamp: early warning at +24h, incident notification at +72h,
// final report one CALENDAR month later.
//
// The final report used to be 30*24h. One month is a calendar period, not 30
// days: 12 such periods are 360 days, and the due date drifts through the month
// on every renewal. addMonthsClamped keeps the day-of-month and clamps to the
// last day of the target month, so 31 January + 1 month is 28 February, not
// 3 March (which is what time.AddDate alone would produce, because it
// normalises overflow instead of clamping).
//
// Known deviation, deliberately not changed here: Art. 23(4)(d) counts the final
// report from the SUBMISSION of the 72h notification, not from discovery. Our
// anchor is discovery, which is always the earlier — and therefore stricter —
// date, so the UI can never show an already expired deadline as still running.
// Moving the anchor to the actual submission is a product decision, not a bugfix.
func NIS2Deadlines24_72_1M(discoveredAt time.Time) (earlyWarning, fullReport, finalReport time.Time) {
	return discoveredAt.Add(24 * time.Hour),
		discoveredAt.Add(72 * time.Hour),
		addMonthsClamped(discoveredAt, 1)
}

// addMonthsClamped adds calendar months and clamps the day to the last day of
// the target month. time.AddDate normalises instead of clamping, which pushes
// a month-end date into the following month.
func addMonthsClamped(t time.Time, months int) time.Time {
	y, m, d := t.Date()
	hh, mi, s := t.Clock()
	// Day 1 of the target month, so the month arithmetic cannot overflow.
	target := time.Date(y, m+time.Month(months), 1, hh, mi, s, t.Nanosecond(), t.Location())
	if last := daysInMonth(target.Year(), target.Month()); d > last {
		d = last
	}
	return time.Date(target.Year(), target.Month(), d, hh, mi, s, t.Nanosecond(), t.Location())
}

// daysInMonth returns the number of days in the given month. Day 0 of the next
// month is the last day of this one.
func daysInMonth(year int, m time.Month) int {
	return time.Date(year, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// MarkIncidentReportable marks an incident as NIS2-meldepflichtig and sets the three deadlines.
//
// The deadline anchor is the incident's own discovered_at — NIS2 Art. 23(4)
// counts from becoming aware (Kenntnisnahme), not from the moment an operator
// happens to open the assessment dialog. The anchor is deliberately NOT taken
// from the request body: the previous version used time.Now(), so an incident
// discovered three days ago was shown a 24-hour deadline starting now, while the
// legal deadline had in fact expired two days earlier.
//
// If discovered_at is unusable (a Go zero time written into the NOT NULL
// column), this returns ErrNoDiscoveryTime and writes nothing. A deadline
// counted from now() looks plausible and walks the operator into a missed legal
// deadline; a deadline counted from year 1 is expired by two millennia. Both are
// worse than an honest refusal that names the missing datum.
func (s *Service) MarkIncidentReportable(ctx context.Context, orgID string, incidentID uuid.UUID, check NIS2ReportabilityCheck) (*NIS2Assessment, error) {
	inc, err := s.repo.GetNIS2Incident(ctx, orgID, incidentID.String())
	if err != nil {
		return nil, fmt.Errorf("get incident: %w", err)
	}
	detectedAt := inc.DiscoveredAt.UTC()
	if detectedAt.IsZero() || detectedAt.Before(earliestPlausibleDiscovery) {
		return nil, ErrNoDiscoveryTime
	}

	earlyWarning, fullReport, finalReport := NIS2Deadlines24_72_1M(detectedAt)

	if err := s.repo.SetNIS2Reportable(ctx, orgID, incidentID.String(), detectedAt, earlyWarning, fullReport, finalReport, check.IsReportable()); err != nil {
		return nil, fmt.Errorf("set nis2 reportable: %w", err)
	}
	log.Info().
		Str("org_id", orgID).
		Str("incident_id", incidentID.String()).
		Time("detected_at", detectedAt).
		Time("early_warning_due", earlyWarning).
		Bool("is_reportable", check.IsReportable()).
		Msg("nis2 reportability assessed")
	return &NIS2Assessment{IsReportable: check.IsReportable(), DetectedAt: detectedAt}, nil
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
	// R1-W4A-N1: Vorher stand hier ein Send ohne Rueckgabewert und darunter
	// bedingungslos „notification sent". Der Zwischenaufruf hat den Fehler
	// nicht erzeugt, aber er hat ihn weitergereicht: eine Funktion ohne
	// Rueckgabewert kann ihren Aufrufern nichts anderes anbieten als eine
	// Behauptung.
	if err := notify.Send(ctx, s.db, orgID, title, body, "nis2_deadline", "vaktcomply"); err != nil {
		log.Error().Err(err).Str("org_id", orgID).Str("incident_id", inc.ID).Str("stage", stageName).
			Msg("nis2 deadline notification NICHT geschrieben")
		return
	}
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
