// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S69-1: Cross-Framework Mapping Completeness
// New seeder functions and prerequisite chain logic.

package vaktcomply

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
)

// SeedDORAMappingsFixed idempotently seeds DORA ↔ ISO 27001:2022 mappings.
// Uses doraISO27001Mapping (service_helpers.go) as the single source of truth —
// that map was already correct 2022-style and is also used for UI display.
func (s *Service) SeedDORAMappingsFixed(ctx context.Context, orgID string) error {
	doraFW, err := s.repo.FindFrameworkByName(ctx, orgID, "DORA")
	if err != nil {
		return fmt.Errorf("find DORA framework: %w", err)
	}
	if doraFW == nil {
		return nil
	}
	isoFW, err := s.repo.FindFrameworkByName(ctx, orgID, "ISO27001")
	if err != nil {
		return fmt.Errorf("find ISO27001 framework: %w", err)
	}
	if isoFW == nil {
		return nil
	}

	seeded := 0
	for doraCode, isoCodes := range doraISO27001Mapping {
		for _, isoCode := range splitCodes(isoCodes) {
			if err := s.repo.SeedGlobalControlMapping(ctx, "DORA", doraCode, "ISO27001", isoCode, "equivalent"); err != nil {
				log.Warn().Err(err).Str("dora", doraCode).Str("iso", isoCode).Msg("seed DORA mapping failed")
			} else {
				seeded++
			}
		}
	}
	log.Info().Int("seeded", seeded).Msg("SeedDORAMappingsFixed: done")
	return nil
}

// splitCodes splits a ", "-separated list of control codes.
func splitCodes(raw string) []string {
	out := []string{}
	cur := ""
	for i := 0; i < len(raw); i++ {
		if raw[i] == ',' {
			if cur != "" {
				out = append(out, trimSpace(cur))
				cur = ""
			}
			// skip following space
			if i+1 < len(raw) && raw[i+1] == ' ' {
				i++
			}
		} else {
			cur += string(raw[i])
		}
	}
	if cur != "" {
		out = append(out, trimSpace(cur))
	}
	return out
}

func trimSpace(s string) string {
	start, end := 0, len(s)-1
	for start <= end && s[start] == ' ' {
		start++
	}
	for end >= start && s[end] == ' ' {
		end--
	}
	return s[start : end+1]
}

// ── CRA ↔ ISO 27001 + NIS2 ──────────────────────────────────────────────────

var craMappings = []frameworkPair{
	{src: "CRA", srcCode: "CRA-1.1", tgt: "ISO27001", tgtCode: "A.8.26", mtype: "equivalent"},  // Application security requirements
	{src: "CRA", srcCode: "CRA-1.2", tgt: "ISO27001", tgtCode: "A.5.1", mtype: "equivalent"},   // Policies for IS
	{src: "CRA", srcCode: "CRA-1.3", tgt: "ISO27001", tgtCode: "A.8.8", mtype: "equivalent"},   // Technical vulnerability management
	{src: "CRA", srcCode: "CRA-1.4", tgt: "ISO27001", tgtCode: "A.8.8", mtype: "partial"},      // Vuln mgmt (partial)
	{src: "CRA", srcCode: "CRA-1.5", tgt: "ISO27001", tgtCode: "A.8.25", mtype: "equivalent"},  // Secure development life cycle
	{src: "CRA", srcCode: "CRA-1.6", tgt: "ISO27001", tgtCode: "A.8.8", mtype: "equivalent"},   // Patch management
	{src: "CRA", srcCode: "CRA-1.7", tgt: "ISO27001", tgtCode: "A.8.8", mtype: "equivalent"},   // Vuln disclosure
	{src: "CRA", srcCode: "CRA-1.8", tgt: "ISO27001", tgtCode: "A.8.3", mtype: "equivalent"},   // Information access restriction
	{src: "CRA", srcCode: "CRA-1.9", tgt: "ISO27001", tgtCode: "A.8.24", mtype: "equivalent"},  // Use of cryptography
	{src: "CRA", srcCode: "CRA-1.10", tgt: "ISO27001", tgtCode: "A.8.15", mtype: "equivalent"}, // Logging
	{src: "CRA", srcCode: "CRA-2.1", tgt: "ISO27001", tgtCode: "A.5.24", mtype: "equivalent"},  // Incident management planning
	{src: "CRA", srcCode: "CRA-2.2", tgt: "ISO27001", tgtCode: "A.8.8", mtype: "partial"},      // Vuln handling after incident
	{src: "CRA", srcCode: "CRA-2.3", tgt: "ISO27001", tgtCode: "A.5.24", mtype: "partial"},     // Incident response
	{src: "CRA", srcCode: "CRA-3.1", tgt: "ISO27001", tgtCode: "A.8.26", mtype: "equivalent"},  // Security requirements
	{src: "CRA", srcCode: "CRA-3.2", tgt: "ISO27001", tgtCode: "A.8.8", mtype: "partial"},      // Vulnerability testing
	{src: "CRA", srcCode: "CRA-3.3", tgt: "ISO27001", tgtCode: "A.8.25", mtype: "partial"},     // Secure dev process
}

var craNIS2Mappings = []frameworkPair{
	{src: "CRA", srcCode: "CRA-1.1", tgt: "NIS2", tgtCode: "NIS2-E.1", mtype: "equivalent"},
	{src: "CRA", srcCode: "CRA-1.2", tgt: "NIS2", tgtCode: "NIS2-A.3", mtype: "equivalent"},
	{src: "CRA", srcCode: "CRA-1.3", tgt: "NIS2", tgtCode: "NIS2-E.3", mtype: "equivalent"},
	{src: "CRA", srcCode: "CRA-1.3", tgt: "NIS2", tgtCode: "NIS2-E.6", mtype: "partial"},
	{src: "CRA", srcCode: "CRA-1.4", tgt: "NIS2", tgtCode: "NIS2-D.5", mtype: "partial"},
	{src: "CRA", srcCode: "CRA-1.6", tgt: "NIS2", tgtCode: "NIS2-E.4", mtype: "equivalent"},
	{src: "CRA", srcCode: "CRA-1.7", tgt: "NIS2", tgtCode: "NIS2-E.3", mtype: "equivalent"},
	{src: "CRA", srcCode: "CRA-1.8", tgt: "NIS2", tgtCode: "NIS2-F.1", mtype: "equivalent"},
	{src: "CRA", srcCode: "CRA-2.1", tgt: "NIS2", tgtCode: "NIS2-B.5", mtype: "equivalent"},
	{src: "CRA", srcCode: "CRA-2.1", tgt: "NIS2", tgtCode: "NIS2-B.6", mtype: "partial"},
	{src: "CRA", srcCode: "CRA-2.2", tgt: "NIS2", tgtCode: "NIS2-E.6", mtype: "equivalent"},
	{src: "CRA", srcCode: "CRA-2.3", tgt: "NIS2", tgtCode: "NIS2-E.6", mtype: "equivalent"},
	{src: "CRA", srcCode: "CRA-3.1", tgt: "NIS2", tgtCode: "NIS2-E.1", mtype: "partial"},
	{src: "CRA", srcCode: "CRA-3.2", tgt: "NIS2", tgtCode: "NIS2-E.5", mtype: "equivalent"},
}

// SeedCRAMappings idempotently seeds CRA ↔ ISO27001 and CRA ↔ NIS2 mappings.
func (s *Service) SeedCRAMappings(ctx context.Context, orgID string) error {
	craFW, err := s.repo.FindFrameworkByName(ctx, orgID, "CRA")
	if err != nil {
		return fmt.Errorf("find CRA framework: %w", err)
	}
	if craFW == nil {
		return nil
	}
	for _, pairs := range [][]frameworkPair{craMappings, craNIS2Mappings} {
		if err := s.seedPairs(ctx, orgID, pairs); err != nil {
			log.Warn().Err(err).Msg("CRA seed partial")
		}
	}
	return nil
}

// ── NIS2 ↔ DORA ─────────────────────────────────────────────────────────────

var nis2DORAMappings = []frameworkPair{
	{src: "NIS2", srcCode: "NIS2-A.1", tgt: "DORA", tgtCode: "DORA-1.2", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-A.2", tgt: "DORA", tgtCode: "DORA-1.1", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-A.3", tgt: "DORA", tgtCode: "DORA-1.2", mtype: "partial"},
	{src: "NIS2", srcCode: "NIS2-B.1", tgt: "DORA", tgtCode: "DORA-2.3", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-B.5", tgt: "DORA", tgtCode: "DORA-2.2", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-B.6", tgt: "DORA", tgtCode: "DORA-2.2", mtype: "partial"},
	{src: "NIS2", srcCode: "NIS2-C.1", tgt: "DORA", tgtCode: "DORA-1.6", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-C.4", tgt: "DORA", tgtCode: "DORA-1.7", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-D.1", tgt: "DORA", tgtCode: "DORA-4.1", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-D.3", tgt: "DORA", tgtCode: "DORA-4.2", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-E.3", tgt: "DORA", tgtCode: "DORA-3.1", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-E.4", tgt: "DORA", tgtCode: "DORA-1.8", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-H.1", tgt: "DORA", tgtCode: "DORA-1.4", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-A.8", tgt: "DORA", tgtCode: "DORA-1.3", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-B.7", tgt: "DORA", tgtCode: "DORA-2.4", mtype: "equivalent"},
}

// SeedNIS2DORAMappings seeds NIS2 ↔ DORA (full, Art. 5–15) bidirectional mappings.
func (s *Service) SeedNIS2DORAMappings(ctx context.Context, orgID string) error {
	return s.seedBidirectionalMappings(ctx, orgID, "NIS2", "DORA", nis2DORAMappings)
}

// ── NIS2 ↔ DORA Simplified (Art. 16, RTS EU 2024/1774 Chapter II) ────────────
// Maps NIS2 controls to the 15 DORA-S.x controls for small/non-interconnected entities.

var nis2DORASimplifiedMappings = []frameworkPair{
	{src: "NIS2", srcCode: "NIS2-A.1", tgt: "DORA", tgtCode: "DORA-S.1", mtype: "equivalent"},  // Governance → Org. Rahmen
	{src: "NIS2", srcCode: "NIS2-A.1", tgt: "DORA", tgtCode: "DORA-S.2", mtype: "equivalent"},  // Risikoanalyse → IKT-Risikobewertung
	{src: "NIS2", srcCode: "NIS2-A.2", tgt: "DORA", tgtCode: "DORA-S.1", mtype: "partial"},     // Policy → Org. Rahmen
	{src: "NIS2", srcCode: "NIS2-A.1", tgt: "DORA", tgtCode: "DORA-S.4", mtype: "equivalent"},  // Risikoanalyse → Policy (vereinfacht)
	{src: "NIS2", srcCode: "NIS2-A.8", tgt: "DORA", tgtCode: "DORA-S.3", mtype: "equivalent"},  // Asset-Management → IKT-Asset-Inventar
	{src: "NIS2", srcCode: "NIS2-E.1", tgt: "DORA", tgtCode: "DORA-S.5", mtype: "partial"},     // Netz-/IS-Sicherheit → Sicherheitsleitlinie
	{src: "NIS2", srcCode: "NIS2-H.1", tgt: "DORA", tgtCode: "DORA-S.5", mtype: "partial"},     // Kryptografie → Sicherheitsleitlinie
	{src: "NIS2", srcCode: "NIS2-F.1", tgt: "DORA", tgtCode: "DORA-S.5", mtype: "partial"},     // Zugriffssteuerung → Sicherheitsleitlinie
	{src: "NIS2", srcCode: "NIS2-E.3", tgt: "DORA", tgtCode: "DORA-S.6", mtype: "equivalent"},  // Schwachstellenmanagement → Schutzmaßnahmen
	{src: "NIS2", srcCode: "NIS2-E.4", tgt: "DORA", tgtCode: "DORA-S.6", mtype: "partial"},     // Patch-Management → Schutzmaßnahmen
	{src: "NIS2", srcCode: "NIS2-B.1", tgt: "DORA", tgtCode: "DORA-S.7", mtype: "partial"},     // Incident-Handling → Anomalie-Erkennung
	{src: "NIS2", srcCode: "NIS2-B.1", tgt: "DORA", tgtCode: "DORA-S.8", mtype: "equivalent"},  // Incident-Handling → Reaktionsplan
	{src: "NIS2", srcCode: "NIS2-B.3", tgt: "DORA", tgtCode: "DORA-S.8", mtype: "partial"},     // Incident-Klassifizierung → Reaktionsplan
	{src: "NIS2", srcCode: "NIS2-C.4", tgt: "DORA", tgtCode: "DORA-S.9", mtype: "equivalent"},  // BCM Backup → Backup & Wiederherstellung
	{src: "NIS2", srcCode: "NIS2-C.3", tgt: "DORA", tgtCode: "DORA-S.9", mtype: "partial"},     // BCM Tests → Backup-Tests
	{src: "NIS2", srcCode: "NIS2-C.1", tgt: "DORA", tgtCode: "DORA-S.10", mtype: "equivalent"}, // BCM-Richtlinie → BCP (vereinfacht)
	{src: "NIS2", srcCode: "NIS2-C.2", tgt: "DORA", tgtCode: "DORA-S.10", mtype: "partial"},    // BCM-Planung → BCP
	{src: "NIS2", srcCode: "NIS2-A.1", tgt: "DORA", tgtCode: "DORA-S.11", mtype: "partial"},    // Risikoanalyse → Resilienztests
	{src: "NIS2", srcCode: "NIS2-F.2", tgt: "DORA", tgtCode: "DORA-S.11", mtype: "partial"},    // Wirksamkeitsbewertung → Tests
	{src: "NIS2", srcCode: "NIS2-D.1", tgt: "DORA", tgtCode: "DORA-S.12", mtype: "equivalent"}, // Lieferkette → Drittparteien-IKT
	{src: "NIS2", srcCode: "NIS2-D.2", tgt: "DORA", tgtCode: "DORA-S.12", mtype: "partial"},    // Lieferantenverträge → Drittparteien
	{src: "NIS2", srcCode: "NIS2-D.3", tgt: "DORA", tgtCode: "DORA-S.13", mtype: "equivalent"}, // Lieferanten-Monitoring → Ausstiegsstrategie
	{src: "NIS2", srcCode: "NIS2-D.6", tgt: "DORA", tgtCode: "DORA-S.13", mtype: "partial"},    // Konzentrations-Risiken → Ausstieg
	{src: "NIS2", srcCode: "NIS2-B.5", tgt: "DORA", tgtCode: "DORA-S.14", mtype: "equivalent"}, // 24h-Meldung → Meldeverfahren
	{src: "NIS2", srcCode: "NIS2-B.7", tgt: "DORA", tgtCode: "DORA-S.14", mtype: "partial"},    // Incident-Reporting → Meldeverfahren
	{src: "NIS2", srcCode: "NIS2-A.2", tgt: "DORA", tgtCode: "DORA-S.15", mtype: "equivalent"}, // Governance → Berichterstattung Leitungsorgan
	{src: "NIS2", srcCode: "NIS2-A.7", tgt: "DORA", tgtCode: "DORA-S.15", mtype: "partial"},    // Compliance-Reporting → Leitungsorgan
}

// SeedNIS2DORASimplifiedMappings seeds NIS2 ↔ DORA Simplified (Art. 16) bidirectional mappings.
func (s *Service) SeedNIS2DORASimplifiedMappings(ctx context.Context, orgID string) error {
	return s.seedBidirectionalMappings(ctx, orgID, "NIS2", "DORA", nis2DORASimplifiedMappings)
}

// ── NIS2 ↔ BSI ──────────────────────────────────────────────────────────────

