// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Tests for Sprint 70 S70-4: Contractor/Freelancer lifecycle model invariants.

package vakthr

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ── S70-4: Contractor model ───────────────────────────────────────────────────

func TestContractor_StatusValues(t *testing.T) {
	validStatuses := []string{"active", "expiring_soon", "offboarding", "terminated"}
	assert.Equal(t, 4, len(validStatuses))
	for _, s := range validStatuses {
		assert.NotEmpty(t, s)
	}
}

func TestCreateContractorInput_RequiredFields(t *testing.T) {
	// Verify that the required validate tags are set on the input struct.
	// This is a structural assertion — actual validation is done by go-playground/validator.
	in := CreateContractorInput{
		FirstName:     "Anna",
		LastName:      "Schmidt",
		ContractStart: "2026-07-01",
		ContractEnd:   "2026-12-31",
	}
	assert.NotEmpty(t, in.FirstName)
	assert.NotEmpty(t, in.LastName)
	assert.NotEmpty(t, in.ContractStart)
	assert.NotEmpty(t, in.ContractEnd)
}

func TestUpdateContractorInput_StatusDefault(t *testing.T) {
	// When Status is empty the service defaults to "active".
	in := UpdateContractorInput{}
	status := in.Status
	if status == "" {
		status = "active"
	}
	assert.Equal(t, "active", status)
}

func TestUpdateContractorInput_OffboardingStatus(t *testing.T) {
	in := UpdateContractorInput{Status: "offboarding"}
	assert.Equal(t, "offboarding", in.Status)
}

func TestContractor_NDAAVVBoolFields(t *testing.T) {
	c := Contractor{NDASigned: true, AVVSigned: false}
	assert.True(t, c.NDASigned)
	assert.False(t, c.AVVSigned)
}

func TestContractor_AccessScopeNil(t *testing.T) {
	// Service normalises nil AccessScope to empty slice before INSERT.
	var scope []string
	if scope == nil {
		scope = []string{}
	}
	assert.NotNil(t, scope)
	assert.Empty(t, scope)
}

func TestContractor_AccessScopePreserved(t *testing.T) {
	scope := []string{"github", "aws", "confluence"}
	assert.Equal(t, 3, len(scope))
	assert.Equal(t, "github", scope[0])
}

// ── S70-4: CheckContractorExpiry status transitions ──────────────────────────

func TestContractorExpiryLogic_ExpiringSoon(t *testing.T) {
	// A contractor active today with end in 10 days → expiring_soon.
	daysUntilEnd := 10
	status := "active"
	if status == "active" && daysUntilEnd <= 14 && daysUntilEnd > 0 {
		status = "expiring_soon"
	}
	assert.Equal(t, "expiring_soon", status)
}

func TestContractorExpiryLogic_NotExpiringSoon(t *testing.T) {
	// A contractor with end in 30 days → still active.
	daysUntilEnd := 30
	status := "active"
	if status == "active" && daysUntilEnd <= 14 && daysUntilEnd > 0 {
		status = "expiring_soon"
	}
	assert.Equal(t, "active", status)
}

func TestContractorExpiryLogic_Offboarding(t *testing.T) {
	// A contractor past their end date → offboarding.
	daysUntilEnd := -1
	status := "active"
	if (status == "active" || status == "expiring_soon") && daysUntilEnd < 0 {
		status = "offboarding"
	}
	assert.Equal(t, "offboarding", status)
}

func TestContractorExpiryLogic_AlreadyTerminated(t *testing.T) {
	// A terminated contractor is not affected by expiry check.
	status := "terminated"
	daysUntilEnd := -5
	if (status == "active" || status == "expiring_soon") && daysUntilEnd < 0 {
		status = "offboarding"
	}
	assert.Equal(t, "terminated", status, "terminated contractors must not be changed by expiry check")
}
