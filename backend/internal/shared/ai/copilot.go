// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package ai

import (
	"context"
	"fmt"
)

// DraftPolicy generates a policy draft for the given control or topic using
// the configured local LLM. The result is a Markdown document ready for an
// admin to edit and publish — the LLM never has access to the org's database
// directly, only to the topic string the caller passes in.
//
// Privacy: this call runs against the configured AI provider (local Ollama
// by default). If the operator points OPENAI_BASE_URL at a cloud endpoint,
// the topic string leaves the instance — by Vakt's defaults, it doesn't.
func (s *Service) DraftPolicy(ctx context.Context, topic, framework string) (string, error) {
	if topic == "" {
		return "", fmt.Errorf("topic is required")
	}
	system := `Du bist ein erfahrener IT-Security-Berater für DACH-Mittelstand. ` +
		`Erstelle eine schlanke, umsetzbare Sicherheitsrichtlinie in deutscher Sprache. ` +
		`Format: Markdown mit Abschnitten "Zweck", "Geltungsbereich", "Anforderungen", ` +
		`"Rollen & Verantwortlichkeiten", "Verstöße". Vermeide Floskeln und Marketing-Sprache. ` +
		`Halte den Text unter 600 Wörtern.`

	prompt := fmt.Sprintf(
		"Erstelle einen Richtlinien-Entwurf zum Thema: %s.",
		topic,
	)
	if framework != "" {
		prompt += fmt.Sprintf("\nDie Richtlinie soll den Anforderungen von %s genügen.", framework)
	}

	return s.client.GenerateWithSystem(ctx, system, prompt)
}

// IncidentResponseGuide produces a step-by-step response checklist for an
// incident description. Useful at the moment an incident is created — the
// operator can paste it directly into the incident notes.
func (s *Service) IncidentResponseGuide(ctx context.Context, incidentSummary, incidentType string) (string, error) {
	if incidentSummary == "" {
		return "", fmt.Errorf("incident summary is required")
	}
	system := `Du bist ein erfahrener Incident-Response-Coach. ` +
		`Du beantwortest mit einer nummerierten Sofort-Checkliste in deutscher Sprache. ` +
		`Format: 5-8 konkrete Schritte. Pro Schritt: ein Satz Aktion + (in Klammern) ein Satz Begründung/Risiko. ` +
		`Beziehe gesetzliche Fristen ein, wenn relevant (NIS2: T+24h Erstmeldung, T+72h Update; ` +
		`DSGVO Art. 33: 72h-Meldepflicht bei Personenbezug; DORA: T+4h Erstmeldung kritisch). ` +
		`Keine Floskeln, kein "wir empfehlen" — direkte Imperative.`

	prompt := fmt.Sprintf("Vorfall: %s.", incidentSummary)
	if incidentType != "" {
		prompt += fmt.Sprintf("\nTyp: %s.", incidentType)
	}

	return s.client.GenerateWithSystem(ctx, system, prompt)
}
