// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package policy

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/db"
	"github.com/matharnica/vakt/internal/shared/notify"
)

// --- Frameworks ---

// ListFrameworks returns all frameworks enabled for the given organisation.
func (s *Service) ListFrameworks(ctx context.Context, orgID string) ([]Framework, error) {
	frameworks, err := s.repo.ListFrameworks(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list frameworks: %w", err)
	}
	return frameworks, nil
}

// DeleteFramework removes a framework and all associated data.
func (s *Service) DeleteFramework(ctx context.Context, orgID, frameworkID string) error {
	return s.repo.DeleteFramework(ctx, orgID, frameworkID)
}

// EnableFramework creates a new framework (and seeds its controls) for the organisation.
// If the framework is already enabled, it returns the existing record.
// variant is "full" or "simplified" (only meaningful for DORA); empty string defaults to "full".
// name is normalised to upper-case before any look-up so "bsi" and "BSI" behave identically.
func (s *Service) EnableFramework(ctx context.Context, orgID, name, variant string) (*Framework, error) {
	name = strings.ToUpper(name)
	if variant == "" {
		variant = "full"
	}

	// Reject enabling of draft frameworks.
	for _, b := range builtinAvailable {
		if strings.EqualFold(b.name, name) && b.status == "draft" {
			return nil, fmt.Errorf("framework %s is in draft status and cannot be enabled yet", name)
		}
	}

	exists, err := s.repo.FrameworkExists(ctx, orgID, name)
	if err != nil {
		return nil, err
	}
	if exists {
		// Return the already-enabled framework.
		frameworks, err := s.repo.ListFrameworks(ctx, orgID)
		if err != nil {
			return nil, err
		}
		for i := range frameworks {
			if frameworks[i].Name == name {
				return &frameworks[i], nil
			}
		}
	}

	// Determine version from built-in templates.
	version := BuiltinVersion(name)
	isBuiltin := version != ""
	if version == "" {
		version = "1.0"
	}

	fw, err := s.repo.CreateFramework(ctx, orgID, name, version, isBuiltin, variant)
	if err != nil {
		return nil, fmt.Errorf("enable framework %s: %w", name, err)
	}

	// Seed controls from built-in template.
	controls := BuiltinControls(fw.ID, orgID, name, variant)
	if len(controls) > 0 {
		if err := s.repo.BulkInsertControls(ctx, controls); err != nil {
			log.Warn().Err(err).Str("framework", name).Msg("failed to seed controls")
		}
	}

	// S76-1: Populate requirement_level for BSI controls (Basis/Standard/Erhöht).
	if name == "BSI" {
		levels := bsiRequirementLevels()
		if len(levels) > 0 {
			if err := s.repo.UpdateBSIRequirementLevels(ctx, fw.ID, orgID, levels); err != nil {
				log.Warn().Err(err).Str("framework", name).Msg("failed to update BSI requirement levels")
			}
		}
	}

	// Seed cross-framework mappings via registry (S79-9).
	// Each entry fires when the newly enabled framework matches either fwA or fwB.
	for _, entry := range s.MappingRegistry() {
		if name == entry.fwA || name == entry.fwB {
			if seedErr := entry.fn(ctx, orgID); seedErr != nil {
				log.Warn().Err(seedErr).
					Str("fw_a", entry.fwA).Str("fw_b", entry.fwB).Str("framework", name).
					Msg("failed to seed cross-framework mappings (non-critical)")
			}
		}
	}

	return fw, nil
}

// ListAvailableFrameworks returns all frameworks that can be enabled, merged with current org state.
func (s *Service) ListAvailableFrameworks(ctx context.Context, orgID string) ([]AvailableFramework, error) {
	enabled, err := s.repo.ListFrameworks(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list frameworks: %w", err)
	}
	enabledByName := make(map[string]bool, len(enabled))
	for _, fw := range enabled {
		enabledByName[fw.Name] = true
	}

	result := make([]AvailableFramework, 0, len(builtinAvailable))
	for _, b := range builtinAvailable {
		result = append(result, AvailableFramework{
			Name:                b.name,
			Version:             BuiltinVersion(b.name),
			Description:         b.description,
			IsBuiltin:           true,
			IsEnabled:           enabledByName[b.name],
			Status:              b.status,
			ExpectedPublication: b.expectedPublication,
		})
	}
	return result, nil
}

// InstallFrameworkPlugin parses a FrameworkPlugin and creates the framework with its controls.
func (s *Service) InstallFrameworkPlugin(ctx context.Context, orgID string, plugin *FrameworkPlugin) (*Framework, error) {
	if plugin.Name == "" {
		return nil, fmt.Errorf("plugin name is required")
	}
	version := plugin.Version
	if version == "" {
		version = "1.0"
	}

	fw, err := s.repo.CreateFramework(ctx, orgID, plugin.Name, version, false, "full")
	if err != nil {
		return nil, fmt.Errorf("create plugin framework %s: %w", plugin.Name, err)
	}

	controls := make([]Control, 0, len(plugin.Controls))
	for _, pc := range plugin.Controls {
		evType := pc.EvidenceType
		if evType == "" {
			evType = "manual"
		}
		controls = append(controls, Control{
			FrameworkID:  fw.ID,
			OrgID:        orgID,
			ControlID:    pc.ID,
			Title:        pc.Title,
			Description:  pc.Description,
			Domain:       pc.Domain,
			EvidenceType: evType,
			Weight:       pc.Weight,
		})
	}
	if len(controls) > 0 {
		if err := s.repo.BulkInsertControls(ctx, controls); err != nil {
			return nil, fmt.Errorf("seed plugin controls: %w", err)
		}
	}
	return fw, nil
}

// ReseedBuiltinControls reseeds controls for all builtin frameworks across all orgs.
// Called on startup after migrations to ensure controls are up-to-date.
func (s *Service) ReseedBuiltinControls(ctx context.Context) {
	frameworks, err := s.repo.ListAllBuiltinFrameworks(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("reseed: failed to list builtin frameworks")
		return
	}
	for _, fw := range frameworks {
		controls := BuiltinControls(fw.ID, fw.OrgID, fw.Name, fw.FrameworkVariant)
		if len(controls) == 0 {
			continue
		}
		if err := s.repo.BulkInsertControls(ctx, controls); err != nil {
			log.Warn().Err(err).Str("framework", fw.Name).Msg("reseed: failed to insert controls")
		} else {
			log.Info().Str("framework", fw.Name).Int("controls", len(controls)).Msg("reseeded builtin controls")
		}
	}
}

