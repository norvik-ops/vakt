// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S88-4: verinice-(.vna)-Import als Migrationsbrücke. Preview (dry-run) +
// Commit. Mapping SNCA → Vakt (ADR-0062): asset → ck_bsi_target_objects,
// control/safeguard → Control in einem "verinice-Import"-Framework,
// incident_scenario → ck_risks. Untrusted Input wird im veriniceimport-Package
// defensiv geparst (Größenlimits, kein XXE, Fuzz-getestet).

package vaktcomply

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/modules/vaktcomply/bsi"
	"github.com/matharnica/vakt/internal/shared/veriniceimport"
)

// ImportResult reports what a commit actually created.
type ImportResult struct {
	AssetsCreated   int    `json:"assets_created"`
	ControlsCreated int    `json:"controls_created"`
	RisksCreated    int    `json:"risks_created"`
	Skipped         int    `json:"skipped"`
	FrameworkID     string `json:"framework_id,omitempty"`
}

// PreviewVeriniceImport parses the .vna and returns a dry-run preview (no writes).
func (s *Service) PreviewVeriniceImport(data []byte) (veriniceimport.Preview, error) {
	objs, err := veriniceimport.ParseVNA(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return veriniceimport.Preview{}, err
	}
	return veriniceimport.BuildPreview(objs), nil
}

// CommitVeriniceImport parses the .vna and creates the mapped Vakt entities.
func (s *Service) CommitVeriniceImport(ctx context.Context, orgID, userID string, data []byte) (ImportResult, error) {
	objs, err := veriniceimport.ParseVNA(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return ImportResult{}, err
	}
	var res ImportResult

	for _, o := range objs {
		switch o.Category {
		case "asset":
			itype := mapAssetType(o.Type)
			if _, err := s.BSI.CreateBSITargetObject(ctx, orgID, bsi.CreateBSITargetObjectInput{
				Name:               truncateStr(o.Title, 200),
				Type:               itype,
				Description:        "Importiert aus verinice (.vna)",
				Absicherungsniveau: "standard",
			}); err != nil {
				log.Warn().Err(err).Str("ext_id", o.ExtID).Msg("verinice import: asset")
				res.Skipped++
				continue
			}
			res.AssetsCreated++

		case "risk":
			if _, err := s.Risk.CreateRisk(ctx, orgID, CreateRiskInput{
				Title:          truncateStr(o.Title, 255),
				Description:    "Importiert aus verinice (.vna). Bitte Eintrittswahrscheinlichkeit und Auswirkung prüfen.",
				Category:       "verinice-Import",
				Likelihood:     3,
				Impact:         3,
				Treatment:      "mitigate",
				TreatmentNotes: "",
			}); err != nil {
				log.Warn().Err(err).Str("ext_id", o.ExtID).Msg("verinice import: risk")
				res.Skipped++
				continue
			}
			res.RisksCreated++

		case "control":
			if res.FrameworkID == "" {
				fwID, err := s.ensureVeriniceImportFramework(ctx, orgID)
				if err != nil {
					return res, fmt.Errorf("verinice import: framework: %w", err)
				}
				res.FrameworkID = fwID
			}
			if err := s.insertImportedControl(ctx, orgID, res.FrameworkID, o); err != nil {
				log.Warn().Err(err).Str("ext_id", o.ExtID).Msg("verinice import: control")
				res.Skipped++
				continue
			}
			res.ControlsCreated++

		default:
			res.Skipped++
		}
	}
	return res, nil
}

// ensureVeriniceImportFramework returns the id of the org's "verinice-Import"
// framework, creating it on first use.
func (s *Service) ensureVeriniceImportFramework(ctx context.Context, orgID string) (string, error) {
	var id string
	err := s.db.QueryRow(ctx,
		`SELECT id::text FROM ck_frameworks WHERE org_id = $1 AND name = 'verinice-Import'`,
		orgID).Scan(&id)
	if err == nil && id != "" {
		return id, nil
	}
	if insErr := s.db.QueryRow(ctx, `
		INSERT INTO ck_frameworks (org_id, name)
		VALUES ($1, 'verinice-Import')
		RETURNING id::text`, orgID).Scan(&id); insErr != nil {
		return "", insErr
	}
	return id, nil
}

func (s *Service) insertImportedControl(ctx context.Context, orgID, frameworkID string, o veriniceimport.ImportObject) error {
	controlID := o.ExtID
	if controlID == "" {
		controlID = "VN-" + truncateStr(o.Title, 40)
	}
	_, err := s.db.Exec(ctx, `
		INSERT INTO ck_controls (framework_id, org_id, control_id, title, description, domain)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, 'verinice-Import')`,
		frameworkID, orgID, controlID, truncateStr(o.Title, 255), "Importiert aus verinice (.vna)")
	return err
}

// mapAssetType maps an SNCA asset-ish type to a Vakt target-object type.
func mapAssetType(snca string) string {
	switch {
	case strings.Contains(snca, "network"):
		return "network"
	case strings.Contains(snca, "application"), strings.Contains(snca, "app"):
		return "application"
	case strings.Contains(snca, "room"), strings.Contains(snca, "raum"):
		return "room"
	case strings.Contains(snca, "process"), strings.Contains(snca, "prozess"):
		return "process"
	default:
		return "it_system"
	}
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
