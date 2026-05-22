package nis2wizard

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Sprint 19 / S19-1 + S19-3 + S19-6: Public Wizard ohne Auth.
//
// Flow:
//   1. POST /public/nis2-assessment/start  → erzeugt anonymen Run + Magic-Token
//   2. POST /public/nis2-assessment/answer → speichert eine Antwort
//   3. GET  /public/nis2-assessment/result → Live-Score (auch unfertig)
//   4. POST /public/nis2-assessment/migrate-to-org → bei Sign-up: Run-Migration

// Service kapselt DB-Zugriff. Keine OrgID, weil anonym.
type Service struct {
	db *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

// Run ist der API-View eines anonymen Runs.
type Run struct {
	Token       string                  `json:"token"`
	Answers     map[string]AnswerEntry  `json:"answers"`
	Score       *int                    `json:"score,omitempty"`
	ScoreByArea map[Area]int            `json:"score_by_area,omitempty"`
	CompletedAt *time.Time              `json:"completed_at,omitempty"`
	ExpiresAt   time.Time               `json:"expires_at"`
}

// AnswerEntry — eine Antwort pro Frage.
type AnswerEntry struct {
	Value   int    `json:"value"`             // 0..4
	Comment string `json:"comment,omitempty"` // optional
}

// StartRun legt einen neuen anonymen Run an + erzeugt einen Token. ipHash
// ist der sha256 der Klient-IP (nicht der Klartext — DSGVO).
func (s *Service) StartRun(ctx context.Context, referrer, userAgent, ipHash string) (*Run, error) {
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)
	expires := time.Now().UTC().Add(7 * 24 * time.Hour)
	if _, err := s.db.Exec(ctx, `
		INSERT INTO nis2_anonymous_runs (token, answers, referrer, user_agent, ip_hash, expires_at)
		VALUES ($1, '{}'::jsonb, NULLIF($2, ''), NULLIF($3, ''), NULLIF($4, ''), $5)`,
		token, referrer, userAgent, ipHash, expires,
	); err != nil {
		return nil, fmt.Errorf("insert run: %w", err)
	}
	return &Run{Token: token, Answers: map[string]AnswerEntry{}, ExpiresAt: expires}, nil
}

// Answer speichert eine Antwort. Validiert questionID + value-Range.
func (s *Service) Answer(ctx context.Context, token, questionID string, value int, comment string) (*Run, error) {
	if value < 0 || value > 4 {
		return nil, fmt.Errorf("value must be 0..4")
	}
	if !validQuestionID(questionID) {
		return nil, fmt.Errorf("unknown question id: %s", questionID)
	}
	run, err := s.LoadRun(ctx, token)
	if err != nil {
		return nil, err
	}
	run.Answers[questionID] = AnswerEntry{Value: value, Comment: comment}

	// Score (re-)berechnen — auch wenn nicht alle Fragen beantwortet sind,
	// gibt der Live-Score-Hinweis dem User Orientierung.
	score, byArea := computeScore(run.Answers)
	run.Score = &score
	run.ScoreByArea = byArea
	if len(run.Answers) >= len(Questions) {
		now := time.Now().UTC()
		run.CompletedAt = &now
	}

	answersJSON, _ := json.Marshal(run.Answers)
	byAreaJSON, _ := json.Marshal(byArea)
	if _, err := s.db.Exec(ctx, `
		UPDATE nis2_anonymous_runs
		SET answers = $1, score = $2, score_by_area = $3,
		    completed_at = $4
		WHERE token = $5`,
		answersJSON, score, byAreaJSON, run.CompletedAt, token,
	); err != nil {
		return nil, fmt.Errorf("update run: %w", err)
	}
	return run, nil
}

// LoadRun lädt einen anonymen Run via Token. Liefert Fehler wenn abgelaufen.
func (s *Service) LoadRun(ctx context.Context, token string) (*Run, error) {
	var (
		answersJSON   []byte
		score         *int
		byAreaJSON    []byte
		completedAt   *time.Time
		expiresAt     time.Time
	)
	if err := s.db.QueryRow(ctx, `
		SELECT answers, score, score_by_area, completed_at, expires_at
		FROM nis2_anonymous_runs
		WHERE token = $1 AND expires_at > NOW()`,
		token,
	).Scan(&answersJSON, &score, &byAreaJSON, &completedAt, &expiresAt); err != nil {
		return nil, fmt.Errorf("load run: %w", err)
	}
	run := &Run{
		Token:       token,
		Score:       score,
		CompletedAt: completedAt,
		ExpiresAt:   expiresAt,
	}
	if err := json.Unmarshal(answersJSON, &run.Answers); err != nil || run.Answers == nil {
		run.Answers = map[string]AnswerEntry{}
	}
	if len(byAreaJSON) > 0 {
		_ = json.Unmarshal(byAreaJSON, &run.ScoreByArea)
	}
	return run, nil
}

