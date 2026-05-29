package ai

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/license"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// ceLimitResponse is the standard body returned when a CE org exceeds the monthly AI limit.
type ceLimitResponse struct {
	Error     string `json:"error"`
	Code      string `json:"code"`
	Used      int    `json:"used"`
	Limit     int    `json:"limit"`
	ResetHint string `json:"reset_hint"`
}

// checkCELimit returns HTTP 402 when a Community-tier org has reached the 25-request monthly cap.
// For Pro/Enterprise orgs (or when usage tracking is unavailable) it is a no-op and returns nil.
func (h *Handler) checkCELimit(c echo.Context) error {
	lic, _ := c.Get("license").(*license.License)
	if lic == nil || lic.IsPro() {
		return nil
	}
	// CE org — check monthly request count.
	if h.svc.usage == nil {
		return nil
	}
	orgID, _ := c.Get("org_id").(string)
	used := h.svc.usage.CEMonthlyUsage(c.Request().Context(), orgID)
	if used >= CEMonthlyLimit {
		return c.JSON(http.StatusPaymentRequired, ceLimitResponse{
			Error:     "AI-Limit für Community Edition erreicht",
			Code:      "AI_CE_MONTHLY_LIMIT",
			Used:      used,
			Limit:     CEMonthlyLimit,
			ResetHint: "Limit wird am 1. des nächsten Monats zurückgesetzt. Upgrade auf Vakt Pro für unbegrenzte AI-Anfragen.",
		})
	}
	return nil
}

// Status checks if the configured AI provider is reachable.
//
// The response includes `provider_host`, which the frontend's LocalLLMBadge
// uses to decide whether the AI provider is local (Ollama, LM-Studio, llm-
// proxy) or a cloud endpoint (OpenAI, Mistral, …). Without this field the
// badge falls back to "lokal" — which would be a trust-cue lie when the
// admin has pointed Vakt at a cloud provider. Audit finding F2.
func (h *Handler) Status(c echo.Context) error {
	available := h.svc.IsAvailable(c.Request().Context())
	model := h.svc.client.model
	providerHost := providerHostFromBaseURL(h.svc.client.baseURL)
	return c.JSON(http.StatusOK, map[string]any{
		"available":     available,
		"model":         model,
		"provider_host": providerHost,
	})
}

// providerHostFromBaseURL extracts the host portion of the configured AI
// base URL. Returns an empty string when the URL cannot be parsed — the
// frontend treats that as "unknown", which is rendered as a Cloud-Badge to
// stay on the safe side.
func providerHostFromBaseURL(baseURL string) string {
	if baseURL == "" {
		return ""
	}
	u, err := url.Parse(baseURL)
	if err != nil || u.Host == "" {
		return ""
	}
	return u.Hostname()
}

// Usage returns the current CE monthly AI usage for the authenticated org.
// Pro/Enterprise orgs always get {"used":0,"limit":-1,"is_pro":true}.
// This endpoint is used by the frontend to display "18/25 Anfragen diesen Monat".
func (h *Handler) Usage(c echo.Context) error {
	lic, _ := c.Get("license").(*license.License)
	isPro := lic != nil && lic.IsPro()
	if isPro {
		return c.JSON(http.StatusOK, map[string]any{
			"used":   0,
			"limit":  -1,
			"is_pro": true,
		})
	}
	orgID, _ := c.Get("org_id").(string)
	used := 0
	if h.svc.usage != nil && orgID != "" {
		used = h.svc.usage.CEMonthlyUsage(c.Request().Context(), orgID)
	}
	return c.JSON(http.StatusOK, map[string]any{
		"used":   used,
		"limit":  CEMonthlyLimit,
		"is_pro": false,
	})
}

