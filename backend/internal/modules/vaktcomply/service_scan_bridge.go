// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S88-8: Scan→Comply-Evidence-Brücke. Consumes vaktscan finding-created events
// (delivered via the shared cross-module event task — NO import of the vaktscan
// package) and attaches the finding as evidence to the vulnerability /
// configuration controls (ISO A.8.8 / A.8.9). Idempotent via ck_scan_evidence_map
// so a re-scan re-emitting the same finding never duplicates evidence.

package vaktcomply

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
)

// scanEvidenceKeywords selects the controls a scanner finding maps onto:
// A.8.8 (technical vulnerabilities) and A.8.9 (configuration management).
// Stems (not full words) so both singular/plural and DE/EN spellings match the
// control title/domain ILIKE (e.g. "vulnerabilit" matches "vulnerabilities").
var scanEvidenceKeywords = []string{"vulnerabilit", "schwachstell", "configuration", "konfiguration", "patch", "hardening"}

// RecordScanFindingEvidence attaches a scanner finding as evidence to the matching
// vulnerability/configuration controls. The (org, finding, control) tuple is
// recorded in ck_scan_evidence_map; evidence is only written for tuples that are
// newly inserted, making re-delivery of the same finding a no-op (idempotent).
// findingID is an opaque key — vaktcomply never reads vaktscan tables.
func (s *Service) RecordScanFindingEvidence(ctx context.Context, orgID, findingID, title string) (int, error) {
	if orgID == "" || findingID == "" {
		return 0, fmt.Errorf("scan bridge: org_id and finding_id required")
	}
	controls, err := s.repo.FindControlsByKeywords(ctx, orgID, scanEvidenceKeywords)
	if err != nil {
		return 0, fmt.Errorf("scan bridge: find controls: %w", err)
	}
	written := 0
	for _, ctrl := range controls {
		// Idempotency guard: only the first delivery for this (finding, control)
		// inserts a row; ON CONFLICT makes re-scans a no-op.
		tag, err := s.db.Exec(ctx, `
			INSERT INTO ck_scan_evidence_map (org_id, finding_id, control_id)
			VALUES ($1, $2, $3::uuid)
			ON CONFLICT (org_id, finding_id, control_id) DO NOTHING`,
			orgID, findingID, ctrl.ID)
		if err != nil {
			log.Warn().Err(err).Str("org_id", orgID).Str("control_id", ctrl.ID).Msg("scan bridge: map insert")
			continue
		}
		if tag.RowsAffected() == 0 {
			continue // already mapped — skip (idempotent)
		}
		payload := []byte(fmt.Sprintf(`{"source":"scan_bridge","finding_id":%q}`, findingID))
		if _, evErr := s.repo.AddCollectorEvidence(ctx, orgID, ctrl.ID, "", "automated",
			"Scan-Finding: "+title, payload); evErr != nil {
			log.Warn().Err(evErr).Str("control_id", ctrl.ID).Msg("scan bridge: add evidence")
			continue
		}
		written++
	}
	return written, nil
}
