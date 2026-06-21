// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S88-3: Generischer Gefährdungs-/Maßnahmen-Katalog (Risk-Catalog).
// Katalog-als-Daten via go:embed (ADR-0061). Der ISB wählt beim Anlegen eines
// Risikos ein Katalog-Item und erhält ein vorbefülltes Risiko inkl.
// Maßnahmenvorschlag + Control-Verknüpfung — Time-to-Value von Tagen auf Stunden.

package vaktcomply

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"
)

//go:embed catalogs/threat-library.json
var threatLibraryData []byte

// ThreatCatalogItem is one generic threat/scenario in the embedded library.
type ThreatCatalogItem struct {
	ID                string   `json:"id"`
	Title             string   `json:"title"`
	Category          string   `json:"category"`
	AssetTypes        []string `json:"asset_types"`
	CIA               []string `json:"cia"`
	Frameworks        []string `json:"frameworks"`
	Scenario          string   `json:"scenario"`
	SuggestedMeasure  string   `json:"suggested_measure"`
	ControlLinks      []string `json:"control_links"`
	DefaultLikelihood int      `json:"default_likelihood"`
	DefaultImpact     int      `json:"default_impact"`
}

type threatLibraryRoot struct {
	Version string              `json:"version"`
	Edition string              `json:"edition"`
	Threats []ThreatCatalogItem `json:"threats"`
}

var (
	threatLibOnce  sync.Once
	threatLibCache *threatLibraryRoot
)

func loadThreatLibrary() *threatLibraryRoot {
	threatLibOnce.Do(func() {
		var root threatLibraryRoot
		if err := json.Unmarshal(threatLibraryData, &root); err != nil {
			// Compiled in via go:embed — corrupt JSON means a broken build (fail fast).
			panic(fmt.Sprintf("threat library: corrupt embedded JSON: %v", err))
		}
		log.Info().Str("version", root.Version).Int("threats", len(root.Threats)).Msg("threat library loaded")
		threatLibCache = &root
	})
	return threatLibCache
}

// ThreatCatalogVersion returns the embedded library version (for link provenance).
func ThreatCatalogVersion() string {
	return loadThreatLibrary().Version
}

// ThreatCatalogFilter narrows the catalog by framework, asset type and/or CIA goal.
type ThreatCatalogFilter struct {
	Framework string // e.g. "ISO27001", "BSI", "NIS2", "DSGVO-TOM", "C5"
	AssetType string // e.g. "server", "data", "identity"
	CIA       string // "confidentiality" | "integrity" | "availability"
}

func sliceContainsFold(haystack []string, needle string) bool {
	for _, h := range haystack {
		if strings.EqualFold(h, needle) {
			return true
		}
	}
	return false
}

// ListThreatCatalog returns catalog items matching the filter (empty filter = all).
func (s *Service) ListThreatCatalog(f ThreatCatalogFilter) []ThreatCatalogItem {
	all := loadThreatLibrary().Threats
	if f.Framework == "" && f.AssetType == "" && f.CIA == "" {
		return all
	}
	out := make([]ThreatCatalogItem, 0, len(all))
	for _, it := range all {
		if f.Framework != "" && !sliceContainsFold(it.Frameworks, f.Framework) {
			continue
		}
		if f.AssetType != "" && !sliceContainsFold(it.AssetTypes, f.AssetType) {
			continue
		}
		if f.CIA != "" && !sliceContainsFold(it.CIA, f.CIA) {
			continue
		}
		out = append(out, it)
	}
	return out
}

func findThreatCatalogItem(id string) (ThreatCatalogItem, bool) {
	for _, it := range loadThreatLibrary().Threats {
		if it.ID == id {
			return it, true
		}
	}
	return ThreatCatalogItem{}, false
}

// CreateRiskFromCatalogInput allows the caller to override catalog defaults.
type CreateRiskFromCatalogInput struct {
	CatalogID  string `json:"catalog_id" validate:"required"`
	Likelihood int    `json:"likelihood" validate:"omitempty,min=1,max=5"`
	Impact     int    `json:"impact" validate:"omitempty,min=1,max=5"`
	Owner      string `json:"owner"`
}

// CreateRiskFromCatalog creates a pre-filled risk from a catalog item and records
// the provenance in ck_threat_library_links.
func (s *Service) CreateRiskFromCatalog(ctx context.Context, orgID string, in CreateRiskFromCatalogInput, userID string) (*Risk, error) {
	item, ok := findThreatCatalogItem(in.CatalogID)
	if !ok {
		return nil, fmt.Errorf("unknown catalog item: %s", in.CatalogID)
	}
	likelihood := in.Likelihood
	if likelihood == 0 {
		likelihood = item.DefaultLikelihood
	}
	impact := in.Impact
	if impact == 0 {
		impact = item.DefaultImpact
	}
	desc := item.Scenario
	if len(item.ControlLinks) > 0 {
		desc += "\n\nVorgeschlagene Maßnahme: " + item.SuggestedMeasure +
			"\nControl-Verknüpfung: " + strings.Join(item.ControlLinks, ", ")
	} else {
		desc += "\n\nVorgeschlagene Maßnahme: " + item.SuggestedMeasure
	}
	risk, err := s.Risk.CreateRisk(ctx, orgID, CreateRiskInput{
		Title:          item.Title,
		Description:    desc,
		Category:       item.Category,
		Likelihood:     likelihood,
		Impact:         impact,
		Owner:          in.Owner,
		Treatment:      "mitigate",
		TreatmentNotes: item.SuggestedMeasure,
	})
	if err != nil {
		return nil, err
	}
	// Record provenance (best-effort — the risk already exists).
	if _, linkErr := s.db.Exec(ctx, `
		INSERT INTO ck_threat_library_links (org_id, risk_id, catalog_id, catalog_version)
		VALUES ($1, $2, $3, $4)`,
		orgID, risk.ID, item.ID, loadThreatLibrary().Version); linkErr != nil {
		log.Warn().Err(linkErr).Str("org_id", orgID).Str("catalog_id", item.ID).Msg("threat library link insert")
	}
	return risk, nil
}