// ComplianceAdvice handles POST /vaktcomply/ai/advice.
// It collects the org's current compliance gaps and asks the LLM for a
// prioritized weekly action plan. Returns {"advice": "1. ...\n2. ..."}.
func (h *Handler) ComplianceAdvice(c echo.Context) error {
	orgID, ok := c.Get("org_id").(string)
	if !ok || orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	if err := h.checkCELimit(c); err != nil {
		return err
	}

	advice, err := h.svc.ComplianceAdvice(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Msg("ComplianceAdvice failed")
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "KI temporär nicht verfügbar"})
	}

	return c.JSON(http.StatusOK, map[string]string{"advice": advice})
}

// DraftPolicy handles POST /vaktcomply/ai/draft-policy.
// Body: { topic: string, framework?: string }
// Returns: { draft: string } — Markdown policy draft for the admin to review.
func (h *Handler) DraftPolicy(c echo.Context) error {
	if err := h.checkCELimit(c); err != nil {
		return err
	}
	var input struct {
		Topic     string `json:"topic"`
		Framework string `json:"framework"`
	}
	if err := c.Bind(&input); err != nil || input.Topic == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "topic required"})
	}
	draft, err := h.svc.DraftPolicy(c.Request().Context(), input.Topic, input.Framework)
	if err != nil {
		log.Error().Err(err).Msg("DraftPolicy failed")
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "KI temporär nicht verfügbar"})
	}
	return c.JSON(http.StatusOK, map[string]string{"draft": draft})
}

// IncidentResponseGuide handles POST /vaktcomply/ai/incident-guide.
// Body: { summary: string, type?: string }
// Returns: { guide: string } — numbered checklist with response steps + deadline hints.
func (h *Handler) IncidentResponseGuide(c echo.Context) error {
	if err := h.checkCELimit(c); err != nil {
		return err
	}
	var input struct {
		Summary string `json:"summary"`
		Type    string `json:"type"`
	}
	if err := c.Bind(&input); err != nil || input.Summary == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "summary required"})
	}
	guide, err := h.svc.IncidentResponseGuide(c.Request().Context(), input.Summary, input.Type)
	if err != nil {
		log.Error().Err(err).Msg("IncidentResponseGuide failed")
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "KI temporär nicht verfügbar"})
	}
	return c.JSON(http.StatusOK, map[string]string{"guide": guide})
}

// GenerateReport creates an AI-generated report for the org.
func (h *Handler) GenerateReport(c echo.Context) error {
	orgID, ok := c.Get("org_id").(string)
	if !ok || orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	var input struct {
		Type string `json:"type"`
	}
	if err := c.Bind(&input); err != nil || input.Type == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "type required (gap_analysis|risk_summary|executive_summary)"})
	}

	reportType := ReportType(input.Type)
	switch reportType {
	case ReportGapAnalysis, ReportRiskSummary, ReportExecutiveSummary:
	default:
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "unknown report type"})
	}

	text, err := h.svc.GenerateReport(c.Request().Context(), orgID, reportType)
	if err != nil {
		log.Error().Err(err).Msg("GenerateReport failed")
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "AI report generation failed"})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"type":   input.Type,
		"report": text,
	})
}

