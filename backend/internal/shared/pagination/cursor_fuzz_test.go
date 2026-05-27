// Fuzz tests for the opaque pagination cursor decoders.
//
// These functions take untrusted query-string input (base64 → JSON) and are
// hot paths on every paginated endpoint. The invariants are simple but worth
// guarding with fuzz coverage:
//
//  1. DecodeCursor / DecodeControlCursor never panic, regardless of input.
//  2. EncodeCursor → DecodeCursor is a faithful round-trip for any valid
//     (id, ts) pair we hand in.
//
// CI runs fuzz with a short -fuzztime so each PR gets a smoke pass without
// blowing the budget; longer dedicated fuzz sessions can be triggered manually.
package pagination

import (
	"testing"
	"time"
	"unicode/utf8"
)

func FuzzDecodeCursor(f *testing.F) {
	// Seed corpus: empty, malformed, valid encoded payloads.
	f.Add("")
	f.Add("garbage")
	f.Add("?!@#$%^&*()")
	f.Add("AAA")  // base64 valid but JSON nonsense
	f.Add("eyJpZCI6IiIsInRzIjoiMjAyNi0wMS0wMVQwMDowMDowMFoifQ") // valid round-trip
	f.Add(EncodeCursor("00000000-0000-0000-0000-000000000000", time.Unix(0, 0).UTC()))
	f.Add(EncodeCursor("a-b-c-d", time.Now().UTC()))

	f.Fuzz(func(t *testing.T, input string) {
		// Must never panic. The function is documented to swallow errors
		// and return zero values; verify the contract.
		_, _ = DecodeCursor(input)
	})
}

func FuzzDecodeControlCursor(f *testing.F) {
	f.Add("")
	f.Add("garbage")
	f.Add(EncodeControlCursor("ISO-A.5.1", "11111111-1111-1111-1111-111111111111"))
	f.Add(EncodeControlCursor("", ""))

	f.Fuzz(func(t *testing.T, input string) {
		_, _ = DecodeControlCursor(input)
	})
}

func FuzzEncodeDecodeCursorRoundTrip(f *testing.F) {
	f.Add("11111111-1111-1111-1111-111111111111", int64(0))
	f.Add("ffffffff-ffff-ffff-ffff-ffffffffffff", time.Now().Unix())

	f.Fuzz(func(t *testing.T, id string, tsSec int64) {
		// Production IDs are UUIDs (valid UTF-8). The round-trip via JSON
		// is lossy for non-UTF-8 byte sequences (json.Marshal replaces them
		// with U+FFFD). Skip those — they cannot occur with real IDs.
		if !utf8.ValidString(id) {
			t.Skip("non-UTF-8 input is out of scope for real cursor IDs")
		}
		ts := time.Unix(tsSec, 0).UTC()
		encoded := EncodeCursor(id, ts)
		gotID, gotTS := DecodeCursor(encoded)
		if gotID != id {
			t.Errorf("id round-trip lost: in=%q out=%q (encoded=%q)", id, gotID, encoded)
		}
		if !gotTS.Equal(ts) {
			t.Errorf("ts round-trip lost: in=%v out=%v (encoded=%q)", ts, gotTS, encoded)
		}
	})
}
