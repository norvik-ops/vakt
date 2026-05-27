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

		// Invariant 2: the redacted local-part must be either "" or "***".
		// We don't substring-check against the input because the local-part
		// can coincidentally equal the domain ("ce@ce" → "***@ce" trips a
		// naive contains check). The semantically correct invariant: any
		// segment of `out` to the left of "@" comes from the redaction
		// alphabet, not from input.
		outAt := strings.LastIndexByte(out, '@')
		var outLocal string
		if outAt < 0 {
			outLocal = out // "***" or ""
		} else {
			outLocal = out[:outAt]
		}
		if outLocal != "" && outLocal != "***" {
			t.Errorf("redacted local-part must be empty or \"***\", got %q (input=%q, full out=%q)",
				outLocal, input, out)
		}
	})
}
