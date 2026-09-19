package vaktprivacy

import (
	"errors"
	"testing"
)

// TestValidateBreachTransition covers the confirmed Art. 33/34 DSGVO lifecycle
// (open → authority_notified → subjects_notified → closed): forward-only,
// terminal 'closed', the rationale-gated open->closed skip, and the explicit
// rejection of the out-of-order open->subjects_notified jump.
func TestValidateBreachTransition(t *testing.T) {
	const rationale = "kein Risiko für die Betroffenen (Art. 33 Abs. 1)"

	cases := []struct {
		name      string
		current   string
		target    string
		rationale string
		want      error
	}{
		// Valid forward chain (the full confirmed path).
		{"open->authority", BreachStatusOpen, BreachStatusAuthorityNotified, "", nil},
		{"authority->subjects", BreachStatusAuthorityNotified, BreachStatusSubjectsNotified, "", nil},
		{"subjects->closed", BreachStatusSubjectsNotified, BreachStatusClosed, "", nil},
		{"authority->closed", BreachStatusAuthorityNotified, BreachStatusClosed, "", nil},

		// Direct open->closed: allowed only with a rationale (Art. 33 Abs. 1).
		{"open->closed with rationale", BreachStatusOpen, BreachStatusClosed, rationale, nil},
		{"open->closed without rationale", BreachStatusOpen, BreachStatusClosed, "", ErrBreachRationaleRequired},
		{"open->closed blank rationale", BreachStatusOpen, BreachStatusClosed, "   ", ErrBreachRationaleRequired},

		// Out-of-order jump: skipping authority_notified is forbidden.
		{"open->subjects rejected", BreachStatusOpen, BreachStatusSubjectsNotified, "", ErrBreachTransitionInvalid},

		// Backwards moves are forbidden.
		{"authority->open rejected", BreachStatusAuthorityNotified, BreachStatusOpen, "", ErrBreachTransitionInvalid},
		{"subjects->authority rejected", BreachStatusSubjectsNotified, BreachStatusAuthorityNotified, "", ErrBreachTransitionInvalid},

		// 'closed' is terminal.
		{"closed->open rejected", BreachStatusClosed, BreachStatusOpen, "", ErrBreachTransitionInvalid},
		{"closed->closed rejected", BreachStatusClosed, BreachStatusClosed, "", ErrBreachTransitionInvalid},

		// Same-state (no-op) is not a forward transition.
		{"open->open rejected", BreachStatusOpen, BreachStatusOpen, "", ErrBreachTransitionInvalid},

		// Unknown status values.
		{"unknown target", BreachStatusOpen, "archived", "", ErrBreachStatusUnknown},
		{"unknown current", "bogus", BreachStatusClosed, "", ErrBreachStatusUnknown},
		{"empty target", BreachStatusOpen, "", "", ErrBreachStatusUnknown},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateBreachTransition(tc.current, tc.target, tc.rationale)
			if !errors.Is(err, tc.want) {
				t.Fatalf("ValidateBreachTransition(%q,%q,rationale=%q) = %v, want %v",
					tc.current, tc.target, tc.rationale, err, tc.want)
			}
		})
	}
}
