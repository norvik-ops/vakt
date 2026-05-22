package nis2wizard

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/rs/zerolog/log"
)

// Sprint 28 / S28-4: Multi-Framework-Assessment-Service.
//
// Nutzt dieselbe nis2_anonymous_runs-Tabelle wie der NIS2-Only-Wizard, aber
// mit framework_mode = 'multi' (gespeichert im JSONB-Feld meta).
// So bleiben bestehende NIS2-Flows vollständig unberührt.

const frameworkModeMulti = "multi"

// MultiRun ist der API-View eines Multi-Framework-Runs.
type MultiRun struct {
	Token       string                 `json:"token"`
	Answers     map[string]AnswerEntry `json:"answers"`
	Score       *MultiFrameworkScore   `json:"score,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	ExpiresAt   time.Time              `json:"expires_at"`
}

// MultiFrameworkScore enthält Scores pro Framework + Gesamt-Score.
type MultiFrameworkScore struct {
	NIS2        int            `json:"nis2_score"`
	ISO27001    int            `json:"iso27001_score"`
	DSGVO       int            `json:"dsgvo_score"`
	Overall     int            `json:"overall_score"`
	ByFramework map[string]int `json:"by_framework"`
	TopGaps     []MultiGap     `json:"top_gaps"`
}

// MultiGap beschreibt eine identifizierte Lücke pro Framework/Bereich.
type MultiGap struct {
	Framework  string `json:"framework"`
	Area       string `json:"area"`
	AreaTitle  string `json:"area_title"`
	Score      int    `json:"score"`
}

// runMeta wird im JSONB-meta-Feld der nis2_anonymous_runs-Tabelle gespeichert,
// um framework_mode = 'multi' zu kennzeichnen, ohne das Schema zu ändern.
type runMeta struct {
	FrameworkMode string `json:"framework_mode,omitempty"`
}

// StartMultiRun legt einen neuen Multi-Framework-Run an. Analog zu StartRun.
func (s *Service) StartMultiRun(ctx context.Context, referrer, userAgent, ipHash string) (*MultiRun, error) {
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)
	expires := time.Now().UTC().Add(7 * 24 * time.Hour)

	metaJSON, _ := json.Marshal(runMeta{FrameworkMode: frameworkModeMulti})

	if _, err := s.db.Exec(ctx, `
		INSERT INTO nis2_anonymous_runs
		  (token, answers, referrer, user_agent, ip_hash, expires_at, meta)
		VALUES ($1, '{}'::jsonb, NULLIF($2, ''), NULLIF($3, ''), NULLIF($4, ''), $5, $6)`,
		token, referrer, userAgent, ipHash, expires, metaJSON,
	); err != nil {
		// Fallback: Tabelle hat möglicherweise noch keine meta-Spalte — insert ohne meta.
		if _, err2 := s.db.Exec(ctx, `
			INSERT INTO nis2_anonymous_runs
			  (token, answers, referrer, user_agent, ip_hash, expires_at)
			VALUES ($1, '{}'::jsonb, NULLIF($2, ''), NULLIF($3, ''), NULLIF($4, ''), $5)`,
			token, referrer, userAgent, ipHash, expires,
		); err2 != nil {
			return nil, fmt.Errorf("insert multi run: %w", err2)
		}
		log.Warn().Err(err).Msg("nis2wizard.multi: meta column not available, inserted without meta")
	}

	return &MultiRun{Token: token, Answers: map[string]AnswerEntry{}, ExpiresAt: expires}, nil
}

// AnswerMulti speichert eine Antwort für einen Multi-Framework-Run.
func (s *Service) AnswerMulti(ctx context.Context, token, questionID string, value int, comment string) (*MultiRun, error) {
	if value < 0 || value > 4 {
		return nil, fmt.Errorf("value must be 0..4")
	}
	if !validMultiFrameworkQuestionID(questionID) {
		return nil, fmt.Errorf("unknown multi-framework question id: %s", questionID)
	}

	run, err := s.loadMultiRun(ctx, token)
	if err != nil {
		return nil, err
	}
	run.Answers[questionID] = AnswerEntry{Value: value, Comment: comment}

	score := ComputeMultiFrameworkScore(run.Answers)
	run.Score = &score

	if len(run.Answers) >= len(MultiFrameworkQuestions) {
		now := time.Now().UTC()
		run.CompletedAt = &now
	}

	answersJSON, _ := json.Marshal(run.Answers)
	scoreJSON, _ := json.Marshal(score)

	if _, err := s.db.Exec(ctx, `
		UPDATE nis2_anonymous_runs
		SET answers    = $1,
		    score      = $2,
		    score_by_area = $3,
		    completed_at = $4
		WHERE token = $5`,
		answersJSON, score.Overall, scoreJSON, run.CompletedAt, token,
	); err != nil {
		return nil, fmt.Errorf("update multi run: %w", err)
	}
	return run, nil
}

// LoadMultiRunResult lädt einen Multi-Framework-Run und gibt ihn zurück.
func (s *Service) LoadMultiRunResult(ctx context.Context, token string) (*MultiRun, error) {
	return s.loadMultiRun(ctx, token)
}

// loadMultiRun lädt einen anonymen Run via Token (Multi-Framework-Variante).
func (s *Service) loadMultiRun(ctx context.Context, token string) (*MultiRun, error) {
	var (
		answersJSON  []byte
		scoreJSON    []byte
		completedAt  *time.Time
		expiresAt    time.Time
	)
	if err := s.db.QueryRow(ctx, `
		SELECT answers, score_by_area, completed_at, expires_at
		FROM nis2_anonymous_runs
		WHERE token = $1 AND expires_at > NOW()`,
		token,
	).Scan(&answersJSON, &scoreJSON, &completedAt, &expiresAt); err != nil {
		return nil, fmt.Errorf("load multi run: %w", err)
	}

	run := &MultiRun{
		Token:       token,
		CompletedAt: completedAt,
		ExpiresAt:   expiresAt,
	}

	if err := json.Unmarshal(answersJSON, &run.Answers); err != nil || run.Answers == nil {
		run.Answers = map[string]AnswerEntry{}
	}

	// Rekonstruiere den Score aus den gespeicherten Antworten (deterministisch).
	if len(run.Answers) > 0 {
		score := ComputeMultiFrameworkScore(run.Answers)
		run.Score = &score
	}

	return run, nil
}

// ComputeMultiFrameworkScore berechnet Scores für alle drei Frameworks.
//
// Algorithmus analog zu computeScore: gewichteter Schnitt pro Framework.
// Cross-Framework-Fragen zählen für alle aufgeführten Frameworks.
// TopGaps: die 3 am niedrigsten gescorten Bereiche.
func ComputeMultiFrameworkScore(answers map[string]AnswerEntry) MultiFrameworkScore {
	type frameworkAccum struct {
		earned float64
		max    float64
	}
	fwAccum := map[string]*frameworkAccum{
		FrameworkNIS2:     {},
		FrameworkISO27001: {},
		FrameworkDSGVOTOM: {},
	}

	type areaKey struct{ fw, area string }
	areaEarned := map[areaKey]float64{}
	areaMax := map[areaKey]float64{}

	for _, q := range MultiFrameworkQuestions {
		ans, ok := answers[q.ID]
		if !ok {
			continue
		}
		earned := q.Weight * float64(ans.Value)
		max := q.Weight * 4.0

		// Primäres Framework
		fwAccum[q.Framework].earned += earned
		fwAccum[q.Framework].max += max
		ak := areaKey{fw: q.Framework, area: q.Area}
		areaEarned[ak] += earned
		areaMax[ak] += max

		// Cross-Framework-Fragen zählen für alle aufgelisteten Frameworks mit.
		for _, cf := range q.CrossFrameworks {
			if acc, exists := fwAccum[cf]; exists {
				acc.earned += earned
				acc.max += max
			}
		}
	}

	toScore := func(earned, max float64) int {
		if max == 0 {
			return 0
		}
		return int(earned * 100 / max)
	}

	nis2Score := toScore(fwAccum[FrameworkNIS2].earned, fwAccum[FrameworkNIS2].max)
	isoScore := toScore(fwAccum[FrameworkISO27001].earned, fwAccum[FrameworkISO27001].max)
	dsgvoScore := toScore(fwAccum[FrameworkDSGVOTOM].earned, fwAccum[FrameworkDSGVOTOM].max)

	// Overall = Durchschnitt über alle drei Frameworks (gleichwertig).
	overallEarned := fwAccum[FrameworkNIS2].earned + fwAccum[FrameworkISO27001].earned + fwAccum[FrameworkDSGVOTOM].earned
	overallMax := fwAccum[FrameworkNIS2].max + fwAccum[FrameworkISO27001].max + fwAccum[FrameworkDSGVOTOM].max
	overall := toScore(overallEarned, overallMax)

	byFramework := map[string]int{
		FrameworkNIS2:     nis2Score,
		FrameworkISO27001: isoScore,
		FrameworkDSGVOTOM: dsgvoScore,
	}

	// Top-Gaps: alle Bereiche mit Score < 100, sortiert aufsteigend, top 5.
	type areaScoreEntry struct {
		fw    string
		area  string
		score int
	}
	var areaScores []areaScoreEntry
	for ak, max := range areaMax {
		if max == 0 {
			continue
		}
		s := toScore(areaEarned[ak], max)
		areaScores = append(areaScores, areaScoreEntry{fw: ak.fw, area: ak.area, score: s})
	}
	sort.Slice(areaScores, func(i, j int) bool {
		return areaScores[i].score < areaScores[j].score
	})

	topN := 5
	if len(areaScores) < topN {
		topN = len(areaScores)
	}
	topGaps := make([]MultiGap, 0, topN)
	for i := 0; i < topN; i++ {
		ae := areaScores[i]
		title := MultiFrameworkAreaTitles[ae.area]
		if title == "" {
			title = ae.area
		}
		topGaps = append(topGaps, MultiGap{
			Framework: ae.fw,
			Area:      ae.area,
			AreaTitle: title,
			Score:     ae.score,
		})
	}

	return MultiFrameworkScore{
		NIS2:        nis2Score,
		ISO27001:    isoScore,
		DSGVO:       dsgvoScore,
		Overall:     overall,
		ByFramework: byFramework,
		TopGaps:     topGaps,
	}
}
