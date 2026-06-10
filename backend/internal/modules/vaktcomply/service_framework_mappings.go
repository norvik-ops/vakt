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

// ── DORA Hotfix (A0b) ───────────────────────────────────────────────────────
//
// SeedDORAMappingsFixed replaces the broken SeedDORAMappings.
// The original doraISO27001Mapping used ISO 27001:2022 control IDs (e.g. "A.5.30")
// that don't exist in the DB which stores 2013-style codes ("A.5.1", "A.12.6").
// This caused 0 DORA↔ISO mappings to ever be persisted.
//
// This version uses correct 2013-style ISO 27001 control IDs matching iso27001Controls().

var doraISO27001MappingFixed = map[string]string{
	"DORA-1.1": "A.17.1, A.12.3", // BCM/Risiko-Framework
	"DORA-1.2": "A.5.1, A.6.1",   // Governance
	"DORA-1.3": "A.8.1",          // Asset-Inventar
	"DORA-1.4": "A.14.2, A.12.6", // Schutzmaßnahmen
	"DORA-1.5": "A.12.4",         // Erkennung/Monitoring
	"DORA-1.6": "A.17.1, A.12.3", // BCM
	"DORA-1.7": "A.12.3",         // Backup
	"DORA-1.8": "A.12.6",         // Patch/Schwachstellen
	"DORA-2.1": "A.16.1",         // Incident-Klassifizierung
	"DORA-2.2": "A.16.1",         // Meldepflichten
	"DORA-2.3": "A.16.1",         // IR-Prozess
	"DORA-2.4": "A.16.1",         // Post-Incident-Review
	"DORA-3.1": "A.14.2, A.12.6", // Resilienztests
	"DORA-3.2": "A.12.6",         // TLPT
	"DORA-3.3": "A.14.2",         // Szenarientests
	"DORA-4.1": "A.15.1",         // Drittparteienrisiko
	"DORA-4.2": "A.15.1, A.18.1", // Vertragsanforderungen
	"DORA-4.3": "A.15.1",         // Ausstiegsstrategie
	"DORA-4.4": "A.15.1",         // Register
	"DORA-4.5": "A.15.1",         // Vertragsklauseln
	"DORA-4.6": "A.15.1",         // Konzentrationsrisiko
	"DORA-5.1": "A.6.1",          // Informationsaustausch-Rahmen
	"DORA-5.2": "A.12.1",         // Threat-Intel
	"DORA-5.3": "A.16.1",         // Behörden-Meldung
}

