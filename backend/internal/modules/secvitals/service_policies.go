// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package secvitals

import (
	"context"
	"fmt"
	"strings"
)

// --- Policy Management (FR-CK14) ---

func (s *Service) ListPolicies(ctx context.Context, orgID string) ([]Policy, error) {
	policies, err := s.repo.ListPolicies(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list policies: %w", err)
	}
	if policies == nil {
		policies = []Policy{}
	}
	return policies, nil
}

func (s *Service) GetPolicy(ctx context.Context, orgID, id string) (*Policy, error) {
	return s.repo.GetPolicy(ctx, orgID, id)
}

func (s *Service) CreatePolicy(ctx context.Context, orgID string, in CreatePolicyInput) (*Policy, error) {
	return s.repo.CreatePolicy(ctx, orgID, in)
}

func (s *Service) UpdatePolicy(ctx context.Context, orgID, id string, in UpdatePolicyInput) (*Policy, error) {
	return s.repo.UpdatePolicy(ctx, orgID, id, in)
}

// ListPolicyVersions returns all historical version snapshots for a policy (Migration 076).
func (s *Service) ListPolicyVersions(ctx context.Context, orgID, policyID string) ([]PolicyVersion, error) {
	return s.repo.ListPolicyVersions(ctx, orgID, policyID)
}

// GetPolicyVersion returns a single historical version snapshot (Migration 076).
func (s *Service) GetPolicyVersion(ctx context.Context, orgID, policyID string, version int) (PolicyVersion, error) {
	return s.repo.GetPolicyVersion(ctx, orgID, policyID, version)
}

// --- AI Policy Generator ---

// GeneratePolicyDraft generates a policy draft in German using the configured AI provider.
// It returns the generated text; the caller decides whether to persist it.
func (s *Service) GeneratePolicyDraft(ctx context.Context, orgID string, in GeneratePolicyDraftInput) (string, error) {
	if s.aiClient == nil {
		return "", fmt.Errorf("AI-Features nicht konfiguriert. Bitte VAKT_AI_BASE_URL und VAKT_AI_PROVIDER setzen")
	}

	// Resolve org name if not provided.
	orgName := in.OrgName
	if orgName == "" {
		orgName = fetchOrgName(ctx, s.db, orgID)
		if orgName == "" {
			orgName = "Ihr Unternehmen"
		}
	}

	// Optionally load top-10 framework controls for context.
	frameworkContext := ""
	if in.FrameworkID != "" {
		rows, err := s.db.Query(ctx, `
			SELECT control_id, title FROM ck_controls
			WHERE framework_id = $1::uuid AND org_id = $2::uuid
			ORDER BY weight DESC LIMIT 10`,
			in.FrameworkID, orgID,
		)
		if err == nil {
			defer rows.Close()
			var lines []string
			for rows.Next() {
				var controlID, title string
				if rows.Scan(&controlID, &title) == nil {
					lines = append(lines, controlID+": "+title)
				}
			}
			if len(lines) > 0 {
				frameworkContext = "Relevante ISO 27001 Anforderungen als Kontext:\n" + strings.Join(lines, "\n")
			}
		}
	}

	customContext := ""
	if in.CustomContext != "" {
		customContext = "Zusätzlicher Kontext vom Nutzer:\n" + in.CustomContext
	}

	prompt := fmt.Sprintf(`Du bist ein erfahrener Datenschutz- und IT-Sicherheitsexperte in Deutschland.
Erstelle eine professionelle %s für das Unternehmen "%s".

Die Richtlinie muss:
- Den Anforderungen von ISO 27001:2022 entsprechen
- Auf Deutsch verfasst sein
- Eine klare Struktur haben: Zweck, Geltungsbereich, Grundsätze, Verantwortlichkeiten, Maßnahmen, Gültigkeitsdauer
- Praxistauglich und verständlich für Mitarbeiter ohne technischen Hintergrund sein
- Zwischen 400 und 800 Wörtern lang sein

%s
%s

Erstelle jetzt die vollständige Richtlinie:`,
		in.PolicyType, orgName, frameworkContext, customContext,
	)

	return s.aiClient.Generate(ctx, prompt)
}
