// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktscan

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/matharnica/vakt/internal/db"
)

// ---------------------------------------------------------------------------
// Reports
// ---------------------------------------------------------------------------

// CreateReport inserts a new report record.
func (r *Repository) CreateReport(ctx context.Context, orgID, userID string, scope ReportScope) (*Report, error) {
	scopeJSON, err := json.Marshal(scope)
	if err != nil {
		return nil, fmt.Errorf("marshal report scope: %w", err)
	}
	row, err := r.q.CreateSPReport(ctx, db.CreateSPReportParams{
		OrgID:       orgID,
		GeneratedBy: spOptUUID(&userID),
		Scope:       scopeJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("insert report: %w", err)
	}
	rpt := reportFromFields(reportFields{
		ID: row.ID, OrgID: row.OrgID, GeneratedBy: row.GeneratedBy,
		Scope: row.Scope, FilePath: row.FilePath, Status: row.Status,
		ExpiresAt: row.ExpiresAt, CreatedAt: row.CreatedAt,
	})
	return &rpt, nil
}

// GetReport fetches a report by ID within the org.
func (r *Repository) GetReport(ctx context.Context, orgID, reportID string) (*Report, error) {
	row, err := r.q.GetSPReport(ctx, db.GetSPReportParams{ID: reportID, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("get report: %w", err)
	}
	rpt := reportFromFields(reportFields{
		ID: row.ID, OrgID: row.OrgID, GeneratedBy: row.GeneratedBy,
		Scope: row.Scope, FilePath: row.FilePath, Status: row.Status,
		ExpiresAt: row.ExpiresAt, CreatedAt: row.CreatedAt,
	})
	return &rpt, nil
}

// ListReports returns reports for an org, newest first (metadata only — no PDF blob).
func (r *Repository) ListReports(ctx context.Context, orgID string) ([]Report, error) {
	rows, err := r.q.ListSPReports(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list reports: %w", err)
	}
	out := make([]Report, 0, len(rows))
	for _, row := range rows {
		out = append(out, reportFromFields(reportFields{
			ID: row.ID, OrgID: row.OrgID, GeneratedBy: row.GeneratedBy,
			Scope: row.Scope, FilePath: row.FilePath, Status: row.Status,
			ExpiresAt: row.ExpiresAt, CreatedAt: row.CreatedAt,
		}))
	}
	return out, nil
}

// UpsertImportedFinding legt einen importierten Fund an oder führt ihn mit einem
// vorhandenen zusammen — und wählt dafür den Dedup-Schlüssel, der zu dem Fund
// passt (SARIF, CycloneDX, CSV, Wazuh).
//
// Warum die Wahl hier steht und nicht beim Aufrufer (R1-RV-01):
//
// vb_findings hat drei partielle Unique-Indexe (Migration 120). Zwei davon
// betreffen den Import-Weg:
//
//	idx_vb_findings_dedup_cve   (org_id, asset_id, cve_id) WHERE cve_id IS NOT NULL
//	idx_vb_findings_dedup_rawid (org_id, raw_id, scanner)  WHERE raw_id IS NOT NULL
//
// Der CVE-Index ist der einzige SCANNER-AGNOSTISCHE. Nur über ihn führen zwei
// verschiedene Werkzeuge denselben Fund zusammen; der raw_id-Index kann das
// nicht, er trägt den Scanner im Schlüssel.
//
// Beide bewachen aber dieselbe Zeile. Wer `cve_id` füllt und weiterhin über den
// raw_id-Arbiter schreibt, bekommt deshalb SQLSTATE 23505 statt einer
// Zusammenführung, sobald ein zweiter Scanner dieselbe CVE auf demselben Asset
// meldet. „cve_id setzen" und „Arbiter umstellen" sind aus diesem Grund keine
// zwei Aufgaben, sondern eine — und sie gehören an EINE Stelle, sonst driften
// sie auseinander.
//
// Regel: Trägt der Fund eine CVE, ist die CVE sein Dedup-Schlüssel und `raw_id`
// bleibt NULL (die Query lässt die Spalte weg — Begründung dort). Trägt er
// keine, bleibt es beim raw_id-Schlüssel wie bisher.
//
// Umbenannt aus `UpsertFindingByRawID`: Der alte Name beschrieb ab dem Moment
// das Falsche, in dem die Methode zwei Arbiter kennt.
func (r *Repository) UpsertImportedFinding(ctx context.Context, orgID string, f Finding) (*Finding, error) {
	// Derselbe Choke-Point wie in BatchUpsertFindings: Ein Schweregrad, den der
	// CHECK nicht kennt, darf keinen Import abbrechen. Hier trifft es zwar nur
	// eine Zeile statt eines ganzen Stapels — der Aufrufer (SARIF, CycloneDX)
	// bricht die Schleife aber beim ersten Fehler ab, der Rest der Datei geht
	// also genauso verloren.
	f.Severity, _ = normalizeSeverity(f.Severity)

	sources := f.Sources
	if sources == nil {
		sources = []string{}
	}

	if cve := cveKey(f.CVEID); cve != nil {
		row, err := r.q.UpsertSPFindingByCVE(ctx, db.UpsertSPFindingByCVEParams{
			OrgID:       orgID,
			AssetID:     f.AssetID,
			CveID:       optTextPtr(cve),
			Title:       f.Title,
			Description: spOptText(f.Description),
			Severity:    f.Severity,
			CvssScore:   float64PtrToNumeric(f.CVSSScore),
			Status:      f.Status,
			Scanner:     f.Scanner,
			Sources:     sources,
			SlaDueAt:    spOptTs(f.SLADueAt),
		})
		if err != nil {
			return nil, fmt.Errorf("upsert finding by cve_id: %w", err)
		}
		out := findingFromVbFindings(row)
		r.emitNewFinding(ctx, orgID, out)
		return &out, nil
	}

	row, err := r.q.UpsertSPFindingByRawID(ctx, db.UpsertSPFindingByRawIDParams{
		OrgID:   orgID,
		AssetID: f.AssetID,
		// Per Definition dieses Zweigs leer: hätte der Fund eine CVE, liefe er
		// oben. Ein Leerstring darf hier nicht stehen — `''` ist NOT NULL und
		// würde den CVE-Index für jede Zeile scharf machen (siehe cveKey).
		CveID:       optTextPtr(nil),
		Title:       f.Title,
		Description: spOptText(f.Description),
		Severity:    f.Severity,
		CvssScore:   float64PtrToNumeric(f.CVSSScore),
		Status:      f.Status,
		Scanner:     f.Scanner,
		RawID:       spOptText(f.RawID),
		Sources:     sources,
		SlaDueAt:    spOptTs(f.SLADueAt),
	})
	if err != nil {
		return nil, fmt.Errorf("upsert finding by raw_id: %w", err)
	}
	out := findingFromVbFindings(row)
	r.emitNewFinding(ctx, orgID, out)
	return &out, nil
}

// UpdateReport updates a report's file path, status, and expiry.
func (r *Repository) UpdateReport(ctx context.Context, reportID, filePath, status string, expiresAt *time.Time) error {
	err := r.q.UpdateSPReport(ctx, db.UpdateSPReportParams{
		ID:        reportID,
		FilePath:  spOptText(filePath),
		Status:    status,
		ExpiresAt: spOptTs(expiresAt),
	})
	if err != nil {
		return fmt.Errorf("update report: %w", err)
	}
	return nil
}

// StoreReportContent saves a generated PDF and marks the report completed.
func (r *Repository) StoreReportContent(ctx context.Context, reportID string, content []byte, expiresAt time.Time) error {
	err := r.q.StoreSPReportContent(ctx, db.StoreSPReportContentParams{
		ID:        reportID,
		Content:   content,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("store report content: %w", err)
	}
	return nil
}

// GetReportContent returns the raw PDF bytes and title for a completed report.
func (r *Repository) GetReportContent(ctx context.Context, orgID, reportID string) ([]byte, string, error) {
	row, err := r.q.GetSPReportContent(ctx, db.GetSPReportContentParams{ID: reportID, OrgID: orgID})
	if err != nil {
		return nil, "", fmt.Errorf("get report content: %w", err)
	}
	var scope ReportScope
	if len(row.Scope) > 0 {
		_ = json.Unmarshal(row.Scope, &scope)
	}
	title := scope.Title
	if title == "" {
		title = "report"
	}
	return row.Content, title, nil
}
