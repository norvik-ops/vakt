// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package policy_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/modules/vaktcomply/policy"
)

// Kopf und Domänen-Tabelle der Compliance-Übersicht zählen nach derselben Regel
// wie der Bereitschaftsgrad im Dashboard: „Abgedeckt“ heißt voll umgesetzt,
// eine teilweise umgesetzte Kontrolle zählt im Score halb und nie als abgedeckt.
// Vorher stand im Kopf „Abgedeckt: 1“, die Domänen-Spalte summierte 2, und die
// Domäne rechnete die teilweise Kontrolle voll (1/6 = 17 % statt 8 %).
func TestComputeReadinessReport_TeilweiseZaehltInDomaeneWieImKopf(t *testing.T) {
	fw := &policy.Framework{ID: "fw-1", Name: "ISO 27001"}
	controls := []policy.Control{
		{ID: "c-1", Domain: "Organisation", ManualStatus: "partial"},
		{ID: "c-2", Domain: "Organisation"},
		{ID: "c-3", Domain: "Organisation"},
		{ID: "c-4", Domain: "Organisation"},
		{ID: "c-5", Domain: "Organisation"},
		{ID: "c-6", Domain: "Organisation"},
		{ID: "c-7", Domain: "Technik", ManualStatus: "implemented"},
		{ID: "c-8", Domain: "Technik", ManualStatus: "in_progress"},
	}
	report := policy.ComputeReadinessReport(fw, controls, map[string]int{})

	require.Equal(t, 1, report.Covered)
	require.Equal(t, 2, report.Partial)

	byDomain := map[string]policy.DomainScore{}
	sumCovered := 0
	for _, d := range report.ByDomain {
		byDomain[d.Domain] = d
		sumCovered += d.Covered
	}
	assert.Equal(t, report.Covered, sumCovered,
		"Summe der Domänen-Spalte „Abgedeckt“ muss dem Kopf entsprechen")

	assert.Equal(t, 0, byDomain["Organisation"].Covered)
	assert.InDelta(t, 0.5/6*100, byDomain["Organisation"].Score, 0.001,
		"teilweise umgesetzt zählt im Domänen-Score halb")
	assert.Equal(t, 1, byDomain["Technik"].Covered)
	assert.InDelta(t, 75.0, byDomain["Technik"].Score, 0.001)
}
