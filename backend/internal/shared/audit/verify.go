// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ChainStatus classifies the outcome of a per-org chain replay.
//
// The three states are deliberately distinct. A row without an entry_hash was
// never chained, so the verifier can make NO statement about it — neither
// "unchanged" nor "tampered". Folding those rows into "intact" would report a
// conclusion about evidence that was never examined, which for an audit trail
// is the worst kind of wrong: plausibly wrong.
type ChainStatus string

const (
	// ChainIntact means every row of the org carried an entry_hash and every
	// one of them replayed correctly. This is the ONLY status that may be
	// read as "the audit trail is verified".
	ChainIntact ChainStatus = "intact"

	// ChainUnverifiable means no break was found among the chained rows, but
	// at least one row carries no entry_hash and was therefore not checked.
	// The trail is neither proven intact nor proven broken.
	ChainUnverifiable ChainStatus = "unverifiable"

	// ChainBroken means a chained row failed to replay — a missing link, a
	// rewritten field, or a deleted predecessor. Forensic evidence.
	ChainBroken ChainStatus = "broken"
)

// ChainResult is the outcome of replaying one org's audit-log hash chain.
//
// Verified and Unverifiable are both reported on purpose: a verification
// result without its denominator cannot be audited. Verified+Unverifiable is
// the number of rows the replay actually looked at.
//
// Counting stops at the first break — every row after a broken link would
// fail trivially, so the counts describe the rows examined up to and
// including FirstBadRow, not the whole org.
type ChainResult struct {
	// Status is the single classification of this replay. Broken wins over
	// Unverifiable wins over Intact.
	Status ChainStatus
	// FirstBadRow is the UUID of the first row that failed to replay, or ""
	// when no break was found.
	FirstBadRow string
	// Verified counts rows that carried an entry_hash and replayed correctly.
	Verified int
	// Unverifiable counts rows with a NULL entry_hash — never chained, so
	// nothing about them was checked.
	Unverifiable int
	// Total counts every row the replay examined (Verified + Unverifiable,
	// plus the broken row itself when Status is ChainBroken).
	Total int
}

// FullyVerified reports whether every examined row was chained and replayed
// correctly. It is false for both ChainBroken and ChainUnverifiable — callers
// must not treat "no break found" as "verified".
func (r ChainResult) FullyVerified() bool { return r.Status == ChainIntact }

// VerifyOrgChain replays the per-org audit-log hash chain and reports whether
// it is intact, broken, or not fully verifiable.
//
// The walk is in created_at ASC + id ASC order (matching the writer-side
// chain extension). Rows whose entry_hash is NULL cannot be replayed — they
// predate migration 149, or were written by a path that bypassed the chained
// writer. They are counted in ChainResult.Unverifiable and force the status
// away from ChainIntact; they are never silently skipped.
//
// A non-empty FirstBadRow always reflects a real chain break — the caller
// should treat it as forensic evidence (preserve DB snapshot, rotate access
// keys, invoke the response under ADR-0040).
func VerifyOrgChain(ctx context.Context, pool *pgxpool.Pool, orgID string) (ChainResult, error) {
	res := ChainResult{Status: ChainIntact}
	rows, err := pool.Query(ctx, `
		SELECT
			id::text,
			COALESCE(user_id::text, ''),
			COALESCE(user_email, ''),
			action,
			resource_type,
			COALESCE(resource_id, ''),
			COALESCE(resource_name, ''),
			details,
			COALESCE(ip_address, ''),
			created_at,
			prev_hash,
			entry_hash
		FROM audit_log
		WHERE org_id = $1::uuid
		ORDER BY created_at ASC, id ASC`, orgID)
	if err != nil {
		return ChainResult{}, fmt.Errorf("query audit_log for org %s: %w", orgID, err)
	}
	defer rows.Close()

	var expectedPrev []byte
	chainStarted := false

	for rows.Next() {
		var (
			id, userID, userEmail, action, resourceType, resourceID, resourceName, ipAddress string
			detailsJSON                                                                      []byte
			createdAt                                                                        time.Time
			storedPrev, storedEntry                                                          []byte
		)
		if err := rows.Scan(&id, &userID, &userEmail, &action, &resourceType, &resourceID, &resourceName, &detailsJSON, &ipAddress, &createdAt, &storedPrev, &storedEntry); err != nil {
			return ChainResult{}, fmt.Errorf("scan audit row: %w", err)
		}
		res.Total++

		// Unchained row: written before migration 149, or by a writer that
		// bypassed audit.Write. Nothing about it can be checked — count it
		// and keep going, but the org can no longer be reported as intact.
		if storedEntry == nil {
			res.Unverifiable++
			continue
		}

		// First chained row of this org: storedPrev must be NULL.
		if !chainStarted {
			if storedPrev != nil {
				return broken(res, id), nil // chain starts mid-stream — link missing
			}
			chainStarted = true
		} else if !bytes.Equal(storedPrev, expectedPrev) {
			return broken(res, id), nil // link does not match the previous entry_hash
		}

		// Rebuild the input and recompute the hash.
		details := map[string]string{}
		if len(detailsJSON) > 0 {
			_ = json.Unmarshal(detailsJSON, &details)
		}

		in := ChainInput{
			ID:           id,
			OrgID:        orgID,
			UserID:       userID,
			UserEmail:    userEmail,
			Action:       action,
			ResourceType: resourceType,
			ResourceID:   resourceID,
			ResourceName: resourceName,
			Details:      details,
			IPAddress:    ipAddress,
			CreatedAt:    createdAt,
		}
		recomputed := EntryHash(storedPrev, in)
		if !bytes.Equal(recomputed, storedEntry) {
			return broken(res, id), nil
		}
		res.Verified++
		expectedPrev = storedEntry
	}
	if err := rows.Err(); err != nil {
		return ChainResult{}, err
	}
	if res.Unverifiable > 0 {
		res.Status = ChainUnverifiable
	}
	return res, nil
}

// broken stamps a replay result as a confirmed chain break. Broken outranks
// unverifiable: an org with both a rewritten row and unchained rows is broken,
// not merely unproven.
func broken(res ChainResult, badRow string) ChainResult {
	res.Status = ChainBroken
	res.FirstBadRow = badRow
	return res
}
