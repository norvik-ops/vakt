// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package policy

// One source for the implementation status of a SoA entry.
//
// ck_soa_entries.implementation_status used to be a second, manually kept
// status next to the control status. The same control then read "In
// Bearbeitung" on the control page and "Nicht begonnen" in the SoA. The SoA
// now shows the status of its control while the version is a draft. Only on
// approval is the status written into ck_soa_entries, as a frozen snapshot of
// the approved document. The column stays (no migration); it is the snapshot
// store, not a second place to edit the status.

// Values of SoADedicatedEntry.StatusSource.
const (
	// SoAStatusSourceControl: read live from the linked ck_control (draft).
	SoAStatusSourceControl = "control"
	// SoAStatusSourceSnapshot: frozen at approval (approved version).
	SoAStatusSourceSnapshot = "snapshot"
	// SoAStatusSourceSoA: no control linked; the stored ck_soa_entries value.
	SoAStatusSourceSoA = "soa"
)

// soaControlLink is the control a SoA entry takes its status from, with the
// three inputs of ResolveStatus.
type soaControlLink struct {
	ID            string
	FrameworkID   string
	Title         string
	NotApplicable bool
	ManualStatus  string
	EvidenceCount int
}

// SoAStatusFromControl maps the effective control status (ResolveStatus, the
// rule of the control page and the framework view) to the SoA vocabulary that
// the ck_soa_entries CHECK accepts:
//
//	implemented / covered    -> implemented  (counts fully)
//	in_progress / partial    -> partial      (counts half)
//	missing / not_applicable -> not_started
//
// "planned" has no control equivalent; it only survives in snapshots that were
// approved before this rule.
func SoAStatusFromControl(notApplicable bool, manualStatus string, evidenceCount int) string {
	switch ResolveStatus(Control{NotApplicable: notApplicable, ManualStatus: manualStatus, EvidenceCount: evidenceCount}) {
	case "implemented", "covered":
		return "implemented"
	case "in_progress", "partial", "partially_implemented":
		return "partial"
	default:
		return "not_started"
	}
}

// resolveSoAEntry sets ImplementationStatus, ControlName and StatusSource of e.
//
//   - approved version: the stored values stay untouched (snapshot);
//   - draft with a linked control: status and title of the control;
//   - draft without a control: the stored values (nothing else to read).
//
// The linked control is reported in every case, so the page can link to it.
func resolveSoAEntry(e *SoADedicatedEntry, versionApproved bool, link *soaControlLink) {
	if link != nil {
		id, fw := link.ID, link.FrameworkID
		e.LinkedControlID = &id
		e.LinkedFrameworkID = &fw
	}
	switch {
	case versionApproved:
		e.StatusSource = SoAStatusSourceSnapshot
	case link != nil:
		e.StatusSource = SoAStatusSourceControl
		e.ImplementationStatus = SoAStatusFromControl(link.NotApplicable, link.ManualStatus, link.EvidenceCount)
		if link.Title != "" {
			e.ControlName = link.Title
		}
	default:
		e.StatusSource = SoAStatusSourceSoA
	}
}

// summarizeSoA counts resolved entries. ImplementationPct follows the shared
// counting rule (ReadinessScore): implemented counts fully, partial half.
func summarizeSoA(entries []SoADedicatedEntry) SoASummary {
	var s SoASummary
	for _, e := range entries {
		if !e.Applicable {
			s.ExcludedCount++
			continue
		}
		s.ApplicableCount++
		switch e.ImplementationStatus {
		case "implemented":
			s.ImplementedCount++
		case "partial":
			s.PartialCount++
		case "planned":
			s.PlannedCount++
		default:
			s.NotStartedCount++
		}
	}
	s.ImplementationPct = ReadinessScore(s.ImplementedCount, s.PartialCount, s.ApplicableCount)
	return s
}
