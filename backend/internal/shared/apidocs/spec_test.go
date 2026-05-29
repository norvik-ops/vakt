// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package apidocs

import (
	"testing"

	"gopkg.in/yaml.v3"
)

// minSpec is the subset of the OpenAPI document we care about validating at
// build time. We do not need a full OpenAPI 3.0 parser here — the goal is to
// catch obvious breakage (malformed YAML, missing top-level fields, missing
// paths) without pulling in a heavy schema validator.
type minSpec struct {
	OpenAPI string                 `yaml:"openapi"`
	Info    map[string]interface{} `yaml:"info"`
	Paths   map[string]interface{} `yaml:"paths"`
}

// TestSpec_Parses guarantees the embedded openapi.yaml is valid YAML and
// has the structural shape OpenAPI 3.0.x consumers expect (openapi version
// declaration, info block, paths map). If this test fails, the Swagger UI
// will not render at runtime — block the build.
func TestSpec_Parses(t *testing.T) {
	data, err := SpecBytes()
	if err != nil {
		t.Fatalf("read embedded spec: %v", err)
	}
	if len(data) < 100 {
		t.Fatalf("spec is suspiciously small (%d bytes) — likely empty or truncated", len(data))
	}

	var spec minSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		t.Fatalf("spec is not valid YAML: %v", err)
	}
	if spec.OpenAPI == "" || spec.OpenAPI[:3] != "3.0" {
		t.Fatalf("openapi field missing or not 3.0.x: %q", spec.OpenAPI)
	}
	if spec.Info["title"] == nil || spec.Info["version"] == nil {
		t.Fatalf("info.title or info.version missing")
	}
	if len(spec.Paths) < 30 {
		t.Fatalf("only %d paths documented — spec appears incomplete (expected ≥30)", len(spec.Paths))
	}
}

// TestSpec_DocumentsCoreEndpoints verifies that endpoints which have been
// shipped (and are therefore part of the public API surface) appear in the
// spec. A new path can be added by simply adding it to the list below — if
// you ship a new endpoint without documenting it, this test goes red.
func TestSpec_DocumentsCoreEndpoints(t *testing.T) {
	data, err := SpecBytes()
	if err != nil {
		t.Fatalf("read embedded spec: %v", err)
	}
	var spec minSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		t.Fatalf("spec is not valid YAML: %v", err)
	}

	required := []string{
		// Auth
		"/auth/login",
		"/auth/register",
		"/auth/sessions",
		// HR (Sprint 1)
		"/hr/employees",
		"/hr/checklists",
		"/hr/checklist-runs",
		// SecVitals (core compliance)
		"/vaktcomply/frameworks",
		"/vaktcomply/controls/{id}",
		// SecPulse
		"/vaktscan/assets",
		"/vaktscan/findings",
		// SecPrivacy
		"/vaktprivacy/dsr",
		// SecVault
		"/vaktvault/projects",
		// SecReflex
		"/vaktaware/campaigns",
		"/vaktaware/templates/presets",
		// Cross-module
		"/search",
		"/dashboard/score",
	}

	for _, path := range required {
		if _, ok := spec.Paths[path]; !ok {
			t.Errorf("required path missing from openapi.yaml: %s", path)
		}
	}
}
