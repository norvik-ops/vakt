// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
)

// --- Questionnaire Builder (Story 29.2) ---

// needsSeed returns true when no templates are present and seeding is required.
func needsSeed(templates []Questionnaire) bool {
	return len(templates) == 0
}

// SeedBuiltinQuestionnaires creates the 3 built-in questionnaire templates if they don't exist.
// Idempotent: does nothing if templates are already present.
func (s *Service) SeedBuiltinQuestionnaires(ctx context.Context, orgID string) error {
	isTemplate := true
	existing, err := s.repo.ListQuestionnaires(ctx, orgID, &isTemplate)
	if err != nil {
		return fmt.Errorf("seed questionnaires: list existing: %w", err)
	}
	if !needsSeed(existing) {
		return nil
	}

	type templateDef struct {
		name      string
		questions []string
	}
	templates := []templateDef{
		{
			name: "NIS2 Lieferanten-Assessment",
			questions: []string{
				"Netzwerksicherheit",
				"Zugriffskontrollen",
				"Incident-Response",
				"Backup",
				"Patch-Management",
				"Supply-Chain-Checks",
				"Kryptographie",
				"Physische Sicherheit",
				"Personalschulungen",
				"Auditlogs",
			},
		},
		{
			name: "DORA IKT-Drittanbieter",
			questions: []string{
				"IKT-Risikomanagement",
				"Incident-Klassifizierung",
				"Resilienztests",
				"Drittanbieter-Verträge",
				"Informationsaustausch",
				"Wiederherstellungstests",
				"Aufsichtsmeldung",
				"Kontrollrahmen",
			},
		},
		{
			name: "ISO 27001 Basischeck",
			questions: []string{
				"Asset-Inventar",
				"Risikobehandlung",
				"Zugriffsrechte",
				"Kryptographie",
				"Lieferantensicherheit",
				"Compliance",
				"Awareness",
				"Audit",
				"Business-Continuity",
				"HR-Sicherheit",
				"Physische Kontrollen",
				"Kommunikationssicherheit",
			},
		},
	}

	for _, t := range templates {
		q, err := s.repo.CreateQuestionnaire(ctx, orgID, t.name, "", true)
		if err != nil {
			return fmt.Errorf("seed questionnaire %q: %w", t.name, err)
		}
		for _, text := range t.questions {
			if _, err := s.repo.CreateQuestion(ctx, q.ID, text, "yes_no", nil, true, nil); err != nil {
				return fmt.Errorf("seed question %q: %w", text, err)
			}
		}
	}
	return nil
}

// ListTemplates seeds built-in templates (if needed) then returns all templates.
func (s *Service) ListTemplates(ctx context.Context, orgID string) ([]Questionnaire, error) {
	if err := s.SeedBuiltinQuestionnaires(ctx, orgID); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("seed built-in questionnaires")
	}
	isTemplate := true
	templates, err := s.repo.ListQuestionnaires(ctx, orgID, &isTemplate)
	if err != nil {
		return nil, fmt.Errorf("list templates: %w", err)
	}
	// Load questions for each template.
	for i := range templates {
		questions, err := s.repo.ListQuestions(ctx, templates[i].ID)
		if err != nil {
			return nil, fmt.Errorf("list template questions: %w", err)
		}
		templates[i].Questions = questions
	}
	return templates, nil
}

// ListQuestionnaires returns questionnaires optionally filtered by is_template.
func (s *Service) ListQuestionnaires(ctx context.Context, orgID string, isTemplate *bool) ([]Questionnaire, error) {
	return s.repo.ListQuestionnaires(ctx, orgID, isTemplate)
}

// GetQuestionnaire returns a single questionnaire with its questions.
func (s *Service) GetQuestionnaire(ctx context.Context, orgID, id string) (*Questionnaire, error) {
	return s.repo.GetQuestionnaire(ctx, orgID, id)
}

