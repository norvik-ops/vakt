// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Package apikeys provides CRUD operations for personal API keys that allow
// programmatic access to the Vakt API (CI/CD, integrations).
package apikeys

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// APIKey is the public representation of a key (raw secret never returned after creation).
type APIKey struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	KeyPrefix  string     `json:"key_prefix"`
	Scopes     []string   `json:"scopes"`
	LastUsedAt *time.Time `json:"last_used_at"`
	LastUsedIP *string    `json:"last_used_ip,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at"`
	CreatedAt  time.Time  `json:"created_at"`
	RotatedAt  *time.Time `json:"rotated_at,omitempty"`
}

// CreateResult is returned once on key creation — the raw key is included here
// and is never stored or returned again.
type CreateResult struct {
	APIKey
	RawKey string `json:"raw_key"`
}

// CreateInput contains the user-supplied fields for a new key.
type CreateInput struct {
	Name      string     `json:"name"      validate:"required,min=1,max=100"`
	ExpiresAt *time.Time `json:"expires_at"`
	Scopes    []string   `json:"scopes"`
}

// Service implements the business logic for API key management.
type Service struct {
	db *pgxpool.Pool
}

// NewService constructs a new Service backed by the given DB pool.
func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

// Create generates a new API key for the given user / org, persists the SHA-256
// hash, and returns the raw key exactly once.
func (s *Service) Create(ctx context.Context, orgID, userID string, input CreateInput) (*CreateResult, error) {
	// Generate 32 random bytes, encode as base64url to form the secret part.
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("apikeys: failed to generate random bytes: %w", err)
	}
	secret := base64.RawURLEncoding.EncodeToString(raw)
	rawKey := "vakt_" + secret

	// SHA-256 hash — stored in the DB.
	sum := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(sum[:])

	// First 12 chars as human-readable prefix (e.g. "vakt_abc123").
	keyPrefix := rawKey
	if len(rawKey) > 12 {
		keyPrefix = rawKey[:12]
	}

	scopes := input.Scopes
	if scopes == nil {
		scopes = []string{}
	}

	const q = `
		INSERT INTO api_keys (org_id, created_by, name, key_hash, key_prefix, scopes, expires_at)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7)
		RETURNING id, name, key_prefix, scopes, last_used_at, expires_at, created_at`

	var k APIKey
	err := s.db.QueryRow(ctx, q,
		orgID, userID, input.Name, keyHash, keyPrefix, scopes, input.ExpiresAt,
	).Scan(&k.ID, &k.Name, &k.KeyPrefix, &k.Scopes, &k.LastUsedAt, &k.ExpiresAt, &k.CreatedAt)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("apikeys: create failed")
		return nil, fmt.Errorf("apikeys: insert failed: %w", err)
	}

	return &CreateResult{APIKey: k, RawKey: rawKey}, nil
}

// List returns all non-revoked API keys belonging to the given user within the org.
func (s *Service) List(ctx context.Context, orgID, userID string) ([]APIKey, error) {
	const q = `
		SELECT id, name, key_prefix, scopes, last_used_at, last_used_ip, expires_at, created_at, rotated_at
		FROM api_keys
		WHERE org_id = $1::uuid
		  AND created_by = $2::uuid
		  AND revoked_at IS NULL
		ORDER BY created_at DESC`

	rows, err := s.db.Query(ctx, q, orgID, userID)
	if err != nil {
		return nil, fmt.Errorf("apikeys: list query failed: %w", err)
	}
	defer rows.Close()

	var keys []APIKey
	for rows.Next() {
		var k APIKey
		if err := rows.Scan(&k.ID, &k.Name, &k.KeyPrefix, &k.Scopes, &k.LastUsedAt, &k.LastUsedIP, &k.ExpiresAt, &k.CreatedAt, &k.RotatedAt); err != nil {
			return nil, fmt.Errorf("apikeys: scan failed: %w", err)
		}
		keys = append(keys, k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("apikeys: rows error: %w", err)
	}
	if keys == nil {
		keys = []APIKey{}
	}
	return keys, nil
}

// Revoke soft-deletes an API key by setting revoked_at. It verifies that the
// key belongs to the requesting user within the org before revoking.
func (s *Service) Revoke(ctx context.Context, orgID, userID, keyID string) error {
	const q = `
		UPDATE api_keys
		SET revoked_at = NOW()
		WHERE id = $1::uuid
		  AND org_id = $2::uuid
		  AND created_by = $3::uuid
		  AND revoked_at IS NULL`

	tag, err := s.db.Exec(ctx, q, keyID, orgID, userID)
	if err != nil {
		return fmt.Errorf("apikeys: revoke failed: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ErrNotFound is returned when the key does not exist or does not belong to the caller.
var ErrNotFound = fmt.Errorf("apikeys: key not found")

// APIKeyWithOwner is an API key plus the email of the user who created it, for
// the admin org-wide view (S131-D15-08).
type APIKeyWithOwner struct {
	APIKey
	CreatedByEmail string `json:"created_by_email"`
}

// ListOrg returns every non-revoked API key in the org together with its owner's
// email. S131-D15-08: the per-user List/Revoke are scoped to created_by, so an
// admin could neither see nor revoke another user's key — a hole in the offboarding
// story (a departed user's key stayed invisible and unrevocable outside the HR
// offboarding flow). This admin view is org-scoped, NOT created_by-scoped.
func (s *Service) ListOrg(ctx context.Context, orgID string) ([]APIKeyWithOwner, error) {
	const q = `
		SELECT ak.id, ak.name, ak.key_prefix, ak.scopes, ak.last_used_at, ak.last_used_ip,
		       ak.expires_at, ak.created_at, ak.rotated_at, u.email
		FROM api_keys ak
		JOIN users u ON u.id = ak.created_by
		WHERE ak.org_id = $1::uuid
		  AND ak.revoked_at IS NULL
		ORDER BY ak.created_at DESC`
	rows, err := s.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("apikeys: list org query failed: %w", err)
	}
	defer rows.Close()
	out := []APIKeyWithOwner{}
	for rows.Next() {
		var k APIKeyWithOwner
		if err := rows.Scan(&k.ID, &k.Name, &k.KeyPrefix, &k.Scopes, &k.LastUsedAt, &k.LastUsedIP,
			&k.ExpiresAt, &k.CreatedAt, &k.RotatedAt, &k.CreatedByEmail); err != nil {
			return nil, fmt.Errorf("apikeys: scan org key: %w", err)
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// RevokeOrg revokes any key in the org by id, regardless of who created it —
// admin-only (S131-D15-08). Org-scoped so it can never touch another org's key.
func (s *Service) RevokeOrg(ctx context.Context, orgID, keyID string) error {
	const q = `
		UPDATE api_keys
		SET revoked_at = NOW()
		WHERE id = $1::uuid
		  AND org_id = $2::uuid
		  AND revoked_at IS NULL`
	tag, err := s.db.Exec(ctx, q, keyID, orgID)
	if err != nil {
		return fmt.Errorf("apikeys: revoke org failed: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
