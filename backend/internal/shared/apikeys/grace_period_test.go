package apikeys

import "testing"

// Sprint 22 / S22-1: Test fuer ScopeAllows wird ergänzt. Der eigentliche
// Grace-Period-Roundtrip ist ein Integration-Test (siehe
// internal/integration_test/), weil er gegen eine echte DB läuft. Hier
// nur die Helper-Funktion isoliert.

func TestScopeAllows(t *testing.T) {
	tests := []struct {
		name     string
		scopes   []string
		required string
		want     bool
	}{
		{"wildcard all", []string{"*"}, "vaktvault.secrets.read", true},
		{"exact match", []string{"vaktvault.secrets.read"}, "vaktvault.secrets.read", true},
		{"module wildcard match", []string{"vaktvault.*"}, "vaktvault.secrets.read", true},
		{"module wildcard mismatch", []string{"vaktscan.*"}, "vaktvault.secrets.read", false},
		{"empty scopes", []string{}, "vaktvault.secrets.read", false},
		{"non-matching scope", []string{"vaktscan.findings.read"}, "vaktvault.secrets.read", false},
		{"multiple scopes one matches", []string{"vaktscan.findings.read", "vaktvault.secrets.read"}, "vaktvault.secrets.read", true},
		{"wildcard combined with specific", []string{"vaktvault.*", "vaktscan.findings.read"}, "vaktscan.findings.write", false},
		{"wildcard combined match", []string{"vaktvault.*", "vaktscan.findings.read"}, "vaktvault.secrets.write", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ScopeAllows(tc.scopes, tc.required)
			if got != tc.want {
				t.Errorf("ScopeAllows(%v, %q) = %v, want %v", tc.scopes, tc.required, got, tc.want)
			}
		})
	}
}
