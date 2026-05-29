# ADR-0020: AI-Agent-Tool-Permissions

**Status:** Accepted
**Datum:** 2026-05-22
**Entscheider:** Stefan (Maintainer)

## Kontext

Sprint 18 führt den AgentRunner ein (Plan/Execute/Reflect-Loop), der dem LLM erlaubt, Tools wie `list_open_findings`, `list_stale_evidence`, `list_controls_without_evidence` aufzurufen. Mit der Zeit kommen mutierende Tools hinzu (`update_finding_status`, `add_evidence`, `mark_control_implemented`).

Sobald der Agent mutieren darf, entsteht ein scharfes Berechtigungs-Problem: ein User mit `viewer`-Rolle (Read-Only) könnte einen Agent-Run starten, der unter der Haube ein `admin`-Tool aufruft. Das wäre eine klassische Privilege-Escalation via AI.

Vergleichbar mit anderen Plattformen:
- Vanta: AI-Agent darf nur Aktionen ausführen, die der initiierende User auch direkt machen könnte (analog GitHub Copilot Workspace).
- Anthropic Computer Use: gleiche Regel, plus explizite User-Approval pro Tool-Call.

Vakt braucht eine codifizierte Regel, BEVOR der erste mutierende Tool-Call hinzukommt.

## Entscheidung

**Der AI-Agent darf nur Tools aufrufen, die der initiierende User direkt aufrufen könnte.** Konkret:

1. Jedes `AgentTool` deklariert via `RequireScope() string` einen Permission-String (z.B. `vaktscan.findings.read`, `vaktcomply.controls.write`).
2. Beim Agent-Run wird die User-Permission-Liste aus dem Echo-Context geladen (gleicher Pfad wie `RequirePermission`-Middleware).
3. Der `AgentRunner` filtert die Tool-Liste, BEVOR sie dem LLM im Plan-Prompt präsentiert wird — der Agent „sieht" Tools, die er nicht nutzen darf, gar nicht erst.
4. Vor jedem `Tool.Execute` läuft ein zweiter Scope-Check, defensiv: wenn der LLM trotz Filter ein nicht-erlaubtes Tool wählt, blockt der Runner mit `ErrToolNotAllowed`.
5. **Keine Privilege-Escalation via Workflow:** wenn ein Workflow „X + Y + Z" benötigt und der User nur X und Y darf, schlägt der Workflow an Z fehl — der Agent meldet das im Stream als `AgentEventError`, der Workflow läuft NICHT halbfertig durch.
6. Mutierende Tools (`*.write`, `*.delete`) bekommen zusätzlich einen **`ApproveBeforeApply`-Flag**. Diese werden im Frontend als „Approve"-Card visualisiert; der Agent wartet auf User-Klick, bevor das Tool ausgeführt wird. Default-on für alle write-Tools, Default-off für read-Tools.

## Alternativen

- **Agent läuft als Service-Account mit eigener Berechtigung** — verworfen. Damit könnte ein `viewer`-User über den Agent Operationen anstoßen, die er selbst nicht darf. Audit-Trail wäre verfälscht („wer hat die Operation initiiert?").
- **Pre-Approval pro Tool-Call statt pro Scope** — verworfen für read-Tools (zu viel Friction), beibehalten für write-Tools (siehe Punkt 6).
- **Globaler Agent-Kill-Switch ohne Scope-Check** — verworfen. Das ist eine Org-Setting-Frage („Org X verbietet alle Agent-Runs"), keine Permission-Architektur.
- **OAuth-Style scoped tokens für Agent** — verworfen, weil unnötig komplex. Agent läuft im selben Request-Context wie der User, der ihn startet; das bestehende Permission-System reicht.

## Konsequenzen

### Positive

- Privilege-Escalation via AI ist strukturell ausgeschlossen.
- Audit-Trail bleibt sauber: jeder Agent-Tool-Call wird als `actor=ai_agent, initiator=<user_email>, agent_run_id=<uuid>` persistiert. Auditor kann pro User die Agent-Aktionen filtern.
- Frontend kann „verfügbare Tools" pro User korrekt anzeigen — was der Agent sehen kann, sieht auch der User in der Tool-Liste.
- Compliance-Story für Enterprise-Sales: „der Agent ist kein Superuser, sondern ein Co-Worker im RBAC-Rahmen des Users".

### Negative

- Tool-Implementierung wird leicht komplexer: jedes neue Tool muss `RequireScope()` explizit deklarieren. Verhindert auch, dass jemand ein Tool ohne RBAC-Markierung commits — golangci-Linter-Custom-Rule planen (`forbidigo` mit Pattern `RequireScope.*return ""`-Detector).
- Multi-Step-Workflows können an einer einzigen fehlenden Permission scheitern. Frontend muss klare Fehler-UX liefern: „Workflow stoppt — User braucht zusätzlich `vaktcomply.controls.write` für Schritt 3".

### Neutrale

- Aktuell sind alle drei DefaultAgentTools Read-Only mit `RequireScope()` aus `*.read`-Familie. Erst die zweite Welle (mutierende Tools) macht die Approve-Logik notwendig.
- ApproveBeforeApply-Flow benötigt eine Backend-Mechanik (Pending-Approval-Queue) — das ist eigene Implementation in einer Folge-Welle, nicht Sprint 18.

## Referenzen

- Sprint 18 Backlog: S18-1 bis S18-8
- `backend/internal/services/ai/agent.go` — AgentRunner mit Scope-Filter
- `backend/internal/services/ai/agent_tools.go` — drei initiale Tools mit RequireScope
- `backend/internal/services/ai/agent_handler.go` — SSE-Endpoint POST /ai/agent/run
- Verwandte ADRs:
  - [ADR-0014 — AI-Copilot Community](0014-ai-copilot-community-feature.md): definiert, dass AI als Community-Feature läuft. Agent-Runs werden gleich behandelt.
  - [ADR-0019 — SSE-Pattern](0019-sse-statt-websocket-fuer-realtime.md): Agent-Stream nutzt das gleiche Transport.
- Vergleich: GitHub Copilot Workspace Sandbox-Regel, Anthropic Computer Use Approval-Pattern.
