// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package comments

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/matharnica/vakt/internal/shared/apperr"
)

// Comment is the canonical comment record returned to callers.
type Comment struct {
	ID         string    `json:"id"`
	AuthorName string    `json:"author_name"`
	AuthorID   string    `json:"author_id"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
	CanDelete  bool      `json:"can_delete"` // true if current user is author or admin
}

// Repository handles comment data access.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new comments repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// ListComments returns all non-deleted comments for an entity, enriched with
// author display names and a can_delete flag for the requesting user.
func (r *Repository) ListComments(
	ctx context.Context,
	orgID, entityType, entityID, currentUserID string,
	isAdmin bool,
) ([]Comment, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
			c.id::text,
			COALESCE(u.display_name, u.email) AS author_name,
			c.author_id::text,
			c.content,
			c.created_at
		FROM comments c
		JOIN users u ON u.id = c.author_id
		WHERE c.org_id     = $1::uuid
		  AND c.entity_type = $2
		  AND c.entity_id  = $3::uuid
		  AND c.deleted_at IS NULL
		ORDER BY c.created_at ASC`,
		orgID, entityType, entityID,
	)
	if err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}
	defer rows.Close()

	var out []Comment
	for rows.Next() {
		var cmt Comment
		if err := rows.Scan(&cmt.ID, &cmt.AuthorName, &cmt.AuthorID, &cmt.Content, &cmt.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan comment: %w", err)
		}
		cmt.CanDelete = isAdmin || cmt.AuthorID == currentUserID
		out = append(out, cmt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate comments: %w", err)
	}
	return out, nil
}

// CreateComment inserts a new comment and returns the created record.
func (r *Repository) CreateComment(
	ctx context.Context,
	orgID, entityType, entityID, authorID, content string,
) (Comment, error) {
	var cmt Comment
	err := r.db.QueryRow(ctx, `
		INSERT INTO comments (org_id, entity_type, entity_id, author_id, content)
		VALUES ($1::uuid, $2, $3::uuid, $4::uuid, $5)
		RETURNING id::text, $4::text, author_id::text, content, created_at`,
		orgID, entityType, entityID, authorID, content,
	).Scan(&cmt.ID, &cmt.AuthorName, &cmt.AuthorID, &cmt.Content, &cmt.CreatedAt)
	if err != nil {
		return Comment{}, fmt.Errorf("create comment: %w", err)
	}

	// Resolve display name — best-effort, fall back to author ID on error.
	var displayName string
	_ = r.db.QueryRow(ctx,
		`SELECT COALESCE(display_name, email) FROM users WHERE id = $1::uuid`, authorID,
	).Scan(&displayName)
	if displayName != "" {
		cmt.AuthorName = displayName
	}
	cmt.CanDelete = true // creator can always delete their own comment
	return cmt, nil
}

// DeleteComment soft-deletes a comment if the caller is the author or an admin.
// Returns an error if the comment is not found or the caller is not permitted.
func (r *Repository) DeleteComment(
	ctx context.Context,
	orgID, commentID, callerID string,
	isAdmin bool,
) error {
	var authorID string
	err := r.db.QueryRow(ctx,
		`SELECT author_id::text FROM comments
		 WHERE id = $1::uuid AND org_id = $2::uuid AND deleted_at IS NULL`,
		commentID, orgID,
	).Scan(&authorID)
	if err != nil {
		return fmt.Errorf("comment %w", apperr.ErrNotFound)
	}

	if !isAdmin && authorID != callerID {
		return fmt.Errorf("permission denied")
	}

	_, err = r.db.Exec(ctx,
		`UPDATE comments SET deleted_at = NOW(), updated_at = NOW()
		 WHERE id = $1::uuid AND org_id = $2::uuid`,
		commentID, orgID,
	)
	if err != nil {
		return fmt.Errorf("soft-delete comment: %w", err)
	}
	return nil
}
