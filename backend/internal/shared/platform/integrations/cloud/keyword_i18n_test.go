// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package cloud

import (
	"strings"
	"testing"
)

// TestWithGerman_EnglishKeywordHitsGermanControl pins R1-M1-N1: the shipped
// control catalogue is German, the collectors pass English keywords. Each pair
// below is (English collector keyword, a real German control title/domain that
// ships in the Go catalogue, service_helpers.go). FindCKControlsByKeywords only
// searches lower(title)/lower(domain), so every German string here is a real
// title or domain — never description text. Before the fix the English keyword's
// %ILIKE% never matched; withGerman must now produce a substring that does.
func TestWithGerman_EnglishKeywordHitsGermanControl(t *testing.T) {
	cases := []struct {
		keyword     string
		germanTitle string
	}{
		{"access", "Zugangskontrolle"},                   // A.5.15 title + domain
		{"access", "Zugriffsrechte"},                     // A.5.18 title
		{"identity", "Identitätsmanagement"},             // A.5.16 title
		{"encryption", "Verschlüsselung ruhender Daten"}, // A.8.24-style title
	}

	for _, tc := range cases {
		expanded := withGerman(tc.keyword)
		hay := strings.ToLower(tc.germanTitle)
		matched := false
		for _, kw := range expanded {
			if strings.Contains(hay, strings.ToLower(kw)) {
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("withGerman(%q)=%v does not match German control %q — English-only keywords miss the German catalogue (R1-M1-N1)",
				tc.keyword, expanded, tc.germanTitle)
		}
	}
}

// TestWithGerman_PreservesEnglishKeywords guarantees the fix is additive: every
// original English keyword survives, so a customer running an English catalogue
// keeps matching exactly as before.
func TestWithGerman_PreservesEnglishKeywords(t *testing.T) {
	in := []string{"iam", "access", "identity", "password", "mfa"}
	got := withGerman(in...)
	for _, want := range in {
		if !contains(got, want) {
			t.Errorf("withGerman dropped original keyword %q; result=%v", want, got)
		}
	}
}

// TestWithGerman_NoDuplicates ensures overlapping synonyms (e.g. "key" and
// "encryption" both add "schlüssel") do not produce duplicate patterns.
func TestWithGerman_NoDuplicates(t *testing.T) {
	got := withGerman("encryption", "key")
	seen := map[string]int{}
	for _, kw := range got {
		seen[kw]++
		if seen[kw] > 1 {
			t.Errorf("withGerman produced duplicate keyword %q in %v", kw, got)
		}
	}
	if !contains(got, "schlüssel") {
		t.Errorf("expected shared synonym 'schlüssel' in %v", got)
	}
}

// TestWithGerman_UnknownKeywordPassesThrough covers loanwords with no German
// entry (firewall, mfa): they must survive untouched and add nothing.
func TestWithGerman_UnknownKeywordPassesThrough(t *testing.T) {
	got := withGerman("firewall", "mfa")
	if len(got) != 2 || got[0] != "firewall" || got[1] != "mfa" {
		t.Errorf("loanword keywords should pass through unchanged, got %v", got)
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
