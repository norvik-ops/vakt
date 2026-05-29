package ai

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// Sprint 18 S18-3: AgentRunHandler ist der SSE-Endpoint für Agent-Runs.
//
// POST /api/v1/vaktcomply/ai/agent/run mit Body:
//   { "goal": "...", "context_hints": [...] }
//
// Response: text/event-stream mit Frames:
//   data: {"type":"run_started","message":"<run_id>"}
//   data: {"type":"plan","step":0,"message":"1. ..."}
//   data: {"type":"tool_call","step":1,"tool":"list_open_findings","arguments":{}}
//   data: {"type":"tool_result","step":1,"tool":"list_open_findings","result":[...]}
//   data: {"type":"approval_required","step":N,"tool":"add_control_note","arguments":{...}}
//   data: {"type":"final","message":"..."}
//   data: [DONE]
//
// S32-2: Approve/Reject-Endpoints für Write-Tool-Freigabe.
//   POST /api/v1/vaktcomply/ai/agent/runs/:run_id/approve
//   POST /api/v1/vaktcomply/ai/agent/runs/:run_id/reject
//
// Permissions kommen aus dem User-Context (org_id + user_id + perms). Tools
// werden nur ausgeführt, wenn der User die zugehörigen Scopes hat (ADR-0020).

// AgentHandler bündelt die Dependencies für Agent-Runs.
type AgentHandler struct {
	runner *AgentRunner
	runMgr *AgentRunManager
	db     *pgxpool.Pool
}

// NewAgentHandler baut einen Handler. tools wird über DefaultAgentTools(db)
// gespeist (siehe routes.go).
func NewAgentHandler(client *AIClient, model string, runner *AgentRunner, runMgr *AgentRunManager, db *pgxpool.Pool) *AgentHandler {
	return &AgentHandler{runner: runner, runMgr: runMgr, db: db}
}

// AgentRun ist der SSE-Endpoint-Handler.
func (h *AgentHandler) AgentRun(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	userID, _ := c.Get("user_id").(string)
	if orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	// Permissions aus Context. Wenn nichts gesetzt: leere Liste = Agent darf
	// nur Tools mit RequireScope="" nutzen.
	perms, _ := c.Get("permissions").([]string)

	var input struct {
		Goal          string   `json:"goal"`
		ContextHints  []string `json:"context_hints"`
		MaxIterations int      `json:"max_iterations"`
	}
	if err := c.Bind(&input); err != nil || input.Goal == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "goal required"})
	}

	// Eindeutige Run-ID für Approval-Flows generieren.
	runID := uuid.New().String()

	resp := c.Response()
	resp.Header().Set(echo.HeaderContentType, "text/event-stream")
	resp.Header().Set("Cache-Control", "no-cache")
	resp.Header().Set("Connection", "keep-alive")
	resp.Header().Set("X-Accel-Buffering", "no")
	resp.WriteHeader(http.StatusOK)

	h.runner.Run(c.Request().Context(), AgentRunRequest{
		Goal:          input.Goal,
		ContextHints:  input.ContextHints,
		MaxIterations: input.MaxIterations,
		OrgID:         orgID,
		UserID:        userID,
		Permissions:   perms,
		RunID:         runID,
	}, func(evt AgentEvent) {
		payload, err := json.Marshal(evt)
		if err != nil {
			log.Warn().Err(err).Msg("ai.agent: marshal event failed")
			return
		}
		if _, werr := fmt.Fprintf(resp.Writer, "data: %s\n\n", payload); werr != nil {
			// Client disconnect — Runner-Context cancelt eh schon.
			return
		}
		resp.Flush()
	})

	_, _ = fmt.Fprint(resp.Writer, "data: [DONE]\n\n")
	resp.Flush()
	return nil
}

// ApproveRun genehmigt einen wartenden Write-Tool-Call in einem laufenden
// Agent-Run.  The caller's org and user are verified against the owner of
// the run that was recorded at Register-time; without this check any
// authenticated user could approve a write-tool call in another org's
// agent run by guessing the run_id (audit finding F11/Cross-Org-Hijack).
//
// POST /api/v1/vaktcomply/ai/agent/runs/:run_id/approve
func (h *AgentHandler) ApproveRun(c echo.Context) error {
	return h.decideRun(c, true)
}

// RejectRun lehnt einen wartenden Write-Tool-Call ab. Owner-Check analog
// zu ApproveRun.
//
// POST /api/v1/vaktcomply/ai/agent/runs/:run_id/reject
func (h *AgentHandler) RejectRun(c echo.Context) error {
	return h.decideRun(c, false)
}

// decideRun ist die gemeinsame Implementierung für Approve und Reject.
// Beide Endpoints haben dieselbe Auth-Logik — der einzige Unterschied ist
// der Approved-Bool.
func (h *AgentHandler) decideRun(c echo.Context, approve bool) error {
	if h.runMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "approval manager not available"})
	}
	runID := c.Param("run_id")
	if runID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "run_id required"})
	}
	orgID, _ := c.Get("org_id").(string)
	userID, _ := c.Get("user_id").(string)
	if orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	err := h.runMgr.Decide(runID, orgID, userID, ApprovalDecision{Approved: approve, UserID: userID})
	switch {
	case err == nil:
		status := "approved"
		if !approve {
			status = "rejected"
		}
		return c.JSON(http.StatusOK, map[string]string{"status": status, "run_id": runID})
	case errors.Is(err, ErrApprovalForbidden):
		log.Warn().Str("run_id", runID).Str("caller_org", orgID).Msg("agent: rejected cross-org approval attempt")
		// Return 404 (not 403) so an attacker who guesses a foreign run_id
		// cannot distinguish "exists in another org" from "does not exist".
		return c.JSON(http.StatusNotFound, map[string]string{"error": "run not found or already decided"})
	default:
		return c.JSON(http.StatusNotFound, map[string]string{"error": "run not found or already decided"})
	}
}
