// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S89-5: guided "first 30 days" ISB path. The per-step completion status is
// derived live from real org data (e.g. step "risks" is done once the org has
// at least one risk), so the checklist always reflects what the customer has
// actually done — no separate progress bookkeeping to drift out of sync.
//
// This extends the existing onboarding package (which already persists the
// dismiss flag in organizations.onboarding_dismissed and backs the 4-step
// wizard) rather than introducing a parallel system. Pure SQL, no module
// imports — cross-module isolation preserved.

package onboarding

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ProgressStep is one guided step. Key maps to an i18n string; Path is the
// in-app route that performs the step (links to existing functionality).
type ProgressStep struct {
	Key  string `json:"key"`
	Done bool   `json:"done"`
	Path string `json:"path"`
}

// Progress is the full "first 30 days" state for an org.
type Progress struct {
	Steps          []ProgressStep `json:"steps"`
	CompletedCount int            `json:"completed_count"`
	Total          int            `json:"total"`
	PercentDone    int            `json:"percent_done"`
	Dismissed      bool           `json:"dismissed"`
	AllComplete    bool           `json:"all_complete"`
}

// progressStepDef defines a step's stable key, deep link, and the EXISTS query
// (org-scoped via $1) that proves it is done.
type progressStepDef struct {
	key   string
	path  string
	query string
}

// progressSteps is the canonical 7-step path. Order is meaningful.
var progressSteps = []progressStepDef{
	{"scope", "/vaktcomply/isms-scope",
		`SELECT EXISTS(SELECT 1 FROM ck_isms_scope WHERE org_id = $1::uuid)`},
	{"assets", "/vaktscan/assets",
		`SELECT EXISTS(SELECT 1 FROM vb_assets WHERE org_id = $1::uuid AND is_deleted = FALSE)`},
	{"protection_need", "/vaktcomply/bsi/target-objects",
		`SELECT EXISTS(SELECT 1 FROM ck_protection_need_assessments WHERE org_id = $1::uuid)`},
	{"framework", "/vaktcomply/frameworks",
		`SELECT EXISTS(SELECT 1 FROM ck_frameworks WHERE org_id = $1::uuid)`},
	{"risks", "/vaktcomply/risks",
		`SELECT EXISTS(SELECT 1 FROM ck_risks WHERE org_id = $1::uuid)`},
	{"evidence", "/vaktcomply/evidence",
		`SELECT EXISTS(SELECT 1 FROM ck_evidence WHERE org_id = $1::uuid)`},
	{"policy", "/vaktcomply/policies",
		`SELECT EXISTS(SELECT 1 FROM ck_policies WHERE org_id = $1::uuid)`},
}

// GetProgress computes the live per-step status and reads the dismiss flag from
// the existing organizations.onboarding_dismissed column.
func GetProgress(ctx context.Context, db *pgxpool.Pool, orgID string) (Progress, error) {
	p := Progress{Total: len(progressSteps), Steps: make([]ProgressStep, 0, len(progressSteps))}
	for _, s := range progressSteps {
		var done bool
		if err := db.QueryRow(ctx, s.query, orgID).Scan(&done); err != nil {
			return Progress{}, fmt.Errorf("onboarding step %q: %w", s.key, err)
		}
		if done {
			p.CompletedCount++
		}
		p.Steps = append(p.Steps, ProgressStep{Key: s.key, Done: done, Path: s.path})
	}
	if p.Total > 0 {
		p.PercentDone = p.CompletedCount * 100 / p.Total
	}
	p.AllComplete = p.CompletedCount == p.Total

	_ = db.QueryRow(ctx,
		`SELECT COALESCE(onboarding_dismissed, false) FROM organizations WHERE id = $1::uuid`,
		orgID).Scan(&p.Dismissed)
	return p, nil
}
