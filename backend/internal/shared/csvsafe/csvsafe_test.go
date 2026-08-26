// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package csvsafe

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCell(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		// Die Nutzlast aus dem Befund R1-24-D03.
		{"DDE-Befehl", `=cmd|/c calc!A1`, `'=cmd|/c calc!A1`},
		{"Formel", "=1+1", "'=1+1"},
		{"Plus", "+SUM(A1:A9)", "'+SUM(A1:A9)"},
		{"Minus vor Formel", "-2+3+cmd|' /C calc'!A0", "'-2+3+cmd|' /C calc'!A0"},
		{"At", "@SUM(1)", "'@SUM(1)"},
		{"Tabulator", "\tinjiziert", "'\tinjiziert"},
		{"Wagenruecklauf", "\rinjiziert", "'\rinjiziert"},

		// Nichts kaputtmachen: Zahlen bleiben Zahlen, sonst gehen Summen in
		// fertigen Auswertungen verloren.
		{"negative Zahl", "-5", "-5"},
		{"negative Kommazahl", "-3.5", "-3.5"},
		{"positive Zahl mit Vorzeichen", "+42", "+42"},
		{"gewoehnlicher Text", "Nicht anwendbar, da kein Rechenzentrum", "Nicht anwendbar, da kein Rechenzentrum"},
		{"leer", "", ""},
		{"Gleichheitszeichen in der Mitte", "a=b", "a=b"},
		{"Umlaut am Anfang", "Übergabe", "Übergabe"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, Cell(tc.in))
		})
	}
}

func TestRow_LaesstEingabeUnveraendert(t *testing.T) {
	in := []string{"=1+1", "harmlos"}
	out := Row(in)
	assert.Equal(t, []string{"'=1+1", "harmlos"}, out)
	assert.Equal(t, []string{"=1+1", "harmlos"}, in, "Row darf die Eingabe nicht ueberschreiben")
}