// CreateQuestionnaire creates a new questionnaire, cloning from a source if CloneFromID is set.
func (s *Service) CreateQuestionnaire(ctx context.Context, orgID string, in CreateQuestionnaireInput) (*Questionnaire, error) {
	if in.CloneFromID != "" {
		return s.CloneQuestionnaire(ctx, orgID, in.CloneFromID, in.Name)
	}
	return s.repo.CreateQuestionnaire(ctx, orgID, in.Name, in.Description, in.IsTemplate)
}

// CloneQuestionnaire copies a questionnaire and all its questions.
func (s *Service) CloneQuestionnaire(ctx context.Context, orgID, sourceID, name string) (*Questionnaire, error) {
	return s.repo.CloneQuestionnaire(ctx, orgID, sourceID, name)
}

// UpdateQuestionnaire updates questionnaire metadata.
func (s *Service) UpdateQuestionnaire(ctx context.Context, orgID, id string, in UpdateQuestionnaireInput) (*Questionnaire, error) {
	return s.repo.UpdateQuestionnaire(ctx, orgID, id, in.Name, in.Description, in.IsTemplate)
}

// DeleteQuestionnaire removes a questionnaire.
func (s *Service) DeleteQuestionnaire(ctx context.Context, orgID, id string) error {
	return s.repo.DeleteQuestionnaire(ctx, orgID, id)
}

// AddQuestion adds a question to a questionnaire.
// For multiple_choice type, options must be non-empty.
func (s *Service) AddQuestion(ctx context.Context, orgID, questionnaireID string, in CreateQuestionInput) (*Question, error) {
	if in.QuestionType == "multiple_choice" && len(in.Options) == 0 {
		return nil, fmt.Errorf("multiple_choice question requires non-empty options")
	}
	// Verify org owns the questionnaire.
	if _, err := s.repo.GetQuestionnaire(ctx, orgID, questionnaireID); err != nil {
		return nil, fmt.Errorf("questionnaire not found or access denied: %w", err)
	}
	var controlID *string
	if in.ControlID != "" {
		controlID = &in.ControlID
	}
	return s.repo.CreateQuestion(ctx, questionnaireID, in.QuestionText, in.QuestionType, in.Options, in.Required, controlID)
}

// UpdateQuestion updates an existing question.
func (s *Service) UpdateQuestion(ctx context.Context, orgID, questionnaireID, questionID string, in CreateQuestionInput) (*Question, error) {
	if in.QuestionType == "multiple_choice" && len(in.Options) == 0 {
		return nil, fmt.Errorf("multiple_choice question requires non-empty options")
	}
	if _, err := s.repo.GetQuestionnaire(ctx, orgID, questionnaireID); err != nil {
		return nil, fmt.Errorf("questionnaire not found or access denied: %w", err)
	}
	var controlID *string
	if in.ControlID != "" {
		controlID = &in.ControlID
	}
	return s.repo.UpdateQuestion(ctx, questionnaireID, questionID, in.QuestionText, in.QuestionType, in.Options, in.Required, controlID)
}

// DeleteQuestion removes a question from a questionnaire.
func (s *Service) DeleteQuestion(ctx context.Context, orgID, questionnaireID, questionID string) error {
	if _, err := s.repo.GetQuestionnaire(ctx, orgID, questionnaireID); err != nil {
		return fmt.Errorf("questionnaire not found or access denied: %w", err)
	}
	return s.repo.DeleteQuestion(ctx, questionnaireID, questionID)
}

// ReorderQuestions updates the order of questions in a questionnaire.
func (s *Service) ReorderQuestions(ctx context.Context, orgID, questionnaireID string, order []string) error {
	if _, err := s.repo.GetQuestionnaire(ctx, orgID, questionnaireID); err != nil {
		return fmt.Errorf("questionnaire not found or access denied: %w", err)
	}
	return s.repo.ReorderQuestions(ctx, questionnaireID, order)
}