// SwitchDORAVariant switches an enabled DORA framework between "full" (Art. 5–15)
// and "simplified" (Art. 16) variants.
//
// When switching:
//   - Controls from the OLD variant are marked not_applicable (evidence is preserved).
//   - Controls from the NEW variant that don't yet exist are inserted as not_implemented.
//   - The framework_variant column is updated.
func (s *Service) SwitchDORAVariant(ctx context.Context, orgID, frameworkID, newVariant string) (*Framework, error) {
	fw, err := s.repo.GetFramework(ctx, orgID, frameworkID)
	if err != nil {
		return nil, fmt.Errorf("get framework: %w", err)
	}
	if fw.Name != "DORA" {
		return nil, fmt.Errorf("framework is not DORA")
	}
	if fw.FrameworkVariant == newVariant {
		return fw, nil // already on requested variant — nothing to do
	}

	// Determine which control ID prefix belongs to the OLD variant.
	oldPrefix := "DORA-"      // full variant: DORA-1.x … DORA-5.x
	if newVariant == "full" { // switching from simplified → the old ones have DORA-S. prefix
		oldPrefix = "DORA-S."
	}

	// Load all current controls.
	allControls, err := s.repo.ListControls(ctx, orgID, frameworkID)
	if err != nil {
		return nil, fmt.Errorf("list controls: %w", err)
	}

	// Mark old-variant controls as not_applicable.
	for _, ctrl := range allControls {
		isOldVariant := false
		if newVariant == "full" {
			// we're switching full→simplified, old controls have numeric prefix e.g. DORA-1.1
			isOldVariant = strings.HasPrefix(ctrl.ControlID, "DORA-") && !strings.HasPrefix(ctrl.ControlID, "DORA-S.")
		} else {
			// switching simplified→full, old controls have DORA-S. prefix
			isOldVariant = strings.HasPrefix(ctrl.ControlID, oldPrefix)
		}
		if !isOldVariant {
			continue
		}
		_ = s.repo.UpdateControl(ctx, orgID, ctrl.ID, true, "not applicable — switched to "+newVariant+" framework variant", "", "", nil, nil)
	}

	// Seed new-variant controls (BulkInsertControls is ON CONFLICT DO UPDATE for title/desc).
	newControls := BuiltinControls(frameworkID, orgID, "DORA", newVariant)
	if len(newControls) > 0 {
		if err := s.repo.BulkInsertControls(ctx, newControls); err != nil {
			return nil, fmt.Errorf("seed new variant controls: %w", err)
		}
	}

	// Persist new variant in DB.
	if err := s.repo.UpdateFrameworkVariant(ctx, orgID, frameworkID, newVariant); err != nil {
		return nil, fmt.Errorf("update framework variant: %w", err)
	}

	fw.FrameworkVariant = newVariant
	return fw, nil
}

// GetControlMappings returns all global cross-framework mappings for a given control,
// resolved to org-specific control UUIDs.
func (s *Service) GetControlMappings(ctx context.Context, orgID, controlID string) ([]ControlMapping, error) {
	mappings, err := s.repo.GetMappingsForControl(ctx, orgID, controlID)
	if err != nil {
		return nil, fmt.Errorf("get control mappings: %w", err)
	}
	return mappings, nil
}

