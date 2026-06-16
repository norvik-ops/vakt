// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import "testing"

func TestThreatLibrary_LoadsAtLeast60(t *testing.T) {
	root := loadThreatLibrary()
	if len(root.Threats) < 60 {
		t.Fatalf("threat library has %d entries, want >=60", len(root.Threats))
	}
	if root.Version == "" {
		t.Error("threat library version must be set (for link provenance)")
	}
	seen := map[string]bool{}
	for _, it := range root.Threats {
		if it.ID == "" || it.Title == "" {
			t.Errorf("threat %q has empty id/title", it.ID)
		}
		if seen[it.ID] {
			t.Errorf("duplicate threat id %q", it.ID)
		}
		seen[it.ID] = true
		if it.DefaultLikelihood < 1 || it.DefaultLikelihood > 5 {
			t.Errorf("threat %s default_likelihood out of range: %d", it.ID, it.DefaultLikelihood)
		}
		if it.DefaultImpact < 1 || it.DefaultImpact > 5 {
			t.Errorf("threat %s default_impact out of range: %d", it.ID, it.DefaultImpact)
		}
		if it.SuggestedMeasure == "" {
			t.Errorf("threat %s has no suggested measure", it.ID)
		}
	}
}

func TestThreatCatalogFilter(t *testing.T) {
	svc := &Service{}

	all := svc.ListThreatCatalog(ThreatCatalogFilter{})
	if len(all) < 60 {
		t.Fatalf("unfiltered returned %d, want >=60", len(all))
	}

	iso := svc.ListThreatCatalog(ThreatCatalogFilter{Framework: "ISO27001"})
	if len(iso) == 0 || len(iso) >= len(all) {
		t.Errorf("ISO27001 filter returned %d (want >0 and <%d)", len(iso), len(all))
	}
	for _, it := range iso {
		if !sliceContainsFold(it.Frameworks, "ISO27001") {
			t.Errorf("threat %s leaked into ISO27001 filter", it.ID)
		}
	}

	data := svc.ListThreatCatalog(ThreatCatalogFilter{AssetType: "data"})
	for _, it := range data {
		if !sliceContainsFold(it.AssetTypes, "data") {
			t.Errorf("threat %s leaked into data asset filter", it.ID)
		}
	}

	conf := svc.ListThreatCatalog(ThreatCatalogFilter{CIA: "confidentiality"})
	for _, it := range conf {
		if !sliceContainsFold(it.CIA, "confidentiality") {
			t.Errorf("threat %s leaked into confidentiality filter", it.ID)
		}
	}

	// Combined filter must be a subset of each single filter.
	combined := svc.ListThreatCatalog(ThreatCatalogFilter{Framework: "NIS2", CIA: "availability"})
	for _, it := range combined {
		if !sliceContainsFold(it.Frameworks, "NIS2") || !sliceContainsFold(it.CIA, "availability") {
			t.Errorf("combined filter leaked %s", it.ID)
		}
	}
}

func TestFindThreatCatalogItem(t *testing.T) {
	if _, ok := findThreatCatalogItem("T-RANSOMWARE"); !ok {
		t.Error("T-RANSOMWARE must exist in the catalog")
	}
	if _, ok := findThreatCatalogItem("T-NOPE"); ok {
		t.Error("unknown id must not resolve")
	}
}
