// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S88-5: Physische-Maßnahmen-Templates (ISO 27001:2022 A.7.x) als geführte
// Checklisten. Keine Migration — Katalog-als-Daten (go:embed) + bestehender
// Evidence-Mechanismus. "Checkliste anwenden" erzeugt strukturierte Evidence
// am passenden A.7-Control.

package vaktcomply

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/rs/zerolog/log"
)

//go:embed catalogs/physical-controls-templates.json
var physicalTemplatesData []byte

// PhysicalControlTemplate is a guided checklist for one ISO 27001:2022 A.7 control.
type PhysicalControlTemplate struct {
	ControlCode string   `json:"control_code"`
	Title       string   `json:"title"`
	Items       []string `json:"items"`
}

type physicalTemplateRoot struct {
	Edition   string                    `json:"edition"`
	Templates []PhysicalControlTemplate `json:"templates"`
}

var (
	physTemplatesOnce  sync.Once
	physTemplatesCache *physicalTemplateRoot
)

func loadPhysicalTemplates() *physicalTemplateRoot {
	physTemplatesOnce.Do(func() {
		var root physicalTemplateRoot
		if err := json.Unmarshal(physicalTemplatesData, &root); err != nil {
			// Compiled in via go:embed — corrupt JSON means a broken build.
			panic(fmt.Sprintf("physical-controls templates: corrupt embedded JSON: %v", err))
		}
		log.Info().Int("templates", len(root.Templates)).Msg("physical-controls templates loaded")
		physTemplatesCache = &root
	})
	return physTemplatesCache
}

// ListPhysicalControlTemplates returns all A.7.x checklist templates.
func (s *Service) ListPhysicalControlTemplates() []PhysicalControlTemplate {
	return loadPhysicalTemplates().Templates
}

// findPhysicalTemplate returns the template for a control code, or false.
func findPhysicalTemplate(code string) (PhysicalControlTemplate, bool) {
	for _, t := range loadPhysicalTemplates().Templates {
		if t.ControlCode == code {
			return t, true
		}
	}
	return PhysicalControlTemplate{}, false
}

// ApplyPhysicalControlTemplate attaches the checklist for controlCode as
// structured evidence to the matching control in the org. Returns an error if
// the control is not present (framework not enabled) or the template is unknown.
func (s *Service) ApplyPhysicalControlTemplate(ctx context.Context, orgID, controlCode, userID string) (*Evidence, error) {
	tmpl, ok := findPhysicalTemplate(controlCode)
	if !ok {
		return nil, fmt.Errorf("unknown physical control template: %s", controlCode)
	}
	controlID, err := s.repo.FindControlByCode(ctx, orgID, controlCode)
	if err != nil || controlID == "" {
		return nil, fmt.Errorf("control %s not found — is ISO 27001 enabled?", controlCode)
	}
	payload, err := json.Marshal(map[string]any{
		"source":       "physical_control_template",
		"control_code": controlCode,
		"checklist":    tmpl.Items,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal checklist payload: %w", err)
	}
	title := fmt.Sprintf("Checkliste: %s (%s)", tmpl.Title, controlCode)
	ev, err := s.repo.AddCollectorEvidence(ctx, orgID, controlID, userID, "checklist", title, payload)
	if err != nil {
		return nil, fmt.Errorf("apply physical template: %w", err)
	}
	return ev, nil
}
