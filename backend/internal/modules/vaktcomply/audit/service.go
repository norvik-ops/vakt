// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package audit

import (
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a requested audit resource does not exist.
var ErrNotFound = errors.New("not found")

// Service provides internal-audit, audit-program, management-review and
// approval-workflow business logic.
type Service struct {
	db   *pgxpool.Pool
	repo *Repository
}

// NewService creates a new audit service.
func NewService(pool *pgxpool.Pool) *Service {
	return &Service{db: pool, repo: NewRepository(pool)}
}