// SeedFrameworkMappings idempotently seeds the global ISO 27001 ↔ NIS2 and
// ISO 27001 ↔ BSI cross-framework control mappings into ck_framework_control_mappings.
// These are global text-code entries — no org_id required.
// Called once on startup alongside ReseedBuiltinControls.
func (s *Service) SeedFrameworkMappings(ctx context.Context) error {
	type entry struct {
		srcFW, srcCode, tgtFW, tgtCode, mappingType string
	}

	// Framework slugs must match the lower(name) LIKE '%<slug>%' pattern used at query time.
	// Use the exact framework names stored in ck_frameworks.
	const (
		iso  = "ISO27001"
		nis2 = "NIS2"
		bsi  = "BSI"
	)

	// ISO 27001:2022 ↔ NIS2 / §30 BISG bidirectional mappings.
	// Source: BSI Onepager "NIS-2 und ISO/IEC 27001:2022" (§30 Abs. 2 S. 2 Nr. 1–10).
	// §30 BISG Nr. = NIS2 Art. 21 §2 sub-clause (a)–(j); NIS2 control IDs as stored in DB.
	isoNIS2 := []entry{
		// §30 Nr. 1 / §2(a) — Risikoanalyse + IT-Sicherheitskonzept → NIS2-A.1
		{iso, "A.5.1", nis2, "NIS2-A.1", "equivalent"},  // Policies for IS
		{iso, "A.5.2", nis2, "NIS2-A.1", "partial"},     // IS roles and responsibilities
		{iso, "A.5.3", nis2, "NIS2-A.1", "partial"},     // Segregation of duties
		{iso, "A.5.4", nis2, "NIS2-A.1", "partial"},     // Management responsibilities
		{iso, "A.5.7", nis2, "NIS2-A.1", "partial"},     // Threat intelligence
		{iso, "A.5.31", nis2, "NIS2-A.1", "partial"},    // Legal/regulatory requirements
		{iso, "A.5.35", nis2, "NIS2-A.1", "partial"},    // Independent review of IS
		{iso, "A.5.36", nis2, "NIS2-A.1", "equivalent"}, // Compliance with IS policies
		{iso, "A.8.34", nis2, "NIS2-A.1", "partial"},    // Protection during audit testing

		// §30 Nr. 2 / §2(b) — Bewältigung von Sicherheitsvorfällen → NIS2-B.1, NIS2-B.5
		{iso, "A.5.24", nis2, "NIS2-B.1", "equivalent"}, // Incident management planning
		{iso, "A.5.25", nis2, "NIS2-B.1", "partial"},    // Assessment of IS events
		{iso, "A.5.26", nis2, "NIS2-B.1", "partial"},    // Response to IS incidents
		{iso, "A.5.27", nis2, "NIS2-B.1", "partial"},    // Learning from incidents
		{iso, "A.5.28", nis2, "NIS2-B.1", "partial"},    // Collection of evidence
		{iso, "A.6.8", nis2, "NIS2-B.1", "partial"},     // IS event reporting
		{iso, "A.8.15", nis2, "NIS2-B.1", "partial"},    // Logging (incident evidence)
		{iso, "A.8.16", nis2, "NIS2-B.1", "partial"},    // Monitoring activities
		{iso, "A.8.17", nis2, "NIS2-B.1", "partial"},    // Clock synchronisation
		{iso, "A.6.8", nis2, "NIS2-B.5", "equivalent"},  // IS event reporting (24h-Meldung)
		{iso, "A.5.24", nis2, "NIS2-B.5", "partial"},    // Incident planning includes notification

		// §30 Nr. 3 / §2(c) — Business Continuity Management → NIS2-C.1, NIS2-C.4
		{iso, "A.5.29", nis2, "NIS2-C.1", "equivalent"}, // IS during disruption
		{iso, "A.5.30", nis2, "NIS2-C.1", "equivalent"}, // ICT readiness for BCM
		{iso, "A.5.26", nis2, "NIS2-C.1", "partial"},    // Response to incidents (crisis)
		{iso, "A.7.11", nis2, "NIS2-C.1", "partial"},    // Supporting utilities
		{iso, "A.8.14", nis2, "NIS2-C.1", "partial"},    // Redundancy of processing
		{iso, "A.8.13", nis2, "NIS2-C.4", "equivalent"}, // Information backup

		// §30 Nr. 4 / §2(d) — Sichere Lieferkette → NIS2-D.1
		{iso, "A.5.19", nis2, "NIS2-D.1", "equivalent"}, // IS in supplier relationships
		{iso, "A.5.20", nis2, "NIS2-D.1", "equivalent"}, // IS in supplier agreements
		{iso, "A.5.21", nis2, "NIS2-D.1", "partial"},    // IS in ICT supply chain
		{iso, "A.5.22", nis2, "NIS2-D.1", "partial"},    // Supplier service monitoring
		{iso, "A.8.30", nis2, "NIS2-D.1", "partial"},    // Outsourced development

		// §30 Nr. 5 / §2(e) — Sicherheitsmaßnahmen + Schwachstellenmanagement → NIS2-E.3, NIS2-E.4
		{iso, "A.8.8", nis2, "NIS2-E.3", "equivalent"}, // Technical vulnerability management
		{iso, "A.8.9", nis2, "NIS2-E.3", "partial"},    // Configuration management
		{iso, "A.8.7", nis2, "NIS2-E.3", "partial"},    // Protection against malware
		{iso, "A.5.23", nis2, "NIS2-E.3", "partial"},   // IS for use of cloud services
		{iso, "A.5.21", nis2, "NIS2-E.3", "partial"},   // IS in ICT supply chain
		{iso, "A.7.3", nis2, "NIS2-E.3", "partial"},    // Securing offices, rooms, facilities
		{iso, "A.7.5", nis2, "NIS2-E.3", "partial"},    // Protecting against physical threats
		{iso, "A.7.13", nis2, "NIS2-E.3", "partial"},   // Equipment maintenance
		{iso, "A.8.16", nis2, "NIS2-E.3", "partial"},   // Monitoring activities
		{iso, "A.8.20", nis2, "NIS2-E.3", "partial"},   // Network security
		{iso, "A.8.22", nis2, "NIS2-E.3", "partial"},   // Segregation of networks
		{iso, "A.8.25", nis2, "NIS2-E.3", "partial"},   // Secure development life cycle
		{iso, "A.8.29", nis2, "NIS2-E.3", "partial"},   // Security testing
		{iso, "A.8.31", nis2, "NIS2-E.3", "partial"},   // Separation of dev/test/prod
		{iso, "A.8.33", nis2, "NIS2-E.3", "partial"},   // Test information
		{iso, "A.8.34", nis2, "NIS2-E.3", "partial"},   // Protection during audit testing
		{iso, "A.8.8", nis2, "NIS2-E.4", "equivalent"}, // Vuln mgmt covers patching
		{iso, "A.8.32", nis2, "NIS2-E.4", "partial"},   // Change management (patches)

		// §30 Nr. 7 / §2(g) — Schulungen + Sensibilisierungsmaßnahmen → NIS2-G.2
		{iso, "A.6.3", nis2, "NIS2-G.2", "equivalent"}, // IS awareness, education, training
		{iso, "A.8.7", nis2, "NIS2-G.2", "partial"},    // Malware protection (user-behaviour)

		// §30 Nr. 8 / §2(h) — Kryptografische Verfahren → NIS2-H.1
		{iso, "A.8.24", nis2, "NIS2-H.1", "equivalent"}, // Use of cryptography
		{iso, "A.5.31", nis2, "NIS2-H.1", "partial"},    // Legal requirements on cryptography

		// §30 Nr. 9 / §2(i) — Personalsicherheit, Zugriffskontrolle + Assetmanagement
		// Asset management → NIS2-A.8
		{iso, "A.5.9", nis2, "NIS2-A.8", "equivalent"}, // Inventory of assets
		{iso, "A.5.10", nis2, "NIS2-A.8", "partial"},   // Acceptable use of assets
		{iso, "A.5.11", nis2, "NIS2-A.8", "partial"},   // Return of assets
		{iso, "A.5.12", nis2, "NIS2-A.8", "partial"},   // Classification of information
		{iso, "A.5.13", nis2, "NIS2-A.8", "partial"},   // Labelling of information
		{iso, "A.5.14", nis2, "NIS2-A.8", "partial"},   // Information transfer
		{iso, "A.7.10", nis2, "NIS2-A.8", "partial"},   // Storage media
		// HR / people security → NIS2-A.6
		{iso, "A.6.1", nis2, "NIS2-A.6", "equivalent"}, // Screening
		{iso, "A.6.2", nis2, "NIS2-A.6", "partial"},    // Terms and conditions of employment
		{iso, "A.6.4", nis2, "NIS2-A.6", "partial"},    // Disciplinary process
		{iso, "A.6.5", nis2, "NIS2-A.6", "partial"},    // Responsibilities after termination
		{iso, "A.7.1", nis2, "NIS2-A.6", "partial"},    // Physical security perimeters
		{iso, "A.7.4", nis2, "NIS2-A.6", "partial"},    // Physical security monitoring
		{iso, "A.7.7", nis2, "NIS2-A.6", "partial"},    // Clear desk and clear screen
		// Access control → NIS2-F.1
		{iso, "A.5.15", nis2, "NIS2-F.1", "equivalent"}, // Access control policy
		{iso, "A.5.16", nis2, "NIS2-F.1", "equivalent"}, // Identity management
		{iso, "A.5.17", nis2, "NIS2-F.1", "partial"},    // Authentication information
		{iso, "A.5.18", nis2, "NIS2-F.1", "partial"},    // Access rights
		{iso, "A.5.28", nis2, "NIS2-F.1", "partial"},    // Collection of evidence
		{iso, "A.8.2", nis2, "NIS2-F.1", "partial"},     // Privileged access rights
		{iso, "A.8.3", nis2, "NIS2-F.1", "partial"},     // Information access restriction
		{iso, "A.8.18", nis2, "NIS2-F.1", "partial"},    // Use of privileged utility programs
		{iso, "A.8.21", nis2, "NIS2-F.1", "partial"},    // Security of network services

		// §30 Nr. 10 / §2(j) — Multi-Faktor-Authentisierung + gesicherte Kommunikation
		// MFA → NIS2-F.1 (secure authentication is part of access control)
		{iso, "A.8.5", nis2, "NIS2-F.1", "equivalent"}, // Secure authentication (incl. MFA)
		// Secure communications → NIS2-E.8
		{iso, "A.8.20", nis2, "NIS2-E.8", "equivalent"}, // Network security
		{iso, "A.8.21", nis2, "NIS2-E.8", "equivalent"}, // Security of network services
		{iso, "A.8.22", nis2, "NIS2-E.8", "partial"},    // Segregation of networks
		{iso, "A.7.2", nis2, "NIS2-E.8", "partial"},     // Physical entry controls

		// ENISA TIG v1.2 — additions: Req. 6.x (Secure Development) → NIS2-E.1
		// Source: ENISA Technical Implementation Guidance EU 2024/2690, Req. 6.2.1
		{iso, "A.8.26", nis2, "NIS2-E.1", "equivalent"}, // Application security requirements
		{iso, "A.8.27", nis2, "NIS2-E.1", "partial"},    // Secure systems architecture
		{iso, "A.8.28", nis2, "NIS2-E.1", "partial"},    // Secure coding
		{iso, "A.5.8", nis2, "NIS2-E.1", "partial"},     // IS in project management

		// ENISA TIG v1.2 — Req. 13.x (Physical Security) missing A.7.x entries
		{iso, "A.7.8", nis2, "NIS2-E.3", "partial"},  // Workplace security (clean desk/area)
		{iso, "A.7.12", nis2, "NIS2-E.3", "partial"}, // Cabling security
	}

	// ISO 27001:2022 ↔ BSI IT-Grundschutz bidirectional mappings.
	// BSI codes as stored in DB: BSI-ORP.1, BSI-ORP.2, BSI-DER.2.1, BSI-OPS.1.1.2, BSI-CON.3, BSI-NET.1.1, BSI-SYS.1.1.
	isoBSI := []entry{
		{iso, "A.5.1", bsi, "BSI-ORP.1", "equivalent"},     // Policies ↔ Organisation
		{iso, "A.5.2", bsi, "BSI-ORP.1", "partial"},        // IS roles ↔ Organisation
		{iso, "A.6.1", bsi, "BSI-ORP.2", "partial"},        // Screening ↔ Personnel
		{iso, "A.6.2", bsi, "BSI-ORP.2", "partial"},        // Terms ↔ Personnel
		{iso, "A.5.9", bsi, "BSI-OPS.1.1.2", "partial"},    // Asset inventory ↔ IT-Admin
		{iso, "A.8.8", bsi, "BSI-OPS.1.1.2", "equivalent"}, // Vuln mgmt/patching ↔ IT-Admin
		{iso, "A.5.37", bsi, "BSI-OPS.1.1.2", "partial"},   // Documented procedures ↔ IT-Admin
		{iso, "A.8.13", bsi, "BSI-CON.3", "equivalent"},    // Backup ↔ Data Backup Policy
		{iso, "A.8.24", bsi, "BSI-SYS.1.1", "informative"}, // Cryptography ↔ General Server
		{iso, "A.5.24", bsi, "BSI-DER.2.1", "equivalent"},  // Incident mgmt ↔ Incident Handling
		{iso, "A.5.15", bsi, "BSI-NET.1.1", "informative"}, // Access control policy ↔ Network Architecture
		{iso, "A.8.20", bsi, "BSI-NET.1.1", "equivalent"},  // Network security ↔ Network Architecture
	}

	seed := func(entries []entry) error {
		for _, e := range entries {
			if err := s.repo.SeedGlobalControlMapping(ctx, e.srcFW, e.srcCode, e.tgtFW, e.tgtCode, e.mappingType); err != nil {
				log.Warn().Err(err).
					Str("src", e.srcFW+"/"+e.srcCode).
					Str("tgt", e.tgtFW+"/"+e.tgtCode).
					Msg("seed framework mapping failed (non-critical)")
			}
			// Reverse direction
			if err := s.repo.SeedGlobalControlMapping(ctx, e.tgtFW, e.tgtCode, e.srcFW, e.srcCode, e.mappingType); err != nil {
				log.Warn().Err(err).
					Str("src", e.tgtFW+"/"+e.tgtCode).
					Str("tgt", e.srcFW+"/"+e.srcCode).
					Msg("seed reverse framework mapping failed (non-critical)")
			}
		}
		return nil
	}

	// CIS Controls v8 (IG1) ↔ ISO 27001:2022 bidirectional mappings.
	// CIS group codes as stored in DB (e.g. "CIS-1.1") mapped to ISO 27001:2022 control IDs.
	const cis = "CIS"

	isoCIS := []entry{
		// CIS 1 (Asset Inventory) ↔ ISO A.5.9 (Inventory of assets)
		{iso, "A.5.9", cis, "CIS-1.1", "equivalent"},
		{iso, "A.5.9", cis, "CIS-1.2", "partial"},
		// CIS 2 (Software Inventory) ↔ ISO A.5.9
		{iso, "A.5.9", cis, "CIS-2.1", "equivalent"},
		// CIS 3 (Data Protection) ↔ ISO A.5.12 (Classification) + A.7.10 (Media)
		{iso, "A.5.12", cis, "CIS-3.2", "equivalent"},
		{iso, "A.7.10", cis, "CIS-3.3", "equivalent"},
		// CIS 4 (Secure Configuration) ↔ ISO A.8.8 (Technical vulnerability mgmt / hardening)
		{iso, "A.8.8", cis, "CIS-4.1", "equivalent"},
		{iso, "A.8.8", cis, "CIS-4.4", "partial"},
		// CIS 5 (Account Management) ↔ ISO A.5.16 (Identity management)
		{iso, "A.5.16", cis, "CIS-5.1", "equivalent"},
		{iso, "A.5.16", cis, "CIS-5.3", "equivalent"},
		// CIS 6 (Access Control) ↔ ISO A.5.15 (Access control policy) + A.8.3 (Access restriction)
		{iso, "A.5.15", cis, "CIS-6.1", "equivalent"},
		{iso, "A.8.3", cis, "CIS-6.3", "equivalent"},
		// CIS 7 (Vulnerability Management) ↔ ISO A.8.8
		{iso, "A.8.8", cis, "CIS-7.1", "equivalent"},
		{iso, "A.8.8", cis, "CIS-7.2", "equivalent"},
		// CIS 8 (Audit Logs) ↔ ISO A.8.15 (Logging)
		{iso, "A.8.15", cis, "CIS-8.2", "equivalent"},
		{iso, "A.8.15", cis, "CIS-8.4", "equivalent"},
		// CIS 9 (Email/Web) ↔ ISO A.6.3 (Awareness training covers email/web threats)
		{iso, "A.6.3", cis, "CIS-9.3", "partial"},
		// CIS 10 (Malware) ↔ ISO A.8.7 (Protection against malware)
		{iso, "A.8.7", cis, "CIS-10.1", "equivalent"},
		// CIS 11 (Data Recovery) ↔ ISO A.8.13 (Information backup)
		{iso, "A.8.13", cis, "CIS-11.1", "equivalent"},
		{iso, "A.8.13", cis, "CIS-11.2", "equivalent"},
		{iso, "A.8.13", cis, "CIS-11.4", "partial"},
		// CIS 12 (Network Infrastructure) ↔ ISO A.8.20 (Network security)
		{iso, "A.8.20", cis, "CIS-12.1", "equivalent"},
		// CIS 13 (Network Monitoring) ↔ ISO A.8.15 (Logging)
		{iso, "A.8.15", cis, "CIS-13.1", "partial"},
		// CIS 14 (Security Awareness) ↔ ISO A.6.3 (IS awareness, education and training)
		{iso, "A.6.3", cis, "CIS-14.1", "equivalent"},
		{iso, "A.6.3", cis, "CIS-14.2", "equivalent"},
		{iso, "A.6.3", cis, "CIS-14.3", "partial"},
		// CIS 15 (Service Provider Management) ↔ ISO A.5.19 (IS in supplier relationships)
		{iso, "A.5.19", cis, "CIS-15.1", "equivalent"},
		{iso, "A.5.19", cis, "CIS-15.2", "equivalent"},
		// CIS 16 (Application Security) ↔ ISO A.8.26 (Application security requirements)
		{iso, "A.8.26", cis, "CIS-16.1", "equivalent"},
		{iso, "A.8.26", cis, "CIS-16.3", "partial"},
		// CIS 17 (Incident Response) ↔ ISO A.5.24 (Incident management planning)
		{iso, "A.5.24", cis, "CIS-17.1", "equivalent"},
		{iso, "A.5.24", cis, "CIS-17.2", "partial"},
		// CIS 18 (Penetration Testing) ↔ ISO A.8.8 (Vulnerability management)
		{iso, "A.8.8", cis, "CIS-18.2", "informative"},
	}

	if err := seed(isoNIS2); err != nil {
		return err
	}
	if err := seed(isoBSI); err != nil {
		return err
	}
	if err := seed(isoCIS); err != nil {
		return err
	}

	// S75: additional DACH-market mappings (seeded globally, no org-check needed).
	for _, p := range iso27001BSIMappings {
		if err := s.repo.SeedGlobalControlMapping(ctx, p.src, p.srcCode, p.tgt, p.tgtCode, p.mtype); err != nil {
			log.Warn().Err(err).Str("src", p.srcCode).Str("tgt", p.tgtCode).Msg("S75 ISO27001↔BSI seed failed")
		}
		if err := s.repo.SeedGlobalControlMapping(ctx, p.tgt, p.tgtCode, p.src, p.srcCode, p.mtype); err != nil {
			log.Warn().Err(err).Msg("S75 ISO27001↔BSI reverse seed failed")
		}
	}
	s75Pairs := append(append(append(append(append(append(
		nis2BSIExtendedMappings,
		dsgvoTOMNIS2Mappings...),
		dsgvoTOMBSIMappings...),
		cisISO27001Mappings...),
		cisBSIMappings...),
		tisaxBSIMappings...),
		tisaxDSGVOTOMMappings...)
	for _, p := range s75Pairs {
		if err := s.repo.SeedGlobalControlMapping(ctx, p.src, p.srcCode, p.tgt, p.tgtCode, p.mtype); err != nil {
			log.Warn().Err(err).Str("src", p.srcCode).Str("tgt", p.tgtCode).Msg("S75 mapping seed failed")
		}
		if err := s.repo.SeedGlobalControlMapping(ctx, p.tgt, p.tgtCode, p.src, p.srcCode, p.mtype); err != nil {
			log.Warn().Err(err).Msg("S75 mapping reverse seed failed")
		}
	}
	// S86: DER.4 ↔ ISO 27001:2022 + NIS2 + DORA cross-mappings
	for _, p := range der4CrossMappings {
		if err := s.repo.SeedGlobalControlMapping(ctx, p.src, p.srcCode, p.tgt, p.tgtCode, p.mtype); err != nil {
			log.Warn().Err(err).Str("src", p.srcCode).Str("tgt", p.tgtCode).Msg("S86 DER.4 mapping seed failed")
		}
		if err := s.repo.SeedGlobalControlMapping(ctx, p.tgt, p.tgtCode, p.src, p.srcCode, p.mtype); err != nil {
			log.Warn().Err(err).Msg("S86 DER.4 reverse mapping seed failed")
		}
	}
	return nil
}

