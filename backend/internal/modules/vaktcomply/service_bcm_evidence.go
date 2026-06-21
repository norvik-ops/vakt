// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S86-4: Asynq-Job comply:bcm_evidence_sync
// Automatically creates/updates evidence entries for DER.4 controls based on
// BIA data, recovery plans, emergency contacts, and BCP tests.

package vaktcomply

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
)

const TaskBCMEvidenceSync = "comply:bcm_evidence_sync"

// SyncBCMEvidence creates evidence for DER.4 controls when the corresponding
// BCM data is present for the organisation.
func (s *Service) SyncBCMEvidence(ctx context.Context, orgID string) error {
	// DER.4.A4 — BIA erstellt (if ≥1 critical process)
	highCount, err := s.BCM.CountHighCriticalityBIAProcesses(ctx, orgID)
	if err != nil {
		return fmt.Errorf("bcm_evidence_sync: count bia processes: %w", err)
	}
	if highCount > 0 {
		if err := s.ensureBCMEvidence(ctx, orgID, "BSI-DER.4.A4",
			"BIA: Kritische Prozesse dokumentiert",
			fmt.Sprintf("%d kritische Geschäftsprozesse in BIA erfasst (BSI-200-4 DER.4.A4)", highCount),
		); err != nil {
			log.Warn().Err(err).Str("org_id", orgID).Str("control", "DER.4.A4").Msg("bcm_evidence_sync")
		}
	}

	// DER.4.A5 — Notfallkonzept vorhanden (if ≥1 active WAP)
	activeWAPs, err := s.BCM.CountRecoveryPlansActive(ctx, orgID)
	if err != nil {
		return fmt.Errorf("bcm_evidence_sync: count active waps: %w", err)
	}
	if activeWAPs > 0 {
		if err := s.ensureBCMEvidence(ctx, orgID, "BSI-DER.4.A5",
			"Wiederanlaufplan (WAP) vorhanden",
			fmt.Sprintf("%d aktive Wiederanlaufpläne dokumentiert (BSI-200-4 DER.4.A5)", activeWAPs),
		); err != nil {
			log.Warn().Err(err).Str("org_id", orgID).Str("control", "DER.4.A5").Msg("bcm_evidence_sync")
		}
	}

	// DER.4.A6 — Übung durchgeführt (if ≥1 tested WAP in last 12 months)
	testedCount, err := s.BCM.CountRecoveryPlansTested(ctx, orgID)
	if err != nil {
		return fmt.Errorf("bcm_evidence_sync: count tested waps: %w", err)
	}
	if testedCount > 0 {
		if err := s.ensureBCMEvidence(ctx, orgID, "BSI-DER.4.A6",
			"Notfallübung dokumentiert",
			fmt.Sprintf("%d Wiederanlaufpläne in den letzten 12 Monaten getestet (BSI-200-4 DER.4.A6)", testedCount),
		); err != nil {
			log.Warn().Err(err).Str("org_id", orgID).Str("control", "DER.4.A6").Msg("bcm_evidence_sync")
		}
	}

	// DER.4.A8 — Alarmierungsplan/Kontaktverzeichnis vorhanden (if ≥1 contact)
	contactCount, err := s.BCM.CountEmergencyContacts(ctx, orgID)
	if err != nil {
		return fmt.Errorf("bcm_evidence_sync: count emergency contacts: %w", err)
	}
	if contactCount > 0 {
		if err := s.ensureBCMEvidence(ctx, orgID, "BSI-DER.4.A8",
			"Alarmierungsplan/Kontaktverzeichnis gepflegt",
			fmt.Sprintf("%d Notfallkontakte im Alarmierungsplan erfasst (BSI-200-4 DER.4.A8)", contactCount),
		); err != nil {
			log.Warn().Err(err).Str("org_id", orgID).Str("control", "DER.4.A8").Msg("bcm_evidence_sync")
		}
	}

	return nil
}

// ensureBCMEvidence idempotently creates an evidence entry for the given BSI control.
// It uses AddCollectorEvidence which is upsert-safe.
func (s *Service) ensureBCMEvidence(ctx context.Context, orgID, controlCode, title, description string) error {
	controlID, err := s.repo.FindControlByCode(ctx, orgID, controlCode)
	if err != nil || controlID == "" {
		// Control not found — BSI framework may not be enabled; skip silently
		return nil
	}
	payload := []byte(fmt.Sprintf(`{"source":"bcm_evidence_sync","control_code":%q,"description":%q}`, controlCode, description))
	_, err = s.repo.AddCollectorEvidence(ctx, orgID, controlID, "", "automated", title, payload)
	return err
}
