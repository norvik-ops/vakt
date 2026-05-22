package ai

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Status checks if the configured AI provider is reachable.
func (h *Handler) Status(c echo.Context) error {
	available := h.svc.IsAvailable(c.Request().Context())
	model := h.svc.client.model
	return c.JSON(http.StatusOK, map[string]any{
		"available": available,
		"model":     model,
	})
}

// ComplianceAdvice handles POST /secvitals/ai/advice.
// It collects the org's current compliance gaps and asks the LLM for a
// prioritized weekly action plan. Returns {"advice": "1. ...\n2. ..."}.
func (h *Handler) ComplianceAdvice(c echo.Context) error {
	orgID, ok := c.Get("org_id").(string)
	if !ok || orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	advice, err := h.svc.ComplianceAdvice(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Msg("ComplianceAdvice failed")
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "KI temporär nicht verfügbar"})
	}

	return c.JSON(http.StatusOK, map[string]string{"advice": advice})
}

// DraftPolicy handles POST /secvitals/ai/draft-policy.
// Body: { topic: string, framework?: string }
// Returns: { draft: string } — Markdown policy draft for the admin to review.
func (h *Handler) DraftPolicy(c echo.Context) error {
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

// IncidentResponseGuide handles POST /secvitals/ai/incident-guide.
// Body: { summary: string, type?: string }
// Returns: { guide: string } — numbered checklist with response steps + deadline hints.
func (h *Handler) IncidentResponseGuide(c echo.Context) error {
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
			h.svc.usage.Record(c.Request().Context(), UsageRecord{
				OrgID: orgID, Model: h.svc.model, Status: "rate_limited", RequestID: "chat.stream",
			})
			return c.JSON(http.StatusTooManyRequests, map[string]string{"error": err.Error(), "code": "AI_RATE_LIMITED"})
		}
		if err := h.svc.usage.CheckDailyQuota(c.Request().Context(), orgID); err != nil {
			h.svc.usage.Record(c.Request().Context(), UsageRecord{
				OrgID: orgID, Model: h.svc.model, Status: "rate_limited", RequestID: "chat.stream",
			})
			return c.JSON(http.StatusForbidden, map[string]string{"error": err.Error(), "code": "AI_QUOTA_EXCEEDED"})
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
		_, _ = fmt.Fprintf(resp.Writer, "event: error\ndata: %s\n\n", err.Error())
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

// ListOllamaModels handles GET /api/v1/secvitals/ai/models.
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
