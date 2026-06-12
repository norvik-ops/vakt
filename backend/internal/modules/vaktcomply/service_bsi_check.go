// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S74-1: IT-Grundschutz-Check-Workflow + S74-2: Grundschutz-Cockpit

package vaktcomply

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"
)

// ── Target Objects (Strukturanalyse) ──────────────────────────────────────────

const bsiTargetObjectCols = `id, org_id, name, type, description,
       protection_c, protection_i, protection_a,
       absicherungsniveau,
       override_c, override_i, override_a, override_reason, override_effect,
       created_at, updated_at`

func scanBSITargetObject(row interface {
	Scan(dest ...any) error
}) (BSITargetObject, error) {
	var o BSITargetObject
	err := row.Scan(
		&o.ID, &o.OrgID, &o.Name, &o.Type, &o.Description,
		&o.ProtectionC, &o.ProtectionI, &o.ProtectionA,
		&o.Absicherungsniveau,
		&o.OverrideC, &o.OverrideI, &o.OverrideA, &o.OverrideReason, &o.OverrideEffect,
		&o.CreatedAt, &o.UpdatedAt,
	)
	return o, err
}

// ListBSITargetObjects returns all Zielobjekte for an org, enriched with effective CIA values.
func (s *Service) ListBSITargetObjects(ctx context.Context, orgID string) ([]BSITargetObject, error) {
	rows, err := s.db.Query(ctx, `
		SELECT `+bsiTargetObjectCols+`
		FROM ck_bsi_target_objects
		WHERE org_id = $1
		ORDER BY name`, orgID)
	if err != nil {
		return nil, fmt.Errorf("list bsi target objects: %w", err)
	}
	defer rows.Close()

	var out []BSITargetObject
	for rows.Next() {
		o, err := scanBSITargetObject(rows)
		if err != nil {
			return nil, fmt.Errorf("scan bsi target object: %w", err)
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return s.enrichWithEffective(ctx, orgID, out)
}

// GetBSITargetObject returns a single Zielobjekt by ID, scoped to org.
func (s *Service) GetBSITargetObject(ctx context.Context, orgID, id string) (*BSITargetObject, error) {
	o, err := scanBSITargetObject(s.db.QueryRow(ctx, `
		SELECT `+bsiTargetObjectCols+`
		FROM ck_bsi_target_objects
		WHERE org_id = $1 AND id = $2`, orgID, id))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get bsi target object: %w", err)
	}
	objs, err := s.enrichWithEffective(ctx, orgID, []BSITargetObject{o})
	if err != nil {
		return nil, err
	}
	return &objs[0], nil
}

// CreateBSITargetObject creates a new Zielobjekt.
func (s *Service) CreateBSITargetObject(ctx context.Context, orgID string, in CreateBSITargetObjectInput) (*BSITargetObject, error) {
	niveau := in.Absicherungsniveau
	if niveau == "" {
		niveau = "standard"
	}
	o, err := scanBSITargetObject(s.db.QueryRow(ctx, `
		INSERT INTO ck_bsi_target_objects
		  (org_id, name, type, description, protection_c, protection_i, protection_a, absicherungsniveau)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING `+bsiTargetObjectCols,
		orgID, in.Name, in.Type, in.Description,
		in.ProtectionC, in.ProtectionI, in.ProtectionA, niveau))
	if err != nil {
		return nil, fmt.Errorf("create bsi target object: %w", err)
	}
	return &o, nil
}

// UpdateBSITargetObject updates an existing Zielobjekt.
func (s *Service) UpdateBSITargetObject(ctx context.Context, orgID, id string, in UpdateBSITargetObjectInput) (*BSITargetObject, error) {
	niveau := in.Absicherungsniveau
	if niveau == "" {
		niveau = "standard"
	}
	o, err := scanBSITargetObject(s.db.QueryRow(ctx, `
		UPDATE ck_bsi_target_objects
		SET name=$3, type=$4, description=$5,
		    protection_c=$6, protection_i=$7, protection_a=$8,
		    absicherungsniveau=$9, updated_at=NOW()
		WHERE org_id=$1 AND id=$2
		RETURNING `+bsiTargetObjectCols,
		orgID, id, in.Name, in.Type, in.Description,
		in.ProtectionC, in.ProtectionI, in.ProtectionA, niveau))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update bsi target object: %w", err)
	}
	objs, err := s.enrichWithEffective(ctx, orgID, []BSITargetObject{o})
	if err != nil {
		return nil, err
	}
	return &objs[0], nil
}

