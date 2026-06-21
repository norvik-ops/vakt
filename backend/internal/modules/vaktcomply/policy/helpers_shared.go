// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package policy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// ErrNotFound is returned when a requested resource does not exist.
// Duplicated from the parent vaktcomply package for policy-domain repositories.
var ErrNotFound = errors.New("not found")

// hashToken returns the hex-encoded SHA-256 of raw. Duplicated from the parent
// vaktcomply package (service_suppliers.go) for policy-acceptance token hashing.
func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// fetchOrgName reads the organisation name for reports/PDFs. Duplicated from the
// parent vaktcomply package (orgname.go); falls back to an empty string on error.
func fetchOrgName(ctx context.Context, db *pgxpool.Pool, orgID string) string {
	if db == nil || orgID == "" {
		return ""
	}
	var name string
	if err := db.QueryRow(ctx,
		`SELECT name FROM organizations WHERE id = $1::uuid`,
		orgID,
	).Scan(&name); err != nil {
		log.Warn().Err(err).
			Str("org_id", orgID).
			Str("module", "vaktcomply").
			Msg("fetchOrgName: SELECT failed — using empty org name for downstream report")
		return ""
	}
	return name
}
