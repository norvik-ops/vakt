// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package db

import "testing"

// R1-B0-N2 — es gibt bewusst kein "down all".
//
// Ein Rollback ist destruktiv, und die Schrittzahl ist die einzige Bremse
// zwischen einem gezielten Rueckweg und einem leeren Schema. Der Guard steht
// deshalb VOR dem Verbindungsaufbau: eine 0 oder eine negative Zahl darf die
// Datenbank gar nicht erst erreichen.
func TestMigrateDownRefusesNonPositiveSteps(t *testing.T) {
	for _, n := range []int{0, -1, -100} {
		// Die DSN ist absichtlich unbrauchbar: kommt hier ein Verbindungsfehler
		// statt des Guard-Fehlers zurueck, stand der Guard zu spaet.
		err := MigrateDown("postgres://nirgendwo/nichts", "/nirgendwo", n)
		if err == nil {
			t.Fatalf("n=%d muss abgelehnt werden", n)
		}
		if got := err.Error(); got == "" || !contains(got, "Schrittzahl muss positiv sein") {
			t.Fatalf("n=%d: erwartet wurde der Guard-Fehler, bekommen: %v", n, err)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
