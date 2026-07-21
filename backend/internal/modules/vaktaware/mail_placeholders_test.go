// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktaware

import (
	"html/template"
	"strings"
	"testing"
)

// TestNormaliseMailPlaceholders is the regression guard for R-C07: every shipped
// preset used snake_case tokens ({{first_name}}) that html/template reads as an
// undefined function, so Parse failed and no campaign mail was ever sent.
func TestNormaliseMailPlaceholders(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Hallo {{first_name}} {{last_name}}", "Hallo {{.FirstName}} {{.LastName}}"},
		{"Klick: {{tracking_url}}", "Klick: {{.TrackingURL}}"},
		{"{{ first_name }} mit Leerzeichen", "{{.FirstName}} mit Leerzeichen"},
		{"Mail: {{email}}", "Mail: {{.Email}}"},
		{"kein Platzhalter", "kein Platzhalter"},
		{"unbekannt {{foo}} bleibt", "unbekannt {{foo}} bleibt"},
	}
	for _, c := range cases {
		if got := normaliseMailPlaceholders(c.in); got != c.want {
			t.Errorf("normaliseMailPlaceholders(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestNormalisedPlaceholdersParseAndRender proves the normalised output actually
// parses under html/template and renders per-target data — the exact path that was
// born-broken.
func TestNormalisedPlaceholdersParseAndRender(t *testing.T) {
	body := normaliseMailPlaceholders("Hallo {{first_name}} {{last_name}}, Link: {{tracking_url}}")
	tmpl, err := template.New("body").Parse(body)
	if err != nil {
		t.Fatalf("parse normalised body: %v", err)
	}
	var sb strings.Builder
	data := map[string]string{"FirstName": "Ada", "LastName": "Lovelace", "TrackingURL": "https://x/t"}
	if err := tmpl.Execute(&sb, data); err != nil {
		t.Fatalf("execute: %v", err)
	}
	want := "Hallo Ada Lovelace, Link: https://x/t"
	if sb.String() != want {
		t.Errorf("render = %q, want %q", sb.String(), want)
	}
}
