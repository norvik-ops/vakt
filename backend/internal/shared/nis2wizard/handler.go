package nis2wizard

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// Sprint 19 / S19-1 + S19-3: HTTP-Handler für den Public Wizard.
//
// Alle Endpoints sind PUBLIC — keine Auth-Middleware. Token im Body / URL ist
// die einzige "Berechtigung". Token expiriert nach 7 Tagen, dann ist der Run
// futsch.

type Handler struct {
	svc       *Service
	secretKey string
}

func NewHandler(svc *Service, secretKey string) *Handler {
	return &Handler{svc: svc, secretKey: secretKey}
}

// Register mountet die Public-Endpoints. Der Aufrufer übergibt eine
// `/public/nis2-assessment`-Gruppe OHNE Auth-Middleware.
func Register(g *echo.Group, h *Handler) {
	g.POST("/start", h.Start)
	g.POST("/answer", h.Answer)
	g.GET("/result", h.Result)
	g.GET("/questions", h.Questions)
}

// Start legt einen neuen Run an + gibt Token zurück. Im Body optional
// `referrer` (Marketing-Attribution).
func (h *Handler) Start(c echo.Context) error {
	var input struct {
		Referrer string `json:"referrer"`
	}
	_ = c.Bind(&input)
	ipHash := HashIP(c.RealIP(), h.secretKey)
	run, err := h.svc.StartRun(c.Request().Context(), input.Referrer, c.Request().UserAgent(), ipHash)
	if err != nil {
		log.Error().Err(err).Msg("nis2wizard: start failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "start failed"})
	}
	return c.JSON(http.StatusOK, run)
}

// Answer speichert eine Antwort + gibt Live-Score zurück.
func (h *Handler) Answer(c echo.Context) error {
	var input struct {
		Token      string `json:"token"`
		QuestionID string `json:"question_id"`
		Value      int    `json:"value"`
		Comment    string `json:"comment"`
	}
	if err := c.Bind(&input); err != nil || input.Token == "" || input.QuestionID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "token + question_id required"})
	}
	run, err := h.svc.Answer(c.Request().Context(), input.Token, input.QuestionID, input.Value, input.Comment)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, run)
}

// Result liefert den aktuellen Stand + Top-Gaps.
func (h *Handler) Result(c echo.Context) error {
	token := c.QueryParam("token")
	if token == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "token query param required"})
	}
	run, err := h.svc.LoadRun(c.Request().Context(), token)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "run not found or expired"})
	}
	type resultOut struct {
		*Run
		TopGaps []Gap `json:"top_gaps"`
	}
	return c.JSON(http.StatusOK, resultOut{Run: run, TopGaps: run.TopGaps(3)})
}

// Questions liefert die statische Fragen-Liste für den Wizard-Flow.
// Public — keine Auth, kein Rate-Limit (Cache-fähig im CDN).
func (h *Handler) Questions(c echo.Context) error {
	c.Response().Header().Set("Cache-Control", "public, max-age=3600")
	return c.JSON(http.StatusOK, map[string]any{
		"questions": Questions,
		"areas":     AllAreas,
	})
}
