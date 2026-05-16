package ai

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ReportType string

const (
	ReportGapAnalysis      ReportType = "gap_analysis"
	ReportRiskSummary      ReportType = "risk_summary"
	ReportExecutiveSummary ReportType = "executive_summary"
)

type ComplianceContext struct {
	OrgName          string
	GeneratedAt      time.Time
	TotalControls    int
	Implemented      int
	InProgress       int
	Missing          int
	OverallScore     int
	OpenFindings     int
	CriticalRisks    int
	OpenIncidents    int
	ActiveFrameworks []string
	TopGaps          []string // top 5 missing control titles
	TopRisks         []string // top 5 high/critical risk titles
}

func GatherContext(ctx context.Context, db *pgxpool.Pool, orgID string) (*ComplianceContext, error) {
	cc := &ComplianceContext{GeneratedAt: time.Now()}

	_ = db.QueryRow(ctx, `SELECT name FROM organizations WHERE id = $1::uuid`, orgID).Scan(&cc.OrgName)

	// Control statistics
	_ = db.QueryRow(ctx, `
        SELECT
            COUNT(*) FILTER (WHERE status = 'implemented'),
            COUNT(*) FILTER (WHERE status = 'in_progress'),
            COUNT(*) FILTER (WHERE status = 'missing'),
            COUNT(*)
        FROM ck_controls
        WHERE org_id = $1::uuid AND status != 'not_applicable'`, orgID,
	).Scan(&cc.Implemented, &cc.InProgress, &cc.Missing, &cc.TotalControls)

	if cc.TotalControls > 0 {
		cc.OverallScore = (cc.Implemented * 100) / cc.TotalControls
	}

	// Open findings
	_ = db.QueryRow(ctx, `
        SELECT COUNT(*) FROM vb_findings
        WHERE org_id = $1::uuid AND status NOT IN ('resolved', 'false_positive')`, orgID,
	).Scan(&cc.OpenFindings)

	// Critical risks
	_ = db.QueryRow(ctx, `
        SELECT COUNT(*) FROM ck_risks
        WHERE org_id = $1::uuid AND status NOT IN ('accepted','closed','mitigated')
          AND likelihood * impact >= 15`, orgID,
	).Scan(&cc.CriticalRisks)

	// Open incidents
	_ = db.QueryRow(ctx, `
        SELECT COUNT(*) FROM ck_incidents
        WHERE org_id = $1::uuid AND status NOT IN ('resolved','closed')`, orgID,
	).Scan(&cc.OpenIncidents)

	// Active frameworks
	rows, err := db.Query(ctx, `SELECT name FROM ck_frameworks WHERE org_id = $1::uuid AND is_active = true ORDER BY name`, orgID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var name string
			if rows.Scan(&name) == nil {
				cc.ActiveFrameworks = append(cc.ActiveFrameworks, name)
			}
		}
	}

	// Top 5 missing controls
	gapRows, err := db.Query(ctx, `
        SELECT c.title FROM ck_controls c
        WHERE c.org_id = $1::uuid AND c.status = 'missing' AND c.weight >= 3
        ORDER BY c.weight DESC LIMIT 5`, orgID)
	if err == nil {
		defer gapRows.Close()
		for gapRows.Next() {
			var title string
			if gapRows.Scan(&title) == nil {
				cc.TopGaps = append(cc.TopGaps, title)
			}
		}
	}

	// Top 5 high/critical risks
	riskRows, err := db.Query(ctx, `
        SELECT title FROM ck_risks
        WHERE org_id = $1::uuid AND status NOT IN ('accepted','closed','mitigated')
        ORDER BY likelihood * impact DESC LIMIT 5`, orgID)
	if err == nil {
		defer riskRows.Close()
		for riskRows.Next() {
			var title string
			if riskRows.Scan(&title) == nil {
				cc.TopRisks = append(cc.TopRisks, title)
			}
		}
	}

	return cc, nil
}

