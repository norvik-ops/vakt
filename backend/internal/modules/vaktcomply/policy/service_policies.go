// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package policy

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/matharnica/vakt/internal/db"
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

// PolicyApprovalEvent says what an update did to the approval, so the caller
// can write the matching audit-trail entry.
type PolicyApprovalEvent int

const (
	// ApprovalUnchanged: the update did not touch approved_by / approved_at.
	ApprovalUnchanged PolicyApprovalEvent = iota
	// ApprovalGranted: the policy moved to active; the actor is the approver.
	ApprovalGranted
	// ApprovalRevoked: the policy left active; the approval was cleared.
	ApprovalRevoked
)

// ErrApproverUnknown is returned when a policy would be approved without an
// authenticated user identity — an approval nobody can be held to is refused.
var ErrApproverUnknown = errors.New("policy approval requires an authenticated user")

// approvalChange is the write the repository applies to approved_by/approved_at.
// Touch=false leaves both columns as they are; Touch=true writes By/At (an empty
// By and a nil At clear them).
type approvalChange struct {
	Touch bool
	By    string
	At    *time.Time
	Event PolicyApprovalEvent
}

// approvalTransition decides the approval write from the status transition.
//
//   - into active (from draft, archived): approver = actorID, approved_at = now.
//   - out of active (to draft, archived): both cleared. A policy that is no longer
//     in force has no valid approval; the next activation needs a new one. The
//     historic approval stays only in the audit log (action 'approve');
//     ck_policy_versions does not record who approved.
//   - everything else (active → active, draft → archived, …): unchanged. Editing
//     the text of an active policy does not re-approve it.
//
// actorID is the authenticated user id from the request context — never a value
// from the request body.
func approvalTransition(prevStatus, newStatus, actorID string, now time.Time) (approvalChange, error) {
	const active = "active"
	switch {
	case newStatus == active && prevStatus != active:
		if actorID == "" {
			return approvalChange{}, ErrApproverUnknown
		}
		at := now
		return approvalChange{Touch: true, By: actorID, At: &at, Event: ApprovalGranted}, nil
	case newStatus != active && prevStatus == active:
		return approvalChange{Touch: true, Event: ApprovalRevoked}, nil
	default:
		return approvalChange{Event: ApprovalUnchanged}, nil
	}
}

// UpdatePolicy applies a policy update. actorID is the authenticated user from the
// request context; it is the only source for who approved the policy (R1-KW-60
// fix: last_updated_by is client free text and never counts as an approval).
func (s *Service) UpdatePolicy(ctx context.Context, orgID, id, actorID string, in UpdatePolicyInput) (*Policy, PolicyApprovalEvent, error) {
	now := time.Now().UTC()
	var event PolicyApprovalEvent
	p, err := s.repo.UpdatePolicy(ctx, orgID, id, in, func(prevStatus string) (approvalChange, error) {
		change, err := approvalTransition(prevStatus, in.Status, actorID, now)
		event = change.Event
		return change, err
	})
	if err != nil {
		return nil, ApprovalUnchanged, err
	}
	return p, event, nil
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
		controls, err := s.q.ListCKTopControlsByFramework(ctx, db.ListCKTopControlsByFrameworkParams{
			FrameworkID: in.FrameworkID,
			OrgID:       orgID,
		})
		if err == nil {
			var lines []string
			for _, c := range controls {
				lines = append(lines, c.ControlID+": "+c.Title)
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