// ChatStream handles POST /api/v1/.../ai/chat/stream.
//
// Body: { "system": "...", "prompt": "...", "max_tokens": 600 }
// Response: text/event-stream mit OpenAI-konformen SSE-Frames:
//
//	data: {"delta":{"content":"…"}}
//	data: {"delta":{"content":"…"}}
//	data: [DONE]
//
// Sprint 15 / S15-5. Vor dem Streaming-Start läuft Rate-Limit + Quota durch
// gateAndStream — analog zu gateAndGenerate für nicht-streaming-Calls.
func (h *Handler) ChatStream(c echo.Context) error {
	if err := h.checkCELimit(c); err != nil {
		return err
	}
	orgID, _ := c.Get("org_id").(string)
	if orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	var input struct {
		System    string `json:"system"`
		Prompt    string `json:"prompt"`
		MaxTokens int    `json:"max_tokens"`
	}
	if err := c.Bind(&input); err != nil || input.Prompt == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "prompt required"})
	}
	if input.MaxTokens <= 0 || input.MaxTokens > 4096 {
		input.MaxTokens = 1200
	}

	// Rate-Limit + Quota vor dem Stream-Start.
	if h.svc.usage != nil {
		if err := h.svc.usage.CheckRateLimit(c.Request().Context(), orgID); err != nil {
			if errors.Is(err, ErrUsageCheckUnavailable) {
				return c.JSON(http.StatusServiceUnavailable, map[string]string{
					"error": "AI temporarily unavailable. Please retry shortly.",
					"code":  "AI_USAGE_CHECK_UNAVAILABLE",
				})
			}
			h.svc.usage.Record(c.Request().Context(), UsageRecord{
				OrgID: orgID, Model: h.svc.model, Status: "rate_limited", RequestID: "chat.stream",
			})
			// Retry-After: Sekunden bis zum nächsten 60-Sekunden-Fenster.
			retryAfter := 60 - (time.Now().UTC().Unix() % 60)
			c.Response().Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
			return c.JSON(http.StatusTooManyRequests, map[string]string{"error": "AI rate limit exceeded", "code": "AI_RATE_LIMITED"})
		}
		if err := h.svc.usage.CheckDailyQuota(c.Request().Context(), orgID); err != nil {
			if errors.Is(err, ErrUsageCheckUnavailable) {
				return c.JSON(http.StatusServiceUnavailable, map[string]string{
					"error": "AI temporarily unavailable. Please retry shortly.",
					"code":  "AI_USAGE_CHECK_UNAVAILABLE",
				})
			}
			h.svc.usage.Record(c.Request().Context(), UsageRecord{
				OrgID: orgID, Model: h.svc.model, Status: "rate_limited", RequestID: "chat.stream",
			})
			return c.JSON(http.StatusForbidden, map[string]string{"error": "AI daily quota exceeded", "code": "AI_QUOTA_EXCEEDED"})
		}
	}

	// SSE-Header setzen.
	resp := c.Response()
	resp.Header().Set(echo.HeaderContentType, "text/event-stream")
	resp.Header().Set("Cache-Control", "no-cache")
	resp.Header().Set("Connection", "keep-alive")
	resp.Header().Set("X-Accel-Buffering", "no") // nginx: disable buffering
	resp.WriteHeader(http.StatusOK)

	stream, err := h.svc.client.StreamGenerate(c.Request().Context(), input.System, input.Prompt, input.MaxTokens)
	if err != nil {
		log.Error().Err(err).Msg("ai stream: provider error")
		if h.svc.usage != nil {
			h.svc.usage.Record(c.Request().Context(), UsageRecord{
				OrgID: orgID, Model: h.svc.model, Status: "provider_error", RequestID: "chat.stream",
			})
		}
		_, _ = fmt.Fprintf(resp.Writer, "event: error\ndata: AI provider error\n\n")
		resp.Flush()
		return nil
	}

	start := time.Now()
	var totalContent string
	for chunk := range stream {
		if chunk.Done {
			break
		}
		// JSON-encode den Content-Chunk fuer trivialen Frontend-Decode.
		payload, _ := json.Marshal(map[string]string{"content": chunk.Content})
		if _, werr := fmt.Fprintf(resp.Writer, "data: %s\n\n", payload); werr != nil {
			// Client disconnect — kontrollierter Abbruch.
			if !errors.Is(werr, http.ErrHandlerTimeout) {
				log.Debug().Err(werr).Msg("ai stream: client disconnect")
			}
			break
		}
		resp.Flush()
		totalContent += chunk.Content
	}
	// End-Frame
	_, _ = fmt.Fprintf(resp.Writer, "data: [DONE]\n\n")
	resp.Flush()

	// Usage persistieren (Tokens unbekannt; only duration + status).
	if h.svc.usage != nil {
		h.svc.usage.Record(c.Request().Context(), UsageRecord{
			OrgID: orgID, Model: h.svc.model,
			DurationMs: int(time.Since(start).Milliseconds()),
			Status:     "ok",
			RequestID:  "chat.stream",
		})
	}
	return nil
}

