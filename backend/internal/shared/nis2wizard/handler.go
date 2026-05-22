package nis2wizard

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/shared/platform/features"
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
//
// CORS ist bewusst offen (*): diese Endpoints sind das Top-of-Funnel-Asset
// und müssen von beliebigen Partner-Websites aufrufbar sein. Credentials
// werden nicht übertragen (AllowCredentials bleibt false).
func Register(g *echo.Group, h *Handler) {
	g.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodOptions},
		AllowHeaders: []string{"Content-Type", "Accept"},
		MaxAge:       86400,
	}))
	g.POST("/start", h.Start)
	g.POST("/answer", h.Answer)
	g.GET("/result", h.Result)
	g.GET("/questions", h.Questions)

	// Sprint 28 / S28-4: Multi-Framework-Fragen sind public (Cache-fähig).
	g.GET("/multi/questions", h.MultiFrameworkQuestions)
}

// RegisterAuthenticated mountet den Migrate-Endpoint, den PDF-Export-Endpoint
// sowie die Re-Assessment-, History- und Multi-Framework-Endpoints.
// Sprint 22 S22-4/5/6 + Sprint 28 S28-2 + Sprint 28 S28-3 + Sprint 28 S28-4.
// Aufrufer übergibt eine authentifizierte Echo-Gruppe.
func RegisterAuthenticated(g *echo.Group, h *Handler) {
	g.POST("/nis2-assessment/migrate-from-anonymous", h.MigrateFromAnonymous)
	g.POST("/nis2-assessment/pdf", h.ExportPDF)

	// Sprint 28 / S28-3: Re-Assessment + History (ProGate: FeatureNIS2Reporting).
	g.POST("/reassess", h.StartReassessment, features.Require(features.FeatureNIS2Reporting))
	g.POST("/reassess/:id/answer", h.AnswerReassessment)
	g.GET("/reassess/:id/result", h.GetReassessmentResult)
	g.GET("/history", h.GetHistory, features.Require(features.FeatureNIS2Reporting))

	// Sprint 28 / S28-4: Multi-Framework-Assessment (NIS2 + ISO27001 + DSGVO-TOM).
	// ProGate: FeatureNIS2Reporting für Start; Answer + Result sind nach Start frei.
	g.POST("/nis2-assessment/multi/start", h.StartMultiFramework, features.Require(features.FeatureNIS2Reporting))
	g.POST("/nis2-assessment/multi/:id/answer", h.AnswerMultiFramework)
	g.GET("/nis2-assessment/multi/:id/result", h.GetMultiFrameworkResult)
}

// MigrateFromAnonymous ist der authentifizierte Endpoint, der einen anonymen
// Magic-Token in die Org des Aufrufers migriert + die Antworten als
// initialer manual_status auf NIS2-Controls projiziert (Auto-Mapping).
//
// Body: { "token": "<32-hex>" }
// Response: { "assessment_id": "<uuid>", "controls_mapped": N }
func (h *Handler) MigrateFromAnonymous(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	userID, _ := c.Get("user_id").(string)
	if orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	var input struct {
		Token string `json:"token"`
	}
	if err := c.Bind(&input); err != nil || input.Token == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "token required"})
	}
	assessmentID, mapped, err := h.svc.MigrateAndAutoMap(c.Request().Context(), input.Token, orgID, userID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("nis2: migrate-from-anonymous failed")
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"assessment_id":   assessmentID,
		"controls_mapped": mapped,
	})
}

// ExportPDF exportiert ein NIS2-Assessment-Ergebnis als PDF.
// Sprint 28 / S28-2 (S19-8): ProGate FeatureNIS2Reporting.
//
// Query-Param: ?token=<32-hex>
// Response: application/pdf mit Content-Disposition: attachment.
func (h *Handler) ExportPDF(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	if orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	// ProGate: FeatureNIS2Reporting erforderlich.
	if !features.IsEnabled(c, features.FeatureNIS2Reporting) {
		return c.JSON(http.StatusForbidden, map[string]string{
			"error": "NIS2 PDF export requires Pro license",
		})
	}

	token := c.QueryParam("token")
	if token == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "token query param required"})
	}

	run, err := h.svc.LoadRun(c.Request().Context(), token)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "run not found or expired"})
	}

	// Org-Name aus dem Kontext holen (gesetzt von AuthMiddleware), Fallback auf orgID.
	orgName, _ := c.Get("org_name").(string)
	if orgName == "" {
		orgName = fmt.Sprintf("Organisation %s", orgID[:8])
	}

	pdfBytes, err := RenderAssessmentPDF(orgName, run)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("nis2: pdf render failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "pdf generation failed"})
	}

	c.Response().Header().Set("Content-Type", "application/pdf")
	c.Response().Header().Set("Content-Disposition", "attachment; filename=\"nis2-assessment.pdf\"")
	c.Response().Header().Set("Content-Length", fmt.Sprintf("%d", len(pdfBytes)))
	return c.Blob(http.StatusOK, "application/pdf", pdfBytes)
}