// SeedDORAMappingsFixed idempotently seeds DORA ↔ ISO 27001 mappings using
// corrected ISO 27001 control IDs (A0b hotfix for S69-1).
func (s *Service) SeedDORAMappingsFixed(ctx context.Context, orgID string) error {
	// Verify both frameworks are active for this org before seeding global table.
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
	for doraCode, isoCodes := range doraISO27001MappingFixed {
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
	{src: "CRA", srcCode: "CRA-1.1", tgt: "ISO27001", tgtCode: "A.14.1", mtype: "equivalent"},
	{src: "CRA", srcCode: "CRA-1.2", tgt: "ISO27001", tgtCode: "A.5.1", mtype: "equivalent"},
	{src: "CRA", srcCode: "CRA-1.3", tgt: "ISO27001", tgtCode: "A.12.6", mtype: "equivalent"},
	{src: "CRA", srcCode: "CRA-1.4", tgt: "ISO27001", tgtCode: "A.12.6", mtype: "partial"},
	{src: "CRA", srcCode: "CRA-1.5", tgt: "ISO27001", tgtCode: "A.14.2", mtype: "equivalent"},
	{src: "CRA", srcCode: "CRA-1.6", tgt: "ISO27001", tgtCode: "A.12.6", mtype: "equivalent"},
	{src: "CRA", srcCode: "CRA-1.7", tgt: "ISO27001", tgtCode: "A.12.6", mtype: "equivalent"},
	{src: "CRA", srcCode: "CRA-1.8", tgt: "ISO27001", tgtCode: "A.9.4", mtype: "equivalent"},
	{src: "CRA", srcCode: "CRA-1.9", tgt: "ISO27001", tgtCode: "A.10.1", mtype: "equivalent"},
	{src: "CRA", srcCode: "CRA-1.10", tgt: "ISO27001", tgtCode: "A.12.4", mtype: "equivalent"},
	{src: "CRA", srcCode: "CRA-2.1", tgt: "ISO27001", tgtCode: "A.16.1", mtype: "equivalent"},
	{src: "CRA", srcCode: "CRA-2.2", tgt: "ISO27001", tgtCode: "A.12.6", mtype: "partial"},
	{src: "CRA", srcCode: "CRA-2.3", tgt: "ISO27001", tgtCode: "A.16.1", mtype: "partial"},
	{src: "CRA", srcCode: "CRA-3.1", tgt: "ISO27001", tgtCode: "A.14.1", mtype: "equivalent"},
	{src: "CRA", srcCode: "CRA-3.2", tgt: "ISO27001", tgtCode: "A.12.6", mtype: "partial"},
	{src: "CRA", srcCode: "CRA-3.3", tgt: "ISO27001", tgtCode: "A.14.2", mtype: "partial"},
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

// SeedNIS2DORAMappings seeds NIS2 ↔ DORA bidirectional mappings.
func (s *Service) SeedNIS2DORAMappings(ctx context.Context, orgID string) error {
	return s.seedBidirectionalMappings(ctx, orgID, "NIS2", "DORA", nis2DORAMappings)
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
func (s *Service) SeedNIS2BSIMappings(ctx context.Context, orgID string) error {
	return s.seedBidirectionalMappings(ctx, orgID, "NIS2", "BSI", nis2BSIMappings)
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
	{src: "EUAIACT", srcCode: "AIACT-7.3", tgt: "ISO27001", tgtCode: "A.14.2", mtype: "equivalent"},
	{src: "EUAIACT", srcCode: "AIACT-7.3", tgt: "ISO27001", tgtCode: "A.12.6", mtype: "partial"},
	{src: "EUAIACT", srcCode: "AIACT-4.1", tgt: "ISO27001", tgtCode: "A.12.4", mtype: "equivalent"},
	{src: "EUAIACT", srcCode: "AIACT-7.2", tgt: "ISO27001", tgtCode: "A.12.6", mtype: "partial"},
	{src: "EUAIACT", srcCode: "AIACT-8.1", tgt: "ISO27001", tgtCode: "A.18.1", mtype: "equivalent"},
}

// SeedEUAIActISO27001Mappings seeds EU AI Act ↔ ISO 27001 bidirectional mappings.
func (s *Service) SeedEUAIActISO27001Mappings(ctx context.Context, orgID string) error {
	return s.seedBidirectionalMappings(ctx, orgID, "EUAIACT", "ISO27001", euAIActISO27001Mappings)
}

// ── ISO 42001 ↔ ISO 27001 ────────────────────────────────────────────────────

var iso42001ISO27001Mappings = []frameworkPair{
	{src: "ISO42001", srcCode: "42001-5.1", tgt: "ISO27001", tgtCode: "A.5.1", mtype: "equivalent"},
	{src: "ISO42001", srcCode: "42001-6.1", tgt: "ISO27001", tgtCode: "A.5.1", mtype: "partial"},
	{src: "ISO42001", srcCode: "42001-7.1", tgt: "ISO27001", tgtCode: "A.6.1", mtype: "partial"},
	{src: "ISO42001", srcCode: "42001-7.3", tgt: "ISO27001", tgtCode: "A.12.1", mtype: "equivalent"},
	{src: "ISO42001", srcCode: "42001-9.1", tgt: "ISO27001", tgtCode: "A.18.1", mtype: "equivalent"},
}

// SeedISO42001ISO27001Mappings seeds ISO 42001 ↔ ISO 27001 bidirectional mappings.
func (s *Service) SeedISO42001ISO27001Mappings(ctx context.Context, orgID string) error {
	return s.seedBidirectionalMappings(ctx, orgID, "ISO42001", "ISO27001", iso42001ISO27001Mappings)
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
		// ── ISO 27001 Clause-Hierarchie ──────────────────────────────────
		{
			ControlFW: "ISO27001", ControlCode: "A.5.1.2",
			PrereqFW: "ISO27001", PrereqCode: "A.5.1.1",
			DependencyType: "required",
			Rationale:      "Richtlinienüberprüfung setzt genehmigte Richtlinien voraus",
			Source:         "ISO 27001:2022 §A.5.1",
		},
		{
			ControlFW: "ISO27001", ControlCode: "A.9.2",
			PrereqFW: "ISO27001", PrereqCode: "A.9.1",
			DependencyType: "required",
			Rationale:      "Benutzerzugangsverwaltung setzt Zugangskontrollrichtlinie voraus",
			Source:         "ISO 27001:2022 §A.9",
		},
		{
			ControlFW: "ISO27001", ControlCode: "A.9.4",
			PrereqFW: "ISO27001", PrereqCode: "A.9.2",
			DependencyType: "recommended",
			Rationale:      "Systemzugangskontrolle setzt definierten Provisionierungsprozess voraus",
			Source:         "ISO 27001:2022 §A.9",
		},
		{
			ControlFW: "ISO27001", ControlCode: "A.12.4",
			PrereqFW: "ISO27001", PrereqCode: "A.12.1",
			DependencyType: "recommended",
			Rationale:      "Protokollierung setzt dokumentierte Betriebsverfahren voraus",
			Source:         "ISO 27001:2022 §A.12",
		},
		{
			ControlFW: "ISO27001", ControlCode: "A.12.6",
			PrereqFW: "ISO27001", PrereqCode: "A.12.4",
			DependencyType: "recommended",
			Rationale:      "Schwachstellenerkennung setzt Log-Infrastruktur voraus",
			Source:         "ISO 27001:2022 §A.12",
		},
		{
			ControlFW: "ISO27001", ControlCode: "A.14.2",
			PrereqFW: "ISO27001", PrereqCode: "A.14.1",
			DependencyType: "required",
			Rationale:      "Sicherheit in Entwicklungsprozessen setzt definierte Sicherheitsanforderungen voraus",
			Source:         "ISO 27001:2022 §A.14",
		},
		{
			ControlFW: "ISO27001", ControlCode: "A.15.2",
			PrereqFW: "ISO27001", PrereqCode: "A.15.1",
			DependencyType: "required",
			Rationale:      "Lieferanten-Überwachung setzt Lieferantenvereinbarungen voraus",
			Source:         "ISO 27001:2022 §A.15",
		},
		{
			ControlFW: "ISO27001", ControlCode: "A.16.1.2",
			PrereqFW: "ISO27001", PrereqCode: "A.16.1.1",
			DependencyType: "required",
			Rationale:      "Meldewege setzen definierte Zuständigkeiten voraus",
			Source:         "ISO 27001:2022 §A.16.1",
		},
		{
			ControlFW: "ISO27001", ControlCode: "A.17.1.2",
			PrereqFW: "ISO27001", PrereqCode: "A.17.1.1",
			DependencyType: "required",
			Rationale:      "BCM-Implementierung folgt BCM-Richtlinie",
			Source:         "ISO 27001:2022 §A.17",
		},
		{
			ControlFW: "ISO27001", ControlCode: "A.18.2",
			PrereqFW: "ISO27001", PrereqCode: "A.18.1",
			DependencyType: "required",
			Rationale:      "Compliance-Überprüfung setzt identifizierte Anforderungen voraus",
			Source:         "ISO 27001:2022 §A.18",
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
			PrereqFW: "ISO27001", PrereqCode: "A.15.1",
			DependencyType: "recommended",
			Rationale:      "DORA-Drittparteienrisiko baut auf ISO-Lieferantenkonzept auf",
			Source:         "DORA Art.28",
		},
		{
			ControlFW: "ISO27001", ControlCode: "A.9.2",
			PrereqFW: "BSI", PrereqCode: "BSI-ORP.2",
			DependencyType: "recommended",
			Rationale:      "ISO-Benutzer-Provisioning erfordert BSI-Identitätsmanagement-Konzept",
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
