// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package risk

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
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
