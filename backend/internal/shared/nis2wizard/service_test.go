package nis2wizard

import "testing"

// Sprint 19 S19-3: deterministischer Score-Engine-Test. Wenn die
// Frage-Gewichtung sich ändert, brechen diese Tests — Erinnerung an die
// Pro-Tier-Trend-View, die historische Scores zeigt.

func TestComputeScoreEmpty(t *testing.T) {
	score, byArea := computeScore(map[string]AnswerEntry{})
	if score != 0 {
		t.Errorf("empty answers should give score 0, got %d", score)
	}
	if len(byArea) != 0 {
		t.Errorf("empty answers should give no area scores, got %d", len(byArea))
	}
}

func TestComputeScorePerfect(t *testing.T) {
	answers := map[string]AnswerEntry{}
	for _, q := range Questions {
		answers[q.ID] = AnswerEntry{Value: 4}
	}
	score, byArea := computeScore(answers)
	if score != 100 {
		t.Errorf("all-4 answers should give score 100, got %d", score)
	}
	for _, a := range AllAreas {
		if byArea[a] != 100 {
			t.Errorf("area %s should be 100, got %d", a, byArea[a])
		}
	}
}

func TestComputeScoreZero(t *testing.T) {
	answers := map[string]AnswerEntry{}
	for _, q := range Questions {
		answers[q.ID] = AnswerEntry{Value: 0}
	}
	score, _ := computeScore(answers)
	if score != 0 {
		t.Errorf("all-0 answers should give score 0, got %d", score)
	}
}

func TestComputeScoreHalf(t *testing.T) {
	answers := map[string]AnswerEntry{}
	for _, q := range Questions {
		answers[q.ID] = AnswerEntry{Value: 2}
	}
	score, _ := computeScore(answers)
	// Value 2 von 4 = 50%.
	if score != 50 {
		t.Errorf("all-2 answers should give score 50, got %d", score)
	}
}

// TestComputeScoreUnfinishedRunsAreaScoresIgnoreEmpty stellt sicher, dass
// ein unfertiger Run nicht künstlich schlecht aussieht, weil unanswered
// Areas als 0 gezählt würden. Die UI zeigt die unfertigen Areas als "?".
func TestComputeScoreUnfinishedRunsAreaScoresIgnoreEmpty(t *testing.T) {
	// Beantwortet nur Governance-Fragen, alle anderen Areas leer.
	answers := map[string]AnswerEntry{}
	for _, q := range Questions {
		if q.Area == AreaGovernance {
			answers[q.ID] = AnswerEntry{Value: 4}
		}
	}
	_, byArea := computeScore(answers)
	if byArea[AreaGovernance] != 100 {
		t.Errorf("Governance should be 100, got %d", byArea[AreaGovernance])
	}
	// Andere Areas sollten NICHT in byArea sein (kein 0-Wert).
	if _, ok := byArea[AreaRiskManagement]; ok {
		t.Errorf("RiskMgmt should be absent (no answers), but is %d", byArea[AreaRiskManagement])
	}
}

func TestTopGapsOrder(t *testing.T) {
	r := &Run{ScoreByArea: map[Area]int{
		AreaGovernance:       100,
		AreaRiskManagement:   25,
		AreaIncidentResponse: 50,
		AreaBusinessCont:     10,
	}}
	gaps := r.TopGaps(2)
	if len(gaps) != 2 {
		t.Fatalf("expected 2 gaps, got %d", len(gaps))
	}
	if gaps[0].Area != AreaBusinessCont {
		t.Errorf("worst gap should be BusinessCont (10), got %s (%d)", gaps[0].Area, gaps[0].Score)
	}
	if gaps[1].Area != AreaRiskManagement {
		t.Errorf("second gap should be RiskMgmt (25), got %s (%d)", gaps[1].Area, gaps[1].Score)
	}
}

func TestHashIPDeterminism(t *testing.T) {
	a := HashIP("1.2.3.4", "secret-salt")
	b := HashIP("1.2.3.4", "secret-salt")
	if a != b {
		t.Errorf("HashIP should be deterministic for same input")
	}
	c := HashIP("1.2.3.4", "other-salt")
	if a == c {
		t.Errorf("HashIP with different salt should differ")
	}
	if HashIP("", "secret-salt") != "" {
		t.Errorf("HashIP with empty IP should return empty")
	}
}

func TestValidQuestionID(t *testing.T) {
	if !validQuestionID("gov.policy") {
		t.Error("gov.policy should be valid")
	}
	if validQuestionID("nope") {
		t.Error("nope should not be valid")
	}
}

// TestQuestionCount verifiziert, dass die 30-Fragen-Doku-Aussage stimmt.
// Wenn jemand Fragen hinzufügt/entfernt, soll dieser Test brechen, damit
// CHANGELOG + GLOSSAR + Marketing-Material angepasst werden.
func TestQuestionCount(t *testing.T) {
	if len(Questions) != 30 {
		t.Errorf("Sprint-19-Spec: 30 Fragen, aktuell %d", len(Questions))
	}
}
