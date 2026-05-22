package nis2wizard

import "testing"

// Sprint 28 / S28-4: Tests für ComputeMultiFrameworkScore.

func TestComputeMultiFrameworkScore_Empty(t *testing.T) {
	score := ComputeMultiFrameworkScore(map[string]AnswerEntry{})
	if score.Overall != 0 {
		t.Errorf("empty answers should give overall 0, got %d", score.Overall)
	}
	if score.NIS2 != 0 {
		t.Errorf("empty answers should give NIS2 0, got %d", score.NIS2)
	}
	if score.ISO27001 != 0 {
		t.Errorf("empty answers should give ISO27001 0, got %d", score.ISO27001)
	}
	if score.DSGVO != 0 {
		t.Errorf("empty answers should give DSGVO 0, got %d", score.DSGVO)
	}
}

func TestComputeMultiFrameworkScore_PerfectAll(t *testing.T) {
	answers := map[string]AnswerEntry{}
	for _, q := range MultiFrameworkQuestions {
		answers[q.ID] = AnswerEntry{Value: 4}
	}
	score := ComputeMultiFrameworkScore(answers)
	if score.Overall != 100 {
		t.Errorf("all-4 answers should give overall 100, got %d", score.Overall)
	}
	if score.NIS2 != 100 {
		t.Errorf("all-4 answers should give NIS2 100, got %d", score.NIS2)
	}
	if score.ISO27001 != 100 {
		t.Errorf("all-4 answers should give ISO27001 100, got %d", score.ISO27001)
	}
	if score.DSGVO != 100 {
		t.Errorf("all-4 answers should give DSGVO 100, got %d", score.DSGVO)
	}
}

func TestComputeMultiFrameworkScore_ZeroAll(t *testing.T) {
	answers := map[string]AnswerEntry{}
	for _, q := range MultiFrameworkQuestions {
		answers[q.ID] = AnswerEntry{Value: 0}
	}
	score := ComputeMultiFrameworkScore(answers)
	if score.Overall != 0 {
		t.Errorf("all-0 answers should give overall 0, got %d", score.Overall)
	}
}

func TestComputeMultiFrameworkScore_HalfAll(t *testing.T) {
	answers := map[string]AnswerEntry{}
	for _, q := range MultiFrameworkQuestions {
		answers[q.ID] = AnswerEntry{Value: 2}
	}
	score := ComputeMultiFrameworkScore(answers)
	// Value 2 von 4 = 50%.
	if score.Overall != 50 {
		t.Errorf("all-2 answers should give overall 50, got %d", score.Overall)
	}
	if score.NIS2 != 50 {
		t.Errorf("all-2 NIS2 answers should give 50, got %d", score.NIS2)
	}
}

func TestComputeMultiFrameworkScore_ByFrameworkPopulated(t *testing.T) {
	answers := map[string]AnswerEntry{}
	for _, q := range MultiFrameworkQuestions {
		answers[q.ID] = AnswerEntry{Value: 3}
	}
	score := ComputeMultiFrameworkScore(answers)
	for _, fw := range []string{FrameworkNIS2, FrameworkISO27001, FrameworkDSGVOTOM} {
		if _, ok := score.ByFramework[fw]; !ok {
			t.Errorf("ByFramework should contain %s", fw)
		}
	}
}

func TestComputeMultiFrameworkScore_TopGapsAreOrdered(t *testing.T) {
	// Beantworte nur NIS2-Fragen mit hohem Score, alle anderen mit 0.
	answers := map[string]AnswerEntry{}
	for _, q := range MultiFrameworkQuestions {
		if q.Framework == FrameworkNIS2 {
			answers[q.ID] = AnswerEntry{Value: 4}
		} else {
			answers[q.ID] = AnswerEntry{Value: 0}
		}
	}
	score := ComputeMultiFrameworkScore(answers)
	// Top-Gaps sollten vorhanden sein.
	if len(score.TopGaps) == 0 {
		t.Error("expected top gaps when non-NIS2 areas score 0")
	}
	// Scores sollten aufsteigend sortiert sein.
	for i := 1; i < len(score.TopGaps); i++ {
		if score.TopGaps[i].Score < score.TopGaps[i-1].Score {
			t.Errorf("TopGaps not sorted ascending: [%d]=%d > [%d]=%d",
				i-1, score.TopGaps[i-1].Score, i, score.TopGaps[i].Score)
		}
	}
}