type Service struct {
	db     *pgxpool.Pool
	client *AIClient
}

func NewService(db *pgxpool.Pool, baseURL, apiKey, model string) *Service {
	return &Service{
		db:     db,
		client: NewAIClient(baseURL, apiKey, model),
	}
}

func (s *Service) IsAvailable(ctx context.Context) bool {
	return s.client.IsAvailable(ctx)
}

// AdviceContext holds the minimal data needed to build a weekly action-plan prompt.
type AdviceContext struct {
	OrgName         string
	FrameworkScores []frameworkScore
	OpenCAPAs       int
	OverdueControls int
	OverdueTasks    int
	CriticalRisks   []string // top 5 titles (score >= 15)
	OpenIncidents   int
	DraftPolicies   int
}

type frameworkScore struct {
	Name        string
	Implemented int
	Total       int
}

// GatherAdviceContext collects the compact dataset needed for the weekly advice prompt.
// All queries soft-fail so a missing table never blocks the response.
func GatherAdviceContext(ctx context.Context, db *pgxpool.Pool, orgID string) (*AdviceContext, error) {
	ac := &AdviceContext{}

	// Org name
	_ = db.QueryRow(ctx, `SELECT name FROM organizations WHERE id = $1::uuid`, orgID).Scan(&ac.OrgName)
	if ac.OrgName == "" {
		ac.OrgName = "Ihre Organisation"
	}

	// Per-framework scores
	rows, err := db.Query(ctx, `
		SELECT f.name,
		       COUNT(c.id) FILTER (WHERE c.manual_status IN ('implemented','partially_implemented'))::int,
		       COUNT(c.id)::int
		FROM ck_frameworks f
		LEFT JOIN ck_controls c ON c.framework_id = f.id AND c.org_id = f.org_id
		WHERE f.org_id = $1::uuid
		GROUP BY f.id, f.name
		ORDER BY f.name`, orgID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var fs frameworkScore
			if rows.Scan(&fs.Name, &fs.Implemented, &fs.Total) == nil {
				ac.FrameworkScores = append(ac.FrameworkScores, fs)
			}
		}
	}

	// Open CAPAs
	_ = db.QueryRow(ctx,
		`SELECT COUNT(*)::int FROM ck_capas WHERE org_id=$1::uuid AND status != 'closed'`,
		orgID).Scan(&ac.OpenCAPAs)

	// Overdue controls
	_ = db.QueryRow(ctx,
		`SELECT COUNT(*)::int FROM ck_controls
		 WHERE org_id=$1::uuid AND next_review_due IS NOT NULL AND next_review_due < NOW()`,
		orgID).Scan(&ac.OverdueControls)

	// Overdue tasks
	_ = db.QueryRow(ctx,
		`SELECT COUNT(*)::int FROM ck_tasks
		 WHERE org_id=$1::uuid AND due_date IS NOT NULL AND due_date < NOW() AND status != 'done'`,
		orgID).Scan(&ac.OverdueTasks)

	// Critical risk titles (score >= 15, top 5)
	riskRows, err := db.Query(ctx,
		`SELECT title FROM ck_risks
		 WHERE org_id=$1::uuid AND status NOT IN ('accepted','closed','mitigated')
		   AND likelihood * impact >= 15
		 ORDER BY likelihood * impact DESC LIMIT 5`, orgID)
	if err == nil {
		defer riskRows.Close()
		for riskRows.Next() {
			var t string
			if riskRows.Scan(&t) == nil {
				ac.CriticalRisks = append(ac.CriticalRisks, t)
			}
		}
	}

	// Open incidents
	_ = db.QueryRow(ctx,
		`SELECT COUNT(*)::int FROM ck_incidents
		 WHERE org_id=$1::uuid AND status NOT IN ('resolved','closed')`,
		orgID).Scan(&ac.OpenIncidents)

	// Policies in draft or with no version (need review)
	_ = db.QueryRow(ctx,
		`SELECT COUNT(*)::int FROM ck_policies
		 WHERE org_id=$1::uuid AND (status = 'draft' OR version IS NULL OR version = '')`,
		orgID).Scan(&ac.DraftPolicies)

	return ac, nil
}

