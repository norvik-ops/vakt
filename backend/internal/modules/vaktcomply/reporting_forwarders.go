// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ── KPI Dashboard forwarders ──────────────────────────────────────────────────

// CalculateAndStoreKPIs computes all ISMS KPIs for the organisation and persists
// them as a daily snapshot. Delegates to the Reporting sub-service.
func (s *Service) CalculateAndStoreKPIs(ctx context.Context, orgID string) error {
	return s.Reporting.CalculateAndStoreKPIs(ctx, orgID)
}

// GetKPIDashboard returns the latest KPI snapshot and the 90-day history.
func (s *Service) GetKPIDashboard(ctx context.Context, orgID string) (KPIDashboard, error) {
	return s.Reporting.GetKPIDashboard(ctx, orgID)
}

// ── NIS2 Art.23 forwarders ────────────────────────────────────────────────────

// MarkIncidentReportable marks an incident as NIS2-meldepflichtig and sets the three deadlines.
func (s *Service) MarkIncidentReportable(ctx context.Context, orgID string, incidentID uuid.UUID, detectedAt time.Time, check NIS2ReportabilityCheck) error {
	return s.Reporting.MarkIncidentReportable(ctx, orgID, incidentID, detectedAt, check)
}

// SubmitNIS2Stage saves report content for a stage and marks it submitted.
func (s *Service) SubmitNIS2Stage(ctx context.Context, orgID, incidentID, userID, stage string, input NIS2ReportInput) (*NIS2StageReport, error) {
	return s.Reporting.SubmitNIS2Stage(ctx, orgID, incidentID, userID, stage, input)
}

// GetNIS2Status returns the full NIS2 reporting status for an incident.
func (s *Service) GetNIS2Status(ctx context.Context, orgID, incidentID string) (*NIS2ReportStatus, error) {
	return s.Reporting.GetNIS2Status(ctx, orgID, incidentID)
}

// CheckNIS2StagingDeadlines checks all open NIS2 incidents for upcoming deadlines.
func (s *Service) CheckNIS2StagingDeadlines(ctx context.Context, orgID string) error {
	return s.Reporting.CheckNIS2StagingDeadlines(ctx, orgID)
}

// ListAuthorityContacts returns authority contacts for the given org (including built-ins).
func (s *Service) ListAuthorityContacts(ctx context.Context, orgID string) ([]AuthorityContact, error) {
	return s.Reporting.ListAuthorityContacts(ctx, orgID)
}

// CreateAuthorityContact creates a custom authority contact for an org.
func (s *Service) CreateAuthorityContact(ctx context.Context, orgID string, in AuthorityContact) (*AuthorityContact, error) {
	return s.Reporting.CreateAuthorityContact(ctx, orgID, in)
}
