// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/matharnica/vakt/internal/modules/vaktcomply/policy"
)

// Thin forwarders that preserve the root vaktcomply API for startup wiring that
// lives outside the package (cmd/api). The implementations moved into the
// policy/ sub-package (ADR-0066); these delegate to s.Policy.

// ReseedBuiltinControls reseeds controls for all builtin frameworks across all orgs.
func (s *Service) ReseedBuiltinControls(ctx context.Context) {
	s.Policy.ReseedBuiltinControls(ctx)
}

// SeedFrameworkMappings idempotently seeds the global cross-framework control mappings.
func (s *Service) SeedFrameworkMappings(ctx context.Context) error {
	return s.Policy.SeedFrameworkMappings(ctx)
}

// SeedPrerequisiteChains seeds the global control prerequisite chains.
func (s *Service) SeedPrerequisiteChains(ctx context.Context) error {
	return s.Policy.SeedPrerequisiteChains(ctx)
}

// SeedPolicyTemplates re-exports the policy-template seeder for startup wiring.
func SeedPolicyTemplates(ctx context.Context, db *pgxpool.Pool) error {
	return policy.SeedPolicyTemplates(ctx, db)
}
