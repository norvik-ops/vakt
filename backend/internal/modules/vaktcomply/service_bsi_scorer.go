// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S82-3: ComplianceScorer — abstracts BSI 200-2 §8.4 formula behind an interface
// so that the GS++ Leistungszahlen-engine can be swapped in Sprint 84 without
// touching the GS-Check workflow (ADR-0056).

package vaktcomply

// ComplianceScorer abstracts the compliance progress formula.
// KompendiumScorer implements BSI 200-2 §8.4; Sprint 84 will add a
// LeistungszahlenScorer for GS++.
type ComplianceScorer interface {
	// Score returns the Umsetzungsgrad as a percentage [0,100].
	Score(ja, teilweise, entbehrlich, total int) float64
	// ScoreFiltered returns progress for rows matching the Absicherungsniveau level.
	ScoreFiltered(rows []BSICheckResult, absicherungsniveau string) float64
}

// KompendiumScorer implements ComplianceScorer using the BSI 200-2 §8.4 formula:
// (ja × 1.0 + teilweise × 0.5) / relevante × 100
// where relevante = total − entbehrlich.
type KompendiumScorer struct{}

func (KompendiumScorer) Score(ja, teilweise, entbehrlich, total int) float64 {
	relevante := total - entbehrlich
	if relevante <= 0 {
		return 100.0 // all entbehrlich = fully handled
	}
	punkte := float64(ja)*1.0 + float64(teilweise)*0.5
	return punkte / float64(relevante) * 100.0
}

// ScoreFiltered applies Absicherungsniveau-based filtering before scoring:
//   - "basis"           → only requirement_level == "basis" (or empty)
//   - "standard"/"kern" → basis + standard (BSI 200-2 §8.3: Kern = Standard scope)
//   - anything else     → all rows
func (s KompendiumScorer) ScoreFiltered(rows []BSICheckResult, absicherungsniveau string) float64 {
	relevant := func(level string) bool {
		switch absicherungsniveau {
		case "basis":
			return level == "basis" || level == ""
		case "standard", "kern":
			return level == "basis" || level == "standard" || level == ""
		default:
			return true
		}
	}
	var ja, teilweise, entbehrlich, total int
	for _, r := range rows {
		if !relevant(r.RequirementLevel) {
			continue
		}
		total++
		switch r.Umsetzungsstatus {
		case "ja":
			ja++
		case "teilweise":
			teilweise++
		case "entbehrlich":
			entbehrlich++
		}
	}
	return s.Score(ja, teilweise, entbehrlich, total)
}