// GapExplain handles POST /api/v1/vaktcomply/ai/controls/:id/explain.
// It streams an explanation of why a control is still open and suggests 3 next steps.
// S52-2.
func (h *Handler) GapExplain(c echo.Context) error {
	if err := h.checkCELimit(c); err != nil {
		return err
	}
	orgID, _ := c.Get("org_id").(string)
	if orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	controlID := c.Param("id")
	if controlID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "control id required"})
	}

	// Fetch control details.
	type controlRow struct {
		title         string
		description   string
		framework     string
		evidenceCount int
	}
	var ctrl controlRow
	err := h.svc.db.QueryRow(c.Request().Context(), `
		SELECT c.title, COALESCE(c.description::text, ''), COALESCE(f.name, ''),
		       (SELECT COUNT(*) FROM ck_evidence e WHERE e.control_id = c.id)::int
		FROM ck_controls c
		LEFT JOIN ck_frameworks f ON f.id = c.framework_id
		WHERE c.id = $1::uuid AND c.org_id = $2::uuid
	`, controlID, orgID).Scan(&ctrl.title, &ctrl.description, &ctrl.framework, &ctrl.evidenceCount)
	if err != nil {
		log.Error().Err(err).Str("control_id", controlID).Msg("gap_explain: fetch control failed")
		return c.JSON(http.StatusNotFound, map[string]string{"error": "control not found"})
	}

	systemPrompt := addInjectionGuard("Du bist ein ISO-27001/NIS2/BSI-Compliance-Experte. Antworte auf Deutsch, konkret und handlungsorientiert.")
	userPrompt := fmt.Sprintf(
		"Control: %s. Beschreibung: %s. Framework: %s. Offener Status: gap. Evidence vorhanden: %d. Erkläre in 3–5 Sätzen warum dieser Control offen ist und nenne 3 konkrete nächste Schritte.",
		sanitizeUserInput(ctrl.title),
		sanitizeUserInput(ctrl.description),
		sanitizeUserInput(ctrl.framework),
		ctrl.evidenceCount,
	)

	// Set SSE headers.
	resp := c.Response()
	resp.Header().Set(echo.HeaderContentType, "text/event-stream")
	resp.Header().Set("Cache-Control", "no-cache")
	resp.Header().Set("Connection", "keep-alive")
	resp.Header().Set("X-Accel-Buffering", "no")
	resp.WriteHeader(http.StatusOK)

	stream, err := h.svc.client.StreamGenerate(c.Request().Context(), systemPrompt, userPrompt, 800)
	if err != nil {
		log.Error().Err(err).Str("control_id", controlID).Msg("gap_explain: stream error")
		_, _ = fmt.Fprintf(resp.Writer, "event: error\ndata: AI provider error\n\n")
		resp.Flush()
		return nil
	}

	start := time.Now()
	for chunk := range stream {
		if chunk.Done {
			break
		}
		payload, _ := json.Marshal(map[string]string{"content": chunk.Content})
		if _, werr := fmt.Fprintf(resp.Writer, "data: %s\n\n", payload); werr != nil {
			if !errors.Is(werr, http.ErrHandlerTimeout) {
				log.Debug().Err(werr).Msg("gap_explain: client disconnect")
			}
			break
		}
		resp.Flush()
	}
	_, _ = fmt.Fprintf(resp.Writer, "data: [DONE]\n\n")
	resp.Flush()
	// Record so the CE monthly counter advances. Without this the gate
	// in RequireAILimit would never trip for orgs that only use the SSE
	// endpoints.  Audit F3.
	if h.svc.usage != nil {
		h.svc.usage.Record(c.Request().Context(), UsageRecord{
			OrgID: orgID, Model: h.svc.model,
			DurationMs: int(time.Since(start).Milliseconds()),
			Status:     "ok",
			RequestID:  "controls.explain",
		})
	}
	return nil
}