func TestComputeMultiFrameworkScore_CrossMappingBoostsFrameworks(t *testing.T) {
	// Eine Frage mit Cross-Framework-Mapping auf iso27001 und dsgvo_tom beantworten.
	// Suche die erste NIS2-Frage mit beiden Cross-Frameworks.
	var crossQ *MultiFrameworkQuestion
	for i := range MultiFrameworkQuestions {
		q := &MultiFrameworkQuestions[i]
		if q.Framework == FrameworkNIS2 && len(q.CrossFrameworks) >= 2 {
			crossQ = q
			break
		}
	}
	if crossQ == nil {
		t.Skip("no NIS2 question with 2+ cross-frameworks found")
	}

	answers := map[string]AnswerEntry{
		crossQ.ID: {Value: 4},
	}
	score := ComputeMultiFrameworkScore(answers)

	// NIS2-Score muss > 0 sein.
	if score.NIS2 == 0 {
		t.Errorf("NIS2 score should be > 0 after answering a NIS2 question, got %d", score.NIS2)
	}
	// Mindestens eines der Cross-Framework-Scores muss > 0 sein.
	anyBoost := false
	for _, cf := range crossQ.CrossFrameworks {
		if score.ByFramework[cf] > 0 {
			anyBoost = true
		}
	}
	if !anyBoost {
		t.Errorf("cross-framework mapping should boost at least one of %v", crossQ.CrossFrameworks)
	}
}

func TestMultiFrameworkQuestionCount(t *testing.T) {
	count := MultiFrameworkQuestionCount()
	if count < 80 {
		t.Errorf("expected at least 80 questions, got %d", count)
	}
}

func TestValidMultiFrameworkQuestionID(t *testing.T) {
	if !validMultiFrameworkQuestionID("mf.nis2.gov.policy") {
		t.Error("mf.nis2.gov.policy should be valid")
	}
	if validMultiFrameworkQuestionID("gov.policy") {
		t.Error("gov.policy is NIS2-only and should NOT be valid for multi-framework")
	}
	if validMultiFrameworkQuestionID("nonexistent.question") {
		t.Error("nonexistent question should not be valid")
	}
}

func TestMultiFrameworkQuestions_StableIDs(t *testing.T) {
	// Prüfe, dass alle IDs mit "mf." beginnen und eindeutig sind.
	seen := map[string]bool{}
	for _, q := range MultiFrameworkQuestions {
		if len(q.ID) < 4 || q.ID[:3] != "mf." {
			t.Errorf("question ID %q should start with 'mf.'", q.ID)
		}
		if seen[q.ID] {
			t.Errorf("duplicate question ID: %q", q.ID)
		}
		seen[q.ID] = true
	}
}

func TestMultiFrameworkQuestions_FrameworksValid(t *testing.T) {
	validFWs := map[string]bool{
		FrameworkNIS2:     true,
		FrameworkISO27001: true,
		FrameworkDSGVOTOM: true,
	}
	for _, q := range MultiFrameworkQuestions {
		if !validFWs[q.Framework] {
			t.Errorf("question %q has invalid framework: %q", q.ID, q.Framework)
		}
		for _, cf := range q.CrossFrameworks {
			if !validFWs[cf] {
				t.Errorf("question %q has invalid cross-framework: %q", q.ID, cf)
			}
			if cf == q.Framework {
				t.Errorf("question %q cross-framework %q duplicates primary framework", q.ID, cf)
			}
		}
	}
}
