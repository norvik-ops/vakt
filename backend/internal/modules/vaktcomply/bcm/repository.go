// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package bcm

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/matharnica/vakt/internal/db"
)

// Repository provides BCM-specific database operations.
type Repository struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

// NewRepository creates a new BCM repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, q: db.New(pool)}
}
