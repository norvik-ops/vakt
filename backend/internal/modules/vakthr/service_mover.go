// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S69-4: JML Mover Workflow — role-change checklists (revoke/grant/verify).

package vakthr

import (
	"context"
	"fmt"
	"time"
)

// MoverEvent is one role-change record in hr_mover_events.
type MoverEvent struct {
	ID             string     `json:"id"`
	OrgID          string     `json:"org_id"`
	EmployeeID     string     `json:"employee_id"`
	FromDepartment string     `json:"from_department,omitempty"`
	FromJobTitle   string     `json:"from_job_title,omitempty"`
	ToDepartment   string     `json:"to_department"`
	ToJobTitle     string     `json:"to_job_title"`
	EffectiveDate  time.Time  `json:"effective_date"`
	InitiatedBy    *string    `json:"initiated_by,omitempty"`
	ChecklistRunID *string    `json:"checklist_run_id,omitempty"`
	Status         string     `json:"status"`
	DueDate        time.Time  `json:"due_date"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// MoverTemplate is a reusable checklist template for role changes.
type MoverTemplate struct {
	ID           string              `json:"id"`
	OrgID        string              `json:"org_id"`
	Name         string              `json:"name"`
	FromRoleHint string              `json:"from_role_hint,omitempty"`
	ToRoleHint   string              `json:"to_role_hint,omitempty"`
	IsDefault    bool                `json:"is_default"`
	Items        []MoverTemplateItem `json:"items,omitempty"`
	CreatedAt    time.Time           `json:"created_at"`
}

// MoverTemplateItem is one item in a mover template.
type MoverTemplateItem struct {
	ID              string `json:"id"`
	TemplateID      string `json:"template_id"`
	Section         string `json:"section"`
	Title           string `json:"title"`
	Description     string `json:"description,omitempty"`
	ResponsibleRole string `json:"responsible_role,omitempty"`
	SortOrder       int    `json:"sort_order"`
}

// CreateMoverEventInput is the request body for POST /hr/mover-events.
type CreateMoverEventInput struct {
	EmployeeID     string `json:"employee_id" validate:"required,uuid"`
	FromDepartment string `json:"from_department"`
	FromJobTitle   string `json:"from_job_title"`
	ToDepartment   string `json:"to_department" validate:"required"`
	ToJobTitle     string `json:"to_job_title" validate:"required"`
	EffectiveDate  string `json:"effective_date" validate:"required"`
	DueDaysOffset  int    `json:"due_days_offset"`
}

// CreateMoverEvent creates a new mover event (role-change record).
func (s *Service) CreateMoverEvent(ctx context.Context, orgID, initiatedBy string, in CreateMoverEventInput) (*MoverEvent, error) {
	effectiveDate, err := time.Parse("2006-01-02", in.EffectiveDate)
	if err != nil {
		return nil, fmt.Errorf("invalid effective_date: %w", err)
	}
	dueDaysOffset := in.DueDaysOffset
	if dueDaysOffset <= 0 {
		dueDaysOffset = 14
	}
	dueDate := effectiveDate.Add(time.Duration(dueDaysOffset) * 24 * time.Hour)

	return s.repo.CreateMoverEvent(ctx, orgID, initiatedBy, in, effectiveDate, dueDate)
}

// ListMoverEvents lists all mover events for an org, newest first.
func (s *Service) ListMoverEvents(ctx context.Context, orgID string) ([]MoverEvent, error) {
	return s.repo.ListMoverEvents(ctx, orgID)
}

// GetMoverEvent returns a single mover event.
func (s *Service) GetMoverEvent(ctx context.Context, orgID, id string) (*MoverEvent, error) {
	return s.repo.GetMoverEvent(ctx, orgID, id)
}

// UpdateMoverEventStatus updates the status of a mover event.
func (s *Service) UpdateMoverEventStatus(ctx context.Context, orgID, id, status string) (*MoverEvent, error) {
	return s.repo.UpdateMoverEventStatus(ctx, orgID, id, status)
}

// ListMoverTemplates returns all mover templates for an org.
func (s *Service) ListMoverTemplates(ctx context.Context, orgID string) ([]MoverTemplate, error) {
	return s.repo.ListMoverTemplates(ctx, orgID)
}

// EnsureDefaultMoverTemplate creates the system default mover template if none exist.
func (s *Service) EnsureDefaultMoverTemplate(ctx context.Context, orgID string) error {
	templates, err := s.repo.ListMoverTemplates(ctx, orgID)
	if err != nil || len(templates) > 0 {
		return err
	}

	tmplID, err := s.repo.CreateMoverTemplate(ctx, orgID, "Standard Rollenwechsel", "", "", true)
	if err != nil {
		return fmt.Errorf("create default mover template: %w", err)
	}

	defaultItems := []struct {
		section string
		title   string
		role    string
		order   int
	}{
		{"revoke", "Alle bisherigen Systemzugänge entziehen", "it", 1},
		{"revoke", "Bisherige Netzwerkfreigaben entfernen", "it", 2},
		{"revoke", "Bisherige Distribution-Listen-Mitgliedschaft beenden", "it", 3},
		{"grant", "Neue Systemzugänge provisionieren", "it", 1},
		{"grant", "Neue Netzwerkfreigaben einrichten", "it", 2},
		{"grant", "Neues Distribution-Listen-Mitglied hinzufügen", "it", 3},
		{"grant", "Schulungsnachweis für neue Rolle prüfen", "hr", 4},
		{"verify", "Zugangsprovisionierung bestätigen", "manager", 1},
		{"verify", "Segregation of Duties prüfen", "manager", 2},
		{"verify", "Dokumentation abschließen", "hr", 3},
	}

	for _, item := range defaultItems {
		if err := s.repo.CreateMoverTemplateItem(ctx, tmplID, item.section, item.title, "", item.role, item.order); err != nil {
			return fmt.Errorf("create default item %q: %w", item.title, err)
		}
	}
	return nil
}
