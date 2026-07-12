package mailhdr

import "testing"

func TestSanitize_StripsCRLF(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "Vakt Alert: finding", "Vakt Alert: finding"},
		{"crlf injection", "subject\r\nBcc: attacker@evil.test", "subjectBcc: attacker@evil.test"},
		{"lf only", "line1\nline2", "line1line2"},
		{"cr only", "a\rb", "ab"},
		{"header split with body", "Subject\r\n\r\nInjected body", "SubjectInjected body"},
		{"empty", "", ""},
		{"unicode preserved", "Störung: Ölmühle", "Störung: Ölmühle"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Sanitize(tc.in); got != tc.want {
				t.Errorf("Sanitize(%q) = %q, want %q", tc.in, got, tc.want)
			}
			if got := Sanitize(tc.in); containsCRLF(got) {
				t.Errorf("Sanitize(%q) still contains CR/LF: %q", tc.in, got)
			}
		})
	}
}

func containsCRLF(s string) bool {
	for _, r := range s {
		if r == '\r' || r == '\n' {
			return true
		}
	}
	return false
}
