package nis2wizard

// Sprint 28 / S28-3: Unit-Tests für den Re-Assessment-Service.
//
// DB-freie Tests — nur die reinen Berechnungs- und Hilfsfunktionen.
// Service-Integration-Tests (mit DB) werden in einem separaten
// Integrations-Test abgedeckt sobald eine Test-DB verfügbar ist.

import (
	"testing"
	"time"
)

func TestBuildTopGaps_Order(t *testing.T) {
	byArea := map[Area]int{
		AreaGovernance:       80,
		AreaRiskManagement:   20,
		AreaIncidentResponse: 50,
		AreaBusinessCont:     10,
		AreaSupplyChain:      35,
	}
	gaps := buildTopGaps(byArea, 3)
	if len(gaps) != 3 {
		t.Fatalf("expected 3 gaps, got %d", len(gaps))
	}
	// Niedrigster Score zuerst.
	if gaps[0].Area != AreaBusinessCont {
		t.Errorf("worst gap should be BusinessCont (10), got %s (%d)", gaps[0].Area, gaps[0].Score)
	}
	if gaps[1].Area != AreaRiskManagement {
		t.Errorf("second gap should be RiskMgmt (20), got %s (%d)", gaps[1].Area, gaps[1].Score)
	}
	if gaps[2].Area != AreaSupplyChain {
		t.Errorf("third gap should be SupplyChain (35), got %s (%d)", gaps[2].Area, gaps[2].Score)
	}
}

func TestBuildTopGaps_NMoreThanAreas(t *testing.T) {
	byArea := map[Area]int{
		AreaGovernance: 50,
		AreaCrypto:     30,
	}
	gaps := buildTopGaps(byArea, 10)
	if len(gaps) != 2 {
		t.Errorf("expected 2 gaps (all areas), got %d", len(gaps))
	}
}

func TestBuildTopGaps_Empty(t *testing.T) {
	gaps := buildTopGaps(map[Area]int{}, 3)
	if len(gaps) != 0 {
		t.Errorf("expected 0 gaps for empty input, got %d", len(gaps))
	}
}

func TestBuildTopGaps_AreaTitles(t *testing.T) {
	byArea := map[Area]int{AreaGovernance: 42}
	gaps := buildTopGaps(byArea, 1)
	if len(gaps) != 1 {
		t.Fatalf("expected 1 gap, got %d", len(gaps))
	}
	if gaps[0].AreaTitle == "" {
		t.Error("area_title should not be empty")
	}
	if gaps[0].Score != 42 {
		t.Errorf("score should be 42, got %d", gaps[0].Score)
	}
}

func TestReassessmentCooldown_Duration(t *testing.T) {
	// Sicherstellen, dass die Cooldown-Konstante genau 90 Tage ist.
	want := 90 * 24 * time.Hour
	if reassessmentCooldown != want {
		t.Errorf("reassessmentCooldown = %v, want %v", reassessmentCooldown, want)
	}
}

func TestAssessmentRun_ZeroValue(t *testing.T) {
	var r AssessmentRun
	if r.Answers != nil {
		t.Error("zero-value AssessmentRun.Answers should be nil")
	}
	if r.OverallScore != nil {
		t.Error("zero-value AssessmentRun.OverallScore should be nil")
	}
}
