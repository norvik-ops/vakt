// Package mailhdr provides one canonical guard against SMTP header injection
// (CWE-93): stripping CR and LF from every value interpolated into an email
// header line (From/To/Subject/…).
//
// This class has recurred three times in this codebase — the form-handler
// (S120-3), the alerting service (S122-C5/D10), and every other hand-rolled
// header builder that never chased the variant. Centralising it here means new
// mailers have one obvious thing to call, and a lint/gate can require it.
//
// Bodies are NOT sanitised here — they live after the blank line that separates
// headers from body, so newlines in a body are legitimate and expected.
package mailhdr

import "strings"

var crlfStripper = strings.NewReplacer("\r", "", "\n", "")

// Sanitize removes CR and LF so the value cannot terminate its header line and
// inject additional headers. Apply to every From/To/Subject/Cc value that is
// or may be derived from user, event, or external input.
func Sanitize(v string) string {
	return crlfStripper.Replace(v)
}
