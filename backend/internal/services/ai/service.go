package ai

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/shared/httputil"
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

	if err := db.QueryRow(ctx, `SELECT name FROM organizations WHERE id = $1::uuid`, orgID).Scan(&cc.OrgName); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("ai: could not resolve org name")
	}

	// Control statistics
	if err := db.QueryRow(ctx, `
        SELECT
            COUNT(*) FILTER (WHERE status = 'implemented'),
            COUNT(*) FILTER (WHERE status = 'in_progress'),
            COUNT(*) FILTER (WHERE status = 'missing'),
            COUNT(*)
        FROM ck_controls
        WHERE org_id = $1::uuid AND status != 'not_applicable'`, orgID,
	).Scan(&cc.Implemented, &cc.InProgress, &cc.Missing, &cc.TotalControls); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("ai: could not gather control statistics")
	}

	if cc.TotalControls > 0 {
		cc.OverallScore = (cc.Implemented * 100) / cc.TotalControls
	}

	// Open findings
	if err := db.QueryRow(ctx, `
        SELECT COUNT(*) FROM vb_findings
        WHERE org_id = $1::uuid AND status NOT IN ('resolved', 'false_positive')`, orgID,
	).Scan(&cc.OpenFindings); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("ai: could not count open findings")
	}

	// Critical risks
	if err := db.QueryRow(ctx, `
        SELECT COUNT(*) FROM ck_risks
        WHERE org_id = $1::uuid AND status NOT IN ('accepted','closed','mitigated')
          AND likelihood * impact >= 15`, orgID,
	).Scan(&cc.CriticalRisks); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("ai: could not count critical risks")
	}

	// Open incidents
	if err := db.QueryRow(ctx, `
        SELECT COUNT(*) FROM ck_incidents
        WHERE org_id = $1::uuid AND status NOT IN ('resolved','closed')`, orgID,
	).Scan(&cc.OpenIncidents); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("ai: could not count open incidents")
	}

	// Active frameworks
	// ck_frameworks has no is_active column; a row exists per org for each framework
	// the org has enabled, so the org's frameworks are its active ones.
	rows, err := db.Query(ctx, `SELECT name FROM ck_frameworks WHERE org_id = $1::uuid ORDER BY name`, orgID)
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

// OrgSettings holds per-org AI configuration overrides.
// Empty strings mean "use system default". Wire a resolver via WithOrgSettings.
type OrgSettings struct {
	ModelOverride   string
	BaseURLOverride string
}

// OrgSettingsFunc resolves per-org AI settings from persistent storage.
type OrgSettingsFunc func(ctx context.Context, orgID string) (OrgSettings, error)

type Service struct {
	db      *pgxpool.Pool
	client  *AIClient
	baseURL string
	apiKey  string
	model   string
	// Sprint 15 / S15-1/2/3: optional. Wenn gesetzt, läuft jede AI-Anfrage durch
	// Rate-Limit + Daily-Quota + Response-Cache und schreibt einen Usage-Record.
	// Wenn nil: alte Semantik (unbeschränkt, kein Tracking).
	usage *UsageTracker
	// orgSettings is an optional function to look up per-org model/URL overrides (S79-7).
	orgSettings OrgSettingsFunc
}

func NewService(db *pgxpool.Pool, baseURL, apiKey, model string) *Service {
	return &Service{
		db:      db,
		client:  NewAIClient(baseURL, apiKey, model),
		baseURL: baseURL,
		apiKey:  apiKey,
		model:   model,
	}
}

// WithUsageTracker rüstet einen Service mit Rate-Limit, Quota und Cache nach.
// Beim Wiring in cmd/api/main.go aufrufen, sobald rdb verfügbar ist.
func (s *Service) WithUsageTracker(t *UsageTracker) *Service {
	s.usage = t
	return s
}

// WithOrgSettings attaches a per-org settings resolver (S79-7).
// When set, gateAndGenerate will use per-org model/URL overrides if present.
func (s *Service) WithOrgSettings(fn OrgSettingsFunc) *Service {
	s.orgSettings = fn
	return s
}

func (s *Service) IsAvailable(ctx context.Context) bool {
	return s.client.IsAvailable(ctx)
}

