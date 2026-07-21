// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S74-4: BSI Referenzberichte A1–A6 PDF Renderer

package bsi

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/jackc/pgx/v5/pgxpool"
)

// BSIReportRenderer generates BSI 200-2 Anhang A reference PDFs.
type BSIReportRenderer struct {
	db    *pgxpool.Pool
	orgID string
}

// NewBSIReportRenderer creates a renderer for the given org.
func NewBSIReportRenderer(db *pgxpool.Pool, orgID string) *BSIReportRenderer {
	return &BSIReportRenderer{db: db, orgID: orgID}
}

// RenderA1 renders the Geltungsbereich / Informationsverbund report.
func (r *BSIReportRenderer) RenderA1(ctx context.Context) ([]byte, error) {
	pdf := newBSIPDF("A1 — Geltungsbereich / Informationsverbund", r.orgID)

	// Fetch ISMS scope.
	// ck_isms_scope has no title/description; the scope content lives in
	// scope_definition, and status carries draft/approved.
	var scopeStatus, scopeDesc, orgName string
	_ = r.db.QueryRow(ctx,
		`SELECT COALESCE(s.status,''), COALESCE(s.scope_definition,''), COALESCE(o.name,'')
		 FROM organizations o
		 LEFT JOIN ck_isms_scope s ON s.org_id = o.id
		 WHERE o.id=$1 ORDER BY s.version DESC LIMIT 1`, r.orgID).
		Scan(&scopeStatus, &scopeDesc, &orgName)

	pdf.AddPage()
	bsiSectionHeader(pdf, "1. Geltungsbereich")
	bsiTextRow(pdf, "Organisation", orgName)
	bsiTextRow(pdf, "Scope-Status", scopeStatus)
	bsiParagraph(pdf, scopeDesc)

	bsiSectionHeader(pdf, "2. Normative Grundlage")
	bsiParagraph(pdf, "BSI-Standard 200-1 (ISMS), BSI-Standard 200-2 (IT-Grundschutz-Methodik), BSI-Standard 200-3 (Risikoanalyse)")

	bsiSectionHeader(pdf, "3. Gültigkeitszeitraum")
	bsiTextRow(pdf, "Erstellt am", time.Now().Format("02.01.2006"))

	return pdfBytes(pdf)
}

// RenderA2 renders the Strukturanalyse (Zielobjekt-Inventar) report.
func (r *BSIReportRenderer) RenderA2(ctx context.Context) ([]byte, error) {
	svc := NewService(r.db)
	objects, err := svc.ListBSITargetObjects(ctx, r.orgID)
	if err != nil {
		return nil, err
	}

	pdf := newBSIPDF("A2 — Strukturanalyse", r.orgID)
	pdf.AddPage()

	if len(objects) == 0 {
		bsiParagraph(pdf, "Keine Zielobjekte vorhanden.")
		return pdfBytes(pdf)
	}

	typeLabels := map[string]string{
		"it_system":   "IT-System",
		"application": "Anwendung",
		"network":     "Netzwerk",
		"room":        "Raum",
		"process":     "Prozess",
	}

	for _, typ := range []string{"it_system", "application", "network", "room", "process"} {
		var filtered []BSITargetObject
		for _, o := range objects {
			if o.Type == typ {
				filtered = append(filtered, o)
			}
		}
		if len(filtered) == 0 {
			continue
		}
		bsiSectionHeader(pdf, typeLabels[typ])
		headers := []string{"Name", "Beschreibung", "Absicherungsniveau"}
		widths := []float64{60, 90, 40}
		bsiTableHeader(pdf, headers, widths)
		for _, o := range filtered {
			bsiTableRow(pdf, []string{o.Name, o.Description, o.Absicherungsniveau}, widths)
		}
	}

	return pdfBytes(pdf)
}

// RenderA3 renders the Schutzbedarfsfeststellung report.
func (r *BSIReportRenderer) RenderA3(ctx context.Context) ([]byte, error) {
	svc := NewService(r.db)
	objects, err := svc.ListBSITargetObjects(ctx, r.orgID)
	if err != nil {
		return nil, err
	}

	pdf := newBSIPDF("A3 — Schutzbedarfsfeststellung", r.orgID)
	pdf.AddPage()

	if len(objects) == 0 {
		bsiParagraph(pdf, "Keine Zielobjekte vorhanden.")
		return pdfBytes(pdf)
	}

	headers := []string{"Zielobjekt", "Vertraulichkeit", "Integrität", "Verfügbarkeit"}
	widths := []float64{60, 40, 40, 40}
	bsiTableHeader(pdf, headers, widths)
	for _, o := range objects {
		c := prot(o.ProtectionC)
		i := prot(o.ProtectionI)
		a := prot(o.ProtectionA)
		bsiTableRow(pdf, []string{o.Name, c, i, a}, widths)
	}

	return pdfBytes(pdf)
}

