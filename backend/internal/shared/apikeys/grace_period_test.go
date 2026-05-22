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
		{"wildcard all", []string{"*"}, "secvault.secrets.read", true},
		{"exact match", []string{"secvault.secrets.read"}, "secvault.secrets.read", true},
		{"module wildcard match", []string{"secvault.*"}, "secvault.secrets.read", true},
		{"module wildcard mismatch", []string{"secpulse.*"}, "secvault.secrets.read", false},
		{"empty scopes", []string{}, "secvault.secrets.read", false},
		{"non-matching scope", []string{"secpulse.findings.read"}, "secvault.secrets.read", false},
		{"multiple scopes one matches", []string{"secpulse.findings.read", "secvault.secrets.read"}, "secvault.secrets.read", true},
		{"wildcard combined with specific", []string{"secvault.*", "secpulse.findings.read"}, "secpulse.findings.write", false},
		{"wildcard combined match", []string{"secvault.*", "secpulse.findings.read"}, "secvault.secrets.write", true},
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
