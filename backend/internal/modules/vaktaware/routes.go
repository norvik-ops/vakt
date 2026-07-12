package vaktaware

import (
	"github.com/labstack/echo/v4"
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
	// S121-D3 (C9): delete an org-owned template (the UI delete button 404'd).
	p.DELETE("/templates/:id", h.DeleteTemplate, rw, features.Require(features.FeatureSecReflex))

	// --- Pro: Target groups ---
	p.GET("/groups", h.ListTargetGroups, features.Require(features.FeatureSecReflex))
	p.POST("/groups", h.CreateTargetGroup, rw, features.Require(features.FeatureSecReflex))
	p.DELETE("/groups/:id", h.DeleteTargetGroup, rw, features.Require(features.FeatureSecReflex))
	p.GET("/groups/:id/targets", h.ListTargets, features.Require(features.FeatureSecReflex))
	p.POST("/groups/:id/targets", h.AddTarget, rw, features.Require(features.FeatureSecReflex))
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
	p.GET("/training-modules/:id/assignments", h.ListAssignmentsByModule)
	p.POST("/training-modules/:id/assign", h.AssignModule, rw)

	// --- Community: Basic assignment tracking ---
	p.GET("/assignments", h.ListAssignments)
	// S124-8 (D11): marking an assignment complete writes ISO-27001 A.6.3 awareness
	// evidence that flows into Vakt Comply. It was ungated, so a Viewer or
	// AuditorReadOnly could fabricate that evidence. Gated to writer roles (`rw` =
	// Admin, SecurityAnalyst). NOTE: true per-employee self-service completion is
	// deliberately NOT built here — awareness targets are email recipients, not
	// necessarily Vakt users; department-wide assignments (TargetID == nil) have no
	// single owner; and the caller's email is not in the token. An ownership-based
	// self-service model belongs to a tokenized-completion-link flow (the S127
	// public-route class), not this authenticated management endpoint.
	p.POST("/assignments/:id/complete", h.CompleteAssignment, rw)
	p.GET("/assignments/:id/certificate", h.GetAssignmentCertificate)

	// S127-1 (D4/D5/D6): the public tracking + phish-report routes used to be
	// mounted HERE, under `protected` — so every recipient's mail client / browser
	// (which has no Paseto token) got 401 and Vakt Aware recorded nothing. They now
	// live in RegisterPublic below, mounted on a token-only PUBLIC group in
	// cmd/api/routes.go. Do NOT re-add them here.

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

// RegisterPublic mounts the token-only tracking + phish-report routes that are
// called by parties WITHOUT a session — the open-tracking pixel loaded by the
// phishing target's mail client, the click link and landing-page form from the
// target's browser, and the external phish-report webhook (S127-1, D4).
//
// The caller mounts this on a PUBLIC group (no AuthMiddleware, no CSRF, no
// license gate) with an IP rate limiter, and ONLY when the vaktaware module is
// enabled. Every handler here is already token-based (org resolved from the URL
// token, not the session), so no handler change is needed — only the mount.
func RegisterPublic(g *echo.Group, h *Handler) {
	g.GET("/t/:token", h.TrackClick)
	g.POST("/t/:token/submit", h.TrackFormSubmission)
	g.GET("/track/:token", h.TrackOpen) // open-tracking pixel (1×1 GIF)
	// Phish-report webhook — validated via org_token in the body, not a session.
	g.POST("/phish-report", h.ReceivePhishReport)
}
