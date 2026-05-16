package ai

import (
	"net/http"

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
