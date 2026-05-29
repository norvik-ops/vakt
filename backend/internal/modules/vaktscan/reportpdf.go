package vaktscan

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

type reportData struct {
	OrgName     string
	Title       string
	GeneratedAt time.Time
	Total       int
	Critical    int
	High        int
	Medium      int
	Low         int
	Info        int
	Open        int
	Findings    []reportFinding
	DataError   bool // true wenn mindestens eine DB-Abfrage fehlgeschlagen ist
}

type reportFinding struct {
	Title    string
	Severity string
	Status   string
	Asset    string
}

func gatherReportData(ctx context.Context, db *pgxpool.Pool, orgID, title string) (*reportData, error) {
	d := &reportData{Title: title, GeneratedAt: time.Now()}

	if err := db.QueryRow(ctx, `SELECT name FROM organizations WHERE id=$1::uuid`, orgID).Scan(&d.OrgName); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("vaktscan reportpdf: org name lookup failed")
	}

	if err := db.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE severity='critical'),
			COUNT(*) FILTER (WHERE severity='high'),
			COUNT(*) FILTER (WHERE severity='medium'),
			COUNT(*) FILTER (WHERE severity='low'),
			COUNT(*) FILTER (WHERE severity='info'),
			COUNT(*) FILTER (WHERE status NOT IN ('resolved','false_positive')),
			COUNT(*)
		FROM vb_findings WHERE org_id=$1::uuid`, orgID,
	).Scan(&d.Critical, &d.High, &d.Medium, &d.Low, &d.Info, &d.Open, &d.Total); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("vaktscan reportpdf: stats query failed — PDF will show zero counts")
		d.DataError = true
	}

	rows, err := db.Query(ctx, `
		SELECT f.title, f.severity, f.status, COALESCE(a.name,'–')
		FROM vb_findings f
		LEFT JOIN vb_assets a ON a.id = f.asset_id
		WHERE f.org_id=$1::uuid AND f.status NOT IN ('resolved','false_positive')
		ORDER BY
			CASE f.severity WHEN 'critical' THEN 0 WHEN 'high' THEN 1
			                WHEN 'medium' THEN 2 WHEN 'low' THEN 3 ELSE 4 END,
			f.created_at DESC
		LIMIT 200`, orgID)
	if err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("vaktscan reportpdf: findings query failed")
		d.DataError = true
	} else {
		defer rows.Close()
		for rows.Next() {
			var f reportFinding
			if rows.Scan(&f.Title, &f.Severity, &f.Status, &f.Asset) == nil {
				d.Findings = append(d.Findings, f)
			}
		}
	}

	return d, nil
}

var severityColor = map[string][3]int{
	"critical": {220, 38, 38},
	"high":     {234, 88, 12},
	"medium":   {234, 179, 8},
	"low":      {59, 130, 246},
	"info":     {107, 114, 128},
}

func GenerateReportPDF(ctx context.Context, db *pgxpool.Pool, orgID, title string) ([]byte, error) {
	d, err := gatherReportData(ctx, db, orgID, title)
	if err != nil {
		return nil, err
	}

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 15)

	// ── Title page ────────────────────────────────────────────────────────────
	pdf.AddPage()

	// Fehlerindikator: wenn DB-Abfragen fehlgeschlagen sind, wird ein
	// deutlich sichtbarer Warnhinweis am Seitenanfang eingeblendet.
	if d.DataError {
		pdf.SetFillColor(220, 38, 38)
		pdf.Rect(0, 0, 210, 12, "F")
		pdf.SetTextColor(255, 255, 255)
		pdf.SetFont("Helvetica", "B", 8)
		pdf.SetXY(15, 3)
		pdf.CellFormat(180, 6, "HINWEIS: Datenbankfehler — Bericht unvollständig. Bitte erneut generieren oder Administrator kontaktieren.", "", 1, "C", false, 0, "")
	}

	// Header bar
	pdf.SetFillColor(37, 99, 235)
	pdf.Rect(0, 0, 210, 28, "F")
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 14)
	pdf.SetXY(15, 8)
	pdf.CellFormat(180, 8, "Vakt — Security Report", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetXY(15, 17)
	pdf.CellFormat(180, 6, d.OrgName, "", 1, "L", false, 0, "")

	pdf.SetTextColor(30, 30, 40)
	pdf.SetXY(15, 35)
	pdf.SetFont("Helvetica", "B", 18)
	pdf.CellFormat(180, 10, d.Title, "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(100, 100, 120)
	pdf.CellFormat(180, 7, fmt.Sprintf("Erstellt am %s", d.GeneratedAt.Format("02.01.2006 15:04")), "", 1, "L", false, 0, "")

	// ── Summary boxes ─────────────────────────────────────────────────────────
	pdf.SetY(60)
	pdf.SetFont("Helvetica", "B", 11)
	pdf.SetTextColor(30, 30, 40)
	pdf.CellFormat(180, 8, "Zusammenfassung", "", 1, "L", false, 0, "")

	summaries := []struct {
		label   string
		value   int
		r, g, b int
	}{
		{"Gesamt", d.Total, 55, 65, 81},
		{"Kritisch", d.Critical, 220, 38, 38},
		{"Hoch", d.High, 234, 88, 12},
		{"Mittel", d.Medium, 234, 179, 8},
		{"Niedrig", d.Low, 59, 130, 246},
		{"Offen", d.Open, 124, 58, 237},
	}

	boxW := 28.0
	gap := 2.0
	startX := 15.0
	y := pdf.GetY() + 4

	for i, s := range summaries {
		x := startX + float64(i)*(boxW+gap)
		pdf.SetFillColor(s.r, s.g, s.b)
		pdf.RoundedRect(x, y, boxW, 20, 2, "1234", "F")
		pdf.SetTextColor(255, 255, 255)
		pdf.SetFont("Helvetica", "B", 14)
		pdf.SetXY(x, y+3)
		pdf.CellFormat(boxW, 8, fmt.Sprintf("%d", s.value), "", 1, "C", false, 0, "")
		pdf.SetFont("Helvetica", "", 7)
		pdf.SetXY(x, y+11)
		pdf.CellFormat(boxW, 6, s.label, "", 1, "C", false, 0, "")
	}

	// ── Findings table ────────────────────────────────────────────────────────
	pdf.SetY(y + 30)
	pdf.SetTextColor(30, 30, 40)
	pdf.SetFont("Helvetica", "B", 11)
	pdf.CellFormat(180, 8, fmt.Sprintf("Offene Findings (%d)", d.Open), "", 1, "L", false, 0, "")
	pdf.Ln(1)

	if len(d.Findings) == 0 {
		pdf.SetFont("Helvetica", "I", 10)
		pdf.SetTextColor(100, 100, 120)
		pdf.CellFormat(180, 8, "Keine offenen Findings gefunden.", "", 1, "L", false, 0, "")
	} else {
		// Table header
		pdf.SetFillColor(37, 99, 235)
		pdf.SetTextColor(255, 255, 255)
		pdf.SetFont("Helvetica", "B", 8)
		pdf.CellFormat(80, 7, "Titel", "0", 0, "L", true, 0, "")
		pdf.CellFormat(22, 7, "Schwere", "0", 0, "C", true, 0, "")
		pdf.CellFormat(28, 7, "Status", "0", 0, "C", true, 0, "")
		pdf.CellFormat(50, 7, "Asset", "0", 1, "L", true, 0, "")

		for i, f := range d.Findings {
			if pdf.GetY() > 270 {
				pdf.AddPage()
				// repeat header
				pdf.SetFillColor(37, 99, 235)
				pdf.SetTextColor(255, 255, 255)
				pdf.SetFont("Helvetica", "B", 8)
				pdf.CellFormat(80, 7, "Titel", "0", 0, "L", true, 0, "")
				pdf.CellFormat(22, 7, "Schwere", "0", 0, "C", true, 0, "")
				pdf.CellFormat(28, 7, "Status", "0", 0, "C", true, 0, "")
				pdf.CellFormat(50, 7, "Asset", "0", 1, "L", true, 0, "")
			}

			if i%2 == 0 {
				pdf.SetFillColor(245, 247, 255)
			} else {
				pdf.SetFillColor(255, 255, 255)
			}

			// Severity badge color
			col := severityColor[f.Severity]
			pdf.SetTextColor(30, 30, 40)
			pdf.SetFont("Helvetica", "", 8)

			title := f.Title
			if len(title) > 48 {
				title = title[:45] + "..."
			}
			asset := f.Asset
			if len(asset) > 30 {
				asset = asset[:27] + "..."
			}

			pdf.CellFormat(80, 6, title, "0", 0, "L", true, 0, "")
			pdf.SetTextColor(col[0], col[1], col[2])
			pdf.SetFont("Helvetica", "B", 8)
			pdf.CellFormat(22, 6, f.Severity, "0", 0, "C", true, 0, "")
			pdf.SetTextColor(30, 30, 40)
			pdf.SetFont("Helvetica", "", 8)
			pdf.CellFormat(28, 6, f.Status, "0", 0, "C", true, 0, "")
			pdf.CellFormat(50, 6, asset, "0", 1, "L", true, 0, "")
		}
	}

	// ── Footer on each page ───────────────────────────────────────────────────
	pdf.SetFooterFunc(func() {
		pdf.SetY(-12)
		pdf.SetFont("Helvetica", "I", 7)
		pdf.SetTextColor(150, 150, 160)
		pdf.CellFormat(0, 5,
			fmt.Sprintf("Vakt Security Report — %s — Seite %d/{nb}", d.OrgName, pdf.PageNo()),
			"", 0, "C", false, 0, "")
	})
	pdf.AliasNbPages("{nb}")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("pdf output: %w", err)
	}
	return buf.Bytes(), nil
}
