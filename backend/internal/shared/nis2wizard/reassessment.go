package nis2wizard

// Sprint 28 / S28-3: Re-Assessment-History für eingeloggte Orgs.
//
// Flow:
//   1. POST /secvitals/reassess         → CreateReassessmentRun (ProGate: FeatureNIS2Reporting)
//   2. POST /secvitals/reassess/:id/answer → SaveReassessmentAnswer
//   3. GET  /secvitals/reassess/:id/result → GetReassessmentResult
//   4. GET  /secvitals/history          → GetReassessmentHistory (ProGate: FeatureNIS2Reporting)
//
// 90-Tage-Cooldown: eine Org kann keinen neuen Run starten, wenn der letzte
// Run weniger als 90 Tage alt ist. Fehler: 409 CONFLICT mit Fehlermeldung.

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

// AssessmentRun ist die persistierte Darstellung eines Re-Assessment-Runs
// einer eingeloggten Org.
type AssessmentRun struct {
	ID           string         `json:"id"`
	OrgID        string         `json:"org_id"`
	RunNumber    int            `json:"run_number"`
	Answers      map[string]AnswerEntry `json:"answers"`
	OverallScore *int           `json:"overall_score,omitempty"`
	ScoreByArea  map[Area]int   `json:"score_by_area,omitempty"`
	Gaps         []Gap          `json:"top_gaps,omitempty"`
	CompletedAt  *time.Time     `json:"completed_at,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
}

// reassessmentCooldown ist der Mindestabstand zwischen zwei Runs derselben Org.
const reassessmentCooldown = 90 * 24 * time.Hour

// CreateReassessmentRun legt einen neuen Assessment-Run für eine Org an.
// Gibt ErrCooldown zurück, wenn der letzte Run weniger als 90 Tage alt ist.
func (s *Service) CreateReassessmentRun(ctx context.Context, orgID string) (string, error) {
	// Letzten Run prüfen.
	var lastCreated time.Time
	err := s.db.QueryRow(ctx,
		`SELECT created_at FROM ck_nis2_assessment_runs
		 WHERE org_id = $1::uuid
		 ORDER BY created_at DESC
		 LIMIT 1`,
		orgID,
	).Scan(&lastCreated)
	if err == nil {
		// Kein pgx.ErrNoRows → Run existiert. Cooldown prüfen.
		if time.Since(lastCreated) < reassessmentCooldown {
			next := lastCreated.Add(reassessmentCooldown)
			return "", fmt.Errorf("cooldown: next assessment possible after %s", next.UTC().Format(time.RFC3339))
		}
	}
	// err != nil → kein Run vorhanden, ist OK.

	// run_number = bisherige Anzahl Runs + 1.
	var runNumber int
	_ = s.db.QueryRow(ctx,
		`SELECT COALESCE(MAX(run_number), 0) + 1 FROM ck_nis2_assessment_runs WHERE org_id = $1::uuid`,
		orgID,
	).Scan(&runNumber)

	var id string
	if err := s.db.QueryRow(ctx,
		`INSERT INTO ck_nis2_assessment_runs (org_id, run_number, answers)
		 VALUES ($1::uuid, $2, '{}')
		 RETURNING id::text`,
		orgID, runNumber,
	).Scan(&id); err != nil {
		return "", fmt.Errorf("create reassessment run: %w", err)
	}
	log.Info().Str("org_id", orgID).Str("run_id", id).Int("run_number", runNumber).
		Msg("nis2: reassessment run created")
	return id, nil
}

// SaveReassessmentAnswer speichert eine Antwort für einen existierenden Run.
// Berechnet Score neu nach jeder Antwort. Setzt completed_at, wenn alle
// 30 Fragen beantwortet wurden.
func (s *Service) SaveReassessmentAnswer(ctx context.Context, orgID, runID, questionID string, value int, comment string) (*AssessmentRun, error) {
	if value < 0 || value > 4 {
		return nil, fmt.Errorf("value must be 0..4")
	}
	if !validQuestionID(questionID) {
		return nil, fmt.Errorf("unknown question id: %s", questionID)
	}

	run, err := s.loadReassessmentRun(ctx, orgID, runID)
	if err != nil {
		return nil, err
	}
	if run.CompletedAt != nil {
		return nil, fmt.Errorf("run already completed")
	}

	run.Answers[questionID] = AnswerEntry{Value: value, Comment: comment}

	score, byArea := computeScore(run.Answers)
	run.OverallScore = &score
	run.ScoreByArea = byArea

	var completedAt *time.Time
	if len(run.Answers) >= len(Questions) {
		now := time.Now().UTC()
		completedAt = &now
		run.CompletedAt = completedAt
		run.Gaps = buildTopGaps(byArea, 3)
	}

	answersJSON, _ := json.Marshal(run.Answers)
	byAreaJSON, _ := json.Marshal(byArea)
	var gapsJSON []byte
	if run.Gaps != nil {
		gapsJSON, _ = json.Marshal(run.Gaps)
	}

	if _, err := s.db.Exec(ctx,
		`UPDATE ck_nis2_assessment_runs
		 SET answers = $1, overall_score = $2, score_by_area = $3,
		     top_gaps = $4, completed_at = $5
		 WHERE id = $6::uuid AND org_id = $7::uuid`,
		answersJSON, score, byAreaJSON, gapsJSON, completedAt, runID, orgID,
	); err != nil {
		return nil, fmt.Errorf("save reassessment answer: %w", err)
	}
	return run, nil
}

// GetReassessmentResult gibt den aktuellen Stand eines Runs zurück, inkl.
// Top-Gaps wenn abgeschlossen.
func (s *Service) GetReassessmentResult(ctx context.Context, orgID, runID string) (*AssessmentRun, error) {
	return s.loadReassessmentRun(ctx, orgID, runID)
}

// GetReassessmentHistory gibt alle Runs einer Org zurück, neuester zuerst.
func (s *Service) GetReassessmentHistory(ctx context.Context, orgID string) ([]AssessmentRun, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id::text, org_id::text, run_number, answers, overall_score,
		        score_by_area, top_gaps, completed_at, created_at
		 FROM ck_nis2_assessment_runs
		 WHERE org_id = $1::uuid
		 ORDER BY created_at DESC`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list reassessment runs: %w", err)
	}
	defer rows.Close()

	var runs []AssessmentRun
	for rows.Next() {
		r, err := scanReassessmentRun(rows)
		if err != nil {
			log.Warn().Err(err).Str("org_id", orgID).Msg("nis2: scan reassessment run failed")
			continue
		}
		runs = append(runs, r)
	}
	if runs == nil {
		runs = []AssessmentRun{}
	}
	return runs, nil
}

