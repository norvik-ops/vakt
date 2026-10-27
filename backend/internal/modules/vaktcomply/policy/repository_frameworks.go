// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package policy

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/matharnica/vakt/internal/db"
	"github.com/matharnica/vakt/internal/shared/apperr"
)

// --- Frameworks ---

// CreateFramework inserts a new framework for an organisation.
// variant is "full" or "simplified" (DORA Art. 16); empty defaults to "full".
func (r *Repository) CreateFramework(ctx context.Context, orgID, name, version string, isBuiltin bool, variant string) (*Framework, error) {
	if variant == "" {
		variant = "full"
	}
	row, err := r.q.CreateCKFramework(ctx, db.CreateCKFrameworkParams{
		OrgID:            orgID,
		Name:             name,
		Version:          version,
		IsBuiltin:        isBuiltin,
		FrameworkVariant: variant,
	})
	if err != nil {
		return nil, fmt.Errorf("create framework: %w", err)
	}
	f := frameworkFromCkFrameworks(row)
	return &f, nil
}

// UpdateFrameworkVariant sets the framework_variant column for a framework.
func (r *Repository) UpdateFrameworkVariant(ctx context.Context, orgID, frameworkID, variant string) error {
	return r.q.UpdateCKFrameworkVariant(ctx, db.UpdateCKFrameworkVariantParams{
		FrameworkVariant: variant,
		ID:               frameworkID,
		OrgID:            orgID,
	})
}

// ListFrameworks returns all frameworks enabled for an organisation.
func (r *Repository) ListFrameworks(ctx context.Context, orgID string) ([]Framework, error) {
	rows, err := r.q.ListCKFrameworks(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list frameworks: %w", err)
	}
	out := make([]Framework, 0, len(rows))
	for _, row := range rows {
		out = append(out, frameworkFromCkFrameworks(row))
	}
	return out, nil
}

// ReadinessScoresByFramework rechnet den Bereitschaftsgrad je Framework der
// Organisation aus — in EINER Abfrage ueber alle Controls, nicht einer je
// Framework.
//
// R1-18-D3: Framework.ReadinessScore traegt `omitempty` und wurde von
// ListFrameworks nie gefuellt. Der Wert blieb 0 und fiel damit ganz aus der
// JSON-Antwort; das Auditor-Portal zeigte "Bereitschaft %" ohne Zahl. Der
// Kommentar an frameworkFromCkFrameworks behauptete, der Score werde "computed
// per-call in service layer" — an dieser Stelle geschah das nicht.
//
// Gerechnet wird mit ResolveStatus und ReadinessScore, also mit denselben
// Funktionen wie ComputeReadinessReport. Nicht-anwendbare Controls bleiben
// aussen vor, genau wie dort.
func (r *Repository) ReadinessScoresByFramework(ctx context.Context, orgID string) (map[string]float64, error) {
	byFW, err := r.ReadinessByFramework(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]float64, len(byFW))
	for fwID, t := range byFW {
		out[fwID] = t.Score
	}
	return out, nil
}

// FrameworkReadiness is the canonical readiness tally of one framework
// (ADR-0086): Covered = effective status implemented/covered, Partial =
// in_progress/partial (half weight), Applicable = all controls except
// not_applicable, Score = ReadinessScore over them.
type FrameworkReadiness struct {
	Covered    int
	Partial    int
	Applicable int
	Score      float64
}

// ReadinessByFramework returns the canonical readiness tally per framework of
// the organisation, from one query over all controls. Frameworks without
// controls are absent from the map.
//
// R1-KW-6: the dashboard counted manual_status = implemented over ALL
// controls with its own SQL — evidence ignored, not_applicable in the
// denominator — and so showed a different percentage than the framework
// overview and detail. It reads this tally now.
func (r *Repository) ReadinessByFramework(ctx context.Context, orgID string) (map[string]FrameworkReadiness, error) {
	rows, err := r.q.ListCKFrameworkReadinessRows(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("framework readiness rows: %w", err)
	}
	return TallyReadiness(rows), nil
}

