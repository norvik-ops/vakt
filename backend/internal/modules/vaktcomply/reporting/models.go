// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Package reporting holds the reporting-and-monitoring domain of vaktcomply.
// Currently it owns Continuous Control Monitoring (CCM): automated, scheduled
// compliance checks (HTTP endpoint probes, Trivy critical-finding gates,
// evidence-freshness checks) plus their execution results. The domain is
// self-contained — it depends only on the generated db layer and the shared
// pgxpool, never on the parent vaktcomply package — so it can grow to absorb
// further reporting concerns (KPI dashboards, NIS2 stage reports) as those are
// decoupled from root-level shared types in later steps.
package reporting

import "time"

// CCMCheck represents an automated compliance control check definition.
type CCMCheck struct {
	ID            string            `json:"id"`
	OrgID         string            `json:"org_id"`
	ControlID     string            `json:"control_id"`
	Name          string            `json:"name"`
	CheckType     string            `json:"check_type"`
	Config        map[string]string `json:"config"`
	IntervalHours int               `json:"interval_hours"`
	LastRunAt     *time.Time        `json:"last_run_at,omitempty"`
	LastStatus    string            `json:"last_status,omitempty"`
	LastOutput    string            `json:"last_output,omitempty"`
	Enabled       bool              `json:"enabled"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

// CreateCCMCheckInput holds validated input for creating a CCM check.
type CreateCCMCheckInput struct {
	ControlID     string            `json:"control_id"     validate:"required"`
	Name          string            `json:"name"           validate:"required,max=255"`
	CheckType     string            `json:"check_type"     validate:"required,oneof=http_endpoint trivy_no_critical evidence_freshness custom_script"`
	Config        map[string]string `json:"config"`
	IntervalHours int               `json:"interval_hours" validate:"min=1,max=8760"`
}

// ToggleCCMCheckInput holds the enabled flag for toggling a CCM check.
type ToggleCCMCheckInput struct {
	Enabled bool `json:"enabled"`
}

// CCMResult represents the result of a single CCM check execution.
type CCMResult struct {
	ID      string    `json:"id"`
	CheckID string    `json:"check_id"`
	Status  string    `json:"status"`
	Output  string    `json:"output,omitempty"`
	RanAt   time.Time `json:"ran_at"`
}
