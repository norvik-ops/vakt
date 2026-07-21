package ai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Sprint 18 S18-2 + S18-6: konkrete Tools für den AgentRunner.
//
// Drei initiale Tools verdrahtet:
//   - list_open_findings: lädt offene SecPulse-Findings für Triage-Workflow
//   - list_stale_evidence: lädt SecVitals-Evidence, die in <30 Tagen abläuft
//   - list_controls_without_evidence: SecVitals-Controls ohne aktiven Evidence-Eintrag
//
// S32-2: add_control_note als erstes Write-Tool — erfordert ApproveCard-Freigabe.

// listOpenFindingsTool: SecPulse-Findings mit status='open'. Read-Only.
type listOpenFindingsTool struct {
	db *pgxpool.Pool
}

func (t *listOpenFindingsTool) Name() string { return "list_open_findings" }
func (t *listOpenFindingsTool) Description() string {
	return "Liefert die 20 ältesten offenen Findings aus Vakt Scan (Trivy/Nuclei/OpenVAS). Nutze für Triage-Workflows."
}
func (t *listOpenFindingsTool) ArgumentsSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"severity":{"type":"string","enum":["critical","high","medium","low"]}}}`)
}
func (t *listOpenFindingsTool) RequireScope() string { return "vaktscan.findings.read" }
func (t *listOpenFindingsTool) IsWriteTool() bool    { return false }
func (t *listOpenFindingsTool) Execute(ctx context.Context, orgID string, _ json.RawMessage) (json.RawMessage, error) {
	rows, err := t.db.Query(ctx, `
		SELECT id::text, title, severity, created_at
		FROM vb_findings
		WHERE org_id = $1::uuid AND status = 'open'
		ORDER BY created_at ASC
		LIMIT 20`, orgID)
	if err != nil {
		return nil, fmt.Errorf("query open findings: %w", err)
	}
	defer rows.Close()
	type row struct {
		ID        string `json:"id"`
		Title     string `json:"title"`
		Severity  string `json:"severity"`
		CreatedAt string `json:"created_at"`
	}
	var out []row
	for rows.Next() {
		var r row
		var ts pgtype.Timestamptz
		if err := rows.Scan(&r.ID, &r.Title, &r.Severity, &ts); err == nil {
			r.CreatedAt = ts.Time.UTC().Format("2006-01-02T15:04:05Z")
			out = append(out, r)
		}
	}
	body, _ := json.Marshal(out)
	return body, nil
}

// listStaleEvidenceTool: SecVitals-Evidence-Einträge, die in 30 Tagen ablaufen.
type listStaleEvidenceTool struct {
	db *pgxpool.Pool
}

func (t *listStaleEvidenceTool) Name() string { return "list_stale_evidence" }
func (t *listStaleEvidenceTool) Description() string {
	return "Liefert Evidence-Einträge aus Vakt Comply, die in 30 Tagen ablaufen. Nutze für Evidence-Re-Collection-Plans."
}
func (t *listStaleEvidenceTool) ArgumentsSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{}}`)
}
func (t *listStaleEvidenceTool) RequireScope() string { return "vaktcomply.evidence.read" }
func (t *listStaleEvidenceTool) IsWriteTool() bool    { return false }
func (t *listStaleEvidenceTool) Execute(ctx context.Context, orgID string, _ json.RawMessage) (json.RawMessage, error) {
	rows, err := t.db.Query(ctx, `
		SELECT e.id::text, e.title, e.expires_at
		FROM ck_evidence e
		WHERE e.org_id = $1::uuid
		  AND e.expires_at IS NOT NULL
		  AND e.expires_at > NOW()
		  AND e.expires_at < NOW() + INTERVAL '30 days'
		ORDER BY e.expires_at ASC
		LIMIT 20`, orgID)
	if err != nil {
		return nil, fmt.Errorf("query stale evidence: %w", err)
	}
	defer rows.Close()
	type row struct {
		ID        string `json:"id"`
		Title     string `json:"title"`
		ExpiresAt string `json:"expires_at"`
	}
	var out []row
	for rows.Next() {
		var r row
		var ts pgtype.Timestamptz
		if err := rows.Scan(&r.ID, &r.Title, &ts); err == nil {
			r.ExpiresAt = ts.Time.UTC().Format("2006-01-02T15:04:05Z")
			out = append(out, r)
		}
	}
	body, _ := json.Marshal(out)
	return body, nil
}