// GetFramework returns a single framework by ID, enriched with catalog metadata where available.
func (s *Service) GetFramework(ctx context.Context, orgID, frameworkID string) (*Framework, error) {
	fw, err := s.repo.GetFramework(ctx, orgID, frameworkID)
	if err != nil {
		return nil, err
	}
	if p, ok := catalogRegistry[fw.Name]; ok {
		fw.CatalogEdition = p.Metadata().Edition
	}
	return fw, nil
}

// GetReadinessReport computes a full readiness report for a framework.
func (s *Service) GetReadinessReport(ctx context.Context, orgID, frameworkID string) (*ReadinessReport, error) {
	fw, err := s.repo.GetFramework(ctx, orgID, frameworkID)
	if err != nil {
		return nil, fmt.Errorf("get framework: %w", err)
	}

	controls, err := s.repo.ListControls(ctx, orgID, frameworkID)
	if err != nil {
		return nil, fmt.Errorf("list controls: %w", err)
	}

	evidenceCounts, err := s.repo.CountEvidenceByControl(ctx, orgID, frameworkID)
	if err != nil {
		return nil, fmt.Errorf("count evidence: %w", err)
	}

	report := ComputeReadinessReport(fw, controls, evidenceCounts)
	if fw.Name == "TISAX" {
		report.TISAXMaturity = ComputeTISAXMaturity(controls)
	}
	return report, nil
}

