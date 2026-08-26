// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Package csvsafe entschaerft Werte, die eine Tabellenkalkulation als Formel
// lesen wuerde.
//
// R1-24-D03 (deckt R1-22-I09/SA-22): kein einziger der CSV-Schreiber im Baum
// hatte einen Formel-Schutz. csvEscape in handler_bsi.go quotet Komma,
// Anfuehrungszeichen und Zeilenumbruch — das sind die Zeichen, die das
// CSV-FORMAT braucht, nicht die, die Excel oder LibreOffice zur Auswertung
// bringen. encoding/csv macht dasselbe und auch nicht mehr.
//
// Gemessen am SoA-Export: PATCH /vaktcomply/soa/:control_id mit
// justification = "=cmd|/c calc!A1" antwortet 204, die Exportzeile traegt den
// Wert danach woertlich mit fuehrendem Gleichheitszeichen. Ein Auditor, der die
// SoA in Excel oder LibreOffice oeffnet — der Normalfall — fuehrt die Formel
// aus.
package csvsafe

import (
	"strconv"
	"strings"
)

// dangerous sind die Zeichen, mit denen Excel, LibreOffice Calc und Google
// Sheets eine Zelle als Formel bzw. als Befehl auffassen. Tabulator und
// Wagenruecklauf stehen mit auf der Liste, weil sie beim Einfuegen als
// Zelltrenner wirken und den naechsten Wert an den Zellanfang schieben.
const dangerous = "=+-@\t\r"

// Cell gibt s so zurueck, dass eine Tabellenkalkulation den Wert als Text
// liest. Faengt s mit einem der Formelzeichen an, wird ein einfaches
// Anfuehrungszeichen vorangestellt — die uebliche Entschaerfung, die den Wert
// beim Lesen sichtbar laesst und beim erneuten Speichern wieder verschwindet.
//
// Zahlen bleiben unangetastet: "-5" und "+3.5" sind Messwerte, keine Formeln.
// Ohne diese Ausnahme wuerde der Schutz jede negative Zahl in einen Textwert
// verwandeln und damit Summen in fertigen Auswertungen kaputtmachen.
func Cell(s string) string {
	if s == "" {
		return s
	}
	if !strings.ContainsRune(dangerous, rune(s[0])) {
		return s
	}
	if _, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
		return s
	}
	return "'" + s
}

// Row wendet Cell auf jedes Feld an und gibt eine neue Zeile zurueck.
// Die uebergebene Zeile bleibt unveraendert.
func Row(fields []string) []string {
	out := make([]string, len(fields))
	for i, f := range fields {
		out[i] = Cell(f)
	}
	return out
}
