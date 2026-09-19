package vaktprivacy

import (
	"errors"
	"strings"
)

// Breach lifecycle states per Art. 33/34 DSGVO.
//
// WICHTIG (Schema-Grenze): Die CHECK-Constraint auf po_breaches.status
// (Migration 014, erweitert um 'subjects_notified' in Migration 268) traegt alle
// vier Zustaende. Der Zustand 'subjects_notified' ist Teil der bestätigten
// DSGVO-Kette, aber NOCH NICHT persistierbar — eine Erweiterung der
// CHECK-Constraint braucht eine Migration (bewusst außerhalb dieses Scopes).
// Siehe ADR-0085 und die Behandlung von ErrBreachStatusUnsupported im Service.
const (
	BreachStatusOpen              = "open"
	BreachStatusAuthorityNotified = "authority_notified"
	BreachStatusSubjectsNotified  = "subjects_notified"
	BreachStatusClosed            = "closed"
)

// Sentinel-Fehler der Übergangsprüfung. Der Handler bildet sie auf HTTP-Codes ab
// (ungültiger Übergang → 409, fehlende Begründung → 422, nicht persistierbar → 422).
var (
	// ErrBreachStatusUnknown: Zielstatus ist kein bekannter Lifecycle-Zustand.
	ErrBreachStatusUnknown = errors.New("unbekannter breach-status")
	// ErrBreachTransitionInvalid: Übergang verletzt die Vorwärts-Reihenfolge oder
	// versucht, den terminalen Zustand 'closed' zu verlassen.
	ErrBreachTransitionInvalid = errors.New("ungültiger breach-statusübergang")
	// ErrBreachRationaleRequired: direkter open->closed ohne Meldepflicht braucht
	// eine Pflicht-Begründung (Art. 33 Abs. 1 DSGVO).
	ErrBreachRationaleRequired = errors.New("abschluss ohne meldung braucht eine begründung")
)

// allowedBreachTransitions ist die bestätigte Art. 33/34-Kette, als explizite
// Adjazenzliste (keine reine Ordnungsvergleichs-Logik, weil open->closed ein
// erlaubter Sprung ist, open->subjects_notified aber nicht).
//
//	open  -> authority_notified            (Art. 33: Meldung an die Aufsichtsbehörde)
//	open  -> closed                        (Art. 33 Abs. 1: keine Meldepflicht — Begründung nötig)
//	authority_notified -> subjects_notified (Art. 34: Benachrichtigung der Betroffenen)
//	authority_notified -> closed
//	subjects_notified  -> closed
//	closed -> ∅                            (terminal)
//
// Übergänge nur vorwärts; kein Rücksprung; 'closed' ist terminal.
var allowedBreachTransitions = map[string]map[string]bool{
	BreachStatusOpen: {
		BreachStatusAuthorityNotified: true,
		BreachStatusClosed:            true, // braucht Begründung
	},
	BreachStatusAuthorityNotified: {
		BreachStatusSubjectsNotified: true,
		BreachStatusClosed:           true,
	},
	BreachStatusSubjectsNotified: {
		BreachStatusClosed: true,
	},
	BreachStatusClosed: {}, // terminal
}

// knownBreachStatus meldet, ob s ein bekannter Lifecycle-Zustand ist.
func knownBreachStatus(s string) bool {
	switch s {
	case BreachStatusOpen, BreachStatusAuthorityNotified, BreachStatusSubjectsNotified, BreachStatusClosed:
		return true
	}
	return false
}

// ValidateBreachTransition prüft einen Statuswechsel gegen die bestätigte
// DSGVO-Kette. Reine Funktion, keine DB — daher unit-testbar ohne Postgres.
//
// rationale ist nur beim direkten open->closed Pflicht (nicht leer nach Trim).
func ValidateBreachTransition(current, target, rationale string) error {
	if !knownBreachStatus(current) || !knownBreachStatus(target) {
		return ErrBreachStatusUnknown
	}
	if !allowedBreachTransitions[current][target] {
		return ErrBreachTransitionInvalid
	}
	if current == BreachStatusOpen && target == BreachStatusClosed && strings.TrimSpace(rationale) == "" {
		return ErrBreachRationaleRequired
	}
	return nil
}
