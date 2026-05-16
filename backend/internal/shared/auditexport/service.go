package auditexport

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"html/template"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AuditPackage holds the generated ZIP and metadata.
type AuditPackage struct {
	GeneratedAt time.Time
	OrgName     string
	Zip         []byte
}

// GeneratePackage creates a ZIP containing CSV exports and an HTML gap analysis.
func GeneratePackage(ctx context.Context, db *pgxpool.Pool, orgID string) (*AuditPackage, error) {
	// Fetch org name
	var orgName string
	_ = db.QueryRow(ctx, `SELECT name FROM organizations WHERE id = $1::uuid`, orgID).Scan(&orgName)
	if orgName == "" {
		orgName = orgID
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	now := time.Now()

	// ── Controls CSV ─────────────────────────────────────────────────────────
	if err := writeControlsCSV(ctx, db, orgID, zw); err != nil {
		return nil, fmt.Errorf("controls csv: %w", err)
	}

	// ── Evidence CSV ─────────────────────────────────────────────────────────
	if err := writeEvidenceCSV(ctx, db, orgID, zw); err != nil {
		return nil, fmt.Errorf("evidence csv: %w", err)
	}

	// ── Findings CSV ─────────────────────────────────────────────────────────
	if err := writeFindingsCSV(ctx, db, orgID, zw); err != nil {
		return nil, fmt.Errorf("findings csv: %w", err)
	}

	// ── Risks CSV ─────────────────────────────────────────────────────────────
	if err := writeRisksCSV(ctx, db, orgID, zw); err != nil {
		return nil, fmt.Errorf("risks csv: %w", err)
	}

	// ── Incidents CSV ─────────────────────────────────────────────────────────
	if err := writeIncidentsCSV(ctx, db, orgID, zw); err != nil {
		return nil, fmt.Errorf("incidents csv: %w", err)
	}

	// ── Policies CSV ─────────────────────────────────────────────────────────
	if err := writePoliciesCSV(ctx, db, orgID, zw); err != nil {
		return nil, fmt.Errorf("policies csv: %w", err)
	}

	// ── Gap Analysis HTML ─────────────────────────────────────────────────────
	if err := writeGapAnalysisHTML(ctx, db, orgID, orgName, now, zw); err != nil {
		return nil, fmt.Errorf("gap analysis html: %w", err)
	}

	// ── README ───────────────────────────────────────────────────────────────
	f, _ := zw.Create("README.txt")
	fmt.Fprintf(f, "Vakt Audit-Paket\n")
	fmt.Fprintf(f, "Erstellt: %s\n", now.Format("02.01.2006 15:04"))
	fmt.Fprintf(f, "Organisation: %s\n\n", orgName)
	fmt.Fprintf(f, "Enthaltene Dateien:\n")
	fmt.Fprintf(f, "  controls.csv      — Alle Compliance-Controls mit Status\n")
	fmt.Fprintf(f, "  evidence.csv      — Gesammelte Evidenzen\n")
	fmt.Fprintf(f, "  findings.csv      — Offene Sicherheitslücken (Vakt Scan)\n")
	fmt.Fprintf(f, "  risks.csv         — Risikoregister (Vakt Comply)\n")
	fmt.Fprintf(f, "  incidents.csv     — Vorfallsregister (Vakt Comply)\n")
	fmt.Fprintf(f, "  policies.csv      — Richtlinien (Vakt Comply)\n")
	fmt.Fprintf(f, "  gap_analysis.html — Gap-Analyse-Bericht (im Browser öffnen)\n\n")
	fmt.Fprintf(f, "Für Audits: Bitte gap_analysis.html im Browser öffnen und als PDF drucken.\n")

	zw.Close()

	return &AuditPackage{
		GeneratedAt: now,
		OrgName:     orgName,
		Zip:         buf.Bytes(),
	}, nil
}

func writeControlsCSV(ctx context.Context, db *pgxpool.Pool, orgID string, zw *zip.Writer) error {
	f, _ := zw.Create("controls.csv")
	w := csv.NewWriter(f)
	_ = w.Write([]string{"ID", "Framework", "Domain", "Control-ID", "Titel", "Status", "Gewichtung", "Lücken-Hinweis"})
	rows, err := db.Query(ctx, `
		SELECT c.id, fr.name, c.domain, c.control_id, c.title, c.status, c.weight, COALESCE(c.gap_description, '')
		FROM ck_controls c
		JOIN ck_frameworks fr ON fr.id = c.framework_id
		WHERE c.org_id = $1::uuid
		ORDER BY fr.name, c.domain, c.control_id`, orgID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id, framework, domain, controlID, title, status, gap string
		var weight int
		if err := rows.Scan(&id, &framework, &domain, &controlID, &title, &status, &weight, &gap); err != nil {
			continue
		}
		_ = w.Write([]string{id, framework, domain, controlID, title, status, fmt.Sprint(weight), gap})
	}
	w.Flush()
	return rows.Err()
}

func writeEvidenceCSV(ctx context.Context, db *pgxpool.Pool, orgID string, zw *zip.Writer) error {
	f, _ := zw.Create("evidence.csv")
	w := csv.NewWriter(f)
	_ = w.Write([]string{"Control-ID", "Typ", "Titel", "Quelle", "Erstellt am"})
	rows, err := db.Query(ctx, `
		SELECT c.control_id, e.evidence_type, e.title, COALESCE(e.source, ''), e.created_at
		FROM ck_evidence e
		JOIN ck_controls c ON c.id = e.control_id
		WHERE e.org_id = $1::uuid
		ORDER BY e.created_at DESC`, orgID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var controlID, evType, title, source string
		var createdAt time.Time
		if err := rows.Scan(&controlID, &evType, &title, &source, &createdAt); err != nil {
			continue
		}
		_ = w.Write([]string{controlID, evType, title, source, createdAt.Format("02.01.2006")})
	}
	w.Flush()
	return rows.Err()
}

func writeFindingsCSV(ctx context.Context, db *pgxpool.Pool, orgID string, zw *zip.Writer) error {
	f, _ := zw.Create("findings.csv")
	w := csv.NewWriter(f)
	_ = w.Write([]string{"ID", "Titel", "Schweregrad", "Status", "CVE", "Asset", "SLA-Frist", "Erstellt am"})
	rows, err := db.Query(ctx, `
		SELECT f.id, f.title, f.severity, f.status, COALESCE(f.cve_id, ''), a.name,
		       COALESCE(f.sla_due_at::text, ''), f.created_at
		FROM vb_findings f
		JOIN vb_assets a ON a.id = f.asset_id
		WHERE f.org_id = $1::uuid
		ORDER BY CASE f.severity WHEN 'critical' THEN 1 WHEN 'high' THEN 2 WHEN 'medium' THEN 3 ELSE 4 END`,
		orgID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id, title, severity, status, cve, asset, sla string
		var createdAt time.Time
		if err := rows.Scan(&id, &title, &severity, &status, &cve, &asset, &sla, &createdAt); err != nil {
			continue
		}
		_ = w.Write([]string{id, title, severity, status, cve, asset, sla, createdAt.Format("02.01.2006")})
	}
	w.Flush()
	return rows.Err()
}

func writeRisksCSV(ctx context.Context, db *pgxpool.Pool, orgID string, zw *zip.Writer) error {
	f, _ := zw.Create("risks.csv")
	w := csv.NewWriter(f)
	_ = w.Write([]string{"ID", "Titel", "Kategorie", "Status", "Wahrscheinlichkeit", "Auswirkung", "Risiko-Score", "Behandlung"})
	rows, err := db.Query(ctx, `
		SELECT id, title, category, status, likelihood, impact, likelihood*impact,
		       COALESCE(treatment_strategy, '')
		FROM ck_risks
		WHERE org_id = $1::uuid
		ORDER BY likelihood*impact DESC`, orgID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id, title, category, status, treatment string
		var likelihood, impact, score int
		if err := rows.Scan(&id, &title, &category, &status, &likelihood, &impact, &score, &treatment); err != nil {
			continue
		}
		_ = w.Write([]string{id, title, category, status, fmt.Sprint(likelihood), fmt.Sprint(impact), fmt.Sprint(score), treatment})
	}
	w.Flush()
	return rows.Err()
}

func writeIncidentsCSV(ctx context.Context, db *pgxpool.Pool, orgID string, zw *zip.Writer) error {
	f, _ := zw.Create("incidents.csv")
	w := csv.NewWriter(f)
	_ = w.Write([]string{"ID", "Titel", "Schweregrad", "Status", "Entdeckt am", "Erstellt am"})
	rows, err := db.Query(ctx, `
		SELECT id, title, severity, status,
		       COALESCE(discovered_at::text, ''), created_at
		FROM ck_incidents
		WHERE org_id = $1::uuid
		ORDER BY created_at DESC`, orgID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id, title, severity, status, discoveredAt string
		var createdAt time.Time
		if err := rows.Scan(&id, &title, &severity, &status, &discoveredAt, &createdAt); err != nil {
			continue
		}
		_ = w.Write([]string{id, title, severity, status, discoveredAt, createdAt.Format("02.01.2006")})
	}
	w.Flush()
	return rows.Err()
}

func writePoliciesCSV(ctx context.Context, db *pgxpool.Pool, orgID string, zw *zip.Writer) error {
	f, _ := zw.Create("policies.csv")
	w := csv.NewWriter(f)
	_ = w.Write([]string{"ID", "Titel", "Kategorie", "Status", "Version", "Owner", "Überprüfung fällig"})
	rows, err := db.Query(ctx, `
		SELECT id, title, COALESCE(category, ''), status, COALESCE(version, ''),
		       COALESCE(owner, ''), COALESCE(review_date::text, '')
		FROM ck_policies
		WHERE org_id = $1::uuid
		ORDER BY title`, orgID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id, title, category, status, version, owner, reviewDate string
		if err := rows.Scan(&id, &title, &category, &status, &version, &owner, &reviewDate); err != nil {
			continue
		}
		_ = w.Write([]string{id, title, category, status, version, owner, reviewDate})
	}
	w.Flush()
	return rows.Err()
}

// ── Gap Analysis HTML ─────────────────────────────────────────────────────────

type gapData struct {
	OrgName     string
	GeneratedAt string
	Frameworks  []frameworkGap
	Summary     gapSummary
}

type frameworkGap struct {
	Name        string
	Implemented int
	InProgress  int
	Missing     int
	TotalActive int
	Score       int // percentage
	Domains     []domainGap
}

type domainGap struct {
	Name     string
	Controls []controlGap
}

type controlGap struct {
	ControlID string
	Title     string
	Status    string // implemented | in_progress | missing | not_applicable
	Domain    string
}

type gapSummary struct {
	TotalControls int
	Implemented   int
	InProgress    int
	Missing       int
	OverallScore  int
	OpenFindings  int
	OpenRisks     int
	OpenIncidents int
}

func writeGapAnalysisHTML(ctx context.Context, db *pgxpool.Pool, orgID, orgName string, now time.Time, zw *zip.Writer) error {
	data, err := buildGapData(ctx, db, orgID, orgName, now)
	if err != nil {
		return err
	}

	funcMap := template.FuncMap{
		"statusLabel": func(s string) string {
			switch s {
			case "implemented":
				return "Implementiert"
			case "in_progress":
				return "In Bearbeitung"
			case "missing":
				return "Fehlend"
			case "not_applicable":
				return "N/A"
			default:
				return s
			}
		},
	}

	tmpl, err := template.New("gap").Funcs(funcMap).Parse(gapHTMLTemplate)
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	f, _ := zw.Create("gap_analysis.html")
	return tmpl.Execute(f, data)
}

func buildGapData(ctx context.Context, db *pgxpool.Pool, orgID, orgName string, now time.Time) (*gapData, error) {
	// ── Controls grouped by framework and domain ──────────────────────────────
	rows, err := db.Query(ctx, `
		SELECT fr.name AS framework_name, c.domain, c.control_id, c.title, c.status
		FROM ck_controls c
		JOIN ck_frameworks fr ON fr.id = c.framework_id
		WHERE c.org_id = $1::uuid
		ORDER BY fr.name, c.domain, c.control_id`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type controlRow struct {
		framework string
		domain    string
		controlID string
		title     string
		status    string
	}

	var allControls []controlRow
	for rows.Next() {
		var r controlRow
		if err := rows.Scan(&r.framework, &r.domain, &r.controlID, &r.title, &r.status); err != nil {
			continue
		}
		allControls = append(allControls, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Build framework → domain → controls map
	fwOrder := []string{}
	fwMap := map[string]map[string][]controlGap{}
	for _, r := range allControls {
		if _, ok := fwMap[r.framework]; !ok {
			fwOrder = append(fwOrder, r.framework)
			fwMap[r.framework] = map[string][]controlGap{}
		}
		fwMap[r.framework][r.domain] = append(fwMap[r.framework][r.domain], controlGap{
			ControlID: r.controlID,
			Title:     r.title,
			Status:    r.status,
			Domain:    r.domain,
		})
	}

	// ── Open counts ───────────────────────────────────────────────────────────
	var openFindings int
	_ = db.QueryRow(ctx, `
		SELECT COUNT(*) FROM vb_findings
		WHERE org_id = $1::uuid AND status NOT IN ('resolved','false_positive')`, orgID).Scan(&openFindings)

	var openRisks int
	_ = db.QueryRow(ctx, `
		SELECT COUNT(*) FROM ck_risks
		WHERE org_id = $1::uuid AND status NOT IN ('accepted','closed','mitigated')`, orgID).Scan(&openRisks)

	var openIncidents int
	_ = db.QueryRow(ctx, `
		SELECT COUNT(*) FROM ck_incidents
		WHERE org_id = $1::uuid AND status NOT IN ('resolved','closed')`, orgID).Scan(&openIncidents)

	// ── Assemble framework gaps ───────────────────────────────────────────────
	var frameworks []frameworkGap
	summary := gapSummary{
		OpenFindings:  openFindings,
		OpenRisks:     openRisks,
		OpenIncidents: openIncidents,
	}

	for _, fwName := range fwOrder {
		domainMap := fwMap[fwName]

		// Stable domain order
		domainOrder := []string{}
		domainSeen := map[string]bool{}
		for _, r := range allControls {
			if r.framework == fwName && !domainSeen[r.domain] {
				domainOrder = append(domainOrder, r.domain)
				domainSeen[r.domain] = true
			}
		}

		var domains []domainGap
		var fwImpl, fwProg, fwMiss, fwTotal int
		for _, dName := range domainOrder {
			controls := domainMap[dName]
			domains = append(domains, domainGap{Name: dName, Controls: controls})
			for _, c := range controls {
				switch c.Status {
				case "implemented":
					fwImpl++
				case "in_progress":
					fwProg++
				case "missing":
					fwMiss++
				}
				if c.Status != "not_applicable" {
					fwTotal++
				}
			}
		}

		score := 0
		if fwTotal > 0 {
			score = fwImpl * 100 / fwTotal
		}

		frameworks = append(frameworks, frameworkGap{
			Name:        fwName,
			Implemented: fwImpl,
			InProgress:  fwProg,
			Missing:     fwMiss,
			TotalActive: fwTotal,
			Score:       score,
			Domains:     domains,
		})

		summary.TotalControls += fwTotal + (len(allControls) - fwTotal) // all controls including NA
		summary.Implemented += fwImpl
		summary.InProgress += fwProg
		summary.Missing += fwMiss
	}

	// Recalculate TotalControls correctly
	summary.TotalControls = len(allControls)
	if summary.TotalControls > 0 {
		summary.OverallScore = summary.Implemented * 100 / summary.TotalControls
	}

	return &gapData{
		OrgName:     orgName,
		GeneratedAt: now.Format("02.01.2006 15:04"),
		Frameworks:  frameworks,
		Summary:     summary,
	}, nil
}

// gapHTMLTemplate is the self-contained, print-ready gap analysis report.
// Uses html/template so all user data is auto-escaped.
var gapHTMLTemplate = `<!DOCTYPE html>
<html lang="de">
<head>
<meta charset="UTF-8">
<title>Gap-Analyse — {{.OrgName}}</title>
<style>
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body { font-family: Arial, sans-serif; font-size: 12px; color: #1a1a1a; margin: 40px; line-height: 1.5; }
  h1 { font-size: 22px; border-bottom: 2px solid #6366f1; padding-bottom: 8px; margin-bottom: 16px; }
  h2 { font-size: 16px; margin-top: 32px; margin-bottom: 8px; color: #1a1a1a; }
  h3 { font-size: 13px; color: #555; margin-top: 20px; margin-bottom: 6px; }
  .cover { margin-bottom: 40px; padding-bottom: 24px; border-bottom: 1px solid #e5e7eb; }
  .cover-meta { color: #6b7280; font-size: 11px; margin-top: 4px; }
  .score-block { display: flex; align-items: center; gap: 20px; margin-top: 20px; }
  .score { font-size: 48px; font-weight: bold; color: #6366f1; }
  .score-label { font-size: 13px; color: #6b7280; }
  .badge { display: inline-block; padding: 2px 8px; border-radius: 3px; font-size: 11px; font-weight: bold; }
  .implemented { background: #d1fae5; color: #065f46; }
  .in_progress { background: #fef3c7; color: #92400e; }
  .missing { background: #fee2e2; color: #991b1b; }
  .not_applicable { background: #f3f4f6; color: #6b7280; }
  table { border-collapse: collapse; width: 100%; margin: 8px 0 16px; }
  th { background: #f8f9fa; text-align: left; padding: 6px 8px; border-bottom: 2px solid #ddd; font-size: 11px; text-transform: uppercase; letter-spacing: 0.04em; }
  td { padding: 5px 8px; border-bottom: 1px solid #eee; }
  tr:last-child td { border-bottom: none; }
  .progress-bar { background: #e5e7eb; border-radius: 4px; height: 12px; width: 200px; display: inline-block; vertical-align: middle; }
  .progress-fill { background: #6366f1; border-radius: 4px; height: 12px; }
  .fw-section { margin-top: 40px; page-break-inside: avoid; }
  .fw-header { background: #f8f9fa; padding: 12px 16px; border-radius: 6px; margin-bottom: 12px; display: flex; align-items: center; justify-content: space-between; }
  .fw-title { font-size: 15px; font-weight: bold; }
  .fw-stats { font-size: 11px; color: #6b7280; margin-top: 2px; }
  .summary-table td:first-child { font-weight: bold; color: #374151; width: 220px; }
  .summary-table td:last-child { font-weight: bold; font-size: 14px; }
  @media print {
    body { margin: 20px; }
    .fw-section { page-break-before: auto; }
    .no-print { display: none; }
  }
</style>
</head>
<body>

<div class="cover">
  <h1>Gap-Analyse-Bericht</h1>
  <div class="cover-meta">Organisation: <strong>{{.OrgName}}</strong> &nbsp;·&nbsp; Erstellt: {{.GeneratedAt}}</div>
  <div class="score-block">
    <div>
      <div class="score">{{.Summary.OverallScore}}%</div>
      <div class="score-label">Gesamt-Compliance-Score</div>
    </div>
  </div>
</div>

<h2>Zusammenfassung</h2>
<table class="summary-table">
  <tr><td>Gesamt-Controls</td><td>{{.Summary.TotalControls}}</td></tr>
  <tr><td>Implementiert</td><td><span class="badge implemented">{{.Summary.Implemented}}</span></td></tr>
  <tr><td>In Bearbeitung</td><td><span class="badge in_progress">{{.Summary.InProgress}}</span></td></tr>
  <tr><td>Fehlend / Offen</td><td><span class="badge missing">{{.Summary.Missing}}</span></td></tr>
  <tr><td>Offene Sicherheitslücken</td><td>{{.Summary.OpenFindings}}</td></tr>
  <tr><td>Offene Risiken</td><td>{{.Summary.OpenRisks}}</td></tr>
  <tr><td>Offene Vorfälle</td><td>{{.Summary.OpenIncidents}}</td></tr>
</table>

{{range .Frameworks}}
<div class="fw-section">
  <div class="fw-header">
    <div>
      <div class="fw-title">{{.Name}}</div>
      <div class="fw-stats">{{.Implemented}} implementiert · {{.InProgress}} in Bearbeitung · {{.Missing}} fehlend · {{.TotalActive}} aktive Controls</div>
    </div>
    <div style="text-align:right;">
      <div style="font-size:20px;font-weight:bold;color:#6366f1;">{{.Score}}%</div>
      <div style="margin-top:4px;">
        <div class="progress-bar"><div class="progress-fill" style="width:{{.Score}}%;"></div></div>
      </div>
    </div>
  </div>

  {{range .Domains}}
  <h3>{{.Name}}</h3>
  <table>
    <thead>
      <tr>
        <th style="width:100px;">Control-ID</th>
        <th>Titel</th>
        <th style="width:120px;">Status</th>
      </tr>
    </thead>
    <tbody>
      {{range .Controls}}
      <tr>
        <td style="font-family:monospace;font-size:11px;">{{.ControlID}}</td>
        <td>{{.Title}}</td>
        <td><span class="badge {{.Status}}">{{statusLabel .Status}}</span></td>
      </tr>
      {{end}}
    </tbody>
  </table>
  {{end}}
</div>
{{end}}

</body>
</html>
`