func buildAdvicePrompt(ac *AdviceContext) string {
	var sb strings.Builder

	sb.WriteString("Compliance-Status für ")
	sb.WriteString(ac.OrgName)
	sb.WriteString(":\n\n")

	if len(ac.FrameworkScores) > 0 {
		sb.WriteString("Frameworks:\n")
		for _, fs := range ac.FrameworkScores {
			pct := 0
			if fs.Total > 0 {
				pct = (fs.Implemented * 100) / fs.Total
			}
			fmt.Fprintf(&sb, "- %s: %d/%d Controls implementiert (%d%%)\n",
				fs.Name, fs.Implemented, fs.Total, pct)
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Offene Probleme:\n")
	if len(ac.CriticalRisks) > 0 {
		sb.WriteString(fmt.Sprintf("- %d kritische Risiken: ", len(ac.CriticalRisks)))
		sb.WriteString(strings.Join(ac.CriticalRisks, ", "))
		sb.WriteString("\n")
	}
	if ac.OverdueControls > 0 {
		fmt.Fprintf(&sb, "- %d überfällige Controls\n", ac.OverdueControls)
	}
	if ac.OverdueTasks > 0 {
		fmt.Fprintf(&sb, "- %d überfällige Aufgaben\n", ac.OverdueTasks)
	}
	if ac.OpenCAPAs > 0 {
		fmt.Fprintf(&sb, "- %d offene CAPAs\n", ac.OpenCAPAs)
	}
	if ac.OpenIncidents > 0 {
		fmt.Fprintf(&sb, "- %d offene Vorfälle\n", ac.OpenIncidents)
	}
	if ac.DraftPolicies > 0 {
		fmt.Fprintf(&sb, "- %d Richtlinien benötigen Review\n", ac.DraftPolicies)
	}

	sb.WriteString(`
Erstelle eine priorisierte Liste der 5 wichtigsten Maßnahmen für diese Woche.
Format: Nummerierte Liste, pro Punkt: Maßnahme + kurze Begründung (1 Satz).
Antworte nur mit der Liste, kein weiterer Text.`)

	return sb.String()
}

// ComplianceAdvice analyzes the org's current compliance state and returns
// a prioritized action plan for the current week. It collects compact data
// from the DB, builds a short prompt, and calls the LLM.
func (s *Service) ComplianceAdvice(ctx context.Context, orgID string) (string, error) {
	ac, err := GatherAdviceContext(ctx, s.db, orgID)
	if err != nil {
		return "", fmt.Errorf("gather advice context: %w", err)
	}

	system := "Du bist ein ISO-27001/NIS2-Compliance-Berater. Antworte auf Deutsch, präzise und handlungsorientiert."
	userPrompt := buildAdvicePrompt(ac)

	// Use a dedicated request that includes a system message for better quality
	// on small models, while keeping max_tokens low to stay fast on CPU.
	return s.client.GenerateWithSystem(ctx, system, userPrompt)
}

func (s *Service) GenerateReport(ctx context.Context, orgID string, reportType ReportType) (string, error) {
	cc, err := GatherContext(ctx, s.db, orgID)
	if err != nil {
		return "", fmt.Errorf("gather context: %w", err)
	}

	var prompt string
	switch reportType {
	case ReportGapAnalysis:
		prompt = buildGapAnalysisPrompt(cc)
	case ReportRiskSummary:
		prompt = buildRiskSummaryPrompt(cc)
	case ReportExecutiveSummary:
		prompt = buildExecutiveSummaryPrompt(cc)
	default:
		return "", fmt.Errorf("unknown report type: %s", reportType)
	}

	return s.client.Generate(ctx, prompt)
}

func buildGapAnalysisPrompt(cc *ComplianceContext) string {
	gaps := strings.Join(cc.TopGaps, "\n- ")
	if gaps == "" {
		gaps = "(keine offenen Lücken)"
	}
	frameworks := strings.Join(cc.ActiveFrameworks, ", ")

	return fmt.Sprintf(`Du bist ein erfahrener IT-Sicherheitsberater. Erstelle eine professionelle Gap-Analyse auf Deutsch für folgendes Unternehmen:

Organisation: %s
Aktive Frameworks: %s
Gesamtscore: %d%%
Implementierte Controls: %d von %d
In Bearbeitung: %d
Fehlende Controls: %d
Offene Sicherheitslücken: %d
Kritische Risiken: %d
Offene Vorfälle: %d
Erstellt am: %s

Wichtigste fehlende Controls:
- %s

Schreibe eine strukturierte Gap-Analyse mit:
1. Management-Zusammenfassung (2-3 Sätze)
2. Aktuelle Compliance-Bewertung
3. Kritische Handlungsfelder (priorisiert)
4. Konkrete Empfehlungen für die nächsten 3 Monate
5. Risikoeinschätzung

Antworte ausschließlich auf Deutsch. Verwende professionelle aber verständliche Sprache für IT-Führungskräfte.`,
		cc.OrgName, frameworks, cc.OverallScore,
		cc.Implemented, cc.TotalControls, cc.InProgress, cc.Missing,
		cc.OpenFindings, cc.CriticalRisks, cc.OpenIncidents,
		cc.GeneratedAt.Format("02.01.2006"),
		gaps,
	)
}

func buildRiskSummaryPrompt(cc *ComplianceContext) string {
	risks := strings.Join(cc.TopRisks, "\n- ")
	if risks == "" {
		risks = "(keine kritischen Risiken)"
	}
	return fmt.Sprintf(`Du bist ein erfahrener IT-Sicherheitsberater. Erstelle eine Risikoanalyse auf Deutsch:

Organisation: %s
Kritische und hohe Risiken: %d
Offene Vorfälle: %d
Compliance-Score: %d%%
Offene Sicherheitslücken: %d

Wichtigste Risiken:
- %s

Erstelle eine strukturierte Risikoübersicht mit:
1. Risikoprofil (kurze Bewertung)
2. Top-Risiken im Detail mit Behandlungsempfehlung
3. Sofortmaßnahmen
4. Mittelfristige Risikominderung

Antworte ausschließlich auf Deutsch.`,
		cc.OrgName, cc.CriticalRisks, cc.OpenIncidents, cc.OverallScore, cc.OpenFindings, risks,
	)
}

func buildExecutiveSummaryPrompt(cc *ComplianceContext) string {
	frameworks := strings.Join(cc.ActiveFrameworks, ", ")
	return fmt.Sprintf(`Du bist ein erfahrener IT-Sicherheitsberater. Erstelle eine Executive Summary auf Deutsch für das Top-Management:

Organisation: %s
Datum: %s
Aktive Compliance-Frameworks: %s
Gesamter Compliance-Score: %d%%
Implementierte Controls: %d von %d
Offene Sicherheitslücken: %d
Kritische Risiken: %d
Offene Vorfälle: %d

Schreibe eine prägnante Executive Summary (max. 300 Wörter) mit:
1. Aktuelle Sicherheitslage (1-2 Sätze)
2. Wichtigste Zahlen im Kontext
3. Dringendste Handlungsbedarfe (Top 3)
4. Positive Entwicklungen / Stärken
5. Empfehlung für das Management

Sprache: Deutsch, nicht-technisch, für Geschäftsführung geeignet.`,
		cc.OrgName, cc.GeneratedAt.Format("02.01.2006"),
		frameworks, cc.OverallScore, cc.Implemented, cc.TotalControls,
		cc.OpenFindings, cc.CriticalRisks, cc.OpenIncidents,
	)
}
