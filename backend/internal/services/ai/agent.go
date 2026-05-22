package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// Sprint 18 / S18-1 + S18-2: Plan/Execute/Reflect-Agent-Loop.
//
// Konzept: anstelle einer One-Shot-Prompt-Generation läuft eine Schleife
//   1. Plan        — LLM erstellt einen Schritt-Plan auf Basis von Goal + Tool-Liste
//   2. Tool-Call   — LLM wählt einen Tool-Call mit Argumenten, der Helper führt aus
//   3. Reflect     — LLM bewertet Tool-Output und entscheidet weiter (next-tool/done)
// MaxIterations stoppt die Schleife defensiv.
//
// Jeder Tool-Call wird durch eine RBAC-Prüfung gestützt (siehe Tool.RequireScope
// + AgentRunner.Permissions): der Agent darf NUR Tools nutzen, die der
// initiierende User selbst nutzen kann (ADR-0020).
//
// Streaming: jeder Step wird über den callback `OnEvent` an den Aufrufer
// gemeldet. Der SSE-Handler (agent_handler.go, S18-3) mapped die Events auf
// SSE-Frames.

// AgentEventType klassifiziert ein Stream-Event.
type AgentEventType string

const (
	AgentEventPlan     AgentEventType = "plan"
	AgentEventToolCall AgentEventType = "tool_call"
	AgentEventToolResult AgentEventType = "tool_result"
	AgentEventReflect  AgentEventType = "reflect"
	AgentEventFinal    AgentEventType = "final"
	AgentEventError    AgentEventType = "error"
)