// StartReassessment legt einen neuen Re-Assessment-Run für die authentifizierte
// Org an. ProGate: FeatureNIS2Reporting (via Middleware in RegisterAuthenticated).
//
// Response: { "run_id": "<uuid>", "run_number": N }
func (h *Handler) StartReassessment(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	if orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	runID, err := h.svc.CreateReassessmentRun(c.Request().Context(), orgID)
	if err != nil {
		if strings.HasPrefix(err.Error(), "cooldown:") {
			return c.JSON(http.StatusConflict, map[string]string{
				"error": err.Error(),
				"code":  "REASSESSMENT_COOLDOWN",
			})
		}
		log.Error().Err(err).Str("org_id", orgID).Msg("nis2: create reassessment run failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "create run failed"})
	}
	return c.JSON(http.StatusCreated, map[string]any{"run_id": runID})
}

// AnswerReassessment speichert eine Antwort für einen Re-Assessment-Run.
// Kein ProGate — nur der Start braucht das Feature, Antworten sind frei.
//
// Body: { "question_id": "...", "value": 0..4, "comment": "..." }
func (h *Handler) AnswerReassessment(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	if orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	runID := c.Param("id")
	if runID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "run id required"})
	}
	var input struct {
		QuestionID string `json:"question_id"`
		Value      int    `json:"value"`
		Comment    string `json:"comment"`
	}
	if err := c.Bind(&input); err != nil || input.QuestionID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "question_id required"})
	}
	run, err := h.svc.SaveReassessmentAnswer(c.Request().Context(), orgID, runID, input.QuestionID, input.Value, input.Comment)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, run)
}

// GetReassessmentResult liefert den aktuellen Stand eines Re-Assessment-Runs
// inkl. Top-Gaps wenn abgeschlossen.
func (h *Handler) GetReassessmentResult(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	if orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	runID := c.Param("id")
	if runID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "run id required"})
	}
	run, err := h.svc.GetReassessmentResult(c.Request().Context(), orgID, runID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "run not found"})
	}
	return c.JSON(http.StatusOK, run)
}

// GetHistory gibt alle Re-Assessment-Runs einer Org zurück, neuester zuerst.
// ProGate: FeatureNIS2Reporting (via Middleware in RegisterAuthenticated).
//
// Response: { "runs": [...], "total": N }
func (h *Handler) GetHistory(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	if orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	runs, err := h.svc.GetReassessmentHistory(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("nis2: get history failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "get history failed"})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"runs":  runs,
		"total": len(runs),
	})
}

// MultiFrameworkQuestions liefert die kombinierte ~80-Fragen-Liste für NIS2+ISO27001+DSGVO.
// Public — keine Auth, kein Rate-Limit (Cache-fähig im CDN).
func (h *Handler) MultiFrameworkQuestions(c echo.Context) error {
	c.Response().Header().Set("Cache-Control", "public, max-age=3600")
	return c.JSON(http.StatusOK, map[string]any{
		"questions": MultiFrameworkQuestions,
		"areas":     MultiFrameworkAreas,
		"count":     MultiFrameworkQuestionCount(),
	})
}

// StartMultiFramework legt einen neuen Multi-Framework-Run an.
// ProGate: FeatureNIS2Reporting (via Middleware).
func (h *Handler) StartMultiFramework(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	if orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	ipHash := HashIP(c.RealIP(), h.secretKey)
	run, err := h.svc.StartMultiRun(c.Request().Context(), "", c.Request().UserAgent(), ipHash)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("nis2: start multi-framework run failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "start failed"})
	}
	return c.JSON(http.StatusCreated, run)
}

// AnswerMultiFramework speichert eine Antwort für einen Multi-Framework-Run.
func (h *Handler) AnswerMultiFramework(c echo.Context) error {
	runID := c.Param("id")
	if runID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "run id required"})
	}
	var input struct {
		Token      string `json:"token"`
		QuestionID string `json:"question_id"`
		Value      int    `json:"value"`
		Comment    string `json:"comment"`
	}
	if err := c.Bind(&input); err != nil || input.QuestionID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "question_id required"})
	}
	token := input.Token
	if token == "" {
		token = runID
	}
	run, err := h.svc.AnswerMulti(c.Request().Context(), token, input.QuestionID, input.Value, input.Comment)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, run)
}

// GetMultiFrameworkResult liefert das Ergebnis eines Multi-Framework-Runs.
func (h *Handler) GetMultiFrameworkResult(c echo.Context) error {
	runID := c.Param("id")
	if runID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "run id required"})
	}
	run, err := h.svc.LoadMultiRunResult(c.Request().Context(), runID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "run not found or expired"})
	}
	return c.JSON(http.StatusOK, run)
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
