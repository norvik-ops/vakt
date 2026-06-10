// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S69-3: SLA Enforcement for Findings
// Per-severity SLA policies with overdue detection and summary endpoint.

package vaktscan

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// SLAPolicy is one row from vb_sla_policies.
type SLAPolicy struct {
	ID                      string    `json:"id"`
	OrgID                   string    `json:"org_id"`
	Severity                string    `json:"severity"`
	RemediationDays         int       `json:"remediation_days"`
	NotificationAdvanceDays int       `json:"notification_advance_days"`
	IsDefault               bool      `json:"is_default"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
}

// SLASummary is returned by GetSLASummary.
type SLASummary struct {
	TotalOpen    int            `json:"total_open"`
	Overdue      int            `json:"overdue"`
	AtRisk       int            `json:"at_risk"`
	OnTrack      int            `json:"on_track"`
	BySeverity   map[string]int `json:"by_severity"`
	OverdueBySev map[string]int `json:"overdue_by_severity"`
}

// EnsureDefaultSLAPolicies creates default SLA policies for the org if none exist.
// Defaults: critical=7d, high=30d, medium=90d, low=180d, info=365d.
func (s *Service) EnsureDefaultSLAPolicies(ctx context.Context, orgID string) error {
	existing, err := s.repo.ListSLAPolicies(ctx, orgID)
	if err != nil {
		return fmt.Errorf("list sla policies: %w", err)
	}
	if len(existing) > 0 {
		return nil
	}

	defaults := []struct {
		severity    string
		remDays     int
		advanceDays int
	}{
		{"critical", 7, 2},
		{"high", 30, 5},
		{"medium", 90, 7},
		{"low", 180, 14},
		{"info", 365, 30},
	}
	for _, d := range defaults {
		if err := s.repo.CreateSLAPolicy(ctx, orgID, d.severity, d.remDays, d.advanceDays, true); err != nil {
			log.Warn().Err(err).Str("severity", d.severity).Msg("failed to create default SLA policy")
		}
	}
	return nil
}

// GetSLASummary returns a summary of open findings against their SLA deadlines.
func (s *Service) GetSLASummary(ctx context.Context, orgID string) (*SLASummary, error) {
	rows, err := s.repo.GetSLASummaryRows(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("get sla summary: %w", err)
	}

	summary := &SLASummary{
		BySeverity:   make(map[string]int),
		OverdueBySev: make(map[string]int),
	}
	for _, r := range rows {
		summary.TotalOpen += r.Count
		summary.BySeverity[r.Severity] += r.Count
		switch r.SLAStatus {
		case "overdue":
			summary.Overdue += r.Count
			summary.OverdueBySev[r.Severity] += r.Count
		case "at_risk":
			summary.AtRisk += r.Count
		case "on_track":
			summary.OnTrack += r.Count
		}
	}
	return summary, nil
}

// RunSLACheckForOrg updates sla_status on all open findings for one org.
// Called by the daily sla_check Asynq worker.
func RunSLACheckForOrg(ctx context.Context, pool *pgxpool.Pool, orgID string) error {
	repo := NewRepository(pool)

	policies, err := repo.ListSLAPolicies(ctx, orgID)
	if err != nil {
		return fmt.Errorf("list policies: %w", err)
	}
	policyMap := make(map[string]SLAPolicy, len(policies))
	for _, p := range policies {
		policyMap[p.Severity] = p
	}

	findings, err := repo.ListOpenFindingsWithSLA(ctx, orgID)
	if err != nil {
		return fmt.Errorf("list findings with sla: %w", err)
	}

	now := time.Now()
	for i := range findings {
		f := &findings[i]
		if f.SLADueAt == nil {
			pol, ok := policyMap[f.Severity]
			if !ok {
				continue
			}
			due := f.CreatedAt.Add(time.Duration(pol.RemediationDays) * 24 * time.Hour)
			if err := repo.SetFindingSLADue(ctx, orgID, f.ID, due); err != nil {
				log.Warn().Err(err).Str("finding", f.ID).Msg("set sla_due_at failed")
			}
			f.SLADueAt = &due
		}

		newStatus := "on_track"
		pol, hasPol := policyMap[f.Severity]
		if f.SLADueAt.Before(now) {
			newStatus = "overdue"
		} else if hasPol && f.SLADueAt.Before(now.Add(time.Duration(pol.NotificationAdvanceDays)*24*time.Hour)) {
			newStatus = "at_risk"
		}

		if err := repo.UpdateFindingSLAStatus(ctx, orgID, f.ID, newStatus); err != nil {
			log.Warn().Err(err).Str("finding", f.ID).Msg("update sla_status failed")
		}
	}

	log.Info().Str("org_id", orgID).Int("findings", len(findings)).Msg("SLA check complete")
	return nil
}