// PoolReadiness is the org-wide readiness over several frameworks, pooled as
// ADR-0086 prescribes for aggregates: sum Covered, Partial and Applicable over
// all frameworks, then apply ReadinessScore ONCE — not an average of the
// per-framework percentages.
func PoolReadiness(byFW map[string]FrameworkReadiness) FrameworkReadiness {
	var out FrameworkReadiness
	for _, t := range byFW {
		out.Covered += t.Covered
		out.Partial += t.Partial
		out.Applicable += t.Applicable
	}
	out.Score = ReadinessScore(out.Covered, out.Partial, out.Applicable)
	return out
}

// TallyReadiness folds readiness rows (one per control) into a
// FrameworkReadiness per framework, with ResolveStatus and ReadinessScore —
// the same functions ComputeReadinessReport uses.
func TallyReadiness(rows []db.ListCKFrameworkReadinessRowsRow) map[string]FrameworkReadiness {
	byFW := map[string]FrameworkReadiness{}
	for _, row := range rows {
		t := byFW[row.FrameworkID]
		switch ResolveStatus(Control{
			NotApplicable: row.NotApplicable,
			ManualStatus:  row.ManualStatus,
			EvidenceCount: int(row.EvidenceCount),
		}) {
		case "not_applicable":
			byFW[row.FrameworkID] = t
			continue
		case "covered", "implemented":
			t.Covered++
		case "partial", "in_progress":
			t.Partial++
		}
		t.Applicable++
		byFW[row.FrameworkID] = t
	}
	for fwID, t := range byFW {
		t.Score = ReadinessScore(t.Covered, t.Partial, t.Applicable)
		byFW[fwID] = t
	}
	return byFW
}

// DeleteFramework removes a framework and all its controls/evidence (cascade).
func (r *Repository) DeleteFramework(ctx context.Context, orgID, frameworkID string) error {
	n, err := r.q.DeleteCKFramework(ctx, db.DeleteCKFrameworkParams{ID: frameworkID, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("delete framework: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("framework %w", apperr.ErrNotFound)
	}
	return nil
}

// GetFramework returns a single framework by ID within an organisation.
func (r *Repository) GetFramework(ctx context.Context, orgID, frameworkID string) (*Framework, error) {
	row, err := r.q.GetCKFramework(ctx, db.GetCKFrameworkParams{ID: frameworkID, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("get framework: %w", err)
	}
	f := frameworkFromCkFrameworks(row)
	return &f, nil
}

// FindFrameworkByName returns a single framework by name within an organisation.
// Returns nil, nil if not found.
func (r *Repository) FindFrameworkByName(ctx context.Context, orgID, name string) (*Framework, error) {
	row, err := r.q.FindCKFrameworkByName(ctx, db.FindCKFrameworkByNameParams{OrgID: orgID, Name: name})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find framework by name: %w", err)
	}
	f := frameworkFromCkFrameworks(row)
	return &f, nil
}

// ListAllBuiltinFrameworks returns all builtin frameworks across all organisations.
// Used for startup reseeding of controls.
func (r *Repository) ListAllBuiltinFrameworks(ctx context.Context) ([]Framework, error) {
	rows, err := r.q.ListAllBuiltinCKFrameworks(ctx)
	if err != nil {
		return nil, fmt.Errorf("list all builtin frameworks: %w", err)
	}
	out := make([]Framework, 0, len(rows))
	for _, row := range rows {
		out = append(out, frameworkFromCkFrameworks(row))
	}
	return out, nil
}

// FrameworkExists reports whether a framework with the given name already exists for the org.
func (r *Repository) FrameworkExists(ctx context.Context, orgID, name string) (bool, error) {
	exists, err := r.q.CKFrameworkExists(ctx, db.CKFrameworkExistsParams{OrgID: orgID, Name: name})
	if err != nil {
		return false, fmt.Errorf("framework exists check: %w", err)
	}
	return exists, nil
}
