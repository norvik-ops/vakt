package ai

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// Sprint 18 S18-3: AgentRunHandler ist der SSE-Endpoint für Agent-Runs.
//
// POST /api/v1/secvitals/ai/agent/run mit Body:
//   { "goal": "...", "context_hints": [...] }
//
// Response: text/event-stream mit Frames:
//   data: {"type":"plan","step":0,"message":"1. ..."}
//   data: {"type":"tool_call","step":1,"tool":"list_open_findings","arguments":{}}
//   data: {"type":"tool_result","step":1,"tool":"list_open_findings","result":[...]}
//   data: {"type":"final","message":"..."}
//   data: [DONE]
//
// Permissions kommen aus dem User-Context (org_id + user_id + perms). Tools
// werden nur ausgeführt, wenn der User die zugehörigen Scopes hat (ADR-0020).

// AgentHandler bündelt die Dependencies für Agent-Runs.
type AgentHandler struct {
	runner *AgentRunner
}

// NewAgentHandler baut einen Handler. tools wird über DefaultAgentTools(db)
// gespeist (siehe routes.go).
func NewAgentHandler(client *AIClient, model string, runner *AgentRunner) *AgentHandler {
	return &AgentHandler{runner: runner}
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
