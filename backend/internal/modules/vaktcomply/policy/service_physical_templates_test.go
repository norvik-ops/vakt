// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package policy

import (
	"strings"
	"testing"
)

func TestPhysicalTemplates_AllFourteenControls(t *testing.T) {
	root := loadPhysicalTemplates()
	if len(root.Templates) != 14 {
		t.Fatalf("expected 14 A.7 templates, got %d", len(root.Templates))
	}
	want := map[string]bool{}
	for i := 1; i <= 14; i++ {
		want[strings_Sprintf(i)] = true
	}
	for _, tmpl := range root.Templates {
		if !strings.HasPrefix(tmpl.ControlCode, "A.7.") {
			t.Errorf("template %q is not an A.7 control", tmpl.ControlCode)
		}
		delete(want, tmpl.ControlCode)
		if len(tmpl.Items) < 3 {
			t.Errorf("template %s has %d items, want >=3", tmpl.ControlCode, len(tmpl.Items))
		}
		if tmpl.Title == "" {
			t.Errorf("template %s has empty title", tmpl.ControlCode)
		}
	}
	if len(want) != 0 {
		t.Errorf("missing A.7 controls: %v", want)
	}
}

func TestFindPhysicalTemplate(t *testing.T) {
	if _, ok := findPhysicalTemplate("A.7.7"); !ok {
		t.Error("A.7.7 (clear desk) template must exist")
	}
	if _, ok := findPhysicalTemplate("A.99.99"); ok {
		t.Error("unknown control must not resolve to a template")
	}
}

func strings_Sprintf(i int) string {
	return "A.7." + itoa(i)
}

func itoa(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return string(rune('0'+i/10)) + string(rune('0'+i%10))
}
