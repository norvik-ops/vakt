// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S82-4: CatalogProvider — abstracts catalog loading behind an interface so that
// GS++ (Sprint 83/84) can provide an official BSI-published catalog without
// touching EnableFramework or the GS-Check workflow (ADR-0056).

package policy

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/rs/zerolog/log"
)

//go:embed catalogs/bsi-kompendium-2023.json
var bsiKompendiumData []byte

// ── BSI catalog types ─────────────────────────────────────────────────────────

type bsiCatalogAnforderung struct {
	ID               string `json:"id"`
	Title            string `json:"title"`
	Stufe            string `json:"stufe"`
	Kurzbeschreibung string `json:"kurzbeschreibung"`
}

type bsiCatalogBaustein struct {
	ID               string                  `json:"id"`
	Title            string                  `json:"title"`
	Kurzbeschreibung string                  `json:"kurzbeschreibung"`
	Anforderungen    []bsiCatalogAnforderung `json:"anforderungen"`
}

type bsiCatalogSchicht struct {
	ID        string               `json:"id"`
	Title     string               `json:"title"`
	Bausteine []bsiCatalogBaustein `json:"bausteine"`
}

type bsiCatalogRoot struct {
	Edition   string              `json:"edition"`
	Schichten []bsiCatalogSchicht `json:"schichten"`
}

var (
	bsiCatalogOnce  sync.Once
	bsiCatalogCache *bsiCatalogRoot
	bsiCatalogErr   error
)

func loadBSICatalog() (*bsiCatalogRoot, error) {
	bsiCatalogOnce.Do(func() {
		var cat bsiCatalogRoot
		if err := json.Unmarshal(bsiKompendiumData, &cat); err != nil {
			// The catalog is compiled into the binary via go:embed.
			// A parse failure means the embedded JSON is corrupt — fail fast
			// so the operator gets an immediate error at startup instead of
			// silently serving empty BSI control lists (S78-9).
			panic(fmt.Sprintf("BSI catalog: corrupt embedded JSON — startup aborted: %v", err))
		}
		log.Info().Str("edition", cat.Edition).Int("schichten", len(cat.Schichten)).Msg("BSI catalog loaded")
		bsiCatalogCache = &cat
	})
	return bsiCatalogCache, bsiCatalogErr
}

// bsiRequirementLevels returns a map[controlID]stufe for all BSI catalog entries.
// Used by EnableFramework to batch-update requirement_level after seeding.
func bsiRequirementLevels() map[string]string {
	cat, err := loadBSICatalog()
	if err != nil {
		return nil
	}
	levels := make(map[string]string)
	for _, schicht := range cat.Schichten {
		for _, baustein := range schicht.Bausteine {
			for _, anf := range baustein.Anforderungen {
				if anf.Stufe != "" {
					levels[anf.ID] = anf.Stufe
				}
			}
		}
	}
	return levels
}

// BsiControls builds the full set of Controls from the embedded catalog JSON.
// Replaces the old hard-coded literal slice in service_helpers.go (34 → 111 Bausteine).
func BsiControls(frameworkID, orgID string) []Control {
	cat, err := loadBSICatalog()
	if err != nil {
		return nil
	}
	var controls []Control
	for _, schicht := range cat.Schichten {
		domain := bsiSchichtDomain(schicht.ID)
		evType := bsiSchichtEvidenceType(schicht.ID)
		for _, baustein := range schicht.Bausteine {
			for _, anf := range baustein.Anforderungen {
				controls = append(controls, Control{
					FrameworkID:  frameworkID,
					OrgID:        orgID,
					ControlID:    anf.ID,
					Title:        anf.Title,
					Description:  anf.Kurzbeschreibung,
					Domain:       domain,
					EvidenceType: evType,
					Weight:       bsiStufeWeight(anf.Stufe),
				})
			}
		}
	}
	return controls
}

func bsiSchichtDomain(schichtID string) string {
	m := map[string]string{
		"ISMS": "Sicherheitsmanagement",
		"ORP":  "Organisation und Personal",
		"CON":  "Konzeption",
		"OPS":  "Betrieb",
		"DER":  "Detektion und Reaktion",
		"APP":  "Anwendungen",
		"SYS":  "IT-Systeme",
		"IND":  "Industrielle IT",
		"NET":  "Netze",
		"INF":  "Infrastruktur",
	}
	if d, ok := m[schichtID]; ok {
		return d
	}
	return schichtID
}

func bsiSchichtEvidenceType(schichtID string) string {
	switch schichtID {
	case "OPS", "APP", "SYS", "NET":
		return "automated"
	default:
		return "manual"
	}
}

// bsiStufeWeight maps stufe to Control.Weight: basis is most critical.
func bsiStufeWeight(stufe string) int {
	switch stufe {
	case "basis":
		return 3
	case "standard":
		return 2
	case "erhoeht":
		return 1
	default:
		return 2
	}
}

// ── CatalogProvider ───────────────────────────────────────────────────────────
// S82-4: multi-source catalog abstraction (ADR-0056).
// KompendiumProvider wraps the embedded BSI Kompendium 2023 JSON.
// gsppProvider is a placeholder that returns a clear error until GS++ is
// officially published (it-sa 27.10.2026 → Sprint 83).

// CatalogMetadata describes the origin and version of a control catalog.
type CatalogMetadata struct {
	Source        string
	Edition       string
	SchemaVersion string
}

// CatalogProvider abstracts catalog loading so that different catalog sources
// (Kompendium, GS++, future standards) can be swapped without changing callers.
type CatalogProvider interface {
	Controls(frameworkID, orgID string) ([]Control, error)
	Metadata() CatalogMetadata
}

// KompendiumProvider serves the embedded BSI IT-Grundschutz Kompendium 2023.
type KompendiumProvider struct{}

func (KompendiumProvider) Controls(frameworkID, orgID string) ([]Control, error) {
	return BsiControls(frameworkID, orgID), nil
}

func (KompendiumProvider) Metadata() CatalogMetadata {
	cat, _ := loadBSICatalog()
	edition := ""
	if cat != nil {
		edition = cat.Edition
	}
	return CatalogMetadata{
		Source:  "BSI IT-Grundschutz Kompendium",
		Edition: edition,
	}
}

// gsppProvider is a placeholder that blocks GS++ usage until the official
// Anwenderkatalog-GS++ is published at it-sa (27.10.2026).
type gsppProvider struct{}

func (gsppProvider) Controls(_, _ string) ([]Control, error) {
	return nil, fmt.Errorf("GS++ nicht verfügbar vor Veröffentlichung (it-sa 27.10.2026)")
}

func (gsppProvider) Metadata() CatalogMetadata {
	return CatalogMetadata{
		Source:  "BSI Grundschutz++",
		Edition: "draft",
	}
}

// catalogRegistry maps framework names to their CatalogProvider.
// All callers that need catalog controls should look up this registry so that
// adding a new catalog source (e.g. GS++) only requires registering a provider here.
var catalogRegistry = map[string]CatalogProvider{
	"BSI":  KompendiumProvider{},
	"GSPP": gsppProvider{},
}