// listControlsWithoutEvidenceTool: Controls ohne aktiven Evidence-Eintrag.
type listControlsWithoutEvidenceTool struct {
	db *pgxpool.Pool
}

func (t *listControlsWithoutEvidenceTool) Name() string {
	return "list_controls_without_evidence"
}
func (t *listControlsWithoutEvidenceTool) Description() string {
	return "Liefert Controls aus Vakt Comply, denen aktuell keine Evidence anhängt. Nutze für Compliance-Plan-Workflows."
}
func (t *listControlsWithoutEvidenceTool) ArgumentsSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{}}`)
}
func (t *listControlsWithoutEvidenceTool) RequireScope() string {
	return "vaktcomply.controls.read"
}
func (t *listControlsWithoutEvidenceTool) IsWriteTool() bool { return false }
func (t *listControlsWithoutEvidenceTool) Execute(ctx context.Context, orgID string, _ json.RawMessage) (json.RawMessage, error) {
	rows, err := t.db.Query(ctx, `
		SELECT c.id::text, c.control_id, c.title
		FROM ck_controls c
		WHERE c.org_id = $1::uuid
		  AND NOT EXISTS (
		      SELECT 1 FROM ck_evidence e
		      WHERE e.control_id = c.id AND (e.expires_at IS NULL OR e.expires_at > NOW())
		  )
		LIMIT 20`, orgID)
	if err != nil {
		return nil, fmt.Errorf("query controls without evidence: %w", err)
	}
	defer rows.Close()
	type row struct {
		ID        string `json:"id"`
		ControlID string `json:"control_id"`
		Title     string `json:"title"`
	}
	var out []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.ID, &r.ControlID, &r.Title); err == nil {
			out = append(out, r)
		}
	}
	body, _ := json.Marshal(out)
	return body, nil
}

// addControlNoteTool: writes a note to a SecVitals control. Write-Tool — requires approval.
type addControlNoteTool struct {
	db *pgxpool.Pool
}

func (t *addControlNoteTool) Name() string { return "add_control_note" }
func (t *addControlNoteTool) Description() string {
	return "Fügt eine Agent-Notiz zu einem Vakt-Comply-Control hinzu. Erfordert Benutzer-Freigabe (Write-Tool)."
}
func (t *addControlNoteTool) ArgumentsSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","required":["control_id","note"],"properties":{"control_id":{"type":"string"},"note":{"type":"string"}}}`)
}
func (t *addControlNoteTool) RequireScope() string { return "vaktcomply.controls.write" }
func (t *addControlNoteTool) IsWriteTool() bool    { return true }
func (t *addControlNoteTool) Execute(ctx context.Context, orgID string, args json.RawMessage) (json.RawMessage, error) {
	var a struct {
		ControlID string `json:"control_id"`
		Note      string `json:"note"`
	}
	if err := json.Unmarshal(args, &a); err != nil || a.ControlID == "" || a.Note == "" {
		return nil, fmt.Errorf("control_id and note required")
	}
	_, err := t.db.Exec(ctx, `
		UPDATE ck_controls SET review_note = COALESCE(review_note, '') || $1
		WHERE id::text = $2 AND org_id = $3::uuid`,
		"\n[Agent] "+a.Note, a.ControlID, orgID)
	if err != nil {
		return nil, fmt.Errorf("update control note: %w", err)
	}
	result, _ := json.Marshal(map[string]string{"status": "ok", "control_id": a.ControlID})
	return result, nil
}

// DefaultAgentTools liefert die Tool-Liste, die im Standard-Wiring an den
// AgentRunner gegeben wird. Wer neue Tools registrieren will, erweitert
// diese Liste in ai/routes.go.
func DefaultAgentTools(db *pgxpool.Pool) []AgentTool {
	return []AgentTool{
		&listOpenFindingsTool{db: db},
		&listStaleEvidenceTool{db: db},
		&listControlsWithoutEvidenceTool{db: db},
		&addControlNoteTool{db: db},
	}
}
