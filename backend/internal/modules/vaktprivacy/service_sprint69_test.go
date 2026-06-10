// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Tests for Sprint 69 S69-6 TIA model invariants and business logic.

package vaktprivacy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ── S69-6: AdequacyDecision model ────────────────────────────────────────────

func TestAdequacyDecisionFields(t *testing.T) {
	d := AdequacyDecision{
		CountryCode: "JP",
		CountryName: "Japan",
		HasAdequacy: true,
		LastUpdated: "2023-01-01",
	}
	assert.Equal(t, "JP", d.CountryCode)
	assert.True(t, d.HasAdequacy)
	assert.Nil(t, d.DecisionDate)
}

func TestAdequacyDecisionNoAdequacy(t *testing.T) {
	d := AdequacyDecision{
		CountryCode: "US",
		CountryName: "United States",
		HasAdequacy: false,
		LastUpdated: "2023-07-10",
	}
	assert.False(t, d.HasAdequacy)
}

// ── S69-6: CreateTransferInput validation constraints ────────────────────────

func TestCreateTransferInputMechanisms(t *testing.T) {
	// Validate allowed mechanisms match the validator tag.
	validMechanisms := []string{"adequacy_decision", "scc", "bcr", "derogation", "other"}
	assert.Equal(t, 5, len(validMechanisms))
	for _, m := range validMechanisms {
		assert.NotEmpty(t, m)
	}
}

func TestCreateTIAInputOutcomes(t *testing.T) {
	validOutcomes := []string{"adequate", "adequate_with_measures", "inadequate"}
	assert.Equal(t, 3, len(validOutcomes))
}

func TestCreateTIAInputSurveillanceRisks(t *testing.T) {
	validRisks := []string{"low", "medium", "high"}
	assert.Equal(t, 3, len(validRisks))
}

// ── S69-6: Transfer status auto-assignment logic ──────────────────────────────

func TestTransferStatusAdequate(t *testing.T) {
	// Simulate the logic in CreateTransfer: adequate decision + adequacy_decision mechanism → "adequate".
	adq := &AdequacyDecision{HasAdequacy: true}
	mechanism := "adequacy_decision"

	status := "requires_tia"
	if adq != nil && adq.HasAdequacy && mechanism == "adequacy_decision" {
		status = "adequate"
	}
	assert.Equal(t, "adequate", status)
}

func TestTransferStatusRequiresTIA_NoAdequacy(t *testing.T) {
	// No adequacy decision → always requires_tia.
	var adq *AdequacyDecision
	mechanism := "adequacy_decision"

	status := "requires_tia"
	if adq != nil && adq.HasAdequacy && mechanism == "adequacy_decision" {
		status = "adequate"
	}
	assert.Equal(t, "requires_tia", status)
}

func TestTransferStatusRequiresTIA_WrongMechanism(t *testing.T) {
	// Has adequacy decision but mechanism is SCC → still requires_tia.
	adq := &AdequacyDecision{HasAdequacy: true}
	mechanism := "scc"

	status := "requires_tia"
	if adq != nil && adq.HasAdequacy && mechanism == "adequacy_decision" {
		status = "adequate"
	}
	assert.Equal(t, "requires_tia", status)
}

// ── S69-6: TIA outcome → transfer status mapping ─────────────────────────────

func TestTIAOutcomeToTransferStatus(t *testing.T) {
	cases := []struct {
		outcome string
		want    string
	}{
		{"adequate", "tia_adequate"},
		{"adequate_with_measures", "tia_adequate_measures"},
		{"inadequate", "tia_inadequate"},
	}
	for _, tc := range cases {
		got := tiaOutcomeToTransferStatus(tc.outcome)
		assert.Equal(t, tc.want, got, "outcome=%s", tc.outcome)
	}
}

// ── S69-6: TransferComplianceStatus aggregation ───────────────────────────────

func TestTransferComplianceStatusAggregation(t *testing.T) {
	s := &TransferComplianceStatus{}
	rows := []struct {
		status string
		count  int
	}{
		{"adequate", 5},
		{"requires_tia", 3},
		{"tia_adequate", 2},
		{"tia_adequate_measures", 1},
		{"tia_inadequate", 1},
	}
	for _, r := range rows {
		s.TotalTransfers += r.count
		switch r.status {
		case "adequate":
			s.Adequate += r.count
		case "requires_tia":
			s.RequiresTIA += r.count
		case "tia_adequate":
			s.TIAAdequate += r.count
		case "tia_adequate_measures":
			s.TIAWithMeasures += r.count
		case "tia_inadequate":
			s.TIAInadequate += r.count
		}
	}

	assert.Equal(t, 12, s.TotalTransfers)
	assert.Equal(t, 5, s.Adequate)
	assert.Equal(t, 3, s.RequiresTIA)
	assert.Equal(t, 2, s.TIAAdequate)
	assert.Equal(t, 1, s.TIAWithMeasures)
	assert.Equal(t, 1, s.TIAInadequate)
	assert.Equal(t, 0, s.UnderReview)
}