// MigrateToOrg ist der Sign-up-Flow (S19-6): nach Account-Erstellung mit
// gültigem Token wird der anonyme Run als ck_nis2_assessment in die Org
// migriert. Returnt die ID des migrierten Assessments.
func (s *Service) MigrateToOrg(ctx context.Context, token, orgID, userID string) (string, error) {
	run, err := s.LoadRun(ctx, token)
	if err != nil {
		return "", fmt.Errorf("load run: %w", err)
	}
	if run.CompletedAt == nil {
		return "", fmt.Errorf("assessment not yet completed")
	}
	answersJSON, _ := json.Marshal(run.Answers)
	byAreaJSON, _ := json.Marshal(run.ScoreByArea)
	var id string
	if err := s.db.QueryRow(ctx, `
		INSERT INTO ck_nis2_assessments
		  (org_id, user_id, answers, score, score_by_area, source, completed_at)
		VALUES ($1::uuid, NULLIF($2, '')::uuid, $3, $4, $5, 'wizard_migrated_from_anonymous', $6)
		RETURNING id::text`,
		orgID, userID, answersJSON, *run.Score, byAreaJSON, run.CompletedAt,
	).Scan(&id); err != nil {
		return "", fmt.Errorf("migrate: %w", err)
	}
	// Anonymer Run kann jetzt gelöscht werden (Migration ist abgeschlossen).
	_, _ = s.db.Exec(ctx, `DELETE FROM nis2_anonymous_runs WHERE token = $1`, token)
	return id, nil
}

// HashIP wrapped sha256(ip + secret) für DSGVO-konformes IP-Tracking. Der
// secret-Salt verhindert Rainbow-Table-Lookups; er kommt aus VAKT_SECRET_KEY
// (gemeinsamer App-Secret).
func HashIP(ip, secret string) string {
	if ip == "" {
		return ""
	}
	h := sha256.New()
	h.Write([]byte(secret))
	h.Write([]byte{0})
	h.Write([]byte(ip))
	return hex.EncodeToString(h.Sum(nil))
}

// computeScore berechnet den Gesamt-Score (0..100) + per-Area-Scores.
// Algorithmus: pro Antwort weight * value. Max-Score pro Frage = weight*4.
// Score = sum(weight*value) / sum(weight*4) * 100.
// Per-Area analog, aber Bereiche ohne Antworten werden NICHT als 0 gewertet
// (sonst würde ein unfertiger Run optisch viel zu schlecht aussehen).
func computeScore(answers map[string]AnswerEntry) (int, map[Area]int) {
	var totalEarned, totalMax int
	areaEarned := map[Area]int{}
	areaMax := map[Area]int{}
	for _, q := range Questions {
		ans, ok := answers[q.ID]
		if !ok {
			continue
		}
		earned := q.Weight * ans.Value
		max := q.Weight * 4
		totalEarned += earned
		totalMax += max
		areaEarned[q.Area] += earned
		areaMax[q.Area] += max
	}
	overall := 0
	if totalMax > 0 {
		overall = totalEarned * 100 / totalMax
	}
	byArea := map[Area]int{}
	for _, a := range AllAreas {
		if areaMax[a] > 0 {
			byArea[a] = areaEarned[a] * 100 / areaMax[a]
		}
	}
	return overall, byArea
}

// TopGaps liefert die N am niedrigsten gescorten Areas + zugehörige
// Top-Verbesserungs-Fragen. Für die Result-Page der Public-Wizard und für
// den späteren Auto-Mapping-Schritt nach Sign-up.
func (r *Run) TopGaps(n int) []Gap {
	if r.ScoreByArea == nil {
		return nil
	}
	type areaScore struct {
		area  Area
		score int
	}
	var sorted []areaScore
	for a, s := range r.ScoreByArea {
		sorted = append(sorted, areaScore{a, s})
	}
	// Sortieren: niedrigster Score zuerst (einfach Bubble — N ist klein).
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

// Gap ist eine identifizierte Lücke im Score-Output.
type Gap struct {
	Area      Area   `json:"area"`
	AreaTitle string `json:"area_title"`
	Score     int    `json:"score"`
}

// validQuestionID prüft gegen die Questions-Liste.
func validQuestionID(id string) bool {
	for _, q := range Questions {
		if q.ID == id {
			return true
		}
	}
	return false
}
