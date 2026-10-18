package config

import (
	"reflect"
	"testing"
)

func TestEnabledModules(t *testing.T) {
	cases := []struct {
		csv  string
		want []string
	}{
		{"vaktcomply", []string{"vaktcomply"}},
		// Reihenfolge folgt KnownModules, nicht der Variable; Groß-/Kleinschreibung und Leerzeichen egal.
		{" VaktHR , vaktcomply,vaktscan", []string{"vaktcomply", "vaktscan", "vakthr"}},
		// Unbekannte Namen erreichen das Frontend nicht.
		{"vaktcomply,vaktfoo", []string{"vaktcomply"}},
		// Das ISMS lässt sich nicht abschalten.
		{"", []string{"vaktcomply"}},
		{"vaktscan", []string{"vaktcomply", "vaktscan"}},
	}
	for _, tc := range cases {
		got := (&Config{ModulesEnabled: tc.csv}).EnabledModules()
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("EnabledModules(%q) = %v, want %v", tc.csv, got, tc.want)
		}
	}
}
