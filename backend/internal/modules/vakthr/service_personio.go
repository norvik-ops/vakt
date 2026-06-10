// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vakthr

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

// TriggerPersonioOffboarding is called when Personio sends an employee.departed webhook.
// It looks up the employee by personio_employee_id, creates one if missing, sets departure_date,
// and starts the standard offboarding checklist.
// DSGVO: no PII from the Personio payload is persisted — only personioEmployeeID and departureDate.
func (s *Service) TriggerPersonioOffboarding(ctx context.Context, orgID string, personioEmployeeID int, departureDate time.Time) error {
	employeeID, created, err := s.repo.UpsertEmployeeByPersonioID(ctx, orgID, personioEmployeeID, departureDate)
	if err != nil {
		return fmt.Errorf("upsert employee by personio_id: %w", err)
	}

	if created {
		log.Info().Str("org_id", orgID).Int("personio_id", personioEmployeeID).
			Msg("vakthr: created placeholder employee from Personio webhook")
	}

	// Start offboarding checklist (system actor — no user behind this)
	systemActor := Actor{OrgID: orgID, UserID: "", UserEmail: "personio-webhook", IPAddress: ""}
	run, err := s.startTypedRun(ctx, orgID, employeeID, "offboarding")
	if err != nil {
		return fmt.Errorf("start offboarding checklist for personio employee: %w", err)
	}

	if err := s.repo.SetEmployeeStatus(ctx, orgID, employeeID, "offboarding"); err != nil {
		log.Error().Err(err).Str("employee_id", employeeID).Msg("vakthr: set employee status to offboarding")
	}

	s.audit(ctx, systemActor, "start_offboarding_personio", "hr/checklist_run", run.ID,
		fmt.Sprintf("personio_employee_id=%d", personioEmployeeID))

	return nil
}