// gateAndGenerate ist die zentrale Schleife: Rate-Limit + Quota + Cache + Call
// + Persist. Aufrufer muss orgID und tag (z.B. "advice", "report") angeben.
// system kann leer sein — dann läuft Generate() ohne System-Message.
func (s *Service) gateAndGenerate(ctx context.Context, orgID, tag, system, userPrompt string) (string, error) {
	if s.usage != nil && orgID != "" {
		if err := s.usage.CheckRateLimit(ctx, orgID); err != nil {
			s.usage.Record(ctx, UsageRecord{OrgID: orgID, Model: s.model, Status: "rate_limited", RequestID: tag})
			return "", err
		}
		if err := s.usage.CheckDailyQuota(ctx, orgID); err != nil {
			s.usage.Record(ctx, UsageRecord{OrgID: orgID, Model: s.model, Status: "rate_limited", RequestID: tag})
			return "", err
		}
	}

	// Resolve effective model/URL — per-org overrides take precedence (S79-7).
	// Security: BaseURLOverride is SSRF-validated; the system API key is NEVER
	// forwarded to an org-supplied URL (use empty key — local Ollama needs none,
	// and the org configures its own key out-of-band if pointing at a cloud provider).
	effectiveModel := s.model
	effectiveClient := s.client
	if s.orgSettings != nil && orgID != "" {
		if orgCfg, cfgErr := s.orgSettings(ctx, orgID); cfgErr == nil {
			if orgCfg.ModelOverride != "" || orgCfg.BaseURLOverride != "" {
				baseURL := s.baseURL
				apiKey := s.apiKey
				if orgCfg.BaseURLOverride != "" {
					// Validate SSRF: reject loopback/RFC1918/IMDS/link-local targets.
					// allowPrivate=false — org-supplied URLs must be publicly reachable.
					if validErr := httputil.ValidateOutboundURL(orgCfg.BaseURLOverride, false); validErr != nil {
						log.Warn().Err(validErr).Str("org_id", orgID).
							Str("base_url_override", orgCfg.BaseURLOverride).
							Msg("ai: rejecting org base_url_override — SSRF validation failed")
					} else {
						baseURL = orgCfg.BaseURLOverride
						// Do not forward the system API key to a third-party URL.
						// The org's endpoint is expected to accept requests without auth
						// (self-hosted Ollama/LM Studio) or the org supplies its own key
						// via the URL itself (not yet a stored credential).
						apiKey = ""
					}
				}
				if orgCfg.ModelOverride != "" {
					effectiveModel = orgCfg.ModelOverride
				}
				effectiveClient = NewAIClient(baseURL, apiKey, effectiveModel)
			}
		}
	}

	// Cache-Lookup
	msgs := buildMessages(system, userPrompt)
	cacheKey := CacheKey(effectiveModel, msgs)
	if s.usage != nil {
		if cached, ok := s.usage.CacheGet(ctx, cacheKey); ok {
			s.usage.Record(ctx, UsageRecord{OrgID: orgID, Model: effectiveModel, Status: "cache_hit", RequestID: tag})
			return cached, nil
		}
	}

	// Call upstream.
	start := time.Now()
	var result ChatResult
	var err error
	if system != "" {
		result, err = effectiveClient.GenerateWithSystemFull(ctx, system, userPrompt)
	} else {
		text, genErr := effectiveClient.Generate(ctx, userPrompt)
		result = ChatResult{Text: text}
		err = genErr
	}
	dur := int(time.Since(start).Milliseconds())
	status := "ok"
	if err != nil {
		status = "provider_error"
	}
	if s.usage != nil {
		s.usage.Record(ctx, UsageRecord{
			OrgID: orgID, Model: effectiveModel,
			DurationMs: dur, Status: status, RequestID: tag,
			TokensIn: result.TokensIn, TokensOut: result.TokensOut,
		})
		if err == nil && result.Text != "" {
			s.usage.CacheSet(ctx, cacheKey, result.Text)
		}
	}
	return result.Text, err
}

// buildMessages erstellt die Message-Liste mit strikter Role-Trennung —
// User-Input landet NIE im System-Prompt, immer im user-Role-Message
// (Prompt-Injection-Defense, ADR-Anmerkung S15-4).
func buildMessages(system, userPrompt string) []chatMessage {
	if system == "" {
		return []chatMessage{{Role: "user", Content: userPrompt}}
	}
	return []chatMessage{
		{Role: "system", Content: system},
		{Role: "user", Content: userPrompt},
	}
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
	if err := db.QueryRow(ctx, `SELECT name FROM organizations WHERE id = $1::uuid`, orgID).Scan(&ac.OrgName); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("ai advice: could not resolve org name")
	}
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
	if err := db.QueryRow(ctx,
		`SELECT COUNT(*)::int FROM ck_capas WHERE org_id=$1::uuid AND status != 'closed'`,
		orgID).Scan(&ac.OpenCAPAs); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("ai advice: could not count open CAPAs")
	}

	// Overdue controls
	if err := db.QueryRow(ctx,
		`SELECT COUNT(*)::int FROM ck_controls
		 WHERE org_id=$1::uuid AND next_review_due IS NOT NULL AND next_review_due < NOW()`,
		orgID).Scan(&ac.OverdueControls); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("ai advice: could not count overdue controls")
	}

	// Overdue tasks
	if err := db.QueryRow(ctx,
		`SELECT COUNT(*)::int FROM ck_tasks
		 WHERE org_id=$1::uuid AND due_date IS NOT NULL AND due_date < NOW() AND status != 'done'`,
		orgID).Scan(&ac.OverdueTasks); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("ai advice: could not count overdue tasks")
	}

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
	if err := db.QueryRow(ctx,
		`SELECT COUNT(*)::int FROM ck_incidents
		 WHERE org_id=$1::uuid AND status NOT IN ('resolved','closed')`,
		orgID).Scan(&ac.OpenIncidents); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("ai advice: could not count open incidents")
	}

	// Policies in draft or with no version (need review)
	if err := db.QueryRow(ctx,
		`SELECT COUNT(*)::int FROM ck_policies
		 WHERE org_id=$1::uuid AND (status = 'draft' OR version IS NULL OR version = '')`,
		orgID).Scan(&ac.DraftPolicies); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("ai advice: could not count draft policies")
	}

	return ac, nil
}