// GetGapAnalysis returns controls that are missing or at-risk evidence.
func (s *Service) GetGapAnalysis(ctx context.Context, orgID, frameworkID string) (*GapAnalysis, error) {
	controls, err := s.repo.ListControls(ctx, orgID, frameworkID)
	if err != nil {
		return nil, fmt.Errorf("list controls: %w", err)
	}

	evidenceCounts, err := s.repo.CountEvidenceByControl(ctx, orgID, frameworkID)
	if err != nil {
		return nil, fmt.Errorf("count evidence: %w", err)
	}

	// Expiring evidence: anything expiring in the next 30 days.
	threshold := time.Now().UTC().Add(30 * 24 * time.Hour)
	expiring, err := s.repo.GetExpiringEvidence(ctx, orgID, frameworkID, threshold)
	if err != nil {
		return nil, fmt.Errorf("get expiring evidence: %w", err)
	}

	// Build expiry map keyed by control UUID.
	expiryMap := make(map[string]*time.Time)
	for i := range expiring {
		expiryMap[expiring[i].ControlID] = expiring[i].ExpiresAt
	}

	analysis := &GapAnalysis{FrameworkID: frameworkID}
	for i := range controls {
		c := controls[i]
		count := evidenceCounts[c.ID]
		if count == 0 {
			analysis.Gaps = append(analysis.Gaps, ControlGap{
				Control: c,
				Reason:  "no_evidence",
			})
		} else if ea, ok := expiryMap[c.ID]; ok {
			analysis.Gaps = append(analysis.Gaps, ControlGap{
				Control:   c,
				Reason:    "evidence_expiring",
				ExpiresAt: ea,
			})
		}
	}

	return analysis, nil
}

// --- Controls ---

// ListControls returns all controls for a framework within an organisation.
func (s *Service) ListControls(ctx context.Context, orgID, frameworkID string) ([]Control, error) {
	controls, err := s.repo.ListControls(ctx, orgID, frameworkID)
	if err != nil {
		return nil, fmt.Errorf("list controls: %w", err)
	}

	// Enrich with evidence counts.
	counts, err := s.repo.CountEvidenceByControl(ctx, orgID, frameworkID)
	if err != nil {
		return nil, fmt.Errorf("count evidence for controls: %w", err)
	}

	for i := range controls {
		controls[i].EvidenceCount = counts[controls[i].ID]
		controls[i].Status = ResolveStatus(controls[i])
		if strings.HasPrefix(controls[i].ControlID, "DORA-") {
			if m, ok := DoraISO27001Mapping[controls[i].ControlID]; ok {
				controls[i].ISO27001Mapping = m
			}
		}
	}

	return controls, nil
}

