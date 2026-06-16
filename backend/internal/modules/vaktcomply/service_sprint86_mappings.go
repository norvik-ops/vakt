// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S86-4: DER.4 ↔ ISO 27001:2022 + NIS2 + DORA cross-mappings

package vaktcomply

// der4CrossMappings seeds DER.4 Notfallmanagement ↔ ISO 27001:2022 A.17/A.5.29 / NIS2 / DORA.
// All ISO codes use the 2022 numbering (A.5.x, A.8.x) — no A.9–A.18 legacy codes.
var der4CrossMappings = []struct {
	src, srcCode, tgt, tgtCode, mtype string
}{
	// DER.4 ↔ ISO 27001:2022
	{"BSI", "BSI-DER.4.A1", "ISO27001", "A.5.29", "equivalent"},  // Notfallhandbuch = Kontinuitätsverfahren
	{"BSI", "BSI-DER.4.A4", "ISO27001", "A.5.29", "equivalent"},  // BIA = Planung ICT-Readiness
	{"BSI", "BSI-DER.4.A4", "ISO27001", "A.8.13", "informative"}, // BIA → Backup-Informationen
	{"BSI", "BSI-DER.4.A5", "ISO27001", "A.5.29", "equivalent"},  // Notfallkonzept = Umsetzung
	{"BSI", "BSI-DER.4.A5", "ISO27001", "A.8.14", "informative"}, // WAP → Redundanz IT
	{"BSI", "BSI-DER.4.A6", "ISO27001", "A.5.30", "equivalent"},  // Übungen = ICT readiness for BC
	{"BSI", "BSI-DER.4.A8", "ISO27001", "A.5.29", "informative"}, // Alarmierungsplan

	// DER.4 ↔ NIS2
	{"BSI", "BSI-DER.4.A1", "NIS2", "NIS2-C.1", "equivalent"}, // ICT readiness / BCM
	{"BSI", "BSI-DER.4.A4", "NIS2", "NIS2-B.5", "equivalent"}, // Notfallmanagementplan
	{"BSI", "BSI-DER.4.A5", "NIS2", "NIS2-C.1", "informative"},

	// DER.4 ↔ DORA (Finanzsektor)
	{"BSI", "BSI-DER.4.A5", "DORA", "DORA-S.3", "informative"}, // ICT Recovery Plans
	{"BSI", "BSI-DER.4.A6", "DORA", "DORA-S.4", "informative"}, // Testing
}
