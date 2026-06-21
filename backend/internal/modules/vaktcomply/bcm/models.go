// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package bcm

import "time"

// ── S86: BIA / BCM types ──────────────────────────────────────────────────────

type BIAProcess struct {
	ID                  string    `json:"id"`
	OrgID               string    `json:"org_id"`
	Name                string    `json:"name"`
	Description         string    `json:"description"`
	ProcessOwner        string    `json:"process_owner"`
	Criticality         string    `json:"criticality"`
	Schutzbedarfsklasse int       `json:"schutzbedarfsklasse"`
	RTOHours            int       `json:"rto_hours"`
	RPOHours            int       `json:"rpo_hours"`
	MBCOPercent         int       `json:"mbco_percent"`
	Dependencies        []string  `json:"dependencies"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type CreateBIAProcessInput struct {
	Name                string   `json:"name"                 validate:"required"`
	Description         string   `json:"description"`
	ProcessOwner        string   `json:"process_owner"`
	Criticality         string   `json:"criticality"          validate:"required,oneof=high medium low"`
	Schutzbedarfsklasse int      `json:"schutzbedarfsklasse"  validate:"required,oneof=1 2 3"`
	RTOHours            int      `json:"rto_hours"            validate:"required,min=1"`
	RPOHours            int      `json:"rpo_hours"            validate:"required,min=1"`
	MBCOPercent         int      `json:"mbco_percent"         validate:"min=0,max=100"`
	Dependencies        []string `json:"dependencies"`
}

type UpdateBIAProcessInput struct {
	Name                string   `json:"name"                 validate:"required"`
	Description         string   `json:"description"`
	ProcessOwner        string   `json:"process_owner"`
	Criticality         string   `json:"criticality"          validate:"required,oneof=high medium low"`
	Schutzbedarfsklasse int      `json:"schutzbedarfsklasse"  validate:"required,oneof=1 2 3"`
	RTOHours            int      `json:"rto_hours"            validate:"required,min=1"`
	RPOHours            int      `json:"rpo_hours"            validate:"required,min=1"`
	MBCOPercent         int      `json:"mbco_percent"         validate:"min=0,max=100"`
	Dependencies        []string `json:"dependencies"`
}

type BIASummary struct {
	TotalProcesses   int         `json:"total_processes"`
	CriticalCount    int         `json:"critical_count"`
	ShortestRTOHours int         `json:"shortest_rto_hours"`
	KlasseBreakdown  map[int]int `json:"klasse_breakdown"`
}

// ── Recovery Plans ────────────────────────────────────────────────────────────

type RecoveryStep struct {
	Order       int    `json:"order"`
	Action      string `json:"action"`
	Responsible string `json:"responsible"`
	DurationMin int    `json:"duration_min"`
}

type RecoveryPlan struct {
	ID                 string         `json:"id"`
	OrgID              string         `json:"org_id"`
	BIAProcessID       *string        `json:"bia_process_id"`
	BIAProcessName     string         `json:"bia_process_name"`
	Title              string         `json:"title"`
	ActivationCriteria string         `json:"activation_criteria"`
	Responsible        string         `json:"responsible"`
	RTOHours           int            `json:"rto_hours"`
	Status             string         `json:"status"`
	Steps              []RecoveryStep `json:"steps"`
	LastTestedAt       *string        `json:"last_tested_at"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

type CreateRecoveryPlanInput struct {
	BIAProcessID       *string        `json:"bia_process_id"`
	Title              string         `json:"title"              validate:"required"`
	ActivationCriteria string         `json:"activation_criteria"`
	Responsible        string         `json:"responsible"`
	RTOHours           int            `json:"rto_hours"          validate:"required,min=1"`
	Status             string         `json:"status"             validate:"required,oneof=draft active tested archived"`
	Steps              []RecoveryStep `json:"steps"`
	LastTestedAt       *string        `json:"last_tested_at"`
}

type UpdateRecoveryPlanInput struct {
	BIAProcessID       *string        `json:"bia_process_id"`
	Title              string         `json:"title"              validate:"required"`
	ActivationCriteria string         `json:"activation_criteria"`
	Responsible        string         `json:"responsible"`
	RTOHours           int            `json:"rto_hours"          validate:"required,min=1"`
	Status             string         `json:"status"             validate:"required,oneof=draft active tested archived"`
	Steps              []RecoveryStep `json:"steps"`
	LastTestedAt       *string        `json:"last_tested_at"`
}

// ── Emergency Contacts ────────────────────────────────────────────────────────

type EmergencyContact struct {
	ID              string    `json:"id"`
	OrgID           string    `json:"org_id"`
	Name            string    `json:"name"`
	Role            string    `json:"role"`
	Phone           string    `json:"phone"`
	Email           string    `json:"email"`
	EscalationLevel int       `json:"escalation_level"`
	Available247    bool      `json:"available_24_7"`
	Notes           string    `json:"notes"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type CreateEmergencyContactInput struct {
	Name            string `json:"name"             validate:"required"`
	Role            string `json:"role"`
	Phone           string `json:"phone"`
	Email           string `json:"email"            validate:"omitempty,email"`
	EscalationLevel int    `json:"escalation_level" validate:"required,oneof=1 2 3"`
	Available247    bool   `json:"available_24_7"`
	Notes           string `json:"notes"`
}

type UpdateEmergencyContactInput struct {
	Name            string `json:"name"             validate:"required"`
	Role            string `json:"role"`
	Phone           string `json:"phone"`
	Email           string `json:"email"            validate:"omitempty,email"`
	EscalationLevel int    `json:"escalation_level" validate:"required,oneof=1 2 3"`
	Available247    bool   `json:"available_24_7"`
	Notes           string `json:"notes"`
}

// ── S60: BCP / Notfallhandbuch ────────────────────────────────────────────────

// BCPPlan represents a Business Continuity Plan document.
type BCPPlan struct {
	ID                  string    `json:"id"`
	OrgID               string    `json:"org_id"`
	Title               string    `json:"title"`
	Scope               string    `json:"scope"`
	Version             string    `json:"version"`
	Status              string    `json:"status"`
	Owner               string    `json:"owner"`
	RTOHours            int       `json:"rto_hours"`
	RPOHours            int       `json:"rpo_hours"`
	Schutzbedarfsklasse int       `json:"schutzbedarfsklasse"`
	LastTestedAt        *string   `json:"last_tested_at"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// BCPTest represents a single BCP test record for a plan.
type BCPTest struct {
	ID        string    `json:"id"`
	OrgID     string    `json:"org_id"`
	PlanID    string    `json:"plan_id"`
	TestDate  string    `json:"test_date"`
	TestType  string    `json:"test_type"`
	Outcome   string    `json:"outcome"`
	Findings  string    `json:"findings"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateBCPPlanInput is the request body for creating a BCP plan.
type CreateBCPPlanInput struct {
	Title   string `json:"title"   validate:"required"`
	Scope   string `json:"scope"`
	Version string `json:"version"`
	Status  string `json:"status"  validate:"omitempty,oneof=draft active archived"`
	Owner   string `json:"owner"`
}

// UpdateBCPPlanInput is the request body for updating a BCP plan.
type UpdateBCPPlanInput struct {
	Title   string `json:"title"   validate:"required"`
	Scope   string `json:"scope"`
	Version string `json:"version"`
	Status  string `json:"status"  validate:"required,oneof=draft active archived"`
	Owner   string `json:"owner"`
}

// CreateBCPTestInput is the request body for logging a BCP test.
type CreateBCPTestInput struct {
	TestDate string `json:"test_date" validate:"required"`
	TestType string `json:"test_type" validate:"required,oneof=tabletop walkthrough fulltest"`
	Outcome  string `json:"outcome"   validate:"required,oneof=passed failed partial"`
	Findings string `json:"findings"`
}

// LinkBCPPlanEvidenceInput optionally carries a control_id to link the plan as evidence.
type LinkBCPPlanEvidenceInput struct {
	ControlID string `json:"control_id"`
}

// ── BCM Score ─────────────────────────────────────────────────────────────────

type BCMReadinessScore struct {
	Score    int            `json:"score"`
	Criteria []BCMCriterion `json:"criteria"`
}

type BCMCriterion struct {
	Key    string `json:"key"`
	Met    bool   `json:"met"`
	Points int    `json:"points"`
}
