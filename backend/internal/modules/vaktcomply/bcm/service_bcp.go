// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package bcm

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ESK-12: Fehler der BCP-Plan-Eingabe. ErrSchutzbedarfsklasseInvalid nennt
// ausdruecklich den Wertebereich des CHECK aus Migration 216 — dieselbe Menge
// (1,2,3), damit die Ablehnung vor der DB passiert und einen Namen hat.
var (
	ErrRTOOutOfRange              = errors.New("rto_hours must be between 1 and 8760")
	ErrRPOOutOfRange              = errors.New("rpo_hours must be between 1 and 8760")
	ErrSchutzbedarfsklasseInvalid = errors.New("schutzbedarfsklasse must be 1, 2 or 3")
)

// CreateBCPPlan creates a new BCP plan for the organisation.
func (s *Service) CreateBCPPlan(ctx context.Context, orgID string, in CreateBCPPlanInput) (BCPPlan, error) {
	if err := validateBCPPlanTargets(in.RTOHours, in.RPOHours, in.Schutzbedarfsklasse); err != nil {
		return BCPPlan{}, err
	}
	return s.repo.CreateBCPPlan(ctx, orgID, in)
}

// ListBCPPlans returns all BCP plans for the organisation.
func (s *Service) ListBCPPlans(ctx context.Context, orgID string) ([]BCPPlan, error) {
	return s.repo.ListBCPPlans(ctx, orgID)
}

// GetBCPPlan returns a single BCP plan by ID.
func (s *Service) GetBCPPlan(ctx context.Context, orgID, id string) (BCPPlan, error) {
	return s.repo.GetBCPPlan(ctx, orgID, id)
}

// UpdateBCPPlan updates an existing BCP plan.
//
// Hier steht ABSICHTLICH kein validateBCPPlanTargets: das PATCH mergt
// (UpdateBCPPlanInput), also ist der Eingabezustand nicht der Zielzustand. Ein
// PATCH mit nur rpo_hours=8 gegen einen Plan mit rto_hours=4 saehe hier
// unauffaellig aus und wuerde die Invariante rpo <= rto trotzdem brechen.
// Geprueft wird deshalb im Repository auf dem gemergten Zustand, innerhalb
// derselben Transaktion und Zeilensperre, die auch schreibt.
func (s *Service) UpdateBCPPlan(ctx context.Context, orgID, id string, in UpdateBCPPlanInput) (BCPPlan, error) {
	return s.repo.UpdateBCPPlan(ctx, orgID, id, in)
}

// validateBCPPlanTargets prueft die BSI-200-4-Angaben eines BCP-Plans.
//
// Die Bereichspruefung liegt zusaetzlich in den `validate`-Tags der Input-Structs
// (der Handler ruft sie). Sie steht hier ein zweites Mal, weil der Service auch
// ohne Handler aufrufbar ist und die Schutzbedarfsklasse sonst nur noch vom
// DB-CHECK gefangen wuerde — als 500er statt als benannter Eingabefehler.
//
// ErrRPOExceedsRTO ist dieselbe Domaeneninvariante, die validateBIAProcess fuer
// BIA-Prozesse durchsetzt: man kann nicht weniger Daten verlieren wollen, als
// man Zeit zum Wiederanlauf hat. Verglichen wird nur, wenn BEIDE Werte gesetzt
// sind — gegen `null` laesst sich nichts pruefen, und das ist kein Fehler,
// sondern der Zustand "noch nicht festgelegt".
func validateBCPPlanTargets(rtoHours, rpoHours, schutzbedarfsklasse *int) error {
	if rtoHours != nil && (*rtoHours < 1 || *rtoHours > BCPPlanMaxRTOHours) {
		return fmt.Errorf("%w: rto_hours=%d", ErrRTOOutOfRange, *rtoHours)
	}
	if rpoHours != nil && (*rpoHours < 1 || *rpoHours > BCPPlanMaxRTOHours) {
		return fmt.Errorf("%w: rpo_hours=%d", ErrRPOOutOfRange, *rpoHours)
	}
	if schutzbedarfsklasse != nil && (*schutzbedarfsklasse < 1 || *schutzbedarfsklasse > 3) {
		return fmt.Errorf("%w: schutzbedarfsklasse=%d", ErrSchutzbedarfsklasseInvalid, *schutzbedarfsklasse)
	}
	if rtoHours != nil && rpoHours != nil && *rpoHours > *rtoHours {
		return ErrRPOExceedsRTO
	}
	return nil
}

// DeleteBCPPlan removes a BCP plan.
func (s *Service) DeleteBCPPlan(ctx context.Context, orgID, id string) error {
	return s.repo.DeleteBCPPlan(ctx, orgID, id)
}

// AddBCPTest logs a new test result for the given plan.
// It verifies the plan belongs to the organisation before inserting.
func (s *Service) AddBCPTest(ctx context.Context, orgID, planID string, in CreateBCPTestInput) (BCPTest, error) {
	if _, err := s.repo.GetBCPPlan(ctx, orgID, planID); err != nil {
		return BCPTest{}, err
	}
	return s.repo.AddBCPTest(ctx, orgID, planID, in)
}

// ListBCPTests returns all test records for a BCP plan.
// It verifies the plan belongs to the organisation before querying.
func (s *Service) ListBCPTests(ctx context.Context, orgID, planID string) ([]BCPTest, error) {
	if _, err := s.repo.GetBCPPlan(ctx, orgID, planID); err != nil {
		return nil, err
	}
	return s.repo.ListBCPTests(ctx, orgID, planID)
}

// BCPTestStaleDays is the number of days after which a BCDR test is considered stale.
const BCPTestStaleDays = 365

// BCPTestIsStale returns true if no test date is provided or the given date is
// older than BCPTestStaleDays (12 months).
func BCPTestIsStale(latestDate string) bool {
	if latestDate == "" {
		return true
	}
	t, err := time.Parse("2006-01-02", latestDate)
	if err != nil {
		return true
	}
	return time.Since(t) > BCPTestStaleDays*24*time.Hour
}
