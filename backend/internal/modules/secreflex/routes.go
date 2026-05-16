package secreflex

import (
	"github.com/labstack/echo/v4"
	"github.com/sechealth-app/sechealth/internal/auth"
	"github.com/sechealth-app/sechealth/internal/license"
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
	p.GET("/templates", h.ListTemplates, license.Require(license.FeatureSecReflex))
	p.GET("/templates/presets", h.ListPresets, license.Require(license.FeatureSecReflex))
	p.POST("/templates", h.CreateTemplate, rw, license.Require(license.FeatureSecReflex))

	// --- Pro: Target groups ---
	p.GET("/groups", h.ListTargetGroups, license.Require(license.FeatureSecReflex))
	p.POST("/groups", h.CreateTargetGroup, rw, license.Require(license.FeatureSecReflex))
	p.GET("/groups/:id/targets", h.ListTargets, license.Require(license.FeatureSecReflex))
	p.POST("/groups/:id/targets/import", h.ImportTargetsCSV, rw, license.Require(license.FeatureSecReflex))

	// --- Pro: Landing pages ---
	p.GET("/landing-pages", h.ListLandingPages, license.Require(license.FeatureSecReflex))
	p.POST("/landing-pages", h.CreateLandingPage, rw, license.Require(license.FeatureSecReflex))

	// --- Pro: Campaign management (multi-campaign orchestration) ---
	p.GET("/campaigns", h.ListCampaigns, license.Require(license.FeatureSecReflex))
	p.POST("/campaigns", h.CreateCampaign, rw, license.Require(license.FeatureSecReflex))
	p.GET("/campaigns/:id", h.GetCampaign, license.Require(license.FeatureSecReflex))
	p.POST("/campaigns/:id/launch", h.LaunchCampaign, rw, license.Require(license.FeatureSecReflex))
	p.POST("/campaigns/:id/abort", h.AbortCampaign, rw, license.Require(license.FeatureSecReflex))
	p.GET("/campaigns/:id/stats", h.GetCampaignStats, license.Require(license.FeatureSecReflex))
	p.GET("/campaigns/:id/report", h.ExportCampaignReport, license.Require(license.FeatureSecReflex))

	// --- Community: Training modules (create/list) ---
	p.GET("/training-modules", h.ListModules)
	p.POST("/training-modules", h.CreateModule, rw)

	// --- Community: Basic assignment tracking ---
	p.GET("/assignments", h.ListAssignments)
	p.POST("/assignments/:id/complete", h.CompleteAssignment)

	// Public tracking (no auth required)
	g.GET("/t/:token", h.TrackClick)
	g.POST("/t/:token/submit", h.TrackFormSubmission)

	// Public phish-report webhook (no auth — validated via org_token in body)
	g.POST("/phish-report", h.ReceivePhishReport)

	// --- Pro: Phish-report analytics and token management ---
	p.GET("/phish-reports", h.ListPhishReports, license.Require(license.FeatureSecReflex))
	p.GET("/phish-reports/stats", h.GetPhishReportStats, license.Require(license.FeatureSecReflex))
	p.POST("/phish-report-token/regenerate", h.RegeneratePhishToken, rw, license.Require(license.FeatureSecReflex))
}
