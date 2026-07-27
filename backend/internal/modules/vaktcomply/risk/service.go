// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package risk

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	sharedevents "github.com/matharnica/vakt/internal/shared/events"
)

// Service handles the risk domain of vaktcomply (risks, DORA third parties,
// protection needs, CAPA NC/effectiveness, control exceptions).
type Service struct {
	db              *pgxpool.Pool
	repo            *Repository
	invalidateCache func(context.Context, string)
}

// NewService creates a new risk-domain service.
func NewService(pool *pgxpool.Pool) *Service {
	return &Service{db: pool, repo: NewRepository(pool)}
}

// WithCacheInvalidator injects the dashboard cache-invalidation function from the parent service.
func (s *Service) WithCacheInvalidator(fn func(context.Context, string)) {
	s.invalidateCache = fn
}

// WithAssetProtectionLinker injects the vaktscan-backed writer for the reverse
// protection_need_id link on vb_assets (module isolation, ADR-0079). Wired from
// cmd/api; the repository keeps a no-op default when it is absent.
func (s *Service) WithAssetProtectionLinker(l sharedevents.AssetProtectionLinker) *Service {
	s.repo.WithAssetProtectionLinker(l)
	return s
}
