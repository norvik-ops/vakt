// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package secvitals

import (
	"context"
	"fmt"
	"time"
)

// LinkResilienceTestAsEvidence creates an evidence record on a DORA control
// from a resilience test result (S40-1). Returns the created evidence.
func (s *Service) LinkResilienceTestAsEvidence(ctx context.Context, orgID, testID, controlID, userID string) (*Evidence, error) {
	test, err := s.repo.GetResilienceTest(ctx, orgID, testID)
	if err != nil {
		return nil, fmt.Errorf("get resilience test: %w", err)
	}

	// Build a human-readable summary for the evidence title/description.
	title := fmt.Sprintf("DORA Resilienztest: %s (%s)", test.Type, test.TestDate.Format("02.01.2006"))
	description := fmt.Sprintf("Resilienztest vom %s. Typ: %s. Abhilfestatus: %s.",
		test.TestDate.Format("02.01.2006"), test.Type, test.RemediationStatus)
	if test.Provider != "" {
		description += fmt.Sprintf(" Durchgeführt von: %s.", test.Provider)
	}
	if test.Summary != "" {
		description += " Zusammenfassung: " + test.Summary
	}

	// Set expiry to +1 year from test date (DORA annual retesting requirement).
	expiry := test.TestDate.Add(365 * 24 * time.Hour)

	ev, err := s.AddEvidence(ctx, orgID, controlID, userID, AddEvidenceInput{
		Title:       title,
		Description: description,
		Source:      "manual",
		ExpiresAt:   &expiry,
	})
	if err != nil {
		return nil, fmt.Errorf("add evidence from resilience test: %w", err)
	}
	return ev, nil
}
