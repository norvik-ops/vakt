// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package bsi

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/matharnica/vakt/internal/db"
)

// Repository provides BSI-specific database operations.
type Repository struct {
	db *pgxpool.Pool
	q  *db.Queries
}

// NewRepository creates a new BSI repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{db: pool, q: db.New(pool)}
}

// ckTsToTime converts pgtype.Timestamptz to time.Time (zero on NULL).
func ckTsToTime(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}