// DeleteBSITargetObject deletes a Zielobjekt and all associated check results.
func (s *Service) DeleteBSITargetObject(ctx context.Context, orgID, id string) error {
	tag, err := s.db.Exec(ctx,
		`DELETE FROM ck_bsi_target_objects WHERE org_id=$1 AND id=$2`, orgID, id)
	if err != nil {
		return fmt.Errorf("delete bsi target object: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ── Baustein Assignment ───────────────────────────────────────────────────────

// AssignBausteinToTargetObject links a Baustein to a Zielobjekt and pre-fills
// check results with status "nein" for all relevant Anforderungen.
func (s *Service) AssignBausteinToTargetObject(ctx context.Context, orgID, targetObjectID, bausteinID string) error {
	// Verify target object belongs to org.
	_, err := s.GetBSITargetObject(ctx, orgID, targetObjectID)
	if err != nil {
		return err
	}

	// Get all controls for this Baustein domain (BSI-ORP.1 → domain ORP.1).
	domain := strings.TrimPrefix(bausteinID, "BSI-")
	rows, err := s.db.Query(ctx, `
		SELECT c.control_id
		FROM ck_controls c
		JOIN ck_frameworks f ON c.framework_id = f.id
		WHERE f.org_id = $1 AND f.name = 'BSI'
		  AND c.domain = $2`, orgID, domain)
	if err != nil {
		return fmt.Errorf("list baustein controls: %w", err)
	}
	defer rows.Close()

	var anforderungIDs []string
	for rows.Next() {
		var cid string
		if err := rows.Scan(&cid); err != nil {
			return fmt.Errorf("scan control id: %w", err)
		}
		anforderungIDs = append(anforderungIDs, cid)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	// Bulk-insert check results (ignore duplicates).
	for _, aid := range anforderungIDs {
		_, err := s.db.Exec(ctx, `
			INSERT INTO ck_bsi_check_results
			  (org_id, target_object_id, baustein_id, anforderung_id, umsetzungsstatus)
			VALUES ($1,$2,$3,$4,'nein')
			ON CONFLICT (org_id, target_object_id, anforderung_id) DO NOTHING`,
			orgID, targetObjectID, bausteinID, aid)
		if err != nil {
			log.Warn().Err(err).Str("anforderung", aid).Msg("assign baustein: insert check result")
		}
	}
	return nil
}

// RemoveBausteinFromTargetObject removes the check results for a Baustein from a Zielobjekt.
func (s *Service) RemoveBausteinFromTargetObject(ctx context.Context, orgID, targetObjectID, bausteinID string) error {
	_, err := s.db.Exec(ctx,
		`DELETE FROM ck_bsi_check_results
		 WHERE org_id=$1 AND target_object_id=$2 AND baustein_id=$3`,
		orgID, targetObjectID, bausteinID)
	return err
}

// ── IT-Grundschutz-Check ─────────────────────────────────────────────────────

// checkSheetSQL and gapReportSQL are package-level constants so that unit tests
// can assert on org_id scoping of the ck_controls JOIN (S78-2 regression guard).
const checkSheetSQL = `
		SELECT cr.id, cr.org_id, cr.target_object_id, cr.baustein_id, cr.anforderung_id,
		       COALESCE(c.title, ''), COALESCE(c.requirement_level, 'basis'),
		       cr.umsetzungsstatus,
		       cr.begruendung, cr.verantwortlicher,
		       cr.umsetzungsdatum::text, cr.notiz,
		       cr.created_at, cr.updated_at
		FROM ck_bsi_check_results cr
		LEFT JOIN ck_controls c ON c.control_id = cr.anforderung_id AND c.org_id = cr.org_id
		WHERE cr.org_id=$1 AND cr.target_object_id=$2
		ORDER BY cr.baustein_id, cr.anforderung_id`

const gapReportSQL = `
		SELECT cr.baustein_id, cr.anforderung_id,
		       COALESCE(c.title, ''),
		       t.name,
		       cr.umsetzungsstatus,
		       cr.verantwortlicher,
		       COALESCE(cr.umsetzungsdatum::text, '')
		FROM ck_bsi_check_results cr
		JOIN ck_bsi_target_objects t ON t.id = cr.target_object_id
		LEFT JOIN ck_controls c ON c.control_id = cr.anforderung_id AND c.org_id = cr.org_id
		WHERE cr.org_id=$1 AND cr.umsetzungsstatus IN ('nein','teilweise')
		ORDER BY cr.baustein_id, cr.anforderung_id, t.name`

// GetCheckSheet returns all check results for a Zielobjekt, enriched with control titles and requirement_level.
func (s *Service) GetCheckSheet(ctx context.Context, orgID, targetObjectID string) ([]BSICheckResult, error) {
	rows, err := s.db.Query(ctx, checkSheetSQL, orgID, targetObjectID)
	if err != nil {
		return nil, fmt.Errorf("get check sheet: %w", err)
	}
	defer rows.Close()

	var out []BSICheckResult
	for rows.Next() {
		var r BSICheckResult
		var dateStr *string
		if err := rows.Scan(
			&r.ID, &r.OrgID, &r.TargetObjectID, &r.BausteinID, &r.AnforderungID,
			&r.AnforderungTitle, &r.RequirementLevel, &r.Umsetzungsstatus,
			&r.Begruendung, &r.Verantwortlicher,
			&dateStr, &r.Notiz,
			&r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan check result: %w", err)
		}
		r.Umsetzungsdatum = dateStr
		out = append(out, r)
	}
	return out, rows.Err()
}

// SetCheckResult sets or updates one Anforderung status for a Zielobjekt.
func (s *Service) SetCheckResult(ctx context.Context, orgID, targetObjectID, anforderungID string, in SetCheckResultInput) (*BSICheckResult, error) {
	if in.Umsetzungsstatus == "entbehrlich" && strings.TrimSpace(in.Begruendung) == "" {
		return nil, fmt.Errorf("begruendung_required: Begründung ist bei Status 'entbehrlich' Pflicht")
	}

	// Get baustein_id from existing row, or derive from anforderung_id prefix (e.g. BSI-ORP.1.A1 → BSI-ORP.1).
	var bausteinID string
	_ = s.db.QueryRow(ctx,
		`SELECT baustein_id FROM ck_bsi_check_results WHERE org_id=$1 AND target_object_id=$2 AND anforderung_id=$3`,
		orgID, targetObjectID, anforderungID).Scan(&bausteinID)
	if bausteinID == "" {
		// Derive from anforderung_id: remove trailing .A<n>
		parts := strings.Split(anforderungID, ".A")
		bausteinID = parts[0]
	}

	var r BSICheckResult
	var dateStr *string
	err := s.db.QueryRow(ctx, `
		INSERT INTO ck_bsi_check_results
		  (org_id, target_object_id, baustein_id, anforderung_id,
		   umsetzungsstatus, begruendung, verantwortlicher, umsetzungsdatum, notiz)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8::date,$9)
		ON CONFLICT (org_id, target_object_id, anforderung_id)
		DO UPDATE SET umsetzungsstatus=EXCLUDED.umsetzungsstatus,
		              begruendung=EXCLUDED.begruendung,
		              verantwortlicher=EXCLUDED.verantwortlicher,
		              umsetzungsdatum=EXCLUDED.umsetzungsdatum,
		              notiz=EXCLUDED.notiz,
		              updated_at=NOW()
		RETURNING id, org_id, target_object_id, baustein_id, anforderung_id,
		          umsetzungsstatus, begruendung, verantwortlicher,
		          umsetzungsdatum::text, notiz, created_at, updated_at`,
		orgID, targetObjectID, bausteinID, anforderungID,
		in.Umsetzungsstatus, in.Begruendung, in.Verantwortlicher, in.Umsetzungsdatum, in.Notiz).
		Scan(&r.ID, &r.OrgID, &r.TargetObjectID, &r.BausteinID, &r.AnforderungID,
			&r.Umsetzungsstatus, &r.Begruendung, &r.Verantwortlicher,
			&dateStr, &r.Notiz, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("set check result: %w", err)
	}
	r.Umsetzungsdatum = dateStr
	return &r, nil
}

// BulkSetCheckResults sets multiple check results in a single transaction.
func (s *Service) BulkSetCheckResults(ctx context.Context, orgID, targetObjectID string, items []BulkCheckResultItem) error {
	// Validate entbehrlich items.
	for _, item := range items {
		if item.Umsetzungsstatus == "entbehrlich" && strings.TrimSpace(item.Begruendung) == "" {
			return fmt.Errorf("begruendung_required: Begründung ist bei Status 'entbehrlich' Pflicht für %s", item.AnforderungID)
		}
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	for _, item := range items {
		parts := strings.Split(item.AnforderungID, ".A")
		bausteinID := parts[0]

		_, err := tx.Exec(ctx, `
			INSERT INTO ck_bsi_check_results
			  (org_id, target_object_id, baustein_id, anforderung_id,
			   umsetzungsstatus, begruendung, verantwortlicher, umsetzungsdatum, notiz)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8::date,$9)
			ON CONFLICT (org_id, target_object_id, anforderung_id)
			DO UPDATE SET umsetzungsstatus=EXCLUDED.umsetzungsstatus,
			              begruendung=EXCLUDED.begruendung,
			              verantwortlicher=EXCLUDED.verantwortlicher,
			              umsetzungsdatum=EXCLUDED.umsetzungsdatum,
			              notiz=EXCLUDED.notiz,
			              updated_at=NOW()`,
			orgID, targetObjectID, bausteinID, item.AnforderungID,
			item.Umsetzungsstatus, item.Begruendung, item.Verantwortlicher, item.Umsetzungsdatum, item.Notiz)
		if err != nil {
			return fmt.Errorf("bulk set check result %s: %w", item.AnforderungID, err)
		}
	}
	return tx.Commit(ctx)
}

// GetCheckSummary returns aggregated progress for a Zielobjekt.
func (s *Service) GetCheckSummary(ctx context.Context, orgID, targetObjectID string) (CheckSummary, error) {
	var total, ja, teilweise, nein, entbehrlich int
	err := s.db.QueryRow(ctx, `
		SELECT
			COUNT(*),
			COUNT(*) FILTER (WHERE umsetzungsstatus = 'ja'),
			COUNT(*) FILTER (WHERE umsetzungsstatus = 'teilweise'),
			COUNT(*) FILTER (WHERE umsetzungsstatus = 'nein'),
			COUNT(*) FILTER (WHERE umsetzungsstatus = 'entbehrlich')
		FROM ck_bsi_check_results
		WHERE org_id=$1 AND target_object_id=$2`, orgID, targetObjectID).
		Scan(&total, &ja, &teilweise, &nein, &entbehrlich)
	if err != nil {
		return CheckSummary{}, fmt.Errorf("get check summary: %w", err)
	}

	return CheckSummary{
		TargetObjectID:     targetObjectID,
		TotalAnforderungen: total,
		CountJa:            ja,
		CountTeilweise:     teilweise,
		CountNein:          nein,
		CountEntbehrlich:   entbehrlich,
		UmsetzungsgradPct:  s.scorer.Score(ja, teilweise, entbehrlich, total),
	}, nil
}

// ── S74-2: Grundschutz-Cockpit & GAP-Report ──────────────────────────────────

// GetBSICockpit returns dashboard data: heatmap, top gaps, overall progress.
func (s *Service) GetBSICockpit(ctx context.Context, orgID string) (BSICockpit, error) {
	// Overall summary across all Zielobjekte.
	var totalAll, jaAll, teilweiseAll, entbehrlichAll int
	_ = s.db.QueryRow(ctx, `
		SELECT COUNT(*),
		       COUNT(*) FILTER (WHERE umsetzungsstatus='ja'),
		       COUNT(*) FILTER (WHERE umsetzungsstatus='teilweise'),
		       COUNT(*) FILTER (WHERE umsetzungsstatus='entbehrlich')
		FROM ck_bsi_check_results WHERE org_id=$1`, orgID).
		Scan(&totalAll, &jaAll, &teilweiseAll, &entbehrlichAll)

	gesamtPct := s.scorer.Score(jaAll, teilweiseAll, entbehrlichAll, totalAll)

	// Heatmap: group by baustein_id × target_object.
	heatmap, err := s.buildHeatmap(ctx, orgID)
	if err != nil {
		log.Warn().Err(err).Msg("cockpit: build heatmap")
		heatmap = nil
	}

	// Top-5 gaps: anforderungen with 'nein' in the most Zielobjekte.
	topGaps, err := s.getTopGaps(ctx, orgID, 5)
	if err != nil {
		log.Warn().Err(err).Msg("cockpit: get top gaps")
		topGaps = nil
	}

	// Überfällige: teilweise + umsetzungsdatum in the past.
	var ueberfaellig int
	_ = s.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM ck_bsi_check_results
		WHERE org_id=$1 AND umsetzungsstatus='teilweise'
		  AND umsetzungsdatum < CURRENT_DATE`, orgID).Scan(&ueberfaellig)

	if heatmap == nil {
		heatmap = []HeatmapRow{}
	}
	if topGaps == nil {
		topGaps = []BSIGapEntry{}
	}
	return BSICockpit{
		GesamtFortschrittPct: gesamtPct,
		Heatmap:              heatmap,
		TopGaps:              topGaps,
		UeberfaelligCount:    ueberfaellig,
	}, nil
}

func (s *Service) buildHeatmap(ctx context.Context, orgID string) ([]HeatmapRow, error) {
	rows, err := s.db.Query(ctx, `
		SELECT cr.baustein_id,
		       cr.target_object_id,
		       t.name,
		       COUNT(*) FILTER (WHERE umsetzungsstatus='ja') as ja,
		       COUNT(*) FILTER (WHERE umsetzungsstatus='teilweise') as teilweise,
		       COUNT(*) FILTER (WHERE umsetzungsstatus='entbehrlich') as entb,
		       COUNT(*) as total
		FROM ck_bsi_check_results cr
		JOIN ck_bsi_target_objects t ON t.id = cr.target_object_id
		WHERE cr.org_id=$1
		GROUP BY cr.baustein_id, cr.target_object_id, t.name
		ORDER BY cr.baustein_id, t.name`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bausteinMap := make(map[string]*HeatmapRow)
	var order []string
	for rows.Next() {
		var bid, tid, tname string
		var ja, teilweise, entb, total int
		if err := rows.Scan(&bid, &tid, &tname, &ja, &teilweise, &entb, &total); err != nil {
			return nil, err
		}
		if _, exists := bausteinMap[bid]; !exists {
			bausteinMap[bid] = &HeatmapRow{BausteinID: bid, BausteinTitle: bid}
			order = append(order, bid)
		}
		bausteinMap[bid].Cells = append(bausteinMap[bid].Cells, HeatmapCell{
			TargetObjectID:   tid,
			TargetObjectName: tname,
			FortschrittPct:   s.scorer.Score(ja, teilweise, entb, total),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]HeatmapRow, 0, len(order))
	for _, bid := range order {
		out = append(out, *bausteinMap[bid])
	}
	return out, nil
}

func (s *Service) getTopGaps(ctx context.Context, orgID string, limit int) ([]BSIGapEntry, error) {
	rows, err := s.db.Query(ctx, `
		SELECT baustein_id, anforderung_id,
		       array_agg(DISTINCT t.name) as zielobjekte,
		       COUNT(*) as affected
		FROM ck_bsi_check_results cr
		JOIN ck_bsi_target_objects t ON t.id = cr.target_object_id
		WHERE cr.org_id=$1 AND cr.umsetzungsstatus='nein'
		GROUP BY baustein_id, anforderung_id
		ORDER BY affected DESC
		LIMIT $2`, orgID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []BSIGapEntry
	for rows.Next() {
		var e BSIGapEntry
		var affected int
		if err := rows.Scan(&e.BausteinID, &e.AnforderungID,
			&e.BetroffeneZielobjekte, &affected); err != nil {
			return nil, err
		}
		e.Status = "nein"
		out = append(out, e)
	}
	return out, rows.Err()
}

// GetBSIGapReport returns the full GAP report for an org.
func (s *Service) GetBSIGapReport(ctx context.Context, orgID string) (BSIGapReport, error) {
	var total, entbehrlich, ja, teilweise, nein int
	_ = s.db.QueryRow(ctx, `
		SELECT COUNT(*),
		       COUNT(*) FILTER (WHERE umsetzungsstatus='entbehrlich'),
		       COUNT(*) FILTER (WHERE umsetzungsstatus='ja'),
		       COUNT(*) FILTER (WHERE umsetzungsstatus='teilweise'),
		       COUNT(*) FILTER (WHERE umsetzungsstatus='nein')
		FROM ck_bsi_check_results WHERE org_id=$1`, orgID).
		Scan(&total, &entbehrlich, &ja, &teilweise, &nein)

	rows, err := s.db.Query(ctx, gapReportSQL, orgID)
	if err != nil {
		return BSIGapReport{}, fmt.Errorf("get gap report: %w", err)
	}
	defer rows.Close()

	var gaps []BSIGapDetail
	for rows.Next() {
		var g BSIGapDetail
		if err := rows.Scan(&g.BausteinID, &g.AnforderungID, &g.AnforderungTitle,
			&g.Zielobjekt, &g.Umsetzungsstatus, &g.Verantwortlicher, &g.Umsetzungsdatum); err != nil {
			return BSIGapReport{}, fmt.Errorf("scan gap: %w", err)
		}
		gaps = append(gaps, g)
	}
	if err := rows.Err(); err != nil {
		return BSIGapReport{}, err
	}
	if gaps == nil {
		gaps = []BSIGapDetail{}
	}

	return BSIGapReport{
		OrgID:               orgID,
		GeneratedAt:         time.Now().UTC(),
		GesamtAnforderungen: total,
		GesamtEntbehrlich:   entbehrlich,
		GesamtJa:            ja,
		GesamtTeilweise:     teilweise,
		GesamtNein:          nein,
		UmsetzungsgradPct:   s.scorer.Score(ja, teilweise, entbehrlich, total),
		Gaps:                gaps,
	}, nil
}

// CalculateAndStoreBSIKPISnapshot computes the current BSI check progress and stores
// it in ck_isms_kpi_snapshots.bsi_check_pct. Called by the daily ISMS KPI snapshot job.
func (s *Service) CalculateAndStoreBSIKPISnapshot(ctx context.Context, orgID string) error {
	var total, ja, teilweise, entbehrlich int
	if err := s.db.QueryRow(ctx, `
		SELECT COUNT(*),
		       COUNT(*) FILTER (WHERE umsetzungsstatus='ja'),
		       COUNT(*) FILTER (WHERE umsetzungsstatus='teilweise'),
		       COUNT(*) FILTER (WHERE umsetzungsstatus='entbehrlich')
		FROM ck_bsi_check_results WHERE org_id=$1`, orgID).
		Scan(&total, &ja, &teilweise, &entbehrlich); err != nil {
		return fmt.Errorf("bsi kpi snapshot: count: %w", err)
	}
	pct := s.scorer.Score(ja, teilweise, entbehrlich, total)
	_, err := s.db.Exec(ctx, `
		UPDATE ck_isms_kpi_snapshots
		SET bsi_check_pct = $2
		WHERE org_id=$1
		  AND snapshot_date = (
		      SELECT MAX(snapshot_date) FROM ck_isms_kpi_snapshots WHERE org_id=$1
		  )`, orgID, pct)
	return err
}