func buildAdvicePrompt(ac *AdviceContext) string {
	var sb strings.Builder

	sb.WriteString("Compliance-Status für ")
	sb.WriteString(wrapUserContent(ac.OrgName))
	sb.WriteString(":\n\n")

	if len(ac.FrameworkScores) > 0 {
		sb.WriteString("Frameworks:\n")
		for _, fs := range ac.FrameworkScores {
			pct := 0
			if fs.Total > 0 {
				pct = (fs.Implemented * 100) / fs.Total
			}
			fmt.Fprintf(&sb, "- %s: %d/%d Controls implementiert (%d%%)\n",
				sanitizeUserInput(fs.Name), fs.Implemented, fs.Total, pct)
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Offene Probleme:\n")
	if len(ac.CriticalRisks) > 0 {
		fmt.Fprintf(&sb, "- %d kritische Risiken: ", len(ac.CriticalRisks))
		sanitized := make([]string, len(ac.CriticalRisks))
		for i, r := range ac.CriticalRisks {
			sanitized[i] = wrapUserContent(r)
		}
		sb.WriteString(strings.Join(sanitized, ", "))
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

	system := addInjectionGuard("Du bist ein ISO-27001/NIS2-Compliance-Berater. Antworte auf Deutsch, präzise und handlungsorientiert.")
	userPrompt := buildAdvicePrompt(ac)

	// Sprint 15: gateAndGenerate hängt Rate-Limit + Quota + Cache vor den Call.
	return s.gateAndGenerate(ctx, orgID, "advice", system, userPrompt)
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

	system := addInjectionGuard("Du bist ein erfahrener IT-Sicherheitsberater für DACH-Unternehmen. Antworte ausschließlich auf Deutsch.")
	return s.gateAndGenerate(ctx, orgID, "report-"+string(reportType), system, prompt)
}

func buildGapAnalysisPrompt(cc *ComplianceContext) string {
	sanitizedGaps := make([]string, len(cc.TopGaps))
	for i, g := range cc.TopGaps {
		sanitizedGaps[i] = wrapUserContent(g)
	}
	gaps := strings.Join(sanitizedGaps, "\n- ")
	if gaps == "" {
		gaps = "(keine offenen Lücken)"
	}
	sanitizedFrameworks := make([]string, len(cc.ActiveFrameworks))
	for i, f := range cc.ActiveFrameworks {
		sanitizedFrameworks[i] = sanitizeUserInput(f)
	}
	frameworks := strings.Join(sanitizedFrameworks, ", ")

	return fmt.Sprintf(`Erstelle eine professionelle Gap-Analyse auf Deutsch für folgendes Unternehmen:

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
		wrapUserContent(cc.OrgName), frameworks, cc.OverallScore,
		cc.Implemented, cc.TotalControls, cc.InProgress, cc.Missing,
		cc.OpenFindings, cc.CriticalRisks, cc.OpenIncidents,
		cc.GeneratedAt.Format("02.01.2006"),
		gaps,
	)
}

func buildRiskSummaryPrompt(cc *ComplianceContext) string {
	sanitizedRisks := make([]string, len(cc.TopRisks))
	for i, r := range cc.TopRisks {
		sanitizedRisks[i] = wrapUserContent(r)
	}
	risks := strings.Join(sanitizedRisks, "\n- ")
	if risks == "" {
		risks = "(keine kritischen Risiken)"
	}
	return fmt.Sprintf(`Erstelle eine Risikoanalyse auf Deutsch:

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
		wrapUserContent(cc.OrgName), cc.CriticalRisks, cc.OpenIncidents, cc.OverallScore, cc.OpenFindings, risks,
	)
}

func buildExecutiveSummaryPrompt(cc *ComplianceContext) string {
	sanitizedFrameworks := make([]string, len(cc.ActiveFrameworks))
	for i, f := range cc.ActiveFrameworks {
		sanitizedFrameworks[i] = sanitizeUserInput(f)
	}
	frameworks := strings.Join(sanitizedFrameworks, ", ")
	return fmt.Sprintf(`Erstelle eine Executive Summary auf Deutsch für das Top-Management:

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
		wrapUserContent(cc.OrgName), cc.GeneratedAt.Format("02.01.2006"),
		frameworks, cc.OverallScore, cc.Implemented, cc.TotalControls,
		cc.OpenFindings, cc.CriticalRisks, cc.OpenIncidents,
	)
}
