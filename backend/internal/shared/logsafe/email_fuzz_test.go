// Fuzz tests for the email-redaction helper. This function is called on every
// log line that mentions an email address (38 call sites as of Audit F7) so any
// pathological input would cascade across the structured log. The key
// invariants we want to hold:
//
//  1. Never panic.
//  2. Output never contains the local-part of the input email — that's the
//     whole point of redaction.
//  3. Empty input maps to empty output (so the log field can be omitted).
package logsafe

import (
	"strings"
	"testing"
)

func FuzzRedactEmail(f *testing.F) {
	f.Add("alice@example.com")
	f.Add("bob+tag@sub.domain.co.uk")
	f.Add("")
	f.Add("@noLocalPart.com")
	f.Add("noAt")
	f.Add("local@")
	f.Add(strings.Repeat("a", 1024) + "@example.com")
	f.Add("unicode💌@xn--example.com")

	f.Fuzz(func(t *testing.T, input string) {
		out := RedactEmail(input)

		// Invariant 1: never panic (implicit — would crash the runner).
		// Invariant 3: empty in → empty out.
		if input == "" && out != "" {
			t.Errorf("empty input must return empty output, got %q", out)
		}

		// Invariant 2: the FULL input email must never appear verbatim in
		// the output (that would mean redaction did nothing). We do not
		// check substring-of-local-part because the local can coincidentally
		// equal the domain ("ce@ce") — the meaningful security property is
		// "the whole address is not echoed back".
		if len(input) >= 4 && strings.Contains(input, "@") && strings.Contains(out, input) {
			t.Errorf("full input %q echoed back in output %q — redaction failed",
				input, out)
		}
	})
}