var nis2BSIMappings = []frameworkPair{
	{src: "NIS2", srcCode: "NIS2-A.1", tgt: "BSI", tgtCode: "BSI-ORP.1", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-A.6", tgt: "BSI", tgtCode: "BSI-ORP.2", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-B.1", tgt: "BSI", tgtCode: "BSI-DER.2.1", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-C.4", tgt: "BSI", tgtCode: "BSI-CON.3", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-E.3", tgt: "BSI", tgtCode: "BSI-OPS.1.1.2", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-E.4", tgt: "BSI", tgtCode: "BSI-OPS.1.1.2", mtype: "partial"},
	{src: "NIS2", srcCode: "NIS2-E.8", tgt: "BSI", tgtCode: "BSI-NET.1.1", mtype: "equivalent"},
}

// SeedNIS2BSIMappings seeds NIS2 ↔ BSI Grundschutz bidirectional mappings.
// Includes both the original 7 base pairs and the S75-3 enrichment (all A–J thematic areas).
func (s *Service) SeedNIS2BSIMappings(ctx context.Context, orgID string) error {
	all := append(nis2BSIMappings, nis2BSIExtendedMappings...)
	return s.seedBidirectionalMappings(ctx, orgID, "NIS2", "BSI", all)
}

// ── NIS2 ↔ CIS ──────────────────────────────────────────────────────────────

var nis2CISMappings = []frameworkPair{
	{src: "NIS2", srcCode: "NIS2-A.8", tgt: "CIS", tgtCode: "CIS-1.1", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-A.8", tgt: "CIS", tgtCode: "CIS-2.1", mtype: "partial"},
	{src: "NIS2", srcCode: "NIS2-E.3", tgt: "CIS", tgtCode: "CIS-7.1", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-E.4", tgt: "CIS", tgtCode: "CIS-7.2", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-F.1", tgt: "CIS", tgtCode: "CIS-5.1", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-F.1", tgt: "CIS", tgtCode: "CIS-6.1", mtype: "partial"},
	{src: "NIS2", srcCode: "NIS2-G.2", tgt: "CIS", tgtCode: "CIS-14.1", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-H.1", tgt: "CIS", tgtCode: "CIS-3.3", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-B.1", tgt: "CIS", tgtCode: "CIS-17.1", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-C.4", tgt: "CIS", tgtCode: "CIS-11.1", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-E.8", tgt: "CIS", tgtCode: "CIS-12.1", mtype: "equivalent"},
}

// SeedNIS2CISMappings seeds NIS2 ↔ CIS Controls bidirectional mappings.
func (s *Service) SeedNIS2CISMappings(ctx context.Context, orgID string) error {
	return s.seedBidirectionalMappings(ctx, orgID, "NIS2", "CIS", nis2CISMappings)
}

// ── EU AI Act ↔ ISO 42001 ────────────────────────────────────────────────────

var euAIActISO42001Mappings = []frameworkPair{
	{src: "EUAIACT", srcCode: "AIACT-1.1", tgt: "ISO42001", tgtCode: "42001-6.1", mtype: "equivalent"},
	{src: "EUAIACT", srcCode: "AIACT-1.2", tgt: "ISO42001", tgtCode: "42001-6.1", mtype: "partial"},
	{src: "EUAIACT", srcCode: "AIACT-2.1", tgt: "ISO42001", tgtCode: "42001-A4.2", mtype: "equivalent"},
	{src: "EUAIACT", srcCode: "AIACT-3.1", tgt: "ISO42001", tgtCode: "42001-7.3", mtype: "equivalent"},
	{src: "EUAIACT", srcCode: "AIACT-6.1", tgt: "ISO42001", tgtCode: "42001-A4.3", mtype: "equivalent"},
	{src: "EUAIACT", srcCode: "AIACT-7.1", tgt: "ISO42001", tgtCode: "42001-A4.3", mtype: "partial"},
	{src: "EUAIACT", srcCode: "AIACT-9.1", tgt: "ISO42001", tgtCode: "42001-9.2", mtype: "equivalent"},
	{src: "EUAIACT", srcCode: "AIACT-10.1", tgt: "ISO42001", tgtCode: "42001-7.3", mtype: "partial"},
	{src: "EUAIACT", srcCode: "AIACT-11.1", tgt: "ISO42001", tgtCode: "42001-A3.2", mtype: "partial"},
}

// SeedEUAIActISO42001Mappings seeds EU AI Act ↔ ISO 42001 bidirectional mappings.
func (s *Service) SeedEUAIActISO42001Mappings(ctx context.Context, orgID string) error {
	return s.seedBidirectionalMappings(ctx, orgID, "EUAIACT", "ISO42001", euAIActISO42001Mappings)
}

// ── EU AI Act ↔ ISO 27001 ────────────────────────────────────────────────────

var euAIActISO27001Mappings = []frameworkPair{
	{src: "EUAIACT", srcCode: "AIACT-7.3", tgt: "ISO27001", tgtCode: "A.8.25", mtype: "equivalent"}, // Secure development life cycle
	{src: "EUAIACT", srcCode: "AIACT-7.3", tgt: "ISO27001", tgtCode: "A.8.8", mtype: "partial"},     // Vulnerability management
	{src: "EUAIACT", srcCode: "AIACT-4.1", tgt: "ISO27001", tgtCode: "A.8.15", mtype: "equivalent"}, // Logging
	{src: "EUAIACT", srcCode: "AIACT-7.2", tgt: "ISO27001", tgtCode: "A.8.8", mtype: "partial"},     // Vulnerability handling
	{src: "EUAIACT", srcCode: "AIACT-8.1", tgt: "ISO27001", tgtCode: "A.5.31", mtype: "equivalent"}, // Legal/regulatory requirements
}

// SeedEUAIActISO27001Mappings seeds EU AI Act ↔ ISO 27001 bidirectional mappings.
func (s *Service) SeedEUAIActISO27001Mappings(ctx context.Context, orgID string) error {
	return s.seedBidirectionalMappings(ctx, orgID, "EUAIACT", "ISO27001", euAIActISO27001Mappings)
}

// ── ISO 42001 ↔ ISO 27001 ────────────────────────────────────────────────────

var iso42001ISO27001Mappings = []frameworkPair{
	{src: "ISO42001", srcCode: "42001-5.1", tgt: "ISO27001", tgtCode: "A.5.1", mtype: "equivalent"},  // Policies for IS
	{src: "ISO42001", srcCode: "42001-6.1", tgt: "ISO27001", tgtCode: "A.5.1", mtype: "partial"},     // IS policies (partial)
	{src: "ISO42001", srcCode: "42001-7.1", tgt: "ISO27001", tgtCode: "A.6.3", mtype: "partial"},     // IS awareness/education/training
	{src: "ISO42001", srcCode: "42001-7.3", tgt: "ISO27001", tgtCode: "A.5.37", mtype: "equivalent"}, // Documented operating procedures
	{src: "ISO42001", srcCode: "42001-9.1", tgt: "ISO27001", tgtCode: "A.5.31", mtype: "equivalent"}, // Legal/regulatory requirements
}

// SeedISO42001ISO27001Mappings seeds ISO 42001 ↔ ISO 27001 bidirectional mappings.
func (s *Service) SeedISO42001ISO27001Mappings(ctx context.Context, orgID string) error {
	return s.seedBidirectionalMappings(ctx, orgID, "ISO42001", "ISO27001", iso42001ISO27001Mappings)
}

// ── prEN 18286 ↔ ISO 42001 (STUB — Draft, enquiry closed Jan 2026, pub. end 2026) ──
// prEN 18286 is the CEN/CENELEC harmonized standard for EU AI Act Art. 17 QMS.
// Annex D of prEN 18286 contains the correspondence with ISO/IEC 42001:2023.
// Codes follow the ISO HLS (High-Level Structure) clauses 4–10 + Annex A.

var prEN18286ISO42001Mappings = []frameworkPair{
	{src: "PREN18286", srcCode: "18286-4.1", tgt: "ISO42001", tgtCode: "42001-4.1", mtype: "equivalent"},   // Kontext / Context
	{src: "PREN18286", srcCode: "18286-4.2", tgt: "ISO42001", tgtCode: "42001-4.2", mtype: "equivalent"},   // Interessierte Parteien
	{src: "PREN18286", srcCode: "18286-5.1", tgt: "ISO42001", tgtCode: "42001-5.1", mtype: "equivalent"},   // Leadership
	{src: "PREN18286", srcCode: "18286-6.1", tgt: "ISO42001", tgtCode: "42001-6.1", mtype: "equivalent"},   // Risikomanagement
	{src: "PREN18286", srcCode: "18286-6.2", tgt: "ISO42001", tgtCode: "42001-6.2", mtype: "partial"},      // KI-spezifische Ziele
	{src: "PREN18286", srcCode: "18286-7.1", tgt: "ISO42001", tgtCode: "42001-7.1", mtype: "equivalent"},   // Ressourcen
	{src: "PREN18286", srcCode: "18286-7.3", tgt: "ISO42001", tgtCode: "42001-7.3", mtype: "equivalent"},   // Bewusstsein
	{src: "PREN18286", srcCode: "18286-8.1", tgt: "ISO42001", tgtCode: "42001-8.1", mtype: "equivalent"},   // Operational planning
	{src: "PREN18286", srcCode: "18286-8.2", tgt: "ISO42001", tgtCode: "42001-8.3", mtype: "partial"},      // KI-System-Entwicklungsprozess
	{src: "PREN18286", srcCode: "18286-9.1", tgt: "ISO42001", tgtCode: "42001-9.1", mtype: "equivalent"},   // Monitoring, Messung
	{src: "PREN18286", srcCode: "18286-9.2", tgt: "ISO42001", tgtCode: "42001-9.2", mtype: "equivalent"},   // Internes Audit
	{src: "PREN18286", srcCode: "18286-9.3", tgt: "ISO42001", tgtCode: "42001-9.3", mtype: "equivalent"},   // Management Review
	{src: "PREN18286", srcCode: "18286-10.1", tgt: "ISO42001", tgtCode: "42001-10.1", mtype: "equivalent"}, // Verbesserung
	{src: "PREN18286", srcCode: "18286-A.1", tgt: "ISO42001", tgtCode: "42001-A4.2", mtype: "partial"},     // KI-Impact-Assessment
	{src: "PREN18286", srcCode: "18286-A.2", tgt: "ISO42001", tgtCode: "42001-A4.3", mtype: "partial"},     // Transparenz & Dokumentation
}

// SeedPREN18286ISO42001Mappings seeds prEN 18286 ↔ ISO 42001 draft stub mappings.
// These are based on Annex D of prEN 18286 (enquiry draft, Jan 2026).
// Update when the standard is finalized (expected end 2026).
func (s *Service) SeedPREN18286ISO42001Mappings(ctx context.Context, orgID string) error {
	return s.seedBidirectionalMappings(ctx, orgID, "PREN18286", "ISO42001", prEN18286ISO42001Mappings)
}

// ── BSI C5:2026 ↔ ISO 27001:2022 mappings ────────────────────────────────────

// c5ISO27001Mappings maps BSI C5:2026 topic areas to ISO 27001:2022 Annex A controls.
// Source: BSI C5:2026 Annex B cross-reference table + ENISA CSP mapping guidance.
var c5ISO27001Mappings = []frameworkPair{
	// OIS — Organisation der Informationssicherheit
	{src: "C5", srcCode: "C5-OIS-01", tgt: "ISO27001", tgtCode: "A.5.1", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-OIS-02", tgt: "ISO27001", tgtCode: "A.5.2", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-OIS-03", tgt: "ISO27001", tgtCode: "A.5.4", mtype: "partial"},
	{src: "C5", srcCode: "C5-OIS-04", tgt: "ISO27001", tgtCode: "A.5.3", mtype: "partial"},
	{src: "C5", srcCode: "C5-OIS-05", tgt: "ISO27001", tgtCode: "A.5.35", mtype: "partial"},
	{src: "C5", srcCode: "C5-OIS-06", tgt: "ISO27001", tgtCode: "A.5.36", mtype: "partial"},
	{src: "C5", srcCode: "C5-OIS-07", tgt: "ISO27001", tgtCode: "A.5.7", mtype: "partial"},
	{src: "C5", srcCode: "C5-OIS-08", tgt: "ISO27001", tgtCode: "A.5.31", mtype: "partial"},
	{src: "C5", srcCode: "C5-OIS-09", tgt: "ISO27001", tgtCode: "A.6.7", mtype: "partial"},
	{src: "C5", srcCode: "C5-OIS-10", tgt: "ISO27001", tgtCode: "A.5.5", mtype: "partial"},
	// SP — Sicherheitsrichtlinien
	{src: "C5", srcCode: "C5-SP-01", tgt: "ISO27001", tgtCode: "A.5.1", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-SP-02", tgt: "ISO27001", tgtCode: "A.5.36", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-SP-03", tgt: "ISO27001", tgtCode: "A.5.35", mtype: "partial"},
	// HR — Personal
	{src: "C5", srcCode: "C5-HR-01", tgt: "ISO27001", tgtCode: "A.6.1", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-HR-02", tgt: "ISO27001", tgtCode: "A.6.2", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-HR-03", tgt: "ISO27001", tgtCode: "A.6.3", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-HR-04", tgt: "ISO27001", tgtCode: "A.6.4", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-HR-05", tgt: "ISO27001", tgtCode: "A.6.5", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-HR-06", tgt: "ISO27001", tgtCode: "A.6.6", mtype: "partial"},
	{src: "C5", srcCode: "C5-HR-07", tgt: "ISO27001", tgtCode: "A.7.1", mtype: "partial"},
	{src: "C5", srcCode: "C5-HR-08", tgt: "ISO27001", tgtCode: "A.7.3", mtype: "partial"},
	// AM — Asset Management
	{src: "C5", srcCode: "C5-AM-01", tgt: "ISO27001", tgtCode: "A.5.9", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-AM-02", tgt: "ISO27001", tgtCode: "A.5.10", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-AM-03", tgt: "ISO27001", tgtCode: "A.5.11", mtype: "partial"},
	{src: "C5", srcCode: "C5-AM-04", tgt: "ISO27001", tgtCode: "A.5.12", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-AM-05", tgt: "ISO27001", tgtCode: "A.5.13", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-AM-06", tgt: "ISO27001", tgtCode: "A.5.14", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-AM-07", tgt: "ISO27001", tgtCode: "A.8.1", mtype: "partial"},
	{src: "C5", srcCode: "C5-AM-08", tgt: "ISO27001", tgtCode: "A.7.10", mtype: "partial"},
	{src: "C5", srcCode: "C5-AM-09", tgt: "ISO27001", tgtCode: "A.8.10", mtype: "partial"},
	{src: "C5", srcCode: "C5-AM-10", tgt: "ISO27001", tgtCode: "A.8.11", mtype: "partial"},
	{src: "C5", srcCode: "C5-AM-11", tgt: "ISO27001", tgtCode: "A.8.12", mtype: "partial"},
	{src: "C5", srcCode: "C5-AM-12", tgt: "ISO27001", tgtCode: "A.5.34", mtype: "partial"},
	// PS — Physische Sicherheit
	{src: "C5", srcCode: "C5-PS-01", tgt: "ISO27001", tgtCode: "A.7.1", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-PS-02", tgt: "ISO27001", tgtCode: "A.7.2", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-PS-03", tgt: "ISO27001", tgtCode: "A.7.3", mtype: "partial"},
	{src: "C5", srcCode: "C5-PS-04", tgt: "ISO27001", tgtCode: "A.7.4", mtype: "partial"},
	{src: "C5", srcCode: "C5-PS-05", tgt: "ISO27001", tgtCode: "A.7.5", mtype: "partial"},
	{src: "C5", srcCode: "C5-PS-06", tgt: "ISO27001", tgtCode: "A.7.8", mtype: "partial"},
	{src: "C5", srcCode: "C5-PS-07", tgt: "ISO27001", tgtCode: "A.7.11", mtype: "partial"},
	{src: "C5", srcCode: "C5-PS-08", tgt: "ISO27001", tgtCode: "A.7.12", mtype: "partial"},
	// OPS — Betrieb
	{src: "C5", srcCode: "C5-OPS-01", tgt: "ISO27001", tgtCode: "A.8.8", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-OPS-02", tgt: "ISO27001", tgtCode: "A.8.32", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-OPS-03", tgt: "ISO27001", tgtCode: "A.8.9", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-OPS-04", tgt: "ISO27001", tgtCode: "A.8.20", mtype: "partial"},
	{src: "C5", srcCode: "C5-OPS-05", tgt: "ISO27001", tgtCode: "A.8.22", mtype: "partial"},
	{src: "C5", srcCode: "C5-OPS-06", tgt: "ISO27001", tgtCode: "A.8.21", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-OPS-07", tgt: "ISO27001", tgtCode: "A.8.16", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-OPS-08", tgt: "ISO27001", tgtCode: "A.8.15", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-OPS-09", tgt: "ISO27001", tgtCode: "A.8.17", mtype: "partial"},
	{src: "C5", srcCode: "C5-OPS-10", tgt: "ISO27001", tgtCode: "A.5.33", mtype: "partial"},
	{src: "C5", srcCode: "C5-OPS-11", tgt: "ISO27001", tgtCode: "A.8.13", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-OPS-12", tgt: "ISO27001", tgtCode: "A.8.14", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-OPS-13", tgt: "ISO27001", tgtCode: "A.8.6", mtype: "partial"},
	{src: "C5", srcCode: "C5-OPS-14", tgt: "ISO27001", tgtCode: "A.5.29", mtype: "partial"},
	{src: "C5", srcCode: "C5-OPS-15", tgt: "ISO27001", tgtCode: "A.5.30", mtype: "partial"},
	{src: "C5", srcCode: "C5-OPS-16", tgt: "ISO27001", tgtCode: "A.8.33", mtype: "partial"},
	{src: "C5", srcCode: "C5-OPS-17", tgt: "ISO27001", tgtCode: "A.8.34", mtype: "partial"},
	{src: "C5", srcCode: "C5-OPS-18", tgt: "ISO27001", tgtCode: "A.8.7", mtype: "partial"},
	{src: "C5", srcCode: "C5-OPS-19", tgt: "ISO27001", tgtCode: "A.8.19", mtype: "partial"},
	{src: "C5", srcCode: "C5-OPS-20", tgt: "ISO27001", tgtCode: "A.8.31", mtype: "partial"},
	// IAM — Identitäts- und Zugriffsverwaltung
	{src: "C5", srcCode: "C5-IAM-01", tgt: "ISO27001", tgtCode: "A.5.15", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-IAM-02", tgt: "ISO27001", tgtCode: "A.5.16", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-IAM-03", tgt: "ISO27001", tgtCode: "A.5.17", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-IAM-04", tgt: "ISO27001", tgtCode: "A.5.18", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-IAM-05", tgt: "ISO27001", tgtCode: "A.8.2", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-IAM-06", tgt: "ISO27001", tgtCode: "A.8.3", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-IAM-07", tgt: "ISO27001", tgtCode: "A.8.5", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-IAM-08", tgt: "ISO27001", tgtCode: "A.8.18", mtype: "partial"},
	{src: "C5", srcCode: "C5-IAM-09", tgt: "ISO27001", tgtCode: "A.8.4", mtype: "partial"},
	// CRY — Kryptographie
	{src: "C5", srcCode: "C5-CRY-01", tgt: "ISO27001", tgtCode: "A.8.24", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-CRY-02", tgt: "ISO27001", tgtCode: "A.8.25", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-CRY-03", tgt: "ISO27001", tgtCode: "A.5.31", mtype: "partial"},
	{src: "C5", srcCode: "C5-CRY-04", tgt: "ISO27001", tgtCode: "A.5.32", mtype: "partial"},
	// COS — Kommunikationssicherheit
	{src: "C5", srcCode: "C5-COS-01", tgt: "ISO27001", tgtCode: "A.8.20", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-COS-02", tgt: "ISO27001", tgtCode: "A.8.21", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-COS-03", tgt: "ISO27001", tgtCode: "A.8.22", mtype: "partial"},
	{src: "C5", srcCode: "C5-COS-04", tgt: "ISO27001", tgtCode: "A.5.14", mtype: "partial"},
	{src: "C5", srcCode: "C5-COS-05", tgt: "ISO27001", tgtCode: "A.8.26", mtype: "partial"},
	// DEV — Entwicklung & Änderungsmanagement
	{src: "C5", srcCode: "C5-DEV-01", tgt: "ISO27001", tgtCode: "A.5.8", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-DEV-02", tgt: "ISO27001", tgtCode: "A.8.25", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-DEV-03", tgt: "ISO27001", tgtCode: "A.8.26", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-DEV-04", tgt: "ISO27001", tgtCode: "A.8.27", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-DEV-05", tgt: "ISO27001", tgtCode: "A.8.28", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-DEV-06", tgt: "ISO27001", tgtCode: "A.8.29", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-DEV-07", tgt: "ISO27001", tgtCode: "A.8.30", mtype: "partial"},
	{src: "C5", srcCode: "C5-DEV-08", tgt: "ISO27001", tgtCode: "A.8.31", mtype: "partial"},
	{src: "C5", srcCode: "C5-DEV-09", tgt: "ISO27001", tgtCode: "A.8.32", mtype: "partial"},
	{src: "C5", srcCode: "C5-DEV-10", tgt: "ISO27001", tgtCode: "A.8.33", mtype: "partial"},
	{src: "C5", srcCode: "C5-DEV-11", tgt: "ISO27001", tgtCode: "A.8.34", mtype: "partial"},
	// SSO — Dienstleister-Steuerung (Supply Chain)
	{src: "C5", srcCode: "C5-SSO-01", tgt: "ISO27001", tgtCode: "A.5.19", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-SSO-02", tgt: "ISO27001", tgtCode: "A.5.20", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-SSO-03", tgt: "ISO27001", tgtCode: "A.5.21", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-SSO-04", tgt: "ISO27001", tgtCode: "A.5.22", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-SSO-05", tgt: "ISO27001", tgtCode: "A.5.23", mtype: "partial"},
	{src: "C5", srcCode: "C5-SSO-06", tgt: "ISO27001", tgtCode: "A.5.20", mtype: "partial"},
	{src: "C5", srcCode: "C5-SSO-07", tgt: "ISO27001", tgtCode: "A.8.30", mtype: "partial"},
	// SIM — Sicherheitsvorfallsmanagement
	{src: "C5", srcCode: "C5-SIM-01", tgt: "ISO27001", tgtCode: "A.5.24", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-SIM-02", tgt: "ISO27001", tgtCode: "A.5.25", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-SIM-03", tgt: "ISO27001", tgtCode: "A.5.26", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-SIM-04", tgt: "ISO27001", tgtCode: "A.5.27", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-SIM-05", tgt: "ISO27001", tgtCode: "A.6.8", mtype: "partial"},
	{src: "C5", srcCode: "C5-SIM-06", tgt: "ISO27001", tgtCode: "A.5.28", mtype: "partial"},
	// BCM — Business Continuity Management
	{src: "C5", srcCode: "C5-BCM-01", tgt: "ISO27001", tgtCode: "A.5.29", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-BCM-02", tgt: "ISO27001", tgtCode: "A.5.30", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-BCM-03", tgt: "ISO27001", tgtCode: "A.8.13", mtype: "partial"},
	{src: "C5", srcCode: "C5-BCM-04", tgt: "ISO27001", tgtCode: "A.8.14", mtype: "partial"},
	// COM — Compliance
	{src: "C5", srcCode: "C5-COM-01", tgt: "ISO27001", tgtCode: "A.5.31", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-COM-02", tgt: "ISO27001", tgtCode: "A.5.36", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-COM-03", tgt: "ISO27001", tgtCode: "A.5.32", mtype: "partial"},
	{src: "C5", srcCode: "C5-COM-04", tgt: "ISO27001", tgtCode: "A.5.34", mtype: "partial"},
}

// SeedC5ISO27001Mappings seeds BSI C5:2026 ↔ ISO 27001:2022 bidirectional mappings.
func (s *Service) SeedC5ISO27001Mappings(ctx context.Context, orgID string) error {
	return s.seedBidirectionalMappings(ctx, orgID, "C5", "ISO27001", c5ISO27001Mappings)
}

// ── BSI C5:2026 ↔ NIS2 mappings ──────────────────────────────────────────────

var c5NIS2Mappings = []frameworkPair{
	{src: "C5", srcCode: "C5-OIS-01", tgt: "NIS2", tgtCode: "NIS2-A.1", mtype: "partial"},
	{src: "C5", srcCode: "C5-OIS-02", tgt: "NIS2", tgtCode: "NIS2-A.1", mtype: "partial"},
	{src: "C5", srcCode: "C5-SP-01", tgt: "NIS2", tgtCode: "NIS2-A.1", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-SP-02", tgt: "NIS2", tgtCode: "NIS2-A.1", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-SIM-01", tgt: "NIS2", tgtCode: "NIS2-B.1", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-SIM-02", tgt: "NIS2", tgtCode: "NIS2-B.1", mtype: "partial"},
	{src: "C5", srcCode: "C5-SIM-05", tgt: "NIS2", tgtCode: "NIS2-B.5", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-SIM-01", tgt: "NIS2", tgtCode: "NIS2-B.5", mtype: "partial"},
	{src: "C5", srcCode: "C5-BCM-01", tgt: "NIS2", tgtCode: "NIS2-C.1", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-BCM-02", tgt: "NIS2", tgtCode: "NIS2-C.1", mtype: "partial"},
	{src: "C5", srcCode: "C5-BCM-03", tgt: "NIS2", tgtCode: "NIS2-C.4", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-SSO-01", tgt: "NIS2", tgtCode: "NIS2-D.1", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-SSO-02", tgt: "NIS2", tgtCode: "NIS2-D.1", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-SSO-03", tgt: "NIS2", tgtCode: "NIS2-D.1", mtype: "partial"},
	{src: "C5", srcCode: "C5-OPS-01", tgt: "NIS2", tgtCode: "NIS2-E.3", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-OPS-02", tgt: "NIS2", tgtCode: "NIS2-E.4", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-OPS-07", tgt: "NIS2", tgtCode: "NIS2-E.3", mtype: "partial"},
	{src: "C5", srcCode: "C5-OPS-04", tgt: "NIS2", tgtCode: "NIS2-E.8", mtype: "partial"},
	{src: "C5", srcCode: "C5-OPS-06", tgt: "NIS2", tgtCode: "NIS2-E.8", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-COS-01", tgt: "NIS2", tgtCode: "NIS2-E.8", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-COS-02", tgt: "NIS2", tgtCode: "NIS2-E.8", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-HR-03", tgt: "NIS2", tgtCode: "NIS2-G.2", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-CRY-01", tgt: "NIS2", tgtCode: "NIS2-H.1", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-CRY-02", tgt: "NIS2", tgtCode: "NIS2-H.1", mtype: "partial"},
	{src: "C5", srcCode: "C5-IAM-01", tgt: "NIS2", tgtCode: "NIS2-F.1", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-IAM-07", tgt: "NIS2", tgtCode: "NIS2-F.1", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-AM-01", tgt: "NIS2", tgtCode: "NIS2-A.8", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-AM-04", tgt: "NIS2", tgtCode: "NIS2-A.8", mtype: "partial"},
	{src: "C5", srcCode: "C5-HR-01", tgt: "NIS2", tgtCode: "NIS2-A.6", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-HR-05", tgt: "NIS2", tgtCode: "NIS2-A.6", mtype: "partial"},
	{src: "C5", srcCode: "C5-DEV-01", tgt: "NIS2", tgtCode: "NIS2-E.1", mtype: "partial"},
	{src: "C5", srcCode: "C5-DEV-03", tgt: "NIS2", tgtCode: "NIS2-E.1", mtype: "partial"},
}

// SeedC5NIS2Mappings seeds BSI C5:2026 ↔ NIS2 bidirectional mappings.
func (s *Service) SeedC5NIS2Mappings(ctx context.Context, orgID string) error {
	return s.seedBidirectionalMappings(ctx, orgID, "C5", "NIS2", c5NIS2Mappings)
}

// ── KRITIS-DachG ↔ ISO 27001:2022 mappings ───────────────────────────────────

// kritisISO27001Mappings maps KRITIS-DachG DG.x requirements to ISO 27001:2022 Annex A.
// Source: OpenKRITIS KRITIS-DachG mapping table (BGBl. 2026 I Nr. 66, March 2026).
var kritisISO27001Mappings = []frameworkPair{
	// DG.1 — Registrierung (§4 Abs.2)
	{src: "KRITIS", srcCode: "KRITIS-DG.1", tgt: "ISO27001", tgtCode: "A.5.5", mtype: "partial"},
	{src: "KRITIS", srcCode: "KRITIS-DG.1", tgt: "ISO27001", tgtCode: "A.5.31", mtype: "partial"},
	// DG.2 — Kritikalitätsbewertung (§5)
	{src: "KRITIS", srcCode: "KRITIS-DG.2", tgt: "ISO27001", tgtCode: "A.5.30", mtype: "partial"},
	{src: "KRITIS", srcCode: "KRITIS-DG.2", tgt: "ISO27001", tgtCode: "A.5.2", mtype: "partial"},
	// DG.3 — Risikoanalyse & ISMS (§6)
	{src: "KRITIS", srcCode: "KRITIS-DG.3", tgt: "ISO27001", tgtCode: "A.5.1", mtype: "equivalent"},
	{src: "KRITIS", srcCode: "KRITIS-DG.3", tgt: "ISO27001", tgtCode: "A.5.4", mtype: "partial"},
	{src: "KRITIS", srcCode: "KRITIS-DG.3", tgt: "ISO27001", tgtCode: "A.5.29", mtype: "partial"},
	{src: "KRITIS", srcCode: "KRITIS-DG.3", tgt: "ISO27001", tgtCode: "A.5.30", mtype: "partial"},
	// DG.4 — Lieferkettenmanagement (§7)
	{src: "KRITIS", srcCode: "KRITIS-DG.4", tgt: "ISO27001", tgtCode: "A.5.19", mtype: "equivalent"},
	{src: "KRITIS", srcCode: "KRITIS-DG.4", tgt: "ISO27001", tgtCode: "A.5.20", mtype: "equivalent"},
	{src: "KRITIS", srcCode: "KRITIS-DG.4", tgt: "ISO27001", tgtCode: "A.5.21", mtype: "partial"},
	// DG.5 — Sicherheitsmaßnahmen (§8)
	{src: "KRITIS", srcCode: "KRITIS-DG.5", tgt: "ISO27001", tgtCode: "A.5.15", mtype: "partial"},
	{src: "KRITIS", srcCode: "KRITIS-DG.5", tgt: "ISO27001", tgtCode: "A.8.5", mtype: "partial"},
	{src: "KRITIS", srcCode: "KRITIS-DG.5", tgt: "ISO27001", tgtCode: "A.8.24", mtype: "partial"},
	// DG.6 — Zugriffskontrolle (§8 Abs.3)
	{src: "KRITIS", srcCode: "KRITIS-DG.6", tgt: "ISO27001", tgtCode: "A.5.15", mtype: "equivalent"},
	{src: "KRITIS", srcCode: "KRITIS-DG.6", tgt: "ISO27001", tgtCode: "A.5.16", mtype: "equivalent"},
	{src: "KRITIS", srcCode: "KRITIS-DG.6", tgt: "ISO27001", tgtCode: "A.5.18", mtype: "partial"},
	// DG.7 — Physische Sicherheit (§9)
	{src: "KRITIS", srcCode: "KRITIS-DG.7", tgt: "ISO27001", tgtCode: "A.7.1", mtype: "equivalent"},
	{src: "KRITIS", srcCode: "KRITIS-DG.7", tgt: "ISO27001", tgtCode: "A.7.2", mtype: "equivalent"},
	{src: "KRITIS", srcCode: "KRITIS-DG.7", tgt: "ISO27001", tgtCode: "A.7.3", mtype: "partial"},
	// DG.8 — Umgebungsrisiken (§9 Abs.2)
	{src: "KRITIS", srcCode: "KRITIS-DG.8", tgt: "ISO27001", tgtCode: "A.7.5", mtype: "partial"},
	{src: "KRITIS", srcCode: "KRITIS-DG.8", tgt: "ISO27001", tgtCode: "A.7.11", mtype: "partial"},
	// DG.9 — Überwachung & Logging (§10)
	{src: "KRITIS", srcCode: "KRITIS-DG.9", tgt: "ISO27001", tgtCode: "A.8.15", mtype: "equivalent"},
	{src: "KRITIS", srcCode: "KRITIS-DG.9", tgt: "ISO27001", tgtCode: "A.8.16", mtype: "equivalent"},
	{src: "KRITIS", srcCode: "KRITIS-DG.9", tgt: "ISO27001", tgtCode: "A.8.17", mtype: "partial"},
	// DG.10 — Backup & Wiederherstellung (§10 Abs.2)
	{src: "KRITIS", srcCode: "KRITIS-DG.10", tgt: "ISO27001", tgtCode: "A.8.13", mtype: "equivalent"},
	{src: "KRITIS", srcCode: "KRITIS-DG.10", tgt: "ISO27001", tgtCode: "A.8.14", mtype: "equivalent"},
	// DG.11 — Kryptographie (§11)
	{src: "KRITIS", srcCode: "KRITIS-DG.11", tgt: "ISO27001", tgtCode: "A.8.24", mtype: "equivalent"},
	{src: "KRITIS", srcCode: "KRITIS-DG.11", tgt: "ISO27001", tgtCode: "A.5.31", mtype: "partial"},
	// DG.12 — Netzwerksicherheit (§11 Abs.3)
	{src: "KRITIS", srcCode: "KRITIS-DG.12", tgt: "ISO27001", tgtCode: "A.8.20", mtype: "equivalent"},
	{src: "KRITIS", srcCode: "KRITIS-DG.12", tgt: "ISO27001", tgtCode: "A.8.21", mtype: "equivalent"},
	{src: "KRITIS", srcCode: "KRITIS-DG.12", tgt: "ISO27001", tgtCode: "A.8.22", mtype: "partial"},
	// DG.13 — Schwachstellenmanagement (§12)
	{src: "KRITIS", srcCode: "KRITIS-DG.13", tgt: "ISO27001", tgtCode: "A.8.8", mtype: "equivalent"},
	{src: "KRITIS", srcCode: "KRITIS-DG.13", tgt: "ISO27001", tgtCode: "A.8.32", mtype: "equivalent"},
	// DG.14 — Vorfallsmanagement (§12 Abs.2)
	{src: "KRITIS", srcCode: "KRITIS-DG.14", tgt: "ISO27001", tgtCode: "A.5.24", mtype: "equivalent"},
	{src: "KRITIS", srcCode: "KRITIS-DG.14", tgt: "ISO27001", tgtCode: "A.5.25", mtype: "equivalent"},
	{src: "KRITIS", srcCode: "KRITIS-DG.14", tgt: "ISO27001", tgtCode: "A.6.8", mtype: "partial"},
	// DG.15 — Meldepflichten (§13)
	{src: "KRITIS", srcCode: "KRITIS-DG.15", tgt: "ISO27001", tgtCode: "A.5.5", mtype: "partial"},
	{src: "KRITIS", srcCode: "KRITIS-DG.15", tgt: "ISO27001", tgtCode: "A.5.24", mtype: "partial"},
	// DG.16 — Schulungen & Awareness (§14)
	{src: "KRITIS", srcCode: "KRITIS-DG.16", tgt: "ISO27001", tgtCode: "A.6.3", mtype: "equivalent"},
	{src: "KRITIS", srcCode: "KRITIS-DG.16", tgt: "ISO27001", tgtCode: "A.6.5", mtype: "partial"},
	// DG.18 — Resilienzplan (§13 Abs.4)
	{src: "KRITIS", srcCode: "KRITIS-DG.18", tgt: "ISO27001", tgtCode: "A.5.30", mtype: "equivalent"},
	{src: "KRITIS", srcCode: "KRITIS-DG.18", tgt: "ISO27001", tgtCode: "A.5.29", mtype: "partial"},
	// DG.19 — BCM-Tests (§15)
	{src: "KRITIS", srcCode: "KRITIS-DG.19", tgt: "ISO27001", tgtCode: "A.5.30", mtype: "partial"},
	{src: "KRITIS", srcCode: "KRITIS-DG.19", tgt: "ISO27001", tgtCode: "A.8.14", mtype: "partial"},
	// DG.20 — Leitungsverantwortung (§16)
	{src: "KRITIS", srcCode: "KRITIS-DG.20", tgt: "ISO27001", tgtCode: "A.5.2", mtype: "equivalent"},
	{src: "KRITIS", srcCode: "KRITIS-DG.20", tgt: "ISO27001", tgtCode: "A.5.4", mtype: "partial"},
	// DG.21 — Auditierung (§17)
	{src: "KRITIS", srcCode: "KRITIS-DG.21", tgt: "ISO27001", tgtCode: "A.5.35", mtype: "equivalent"},
	{src: "KRITIS", srcCode: "KRITIS-DG.21", tgt: "ISO27001", tgtCode: "A.5.36", mtype: "partial"},
	// DG.22 — Nachweispflichten (§18)
	{src: "KRITIS", srcCode: "KRITIS-DG.22", tgt: "ISO27001", tgtCode: "A.5.36", mtype: "partial"},
	// DG.23–DG.26 — Behördenkooperation, Strafrecht, Haftung, Sanktionen (§19–§22)
	{src: "KRITIS", srcCode: "KRITIS-DG.23", tgt: "ISO27001", tgtCode: "A.5.5", mtype: "partial"},
	{src: "KRITIS", srcCode: "KRITIS-DG.24", tgt: "ISO27001", tgtCode: "A.5.31", mtype: "partial"},
	{src: "KRITIS", srcCode: "KRITIS-DG.25", tgt: "ISO27001", tgtCode: "A.5.4", mtype: "partial"},
	{src: "KRITIS", srcCode: "KRITIS-DG.26", tgt: "ISO27001", tgtCode: "A.5.36", mtype: "partial"},
}

// SeedKRITISISO27001Mappings seeds KRITIS-DachG ↔ ISO 27001:2022 bidirectional mappings.
func (s *Service) SeedKRITISISO27001Mappings(ctx context.Context, orgID string) error {
	return s.seedBidirectionalMappings(ctx, orgID, "KRITIS", "ISO27001", kritisISO27001Mappings)
}

// ── KRITIS-DachG ↔ NIS2 mappings ─────────────────────────────────────────────

var kritisNIS2Mappings = []frameworkPair{
	{src: "KRITIS", srcCode: "KRITIS-DG.3", tgt: "NIS2", tgtCode: "NIS2-A.1", mtype: "equivalent"},
	{src: "KRITIS", srcCode: "KRITIS-DG.4", tgt: "NIS2", tgtCode: "NIS2-D.1", mtype: "equivalent"},
	{src: "KRITIS", srcCode: "KRITIS-DG.5", tgt: "NIS2", tgtCode: "NIS2-F.1", mtype: "partial"},
	{src: "KRITIS", srcCode: "KRITIS-DG.6", tgt: "NIS2", tgtCode: "NIS2-F.1", mtype: "equivalent"},
	{src: "KRITIS", srcCode: "KRITIS-DG.7", tgt: "NIS2", tgtCode: "NIS2-E.3", mtype: "partial"},
	{src: "KRITIS", srcCode: "KRITIS-DG.9", tgt: "NIS2", tgtCode: "NIS2-E.3", mtype: "partial"},
	{src: "KRITIS", srcCode: "KRITIS-DG.10", tgt: "NIS2", tgtCode: "NIS2-C.4", mtype: "equivalent"},
	{src: "KRITIS", srcCode: "KRITIS-DG.11", tgt: "NIS2", tgtCode: "NIS2-H.1", mtype: "equivalent"},
	{src: "KRITIS", srcCode: "KRITIS-DG.12", tgt: "NIS2", tgtCode: "NIS2-E.8", mtype: "equivalent"},
	{src: "KRITIS", srcCode: "KRITIS-DG.13", tgt: "NIS2", tgtCode: "NIS2-E.3", mtype: "equivalent"},
	{src: "KRITIS", srcCode: "KRITIS-DG.13", tgt: "NIS2", tgtCode: "NIS2-E.4", mtype: "equivalent"},
	{src: "KRITIS", srcCode: "KRITIS-DG.14", tgt: "NIS2", tgtCode: "NIS2-B.1", mtype: "equivalent"},
	{src: "KRITIS", srcCode: "KRITIS-DG.15", tgt: "NIS2", tgtCode: "NIS2-B.5", mtype: "equivalent"},
	{src: "KRITIS", srcCode: "KRITIS-DG.16", tgt: "NIS2", tgtCode: "NIS2-G.2", mtype: "equivalent"},
	{src: "KRITIS", srcCode: "KRITIS-DG.18", tgt: "NIS2", tgtCode: "NIS2-C.1", mtype: "equivalent"},
	{src: "KRITIS", srcCode: "KRITIS-DG.19", tgt: "NIS2", tgtCode: "NIS2-C.1", mtype: "partial"},
	{src: "KRITIS", srcCode: "KRITIS-DG.20", tgt: "NIS2", tgtCode: "NIS2-A.1", mtype: "partial"},
	{src: "KRITIS", srcCode: "KRITIS-DG.21", tgt: "NIS2", tgtCode: "NIS2-A.1", mtype: "partial"},
}

// SeedKRITISNIS2Mappings seeds KRITIS-DachG ↔ NIS2 bidirectional mappings.
func (s *Service) SeedKRITISNIS2Mappings(ctx context.Context, orgID string) error {
	return s.seedBidirectionalMappings(ctx, orgID, "KRITIS", "NIS2", kritisNIS2Mappings)
}

// ── prEN 18286 ↔ EU AI Act (STUB — Annex ZA, primary use case) ───────────────
// prEN 18286 Annex ZA is a direct Art. 17 QMS clause-by-clause crosswalk.
// When published in the OJEU (expected late 2026), conformity with prEN 18286
// grants presumption of conformity under AI Act Art. 40.

// EU AI Act internal control numbering (from euAIActControls in service_helpers.go):
//   AIACT-1.x = Art. 9 Risikomanagementsystem
//   AIACT-2.x = Art. 10 Datenverwaltung
//   AIACT-3.x = Art. 11 Technische Dokumentation
//   AIACT-4.1 = Art. 12 Automatisches Logging
//   AIACT-5.x = Art. 13 Transparenz
//   AIACT-6.x = Art. 14 Menschliche Aufsicht
//   AIACT-9.x = Art. 17 Qualitätsmanagementsystem

var prEN18286EUAIActMappings = []frameworkPair{
	// Art. 9 — Risikomanagementsystem
	{src: "PREN18286", srcCode: "18286-6.1", tgt: "EUAIACT", tgtCode: "AIACT-1.1", mtype: "equivalent"},
	{src: "PREN18286", srcCode: "18286-6.2", tgt: "EUAIACT", tgtCode: "AIACT-1.2", mtype: "partial"},
	// Art. 10 — Datenverwaltung
	{src: "PREN18286", srcCode: "18286-8.2", tgt: "EUAIACT", tgtCode: "AIACT-2.1", mtype: "equivalent"},
	{src: "PREN18286", srcCode: "18286-A.1", tgt: "EUAIACT", tgtCode: "AIACT-2.2", mtype: "partial"},
	// Art. 11 — Technische Dokumentation
	{src: "PREN18286", srcCode: "18286-7.1", tgt: "EUAIACT", tgtCode: "AIACT-3.1", mtype: "equivalent"},
	{src: "PREN18286", srcCode: "18286-A.2", tgt: "EUAIACT", tgtCode: "AIACT-3.1", mtype: "partial"},
	// Art. 12 — Aufzeichnungspflichten (Logging)
	{src: "PREN18286", srcCode: "18286-9.1", tgt: "EUAIACT", tgtCode: "AIACT-4.1", mtype: "equivalent"},
	// Art. 13 — Transparenz und Nutzerinformation
	{src: "PREN18286", srcCode: "18286-A.2", tgt: "EUAIACT", tgtCode: "AIACT-5.1", mtype: "equivalent"},
	{src: "PREN18286", srcCode: "18286-7.3", tgt: "EUAIACT", tgtCode: "AIACT-5.1", mtype: "partial"},
	// Art. 14 — Menschliche Aufsicht
	{src: "PREN18286", srcCode: "18286-6.2", tgt: "EUAIACT", tgtCode: "AIACT-6.1", mtype: "equivalent"},
	{src: "PREN18286", srcCode: "18286-8.1", tgt: "EUAIACT", tgtCode: "AIACT-6.1", mtype: "partial"},
	// Art. 17 — Qualitätsmanagementsystem (Kernzweck von prEN 18286, Annex ZA)
	{src: "PREN18286", srcCode: "18286-4.1", tgt: "EUAIACT", tgtCode: "AIACT-9.1", mtype: "equivalent"},
	{src: "PREN18286", srcCode: "18286-4.2", tgt: "EUAIACT", tgtCode: "AIACT-9.1", mtype: "equivalent"},
	{src: "PREN18286", srcCode: "18286-5.1", tgt: "EUAIACT", tgtCode: "AIACT-9.1", mtype: "equivalent"},
	{src: "PREN18286", srcCode: "18286-6.1", tgt: "EUAIACT", tgtCode: "AIACT-9.1", mtype: "equivalent"},
	{src: "PREN18286", srcCode: "18286-7.1", tgt: "EUAIACT", tgtCode: "AIACT-9.1", mtype: "equivalent"},
	{src: "PREN18286", srcCode: "18286-8.1", tgt: "EUAIACT", tgtCode: "AIACT-9.1", mtype: "equivalent"},
	{src: "PREN18286", srcCode: "18286-9.2", tgt: "EUAIACT", tgtCode: "AIACT-9.2", mtype: "equivalent"},
	{src: "PREN18286", srcCode: "18286-9.3", tgt: "EUAIACT", tgtCode: "AIACT-9.2", mtype: "partial"},
	{src: "PREN18286", srcCode: "18286-10.1", tgt: "EUAIACT", tgtCode: "AIACT-9.1", mtype: "partial"},
	// Art. 18 — Aufbewahrung der technischen Dokumentation
	{src: "PREN18286", srcCode: "18286-9.1", tgt: "EUAIACT", tgtCode: "AIACT-10.1", mtype: "partial"},
}

// SeedPREN18286EUAIActMappings seeds prEN 18286 ↔ EU AI Act (Annex ZA stub) mappings.
// Update when the standard is finalized and listed in the OJEU (expected late 2026).
func (s *Service) SeedPREN18286EUAIActMappings(ctx context.Context, orgID string) error {
	return s.seedBidirectionalMappings(ctx, orgID, "PREN18286", "EUAIACT", prEN18286EUAIActMappings)
}

// ── KRITIS-DachG ↔ DORA ──────────────────────────────────────────────────────
// Dual-obligation for German financial-sector critical infrastructure operators.
// Registration deadline: July 2026 (both BaFin/DORA and BBK/BSI/KRITIS-DachG).
// Source: KRITIS-DachG §§ + DORA Art. 5-30, OpenKRITIS analysis.

var kritisDORAMappings = []frameworkPair{
	// ISMS / Governance
	{src: "KRITIS", srcCode: "KRITIS-DG.3", tgt: "DORA", tgtCode: "DORA-1.1", mtype: "equivalent"}, // Risikoanalyse → IKT-Rahmen
	{src: "KRITIS", srcCode: "KRITIS-DG.3", tgt: "DORA", tgtCode: "DORA-1.2", mtype: "partial"},    // Risikoanalyse → IKT-Risikobewertung
	{src: "KRITIS", srcCode: "KRITIS-DG.20", tgt: "DORA", tgtCode: "DORA-1.1", mtype: "partial"},   // Leitungsverantwortung → Rahmen
	// Schutzmaßnahmen / Zugriffskontrolle
	{src: "KRITIS", srcCode: "KRITIS-DG.5", tgt: "DORA", tgtCode: "DORA-1.4", mtype: "partial"},  // Sicherheitsmaßnahmen → Schutz
	{src: "KRITIS", srcCode: "KRITIS-DG.6", tgt: "DORA", tgtCode: "DORA-1.4", mtype: "partial"},  // Zugriffskontrolle → Schutz
	{src: "KRITIS", srcCode: "KRITIS-DG.11", tgt: "DORA", tgtCode: "DORA-1.4", mtype: "partial"}, // Kryptographie → Schutz
	// Erkennung / Monitoring
	{src: "KRITIS", srcCode: "KRITIS-DG.9", tgt: "DORA", tgtCode: "DORA-1.5", mtype: "equivalent"}, // Überwachung/Logging → Anomalie-Erkennung
	// Backup & Wiederherstellung
	{src: "KRITIS", srcCode: "KRITIS-DG.10", tgt: "DORA", tgtCode: "DORA-1.7", mtype: "equivalent"}, // Backup → Wiederherstellung
	{src: "KRITIS", srcCode: "KRITIS-DG.19", tgt: "DORA", tgtCode: "DORA-1.7", mtype: "partial"},    // BCM-Tests → Tests
	// Netzwerksicherheit / Schwachstellen
	{src: "KRITIS", srcCode: "KRITIS-DG.12", tgt: "DORA", tgtCode: "DORA-1.4", mtype: "equivalent"}, // Netzwerksicherheit → Schutz
	{src: "KRITIS", srcCode: "KRITIS-DG.13", tgt: "DORA", tgtCode: "DORA-1.8", mtype: "equivalent"}, // Schwachstellenmanagement → Vuln-Handling
	// Vorfallsmanagement und Meldung
	{src: "KRITIS", srcCode: "KRITIS-DG.14", tgt: "DORA", tgtCode: "DORA-2.1", mtype: "equivalent"}, // Vorfallsmanagement → IR-Richtlinie
	{src: "KRITIS", srcCode: "KRITIS-DG.14", tgt: "DORA", tgtCode: "DORA-2.3", mtype: "equivalent"}, // Vorfallsmanagement → IR-Prozess
	{src: "KRITIS", srcCode: "KRITIS-DG.15", tgt: "DORA", tgtCode: "DORA-2.2", mtype: "equivalent"}, // Meldepflichten → Meldeverfahren
	// BCM / Resilienz
	{src: "KRITIS", srcCode: "KRITIS-DG.18", tgt: "DORA", tgtCode: "DORA-1.6", mtype: "equivalent"}, // Resilienzplan → BCP
	{src: "KRITIS", srcCode: "KRITIS-DG.19", tgt: "DORA", tgtCode: "DORA-1.6", mtype: "partial"},    // BCM-Tests → BCP-Tests
	// Lieferkette / Drittparteien
	{src: "KRITIS", srcCode: "KRITIS-DG.4", tgt: "DORA", tgtCode: "DORA-4.1", mtype: "equivalent"}, // Lieferkettenmanagement → TPICT-Richtlinie
	{src: "KRITIS", srcCode: "KRITIS-DG.4", tgt: "DORA", tgtCode: "DORA-4.2", mtype: "partial"},    // Lieferkette → TPICT-Register
}

// SeedKRITISDORAMappings seeds KRITIS-DachG ↔ DORA bidirectional mappings.
func (s *Service) SeedKRITISDORAMappings(ctx context.Context, orgID string) error {
	return s.seedBidirectionalMappings(ctx, orgID, "KRITIS", "DORA", kritisDORAMappings)
}

// ── BSI IT-Grundschutz ↔ KRITIS-DachG ────────────────────────────────────────
// Source: OpenKRITIS KRITIS-Cyber-Security mapping PDF + BSI 200-2 Bausteine.
// BSI has not published a formal Grundschutz ↔ KRITIS-DachG table (KRITIS-DachG
// came into force March 2026); this mapping is based on structural alignment.

var bsiKRITISMappings = []frameworkPair{
	// DG.3 — ISMS
	{src: "BSI", srcCode: "BSI-ORP.1", tgt: "KRITIS", tgtCode: "KRITIS-DG.3", mtype: "equivalent"},  // Organisation IS → ISMS
	{src: "BSI", srcCode: "BSI-ORP.2", tgt: "KRITIS", tgtCode: "KRITIS-DG.16", mtype: "equivalent"}, // Personal → Schulungen/Awareness
	// DG.4 — Lieferkette
	{src: "BSI", srcCode: "BSI-OPS.2.3", tgt: "KRITIS", tgtCode: "KRITIS-DG.4", mtype: "equivalent"}, // IT-Service-Provider → Lieferkette
	// DG.7/DG.8 — Physische Sicherheit
	{src: "BSI", srcCode: "BSI-INF.1", tgt: "KRITIS", tgtCode: "KRITIS-DG.7", mtype: "equivalent"}, // Gebäude → Physische Sicherheit
	{src: "BSI", srcCode: "BSI-INF.2", tgt: "KRITIS", tgtCode: "KRITIS-DG.7", mtype: "partial"},    // Rechenzentrum → Physische Sicherheit
	{src: "BSI", srcCode: "BSI-INF.4", tgt: "KRITIS", tgtCode: "KRITIS-DG.8", mtype: "partial"},    // IT-Verkabelung → Umgebungsrisiken
	// DG.9 — Monitoring/Logging
	{src: "BSI", srcCode: "BSI-OPS.1.1.5", tgt: "KRITIS", tgtCode: "KRITIS-DG.9", mtype: "equivalent"}, // Protokollierung → Monitoring
	{src: "BSI", srcCode: "BSI-DER.1", tgt: "KRITIS", tgtCode: "KRITIS-DG.9", mtype: "partial"},        // Detektion → Überwachung
	// DG.10 — Backup
	{src: "BSI", srcCode: "BSI-CON.3", tgt: "KRITIS", tgtCode: "KRITIS-DG.10", mtype: "equivalent"}, // Datensicherung → Backup
	// DG.11 — Kryptographie
	{src: "BSI", srcCode: "BSI-CON.1", tgt: "KRITIS", tgtCode: "KRITIS-DG.11", mtype: "equivalent"}, // Kryptokonzept → Kryptographie
	// DG.12 — Netzwerk
	{src: "BSI", srcCode: "BSI-NET.1.1", tgt: "KRITIS", tgtCode: "KRITIS-DG.12", mtype: "equivalent"}, // Netzarchitektur → Netzwerksicherheit
	{src: "BSI", srcCode: "BSI-NET.1.2", tgt: "KRITIS", tgtCode: "KRITIS-DG.12", mtype: "partial"},    // Netzwerkmanagement → Netzwerksicherheit
	// DG.13 — Schwachstellenmanagement
	{src: "BSI", srcCode: "BSI-OPS.1.1.2", tgt: "KRITIS", tgtCode: "KRITIS-DG.13", mtype: "equivalent"}, // Patch-Management → Schwachstellen
	// DG.14/DG.15 — Vorfallsmanagement und Meldung
	{src: "BSI", srcCode: "BSI-DER.2.1", tgt: "KRITIS", tgtCode: "KRITIS-DG.14", mtype: "equivalent"}, // Incident-Handling → Vorfallsmanagement
	{src: "BSI", srcCode: "BSI-DER.2.2", tgt: "KRITIS", tgtCode: "KRITIS-DG.14", mtype: "partial"},    // IT-Forensik → Vorfallsbehandlung
	{src: "BSI", srcCode: "BSI-DER.2.1", tgt: "KRITIS", tgtCode: "KRITIS-DG.15", mtype: "partial"},    // Incident-Handling → Meldepflichten
	// DG.18/DG.19 — BCM / Resilienzplan
	{src: "BSI", srcCode: "BSI-BCM.1", tgt: "KRITIS", tgtCode: "KRITIS-DG.18", mtype: "equivalent"}, // BCM-Konzeption → Resilienzplan
	{src: "BSI", srcCode: "BSI-BCM.2", tgt: "KRITIS", tgtCode: "KRITIS-DG.19", mtype: "equivalent"}, // BCM-Übungen → BCM-Tests
	// DG.20/DG.21 — Leitungsverantwortung/Audit
	{src: "BSI", srcCode: "BSI-ORP.1", tgt: "KRITIS", tgtCode: "KRITIS-DG.20", mtype: "partial"},      // Organisation → Leitungsverantwortung
	{src: "BSI", srcCode: "BSI-DER.3.1", tgt: "KRITIS", tgtCode: "KRITIS-DG.21", mtype: "equivalent"}, // Revisionen → Auditierung
}

// SeedBSIKRITISMappings seeds BSI IT-Grundschutz ↔ KRITIS-DachG bidirectional mappings.
func (s *Service) SeedBSIKRITISMappings(ctx context.Context, orgID string) error {
	return s.seedBidirectionalMappings(ctx, orgID, "BSI", "KRITIS", bsiKRITISMappings)
}

// ── BSI C5:2026 ↔ BSI IT-Grundschutz ─────────────────────────────────────────
// Based on C5:2020 reference tables (BSI Excel); C5:2026 official cross-reference
// table is pending Q2/Q3 2026 — update when published.

var c5BSIMappings = []frameworkPair{
	// OIS ↔ ORP (Organisation der IS)
	{src: "C5", srcCode: "C5-OIS-01", tgt: "BSI", tgtCode: "BSI-ORP.1", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-OIS-02", tgt: "BSI", tgtCode: "BSI-ORP.1", mtype: "partial"},
	{src: "C5", srcCode: "C5-OIS-03", tgt: "BSI", tgtCode: "BSI-ORP.1", mtype: "partial"},
	// HR ↔ ORP.2 (Personal)
	{src: "C5", srcCode: "C5-HR-01", tgt: "BSI", tgtCode: "BSI-ORP.2", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-HR-02", tgt: "BSI", tgtCode: "BSI-ORP.2", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-HR-03", tgt: "BSI", tgtCode: "BSI-ORP.3", mtype: "equivalent"}, // Sensibilisierung/Schulung
	{src: "C5", srcCode: "C5-HR-04", tgt: "BSI", tgtCode: "BSI-ORP.2", mtype: "partial"},
	{src: "C5", srcCode: "C5-HR-05", tgt: "BSI", tgtCode: "BSI-ORP.2", mtype: "partial"},
	// AM ↔ ORP.4 / ISMS (Asset Management)
	{src: "C5", srcCode: "C5-AM-01", tgt: "BSI", tgtCode: "BSI-ORP.4", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-AM-04", tgt: "BSI", tgtCode: "BSI-ORP.4", mtype: "partial"},
	// PS ↔ INF (Physische Sicherheit)
	{src: "C5", srcCode: "C5-PS-01", tgt: "BSI", tgtCode: "BSI-INF.1", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-PS-02", tgt: "BSI", tgtCode: "BSI-INF.2", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-PS-03", tgt: "BSI", tgtCode: "BSI-INF.1", mtype: "partial"},
	{src: "C5", srcCode: "C5-PS-07", tgt: "BSI", tgtCode: "BSI-INF.3", mtype: "partial"}, // Stromversorgung
	{src: "C5", srcCode: "C5-PS-08", tgt: "BSI", tgtCode: "BSI-INF.4", mtype: "partial"}, // Verkabelung
	// OPS ↔ OPS + CON (Betrieb)
	{src: "C5", srcCode: "C5-OPS-01", tgt: "BSI", tgtCode: "BSI-OPS.1.1.2", mtype: "equivalent"}, // Patch-Management
	{src: "C5", srcCode: "C5-OPS-07", tgt: "BSI", tgtCode: "BSI-OPS.1.1.5", mtype: "equivalent"}, // Protokollierung
	{src: "C5", srcCode: "C5-OPS-08", tgt: "BSI", tgtCode: "BSI-OPS.1.1.5", mtype: "partial"},    // Log-Management
	{src: "C5", srcCode: "C5-OPS-11", tgt: "BSI", tgtCode: "BSI-CON.3", mtype: "equivalent"},     // Backup
	{src: "C5", srcCode: "C5-OPS-12", tgt: "BSI", tgtCode: "BSI-CON.3", mtype: "partial"},        // Recovery
	// IAM ↔ ORP.4 / SYS (Identitäts- und Zugriffsverwaltung)
	{src: "C5", srcCode: "C5-IAM-01", tgt: "BSI", tgtCode: "BSI-ORP.4", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-IAM-02", tgt: "BSI", tgtCode: "BSI-ORP.4", mtype: "partial"},
	// CRY ↔ CON.1 (Kryptographie)
	{src: "C5", srcCode: "C5-CRY-01", tgt: "BSI", tgtCode: "BSI-CON.1", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-CRY-02", tgt: "BSI", tgtCode: "BSI-CON.1", mtype: "partial"},
	// COS ↔ NET (Kommunikationssicherheit)
	{src: "C5", srcCode: "C5-COS-01", tgt: "BSI", tgtCode: "BSI-NET.1.1", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-COS-02", tgt: "BSI", tgtCode: "BSI-NET.1.1", mtype: "partial"},
	{src: "C5", srcCode: "C5-COS-03", tgt: "BSI", tgtCode: "BSI-NET.1.2", mtype: "partial"},
	// DEV ↔ CON.8 / OPS.1.1.6 (Entwicklung)
	{src: "C5", srcCode: "C5-DEV-01", tgt: "BSI", tgtCode: "BSI-CON.8", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-DEV-05", tgt: "BSI", tgtCode: "BSI-CON.8", mtype: "partial"},
	// SSO ↔ OPS.2 (Dienstleister)
	{src: "C5", srcCode: "C5-SSO-01", tgt: "BSI", tgtCode: "BSI-OPS.2.3", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-SSO-02", tgt: "BSI", tgtCode: "BSI-OPS.2.3", mtype: "partial"},
	// SIM ↔ DER.2 (Vorfallsmanagement)
	{src: "C5", srcCode: "C5-SIM-01", tgt: "BSI", tgtCode: "BSI-DER.2.1", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-SIM-02", tgt: "BSI", tgtCode: "BSI-DER.2.1", mtype: "partial"},
	// BCM ↔ BCM (Business Continuity)
	{src: "C5", srcCode: "C5-BCM-01", tgt: "BSI", tgtCode: "BSI-BCM.1", mtype: "equivalent"},
	{src: "C5", srcCode: "C5-BCM-02", tgt: "BSI", tgtCode: "BSI-BCM.1", mtype: "partial"},
	{src: "C5", srcCode: "C5-BCM-03", tgt: "BSI", tgtCode: "BSI-CON.3", mtype: "partial"},
}

// SeedC5BSIMappings seeds BSI C5:2026 ↔ BSI IT-Grundschutz bidirectional mappings.
// Based on C5:2020 reference tables; update when BSI publishes C5:2026 cross-ref (Q3 2026).
func (s *Service) SeedC5BSIMappings(ctx context.Context, orgID string) error {
	return s.seedBidirectionalMappings(ctx, orgID, "C5", "BSI", c5BSIMappings)
}

// ── TISAX ↔ NIS2 ──────────────────────────────────────────────────────────────
// Source: ENX Association "NIS2 fulfilment through TISAX" expert opinion.
// TISAX covers all NIS2 Art. 21 technical/organisational measures.
// Gap: NIS2 Art. 23 reporting obligations to national CSIRTs are NOT covered by TISAX.

var tisaxNIS2Mappings = []frameworkPair{
	{src: "TISAX", srcCode: "TISAX-1.1.1", tgt: "NIS2", tgtCode: "NIS2-A.1", mtype: "equivalent"}, // IS-Politik → Governance
	{src: "TISAX", srcCode: "TISAX-1.1.3", tgt: "NIS2", tgtCode: "NIS2-A.2", mtype: "partial"},    // Leadership → Policy
	{src: "TISAX", srcCode: "TISAX-3.1.2", tgt: "NIS2", tgtCode: "NIS2-G.2", mtype: "equivalent"}, // Schulung → Awareness
	{src: "TISAX", srcCode: "TISAX-4.1.1", tgt: "NIS2", tgtCode: "NIS2-A.8", mtype: "equivalent"}, // Asset-Inventar → Asset Management
	{src: "TISAX", srcCode: "TISAX-4.1.3", tgt: "NIS2", tgtCode: "NIS2-A.8", mtype: "partial"},    // Klassifizierung → Asset Management
	{src: "TISAX", srcCode: "TISAX-5.1.1", tgt: "NIS2", tgtCode: "NIS2-F.1", mtype: "equivalent"}, // Zugriffssteuerung → Zugriffskontrolle
	{src: "TISAX", srcCode: "TISAX-5.1.2", tgt: "NIS2", tgtCode: "NIS2-F.1", mtype: "partial"},    // Authentifizierung → MFA
	{src: "TISAX", srcCode: "TISAX-6.1.1", tgt: "NIS2", tgtCode: "NIS2-H.1", mtype: "equivalent"}, // Kryptographie → Verschlüsselung
	{src: "TISAX", srcCode: "TISAX-7.1.1", tgt: "NIS2", tgtCode: "NIS2-E.3", mtype: "equivalent"}, // Schwachstellen → Vulnerability-Mgmt
	{src: "TISAX", srcCode: "TISAX-7.1.2", tgt: "NIS2", tgtCode: "NIS2-E.4", mtype: "equivalent"}, // Patch-Management → Patch
	{src: "TISAX", srcCode: "TISAX-8.1.1", tgt: "NIS2", tgtCode: "NIS2-B.1", mtype: "equivalent"}, // Vorfallsmanagement → Incident Response
	{src: "TISAX", srcCode: "TISAX-9.1.1", tgt: "NIS2", tgtCode: "NIS2-C.1", mtype: "equivalent"}, // BCM → Continuity
	{src: "TISAX", srcCode: "TISAX-9.1.2", tgt: "NIS2", tgtCode: "NIS2-C.4", mtype: "equivalent"}, // Backup → BCM Backup
	{src: "TISAX", srcCode: "TISAX-2.1.3", tgt: "NIS2", tgtCode: "NIS2-D.1", mtype: "partial"},    // Lieferanten → Supply Chain
	{src: "TISAX", srcCode: "TISAX-2.1.4", tgt: "NIS2", tgtCode: "NIS2-E.8", mtype: "partial"},    // Mobiles Arbeiten → Netzwerksicherheit
}

// SeedTISAXNIS2Mappings seeds TISAX ↔ NIS2 bidirectional mappings.
// Per ENX expert opinion, TISAX covers NIS2 Art. 21 controls but NOT Art. 23 reporting.
func (s *Service) SeedTISAXNIS2Mappings(ctx context.Context, orgID string) error {
	return s.seedBidirectionalMappings(ctx, orgID, "TISAX", "NIS2", tisaxNIS2Mappings)
}

// ── ISO 27017 ↔ ISO 27001:2022 mappings ──────────────────────────────────────
// ISO 27017 extends ISO 27001 with cloud-specific controls.
// Source: ISO 27017:2015 Annex A correlation table.

var iso27017ISO27001Mappings = []frameworkPair{
	{src: "ISO27017", srcCode: "27017-6.3.1", tgt: "ISO27001", tgtCode: "A.5.19", mtype: "partial"},         // Shared responsibility → Supply chain
	{src: "ISO27017", srcCode: "27017-6.3.2", tgt: "ISO27001", tgtCode: "A.5.11", mtype: "equivalent"},      // Asset return → Return of assets
	{src: "ISO27017", srcCode: "27017-8.1.1", tgt: "ISO27001", tgtCode: "A.5.9", mtype: "equivalent"},       // Cloud asset inventory
	{src: "ISO27017", srcCode: "27017-8.1.3", tgt: "ISO27001", tgtCode: "A.5.10", mtype: "partial"},         // Asset acceptable use
	{src: "ISO27017", srcCode: "27017-9.1.2", tgt: "ISO27001", tgtCode: "A.5.15", mtype: "equivalent"},      // Access control
	{src: "ISO27017", srcCode: "27017-9.4.4", tgt: "ISO27001", tgtCode: "A.8.18", mtype: "equivalent"},      // Privileged utilities
	{src: "ISO27017", srcCode: "27017-10.1.1", tgt: "ISO27001", tgtCode: "A.8.24", mtype: "equivalent"},     // Encryption at rest/transit
	{src: "ISO27017", srcCode: "27017-10.1.2", tgt: "ISO27001", tgtCode: "A.8.25", mtype: "equivalent"},     // Key management
	{src: "ISO27017", srcCode: "27017-11.2.7", tgt: "ISO27001", tgtCode: "A.7.10", mtype: "equivalent"},     // Storage media disposal
	{src: "ISO27017", srcCode: "27017-12.1.3", tgt: "ISO27001", tgtCode: "A.8.6", mtype: "partial"},         // Capacity management
	{src: "ISO27017", srcCode: "27017-12.4.1", tgt: "ISO27001", tgtCode: "A.8.15", mtype: "equivalent"},     // Event logging
	{src: "ISO27017", srcCode: "27017-12.6.1", tgt: "ISO27001", tgtCode: "A.8.8", mtype: "equivalent"},      // Vulnerability management
	{src: "ISO27017", srcCode: "27017-13.1.3", tgt: "ISO27001", tgtCode: "A.8.22", mtype: "equivalent"},     // Network segregation
	{src: "ISO27017", srcCode: "27017-15.1.1", tgt: "ISO27001", tgtCode: "A.5.19", mtype: "equivalent"},     // Supplier relationships
	{src: "ISO27017", srcCode: "27017-15.2.1", tgt: "ISO27001", tgtCode: "A.5.22", mtype: "equivalent"},     // Supplier monitoring
	{src: "ISO27017", srcCode: "27017-CLD.6.3.1", tgt: "ISO27001", tgtCode: "A.5.2", mtype: "partial"},      // Shared security — roles
	{src: "ISO27017", srcCode: "27017-CLD.9.5.1", tgt: "ISO27001", tgtCode: "A.8.22", mtype: "equivalent"},  // Segregation in VE
	{src: "ISO27017", srcCode: "27017-CLD.9.5.2", tgt: "ISO27001", tgtCode: "A.8.9", mtype: "partial"},      // VM hardening
	{src: "ISO27017", srcCode: "27017-CLD.12.4.5", tgt: "ISO27001", tgtCode: "A.8.16", mtype: "equivalent"}, // Cloud monitoring
}

// SeedISO27017ISO27001Mappings seeds ISO 27017 ↔ ISO 27001:2022 bidirectional mappings.
func (s *Service) SeedISO27017ISO27001Mappings(ctx context.Context, orgID string) error {
	return s.seedBidirectionalMappings(ctx, orgID, "ISO27017", "ISO27001", iso27017ISO27001Mappings)
}

// ── ISO 27018 ↔ ISO 27001:2022 mappings ──────────────────────────────────────
// ISO 27018 extends ISO 27001 with PII-processor-specific controls for cloud.

var iso27018ISO27001Mappings = []frameworkPair{
	{src: "ISO27018", srcCode: "27018-A.1.1", tgt: "ISO27001", tgtCode: "A.5.31", mtype: "equivalent"}, // Purpose limitation → legal req
	{src: "ISO27018", srcCode: "27018-A.1.2", tgt: "ISO27001", tgtCode: "A.5.36", mtype: "partial"},    // Consent mgmt → compliance
	{src: "ISO27018", srcCode: "27018-A.2.1", tgt: "ISO27001", tgtCode: "A.5.31", mtype: "equivalent"}, // Legal basis for transfers
	{src: "ISO27018", srcCode: "27018-A.3.1", tgt: "ISO27001", tgtCode: "A.5.36", mtype: "partial"},    // Data subject rights
	{src: "ISO27018", srcCode: "27018-A.3.2", tgt: "ISO27001", tgtCode: "A.8.10", mtype: "equivalent"}, // PII deletion/return
	{src: "ISO27018", srcCode: "27018-A.4.1", tgt: "ISO27001", tgtCode: "A.5.19", mtype: "equivalent"}, // Sub-processor disclosure
	{src: "ISO27018", srcCode: "27018-A.4.2", tgt: "ISO27001", tgtCode: "A.5.5", mtype: "partial"},     // Govt access requests
	{src: "ISO27018", srcCode: "27018-A.4.3", tgt: "ISO27001", tgtCode: "A.5.14", mtype: "partial"},    // Processing locations
	{src: "ISO27018", srcCode: "27018-A.5.1", tgt: "ISO27001", tgtCode: "A.8.24", mtype: "equivalent"}, // Encryption at rest
	{src: "ISO27018", srcCode: "27018-A.5.2", tgt: "ISO27001", tgtCode: "A.8.24", mtype: "equivalent"}, // Encryption in transit
	{src: "ISO27018", srcCode: "27018-A.5.3", tgt: "ISO27001", tgtCode: "A.5.15", mtype: "equivalent"}, // Access control for PII
	{src: "ISO27018", srcCode: "27018-A.6.1", tgt: "ISO27001", tgtCode: "A.5.24", mtype: "equivalent"}, // Breach notification
	{src: "ISO27018", srcCode: "27018-A.7.1", tgt: "ISO27001", tgtCode: "A.6.6", mtype: "equivalent"},  // Confidentiality agreements
	{src: "ISO27018", srcCode: "27018-A.7.2", tgt: "ISO27001", tgtCode: "A.8.12", mtype: "partial"},    // Copy restriction → DLP
	{src: "ISO27018", srcCode: "27018-A.8.1", tgt: "ISO27001", tgtCode: "A.8.15", mtype: "equivalent"}, // PII access logging
	{src: "ISO27018", srcCode: "27018-A.8.2", tgt: "ISO27001", tgtCode: "A.5.35", mtype: "equivalent"}, // Compliance audit
}

// SeedISO27018ISO27001Mappings seeds ISO 27018 ↔ ISO 27001:2022 bidirectional mappings.
func (s *Service) SeedISO27018ISO27001Mappings(ctx context.Context, orgID string) error {
	return s.seedBidirectionalMappings(ctx, orgID, "ISO27018", "ISO27001", iso27018ISO27001Mappings)
}

// ── ISO 27017 ↔ NIS2 mappings ─────────────────────────────────────────────────

var iso27017NIS2Mappings = []frameworkPair{
	{src: "ISO27017", srcCode: "27017-9.1.2", tgt: "NIS2", tgtCode: "NIS2-F.1", mtype: "equivalent"},  // Access control → Zugriff/MFA
	{src: "ISO27017", srcCode: "27017-10.1.1", tgt: "NIS2", tgtCode: "NIS2-H.1", mtype: "equivalent"}, // Encryption → Kryptographie
	{src: "ISO27017", srcCode: "27017-12.4.1", tgt: "NIS2", tgtCode: "NIS2-E.3", mtype: "partial"},    // Logging → Monitoring
	{src: "ISO27017", srcCode: "27017-12.6.1", tgt: "NIS2", tgtCode: "NIS2-E.3", mtype: "equivalent"}, // Vuln mgmt → Schwachstellen
	{src: "ISO27017", srcCode: "27017-13.1.3", tgt: "NIS2", tgtCode: "NIS2-E.8", mtype: "equivalent"}, // Network segregation → Netzwerk
	{src: "ISO27017", srcCode: "27017-15.1.1", tgt: "NIS2", tgtCode: "NIS2-D.1", mtype: "equivalent"}, // Supplier → Lieferkette
	{src: "ISO27017", srcCode: "27017-CLD.6.3.1", tgt: "NIS2", tgtCode: "NIS2-A.1", mtype: "partial"}, // Shared responsibility → Governance
}

// SeedISO27017NIS2Mappings seeds ISO 27017 ↔ NIS2 bidirectional mappings.
func (s *Service) SeedISO27017NIS2Mappings(ctx context.Context, orgID string) error {
	return s.seedBidirectionalMappings(ctx, orgID, "ISO27017", "NIS2", iso27017NIS2Mappings)
}

// ── ISO 27017 ↔ BSI C5:2026 ───────────────────────────────────────────────────
// Source: BSI C5:2026 Annex C cross-reference table (ISO/IEC 27017 column).
// C5 cloud criteria map directly to ISO 27017 cloud-specific controls and
// the extended CLD.x controls in the ISO 27017 annex.

var iso27017C5Mappings = []frameworkPair{
	// Shared responsibility / ISMS
	{src: "ISO27017", srcCode: "27017-CLD.6.3.1", tgt: "C5", tgtCode: "C5-OIS-01", mtype: "equivalent"}, // Shared security roles → ISMS
	{src: "ISO27017", srcCode: "27017-CLD.6.3.1", tgt: "C5", tgtCode: "C5-SSO-01", mtype: "partial"},    // Shared responsibility → provider policy
	{src: "ISO27017", srcCode: "27017-6.3.1", tgt: "C5", tgtCode: "C5-OIS-03", mtype: "partial"},        // Cloud-specific roles → interfaces & dependencies
	// Access control (IAM)
	{src: "ISO27017", srcCode: "27017-9.1.2", tgt: "C5", tgtCode: "C5-IAM-01", mtype: "equivalent"}, // Access control policy → IAM policy
	{src: "ISO27017", srcCode: "27017-9.1.2", tgt: "C5", tgtCode: "C5-IAM-02", mtype: "partial"},    // Access management → rights assignment
	{src: "ISO27017", srcCode: "27017-9.4.1", tgt: "C5", tgtCode: "C5-IAM-04", mtype: "equivalent"}, // Access restriction → rights withdrawal
	// Cryptography
	{src: "ISO27017", srcCode: "27017-10.1.1", tgt: "C5", tgtCode: "C5-CRY-04", mtype: "equivalent"}, // Encryption policy → transport encryption
	{src: "ISO27017", srcCode: "27017-10.1.1", tgt: "C5", tgtCode: "C5-CRY-05", mtype: "equivalent"}, // Encryption → at-rest encryption
	{src: "ISO27017", srcCode: "27017-10.1.2", tgt: "C5", tgtCode: "C5-CRY-07", mtype: "equivalent"}, // Key management → key rotation
	{src: "ISO27017", srcCode: "27017-10.1.2", tgt: "C5", tgtCode: "C5-CRY-10", mtype: "equivalent"}, // Key management → key storage
	// Operations
	{src: "ISO27017", srcCode: "27017-12.1.3", tgt: "C5", tgtCode: "C5-OPS-01", mtype: "equivalent"}, // Capacity management → capacity planning
	{src: "ISO27017", srcCode: "27017-12.4.1", tgt: "C5", tgtCode: "C5-OPS-10", mtype: "equivalent"}, // Event logging → logging policy
	{src: "ISO27017", srcCode: "27017-12.4.1", tgt: "C5", tgtCode: "C5-OPS-13", mtype: "partial"},    // Event logging → SIEM
	{src: "ISO27017", srcCode: "27017-12.6.1", tgt: "C5", tgtCode: "C5-OPS-25", mtype: "equivalent"}, // Vulnerability management → vuln scans
	{src: "ISO27017", srcCode: "27017-12.6.1", tgt: "C5", tgtCode: "C5-OPS-27", mtype: "partial"},    // Vulnerability management → patch policy
	// Network security
	{src: "ISO27017", srcCode: "27017-13.1.3", tgt: "C5", tgtCode: "C5-COS-01", mtype: "equivalent"}, // Network segregation → network controls
	{src: "ISO27017", srcCode: "27017-13.1.3", tgt: "C5", tgtCode: "C5-COS-06", mtype: "equivalent"}, // Network segregation → multi-tenant separation
	// Virtual machine / cloud-specific
	{src: "ISO27017", srcCode: "27017-CLD.9.5.1", tgt: "C5", tgtCode: "C5-OPS-30", mtype: "equivalent"},  // VM segregation → data separation policy
	{src: "ISO27017", srcCode: "27017-CLD.9.5.1", tgt: "C5", tgtCode: "C5-OPS-31", mtype: "partial"},     // VM segregation → data separation implementation
	{src: "ISO27017", srcCode: "27017-CLD.9.5.2", tgt: "C5", tgtCode: "C5-OPS-26", mtype: "equivalent"},  // VM hardening → system hardening
	{src: "ISO27017", srcCode: "27017-CLD.9.5.2", tgt: "C5", tgtCode: "C5-OPS-35", mtype: "partial"},     // VM hardening → container security
	{src: "ISO27017", srcCode: "27017-CLD.12.4.5", tgt: "C5", tgtCode: "C5-OPS-13", mtype: "equivalent"}, // Cloud monitoring → SIEM
	{src: "ISO27017", srcCode: "27017-CLD.12.4.5", tgt: "C5", tgtCode: "C5-OPS-15", mtype: "partial"},    // Cloud monitoring → privileged action audit
	// Supplier management
	{src: "ISO27017", srcCode: "27017-15.1.1", tgt: "C5", tgtCode: "C5-SSO-01", mtype: "equivalent"}, // Supplier relationships → provider policy
	{src: "ISO27017", srcCode: "27017-15.1.1", tgt: "C5", tgtCode: "C5-SSO-02", mtype: "partial"},    // Supplier security → provider risk assessment
	{src: "ISO27017", srcCode: "27017-15.2.1", tgt: "C5", tgtCode: "C5-SSO-05", mtype: "equivalent"}, // Supplier monitoring → compliance monitoring
	// Asset management
	{src: "ISO27017", srcCode: "27017-11.2.7", tgt: "C5", tgtCode: "C5-AM-07", mtype: "equivalent"}, // Storage media disposal → hardware disposal
}

// SeedISO27017C5Mappings seeds ISO 27017 ↔ BSI C5:2026 bidirectional mappings.
func (s *Service) SeedISO27017C5Mappings(ctx context.Context, orgID string) error {
	return s.seedBidirectionalMappings(ctx, orgID, "ISO27017", "C5", iso27017C5Mappings)
}

// ── ISO 27018 ↔ DSGVO-TOM ────────────────────────────────────────────────────
// Source: DSK (Datenschutzkonferenz) Orientierungshilfe Auftragsverarbeitung;
// ISO 27018 was designed as the cloud PII-processor complement to DSGVO Art. 28/32.
// The 8 classic TOMs (Zutrittskontrolle … Trennungsgebot) map directly to the
// ISO 27018 Annex A controls for public cloud PII processors.

var iso27018DsgvoTOMMappings = []frameworkPair{
	{src: "ISO27018", srcCode: "27018-A.1.1", tgt: "DSGVO-TOM", tgtCode: "TOM-8", mtype: "equivalent"},  // Purpose limitation → Trennungsgebot
	{src: "ISO27018", srcCode: "27018-A.1.2", tgt: "DSGVO-TOM", tgtCode: "TOM-13", mtype: "partial"},    // Consent management → Überprüfungsverfahren
	{src: "ISO27018", srcCode: "27018-A.2.1", tgt: "DSGVO-TOM", tgtCode: "TOM-4", mtype: "equivalent"},  // Legal basis for transfers → Weitergabekontrolle
	{src: "ISO27018", srcCode: "27018-A.3.1", tgt: "DSGVO-TOM", tgtCode: "TOM-13", mtype: "partial"},    // Data subject rights → Überprüfungsverfahren
	{src: "ISO27018", srcCode: "27018-A.3.2", tgt: "DSGVO-TOM", tgtCode: "TOM-8", mtype: "partial"},     // PII deletion/return → Trennungsgebot
	{src: "ISO27018", srcCode: "27018-A.4.1", tgt: "DSGVO-TOM", tgtCode: "TOM-6", mtype: "equivalent"},  // Sub-processor disclosure → Auftragskontrolle
	{src: "ISO27018", srcCode: "27018-A.4.2", tgt: "DSGVO-TOM", tgtCode: "TOM-3", mtype: "partial"},     // Government access requests → Zugriffskontrolle
	{src: "ISO27018", srcCode: "27018-A.4.3", tgt: "DSGVO-TOM", tgtCode: "TOM-8", mtype: "partial"},     // Processing locations → Trennungsgebot
	{src: "ISO27018", srcCode: "27018-A.5.1", tgt: "DSGVO-TOM", tgtCode: "TOM-10", mtype: "equivalent"}, // Encryption at rest → Verschlüsselung
	{src: "ISO27018", srcCode: "27018-A.5.2", tgt: "DSGVO-TOM", tgtCode: "TOM-4", mtype: "partial"},     // Encryption in transit → Weitergabekontrolle
	{src: "ISO27018", srcCode: "27018-A.5.2", tgt: "DSGVO-TOM", tgtCode: "TOM-10", mtype: "equivalent"}, // Encryption in transit → Verschlüsselung
	{src: "ISO27018", srcCode: "27018-A.5.3", tgt: "DSGVO-TOM", tgtCode: "TOM-3", mtype: "equivalent"},  // Access control for PII → Zugriffskontrolle
	{src: "ISO27018", srcCode: "27018-A.6.1", tgt: "DSGVO-TOM", tgtCode: "TOM-7", mtype: "partial"},     // Breach notification → Verfügbarkeitskontrolle
	{src: "ISO27018", srcCode: "27018-A.7.1", tgt: "DSGVO-TOM", tgtCode: "TOM-2", mtype: "equivalent"},  // Confidentiality agreements → Zugangskontrolle
	{src: "ISO27018", srcCode: "27018-A.7.2", tgt: "DSGVO-TOM", tgtCode: "TOM-3", mtype: "partial"},     // Copy restriction → Zugriffskontrolle
	{src: "ISO27018", srcCode: "27018-A.8.1", tgt: "DSGVO-TOM", tgtCode: "TOM-5", mtype: "equivalent"},  // PII access logging → Eingabekontrolle
	{src: "ISO27018", srcCode: "27018-A.8.2", tgt: "DSGVO-TOM", tgtCode: "TOM-13", mtype: "equivalent"}, // Compliance audit → Überprüfungsverfahren
}

// SeedISO27018DsgvoTOMMappings seeds ISO 27018 ↔ DSGVO-TOM bidirectional mappings.
func (s *Service) SeedISO27018DsgvoTOMMappings(ctx context.Context, orgID string) error {
	return s.seedBidirectionalMappings(ctx, orgID, "ISO27018", "DSGVO-TOM", iso27018DsgvoTOMMappings)
}

// ── DSGVO-TOM ↔ ISO 27001:2022 ───────────────────────────────────────────────
// Source: DSK Orientierungshilfe + ISO 27701 Annex E which links TOM categories
// to ISO 27001 Annex A controls; confirmed by BSI IT-Sicherheitsgesetz commentary.

var dsgvoTOMISO27001Mappings = []frameworkPair{
	{src: "DSGVO-TOM", srcCode: "TOM-1", tgt: "ISO27001", tgtCode: "A.7.1", mtype: "equivalent"},   // Zutrittskontrolle → physical security perimeter
	{src: "DSGVO-TOM", srcCode: "TOM-1", tgt: "ISO27001", tgtCode: "A.7.2", mtype: "partial"},      // Zutrittskontrolle → physical entry controls
	{src: "DSGVO-TOM", srcCode: "TOM-2", tgt: "ISO27001", tgtCode: "A.8.5", mtype: "equivalent"},   // Zugangskontrolle → secure authentication
	{src: "DSGVO-TOM", srcCode: "TOM-2", tgt: "ISO27001", tgtCode: "A.5.15", mtype: "partial"},     // Zugangskontrolle → access control policy
	{src: "DSGVO-TOM", srcCode: "TOM-3", tgt: "ISO27001", tgtCode: "A.5.15", mtype: "equivalent"},  // Zugriffskontrolle → access control
	{src: "DSGVO-TOM", srcCode: "TOM-3", tgt: "ISO27001", tgtCode: "A.5.18", mtype: "partial"},     // Zugriffskontrolle → access rights management
	{src: "DSGVO-TOM", srcCode: "TOM-4", tgt: "ISO27001", tgtCode: "A.8.24", mtype: "equivalent"},  // Weitergabekontrolle → cryptography / TLS
	{src: "DSGVO-TOM", srcCode: "TOM-5", tgt: "ISO27001", tgtCode: "A.8.15", mtype: "equivalent"},  // Eingabekontrolle → logging
	{src: "DSGVO-TOM", srcCode: "TOM-6", tgt: "ISO27001", tgtCode: "A.5.20", mtype: "equivalent"},  // Auftragskontrolle → supplier security in agreements
	{src: "DSGVO-TOM", srcCode: "TOM-6", tgt: "ISO27001", tgtCode: "A.5.19", mtype: "partial"},     // Auftragskontrolle → information security for suppliers
	{src: "DSGVO-TOM", srcCode: "TOM-7", tgt: "ISO27001", tgtCode: "A.8.14", mtype: "equivalent"},  // Verfügbarkeitskontrolle → redundancy/availability
	{src: "DSGVO-TOM", srcCode: "TOM-8", tgt: "ISO27001", tgtCode: "A.8.22", mtype: "equivalent"},  // Trennungsgebot → network segregation
	{src: "DSGVO-TOM", srcCode: "TOM-9", tgt: "ISO27001", tgtCode: "A.8.24", mtype: "partial"},     // Pseudonymisierung → cryptography
	{src: "DSGVO-TOM", srcCode: "TOM-10", tgt: "ISO27001", tgtCode: "A.8.24", mtype: "equivalent"}, // Verschlüsselung → cryptography
	{src: "DSGVO-TOM", srcCode: "TOM-11", tgt: "ISO27001", tgtCode: "A.8.17", mtype: "equivalent"}, // Integrität → clock synchronisation / integrity
	{src: "DSGVO-TOM", srcCode: "TOM-12", tgt: "ISO27001", tgtCode: "A.5.30", mtype: "equivalent"}, // Wiederherstellung → IKT readiness for BCM
	{src: "DSGVO-TOM", srcCode: "TOM-13", tgt: "ISO27001", tgtCode: "A.5.35", mtype: "equivalent"}, // Überprüfungsverfahren → IS review by management
}

// SeedDsgvoTOMISO27001Mappings seeds DSGVO-TOM ↔ ISO 27001:2022 bidirectional mappings.
func (s *Service) SeedDsgvoTOMISO27001Mappings(ctx context.Context, orgID string) error {
	return s.seedBidirectionalMappings(ctx, orgID, "DSGVO-TOM", "ISO27001", dsgvoTOMISO27001Mappings)
}

// ── TISAX ↔ ISO 27001:2022 ────────────────────────────────────────────────────
// Source: VDA ISA 6.0 (2022) cross-reference column "ISO 27001:2022 Annex A".
// TISAX VDA ISA clauses align structurally with ISO 27001:2022; full mapping
// published by ENX Association as part of the TISAX/ISO harmonisation effort.

var tisaxISO27001Mappings = []frameworkPair{
	// Policy & governance
	{src: "TISAX", srcCode: "TISAX-1.1.1", tgt: "ISO27001", tgtCode: "A.5.1", mtype: "equivalent"}, // IS policy → IS policies
	{src: "TISAX", srcCode: "TISAX-1.1.2", tgt: "ISO27001", tgtCode: "A.5.1", mtype: "partial"},    // Policy review → IS policies review
	{src: "TISAX", srcCode: "TISAX-1.1.3", tgt: "ISO27001", tgtCode: "A.5.2", mtype: "equivalent"}, // Leadership commitment → IS roles & responsibilities
	// Organization
	{src: "TISAX", srcCode: "TISAX-2.1.1", tgt: "ISO27001", tgtCode: "A.5.2", mtype: "equivalent"}, // Roles & responsibilities → IS roles
	{src: "TISAX", srcCode: "TISAX-2.1.2", tgt: "ISO27001", tgtCode: "A.5.5", mtype: "equivalent"}, // Contact with authorities → information security in PM
	{src: "TISAX", srcCode: "TISAX-2.1.3", tgt: "ISO27001", tgtCode: "A.5.8", mtype: "equivalent"}, // IS in project management → IS in PM
	{src: "TISAX", srcCode: "TISAX-2.1.4", tgt: "ISO27001", tgtCode: "A.6.7", mtype: "equivalent"}, // Mobile working → remote working security
	// HR security
	{src: "TISAX", srcCode: "TISAX-3.1.1", tgt: "ISO27001", tgtCode: "A.6.1", mtype: "equivalent"}, // Pre-employment screening → screening
	{src: "TISAX", srcCode: "TISAX-3.1.2", tgt: "ISO27001", tgtCode: "A.6.3", mtype: "equivalent"}, // IS awareness & training → awareness/education
	{src: "TISAX", srcCode: "TISAX-3.1.3", tgt: "ISO27001", tgtCode: "A.6.4", mtype: "equivalent"}, // Disciplinary process → disciplinary process
	{src: "TISAX", srcCode: "TISAX-3.1.4", tgt: "ISO27001", tgtCode: "A.6.5", mtype: "equivalent"}, // Termination/change → responsibilities after termination
	// Asset management
	{src: "TISAX", srcCode: "TISAX-4.1.1", tgt: "ISO27001", tgtCode: "A.5.9", mtype: "equivalent"},  // Asset inventory → inventory of assets
	{src: "TISAX", srcCode: "TISAX-4.1.2", tgt: "ISO27001", tgtCode: "A.5.9", mtype: "partial"},     // Asset ownership → inventory of assets
	{src: "TISAX", srcCode: "TISAX-4.1.3", tgt: "ISO27001", tgtCode: "A.5.12", mtype: "equivalent"}, // Classification → classification of information
	{src: "TISAX", srcCode: "TISAX-4.1.4", tgt: "ISO27001", tgtCode: "A.5.13", mtype: "equivalent"}, // Labelling → labelling of information
	{src: "TISAX", srcCode: "TISAX-4.1.5", tgt: "ISO27001", tgtCode: "A.7.10", mtype: "equivalent"}, // Asset disposal → storage media management
	// Access control
	{src: "TISAX", srcCode: "TISAX-5.1.1", tgt: "ISO27001", tgtCode: "A.5.15", mtype: "equivalent"}, // Access control policy → access control
	{src: "TISAX", srcCode: "TISAX-5.1.2", tgt: "ISO27001", tgtCode: "A.5.18", mtype: "equivalent"}, // User access management → access rights management
	{src: "TISAX", srcCode: "TISAX-5.1.3", tgt: "ISO27001", tgtCode: "A.8.2", mtype: "equivalent"},  // Privileged access → privileged access rights
	{src: "TISAX", srcCode: "TISAX-5.1.4", tgt: "ISO27001", tgtCode: "A.8.5", mtype: "equivalent"},  // MFA → secure authentication
	{src: "TISAX", srcCode: "TISAX-5.1.5", tgt: "ISO27001", tgtCode: "A.8.20", mtype: "equivalent"}, // Network/service access → network security
	// Cryptography
	{src: "TISAX", srcCode: "TISAX-6.1.1", tgt: "ISO27001", tgtCode: "A.8.24", mtype: "equivalent"}, // Cryptography policy → use of cryptography
	{src: "TISAX", srcCode: "TISAX-6.1.2", tgt: "ISO27001", tgtCode: "A.8.24", mtype: "partial"},    // Key management → use of cryptography
	{src: "TISAX", srcCode: "TISAX-6.1.3", tgt: "ISO27001", tgtCode: "A.8.24", mtype: "partial"},    // Encryption of sensitive data → cryptography
	// Physical security
	{src: "TISAX", srcCode: "TISAX-7.1.1", tgt: "ISO27001", tgtCode: "A.7.1", mtype: "equivalent"}, // Physical perimeter → physical security perimeter
	{src: "TISAX", srcCode: "TISAX-7.1.2", tgt: "ISO27001", tgtCode: "A.7.2", mtype: "equivalent"}, // Physical access controls → physical entry controls
	{src: "TISAX", srcCode: "TISAX-7.1.3", tgt: "ISO27001", tgtCode: "A.7.8", mtype: "equivalent"}, // Equipment security → equipment siting/protection
	{src: "TISAX", srcCode: "TISAX-7.1.4", tgt: "ISO27001", tgtCode: "A.7.7", mtype: "equivalent"}, // Clear desk/screen → clear desk/clear screen
	// Operations
	{src: "TISAX", srcCode: "TISAX-8.1.1", tgt: "ISO27001", tgtCode: "A.5.37", mtype: "equivalent"}, // Documented procedures → documented operating procedures
	{src: "TISAX", srcCode: "TISAX-8.1.2", tgt: "ISO27001", tgtCode: "A.8.32", mtype: "equivalent"}, // Change management → change management
	{src: "TISAX", srcCode: "TISAX-8.1.3", tgt: "ISO27001", tgtCode: "A.8.7", mtype: "equivalent"},  // Malware protection → protection against malware
	{src: "TISAX", srcCode: "TISAX-8.1.4", tgt: "ISO27001", tgtCode: "A.8.13", mtype: "equivalent"}, // Backup → information backup
	{src: "TISAX", srcCode: "TISAX-8.1.5", tgt: "ISO27001", tgtCode: "A.8.15", mtype: "equivalent"}, // Logging & monitoring → logging
	{src: "TISAX", srcCode: "TISAX-8.1.6", tgt: "ISO27001", tgtCode: "A.8.8", mtype: "equivalent"},  // Vulnerability management → vuln management
	{src: "TISAX", srcCode: "TISAX-8.1.7", tgt: "ISO27001", tgtCode: "A.8.31", mtype: "equivalent"}, // Dev/test/prod separation → separation of environments
	// Network
	{src: "TISAX", srcCode: "TISAX-9.1.1", tgt: "ISO27001", tgtCode: "A.8.20", mtype: "equivalent"}, // Network security → network security controls
	{src: "TISAX", srcCode: "TISAX-9.1.2", tgt: "ISO27001", tgtCode: "A.8.24", mtype: "partial"},    // Secure transfer → cryptography
	{src: "TISAX", srcCode: "TISAX-9.1.3", tgt: "ISO27001", tgtCode: "A.6.6", mtype: "equivalent"},  // NDAs → confidentiality agreements
	// Secure development
	{src: "TISAX", srcCode: "TISAX-10.1.1", tgt: "ISO27001", tgtCode: "A.8.26", mtype: "equivalent"}, // Security requirements → application security
	{src: "TISAX", srcCode: "TISAX-10.1.2", tgt: "ISO27001", tgtCode: "A.8.25", mtype: "equivalent"}, // Secure dev processes → secure development lifecycle
	{src: "TISAX", srcCode: "TISAX-10.1.3", tgt: "ISO27001", tgtCode: "A.8.29", mtype: "equivalent"}, // Security testing → security testing in dev
	// Supplier security
	{src: "TISAX", srcCode: "TISAX-11.1.1", tgt: "ISO27001", tgtCode: "A.5.19", mtype: "equivalent"}, // Supplier requirements → IS for supplier relationships
	{src: "TISAX", srcCode: "TISAX-11.1.2", tgt: "ISO27001", tgtCode: "A.5.20", mtype: "equivalent"}, // Supplier contracts → IS in supplier agreements
	{src: "TISAX", srcCode: "TISAX-11.1.3", tgt: "ISO27001", tgtCode: "A.5.22", mtype: "equivalent"}, // Supplier monitoring → monitoring/review of supplier services
	// Incident management
	{src: "TISAX", srcCode: "TISAX-12.1.1", tgt: "ISO27001", tgtCode: "A.5.24", mtype: "equivalent"}, // Incident response → IS incident management
	{src: "TISAX", srcCode: "TISAX-12.1.2", tgt: "ISO27001", tgtCode: "A.6.8", mtype: "equivalent"},  // Incident reporting → IS event reporting
	{src: "TISAX", srcCode: "TISAX-12.1.3", tgt: "ISO27001", tgtCode: "A.5.24", mtype: "partial"},    // OEM reporting → incident management
	{src: "TISAX", srcCode: "TISAX-12.1.4", tgt: "ISO27001", tgtCode: "A.5.27", mtype: "equivalent"}, // Post-incident review → learning from incidents
	// BCM
	{src: "TISAX", srcCode: "TISAX-13.1.1", tgt: "ISO27001", tgtCode: "A.5.29", mtype: "equivalent"}, // BCM planning → IS in BCM
	{src: "TISAX", srcCode: "TISAX-13.1.2", tgt: "ISO27001", tgtCode: "A.5.30", mtype: "equivalent"}, // BCM testing → IKT readiness for BCM
	// Compliance
	{src: "TISAX", srcCode: "TISAX-14.1.1", tgt: "ISO27001", tgtCode: "A.5.31", mtype: "equivalent"}, // Legal requirements → legal/regulatory requirements
	{src: "TISAX", srcCode: "TISAX-14.1.2", tgt: "ISO27001", tgtCode: "A.5.35", mtype: "equivalent"}, // Internal audits → IS review by management
	{src: "TISAX", srcCode: "TISAX-14.1.3", tgt: "ISO27001", tgtCode: "A.5.36", mtype: "equivalent"}, // TISAX assessment → compliance with policies
}

// SeedTISAXISO27001Mappings seeds TISAX ↔ ISO 27001:2022 bidirectional mappings.
// Source: VDA ISA 6.0 (2022) cross-reference table — the VDA ISA reference column
// links each TISAX clause to the corresponding ISO 27001:2022 Annex A control.
func (s *Service) SeedTISAXISO27001Mappings(ctx context.Context, orgID string) error {
	return s.seedBidirectionalMappings(ctx, orgID, "TISAX", "ISO27001", tisaxISO27001Mappings)
}

// ── BSI IT-Grundschutz ↔ DORA ────────────────────────────────────────────────
// Source: BaFin DORA-Merkblatt + BSI-Standard 200-2 cross-reference.
// German financial sector regulators accept BSI IT-GS certification as partial
// evidence for DORA compliance (DORA Art. 5-8, Art. 17-23, Art. 28-44).

var bsiDORAMappings = []frameworkPair{
	// DORA Art. 5-8: ICT Risk Management Framework
	{src: "BSI", srcCode: "BSI-ISMS.1", tgt: "DORA", tgtCode: "DORA-1.1", mtype: "equivalent"},    // ISMS governance → ICT risk framework
	{src: "BSI", srcCode: "BSI-ORP.1", tgt: "DORA", tgtCode: "DORA-1.2", mtype: "equivalent"},     // IS organisation → ICT governance
	{src: "BSI", srcCode: "BSI-ORP.4", tgt: "DORA", tgtCode: "DORA-1.2", mtype: "partial"},        // IAM → ICT governance
	{src: "BSI", srcCode: "BSI-DER.1", tgt: "DORA", tgtCode: "DORA-1.3", mtype: "partial"},        // Detection → threat intelligence
	{src: "BSI", srcCode: "BSI-OPS.1.1.5", tgt: "DORA", tgtCode: "DORA-1.3", mtype: "partial"},    // Logging → threat intelligence feed
	{src: "BSI", srcCode: "BSI-OPS.1.1.2", tgt: "DORA", tgtCode: "DORA-1.4", mtype: "partial"},    // Patch management → ICT protection measures
	{src: "BSI", srcCode: "BSI-NET.1.1", tgt: "DORA", tgtCode: "DORA-1.4", mtype: "partial"},      // Network architecture → ICT protection
	{src: "BSI", srcCode: "BSI-CON.1", tgt: "DORA", tgtCode: "DORA-1.4", mtype: "partial"},        // Crypto concept → ICT protection measures
	{src: "BSI", srcCode: "BSI-BCM.1", tgt: "DORA", tgtCode: "DORA-1.4", mtype: "partial"},        // BCM → ICT protection/recovery
	{src: "BSI", srcCode: "BSI-CON.3", tgt: "DORA", tgtCode: "DORA-1.4", mtype: "partial"},        // Backup → ICT protection measures
	{src: "BSI", srcCode: "BSI-OPS.1.1.5", tgt: "DORA", tgtCode: "DORA-1.5", mtype: "equivalent"}, // Logging → ICT detection/monitoring
	{src: "BSI", srcCode: "BSI-DER.1", tgt: "DORA", tgtCode: "DORA-1.5", mtype: "equivalent"},     // Detection → ICT detection
	// DORA Art. 17-23: ICT Incident Management
	{src: "BSI", srcCode: "BSI-DER.2.1", tgt: "DORA", tgtCode: "DORA-2.1", mtype: "equivalent"}, // Incident handling → ICT incident management
	{src: "BSI", srcCode: "BSI-DER.2.2", tgt: "DORA", tgtCode: "DORA-2.2", mtype: "partial"},    // IT forensics → ICT incident classification
	{src: "BSI", srcCode: "BSI-DER.2.1", tgt: "DORA", tgtCode: "DORA-2.3", mtype: "partial"},    // Incident handling → major incident reporting
	// DORA Art. 24-27: DORA Testing
	{src: "BSI", srcCode: "BSI-DER.3.1", tgt: "DORA", tgtCode: "DORA-3.1", mtype: "equivalent"}, // Revisions/audits → DORA ICT testing
	{src: "BSI", srcCode: "BSI-DER.3.2", tgt: "DORA", tgtCode: "DORA-3.2", mtype: "equivalent"}, // Penetration tests → TLPT
	// DORA Art. 28-44: Third-Party ICT Risk
	{src: "BSI", srcCode: "BSI-OPS.2.3", tgt: "DORA", tgtCode: "DORA-4.1", mtype: "equivalent"}, // IT service provider mgmt → TPICT management
	{src: "BSI", srcCode: "BSI-OPS.2.4", tgt: "DORA", tgtCode: "DORA-4.1", mtype: "partial"},    // Remote access to IT systems → TPICT
	{src: "BSI", srcCode: "BSI-OPS.2.3", tgt: "DORA", tgtCode: "DORA-4.2", mtype: "partial"},    // IT service provider → TPICT contractual requirements
	{src: "BSI", srcCode: "BSI-INF.1", tgt: "DORA", tgtCode: "DORA-1.4", mtype: "partial"},      // Building/infrastructure → ICT physical protection
}

// SeedBSIDORAMappings seeds BSI IT-Grundschutz ↔ DORA bidirectional mappings.
// Accepted by BaFin as partial DORA evidence under BSI IT-GS-Zertifizierung.
func (s *Service) SeedBSIDORAMappings(ctx context.Context, orgID string) error {
	return s.seedBidirectionalMappings(ctx, orgID, "BSI", "DORA", bsiDORAMappings)
}

// ── ISO 27017 ↔ BSI IT-Grundschutz ───────────────────────────────────────────
// Source: Structural alignment between ISO 27017 cloud controls and BSI
// OPS.2.x (cloud usage) + CON.1 (crypto) + NET.1.x (network) Bausteine.
// BSI has not published a formal ISO 27017 ↔ GS table; this is a best-effort
// mapping based on the BSI C5:2026 intermediate layer.

var iso27017BSIMappings = []frameworkPair{
	{src: "ISO27017", srcCode: "27017-CLD.6.3.1", tgt: "BSI", tgtCode: "BSI-OPS.2.3", mtype: "equivalent"}, // Shared responsibility → IT-Service-Provider
	{src: "ISO27017", srcCode: "27017-CLD.6.3.1", tgt: "BSI", tgtCode: "BSI-ORP.1", mtype: "partial"},      // Shared roles → IS organisation
	{src: "ISO27017", srcCode: "27017-9.1.2", tgt: "BSI", tgtCode: "BSI-ORP.4", mtype: "equivalent"},       // Access control → IAM
	{src: "ISO27017", srcCode: "27017-9.4.1", tgt: "BSI", tgtCode: "BSI-ORP.4", mtype: "partial"},          // Access restriction → IAM
	{src: "ISO27017", srcCode: "27017-10.1.1", tgt: "BSI", tgtCode: "BSI-CON.1", mtype: "equivalent"},      // Encryption policy → crypto concept
	{src: "ISO27017", srcCode: "27017-10.1.2", tgt: "BSI", tgtCode: "BSI-CON.1", mtype: "partial"},         // Key management → crypto concept
	{src: "ISO27017", srcCode: "27017-12.1.3", tgt: "BSI", tgtCode: "BSI-OPS.1.1.2", mtype: "partial"},     // Capacity management → operations
	{src: "ISO27017", srcCode: "27017-12.4.1", tgt: "BSI", tgtCode: "BSI-OPS.1.1.5", mtype: "equivalent"},  // Event logging → logging/monitoring
	{src: "ISO27017", srcCode: "27017-12.6.1", tgt: "BSI", tgtCode: "BSI-OPS.1.1.2", mtype: "equivalent"},  // Vulnerability mgmt → patch management
	{src: "ISO27017", srcCode: "27017-13.1.3", tgt: "BSI", tgtCode: "BSI-NET.1.1", mtype: "equivalent"},    // Network segregation → network architecture
	{src: "ISO27017", srcCode: "27017-15.1.1", tgt: "BSI", tgtCode: "BSI-OPS.2.3", mtype: "equivalent"},    // Supplier relationships → IT-Service-Provider
	{src: "ISO27017", srcCode: "27017-15.2.1", tgt: "BSI", tgtCode: "BSI-OPS.2.3", mtype: "partial"},       // Supplier monitoring → IT-Service-Provider
	{src: "ISO27017", srcCode: "27017-CLD.9.5.1", tgt: "BSI", tgtCode: "BSI-SYS.1.6", mtype: "equivalent"}, // VM segregation → virtualisation security
	{src: "ISO27017", srcCode: "27017-CLD.9.5.2", tgt: "BSI", tgtCode: "BSI-SYS.1.6", mtype: "partial"},    // VM hardening → virtualisation security
	{src: "ISO27017", srcCode: "27017-CLD.12.4.5", tgt: "BSI", tgtCode: "BSI-DER.1", mtype: "equivalent"},  // Cloud monitoring → detection
	{src: "ISO27017", srcCode: "27017-CLD.12.4.5", tgt: "BSI", tgtCode: "BSI-OPS.1.1.5", mtype: "partial"}, // Cloud monitoring → logging
}

// SeedISO27017BSIMappings seeds ISO 27017 ↔ BSI IT-Grundschutz bidirectional mappings.
func (s *Service) SeedISO27017BSIMappings(ctx context.Context, orgID string) error {
	return s.seedBidirectionalMappings(ctx, orgID, "ISO27017", "BSI", iso27017BSIMappings)
}

// ── ISO 27018 ↔ BSI C5:2026 ───────────────────────────────────────────────────
// Source: BSI C5:2026 Annex C + COM/PI/INQ criteria sections.
// C5 COM (Compliance), PI (if available), and INQ (government access) domains
// address the same PII-processor obligations as ISO 27018 for cloud services.

var iso27018C5Mappings = []frameworkPair{
	// Compliance / legal requirements
	{src: "ISO27018", srcCode: "27018-A.1.1", tgt: "C5", tgtCode: "C5-COM-01", mtype: "equivalent"}, // Purpose limitation → legal requirements identification
	{src: "ISO27018", srcCode: "27018-A.2.1", tgt: "C5", tgtCode: "C5-COM-01", mtype: "partial"},    // Legal basis for transfers → legal requirements
	{src: "ISO27018", srcCode: "27018-A.3.1", tgt: "C5", tgtCode: "C5-COM-01", mtype: "partial"},    // Data subject rights → compliance requirements
	// Sub-processor / service provider transparency
	{src: "ISO27018", srcCode: "27018-A.4.1", tgt: "C5", tgtCode: "C5-SSO-03", mtype: "equivalent"}, // Sub-processor disclosure → data processing by providers
	{src: "ISO27018", srcCode: "27018-A.4.1", tgt: "C5", tgtCode: "C5-SSO-04", mtype: "partial"},    // Sub-processor disclosure → provider directory
	// Government access / law enforcement
	{src: "ISO27018", srcCode: "27018-A.4.2", tgt: "C5", tgtCode: "C5-INQ-01", mtype: "equivalent"}, // Government access → legal review of requests
	{src: "ISO27018", srcCode: "27018-A.4.2", tgt: "C5", tgtCode: "C5-INQ-03", mtype: "equivalent"}, // Government access → limit data access
	// Data location
	{src: "ISO27018", srcCode: "27018-A.4.3", tgt: "C5", tgtCode: "C5-PSS-12", mtype: "equivalent"}, // Processing locations → data region documentation
	{src: "ISO27018", srcCode: "27018-A.3.2", tgt: "C5", tgtCode: "C5-PSS-12", mtype: "partial"},    // PII deletion/return → data lifecycle
	// Cryptography
	{src: "ISO27018", srcCode: "27018-A.5.1", tgt: "C5", tgtCode: "C5-CRY-05", mtype: "equivalent"}, // Encryption at rest → at-rest encryption
	{src: "ISO27018", srcCode: "27018-A.5.2", tgt: "C5", tgtCode: "C5-CRY-04", mtype: "equivalent"}, // Encryption in transit → transport encryption
	// Customer data access control
	{src: "ISO27018", srcCode: "27018-A.5.3", tgt: "C5", tgtCode: "C5-IAM-07", mtype: "equivalent"}, // Access control for PII → CSP staff access to customer data
	// Incident / breach notification
	{src: "ISO27018", srcCode: "27018-A.6.1", tgt: "C5", tgtCode: "C5-OPS-24", mtype: "equivalent"}, // Breach notification → customer notification on incidents
	// Logging & audit
	{src: "ISO27018", srcCode: "27018-A.8.1", tgt: "C5", tgtCode: "C5-OPS-10", mtype: "equivalent"}, // PII access logging → logging policy
	{src: "ISO27018", srcCode: "27018-A.8.2", tgt: "C5", tgtCode: "C5-COM-03", mtype: "equivalent"}, // Compliance audit → internal ISMS audits
}

// SeedISO27018C5Mappings seeds ISO 27018 ↔ BSI C5:2026 bidirectional mappings.
func (s *Service) SeedISO27018C5Mappings(ctx context.Context, orgID string) error {
	return s.seedBidirectionalMappings(ctx, orgID, "ISO27018", "C5", iso27018C5Mappings)
}

// ── ISO 27017 ↔ DORA ─────────────────────────────────────────────────────────
// Source: DORA Art. 28 TPICT (Third-Party ICT Risk) explicitly references cloud
// security standards including ISO 27017. The EBA/ESMA/EIOPA joint RTS on TPICT
// cites ISO 27017 cloud controls as accepted evidence for Art. 28 compliance.

var iso27017DORAMappings = []frameworkPair{
	{src: "ISO27017", srcCode: "27017-15.1.1", tgt: "DORA", tgtCode: "DORA-4.1", mtype: "equivalent"},  // Supplier relationships → TPICT management
	{src: "ISO27017", srcCode: "27017-15.2.1", tgt: "DORA", tgtCode: "DORA-4.2", mtype: "equivalent"},  // Supplier monitoring → TPICT contractual requirements
	{src: "ISO27017", srcCode: "27017-CLD.6.3.1", tgt: "DORA", tgtCode: "DORA-4.1", mtype: "partial"},  // Shared responsibility → TPICT management
	{src: "ISO27017", srcCode: "27017-12.4.1", tgt: "DORA", tgtCode: "DORA-1.5", mtype: "equivalent"},  // Event logging → ICT detection/monitoring
	{src: "ISO27017", srcCode: "27017-12.6.1", tgt: "DORA", tgtCode: "DORA-1.4", mtype: "partial"},     // Vulnerability mgmt → ICT protection measures
	{src: "ISO27017", srcCode: "27017-13.1.3", tgt: "DORA", tgtCode: "DORA-1.4", mtype: "partial"},     // Network segregation → ICT protection
	{src: "ISO27017", srcCode: "27017-10.1.1", tgt: "DORA", tgtCode: "DORA-1.4", mtype: "partial"},     // Encryption → ICT protection measures
	{src: "ISO27017", srcCode: "27017-CLD.9.5.1", tgt: "DORA", tgtCode: "DORA-1.4", mtype: "partial"},  // VM segregation → ICT protection
	{src: "ISO27017", srcCode: "27017-12.1.3", tgt: "DORA", tgtCode: "DORA-1.4", mtype: "partial"},     // Capacity management → ICT protection
	{src: "ISO27017", srcCode: "27017-CLD.12.4.5", tgt: "DORA", tgtCode: "DORA-1.5", mtype: "partial"}, // Cloud monitoring → ICT detection
}

// SeedISO27017DORAMappings seeds ISO 27017 ↔ DORA bidirectional mappings.
func (s *Service) SeedISO27017DORAMappings(ctx context.Context, orgID string) error {
	return s.seedBidirectionalMappings(ctx, orgID, "ISO27017", "DORA", iso27017DORAMappings)
}

// ── ISO 27017 ↔ KRITIS-DachG ─────────────────────────────────────────────────
// Source: KRITIS-DachG §12 (Sicherheitsmaßnahmen) + §14 (Outsourcing/Cloud).
// §14 Abs. 3 KRITIS-DachG explicitly allows cloud security standards including
// ISO 27017 as evidence for KRITIS cloud-outsourcing requirements.

var iso27017KRITISMappings = []frameworkPair{
	{src: "ISO27017", srcCode: "27017-CLD.6.3.1", tgt: "KRITIS", tgtCode: "KRITIS-DG.3", mtype: "equivalent"}, // Shared responsibility → ISMS
	{src: "ISO27017", srcCode: "27017-15.1.1", tgt: "KRITIS", tgtCode: "KRITIS-DG.4", mtype: "equivalent"},    // Supplier relationships → supply chain
	{src: "ISO27017", srcCode: "27017-12.4.1", tgt: "KRITIS", tgtCode: "KRITIS-DG.9", mtype: "equivalent"},    // Event logging → monitoring
	{src: "ISO27017", srcCode: "27017-CLD.12.4.5", tgt: "KRITIS", tgtCode: "KRITIS-DG.9", mtype: "partial"},   // Cloud monitoring → monitoring
	{src: "ISO27017", srcCode: "27017-12.6.1", tgt: "KRITIS", tgtCode: "KRITIS-DG.13", mtype: "equivalent"},   // Vulnerability mgmt → vulnerability management
	{src: "ISO27017", srcCode: "27017-13.1.3", tgt: "KRITIS", tgtCode: "KRITIS-DG.12", mtype: "equivalent"},   // Network segregation → network security
	{src: "ISO27017", srcCode: "27017-CLD.9.5.1", tgt: "KRITIS", tgtCode: "KRITIS-DG.12", mtype: "partial"},   // VM segregation → network/environment security
	{src: "ISO27017", srcCode: "27017-10.1.1", tgt: "KRITIS", tgtCode: "KRITIS-DG.11", mtype: "equivalent"},   // Encryption → cryptography
	{src: "ISO27017", srcCode: "27017-10.1.2", tgt: "KRITIS", tgtCode: "KRITIS-DG.11", mtype: "partial"},      // Key management → cryptography
}

// SeedISO27017KRITISMappings seeds ISO 27017 ↔ KRITIS-DachG bidirectional mappings.
func (s *Service) SeedISO27017KRITISMappings(ctx context.Context, orgID string) error {
	return s.seedBidirectionalMappings(ctx, orgID, "ISO27017", "KRITIS", iso27017KRITISMappings)
}

// ── ISO 42001 ↔ NIS2 ─────────────────────────────────────────────────────────
// Source: ENISA Technical Implementation Guidance (TIG) V1.0, Section 4.3
// "AI systems and NIS2 obligations". AI systems are in-scope for NIS2 when
// operated by essential/important entities; ISO 42001 is the primary AI-MS standard.

var iso42001NIS2Mappings = []frameworkPair{
	{src: "ISO42001", srcCode: "42001-4.1", tgt: "NIS2", tgtCode: "NIS2-A.1", mtype: "partial"},    // Context understanding → governance
	{src: "ISO42001", srcCode: "42001-5.1", tgt: "NIS2", tgtCode: "NIS2-A.1", mtype: "partial"},    // Leadership for AI → governance
	{src: "ISO42001", srcCode: "42001-6.1", tgt: "NIS2", tgtCode: "NIS2-A.3", mtype: "equivalent"}, // AI risk assessment → risk management
	{src: "ISO42001", srcCode: "42001-6.2", tgt: "NIS2", tgtCode: "NIS2-A.3", mtype: "partial"},    // AI objectives → risk treatment
	{src: "ISO42001", srcCode: "42001-7.1", tgt: "NIS2", tgtCode: "NIS2-G.2", mtype: "equivalent"}, // Competence for AI → awareness/training
	{src: "ISO42001", srcCode: "42001-8.1", tgt: "NIS2", tgtCode: "NIS2-E.3", mtype: "partial"},    // AI lifecycle management → vulnerability management
	{src: "ISO42001", srcCode: "42001-8.2", tgt: "NIS2", tgtCode: "NIS2-A.3", mtype: "equivalent"}, // AI impact assessment → risk management
	{src: "ISO42001", srcCode: "42001-8.5", tgt: "NIS2", tgtCode: "NIS2-E.3", mtype: "equivalent"}, // AI monitoring → continuous monitoring
	{src: "ISO42001", srcCode: "42001-9.1", tgt: "NIS2", tgtCode: "NIS2-F.3", mtype: "equivalent"}, // Internal audits → effectiveness testing
	{src: "ISO42001", srcCode: "42001-A3.1", tgt: "NIS2", tgtCode: "NIS2-A.3", mtype: "partial"},   // Risk assessment procedure → risk management
	{src: "ISO42001", srcCode: "42001-A4.1", tgt: "NIS2", tgtCode: "NIS2-E.1", mtype: "partial"},   // AI design/spec → network/IS security
	{src: "ISO42001", srcCode: "42001-A5.1", tgt: "NIS2", tgtCode: "NIS2-A.3", mtype: "partial"},   // Training data requirements → risk assessment
}

// SeedISO42001NIS2Mappings seeds ISO 42001 ↔ NIS2 bidirectional mappings.
func (s *Service) SeedISO42001NIS2Mappings(ctx context.Context, orgID string) error {
	return s.seedBidirectionalMappings(ctx, orgID, "ISO42001", "NIS2", iso42001NIS2Mappings)
}

// ── CRA ↔ BSI C5:2026 ────────────────────────────────────────────────────────
// Source: BSI C5:2026 PSS (Produktsicherheit für Kunden) domain + DEV/OPS.
// The CRA Annex I product security requirements map directly to C5 criteria
// for cloud service providers offering software products (PSS, DEV, OPS domains).

var craC5Mappings = []frameworkPair{
	// Product security design → C5 DEV / PSS
	{src: "CRA", srcCode: "CRA-1.1", tgt: "C5", tgtCode: "C5-DEV-01", mtype: "equivalent"}, // Security by design → secure dev requirements
	{src: "CRA", srcCode: "CRA-1.1", tgt: "C5", tgtCode: "C5-PSS-01", mtype: "partial"},    // Security by design → customer security recommendations
	{src: "CRA", srcCode: "CRA-1.2", tgt: "C5", tgtCode: "C5-OIS-08", mtype: "equivalent"}, // Risk assessment → risk analysis
	// PSIRT / vulnerability management
	{src: "CRA", srcCode: "CRA-1.3", tgt: "C5", tgtCode: "C5-PSS-02", mtype: "equivalent"}, // PSIRT/VDP → vulnerability identification
	{src: "CRA", srcCode: "CRA-1.3", tgt: "C5", tgtCode: "C5-OPS-18", mtype: "partial"},    // PSIRT → vulnerability management policy
	// SBOM
	{src: "CRA", srcCode: "CRA-1.4", tgt: "C5", tgtCode: "C5-OPS-29", mtype: "equivalent"}, // SBOM → third-party components monitoring
	// Secure defaults / hardening
	{src: "CRA", srcCode: "CRA-1.5", tgt: "C5", tgtCode: "C5-OPS-26", mtype: "equivalent"}, // Secure by default → system hardening
	{src: "CRA", srcCode: "CRA-1.5", tgt: "C5", tgtCode: "C5-PSS-11", mtype: "partial"},    // Secure defaults → VM/container image hardening
	// Patch management
	{src: "CRA", srcCode: "CRA-1.6", tgt: "C5", tgtCode: "C5-OPS-27", mtype: "equivalent"}, // Security updates → patch policy
	{src: "CRA", srcCode: "CRA-1.6", tgt: "C5", tgtCode: "C5-OPS-28", mtype: "partial"},    // Security updates → patch implementation
	// Known vulnerability protection
	{src: "CRA", srcCode: "CRA-1.7", tgt: "C5", tgtCode: "C5-OPS-25", mtype: "equivalent"}, // Known vuln protection → vulnerability scans
	// Authentication
	{src: "CRA", srcCode: "CRA-1.8", tgt: "C5", tgtCode: "C5-PSS-05", mtype: "equivalent"}, // Authentication controls → customer auth mechanisms
	{src: "CRA", srcCode: "CRA-1.8", tgt: "C5", tgtCode: "C5-PSS-09", mtype: "partial"},    // Authentication → authorisation mechanisms
	// Cryptography
	{src: "CRA", srcCode: "CRA-1.9", tgt: "C5", tgtCode: "C5-CRY-04", mtype: "equivalent"}, // Encryption → transport encryption
	{src: "CRA", srcCode: "CRA-1.9", tgt: "C5", tgtCode: "C5-CRY-05", mtype: "partial"},    // Encryption → at-rest
	// Logging
	{src: "CRA", srcCode: "CRA-1.10", tgt: "C5", tgtCode: "C5-OPS-10", mtype: "equivalent"}, // Logging/auditability → logging policy
	// Incident reporting (CRA Art. 14 → ENISA)
	{src: "CRA", srcCode: "CRA-2.1", tgt: "C5", tgtCode: "C5-OPS-19", mtype: "partial"}, // ENISA reporting → incident management policy
	// Vulnerability disclosure
	{src: "CRA", srcCode: "CRA-2.2", tgt: "C5", tgtCode: "C5-PSS-03", mtype: "equivalent"}, // VDP → customer vuln notification
	// Secure development lifecycle
	{src: "CRA", srcCode: "CRA-3.1", tgt: "C5", tgtCode: "C5-DEV-01", mtype: "equivalent"}, // Secure SDLC → secure dev requirements
	// Customer information
	{src: "CRA", srcCode: "CRA-4.1", tgt: "C5", tgtCode: "C5-PSS-01", mtype: "equivalent"}, // User information guide → customer security recommendations
}

// SeedCRAC5Mappings seeds CRA ↔ BSI C5:2026 bidirectional mappings.
func (s *Service) SeedCRAC5Mappings(ctx context.Context, orgID string) error {
	return s.seedBidirectionalMappings(ctx, orgID, "CRA", "C5", craC5Mappings)
}

// ── CRA ↔ BSI IT-Grundschutz ─────────────────────────────────────────────────
// Source: BSI TR-03183 "Cyber Resilience Requirements for Manufacturers and
// Products" (CRA guidance for German market), Module H cross-reference table.
// BSI TR-03183 explicitly links CRA Annex I requirements to IT-Grundschutz Bausteine.

var craBSIMappings = []frameworkPair{
	// CRA-1.x: Product security requirements
	{src: "CRA", srcCode: "CRA-1.1", tgt: "BSI", tgtCode: "BSI-CON.8", mtype: "equivalent"},      // Security by design → software development
	{src: "CRA", srcCode: "CRA-1.2", tgt: "BSI", tgtCode: "BSI-ISMS.1", mtype: "equivalent"},     // Risk assessment → ISMS governance
	{src: "CRA", srcCode: "CRA-1.3", tgt: "BSI", tgtCode: "BSI-DER.2.1", mtype: "partial"},       // PSIRT → incident handling
	{src: "CRA", srcCode: "CRA-1.4", tgt: "BSI", tgtCode: "BSI-OPS.1.1.2", mtype: "partial"},     // SBOM → patch/asset management
	{src: "CRA", srcCode: "CRA-1.5", tgt: "BSI", tgtCode: "BSI-SYS.1.1", mtype: "partial"},       // Secure defaults → server hardening
	{src: "CRA", srcCode: "CRA-1.5", tgt: "BSI", tgtCode: "BSI-OPS.1.1.6", mtype: "partial"},     // Secure defaults → software release & integrity
	{src: "CRA", srcCode: "CRA-1.6", tgt: "BSI", tgtCode: "BSI-OPS.1.1.2", mtype: "equivalent"},  // Security updates → patch management
	{src: "CRA", srcCode: "CRA-1.7", tgt: "BSI", tgtCode: "BSI-OPS.1.1.2", mtype: "partial"},     // Known vuln protection → patch management
	{src: "CRA", srcCode: "CRA-1.8", tgt: "BSI", tgtCode: "BSI-ORP.4", mtype: "equivalent"},      // Authentication controls → IAM
	{src: "CRA", srcCode: "CRA-1.9", tgt: "BSI", tgtCode: "BSI-CON.1", mtype: "equivalent"},      // Encryption → crypto concept
	{src: "CRA", srcCode: "CRA-1.10", tgt: "BSI", tgtCode: "BSI-OPS.1.1.5", mtype: "equivalent"}, // Logging → logging/monitoring
	// CRA-2.x: Vulnerability handling
	{src: "CRA", srcCode: "CRA-2.1", tgt: "BSI", tgtCode: "BSI-DER.2.1", mtype: "partial"}, // ENISA reporting → incident handling
	{src: "CRA", srcCode: "CRA-2.2", tgt: "BSI", tgtCode: "BSI-DER.2.1", mtype: "partial"}, // VDP → incident response process
	{src: "CRA", srcCode: "CRA-2.3", tgt: "BSI", tgtCode: "BSI-DER.2.2", mtype: "partial"}, // CVD → IT forensics / remediation
	// CRA-3.x: Technical measures
	{src: "CRA", srcCode: "CRA-3.1", tgt: "BSI", tgtCode: "BSI-CON.8", mtype: "equivalent"},   // Secure SDLC → software development
	{src: "CRA", srcCode: "CRA-3.2", tgt: "BSI", tgtCode: "BSI-DER.3.2", mtype: "equivalent"}, // Penetration testing → penetration tests
	{src: "CRA", srcCode: "CRA-3.3", tgt: "BSI", tgtCode: "BSI-OPS.1.1.2", mtype: "partial"},  // Config management → operations/hardening
	{src: "CRA", srcCode: "CRA-3.4", tgt: "BSI", tgtCode: "BSI-SYS.1.1", mtype: "partial"},    // Exploit mitigation → server security
}

// SeedCRABSIMappings seeds CRA ↔ BSI IT-Grundschutz bidirectional mappings.
// Based on BSI TR-03183 Module H cross-reference table.
func (s *Service) SeedCRABSIMappings(ctx context.Context, orgID string) error {
	return s.seedBidirectionalMappings(ctx, orgID, "CRA", "BSI", craBSIMappings)
}

// ── TISAX ↔ BSI C5:2026 ──────────────────────────────────────────────────────
// Source: VDA ISA 6.0 Clause 7.x "Cloud and external services" controls map
// to BSI C5:2026 criteria for cloud customers. ENX Association TISAX ↔ C5
// alignment white paper (2024) used for cloud-specific mappings.

var tisaxC5Mappings = []frameworkPair{
	// Policy & ISMS
	{src: "TISAX", srcCode: "TISAX-1.1.1", tgt: "C5", tgtCode: "C5-OIS-01", mtype: "equivalent"}, // IS policy → ISMS
	{src: "TISAX", srcCode: "TISAX-1.1.1", tgt: "C5", tgtCode: "C5-SP-01", mtype: "partial"},     // IS policy → policy documentation
	{src: "TISAX", srcCode: "TISAX-2.1.1", tgt: "C5", tgtCode: "C5-OIS-04", mtype: "equivalent"}, // Roles & responsibilities → separation of duties
	// HR security
	{src: "TISAX", srcCode: "TISAX-3.1.2", tgt: "C5", tgtCode: "C5-HR-03", mtype: "equivalent"}, // IS awareness training → employee training
	// Asset management
	{src: "TISAX", srcCode: "TISAX-4.1.1", tgt: "C5", tgtCode: "C5-AM-02", mtype: "equivalent"}, // Asset inventory → asset inventory
	{src: "TISAX", srcCode: "TISAX-4.1.3", tgt: "C5", tgtCode: "C5-AM-09", mtype: "equivalent"}, // Asset classification → asset classification
	// Access control / IAM
	{src: "TISAX", srcCode: "TISAX-5.1.1", tgt: "C5", tgtCode: "C5-IAM-01", mtype: "equivalent"}, // Access control policy → IAM policy
	{src: "TISAX", srcCode: "TISAX-5.1.2", tgt: "C5", tgtCode: "C5-IAM-02", mtype: "equivalent"}, // User access management → rights assignment
	{src: "TISAX", srcCode: "TISAX-5.1.3", tgt: "C5", tgtCode: "C5-IAM-06", mtype: "equivalent"}, // Privileged access → privileged access rights
	{src: "TISAX", srcCode: "TISAX-5.1.4", tgt: "C5", tgtCode: "C5-IAM-08", mtype: "equivalent"}, // MFA → authentication mechanisms
	// Cryptography
	{src: "TISAX", srcCode: "TISAX-6.1.1", tgt: "C5", tgtCode: "C5-CRY-01", mtype: "equivalent"}, // Crypto policy → crypto policy
	{src: "TISAX", srcCode: "TISAX-6.1.2", tgt: "C5", tgtCode: "C5-CRY-07", mtype: "equivalent"}, // Key management → key rotation
	{src: "TISAX", srcCode: "TISAX-6.1.2", tgt: "C5", tgtCode: "C5-CRY-10", mtype: "partial"},    // Key management → key storage
	// Physical security
	{src: "TISAX", srcCode: "TISAX-7.1.1", tgt: "C5", tgtCode: "C5-PS-01", mtype: "equivalent"}, // Physical perimeter → physical security
	{src: "TISAX", srcCode: "TISAX-7.1.2", tgt: "C5", tgtCode: "C5-PS-02", mtype: "equivalent"}, // Physical access → physical access controls
	// Operations
	{src: "TISAX", srcCode: "TISAX-8.1.3", tgt: "C5", tgtCode: "C5-OPS-04", mtype: "equivalent"}, // Malware protection → malware policy
	{src: "TISAX", srcCode: "TISAX-8.1.4", tgt: "C5", tgtCode: "C5-OPS-06", mtype: "equivalent"}, // Backup → backup policy
	{src: "TISAX", srcCode: "TISAX-8.1.5", tgt: "C5", tgtCode: "C5-OPS-10", mtype: "equivalent"}, // Logging & monitoring → logging policy
	{src: "TISAX", srcCode: "TISAX-8.1.6", tgt: "C5", tgtCode: "C5-OPS-25", mtype: "equivalent"}, // Vulnerability management → vuln scans
	{src: "TISAX", srcCode: "TISAX-8.1.6", tgt: "C5", tgtCode: "C5-OPS-18", mtype: "partial"},    // Vulnerability management → vuln policy
	// Network security
	{src: "TISAX", srcCode: "TISAX-9.1.1", tgt: "C5", tgtCode: "C5-COS-01", mtype: "equivalent"}, // Network security → network controls
	// Supplier management
	{src: "TISAX", srcCode: "TISAX-11.1.1", tgt: "C5", tgtCode: "C5-SSO-01", mtype: "equivalent"}, // Supplier requirements → provider policy
	{src: "TISAX", srcCode: "TISAX-11.1.1", tgt: "C5", tgtCode: "C5-SSO-02", mtype: "partial"},    // Supplier security → provider risk assessment
	// Incident management
	{src: "TISAX", srcCode: "TISAX-12.1.1", tgt: "C5", tgtCode: "C5-SIM-01", mtype: "equivalent"}, // Incident response → incident management
	{src: "TISAX", srcCode: "TISAX-12.1.1", tgt: "C5", tgtCode: "C5-OPS-19", mtype: "partial"},    // Incident response → incident management policy
	// BCM
	{src: "TISAX", srcCode: "TISAX-13.1.1", tgt: "C5", tgtCode: "C5-BCM-01", mtype: "equivalent"}, // BCM planning → BCM
}

// SeedTISAXC5Mappings seeds TISAX ↔ BSI C5:2026 bidirectional mappings.
func (s *Service) SeedTISAXC5Mappings(ctx context.Context, orgID string) error {
	return s.seedBidirectionalMappings(ctx, orgID, "TISAX", "C5", tisaxC5Mappings)
}

// ── EU AI Act ↔ NIS2 ─────────────────────────────────────────────────────────
// Source: ENISA TIG V1.0 Section 4.3 + ENISA "AI Cybersecurity Challenges"
// report. AI systems operated by NIS2 essential/important entities must meet
// both NIS2 Art. 21 cybersecurity requirements AND EU AI Act obligations.
// The overlap is explicit in Recital 79 NIS2 and EU AI Act Art. 9/17.

var euAIActNIS2Mappings = []frameworkPair{
	{src: "AIACT", srcCode: "AIACT-1.1", tgt: "NIS2", tgtCode: "NIS2-A.3", mtype: "equivalent"}, // AI risk management → NIS2 risk management
	{src: "AIACT", srcCode: "AIACT-1.2", tgt: "NIS2", tgtCode: "NIS2-A.3", mtype: "partial"},    // Risk assessment process → NIS2 risk analysis
	{src: "AIACT", srcCode: "AIACT-2.1", tgt: "NIS2", tgtCode: "NIS2-A.3", mtype: "partial"},    // Training data requirements → risk management
	{src: "AIACT", srcCode: "AIACT-3.1", tgt: "NIS2", tgtCode: "NIS2-F.2", mtype: "partial"},    // Technical documentation → policies/procedures
	{src: "AIACT", srcCode: "AIACT-4.1", tgt: "NIS2", tgtCode: "NIS2-E.3", mtype: "equivalent"}, // AI logging → monitoring/detection
	{src: "AIACT", srcCode: "AIACT-5.1", tgt: "NIS2", tgtCode: "NIS2-A.1", mtype: "partial"},    // Transparency → governance
	{src: "AIACT", srcCode: "AIACT-6.1", tgt: "NIS2", tgtCode: "NIS2-A.3", mtype: "partial"},    // Human oversight → risk treatment
	{src: "AIACT", srcCode: "AIACT-9.1", tgt: "NIS2", tgtCode: "NIS2-A.1", mtype: "partial"},    // QMS → governance/policy
	{src: "AIACT", srcCode: "AIACT-9.2", tgt: "NIS2", tgtCode: "NIS2-F.3", mtype: "equivalent"}, // Conformity assessment → effectiveness testing
	{src: "AIACT", srcCode: "AIACT-12.1", tgt: "NIS2", tgtCode: "NIS2-G.2", mtype: "partial"},   // Deployer transparency → training/awareness
	{src: "AIACT", srcCode: "AIACT-13.1", tgt: "NIS2", tgtCode: "NIS2-G.2", mtype: "partial"},   // Deepfake disclosure → awareness/training
}

// SeedEUAIActNIS2Mappings seeds EU AI Act ↔ NIS2 bidirectional mappings.
func (s *Service) SeedEUAIActNIS2Mappings(ctx context.Context, orgID string) error {
	return s.seedBidirectionalMappings(ctx, orgID, "AIACT", "NIS2", euAIActNIS2Mappings)
}

// ── Helper: generic bidirectional seeder ─────────────────────────────────────

type frameworkPair struct {
	src     string
	srcCode string
	tgt     string
	tgtCode string
	mtype   string
}

// seedBidirectionalMappings seeds pairs into the global ck_framework_control_mappings table
// (text-based, system-wide reference — same as SeedGlobalControlMapping).
// Returns early if either framework is not active for this org.
func (s *Service) seedBidirectionalMappings(ctx context.Context, orgID, fwNameA, fwNameB string, pairs []frameworkPair) error {
	fwA, err := s.repo.FindFrameworkByName(ctx, orgID, fwNameA)
	if err != nil {
		return fmt.Errorf("find framework %s: %w", fwNameA, err)
	}
	if fwA == nil {
		return nil
	}
	fwB, err := s.repo.FindFrameworkByName(ctx, orgID, fwNameB)
	if err != nil {
		return fmt.Errorf("find framework %s: %w", fwNameB, err)
	}
	if fwB == nil {
		return nil
	}
	return s.seedPairs(ctx, orgID, pairs)
}

// seedPairs writes a slice of frameworkPair entries into the global reference table.
func (s *Service) seedPairs(_ context.Context, _ string, pairs []frameworkPair) error {
	ctx := context.Background()
	for _, p := range pairs {
		if err := s.repo.SeedGlobalControlMapping(ctx, p.src, p.srcCode, p.tgt, p.tgtCode, p.mtype); err != nil {
			log.Warn().Err(err).Str("src", p.srcCode).Str("tgt", p.tgtCode).Msg("seed mapping failed")
		}
	}
	return nil
}

// ── Prerequisite Chains (ck_control_prerequisites) ───────────────────────────

// PrerequisiteEntry is a row in ck_control_prerequisites.
type PrerequisiteEntry struct {
	ControlFW      string
	ControlCode    string
	PrereqFW       string
	PrereqCode     string
	DependencyType string
	Rationale      string
	Source         string
}

// ── ISO 27001:2022 ↔ BSI IT-Grundschutz ──────────────────────────────────────

var iso27001BSIMappings = []frameworkPair{
	// ── A.5 Organisatorische Maßnahmen ──────────────────────────────────────
	{src: "ISO27001", srcCode: "A.5.1", tgt: "BSI", tgtCode: "BSI-ISMS.1.A1", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.5.1", tgt: "BSI", tgtCode: "BSI-ISMS.1.A5", mtype: "partial"},
	{src: "ISO27001", srcCode: "A.5.2", tgt: "BSI", tgtCode: "BSI-ISMS.1.A2", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.5.2", tgt: "BSI", tgtCode: "BSI-ORP.1.A1", mtype: "partial"},
	{src: "ISO27001", srcCode: "A.5.4", tgt: "BSI", tgtCode: "BSI-ISMS.1.A4", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.5.5", tgt: "BSI", tgtCode: "BSI-ISMS.1.A6", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.5.9", tgt: "BSI", tgtCode: "BSI-ORP.5.A1", mtype: "partial"},
	{src: "ISO27001", srcCode: "A.5.12", tgt: "BSI", tgtCode: "BSI-CON.9.A1", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.5.14", tgt: "BSI", tgtCode: "BSI-CON.9.A1", mtype: "partial"},
	{src: "ISO27001", srcCode: "A.5.16", tgt: "BSI", tgtCode: "BSI-ORP.4.A1", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.5.17", tgt: "BSI", tgtCode: "BSI-ORP.4.A1", mtype: "partial"},
	{src: "ISO27001", srcCode: "A.5.18", tgt: "BSI", tgtCode: "BSI-ORP.4.A1", mtype: "partial"},
	{src: "ISO27001", srcCode: "A.5.19", tgt: "BSI", tgtCode: "BSI-OPS.2.2.A1", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.5.20", tgt: "BSI", tgtCode: "BSI-OPS.2.2.A1", mtype: "partial"},
	{src: "ISO27001", srcCode: "A.5.23", tgt: "BSI", tgtCode: "BSI-OPS.2.4.A1", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.5.24", tgt: "BSI", tgtCode: "BSI-DER.2.1.A1", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.5.25", tgt: "BSI", tgtCode: "BSI-DER.2.1.A1", mtype: "partial"},
	{src: "ISO27001", srcCode: "A.5.26", tgt: "BSI", tgtCode: "BSI-DER.2.1.A1", mtype: "partial"},
	{src: "ISO27001", srcCode: "A.5.27", tgt: "BSI", tgtCode: "BSI-DER.2.2.A1", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.5.28", tgt: "BSI", tgtCode: "BSI-DER.1.A2", mtype: "partial"},
	{src: "ISO27001", srcCode: "A.5.29", tgt: "BSI", tgtCode: "BSI-BCM.1.A1", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.5.30", tgt: "BSI", tgtCode: "BSI-BCM.1.A2", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.5.31", tgt: "BSI", tgtCode: "BSI-ORP.5.A1", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.5.35", tgt: "BSI", tgtCode: "BSI-ISMS.1.A9", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.5.36", tgt: "BSI", tgtCode: "BSI-ISMS.1.A10", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.5.37", tgt: "BSI", tgtCode: "BSI-OPS.1.1.2.A1", mtype: "partial"},

	// ── A.6 Personenbezogene Maßnahmen ──────────────────────────────────────
	{src: "ISO27001", srcCode: "A.6.1", tgt: "BSI", tgtCode: "BSI-ORP.2.A1", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.6.2", tgt: "BSI", tgtCode: "BSI-ORP.2.A2", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.6.3", tgt: "BSI", tgtCode: "BSI-ORP.3.A1", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.6.3", tgt: "BSI", tgtCode: "BSI-ORP.2.A3", mtype: "partial"},
	{src: "ISO27001", srcCode: "A.6.4", tgt: "BSI", tgtCode: "BSI-ORP.2.A2", mtype: "partial"},
	{src: "ISO27001", srcCode: "A.6.5", tgt: "BSI", tgtCode: "BSI-ORP.2.A4", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.6.7", tgt: "BSI", tgtCode: "BSI-ORP.2.A4", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.6.8", tgt: "BSI", tgtCode: "BSI-DER.1.A1", mtype: "partial"},

	// ── A.7 Physische Maßnahmen ──────────────────────────────────────────────
	{src: "ISO27001", srcCode: "A.7.1", tgt: "BSI", tgtCode: "BSI-INF.1.A1", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.7.2", tgt: "BSI", tgtCode: "BSI-INF.2.A1", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.7.3", tgt: "BSI", tgtCode: "BSI-INF.7.A1", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.7.4", tgt: "BSI", tgtCode: "BSI-INF.1.A1", mtype: "partial"},
	{src: "ISO27001", srcCode: "A.7.5", tgt: "BSI", tgtCode: "BSI-INF.5.A1", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.7.6", tgt: "BSI", tgtCode: "BSI-INF.5.A1", mtype: "partial"},
	{src: "ISO27001", srcCode: "A.7.7", tgt: "BSI", tgtCode: "BSI-INF.8.A1", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.7.8", tgt: "BSI", tgtCode: "BSI-INF.10.A1", mtype: "partial"},
	{src: "ISO27001", srcCode: "A.7.9", tgt: "BSI", tgtCode: "BSI-SYS.3.1.A1", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.7.10", tgt: "BSI", tgtCode: "BSI-SYS.4.5.A1", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.7.11", tgt: "BSI", tgtCode: "BSI-INF.3.A1", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.7.12", tgt: "BSI", tgtCode: "BSI-INF.3.A1", mtype: "partial"},
	{src: "ISO27001", srcCode: "A.7.13", tgt: "BSI", tgtCode: "BSI-SYS.4.1.A1", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.7.14", tgt: "BSI", tgtCode: "BSI-CON.6.A1", mtype: "equivalent"},

	// ── A.8 Technologische Maßnahmen ────────────────────────────────────────
	{src: "ISO27001", srcCode: "A.8.1", tgt: "BSI", tgtCode: "BSI-SYS.2.1.A1", mtype: "partial"},
	{src: "ISO27001", srcCode: "A.8.1", tgt: "BSI", tgtCode: "BSI-SYS.3.1.A1", mtype: "partial"},
	{src: "ISO27001", srcCode: "A.8.2", tgt: "BSI", tgtCode: "BSI-ORP.4.A1", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.8.3", tgt: "BSI", tgtCode: "BSI-ORP.4.A1", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.8.4", tgt: "BSI", tgtCode: "BSI-ORP.4.A1", mtype: "partial"},
	{src: "ISO27001", srcCode: "A.8.5", tgt: "BSI", tgtCode: "BSI-ORP.4.A1", mtype: "partial"},
	{src: "ISO27001", srcCode: "A.8.6", tgt: "BSI", tgtCode: "BSI-SYS.1.1.A1", mtype: "partial"},
	{src: "ISO27001", srcCode: "A.8.7", tgt: "BSI", tgtCode: "BSI-OPS.1.1.4.A1", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.8.7", tgt: "BSI", tgtCode: "BSI-OPS.1.1.4.A2", mtype: "partial"},
	{src: "ISO27001", srcCode: "A.8.8", tgt: "BSI", tgtCode: "BSI-OPS.1.1.3.A1", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.8.9", tgt: "BSI", tgtCode: "BSI-OPS.1.1.2.A1", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.8.10", tgt: "BSI", tgtCode: "BSI-CON.6.A1", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.8.11", tgt: "BSI", tgtCode: "BSI-CON.7.A1", mtype: "partial"},
	{src: "ISO27001", srcCode: "A.8.12", tgt: "BSI", tgtCode: "BSI-DER.1.A1", mtype: "partial"},
	{src: "ISO27001", srcCode: "A.8.13", tgt: "BSI", tgtCode: "BSI-CON.3.A1", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.8.14", tgt: "BSI", tgtCode: "BSI-BCM.1.A1", mtype: "partial"},
	{src: "ISO27001", srcCode: "A.8.15", tgt: "BSI", tgtCode: "BSI-OPS.1.1.5.A1", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.8.15", tgt: "BSI", tgtCode: "BSI-DER.1.A2", mtype: "partial"},
	{src: "ISO27001", srcCode: "A.8.16", tgt: "BSI", tgtCode: "BSI-DER.1.A1", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.8.17", tgt: "BSI", tgtCode: "BSI-OPS.1.1.2.A1", mtype: "partial"},
	{src: "ISO27001", srcCode: "A.8.18", tgt: "BSI", tgtCode: "BSI-OPS.1.1.2.A1", mtype: "partial"},
	{src: "ISO27001", srcCode: "A.8.19", tgt: "BSI", tgtCode: "BSI-OPS.1.1.6.A1", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.8.20", tgt: "BSI", tgtCode: "BSI-NET.3.2.A1", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.8.20", tgt: "BSI", tgtCode: "BSI-NET.1.1.A1", mtype: "partial"},
	{src: "ISO27001", srcCode: "A.8.21", tgt: "BSI", tgtCode: "BSI-NET.1.2.A1", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.8.22", tgt: "BSI", tgtCode: "BSI-NET.1.1.A1", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.8.23", tgt: "BSI", tgtCode: "BSI-APP.1.2.A1", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.8.24", tgt: "BSI", tgtCode: "BSI-CON.1.A1", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.8.25", tgt: "BSI", tgtCode: "BSI-CON.8.A1", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.8.25", tgt: "BSI", tgtCode: "BSI-CON.5.A1", mtype: "partial"},
	{src: "ISO27001", srcCode: "A.8.26", tgt: "BSI", tgtCode: "BSI-APP.3.1.A1", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.8.27", tgt: "BSI", tgtCode: "BSI-SYS.1.1.A1", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.8.28", tgt: "BSI", tgtCode: "BSI-CON.8.A1", mtype: "partial"},
	{src: "ISO27001", srcCode: "A.8.29", tgt: "BSI", tgtCode: "BSI-CON.8.A1", mtype: "partial"},
	{src: "ISO27001", srcCode: "A.8.30", tgt: "BSI", tgtCode: "BSI-OPS.1.1.2.A1", mtype: "partial"},
	{src: "ISO27001", srcCode: "A.8.31", tgt: "BSI", tgtCode: "BSI-OPS.1.1.3.A1", mtype: "partial"},
	{src: "ISO27001", srcCode: "A.8.32", tgt: "BSI", tgtCode: "BSI-OPS.1.1.3.A1", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.8.33", tgt: "BSI", tgtCode: "BSI-OPS.1.1.2.A1", mtype: "partial"},
	{src: "ISO27001", srcCode: "A.8.34", tgt: "BSI", tgtCode: "BSI-DER.3.2.A1", mtype: "equivalent"},

	// ── Weitere spezifische Paare ──────────────────────────────────────────
	{src: "ISO27001", srcCode: "A.5.10", tgt: "BSI", tgtCode: "BSI-CON.4.A1", mtype: "partial"},
	{src: "ISO27001", srcCode: "A.5.33", tgt: "BSI", tgtCode: "BSI-CON.6.A1", mtype: "equivalent"},
	{src: "ISO27001", srcCode: "A.6.6", tgt: "BSI", tgtCode: "BSI-CON.9.A1", mtype: "partial"},
	{src: "ISO27001", srcCode: "A.8.1", tgt: "BSI", tgtCode: "BSI-SYS.3.2.A1", mtype: "partial"},
	{src: "ISO27001", srcCode: "A.5.6", tgt: "BSI", tgtCode: "BSI-ORP.5.A1", mtype: "partial"},
	{src: "ISO27001", srcCode: "A.8.16", tgt: "BSI", tgtCode: "BSI-OPS.1.1.7.A1", mtype: "partial"},
}

// SeedISO27001BSIMappings seeds ISO 27001:2022 ↔ BSI IT-Grundschutz bidirectional mappings.
func (s *Service) SeedISO27001BSIMappings(ctx context.Context, orgID string) error {
	return s.seedBidirectionalMappings(ctx, orgID, "ISO27001", "BSI", iso27001BSIMappings)
}

// ── NIS2 ↔ BSI Anreicherung ──────────────────────────────────────────────────

// nis2BSIExtendedMappings extends the existing nis2BSIMappings (7 base pairs) with full A–J coverage.
var nis2BSIExtendedMappings = []frameworkPair{
	// ── A — Governance / Sicherheitskonzept ──────────────────────────────────
	{src: "NIS2", srcCode: "NIS2-A.2", tgt: "BSI", tgtCode: "BSI-ISMS.1.A1", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-A.3", tgt: "BSI", tgtCode: "BSI-ISMS.1.A2", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-A.4", tgt: "BSI", tgtCode: "BSI-ISMS.1.A4", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-A.5", tgt: "BSI", tgtCode: "BSI-ISMS.1.A9", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-A.7", tgt: "BSI", tgtCode: "BSI-ISMS.1.A6", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-A.8", tgt: "BSI", tgtCode: "BSI-ORP.5.A1", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-A.9", tgt: "BSI", tgtCode: "BSI-ISMS.1.A10", mtype: "equivalent"},

	// ── B — Incident Management ───────────────────────────────────────────────
	{src: "NIS2", srcCode: "NIS2-B.2", tgt: "BSI", tgtCode: "BSI-DER.2.2.A1", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-B.3", tgt: "BSI", tgtCode: "BSI-DER.1.A1", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-B.4", tgt: "BSI", tgtCode: "BSI-DER.1.A2", mtype: "partial"},
	{src: "NIS2", srcCode: "NIS2-B.5", tgt: "BSI", tgtCode: "BSI-DER.4.A1", mtype: "equivalent"},

	// ── C — Business Continuity ───────────────────────────────────────────────
	{src: "NIS2", srcCode: "NIS2-C.1", tgt: "BSI", tgtCode: "BSI-BCM.1.A1", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-C.2", tgt: "BSI", tgtCode: "BSI-BCM.1.A2", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-C.3", tgt: "BSI", tgtCode: "BSI-BCM.2.A1", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-C.5", tgt: "BSI", tgtCode: "BSI-DER.4.A1", mtype: "partial"},

	// ── D — Krisenmanagement ──────────────────────────────────────────────────
	{src: "NIS2", srcCode: "NIS2-D.1", tgt: "BSI", tgtCode: "BSI-DER.4.A1", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-D.2", tgt: "BSI", tgtCode: "BSI-DER.2.1.A1", mtype: "partial"},

	// ── E — Netz- und Systemsicherheit ───────────────────────────────────────
	{src: "NIS2", srcCode: "NIS2-E.1", tgt: "BSI", tgtCode: "BSI-NET.1.1.A1", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-E.2", tgt: "BSI", tgtCode: "BSI-NET.3.2.A1", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-E.5", tgt: "BSI", tgtCode: "BSI-OPS.1.1.3.A1", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-E.6", tgt: "BSI", tgtCode: "BSI-OPS.1.1.4.A1", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-E.7", tgt: "BSI", tgtCode: "BSI-SYS.1.1.A1", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-E.9", tgt: "BSI", tgtCode: "BSI-NET.1.2.A1", mtype: "equivalent"},

	// ── F — Zugriffskontrolle ─────────────────────────────────────────────────
	{src: "NIS2", srcCode: "NIS2-F.1", tgt: "BSI", tgtCode: "BSI-ORP.4.A1", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-F.2", tgt: "BSI", tgtCode: "BSI-ORP.4.A1", mtype: "partial"},
	{src: "NIS2", srcCode: "NIS2-F.3", tgt: "BSI", tgtCode: "BSI-APP.4.4.A1", mtype: "partial"},

	// ── G — Kryptographie ────────────────────────────────────────────────────
	{src: "NIS2", srcCode: "NIS2-G.1", tgt: "BSI", tgtCode: "BSI-CON.1.A1", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-G.2", tgt: "BSI", tgtCode: "BSI-CON.1.A1", mtype: "partial"},

	// ── H — Physische Sicherheit ──────────────────────────────────────────────
	{src: "NIS2", srcCode: "NIS2-H.1", tgt: "BSI", tgtCode: "BSI-INF.2.A1", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-H.2", tgt: "BSI", tgtCode: "BSI-INF.1.A1", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-H.3", tgt: "BSI", tgtCode: "BSI-INF.5.A1", mtype: "partial"},

	// ── I — Lieferkettensicherheit ────────────────────────────────────────────
	{src: "NIS2", srcCode: "NIS2-I.1", tgt: "BSI", tgtCode: "BSI-OPS.2.2.A1", mtype: "equivalent"},
	{src: "NIS2", srcCode: "NIS2-I.2", tgt: "BSI", tgtCode: "BSI-OPS.2.4.A1", mtype: "partial"},

	// ── J — Meldepflichten / Offenlegung ──────────────────────────────────────
	{src: "NIS2", srcCode: "NIS2-J.1", tgt: "BSI", tgtCode: "BSI-DER.2.1.A1", mtype: "partial"},
	{src: "NIS2", srcCode: "NIS2-J.2", tgt: "BSI", tgtCode: "BSI-DER.1.A1", mtype: "partial"},
}

// ── DSGVO-TOM ↔ NIS2 ─────────────────────────────────────────────────────────

var dsgvoTOMNIS2Mappings = []frameworkPair{
	{src: "DSGVO-TOM", srcCode: "TOM-1", tgt: "NIS2", tgtCode: "NIS2-H.1", mtype: "equivalent"},
	{src: "DSGVO-TOM", srcCode: "TOM-1", tgt: "NIS2", tgtCode: "NIS2-H.2", mtype: "partial"},
	{src: "DSGVO-TOM", srcCode: "TOM-2", tgt: "NIS2", tgtCode: "NIS2-F.1", mtype: "equivalent"},
	{src: "DSGVO-TOM", srcCode: "TOM-3", tgt: "NIS2", tgtCode: "NIS2-F.1", mtype: "equivalent"},
	{src: "DSGVO-TOM", srcCode: "TOM-3", tgt: "NIS2", tgtCode: "NIS2-F.2", mtype: "partial"},
	{src: "DSGVO-TOM", srcCode: "TOM-4", tgt: "NIS2", tgtCode: "NIS2-G.1", mtype: "equivalent"},
	{src: "DSGVO-TOM", srcCode: "TOM-5", tgt: "NIS2", tgtCode: "NIS2-E.3", mtype: "equivalent"},
	{src: "DSGVO-TOM", srcCode: "TOM-5", tgt: "NIS2", tgtCode: "NIS2-B.3", mtype: "partial"},
	{src: "DSGVO-TOM", srcCode: "TOM-6", tgt: "NIS2", tgtCode: "NIS2-I.1", mtype: "equivalent"},
	{src: "DSGVO-TOM", srcCode: "TOM-7", tgt: "NIS2", tgtCode: "NIS2-C.4", mtype: "equivalent"},
	{src: "DSGVO-TOM", srcCode: "TOM-7", tgt: "NIS2", tgtCode: "NIS2-C.1", mtype: "partial"},
	{src: "DSGVO-TOM", srcCode: "TOM-10", tgt: "NIS2", tgtCode: "NIS2-G.1", mtype: "equivalent"},
	{src: "DSGVO-TOM", srcCode: "TOM-12", tgt: "NIS2", tgtCode: "NIS2-C.3", mtype: "equivalent"},
	{src: "DSGVO-TOM", srcCode: "TOM-13", tgt: "NIS2", tgtCode: "NIS2-A.5", mtype: "equivalent"},
}

// SeedDSGVOTOMNIS2Mappings seeds DSGVO-TOM ↔ NIS2 bidirectional mappings.
func (s *Service) SeedDSGVOTOMNIS2Mappings(ctx context.Context, orgID string) error {
	return s.seedBidirectionalMappings(ctx, orgID, "DSGVO-TOM", "NIS2", dsgvoTOMNIS2Mappings)
}

// ── DSGVO-TOM ↔ BSI ──────────────────────────────────────────────────────────

var dsgvoTOMBSIMappings = []frameworkPair{
	{src: "DSGVO-TOM", srcCode: "TOM-1", tgt: "BSI", tgtCode: "BSI-INF.2.A1", mtype: "equivalent"},
	{src: "DSGVO-TOM", srcCode: "TOM-1", tgt: "BSI", tgtCode: "BSI-INF.1.A1", mtype: "partial"},
	{src: "DSGVO-TOM", srcCode: "TOM-2", tgt: "BSI", tgtCode: "BSI-ORP.4.A1", mtype: "equivalent"},
	{src: "DSGVO-TOM", srcCode: "TOM-3", tgt: "BSI", tgtCode: "BSI-ORP.4.A1", mtype: "equivalent"},
	{src: "DSGVO-TOM", srcCode: "TOM-4", tgt: "BSI", tgtCode: "BSI-CON.1.A1", mtype: "equivalent"},
	{src: "DSGVO-TOM", srcCode: "TOM-4", tgt: "BSI", tgtCode: "BSI-NET.4.1.A1", mtype: "partial"},
	{src: "DSGVO-TOM", srcCode: "TOM-5", tgt: "BSI", tgtCode: "BSI-OPS.1.1.5.A1", mtype: "equivalent"},
	{src: "DSGVO-TOM", srcCode: "TOM-6", tgt: "BSI", tgtCode: "BSI-OPS.2.2.A1", mtype: "equivalent"},
	{src: "DSGVO-TOM", srcCode: "TOM-7", tgt: "BSI", tgtCode: "BSI-CON.3.A1", mtype: "equivalent"},
	{src: "DSGVO-TOM", srcCode: "TOM-7", tgt: "BSI", tgtCode: "BSI-BCM.1.A1", mtype: "partial"},
	{src: "DSGVO-TOM", srcCode: "TOM-10", tgt: "BSI", tgtCode: "BSI-CON.1.A1", mtype: "equivalent"},
	{src: "DSGVO-TOM", srcCode: "TOM-12", tgt: "BSI", tgtCode: "BSI-BCM.2.A1", mtype: "equivalent"},
	{src: "DSGVO-TOM", srcCode: "TOM-13", tgt: "BSI", tgtCode: "BSI-ISMS.1.A9", mtype: "equivalent"},
}

// SeedDSGVOTOMBSIMappings seeds DSGVO-TOM ↔ BSI IT-Grundschutz bidirectional mappings.
func (s *Service) SeedDSGVOTOMBSIMappings(ctx context.Context, orgID string) error {
	return s.seedBidirectionalMappings(ctx, orgID, "DSGVO-TOM", "BSI", dsgvoTOMBSIMappings)
}

// ── CIS Controls v8 ↔ ISO 27001:2022 ─────────────────────────────────────────

var cisISO27001Mappings = []frameworkPair{
	{src: "CIS", srcCode: "CIS-1.1", tgt: "ISO27001", tgtCode: "A.5.9", mtype: "equivalent"},
	{src: "CIS", srcCode: "CIS-2.1", tgt: "ISO27001", tgtCode: "A.5.9", mtype: "partial"},
	{src: "CIS", srcCode: "CIS-3.3", tgt: "ISO27001", tgtCode: "A.8.24", mtype: "equivalent"},
	{src: "CIS", srcCode: "CIS-3.3", tgt: "ISO27001", tgtCode: "A.5.14", mtype: "partial"},
	{src: "CIS", srcCode: "CIS-5.1", tgt: "ISO27001", tgtCode: "A.5.16", mtype: "equivalent"},
	{src: "CIS", srcCode: "CIS-5.3", tgt: "ISO27001", tgtCode: "A.8.2", mtype: "equivalent"},
	{src: "CIS", srcCode: "CIS-6.1", tgt: "ISO27001", tgtCode: "A.5.18", mtype: "equivalent"},
	{src: "CIS", srcCode: "CIS-7.1", tgt: "ISO27001", tgtCode: "A.8.8", mtype: "equivalent"},
	{src: "CIS", srcCode: "CIS-7.2", tgt: "ISO27001", tgtCode: "A.8.8", mtype: "partial"},
	{src: "CIS", srcCode: "CIS-8.1", tgt: "ISO27001", tgtCode: "A.8.15", mtype: "equivalent"},
	{src: "CIS", srcCode: "CIS-8.2", tgt: "ISO27001", tgtCode: "A.8.16", mtype: "equivalent"},
	{src: "CIS", srcCode: "CIS-10.1", tgt: "ISO27001", tgtCode: "A.8.7", mtype: "equivalent"},
	{src: "CIS", srcCode: "CIS-11.1", tgt: "ISO27001", tgtCode: "A.8.13", mtype: "equivalent"},
	{src: "CIS", srcCode: "CIS-11.1", tgt: "ISO27001", tgtCode: "A.5.30", mtype: "partial"},
	{src: "CIS", srcCode: "CIS-12.1", tgt: "ISO27001", tgtCode: "A.8.20", mtype: "equivalent"},
	{src: "CIS", srcCode: "CIS-12.1", tgt: "ISO27001", tgtCode: "A.8.22", mtype: "partial"},
	{src: "CIS", srcCode: "CIS-13.1", tgt: "ISO27001", tgtCode: "A.8.16", mtype: "partial"},
	{src: "CIS", srcCode: "CIS-14.1", tgt: "ISO27001", tgtCode: "A.6.3", mtype: "equivalent"},
	{src: "CIS", srcCode: "CIS-16.1", tgt: "ISO27001", tgtCode: "A.8.25", mtype: "equivalent"},
	{src: "CIS", srcCode: "CIS-16.1", tgt: "ISO27001", tgtCode: "A.8.26", mtype: "partial"},
	{src: "CIS", srcCode: "CIS-17.1", tgt: "ISO27001", tgtCode: "A.5.24", mtype: "equivalent"},
	{src: "CIS", srcCode: "CIS-17.1", tgt: "ISO27001", tgtCode: "A.5.26", mtype: "partial"},
	{src: "CIS", srcCode: "CIS-18.1", tgt: "ISO27001", tgtCode: "A.8.34", mtype: "equivalent"},
	{src: "CIS", srcCode: "CIS-5.1", tgt: "ISO27001", tgtCode: "A.5.17", mtype: "partial"},
	{src: "CIS", srcCode: "CIS-8.1", tgt: "ISO27001", tgtCode: "A.5.28", mtype: "partial"},
}

// SeedCISISO27001Mappings seeds CIS Controls v8 ↔ ISO 27001:2022 bidirectional mappings.
func (s *Service) SeedCISISO27001Mappings(ctx context.Context, orgID string) error {
	return s.seedBidirectionalMappings(ctx, orgID, "CIS", "ISO27001", cisISO27001Mappings)
}

// ── CIS Controls v8 ↔ BSI IT-Grundschutz ────────────────────────────────────

var cisBSIMappings = []frameworkPair{
	{src: "CIS", srcCode: "CIS-1.1", tgt: "BSI", tgtCode: "BSI-ORP.5.A1", mtype: "partial"},
	{src: "CIS", srcCode: "CIS-3.3", tgt: "BSI", tgtCode: "BSI-CON.1.A1", mtype: "equivalent"},
	{src: "CIS", srcCode: "CIS-5.1", tgt: "BSI", tgtCode: "BSI-ORP.4.A1", mtype: "equivalent"},
	{src: "CIS", srcCode: "CIS-5.3", tgt: "BSI", tgtCode: "BSI-ORP.4.A1", mtype: "equivalent"},
	{src: "CIS", srcCode: "CIS-6.1", tgt: "BSI", tgtCode: "BSI-ORP.4.A1", mtype: "partial"},
	{src: "CIS", srcCode: "CIS-7.1", tgt: "BSI", tgtCode: "BSI-OPS.1.1.3.A1", mtype: "equivalent"},
	{src: "CIS", srcCode: "CIS-7.2", tgt: "BSI", tgtCode: "BSI-OPS.1.1.3.A1", mtype: "partial"},
	{src: "CIS", srcCode: "CIS-8.1", tgt: "BSI", tgtCode: "BSI-OPS.1.1.5.A1", mtype: "equivalent"},
	{src: "CIS", srcCode: "CIS-8.2", tgt: "BSI", tgtCode: "BSI-DER.1.A2", mtype: "equivalent"},
	{src: "CIS", srcCode: "CIS-10.1", tgt: "BSI", tgtCode: "BSI-OPS.1.1.4.A1", mtype: "equivalent"},
	{src: "CIS", srcCode: "CIS-10.1", tgt: "BSI", tgtCode: "BSI-OPS.1.1.4.A2", mtype: "partial"},
	{src: "CIS", srcCode: "CIS-11.1", tgt: "BSI", tgtCode: "BSI-CON.3.A1", mtype: "equivalent"},
	{src: "CIS", srcCode: "CIS-11.1", tgt: "BSI", tgtCode: "BSI-BCM.1.A2", mtype: "partial"},
	{src: "CIS", srcCode: "CIS-12.1", tgt: "BSI", tgtCode: "BSI-NET.1.1.A1", mtype: "equivalent"},
	{src: "CIS", srcCode: "CIS-12.1", tgt: "BSI", tgtCode: "BSI-NET.3.2.A1", mtype: "partial"},
	{src: "CIS", srcCode: "CIS-13.1", tgt: "BSI", tgtCode: "BSI-DER.1.A1", mtype: "equivalent"},
	{src: "CIS", srcCode: "CIS-14.1", tgt: "BSI", tgtCode: "BSI-ORP.3.A1", mtype: "equivalent"},
	{src: "CIS", srcCode: "CIS-16.1", tgt: "BSI", tgtCode: "BSI-CON.8.A1", mtype: "equivalent"},
	{src: "CIS", srcCode: "CIS-17.1", tgt: "BSI", tgtCode: "BSI-DER.2.1.A1", mtype: "equivalent"},
	{src: "CIS", srcCode: "CIS-18.1", tgt: "BSI", tgtCode: "BSI-DER.3.2.A1", mtype: "equivalent"},
}

// SeedCISBSIMappings seeds CIS Controls v8 ↔ BSI IT-Grundschutz bidirectional mappings.
func (s *Service) SeedCISBSIMappings(ctx context.Context, orgID string) error {
	return s.seedBidirectionalMappings(ctx, orgID, "CIS", "BSI", cisBSIMappings)
}

// ── TISAX ↔ BSI IT-Grundschutz ──────────────────────────────────────────────

// tisaxBSIMappings maps actual TISAX control IDs (as defined in tisaxControls()) to BSI controls.
var tisaxBSIMappings = []frameworkPair{
	// ── IS-Management ──────────────────────────────────────────────────────────
	{src: "TISAX", srcCode: "TISAX-1.1.1", tgt: "BSI", tgtCode: "BSI-ISMS.1.A1", mtype: "equivalent"},
	{src: "TISAX", srcCode: "TISAX-2.1.1", tgt: "BSI", tgtCode: "BSI-ISMS.1.A2", mtype: "equivalent"},
	{src: "TISAX", srcCode: "TISAX-1.1.3", tgt: "BSI", tgtCode: "BSI-ISMS.1.A6", mtype: "equivalent"},
	// ── Zugriffskontrolle ──────────────────────────────────────────────────────
	{src: "TISAX", srcCode: "TISAX-5.1.1", tgt: "BSI", tgtCode: "BSI-ORP.4.A1", mtype: "equivalent"},
	{src: "TISAX", srcCode: "TISAX-5.1.2", tgt: "BSI", tgtCode: "BSI-ORP.4.A1", mtype: "partial"},
	// ── Patch & Schwachstellen ─────────────────────────────────────────────────
	{src: "TISAX", srcCode: "TISAX-8.1.6", tgt: "BSI", tgtCode: "BSI-OPS.1.1.3.A1", mtype: "equivalent"},
	// ── Schutz vor Schadsoftware ───────────────────────────────────────────────
	{src: "TISAX", srcCode: "TISAX-8.1.3", tgt: "BSI", tgtCode: "BSI-OPS.1.1.4.A1", mtype: "equivalent"},
	// ── Netzwerksicherheit ────────────────────────────────────────────────────
	{src: "TISAX", srcCode: "TISAX-9.1.1", tgt: "BSI", tgtCode: "BSI-NET.3.2.A1", mtype: "equivalent"},
	// ── Kryptographie ────────────────────────────────────────────────────────
	{src: "TISAX", srcCode: "TISAX-6.1.1", tgt: "BSI", tgtCode: "BSI-CON.1.A1", mtype: "equivalent"},
	// ── Datensicherung ───────────────────────────────────────────────────────
	{src: "TISAX", srcCode: "TISAX-8.1.4", tgt: "BSI", tgtCode: "BSI-CON.3.A1", mtype: "equivalent"},
	// ── Incident Response ─────────────────────────────────────────────────────
	{src: "TISAX", srcCode: "TISAX-12.1.1", tgt: "BSI", tgtCode: "BSI-DER.2.1.A1", mtype: "equivalent"},
	// ── BCM ──────────────────────────────────────────────────────────────────
	{src: "TISAX", srcCode: "TISAX-13.1.1", tgt: "BSI", tgtCode: "BSI-BCM.1.A1", mtype: "equivalent"},
	// ── Personal ─────────────────────────────────────────────────────────────
	{src: "TISAX", srcCode: "TISAX-3.1.1", tgt: "BSI", tgtCode: "BSI-ORP.2.A1", mtype: "equivalent"},
	{src: "TISAX", srcCode: "TISAX-3.1.2", tgt: "BSI", tgtCode: "BSI-ORP.3.A1", mtype: "equivalent"},
	// ── Physischer Schutz ────────────────────────────────────────────────────
	{src: "TISAX", srcCode: "TISAX-7.1.1", tgt: "BSI", tgtCode: "BSI-INF.2.A1", mtype: "equivalent"},
	{src: "TISAX", srcCode: "TISAX-7.1.2", tgt: "BSI", tgtCode: "BSI-INF.1.A1", mtype: "partial"},
	{src: "TISAX", srcCode: "TISAX-7.1.4", tgt: "BSI", tgtCode: "BSI-INF.8.A1", mtype: "partial"},
	// ── Prototypenschutz ─────────────────────────────────────────────────────
	{src: "TISAX", srcCode: "TISAX-4.1.5", tgt: "BSI", tgtCode: "BSI-CON.7.A1", mtype: "equivalent"},
	{src: "TISAX", srcCode: "TISAX-4.1.5", tgt: "BSI", tgtCode: "BSI-CON.6.A1", mtype: "partial"},
	// ── Connected Vehicles ────────────────────────────────────────────────────
	{src: "TISAX", srcCode: "TISAX-16.1.3", tgt: "BSI", tgtCode: "BSI-OPS.1.1.3.A1", mtype: "partial"},
	{src: "TISAX", srcCode: "TISAX-16.1.2", tgt: "BSI", tgtCode: "BSI-NET.1.1.A1", mtype: "partial"},
	{src: "TISAX", srcCode: "TISAX-10.1.2", tgt: "BSI", tgtCode: "BSI-CON.8.A1", mtype: "partial"},
}

// SeedTISAXBSIMappings seeds TISAX ↔ BSI IT-Grundschutz bidirectional mappings.
func (s *Service) SeedTISAXBSIMappings(ctx context.Context, orgID string) error {
	return s.seedBidirectionalMappings(ctx, orgID, "TISAX", "BSI", tisaxBSIMappings)
}

// ── TISAX ↔ DSGVO-TOM ────────────────────────────────────────────────────────

// tisaxDSGVOTOMMappings maps TISAX controls to DSGVO Art. 32 TOMs.
// Uses actual TISAX control IDs from tisaxControls().
var tisaxDSGVOTOMMappings = []frameworkPair{
	{src: "TISAX", srcCode: "TISAX-5.1.1", tgt: "DSGVO-TOM", tgtCode: "TOM-2", mtype: "equivalent"},   // Zugangskontrollrichtlinie → Zugangskontrolle
	{src: "TISAX", srcCode: "TISAX-5.1.3", tgt: "DSGVO-TOM", tgtCode: "TOM-3", mtype: "equivalent"},   // Privilegierte Zugriffsrechte → Zugriffskontrolle
	{src: "TISAX", srcCode: "TISAX-7.1.2", tgt: "DSGVO-TOM", tgtCode: "TOM-1", mtype: "partial"},      // Zugangskontrollen für Sicherheitsbereiche → Zutrittskontrolle
	{src: "TISAX", srcCode: "TISAX-9.1.2", tgt: "DSGVO-TOM", tgtCode: "TOM-4", mtype: "equivalent"},   // Sichere Datenübertragung → Weitergabekontrolle
	{src: "TISAX", srcCode: "TISAX-6.1.3", tgt: "DSGVO-TOM", tgtCode: "TOM-10", mtype: "equivalent"},  // Verschlüsselung sensitiver Daten → Verschlüsselung
	{src: "TISAX", srcCode: "TISAX-8.1.5", tgt: "DSGVO-TOM", tgtCode: "TOM-5", mtype: "equivalent"},   // Protokollierung und Überwachung → Eingabekontrolle
	{src: "TISAX", srcCode: "TISAX-10.1.2", tgt: "DSGVO-TOM", tgtCode: "TOM-11", mtype: "partial"},    // Sichere Entwicklungsprozesse → Integrität
	{src: "TISAX", srcCode: "TISAX-11.1.2", tgt: "DSGVO-TOM", tgtCode: "TOM-6", mtype: "equivalent"},  // Sicherheitsanforderungen in Lieferantenverträgen → Auftragskontrolle
	{src: "TISAX", srcCode: "TISAX-8.1.7", tgt: "DSGVO-TOM", tgtCode: "TOM-8", mtype: "equivalent"},   // Trennung Entwicklung/Test/Betrieb → Trennungsgebot
	{src: "TISAX", srcCode: "TISAX-8.1.4", tgt: "DSGVO-TOM", tgtCode: "TOM-7", mtype: "equivalent"},   // Datensicherung (Backup) → Verfügbarkeitskontrolle
	{src: "TISAX", srcCode: "TISAX-13.1.2", tgt: "DSGVO-TOM", tgtCode: "TOM-12", mtype: "equivalent"}, // BCM-Tests → Wiederherstellung
	{src: "TISAX", srcCode: "TISAX-14.1.2", tgt: "DSGVO-TOM", tgtCode: "TOM-13", mtype: "partial"},    // Interne IS-Audits → Überprüfungsverfahren
	{src: "TISAX", srcCode: "TISAX-4.1.3", tgt: "DSGVO-TOM", tgtCode: "TOM-9", mtype: "partial"},      // Klassifizierung von Informationen → Pseudonymisierung
}

// SeedTISAXDSGVOTOMMappings seeds TISAX ↔ DSGVO-TOM bidirectional mappings.
func (s *Service) SeedTISAXDSGVOTOMMappings(ctx context.Context, orgID string) error {
	return s.seedBidirectionalMappings(ctx, orgID, "TISAX", "DSGVO-TOM", tisaxDSGVOTOMMappings)
}

// SeedPrerequisiteChains idempotently inserts prerequisite relationships into
// ck_control_prerequisites (ON CONFLICT DO NOTHING).
func (s *Service) SeedPrerequisiteChains(ctx context.Context) error {
	chains := buildPrerequisiteChains()
	inserted, failed := 0, 0
	for _, ch := range chains {
		if err := s.repo.UpsertControlPrerequisite(ctx, ch); err != nil {
			failed++
			log.Warn().Err(err).
				Str("control", ch.ControlCode).
				Str("prereq", ch.PrereqCode).
				Msg("seed prerequisite failed")
		} else {
			inserted++
		}
	}
	log.Info().Int("inserted", inserted).Int("failed", failed).Msg("SeedPrerequisiteChains: done")
	return nil
}

func buildPrerequisiteChains() []PrerequisiteEntry {
	return []PrerequisiteEntry{
		// ── ISO 27001:2022 Annex A control hierarchy ──────────────────────
		{
			ControlFW: "ISO27001", ControlCode: "A.5.16",
			PrereqFW: "ISO27001", PrereqCode: "A.5.15",
			DependencyType: "required",
			Rationale:      "Identity management requires an access control policy",
			Source:         "ISO 27001:2022 §A.5",
		},
		{
			ControlFW: "ISO27001", ControlCode: "A.8.3",
			PrereqFW: "ISO27001", PrereqCode: "A.5.16",
			DependencyType: "recommended",
			Rationale:      "Information access restriction requires a defined user provisioning process",
			Source:         "ISO 27001:2022 §A.8",
		},
		{
			ControlFW: "ISO27001", ControlCode: "A.8.15",
			PrereqFW: "ISO27001", PrereqCode: "A.5.37",
			DependencyType: "recommended",
			Rationale:      "Logging requires documented operating procedures",
			Source:         "ISO 27001:2022 §A.8",
		},
		{
			ControlFW: "ISO27001", ControlCode: "A.8.8",
			PrereqFW: "ISO27001", PrereqCode: "A.8.15",
			DependencyType: "recommended",
			Rationale:      "Vulnerability detection requires logging infrastructure",
			Source:         "ISO 27001:2022 §A.8",
		},
		{
			ControlFW: "ISO27001", ControlCode: "A.8.25",
			PrereqFW: "ISO27001", PrereqCode: "A.8.26",
			DependencyType: "required",
			Rationale:      "Secure development life cycle requires defined security requirements",
			Source:         "ISO 27001:2022 §A.8",
		},
		{
			ControlFW: "ISO27001", ControlCode: "A.5.22",
			PrereqFW: "ISO27001", PrereqCode: "A.5.19",
			DependencyType: "required",
			Rationale:      "Supplier service monitoring requires established supplier relationships",
			Source:         "ISO 27001:2022 §A.5",
		},
		{
			ControlFW: "ISO27001", ControlCode: "A.6.8",
			PrereqFW: "ISO27001", PrereqCode: "A.5.24",
			DependencyType: "required",
			Rationale:      "IS event reporting requires incident management planning",
			Source:         "ISO 27001:2022 §A.5/A.6",
		},
		{
			ControlFW: "ISO27001", ControlCode: "A.5.30",
			PrereqFW: "ISO27001", PrereqCode: "A.5.29",
			DependencyType: "required",
			Rationale:      "ICT readiness for BCM requires IS-during-disruption planning",
			Source:         "ISO 27001:2022 §A.5",
		},
		{
			ControlFW: "ISO27001", ControlCode: "A.5.36",
			PrereqFW: "ISO27001", PrereqCode: "A.5.31",
			DependencyType: "required",
			Rationale:      "Compliance with IS policies requires identification of legal requirements",
			Source:         "ISO 27001:2022 §A.5",
		},

		// ── NIS2 Art.21 interne Reihenfolge ──────────────────────────────
		{
			ControlFW: "NIS2", ControlCode: "NIS2-A.3",
			PrereqFW: "NIS2", PrereqCode: "NIS2-A.2",
			DependencyType: "required",
			Rationale:      "Risikoanalyse setzt Risikomanagement-Framework voraus",
			Source:         "ENISA NIS2 Implementation Guide §3.2",
		},
		{
			ControlFW: "NIS2", ControlCode: "NIS2-D.1",
			PrereqFW: "NIS2", PrereqCode: "NIS2-A.3",
			DependencyType: "required",
			Rationale:      "Lieferkettensicherheit erfordert vorherige Risikoanalyse",
			Source:         "NIS2 Erwägungsgrund 85",
		},
		{
			ControlFW: "NIS2", ControlCode: "NIS2-E.3",
			PrereqFW: "NIS2", PrereqCode: "NIS2-A.2",
			DependencyType: "required",
			Rationale:      "Schwachstellenmanagement-Priorisierung folgt Risiko-Rahmen",
			Source:         "ENISA NIS2 Implementation Guide §3.2",
		},
		{
			ControlFW: "NIS2", ControlCode: "NIS2-B.3",
			PrereqFW: "NIS2", PrereqCode: "NIS2-B.1",
			DependencyType: "required",
			Rationale:      "CSIRT braucht IR-Richtlinie als Leitfaden",
			Source:         "NIS2 Art.21 §2(b)",
		},
		{
			ControlFW: "NIS2", ControlCode: "NIS2-B.5",
			PrereqFW: "NIS2", PrereqCode: "NIS2-B.1",
			DependencyType: "required",
			Rationale:      "Meldeverfahren müssen in IR-Richtlinie definiert sein",
			Source:         "NIS2 Art.23",
		},
		{
			ControlFW: "NIS2", ControlCode: "NIS2-B.6",
			PrereqFW: "NIS2", PrereqCode: "NIS2-B.1",
			DependencyType: "required",
			Rationale:      "Eskalationsregeln folgen aus IR-Richtlinie",
			Source:         "NIS2 Art.23",
		},
		{
			ControlFW: "NIS2", ControlCode: "NIS2-C.4",
			PrereqFW: "NIS2", PrereqCode: "NIS2-C.1",
			DependencyType: "required",
			Rationale:      "Backup-Anforderungen folgen aus BCM (RTO/RPO)",
			Source:         "NIS2 Erwägungsgrund 79",
		},

		// ── DORA interne Sequenz ──────────────────────────────────────────
		{
			ControlFW: "DORA", ControlCode: "DORA-1.1",
			PrereqFW: "DORA", PrereqCode: "DORA-1.2",
			DependencyType: "required",
			Rationale:      "ICT-Risikorahmenwerk setzt ICT-Governance voraus",
			Source:         "DORA Art. 5-8",
		},
		{
			ControlFW: "DORA", ControlCode: "DORA-1.4",
			PrereqFW: "DORA", PrereqCode: "DORA-1.1",
			DependencyType: "required",
			Rationale:      "Schutzmaßnahmen bauen auf ICT-Risikorahmenwerk auf",
			Source:         "DORA Art. 9",
		},
		{
			ControlFW: "DORA", ControlCode: "DORA-2.1",
			PrereqFW: "DORA", PrereqCode: "DORA-1.5",
			DependencyType: "recommended",
			Rationale:      "Incident-Klassifizierung profitiert von vorhandenem Monitoring",
			Source:         "DORA Art. 17",
		},
		{
			ControlFW: "DORA", ControlCode: "DORA-4.2",
			PrereqFW: "DORA", PrereqCode: "DORA-4.1",
			DependencyType: "required",
			Rationale:      "Vertragsstandards setzen Drittparteien-Richtlinie voraus",
			Source:         "DORA Art. 30",
		},

		// ── BSI Grundschutz Baustein-Abhängigkeiten ───────────────────────
		{
			ControlFW: "BSI", ControlCode: "BSI-ORP.2",
			PrereqFW: "BSI", PrereqCode: "BSI-ORP.1",
			DependencyType: "required",
			Rationale:      "Personal-Baustein setzt organisatorischen Rahmen voraus",
			Source:         "BSI IT-Grundschutz Kompendium: ORP.1 als Basis-Baustein",
		},
		{
			ControlFW: "BSI", ControlCode: "BSI-OPS.1.1.2",
			PrereqFW: "BSI", PrereqCode: "BSI-ORP.1",
			DependencyType: "required",
			Rationale:      "IT-Administration setzt organisatorische Grundlage voraus",
			Source:         "BSI IT-Grundschutz Kompendium S.47",
		},
		{
			ControlFW: "BSI", ControlCode: "BSI-SYS.1.1",
			PrereqFW: "BSI", PrereqCode: "BSI-ORP.2",
			DependencyType: "recommended",
			Rationale:      "Serversicherheit erfordert kompetentes Personal",
			Source:         "BSI IT-Grundschutz Kompendium",
		},

		// ── Cross-Framework ───────────────────────────────────────────────
		{
			ControlFW: "NIS2", ControlCode: "NIS2-A.3",
			PrereqFW: "ISO27001", PrereqCode: "A.5.1",
			DependencyType: "recommended",
			Rationale:      "NIS2 Risikoanalyse profitiert von dokumentierten IS-Richtlinien",
			Source:         "NIS2 Erwägungsgrund 89",
		},
		{
			ControlFW: "DORA", ControlCode: "DORA-4.1",
			PrereqFW: "ISO27001", PrereqCode: "A.5.19",
			DependencyType: "recommended",
			Rationale:      "DORA third-party risk builds on ISO supplier relationships framework",
			Source:         "DORA Art.28",
		},
		{
			ControlFW: "ISO27001", ControlCode: "A.5.16",
			PrereqFW: "BSI", PrereqCode: "BSI-ORP.2",
			DependencyType: "recommended",
			Rationale:      "ISO identity management requires BSI-ORP.2 personnel security concept",
			Source:         "BSI IT-GS Kompendium: ORP.4",
		},
	}
}

// ── Mapping Coverage API ──────────────────────────────────────────────────────

// FrameworkPairCoverage describes the mapping coverage between two frameworks.
type FrameworkPairCoverage struct {
	FrameworkAID   string `json:"framework_a_id"`
	FrameworkAName string `json:"framework_a_name"`
	FrameworkBID   string `json:"framework_b_id"`
	FrameworkBName string `json:"framework_b_name"`
	MappingCount   int    `json:"mapping_count"`
	IsMapped       bool   `json:"is_mapped"`
}

// MappingCoverageResponse is the full coverage matrix response.
type MappingCoverageResponse struct {
	Pairs                []FrameworkPairCoverage `json:"pairs"`
	TotalMeaningfulPairs int                     `json:"total_meaningful_pairs"`
	MappedPairs          int                     `json:"mapped_pairs"`
	CoveragePct          float64                 `json:"coverage_pct"`
}

// GetMappingCoverage returns the current cross-framework mapping coverage matrix.
func (s *Service) GetMappingCoverage(ctx context.Context, orgID string) (*MappingCoverageResponse, error) {
	frameworks, err := s.repo.ListFrameworks(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list frameworks: %w", err)
	}
	if len(frameworks) < 2 {
		return &MappingCoverageResponse{}, nil
	}

	pairs, err := s.repo.GetFrameworkMappingCounts(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("get mapping counts: %w", err)
	}

	type pairKey struct{ a, b string }
	countMap := make(map[pairKey]int, len(pairs))
	for _, p := range pairs {
		k := pairKey{a: p.FrameworkAName, b: p.FrameworkBName}
		countMap[k] = p.MappingCount
	}

	var result []FrameworkPairCoverage
	for i := 0; i < len(frameworks); i++ {
		for j := i + 1; j < len(frameworks); j++ {
			a, b := frameworks[i], frameworks[j]
			count := countMap[pairKey{a: a.Name, b: b.Name}] + countMap[pairKey{a: b.Name, b: a.Name}]
			result = append(result, FrameworkPairCoverage{
				FrameworkAID:   a.ID,
				FrameworkAName: a.Name,
				FrameworkBID:   b.ID,
				FrameworkBName: b.Name,
				MappingCount:   count,
				IsMapped:       count > 0,
			})
		}
	}

	mapped := 0
	for _, p := range result {
		if p.IsMapped {
			mapped++
		}
	}
	total := len(result)
	var pct float64
	if total > 0 {
		pct = float64(mapped) / float64(total) * 100.0
	}
	return &MappingCoverageResponse{
		Pairs:                result,
		TotalMeaningfulPairs: total,
		MappedPairs:          mapped,
		CoveragePct:          pct,
	}, nil
}

// ── Implementation Path ───────────────────────────────────────────────────────

// PrereqRef is a reference to a prerequisite control for frontend display.
type PrereqRef struct {
	FrameworkID     string `json:"framework_id"`
	FrameworkName   string `json:"framework_name"`
	ControlCode     string `json:"control_code"`
	ControlTitle    string `json:"control_title"`
	CurrentStatus   string `json:"current_status"`
	DependencyType  string `json:"dependency_type"`
	FrameworkActive bool   `json:"framework_active"`
}

// ImplementationStep is one ordered step in the framework implementation path.
type ImplementationStep struct {
	StepNr           int         `json:"step_nr"`
	FrameworkID      string      `json:"framework_id"`
	ControlCode      string      `json:"control_code"`
	ControlTitle     string      `json:"control_title"`
	CurrentStatus    string      `json:"current_status"`
	PrerequisitesMet bool        `json:"prerequisites_met"`
	BlockingPrereqs  []PrereqRef `json:"blocking_prerequisites,omitempty"`
}

// GetImplementationPath returns controls for a framework sorted in recommended
// implementation order using Kahn's topological sort on ck_control_prerequisites.
func (s *Service) GetImplementationPath(ctx context.Context, orgID, frameworkID string) ([]ImplementationStep, error) {
	controls, err := s.repo.ListControls(ctx, orgID, frameworkID)
	if err != nil {
		return nil, fmt.Errorf("list controls: %w", err)
	}
	if len(controls) == 0 {
		return []ImplementationStep{}, nil
	}

	fw, err := s.repo.GetFramework(ctx, orgID, frameworkID)
	if err != nil {
		return nil, fmt.Errorf("get framework: %w", err)
	}

	prereqs, err := s.repo.ListPrerequisitesByFramework(ctx, fw.Name)
	if err != nil {
		return nil, fmt.Errorf("list prerequisites: %w", err)
	}

	// Build adjacency: controlCode → prerequisite codes (intra-framework only).
	prereqMap := make(map[string][]string)
	enablesMap := make(map[string][]string)
	for _, p := range prereqs {
		if p.PrerequisiteFramework == fw.Name {
			prereqMap[p.ControlCode] = append(prereqMap[p.ControlCode], p.PrerequisiteCode)
			enablesMap[p.PrerequisiteCode] = append(enablesMap[p.PrerequisiteCode], p.ControlCode)
		}
	}

	controlByCode := make(map[string]Control, len(controls))
	for _, c := range controls {
		controlByCode[c.ControlID] = c
	}

	// Kahn's topological sort.
	inDegree := make(map[string]int, len(controls))
	for _, c := range controls {
		inDegree[c.ControlID] = len(prereqMap[c.ControlID])
	}

	queue := make([]string, 0, len(controls))
	for _, c := range controls {
		if inDegree[c.ControlID] == 0 {
			queue = append(queue, c.ControlID)
		}
	}

	sorted := make([]string, 0, len(controls))
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		sorted = append(sorted, node)
		for _, next := range enablesMap[node] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	// Append disconnected/cyclic controls at the end.
	sortedSet := make(map[string]bool, len(sorted))
	for _, code := range sorted {
		sortedSet[code] = true
	}
	for _, c := range controls {
		if !sortedSet[c.ControlID] {
			sorted = append(sorted, c.ControlID)
		}
	}

	steps := make([]ImplementationStep, 0, len(sorted))
	for i, code := range sorted {
		c, ok := controlByCode[code]
		if !ok {
			continue
		}
		blocking := make([]PrereqRef, 0)
		for _, preCode := range prereqMap[code] {
			pc, exists := controlByCode[preCode]
			if !exists {
				continue
			}
			if pc.ManualStatus != "implemented" {
				blocking = append(blocking, PrereqRef{
					FrameworkID:     frameworkID,
					FrameworkName:   fw.Name,
					ControlCode:     preCode,
					ControlTitle:    pc.Title,
					CurrentStatus:   pc.ManualStatus,
					DependencyType:  "required",
					FrameworkActive: true,
				})
			}
		}
		steps = append(steps, ImplementationStep{
			StepNr:           i + 1,
			FrameworkID:      frameworkID,
			ControlCode:      c.ControlID,
			ControlTitle:     c.Title,
			CurrentStatus:    c.ManualStatus,
			PrerequisitesMet: len(blocking) == 0,
			BlockingPrereqs:  blocking,
		})
	}
	return steps, nil
}

// ── Evidence Propagation ──────────────────────────────────────────────────────

// PropagateControlStatus marks mapped controls in other frameworks as 'partial'
// when a control is set to 'implemented'. Never downgrades 'implemented' → 'partial'.
func (s *Service) PropagateControlStatus(ctx context.Context, orgID, controlID string) error {
	mappings, err := s.GetControlMappings(ctx, orgID, controlID)
	if err != nil {
		return fmt.Errorf("get control mappings for propagation: %w", err)
	}
	for _, m := range mappings {
		// SetControlPartialIfUnset skips 'implemented' and 'partial' controls.
		if err := s.repo.SetControlPartialIfUnset(ctx, orgID, m.TargetControlID); err != nil {
			log.Warn().Err(err).Str("target_control", m.TargetControlID).Msg("propagate status failed")
		}
	}
	return nil
}
