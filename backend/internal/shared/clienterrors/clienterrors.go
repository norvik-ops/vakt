// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Package clienterrors persists structured errors reported by the React
// ErrorBoundary and exposes an admin view of recent entries. Extracted from
// cmd/api/main.go in S90-2 so the only raw SQL in main.go moves behind a proper
// repository (consistent with the Handler→Repository discipline + org-id lint).
package clienterrors

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/matharnica/vakt/internal/shared/logsafe"
)

// Entry is one persisted client error.
type Entry struct {
	ID             string    `json:"id"`
	OrgID          string    `json:"org_id"`
	UserID         string    `json:"user_id"`
	Message        string    `json:"message"`
	Stack          string    `json:"stack"`
	ComponentStack string    `json:"component_stack"`
	URL            string    `json:"url"`
	UserAgent      string    `json:"user_agent"`
	TraceID        string    `json:"trace_id"`
	OccurredAt     time.Time `json:"occurred_at"`
}

// RecordInput holds the (already org-resolved) fields for a new entry. OrgID and
// UserID may be nil — errors can occur before login.
type RecordInput struct {
	OrgID          *string
	UserID         *string
	Message        string
	Stack          string
	ComponentStack string
	URL            string
	UserAgent      string
	TraceID        string
}

// Repository is the data-access layer for client_errors.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository constructs a Repository.
func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

// Record persists a client error. Inputs are sanitized (log-injection / control
// chars) and length-capped before storage. Errors are returned to the caller,
// which treats persistence as best-effort.
func (r *Repository) Record(ctx context.Context, in RecordInput) error {
	msg := logsafe.SanitizeField(in.Message, 500)
	url := logsafe.SanitizeField(in.URL, 512)
	trace := logsafe.SanitizeField(in.TraceID, 64)
	stack := logsafe.SanitizeField(in.Stack, 4000)
	compStack := logsafe.SanitizeField(in.ComponentStack, 4000)
	ua := logsafe.SanitizeField(in.UserAgent, 300)

	_, err := r.db.Exec(ctx, `
		INSERT INTO client_errors
			(org_id, user_id, message, stack, component_stack, url, user_agent, trace_id)
		VALUES
			($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8)`,
		in.OrgID, in.UserID, msg, stack, compStack, url, ua, trace)
	return err
}

// ListForOrg returns the 200 most recent errors visible to the org: its own plus
// the unscoped (pre-login, org_id IS NULL) entries.
func (r *Repository) ListForOrg(ctx context.Context, orgID string) ([]Entry, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, COALESCE(org_id::text,''), COALESCE(user_id::text,''),
		       message, COALESCE(stack,''), COALESCE(component_stack,''),
		       COALESCE(url,''), COALESCE(user_agent,''), COALESCE(trace_id,''),
		       occurred_at
		FROM client_errors
		WHERE org_id = $1::uuid OR org_id IS NULL
		ORDER BY occurred_at DESC
		LIMIT 200`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Entry, 0, 50)
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.OrgID, &e.UserID, &e.Message, &e.Stack,
			&e.ComponentStack, &e.URL, &e.UserAgent, &e.TraceID, &e.OccurredAt); err != nil {
			continue
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