// UpdateControl updates not_applicable, reason, manual_status, and optionally maturity_score on a control.
func (s *Service) UpdateControl(ctx context.Context, orgID, controlID string, input UpdateControlInput) (*Control, error) {
	if input.MaturityScore != nil && (*input.MaturityScore < 0 || *input.MaturityScore > 3) {
		return nil, fmt.Errorf("maturity_score must be between 0 and 3")
	}
	if err := s.repo.UpdateControl(ctx, orgID, controlID, input.NotApplicable, input.Reason, input.ManualStatus, input.Owner, input.MaturityScore, input.DueDate); err != nil {
		return nil, fmt.Errorf("update control: %w", err)
	}
	s.invalidateDashboardCache(ctx, orgID)
	ctrl, err := s.GetControl(ctx, orgID, controlID)
	if err != nil {
		return nil, err
	}
	if input.ManualStatus != "" || input.NotApplicable {
		s.triggerWebhook(ctx, orgID, "control.status_changed", map[string]any{
			"id":     ctrl.ID,
			"title":  ctrl.Title,
			"status": ctrl.Status,
			"org_id": orgID,
		})
		// Non-blocking: check if this update pushed the framework past a milestone.
		go s.checkFrameworkMilestone(ctx, orgID, ctrl.FrameworkID)
	}
	return ctrl, nil
}

// checkFrameworkMilestone computes the current readiness score for a framework
// and sends a one-time in-app notification when it crosses 60 %, 80 %, or 100 %.
// Deduplication key stored in user_notifications.module as "<frameworkID>:<threshold>".
func (s *Service) checkFrameworkMilestone(ctx context.Context, orgID, frameworkID string) {
	controls, err := s.repo.ListControls(ctx, orgID, frameworkID)
	if err != nil || len(controls) == 0 {
		return
	}

	var covered, partial, total int
	for _, c := range controls {
		status := ResolveStatus(c)
		if status == "not_applicable" {
			continue
		}
		total++
		switch status {
		case "covered", "implemented":
			covered++
		case "partial", "in_progress":
			partial++
		}
	}
	score := ReadinessScore(covered, partial, total)

	fw, err := s.repo.GetFramework(ctx, orgID, frameworkID)
	if err != nil {
		return
	}

	for _, threshold := range []int{60, 80, 100} {
		if int(score) < threshold {
			continue
		}
		dedupeKey := fmt.Sprintf("%s:%d", frameworkID, threshold)
		already, err := s.q.CountCKFrameworkMilestoneNotifs(ctx, db.CountCKFrameworkMilestoneNotifsParams{
			OrgID:     orgID,
			DedupeKey: dedupeKey,
		})
		if err != nil {
			// S13-18: bei Fehler defensiv abbrechen — sonst wuerden wir die
			// Milestone-Notification potenziell doppelt versenden.
			log.Warn().Err(err).
				Str("framework_id", frameworkID).Int("threshold", threshold).
				Msg("milestone dedupe lookup failed — skipping notification")
			continue
		}
		if already > 0 {
			continue
		}
		notify.Send(ctx, s.db, orgID,
			fmt.Sprintf("%d %% Compliance-Meilenstein erreicht", threshold),
			fmt.Sprintf("%s hat %d %% Umsetzungsgrad erreicht.", fw.Name, threshold),
			"framework_milestone",
			dedupeKey,
		)
	}
}

// FilterTISAXByProtectionLevel filters controls based on protection level.
// When protectionLevel != "very_high", controls with ControlID prefix "TISAX-15" are excluded.
func FilterTISAXByProtectionLevel(controls []Control, protectionLevel string) []Control {
	if protectionLevel == "very_high" {
		return controls
	}
	filtered := make([]Control, 0, len(controls))
	for _, c := range controls {
		if !strings.HasPrefix(c.ControlID, "TISAX-15") {
			filtered = append(filtered, c)
		}
	}
	return filtered
}

// BuildTISAXGapAnalysis constructs a TISAXGapAnalysis from a control slice.
func BuildTISAXGapAnalysis(frameworkID string, controls []Control) *TISAXGapAnalysis {
	analysis := &TISAXGapAnalysis{
		FrameworkID: frameworkID,
		TargetScore: 3,
	}
	for _, c := range controls {
		if c.MaturityScore < 3 {
			analysis.Gaps = append(analysis.Gaps, TISAXControlGap{
				Control:      c,
				MaturityGap:  3 - c.MaturityScore,
				CurrentScore: c.MaturityScore,
			})
		}
	}
	return analysis
}

// ComputeTISAXMaturity computes the TISAX maturity summary from a set of controls.
// Controls are grouped by Domain; per-domain stats (avg, total, fully_mature, color) are computed.
// Chapters are sorted by domain name for stable output.
func ComputeTISAXMaturity(controls []Control) *TISAXMaturitySummary {
	if len(controls) == 0 {
		return &TISAXMaturitySummary{
			AvgScore:         0.0,
			ByChapter:        []ChapterMaturity{},
			ReadinessPercent: 0.0,
		}
	}

	// Group by domain.
	type domainAcc struct {
		sum         int
		total       int
		fullyMature int
	}
	domainMap := make(map[string]*domainAcc)
	var totalSum, totalCount int
	for _, c := range controls {
		acc := domainMap[c.Domain]
		if acc == nil {
			acc = &domainAcc{}
			domainMap[c.Domain] = acc
		}
		acc.sum += c.MaturityScore
		acc.total++
		if c.MaturityScore == 3 {
			acc.fullyMature++
		}
		totalSum += c.MaturityScore
		totalCount++
	}

	// Sort domain names.
	domains := make([]string, 0, len(domainMap))
	for d := range domainMap {
		domains = append(domains, d)
	}
	sort.Strings(domains)

	chapters := make([]ChapterMaturity, 0, len(domains))
	for _, d := range domains {
		acc := domainMap[d]
		avg := float64(acc.sum) / float64(acc.total)
		color := "red"
		if avg >= 2.5 {
			color = "green"
		} else if avg >= 1.5 {
			color = "yellow"
		}
		chapters = append(chapters, ChapterMaturity{
			Domain:        d,
			AvgScore:      avg,
			TotalControls: acc.total,
			FullyMature:   acc.fullyMature,
			Color:         color,
		})
	}

	var avgScore float64
	if totalCount > 0 {
		avgScore = float64(totalSum) / float64(totalCount)
	}

	return &TISAXMaturitySummary{
		AvgScore:         avgScore,
		ByChapter:        chapters,
		ReadinessPercent: (avgScore / 3.0) * 100.0,
	}
}