// RiskNarrative handles POST /api/v1/vaktcomply/ai/risks/:id/narrative.
// It generates a 2–3 sentence audit narrative for the risk and persists it.
// S52-3.
func (h *Handler) RiskNarrative(c echo.Context) error {
	if err := h.checkCELimit(c); err != nil {
		return err
	}
	orgID, _ := c.Get("org_id").(string)
	if orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	riskID := c.Param("id")
	if riskID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "risk id required"})
	}

	// Fetch risk details.
	type riskRow struct {
		title       string
		description string
		category    string
		likelihood  int
		impact      int
		riskScore   int
		treatment   string
		owner       string
	}
	var r riskRow
	err := h.svc.db.QueryRow(c.Request().Context(), `
		SELECT
			title,
			COALESCE(description, ''),
			COALESCE(category, ''),
			likelihood,
			impact,
			likelihood * impact,
			COALESCE(treatment, ''),
			COALESCE(owner, '')
		FROM ck_risks
		WHERE id = $1::uuid AND org_id = $2::uuid
	`, riskID, orgID).Scan(
		&r.title, &r.description, &r.category,
		&r.likelihood, &r.impact, &r.riskScore,
		&r.treatment, &r.owner,
	)
	if err != nil {
		log.Error().Err(err).Str("risk_id", riskID).Msg("risk_narrative: fetch risk failed")
		return c.JSON(http.StatusNotFound, map[string]string{"error": "risk not found"})
	}

	systemPrompt := addInjectionGuard("Du bist ein ISO-27001/NIS2/BSI-Compliance-Experte und Auditor. Antworte auf Deutsch, präzise und professionell.")
	userPrompt := fmt.Sprintf(
		"Risiko: %s. Beschreibung: %s. Kategorie: %s. Eintrittswahrscheinlichkeit: %d, Auswirkung: %d, Risiko-Score: %d. Behandlung: %s. Verantwortlicher: %s. Schreibe ein Audit-Narrativ von 2–3 Sätzen für dieses Risiko.",
		sanitizeUserInput(r.title),
		sanitizeUserInput(r.description),
		sanitizeUserInput(r.category),
		r.likelihood, r.impact, r.riskScore,
		sanitizeUserInput(r.treatment),
		sanitizeUserInput(r.owner),
	)

	narrative, err := h.svc.client.GenerateWithSystem(c.Request().Context(), systemPrompt, userPrompt)
	if err != nil {
		log.Error().Err(err).Str("risk_id", riskID).Msg("risk_narrative: generate failed")
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "KI temporär nicht verfügbar"})
	}

	// Persist the narrative.
	if _, dbErr := h.svc.db.Exec(c.Request().Context(),
		`UPDATE ck_risks SET ai_narrative = $1 WHERE id = $2::uuid AND org_id = $3::uuid`,
		narrative, riskID, orgID,
	); dbErr != nil {
		log.Warn().Err(dbErr).Str("risk_id", riskID).Msg("risk_narrative: persist failed")
	}

	// Record so the CE monthly counter advances. Audit F3.
	if h.svc.usage != nil {
		h.svc.usage.Record(c.Request().Context(), UsageRecord{
			OrgID: orgID, Model: h.svc.model,
			Status: "ok", RequestID: "risks.narrative",
		})
	}

	return c.JSON(http.StatusOK, map[string]string{"narrative": narrative})
}