// AgentEvent ist ein einzelnes Stream-Frame über den Agent-Lauf.
type AgentEvent struct {
	Type      AgentEventType `json:"type"`
	Step      int            `json:"step"`
	Message   string         `json:"message,omitempty"`
	Tool      string         `json:"tool,omitempty"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
	Timestamp time.Time      `json:"ts"`
}

// AgentRunRequest beschreibt einen Agent-Lauf.
type AgentRunRequest struct {
	Goal          string   `json:"goal"`
	ContextHints  []string `json:"context_hints"`
	MaxIterations int      `json:"max_iterations"`
	OrgID         string   `json:"org_id"`
	UserID        string   `json:"user_id"`
	// Permissions ist die Liste der Permission-Strings des initiierenden Users.
	// Der Agent darf nur Tools aufrufen, deren RequireScope hier enthalten ist.
	Permissions []string `json:"permissions"`
}

// AgentTool ist die Schnittstelle für ein Tool, das der Agent aufrufen kann.
// Jedes Tool muss seine RBAC-Anforderung deklarieren.
type AgentTool interface {
	Name() string
	Description() string
	// JSONSchema des Arguments-Objekts. Wird dem LLM als Tool-Definition gegeben.
	ArgumentsSchema() json.RawMessage
	// RequireScope wird gegen die User-Permissions geprüft, bevor Execute läuft.
	// Leer = Read-Only-Tool ohne Scope-Restriktion.
	RequireScope() string
	// Execute wird mit den vom LLM gewählten Args aufgerufen. Returnt JSON-
	// Result, das in den Reflect-Step zurückfließt.
	Execute(ctx context.Context, orgID string, args json.RawMessage) (json.RawMessage, error)
}

// AgentRunner orchestriert den Plan/Execute/Reflect-Loop.
type AgentRunner struct {
	client *AIClient
	model  string
	tools  map[string]AgentTool
	db     *pgxpool.Pool
	usage  *UsageTracker
}

// NewAgentRunner baut einen Runner mit den registrierten Tools.
func NewAgentRunner(client *AIClient, model string, db *pgxpool.Pool, usage *UsageTracker, tools []AgentTool) *AgentRunner {
	m := make(map[string]AgentTool, len(tools))
	for _, t := range tools {
		m[t.Name()] = t
	}
	return &AgentRunner{client: client, model: model, tools: m, db: db, usage: usage}
}

// Run führt den Loop aus und ruft onEvent für jedes Event auf. Returnt
// nichts; Fehler werden als AgentEventError-Event gesendet.
//
// Skeleton-Implementierung Sprint 18: Plan + Tool-Loop ist verdrahtet, aber
// der eigentliche LLM-Call läuft One-Shot statt mit echtem Function-Calling.
// Vollständige Function-Calling-Integration (mit JSON-Mode oder tools[]
// im OpenAI-Schema) wandert in Sprint 19 — der Skeleton beweist das Pattern
// und liefert produktiven Audit-Trail.
func (r *AgentRunner) Run(ctx context.Context, req AgentRunRequest, onEvent func(AgentEvent)) {
	if req.MaxIterations <= 0 || req.MaxIterations > 10 {
		req.MaxIterations = 5
	}

	// Rate-Limit + Quota wie bei AI-Chat-Stream.
	if r.usage != nil {
		if err := r.usage.CheckRateLimit(ctx, req.OrgID); err != nil {
			onEvent(AgentEvent{Type: AgentEventError, Message: err.Error(), Timestamp: time.Now().UTC()})
			return
		}
		if err := r.usage.CheckDailyQuota(ctx, req.OrgID); err != nil {
			onEvent(AgentEvent{Type: AgentEventError, Message: err.Error(), Timestamp: time.Now().UTC()})
			return
		}
	}

	// Phase 1: Plan generieren.
	toolList := r.toolListPrompt(req.Permissions)
	system := "Du bist ein Compliance-Agent. Du erhältst ein Ziel und eine Werkzeug-Liste. " +
		"Erstelle einen kurzen Plan (max. 5 Schritte) wie das Ziel erreicht wird. " +
		"Antworte mit nummerierten Schritten auf Deutsch, jeden Schritt einzeilig."
	userPrompt := fmt.Sprintf("Ziel: %s\n\nKontext: %s\n\nVerfügbare Tools:\n%s",
		req.Goal, jsonString(req.ContextHints), toolList,
	)
	plan, err := r.client.GenerateWithSystem(ctx, system, userPrompt)
	if err != nil {
		onEvent(AgentEvent{Type: AgentEventError, Message: "plan generation failed: " + err.Error(), Timestamp: time.Now().UTC()})
		return
	}
	onEvent(AgentEvent{Type: AgentEventPlan, Step: 0, Message: plan, Timestamp: time.Now().UTC()})

	// Audit-Trail-Eintrag für den Agent-Run-Start. Sprint 18 S18-5 vertieft
	// dies pro Tool-Call.
	r.auditAgentStart(ctx, req, plan)

	// Phase 2: Tool-Use-Loop (Skeleton). Ein vollständiges Function-Calling-
	// Roundtrip kommt in einer Folge-Welle; der aktuelle Loop ruft jedes
	// Tool einmal, falls es read-only ist + im Plan referenziert wird.
	for step := 1; step <= req.MaxIterations; step++ {
		// Heuristik: parse Plan auf Tool-Namen-Referenzen.
		// (Echte Implementierung: LLM-Call mit Tool-Definitions, JSON-Mode-Antwort.)
		toolName := r.nextToolFromPlan(plan, step)
		if toolName == "" {
			break
		}
		tool, ok := r.tools[toolName]
		if !ok {
			continue
		}

		// RBAC-Check (ADR-0020): Agent darf NUR Tools nutzen, die User darf.
		if scope := tool.RequireScope(); scope != "" && !hasScope(req.Permissions, scope) {
			onEvent(AgentEvent{
				Type: AgentEventError, Step: step,
				Message: fmt.Sprintf("tool %q requires scope %q which the user lacks", toolName, scope),
				Tool:    toolName,
				Timestamp: time.Now().UTC(),
			})
			continue
		}

		// Tool ausführen mit leerem Args-Objekt (Skeleton: echte Args kommen
		// vom LLM via Function-Calling in Folge-Welle).
		onEvent(AgentEvent{Type: AgentEventToolCall, Step: step, Tool: toolName, Arguments: json.RawMessage(`{}`), Timestamp: time.Now().UTC()})
		result, execErr := tool.Execute(ctx, req.OrgID, json.RawMessage(`{}`))
		if execErr != nil {
			onEvent(AgentEvent{Type: AgentEventError, Step: step, Tool: toolName, Message: execErr.Error(), Timestamp: time.Now().UTC()})
			continue
		}
		onEvent(AgentEvent{Type: AgentEventToolResult, Step: step, Tool: toolName, Result: result, Timestamp: time.Now().UTC()})
	}

	// Phase 3: Final Reflection.
	onEvent(AgentEvent{Type: AgentEventFinal, Message: "Agent-Lauf abgeschlossen.", Timestamp: time.Now().UTC()})
}

// toolListPrompt formatiert die Tools für den Plan-Prompt. Nur Tools, deren
// RequireScope zur User-Permission passt, werden aufgelistet.
func (r *AgentRunner) toolListPrompt(perms []string) string {
	out := ""
	for _, t := range r.tools {
		if scope := t.RequireScope(); scope != "" && !hasScope(perms, scope) {
			continue
		}
		out += fmt.Sprintf("- %s: %s\n", t.Name(), t.Description())
	}
	return out
}

// nextToolFromPlan ist eine Heuristik die nach Tool-Namen im Plan sucht.
// Skeleton-Implementierung; vollständige LLM-Function-Calling-Integration
// kommt in einer Folge-Welle.
func (r *AgentRunner) nextToolFromPlan(plan string, step int) string {
	// Sehr einfache Implementierung: gib das step-te Tool aus der registrierten
	// Liste zurück, wenn sein Name im Plan vorkommt. Sonst leer = Loop-Stop.
	idx := 0
	for name := range r.tools {
		idx++
		if idx == step && contains(plan, name) {
			return name
		}
	}
	return ""
}

// hasScope prüft, ob die User-Permissions den geforderten Scope abdecken.
// Wildcards (`*`, `<modul>.*`) sind erlaubt.
func hasScope(perms []string, required string) bool {
	for _, p := range perms {
		if p == "*" || p == required {
			return true
		}
		// Wildcard: secvitals.*
		if len(p) > 2 && p[len(p)-1] == '*' && p[len(p)-2] == '.' {
			prefix := p[:len(p)-1]
			if len(required) >= len(prefix) && required[:len(prefix)] == prefix {
				return true
			}
		}
	}
	return false
}

func contains(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// jsonString rendert einen []string als JSON-Array für den Prompt.
func jsonString(v []string) string {
	if len(v) == 0 {
		return "(keiner)"
	}
	b, _ := json.Marshal(v)
	return string(b)
}

// auditAgentStart schreibt einen Audit-Log-Eintrag für den Agent-Run-Start.
// Sprint 18 S18-5: jeder Agent-Run hinterlässt im audit_log einen actor=
// ai_agent-Entry mit goal-Excerpt. Vollständige Tool-Call-Audit-Vertiefung
// in der Tool-Implementierung (jeder Tool.Execute schreibt seinen eigenen
// Eintrag via audit.Write).
func (r *AgentRunner) auditAgentStart(ctx context.Context, req AgentRunRequest, plan string) {
	if r.db == nil || req.OrgID == "" {
		return
	}
	goalExcerpt := req.Goal
	if len(goalExcerpt) > 200 {
		goalExcerpt = goalExcerpt[:200] + "…"
	}
	planExcerpt := plan
	if len(planExcerpt) > 500 {
		planExcerpt = planExcerpt[:500] + "…"
	}
	details, _ := json.Marshal(map[string]string{
		"goal":         goalExcerpt,
		"plan_excerpt": planExcerpt,
		"actor":        "ai_agent",
	})
	if _, err := r.db.Exec(ctx, `
		INSERT INTO audit_log
		  (org_id, user_id, user_email, action, resource_type, resource_id, details, ip_address)
		VALUES ($1::uuid, NULLIF($2, '')::uuid, 'ai_agent', 'agent_run_start', 'ai/agent', NULL, $3, NULL)`,
		req.OrgID, req.UserID, details,
	); err != nil {
		log.Warn().Err(err).Str("org_id", req.OrgID).Msg("ai.agent: audit start write failed")
	}
}

// ErrToolNotAllowed wird von Tool.Execute zurückgegeben, wenn der Tool-Run
// trotz initialem Scope-Check verweigert wird (z.B. Org-spezifische
// Feature-Toggles).
var ErrToolNotAllowed = errors.New("ai.agent: tool not allowed for this org or user")