// ListTISAXControls returns controls for a TISAX framework, filtered by protection level.
// When protectionLevel != "very_high", controls with ControlID prefix "TISAX-15" are excluded.
func (s *Service) ListTISAXControls(ctx context.Context, orgID, frameworkID, protectionLevel string) ([]Control, error) {
	controls, err := s.ListControls(ctx, orgID, frameworkID)
	if err != nil {
		return nil, fmt.Errorf("list tisax controls: %w", err)
	}
	return FilterTISAXByProtectionLevel(controls, protectionLevel), nil
}

// GetTISAXGapAnalysis returns TISAX controls that have not yet reached full maturity (score < 3).
func (s *Service) GetTISAXGapAnalysis(ctx context.Context, orgID, frameworkID string) (*TISAXGapAnalysis, error) {
	controls, err := s.ListControls(ctx, orgID, frameworkID)
	if err != nil {
		return nil, fmt.Errorf("get tisax gap analysis: %w", err)
	}
	return BuildTISAXGapAnalysis(frameworkID, controls), nil
}

// TisaxToISO27001Mappings is the static TISAX → ISO 27001:2022 control mapping table.
// All target IDs use 2022 Annex A numbering (A.5.x–A.8.x).
var TisaxToISO27001Mappings = map[string]string{
	// TISAX 1.x — Information Security Policies
	"TISAX-1.1.1": "A.5.1", "TISAX-1.1.2": "A.5.1", "TISAX-1.1.3": "A.5.2",
	// TISAX 2.x — Organization of Information Security
	"TISAX-2.1.1": "A.5.2", "TISAX-2.1.3": "A.5.8", "TISAX-2.1.4": "A.8.1",
	// TISAX 3.x — Human Resource Security
	"TISAX-3.1.1": "A.6.1", "TISAX-3.1.2": "A.6.3", "TISAX-3.1.3": "A.6.4", "TISAX-3.1.4": "A.6.5",
	// TISAX 4.x — Asset Management
	"TISAX-4.1.1": "A.5.9", "TISAX-4.1.2": "A.5.9", "TISAX-4.1.3": "A.5.12", "TISAX-4.1.4": "A.5.13", "TISAX-4.1.5": "A.7.10",
	// TISAX 5.x — Access Control
	"TISAX-5.1.1": "A.5.15", "TISAX-5.1.2": "A.5.16", "TISAX-5.1.3": "A.8.2", "TISAX-5.1.4": "A.8.5", "TISAX-5.1.5": "A.5.15",
	// TISAX 6.x — Cryptography
	"TISAX-6.1.1": "A.8.24", "TISAX-6.1.2": "A.8.24", "TISAX-6.1.3": "A.8.24",
	// TISAX 7.x — Physical and Environmental Security
	"TISAX-7.1.1": "A.7.1", "TISAX-7.1.2": "A.7.2", "TISAX-7.1.3": "A.7.8", "TISAX-7.1.4": "A.7.7",
	// TISAX 8.x — Operations Security
	"TISAX-8.1.2": "A.8.32", "TISAX-8.1.3": "A.8.7", "TISAX-8.1.4": "A.8.13", "TISAX-8.1.5": "A.8.15", "TISAX-8.1.6": "A.8.8",
	// TISAX 9.x — Communications Security
	"TISAX-9.1.1": "A.8.20", "TISAX-9.1.2": "A.5.14", "TISAX-9.1.3": "A.6.6",
	// TISAX 11.x — Supplier Relationships
	"TISAX-11.1.1": "A.5.19", "TISAX-11.1.2": "A.5.20", "TISAX-11.1.3": "A.5.22",
	// TISAX 12.x — Incident Management
	"TISAX-12.1.1": "A.5.24", "TISAX-12.1.2": "A.6.8", "TISAX-12.1.4": "A.5.27",
	// TISAX 13.x — Business Continuity Management
	"TISAX-13.1.1": "A.5.29", "TISAX-13.1.2": "A.5.29",
	// TISAX 14.x — Compliance
	"TISAX-14.1.1": "A.5.31", "TISAX-14.1.2": "A.5.36",
}

// SeedTISAXMappings idempotently seeds the static TISAX → ISO 27001 mappings into ck_framework_mappings.
// Returns nil if either framework is not yet enabled.
func (s *Service) SeedTISAXMappings(ctx context.Context, orgID string) error {
	tisaxFW, err := s.repo.FindFrameworkByName(ctx, orgID, "TISAX")
	if err != nil {
		return fmt.Errorf("find TISAX framework: %w", err)
	}
	if tisaxFW == nil {
		return nil // TISAX not enabled yet — skip silently
	}

	isoFW, err := s.repo.FindFrameworkByName(ctx, orgID, "ISO27001")
	if err != nil {
		return fmt.Errorf("find ISO27001 framework: %w", err)
	}
	if isoFW == nil {
		return nil // ISO27001 not enabled yet — skip silently
	}

	// Build lookup maps: controlID string → UUID string
	tisaxControls, err := s.repo.ListControls(ctx, orgID, tisaxFW.ID)
	if err != nil {
		return fmt.Errorf("list TISAX controls for seed: %w", err)
	}
	isoControls, err := s.repo.ListControls(ctx, orgID, isoFW.ID)
	if err != nil {
		return fmt.Errorf("list ISO27001 controls for seed: %w", err)
	}

	tisaxByControlID := make(map[string]string, len(tisaxControls))
	for _, c := range tisaxControls {
		tisaxByControlID[c.ControlID] = c.ID
	}
	isoByControlID := make(map[string]string, len(isoControls))
	for _, c := range isoControls {
		isoByControlID[c.ControlID] = c.ID
	}

	for tisaxID, isoID := range TisaxToISO27001Mappings {
		tisaxUUID, ok1 := tisaxByControlID[tisaxID]
		isoUUID, ok2 := isoByControlID[isoID]
		if !ok1 || !ok2 {
			continue // control not found in DB — skip silently
		}
		if _, err := s.repo.CreateMapping(ctx, orgID, tisaxUUID, isoUUID); err != nil {
			log.Warn().Err(err).Str("tisax", tisaxID).Str("iso", isoID).Msg("seed mapping failed")
		}
	}
	return nil
}