// aiInsightResponse is the JSON shape returned by ListInsights.
type aiInsightResponse struct {
	ID        string  `json:"id"`
	Type      string  `json:"type"`
	Title     string  `json:"title"`
	Message   string  `json:"message"`
	ControlID *string `json:"control_id,omitempty"`
	RiskID    *string `json:"risk_id,omitempty"`
	FindingID *string `json:"finding_id,omitempty"`
	Urgency   int     `json:"urgency"`
	CreatedAt string  `json:"created_at"`
}

// ListInsights handles GET /api/v1/vaktcomply/ai/insights.
// Returns up to 5 active AI insights for the org. S52-6.
func (h *Handler) ListInsights(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	if orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	rows, err := h.svc.db.Query(c.Request().Context(), `
		SELECT
			id::text, type, title, message,
			control_id::text, risk_id::text, finding_id::text,
			urgency, created_at
		FROM ck_ai_insights
		WHERE org_id = $1::uuid AND dismissed_at IS NULL
		ORDER BY urgency ASC, created_at DESC
		LIMIT 5
	`, orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("list_insights: query failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
	}
	defer rows.Close()

	var results []aiInsightResponse
	for rows.Next() {
		var ins aiInsightResponse
		var createdAt time.Time
		var controlID, riskID, findingID *string
		if err := rows.Scan(
			&ins.ID, &ins.Type, &ins.Title, &ins.Message,
			&controlID, &riskID, &findingID,
			&ins.Urgency, &createdAt,
		); err != nil {
			log.Warn().Err(err).Msg("list_insights: scan failed")
			continue
		}
		ins.ControlID = controlID
		ins.RiskID = riskID
		ins.FindingID = findingID
		ins.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		results = append(results, ins)
	}
	if results == nil {
		results = []aiInsightResponse{}
	}
	return c.JSON(http.StatusOK, results)
}

// DismissInsight handles DELETE /api/v1/vaktcomply/ai/insights/:id.
// Sets dismissed_at on the insight, scoped to the org. S52-6.
func (h *Handler) DismissInsight(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	if orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	insightID := c.Param("id")
	if insightID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "insight id required"})
	}

	tag, err := h.svc.db.Exec(c.Request().Context(), `
		UPDATE ck_ai_insights
		SET dismissed_at = NOW()
		WHERE id = $1::uuid AND org_id = $2::uuid AND dismissed_at IS NULL
	`, insightID, orgID)
	if err != nil {
		log.Error().Err(err).Str("insight_id", insightID).Msg("dismiss_insight: update failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
	}
	if tag.RowsAffected() == 0 {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "insight not found"})
	}
	return c.NoContent(http.StatusNoContent)
}

// ListOllamaModels handles GET /api/v1/vaktcomply/ai/models.
// Proxies the Ollama /api/tags endpoint and returns a simplified model list.
// When the AI provider is not Ollama or is unavailable, returns an empty list.
func (h *Handler) ListOllamaModels(c echo.Context) error {
	baseURL := h.svc.client.baseURL
	if baseURL == "" {
		return c.JSON(http.StatusOK, map[string]any{"models": []string{}})
	}
	// Strip /v1 suffix to get the Ollama root URL.
	ollamaRoot := baseURL
	if len(ollamaRoot) > 3 && ollamaRoot[len(ollamaRoot)-3:] == "/v1" {
		ollamaRoot = ollamaRoot[:len(ollamaRoot)-3]
	}

	ctx := c.Request().Context()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ollamaRoot+"/api/tags", nil)
	if err != nil {
		return c.JSON(http.StatusOK, map[string]any{"models": []string{}})
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return c.JSON(http.StatusOK, map[string]any{"models": []string{}})
	}
	defer resp.Body.Close()

	var payload struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return c.JSON(http.StatusOK, map[string]any{"models": []string{}})
	}
	names := make([]string, 0, len(payload.Models))
	for _, m := range payload.Models {
		names = append(names, m.Name)
	}
	return c.JSON(http.StatusOK, map[string]any{"models": names})
}