// loadReassessmentRun lädt einen einzelnen Run per ID + Org (Ownership-Check).
func (s *Service) loadReassessmentRun(ctx context.Context, orgID, runID string) (*AssessmentRun, error) {
	row := s.db.QueryRow(ctx,
		`SELECT id::text, org_id::text, run_number, answers, overall_score,
		        score_by_area, top_gaps, completed_at, created_at
		 FROM ck_nis2_assessment_runs
		 WHERE id = $1::uuid AND org_id = $2::uuid`,
		runID, orgID,
	)
	r, err := scanReassessmentRun(row)
	if err != nil {
		return nil, fmt.Errorf("load reassessment run: %w", err)
	}
	return &r, nil
}

// scanner ist ein gemeinsames Interface für pgx.Row und pgx.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanReassessmentRun(row scanner) (AssessmentRun, error) {
	var (
		r            AssessmentRun
		answersJSON  []byte
		byAreaJSON   []byte
		gapsJSON     []byte
		completedAt  *time.Time
	)
	if err := row.Scan(
		&r.ID, &r.OrgID, &r.RunNumber,
		&answersJSON, &r.OverallScore,
		&byAreaJSON, &gapsJSON,
		&completedAt, &r.CreatedAt,
	); err != nil {
		return AssessmentRun{}, err
	}
	r.CompletedAt = completedAt

	if err := json.Unmarshal(answersJSON, &r.Answers); err != nil || r.Answers == nil {
		r.Answers = map[string]AnswerEntry{}
	}
	if len(byAreaJSON) > 0 {
		_ = json.Unmarshal(byAreaJSON, &r.ScoreByArea)
	}
	if len(gapsJSON) > 0 {
		_ = json.Unmarshal(gapsJSON, &r.Gaps)
	}
	return r, nil
}

// buildTopGaps erzeugt die Top-N-Gaps aus einem byArea-Score-Map.
func buildTopGaps(byArea map[Area]int, n int) []Gap {
	type areaScore struct {
		area  Area
		score int
	}
	var sorted []areaScore
	for a, s := range byArea {
		sorted = append(sorted, areaScore{a, s})
	}
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].score < sorted[i].score {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	if n > len(sorted) {
		n = len(sorted)
	}
	out := make([]Gap, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, Gap{
			Area:      sorted[i].area,
			AreaTitle: AreaTitle(sorted[i].area),
			Score:     sorted[i].score,
		})
	}
	return out
}
