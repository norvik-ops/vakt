// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package bsi

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Service provides BSI IT-Grundschutz business logic.
type Service struct {
	db     *pgxpool.Pool
	repo   *Repository
	scorer ComplianceScorer
}

// NewService creates a new BSI service.
func NewService(pool *pgxpool.Pool) *Service {
	return &Service{db: pool, repo: NewRepository(pool), scorer: KompendiumScorer{}}
}

// Sentinel errors for the BSI service layer. Handlers use errors.Is to map
// these to HTTP status codes without fragile string matching.
var (
	// ErrNotFound is returned when a requested resource does not exist.
	ErrNotFound = errors.New("not found")
	// ErrConflict is returned when a uniqueness constraint is violated.
	ErrConflict = errors.New("resource already exists")
	// ErrCycle is returned when adding a dependency would create a cycle.
	ErrCycle = errors.New("adding this dependency would create a cycle")
	// ErrOverrideReasonMissing is returned when an override is set without a reason.
	ErrOverrideReasonMissing = errors.New("override_reason is required when setting an override")
)

// IsNotFound returns true for any "resource does not exist" error — either the
// service-layer ErrNotFound sentinel or a raw pgx.ErrNoRows from the repository.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound) || errors.Is(err, pgx.ErrNoRows)
}

// isUniqueViolation returns true for PostgreSQL SQLSTATE 23505 (unique_violation).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
