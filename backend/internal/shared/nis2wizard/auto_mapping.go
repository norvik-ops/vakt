package nis2wizard

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// Sprint 22 / S22-5: Auto-Mapping nach Sign-up.
//
// Wenn ein User über das Magic-Token aus dem anonymen Wizard heraus einen
// Account erstellt, werden seine 30 NIS2-Antworten als initialer
// `manual_status` auf die NIS2-Controls der neuen Org projiziert.
//
// Mapping-Heuristik value 0..4 → manual_status:
//   0 (nicht implementiert)         → not_implemented
//   1 (in Planung)                  → not_implemented
//   2 (teilweise umgesetzt)         → partial
//   3 (weitgehend umgesetzt)        → implemented
//   4 (vollständig + getestet)      → implemented
//
// Mapping pro Frage: Question.NIS2Ref ("Art. 21 Abs. 2 a)") matcht
// gegen ck_controls.description ODER ck_controls.control_id via Substring
// — pragmatische Heuristik, im Marketing als „Übernommen aus NIS2-Wizard"
// gekennzeichnet, damit der User die Aussagen prüfen kann.
//
// Spart Customer ~30 Minuten manueller Setup-Arbeit beim ersten Login.
// CE-Feature (kein License-Gate), nutzt die in Sprint 19 gepflegte
// Wizard-Antwort-Map.

// AutoMapToControls projiziert die Antworten eines anonymen Runs als
// initialer `manual_status` auf die NIS2-Controls der Org.
// Nur Controls ohne aktiven manual_status (NULL oder 'not_implemented')
// werden überschrieben — bestehende Eingaben des Users werden respektiert.
//
// Returns die Anzahl der gemappten Controls und Liste der gemappten
// Question-IDs (für Audit-/UI-Marker).
func AutoMapToControls(ctx context.Context, pool *pgxpool.Pool, orgID string, answers map[string]AnswerEntry) (int, []string, error) {
	if orgID == "" || len(answers) == 0 {
		return 0, nil, nil
	}

	mappedQuestionIDs := make([]string, 0)
	mappedCount := 0

	for _, q := range Questions {
		ans, ok := answers[q.ID]
		if !ok {
			continue
		}
		status := valueToStatus(ans.Value)

		// UPDATE-Strategie: nur Controls anpassen, die noch nicht vom User
		// gesetzt wurden (manual_status IS NULL oder = 'not_implemented').
		// Substring-Match auf description ODER control_id für NIS2-Ref.
		tag, err := pool.Exec(ctx, `
			UPDATE ck_controls c
			SET manual_status = $1
			FROM ck_frameworks f
			WHERE c.framework_id = f.id
			  AND c.org_id = $2::uuid
			  AND f.name = 'NIS2'
			  AND (c.manual_status IS NULL OR c.manual_status = 'not_implemented')
			  AND (
			      c.description ILIKE '%' || $3 || '%'
			      OR c.control_id ILIKE '%' || $3 || '%'
			  )`,
			status, orgID, q.NIS2Ref,
		)
		if err != nil {
			log.Warn().Err(err).
				Str("org_id", orgID).Str("question_id", q.ID).Str("nis2_ref", q.NIS2Ref).
				Msg("nis2.automap: control update failed")
			continue
		}
		if tag.RowsAffected() > 0 {
			mappedCount += int(tag.RowsAffected())
			mappedQuestionIDs = append(mappedQuestionIDs, q.ID)
		}
	}

	log.Info().
		Str("org_id", orgID).
		Int("controls_mapped", mappedCount).
		Int("questions_used", len(mappedQuestionIDs)).
		Msg("nis2.automap: complete")

	return mappedCount, mappedQuestionIDs, nil
}

// valueToStatus mapped die 0-4-Skala auf den ck_controls.manual_status-Enum.
func valueToStatus(value int) string {
	switch {
	case value <= 1:
		return "not_implemented"
	case value == 2:
		return "partial"
	default: // 3, 4
		return "implemented"
	}
}

// MigrateAndAutoMap kombiniert Service.MigrateToOrg + AutoMapToControls
// in einem Schritt. Wird vom Sign-up-Endpoint aufgerufen — ein einzelner
// Aufruf statt zwei separate.
func (s *Service) MigrateAndAutoMap(ctx context.Context, token, orgID, userID string) (assessmentID string, mappedCount int, err error) {
	// 1. Run laden (für Auto-Mapping).
	run, loadErr := s.LoadRun(ctx, token)
	if loadErr != nil {
		return "", 0, fmt.Errorf("load run: %w", loadErr)
	}
	if run.CompletedAt == nil {
		return "", 0, fmt.Errorf("assessment not yet completed")
	}

	// 2. In ck_nis2_assessments persistieren (bestehende Methode).
	assessmentID, err = s.MigrateToOrg(ctx, token, orgID, userID)
	if err != nil {
		return "", 0, err
	}

	// 3. Auto-Mapping auf NIS2-Controls (Best-Effort, blockt Migration nicht).
	mapped, _, mapErr := AutoMapToControls(ctx, s.db, orgID, run.Answers)
	if mapErr != nil {
		log.Warn().Err(mapErr).Str("org_id", orgID).Msg("nis2.automap: failed but migration kept")
	}
	mappedCount = mapped

	return assessmentID, mappedCount, nil
}