// GetTISAXCoverageByISO computes, for each TISAX control, whether the mapped ISO 27001 control is covered.
// A control is covered when its manual_status == "implemented" OR evidence_count >= 1.
func (s *Service) GetTISAXCoverageByISO(ctx context.Context, orgID, tisaxFrameworkID string) ([]MappingResult, error) {
	tisaxControls, err := s.ListControls(ctx, orgID, tisaxFrameworkID)
	if err != nil {
		return nil, fmt.Errorf("list TISAX controls: %w", err)
	}

	// Find ISO27001 framework — if not enabled, return all covered=false.
	isoFW, err := s.repo.FindFrameworkByName(ctx, orgID, "ISO27001")
	if err != nil {
		return nil, fmt.Errorf("find ISO27001 framework: %w", err)
	}

	var isoControls []Control
	var evidenceCounts map[string]int
	if isoFW != nil {
		isoControls, err = s.ListControls(ctx, orgID, isoFW.ID)
		if err != nil {
			return nil, fmt.Errorf("list ISO27001 controls: %w", err)
		}
		evidenceCounts, err = s.repo.CountEvidenceByControl(ctx, orgID, isoFW.ID)
		if err != nil {
			return nil, fmt.Errorf("count ISO27001 evidence: %w", err)
		}
	}

	// Build lookup: ISO control UUID → Control
	isoByUUID := make(map[string]Control, len(isoControls))
	for _, c := range isoControls {
		isoByUUID[c.ID] = c
	}

	// Load mappings for all TISAX control UUIDs.
	tisaxUUIDs := make([]string, 0, len(tisaxControls))
	for _, c := range tisaxControls {
		tisaxUUIDs = append(tisaxUUIDs, c.ID)
	}
	mappings, err := s.repo.GetMappingsBySourceControlIDs(ctx, orgID, tisaxUUIDs)
	if err != nil {
		return nil, fmt.Errorf("get framework mappings: %w", err)
	}

	results := make([]MappingResult, 0, len(tisaxControls))
	for _, tc := range tisaxControls {
		mr := MappingResult{
			TISAXControlID:    tc.ControlID,
			TISAXControlTitle: tc.Title,
		}

		if mapping, hasMapped := mappings[tc.ID]; hasMapped {
			if iso, hasISO := isoByUUID[mapping.TargetControlID]; hasISO {
				mr.ISOControlID = iso.ControlID
				mr.ISOControlTitle = iso.Title
				// Covered if implemented or has evidence.
				mr.Covered = iso.ManualStatus == "implemented" || evidenceCounts[iso.ID] >= 1
			}
		}

		results = append(results, mr)
	}
	return results, nil
}

// GetTISAXGapsAfterISO returns TISAX controls that are NOT covered by the mapped ISO 27001 control.
func (s *Service) GetTISAXGapsAfterISO(ctx context.Context, orgID, tisaxFrameworkID string) ([]Control, error) {
	results, err := s.GetTISAXCoverageByISO(ctx, orgID, tisaxFrameworkID)
	if err != nil {
		return nil, err
	}

	// Load all TISAX controls to return full Control objects.
	allControls, err := s.ListControls(ctx, orgID, tisaxFrameworkID)
	if err != nil {
		return nil, fmt.Errorf("list tisax controls for gap filter: %w", err)
	}
	controlByID := make(map[string]Control, len(allControls))
	for _, c := range allControls {
		controlByID[c.ControlID] = c
	}

	var gaps []Control
	for _, r := range results {
		if !r.Covered {
			if c, ok := controlByID[r.TISAXControlID]; ok {
				gaps = append(gaps, c)
			}
		}
	}
	return gaps, nil
}

// FindFrameworkByName returns a framework by name for an organisation, or nil if not found.
func (s *Service) FindFrameworkByName(ctx context.Context, orgID, name string) (*Framework, error) {
	return s.repo.FindFrameworkByName(ctx, orgID, name)
}

// ListFrameworkMappings returns all framework mappings for an organisation.
func (s *Service) ListFrameworkMappings(ctx context.Context, orgID string) ([]FrameworkMapping, error) {
	return s.repo.ListMappingsByOrg(ctx, orgID)
}

// DeleteFrameworkMapping removes a framework mapping by ID within an organisation.
func (s *Service) DeleteFrameworkMapping(ctx context.Context, orgID, mappingID string) error {
	return s.repo.DeleteMapping(ctx, orgID, mappingID)
}

// GetControl returns a single control by its UUID.
func (s *Service) GetControl(ctx context.Context, orgID, controlID string) (*Control, error) {
	c, err := s.repo.GetControl(ctx, orgID, controlID)
	if err != nil {
		return nil, fmt.Errorf("get control: %w", err)
	}
	if strings.HasPrefix(c.ControlID, "DORA-") {
		if m, ok := DoraISO27001Mapping[c.ControlID]; ok {
			c.ISO27001Mapping = m
		}
	}
	return c, nil
}

// --- Control Tasks ---

func (s *Service) ListControlTasks(ctx context.Context, orgID, controlID string) ([]ControlTask, error) {
	tasks, err := s.repo.ListControlTasks(ctx, orgID, controlID)
	if err != nil {
		return nil, err
	}
	if tasks == nil {
		tasks = []ControlTask{}
	}
	return tasks, nil
}

func (s *Service) CreateControlTask(ctx context.Context, orgID, controlID string, in CreateControlTaskInput) (*ControlTask, error) {
	return s.repo.CreateControlTask(ctx, orgID, controlID, in)
}

func (s *Service) UpdateControlTask(ctx context.Context, orgID, controlID, taskID string, in UpdateControlTaskInput) (*ControlTask, error) {
	return s.repo.UpdateControlTask(ctx, orgID, controlID, taskID, in)
}

func (s *Service) DeleteControlTask(ctx context.Context, orgID, controlID, taskID string) error {
	return s.repo.DeleteControlTask(ctx, orgID, controlID, taskID)
}

// SeedDORAMappings idempotently seeds the DORA ↔ ISO 27001 cross-framework mappings
// into ck_framework_control_mappings. S37-2.
func (s *Service) SeedDORAMappings(ctx context.Context, orgID string) error {
	doraFW, err := s.repo.FindFrameworkByName(ctx, orgID, "DORA")
	if err != nil {
		return fmt.Errorf("find DORA framework: %w", err)
	}
	if doraFW == nil {
		return nil // DORA not enabled yet — skip silently
	}

	isoFW, err := s.repo.FindFrameworkByName(ctx, orgID, "ISO27001")
	if err != nil {
		return fmt.Errorf("find ISO27001 framework: %w", err)
	}
	if isoFW == nil {
		return nil // ISO27001 not enabled yet — skip silently
	}

	DoraControls, err := s.repo.ListControls(ctx, orgID, doraFW.ID)
	if err != nil {
		return fmt.Errorf("list DORA controls for seed: %w", err)
	}
	isoControls, err := s.repo.ListControls(ctx, orgID, isoFW.ID)
	if err != nil {
		return fmt.Errorf("list ISO27001 controls for seed: %w", err)
	}

	doraByControlID := make(map[string]string, len(DoraControls))
	for _, c := range DoraControls {
		doraByControlID[c.ControlID] = c.ID
	}
	isoByControlID := make(map[string]string, len(isoControls))
	for _, c := range isoControls {
		isoByControlID[c.ControlID] = c.ID
	}

	// DoraISO27001Mapping maps DORA control IDs → comma-separated ISO 27001 Annex A IDs.
	// Expand comma-separated entries into individual mappings.
	for doraCode, isoRaw := range DoraISO27001Mapping {
		doraUUID, ok := doraByControlID[doraCode]
		if !ok {
			continue
		}
		for _, isoCode := range strings.Split(isoRaw, ",") {
			isoCode = strings.TrimSpace(isoCode)
			isoUUID, ok2 := isoByControlID[isoCode]
			if !ok2 {
				continue
			}
			if _, err := s.repo.CreateMapping(ctx, orgID, doraUUID, isoUUID); err != nil {
				log.Warn().Err(err).Str("dora", doraCode).Str("iso", isoCode).Msg("seed DORA mapping failed")
			}
		}
	}
	return nil
}