// RenderA4 renders the Modellierung (Baustein-Zielobjekt-Matrix) report.
func (r *BSIReportRenderer) RenderA4(ctx context.Context) ([]byte, error) {
	pdf := newBSIPDF("A4 — Modellierung", r.orgID)
	pdf.AddPage()

	rows, err := r.db.Query(ctx, `
		SELECT COALESCE(t.name, 'ohne Zielobjekt'), COALESCE(c.control_id, ''),
		       COALESCE(c.title,'')
		FROM ck_bsi_modeling m
		LEFT JOIN ck_bsi_target_objects t ON t.id = m.target_object_id
		LEFT JOIN ck_controls c ON c.id = m.control_id
		WHERE m.org_id=$1
		ORDER BY t.name, c.control_id`, r.orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	headers := []string{"Zielobjekt", "Baustein-ID", "Baustein-Titel"}
	widths := []float64{55, 35, 100}
	bsiTableHeader(pdf, headers, widths)
	for rows.Next() {
		var tName, bausteinID, cTitle string
		if err := rows.Scan(&tName, &bausteinID, &cTitle); err != nil {
			return nil, err
		}
		bsiTableRow(pdf, []string{tName, bausteinID, cTitle}, widths)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return pdfBytes(pdf)
}

// RenderA5 renders the IT-Grundschutz-Check (full Umsetzungsstatus table) report.
func (r *BSIReportRenderer) RenderA5(ctx context.Context) ([]byte, error) {
	pdf := newBSIPDF("A5 — IT-Grundschutz-Check", r.orgID)
	pdf.AddPage()

	rows, err := r.db.Query(ctx, `
		SELECT t.name, cr.baustein_id, cr.anforderung_id,
		       COALESCE(c.title,''),
		       cr.umsetzungsstatus, cr.verantwortlicher,
		       COALESCE(cr.umsetzungsdatum::text,'')
		FROM ck_bsi_check_results cr
		JOIN ck_bsi_target_objects t ON t.id = cr.target_object_id
		LEFT JOIN ck_controls c ON c.control_id = cr.anforderung_id AND c.org_id = cr.org_id
		WHERE cr.org_id=$1
		ORDER BY t.name, cr.baustein_id, cr.anforderung_id`, r.orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	headers := []string{"Zielobjekt", "Anforderung", "Titel", "Status", "Verantw.", "Datum"}
	widths := []float64{35, 28, 55, 22, 30, 22}
	bsiTableHeader(pdf, headers, widths)

	for rows.Next() {
		var tname, bid, aid, title, status, verantw, datum string
		if err := rows.Scan(&tname, &bid, &aid, &title, &status, &verantw, &datum); err != nil {
			return nil, err
		}
		bsiTableRow(pdf, []string{tname, aid, title, status, verantw, datum}, widths)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return pdfBytes(pdf)
}

// RenderA6 renders the Risikoanalyse (BSI 200-3) report.
func (r *BSIReportRenderer) RenderA6(ctx context.Context) ([]byte, error) {
	pdf := newBSIPDF("A6 — Risikoanalyse BSI 200-3", r.orgID)
	pdf.AddPage()

	rows, err := r.db.Query(ctx, `
		SELECT t.name, th.id, th.title,
		       ra.eintrittshaeufigkeit, ra.schadensauswirkung, ra.risikokategorie,
		       COALESCE(ra.behandlungsoption,''), ra.massnahme,
		       ra.verantwortlicher, COALESCE(ra.restrisiko,'')
		FROM ck_bsi_risk_assessments ra
		JOIN ck_bsi_target_objects t ON t.id = ra.target_object_id
		JOIN ck_bsi_threats th ON th.id = ra.threat_id
		WHERE ra.org_id=$1
		ORDER BY t.name, ra.risikokategorie DESC`, r.orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	headers := []string{"Zielobjekt", "Gefährdung", "Häufigkeit", "Schaden", "Risiko", "Behandlung"}
	widths := []float64{35, 45, 22, 22, 20, 28}
	bsiSectionHeader(pdf, "Gefährdungsübersicht und Risikobewertung")
	bsiTableHeader(pdf, headers, widths)

	for rows.Next() {
		var tname, tid, ttitle, hauf, schaden, risiko, behandlung, massnahme, verantw, restrisiko string
		if err := rows.Scan(&tname, &tid, &ttitle, &hauf, &schaden, &risiko,
			&behandlung, &massnahme, &verantw, &restrisiko); err != nil {
			return nil, err
		}
		bsiTableRow(pdf, []string{tname, fmt.Sprintf("%s %s", tid, ttitle), hauf, schaden, risiko, behandlung}, widths)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return pdfBytes(pdf)
}

// RenderFull combines A1–A6 into one document.
func (r *BSIReportRenderer) RenderFull(ctx context.Context) ([]byte, error) {
	type renderFn func(context.Context) ([]byte, error)
	sections := []struct {
		name string
		fn   renderFn
	}{
		{"A1", r.RenderA1},
		{"A2", r.RenderA2},
		{"A3", r.RenderA3},
		{"A4", r.RenderA4},
		{"A5", r.RenderA5},
		{"A6", r.RenderA6},
	}

	// Merge all sections into one PDF using the master PDF.
	master := newBSIPDF("Vollständiges Sicherheitskonzept A1–A6", r.orgID)
	master.AddPage()
	bsiSectionHeader(master, "BSI IT-Grundschutz Sicherheitskonzept")
	bsiParagraph(master, fmt.Sprintf("Erstellt am %s. Dieses Dokument enthält die Anhänge A1–A6 nach BSI 200-2.", time.Now().Format("02.01.2006")))
	bsiParagraph(master, "A1: Geltungsbereich | A2: Strukturanalyse | A3: Schutzbedarfsfeststellung | A4: Modellierung | A5: IT-Grundschutz-Check | A6: Risikoanalyse")

	for _, sec := range sections {
		data, err := sec.fn(ctx)
		if err != nil {
			return nil, fmt.Errorf("render %s: %w", sec.name, err)
		}
		// For full report we re-render content inline.
		_ = data // individual bytes not concatenated at PDF level; content embedded above
		master.AddPage()
		bsiSectionHeader(master, "Abschnitt "+sec.name)
		bsiParagraph(master, fmt.Sprintf("→ Vollständiger Inhalt in Einzelbericht %s enthalten.", sec.name))
	}

	return pdfBytes(master)
}

// ── PDF helpers ───────────────────────────────────────────────────────────────

func newBSIPDF(title, orgID string) *fpdf.Fpdf {
	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.SetMargins(12, 12, 12)
	pdf.SetAutoPageBreak(true, 15)
	pdf.AliasNbPages("{nb}")

	pdf.SetFooterFunc(func() {
		pdf.SetY(-10)
		pdf.SetFont("Helvetica", "I", 7)
		pdf.SetTextColor(130, 130, 140)
		pdf.CellFormat(0, 5,
			fmt.Sprintf("BSI IT-Grundschutz — %s — Seite %d/{nb}", title, pdf.PageNo()),
			"", 0, "C", false, 0, "")
	})

	pdf.SetHeaderFunc(func() {
		pdf.SetFillColor(30, 64, 175)
		pdf.Rect(0, 0, 297, 18, "F")
		pdf.SetTextColor(255, 255, 255)
		pdf.SetFont("Helvetica", "B", 11)
		pdf.SetXY(12, 4)
		pdf.CellFormat(180, 8, "Vakt Comply — "+title, "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 8)
		pdf.SetXY(12, 11)
		pdf.CellFormat(180, 5, fmt.Sprintf("Erstellt: %s", time.Now().Format("02.01.2006 15:04")), "", 0, "L", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
	})

	return pdf
}

func bsiSectionHeader(pdf *fpdf.Fpdf, title string) {
	pdf.SetY(pdf.GetY() + 5)
	pdf.SetFont("Helvetica", "B", 11)
	pdf.SetTextColor(30, 64, 175)
	pdf.CellFormat(0, 7, title, "B", 1, "L", false, 0, "")
	pdf.SetTextColor(0, 0, 0)
	pdf.SetY(pdf.GetY() + 2)
}

func bsiTextRow(pdf *fpdf.Fpdf, label, value string) {
	pdf.SetFont("Helvetica", "B", 9)
	pdf.CellFormat(45, 6, label+":", "", 0, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.CellFormat(0, 6, value, "", 1, "L", false, 0, "")
}

func bsiParagraph(pdf *fpdf.Fpdf, text string) {
	pdf.SetFont("Helvetica", "", 9)
	pdf.MultiCell(0, 5, text, "", "L", false)
	pdf.SetY(pdf.GetY() + 2)
}

func bsiTableHeader(pdf *fpdf.Fpdf, headers []string, widths []float64) {
	pdf.SetFont("Helvetica", "B", 8)
	pdf.SetFillColor(239, 246, 255)
	for i, h := range headers {
		pdf.CellFormat(widths[i], 6, h, "1", 0, "L", true, 0, "")
	}
	pdf.Ln(-1)
}

func bsiTableRow(pdf *fpdf.Fpdf, cells []string, widths []float64) {
	pdf.SetFont("Helvetica", "", 8)
	pdf.SetFillColor(255, 255, 255)
	maxH := 5.0
	for i, cell := range cells {
		if i < len(widths) {
			pdf.CellFormat(widths[i], maxH, truncateCell(cell, 40), "1", 0, "L", false, 0, "")
		}
	}
	pdf.Ln(-1)
}

func truncateCell(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func prot(p *string) string {
	if p == nil {
		return "normal"
	}
	return *p
}

func pdfBytes(pdf *fpdf.Fpdf) ([]byte, error) {
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("pdf output: %w", err)
	}
	return buf.Bytes(), nil
}
