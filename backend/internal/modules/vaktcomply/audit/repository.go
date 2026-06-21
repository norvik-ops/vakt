// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package audit

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/matharnica/vakt/internal/db"
)

// Repository provides audit-domain database operations.
type Repository struct {
	db *pgxpool.Pool
	q  *db.Queries
}

// NewRepository creates a new audit repository.
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

// ckTsToTimePtr converts pgtype.Timestamptz to *time.Time (nil on NULL).
func ckTsToTimePtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	tm := t.Time
	return &tm
}

// ckOptText: empty string → invalid pgtype.Text (NULL in DB).
func ckOptText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

// ckOptUUIDFromStr converts a string to pgtype.UUID; empty → invalid.
func ckOptUUIDFromStr(s string) pgtype.UUID {
	if s == "" {
		return pgtype.UUID{}
	}
	var u pgtype.UUID
	_ = u.Scan(s)
	return u
}

// uuidStringFromPgtype returns the UUID as string ("" when invalid).
func uuidStringFromPgtype(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return u.String()
}
