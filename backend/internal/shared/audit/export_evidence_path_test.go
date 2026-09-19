package audit

import "testing"

// TestEvidenceZipPath_TraversalIsNeutralised proves original_name can never
// escape the evidence/ prefix in the ZIP: directory and traversal segments are
// stripped down to the base name. This guards R1-14c-03's path-safety contract.
func TestEvidenceZipPath_TraversalIsNeutralised(t *testing.T) {
	cases := []struct {
		name     string
		original string
		stored   string
		want     string
	}{
		{"plain", "bericht.pdf", "uuid-1.pdf", "evidence/bericht.pdf"},
		{"unix traversal", "../../etc/passwd", "uuid-2", "evidence/passwd"},
		{"absolute path", "/etc/shadow", "uuid-3", "evidence/shadow"},
		{"windows separators", `..\..\secret.txt`, "uuid-4", "evidence/secret.txt"},
		{"nested dirs", "a/b/c/report.docx", "uuid-5", "evidence/report.docx"},
		{"empty original falls back to stored", "", "uuid-6.png", "evidence/uuid-6.png"},
		{"dotdot original falls back to stored", "..", "uuid-7", "evidence/uuid-7"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := evidenceZipPath(tc.original, tc.stored, map[string]bool{})
			if got != tc.want {
				t.Fatalf("evidenceZipPath(%q, %q) = %q, want %q", tc.original, tc.stored, got, tc.want)
			}
		})
	}
}

// TestEvidenceZipPath_DeduplicatesCollisions proves that files sharing an
// original_name do not overwrite each other inside the ZIP — each collision gets
// a distinct numeric suffix before the extension.
func TestEvidenceZipPath_DeduplicatesCollisions(t *testing.T) {
	used := map[string]bool{}
	first := evidenceZipPath("report.pdf", "s1.pdf", used)
	second := evidenceZipPath("report.pdf", "s2.pdf", used)
	third := evidenceZipPath("report.pdf", "s3.pdf", used)

	if first != "evidence/report.pdf" {
		t.Fatalf("first = %q, want evidence/report.pdf", first)
	}
	if second != "evidence/report-2.pdf" {
		t.Fatalf("second = %q, want evidence/report-2.pdf", second)
	}
	if third != "evidence/report-3.pdf" {
		t.Fatalf("third = %q, want evidence/report-3.pdf", third)
	}
	if first == second || second == third || first == third {
		t.Fatalf("collision paths not unique: %q %q %q", first, second, third)
	}
}
