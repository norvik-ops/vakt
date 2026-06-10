package vaktaware

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/matharnica/vakt/internal/auth"
	"github.com/matharnica/vakt/internal/shared/platform/features"
)

// Register wires PhishGuard routes under the provided group.
//
// Feature gating:
//   - Community: basic training assignment + completion tracking (assignments)
//   - Pro (FeatureSecReflex): phishing campaigns, template management, target groups,
//     landing pages, per-campaign analytics and report export, phish-report stats
func Register(g *echo.Group, h *Handler) {
	rw := auth.RequireRole("Admin", "SecurityAnalyst")
	ro := auth.RequireRole("Admin", "SecurityAnalyst", "Viewer", "AuditorReadOnly")

	p := g.Group("", ro)

	// --- Pro: Template management ---
	p.GET("/templates", h.ListTemplates, features.Require(features.FeatureSecReflex))
	p.GET("/templates/presets", h.ListPresets, features.Require(features.FeatureSecReflex))
	p.POST("/templates", h.CreateTemplate, rw, features.Require(features.FeatureSecReflex))

	// --- Pro: Target groups ---
	p.GET("/groups", h.ListTargetGroups, features.Require(features.FeatureSecReflex))
	p.POST("/groups", h.CreateTargetGroup, rw, features.Require(features.FeatureSecReflex))
	p.GET("/groups/:id/targets", h.ListTargets, features.Require(features.FeatureSecReflex))
	p.POST("/groups/:id/targets/import", h.ImportTargetsCSV, rw, features.Require(features.FeatureSecReflex))

	// --- Pro: Landing pages ---
	p.GET("/landing-pages", h.ListLandingPages, features.Require(features.FeatureSecReflex))
	p.POST("/landing-pages", h.CreateLandingPage, rw, features.Require(features.FeatureSecReflex))

	// --- Pro: Campaign management (multi-campaign orchestration) ---
	p.GET("/campaigns", h.ListCampaigns, features.Require(features.FeatureSecReflex))
	p.POST("/campaigns", h.CreateCampaign, rw, features.Require(features.FeatureSecReflex))
	p.GET("/campaigns/:id", h.GetCampaign, features.Require(features.FeatureSecReflex))
	p.POST("/campaigns/:id/launch", h.LaunchCampaign, rw, features.Require(features.FeatureSecReflex))
	p.POST("/campaigns/:id/abort", h.AbortCampaign, rw, features.Require(features.FeatureSecReflex))
	p.GET("/campaigns/:id/stats", h.GetCampaignStats, features.Require(features.FeatureSecReflex))
	p.GET("/campaigns/:id/report", h.ExportCampaignReport, features.Require(features.FeatureSecReflex))

	// --- Community: Training modules (create/list) ---
	p.GET("/training-modules", h.ListModules)
	p.GET("/training-modules/presets", h.ListTrainingPresets)
	p.POST("/training-modules", h.CreateModule, rw)

	// --- Community: Basic assignment tracking ---
	p.GET("/assignments", h.ListAssignments)
	p.POST("/assignments/:id/complete", h.CompleteAssignment)
	p.GET("/assignments/:id/certificate", h.GetAssignmentCertificate)

	// Public tracking (no auth required) — rate-limited to 10 req/min per IP
	// to prevent token enumeration attacks.
	trackingRL := echomiddleware.RateLimiterWithConfig(echomiddleware.RateLimiterConfig{
		Skipper: echomiddleware.DefaultSkipper,
		Store: echomiddleware.NewRateLimiterMemoryStoreWithConfig(
			echomiddleware.RateLimiterMemoryStoreConfig{
				Rate:      10,
				Burst:     10,
				ExpiresIn: time.Minute,
			},
		),
		IdentifierExtractor: func(c echo.Context) (string, error) {
			return c.RealIP(), nil
		},
		ErrorHandler: func(c echo.Context, err error) error {
			return c.JSON(http.StatusTooManyRequests, map[string]string{
				"error": "too many requests",
				"code":  "RATE_LIMIT_EXCEEDED",
			})
		},
		DenyHandler: func(c echo.Context, identifier string, err error) error {
			return c.JSON(http.StatusTooManyRequests, map[string]string{
				"error": "too many requests",
				"code":  "RATE_LIMIT_EXCEEDED",
			})
		},
	})
	tracking := g.Group("", trackingRL)
	tracking.GET("/t/:token", h.TrackClick)
	tracking.POST("/t/:token/submit", h.TrackFormSubmission)
	tracking.GET("/track/:token", h.TrackOpen) // open-tracking pixel (1×1 GIF)

	// Public phish-report webhook (no auth — validated via org_token in body)
	g.POST("/phish-report", h.ReceivePhishReport)

	// --- Pro: Phish-report analytics and token management ---
	p.GET("/phish-reports", h.ListPhishReports, features.Require(features.FeatureSecReflex))
	p.GET("/phish-reports/stats", h.GetPhishReportStats, features.Require(features.FeatureSecReflex))
	p.POST("/phish-report-token/regenerate", h.RegeneratePhishToken, rw, features.Require(features.FeatureSecReflex))

	// --- S65-1: Template library (filtered presets) ---
	p.GET("/templates/library", h.ListPresetsFiltered, features.Require(features.FeatureSecReflex))

	// --- S65-2: Auto-enrollment rules ---
	p.GET("/enrollment-rules", h.ListEnrollmentRules, features.Require(features.FeatureSecReflex))
	p.POST("/enrollment-rules", h.CreateEnrollmentRule, rw, features.Require(features.FeatureSecReflex))
	p.PUT("/enrollment-rules/:id", h.UpdateEnrollmentRuleActive, rw, features.Require(features.FeatureSecReflex))
	p.DELETE("/enrollment-rules/:id", h.DeleteEnrollmentRule, rw, features.Require(features.FeatureSecReflex))

	// --- S65-3: Training evidence export ---
	p.GET("/reports/training-matrix", h.GetTrainingMatrix, features.Require(features.FeatureSecReflex))
	p.GET("/reports/training-matrix/export/pdf", h.ExportTrainingMatrixPDF, features.Require(features.FeatureSecReflex))
	p.GET("/reports/training-matrix/export/csv", h.ExportTrainingMatrixCSV, features.Require(features.FeatureSecReflex))

	// --- S65-4: BSI ORP.3 compliance status ---
	p.GET("/bsi-orp3-status", h.GetORP3Status)
}
