// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package policy

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/matharnica/vakt/internal/db"
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

// DeleteFramework removes a framework and all its controls/evidence (cascade).
func (r *Repository) DeleteFramework(ctx context.Context, orgID, frameworkID string) error {
	n, err := r.q.DeleteCKFramework(ctx, db.DeleteCKFrameworkParams{ID: frameworkID, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("delete framework: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("framework not found")
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
