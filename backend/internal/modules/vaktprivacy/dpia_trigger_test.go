// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktprivacy

import "testing"

func TestDPIARiskIndicator(t *testing.T) {
	cases := []struct {
		name string
		in   CreateVVTInput
		want bool
	}{
		{"special category health", CreateVVTInput{DataCategories: []string{"Gesundheitsdaten"}}, true},
		{"special category biometric", CreateVVTInput{DataCategories: []string{"Biometrische Merkmale"}}, true},
		{"third country transfer", CreateVVTInput{ThirdCountryTransfer: true}, true},
		{"profiling purpose", CreateVVTInput{Purpose: "Automatisiertes Scoring und Profiling der Kunden"}, true},
		{"large scale surveillance", CreateVVTInput{Purpose: "Großflächige systematische Beobachtung"}, true},
		{"ordinary processing", CreateVVTInput{Purpose: "Lohnabrechnung", DataCategories: []string{"Name", "Adresse"}}, false},
		{"empty", CreateVVTInput{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, reason := dpiaRiskIndicator(tc.in)
			if got != tc.want {
				t.Errorf("dpiaRiskIndicator=%v want %v (reason=%q)", got, tc.want, reason)
			}
			if got && reason == "" {
				t.Error("high-risk result must carry a reason")
			}
		})
	}
}
