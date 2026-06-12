// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S74-3: Risikobewertung BSI 200-3

package vaktcomply

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ListBSIThreats returns all 47 elementare Gefährdungen (system-wide, static).
func (s *Service) ListBSIThreats(ctx context.Context) ([]BSIThreat, error) {
	// orgid-lint: global — ck_bsi_threats is a static system catalogue (47 BSI threats), not per-org
	rows, err := s.db.Query(ctx,
		`SELECT id, title, category, description FROM ck_bsi_threats ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list bsi threats: %w", err)
	}
	defer rows.Close()

	var out []BSIThreat
	for rows.Next() {
		var t BSIThreat
		if err := rows.Scan(&t.ID, &t.Title, &t.Category, &t.Description); err != nil {
			return nil, fmt.Errorf("scan bsi threat: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ListBSIRisks returns all risk assessments for a Zielobjekt.
func (s *Service) ListBSIRisks(ctx context.Context, orgID, targetObjectID string) ([]BSIRiskAssessment, error) {
	rows, err := s.db.Query(ctx, `
		SELECT r.id, r.org_id, r.target_object_id, r.threat_id,
		       COALESCE(t.title, ''),
		       r.eintrittshaeufigkeit, r.schadensauswirkung, r.risikokategorie,
		       r.behandlungsoption, r.massnahme, r.verantwortlicher,
		       r.zieldatum::text, r.restrisiko,
		       r.created_at, r.updated_at
		FROM ck_bsi_risk_assessments r
		LEFT JOIN ck_bsi_threats t ON t.id = r.threat_id
		WHERE r.org_id=$1 AND r.target_object_id=$2
		ORDER BY r.risikokategorie DESC, r.threat_id`, orgID, targetObjectID)
	if err != nil {
		return nil, fmt.Errorf("list bsi risks: %w", err)
	}
	defer rows.Close()

	var out []BSIRiskAssessment
	for rows.Next() {
		var r BSIRiskAssessment
		var dateStr *string
		if err := rows.Scan(
			&r.ID, &r.OrgID, &r.TargetObjectID, &r.ThreatID, &r.ThreatTitle,
			&r.Eintrittshaeufigkeit, &r.Schadensauswirkung, &r.Risikokategorie,
			&r.Behandlungsoption, &r.Massnahme, &r.Verantwortlicher,
			&dateStr, &r.Restrisiko,
			&r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan bsi risk: %w", err)
		}
		r.Zieldatum = dateStr
		out = append(out, r)
	}
	return out, rows.Err()
}

// CreateBSIRisk adds a Gefährdung to a Zielobjekt's risk analysis.
func (s *Service) CreateBSIRisk(ctx context.Context, orgID, targetObjectID string, in CreateBSIRiskInput) (*BSIRiskAssessment, error) {
	var r BSIRiskAssessment
	err := s.db.QueryRow(ctx, `
		INSERT INTO ck_bsi_risk_assessments
		  (org_id, target_object_id, threat_id, eintrittshaeufigkeit, schadensauswirkung)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id, org_id, target_object_id, threat_id,
		          eintrittshaeufigkeit, schadensauswirkung, risikokategorie,
		          behandlungsoption, massnahme, verantwortlicher,
		          zieldatum::text, restrisiko,
		          created_at, updated_at`,
		orgID, targetObjectID, in.ThreatID, in.Eintrittshaeufigkeit, in.Schadensauswirkung).
		Scan(&r.ID, &r.OrgID, &r.TargetObjectID, &r.ThreatID,
			&r.Eintrittshaeufigkeit, &r.Schadensauswirkung, &r.Risikokategorie,
			&r.Behandlungsoption, &r.Massnahme, &r.Verantwortlicher,
			&r.Zieldatum, &r.Restrisiko,
			&r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create bsi risk: %w", err)
	}
	return &r, nil
}

// UpdateBSIRisk updates classification and treatment for a risk entry.
func (s *Service) UpdateBSIRisk(ctx context.Context, orgID, targetObjectID, riskID string, in UpdateBSIRiskInput) (*BSIRiskAssessment, error) {
	var r BSIRiskAssessment
	err := s.db.QueryRow(ctx, `
		UPDATE ck_bsi_risk_assessments
		SET eintrittshaeufigkeit=$4, schadensauswirkung=$5,
		    behandlungsoption=$6, massnahme=$7, verantwortlicher=$8,
		    zieldatum=$9::date, restrisiko=$10, updated_at=NOW()
		WHERE org_id=$1 AND target_object_id=$2 AND id=$3
		RETURNING id, org_id, target_object_id, threat_id,
		          eintrittshaeufigkeit, schadensauswirkung, risikokategorie,
		          behandlungsoption, massnahme, verantwortlicher,
		          zieldatum::text, restrisiko,
		          created_at, updated_at`,
		orgID, targetObjectID, riskID,
		in.Eintrittshaeufigkeit, in.Schadensauswirkung,
		in.Behandlungsoption, in.Massnahme, in.Verantwortlicher,
		in.Zieldatum, in.Restrisiko).
		Scan(&r.ID, &r.OrgID, &r.TargetObjectID, &r.ThreatID,
			&r.Eintrittshaeufigkeit, &r.Schadensauswirkung, &r.Risikokategorie,
			&r.Behandlungsoption, &r.Massnahme, &r.Verantwortlicher,
			&r.Zieldatum, &r.Restrisiko,
			&r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update bsi risk: %w", err)
	}
	return &r, nil
}

// DeleteBSIRisk removes a risk entry.
func (s *Service) DeleteBSIRisk(ctx context.Context, orgID, targetObjectID, riskID string) error {
	tag, err := s.db.Exec(ctx,
		`DELETE FROM ck_bsi_risk_assessments WHERE org_id=$1 AND target_object_id=$2 AND id=$3`,
		orgID, targetObjectID, riskID)
	if err != nil {
		return fmt.Errorf("delete bsi risk: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetBSIRiskSummary returns aggregated risk counts by category for a Zielobjekt.
func (s *Service) GetBSIRiskSummary(ctx context.Context, orgID, targetObjectID string) (BSIRiskSummary, error) {
	var gering, mittel, hoch, sehrHoch, offen int
	err := s.db.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE risikokategorie='gering'),
			COUNT(*) FILTER (WHERE risikokategorie='mittel'),
			COUNT(*) FILTER (WHERE risikokategorie='hoch'),
			COUNT(*) FILTER (WHERE risikokategorie='sehr_hoch'),
			COUNT(*) FILTER (WHERE behandlungsoption IS NULL)
		FROM ck_bsi_risk_assessments
		WHERE org_id=$1 AND target_object_id=$2`, orgID, targetObjectID).
		Scan(&gering, &mittel, &hoch, &sehrHoch, &offen)
	if err != nil {
		return BSIRiskSummary{}, fmt.Errorf("get bsi risk summary: %w", err)
	}
	return BSIRiskSummary{
		Gering:   gering,
		Mittel:   mittel,
		Hoch:     hoch,
		SehrHoch: sehrHoch,
		Offen:    offen,
	}, nil
}

// ComputeRisikokategorie computes the 4×4 risk matrix result in Go
// (mirrors the GENERATED ALWAYS AS expression in the DB for pure-function tests).
func ComputeRisikokategorie(eintrittshaeufigkeit, schadensauswirkung string) string {
	switch eintrittshaeufigkeit {
	case "sehr_haeufig":
		return "sehr_hoch"
	case "haeufig":
		switch schadensauswirkung {
		case "vernachlaessigbar":
			return "mittel"
		case "begrenzt":
			return "hoch"
		default:
			return "sehr_hoch"
		}
	case "mittel":
		switch schadensauswirkung {
		case "vernachlaessigbar":
			return "gering"
		case "begrenzt":
			return "mittel"
		case "betraechtlich":
			return "hoch"
		default:
			return "sehr_hoch"
		}
	case "selten":
		switch schadensauswirkung {
		case "vernachlaessigbar", "begrenzt":
			return "gering"
		case "betraechtlich":
			return "mittel"
		default:
			return "hoch"
		}
	}
	return "mittel"
}
