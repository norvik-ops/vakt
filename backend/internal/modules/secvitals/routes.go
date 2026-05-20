// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package secvitals

import (
	"github.com/labstack/echo/v4"

	"github.com/matharnica/vakt/internal/auth"
	"github.com/matharnica/vakt/internal/license"
)

// Register wires ComplyKit routes under the provided group.
// The handler parameter is accepted so the caller controls construction and dependency injection.
func Register(g *echo.Group, h ...*Handler) {
	var handler *Handler
	if len(h) > 0 && h[0] != nil {
		handler = h[0]
	} else {
		// Fallback: construct a service-less handler for skeleton registration.
		// In production the caller should always pass a fully initialised handler.
		handler = &Handler{}
	}
	registerRoutes(g, handler)
}

// RegisterPublic wires the token-based auditor and supplier portal routes that require no Bearer auth.
func RegisterPublic(g *echo.Group, h *Handler) {
	g.GET("/auditor/:token", h.AuditorView)
	g.GET("/auditor/:token/export", h.AuditorExportBundle)
	// Supplier portal routes (Story 29.3) — public, no auth required.
	g.GET("/supplier/:token", h.PortalGetAssessment)
	g.POST("/supplier/:token/save", h.PortalSaveAnswers)
	g.POST("/supplier/:token/submit", h.PortalSubmitAssessment)
	g.POST("/supplier/:token/upload", h.PortalUploadFile)
}

// RegisterPolicyAcceptPublic wires the policy acceptance routes that require no Bearer auth.
// Call this on the top-level api group (e.g. /api/v1) with a rate limiter.
func RegisterPolicyAcceptPublic(g *echo.Group, h *Handler) {
	g.GET("/policy-accept/:token", h.GetAcceptanceInfo)
	g.POST("/policy-accept/:token", h.AcceptPolicy)
}

// registerRoutes is the internal wiring function — testable without public API churn.
func registerRoutes(g *echo.Group, h *Handler) {
	rw := auth.RequireRole("Admin", "SecurityAnalyst")

	// My Tasks — controls and risks assigned to the authenticated user.
	g.GET("/my-tasks", h.GetMyTasks)

	// Frameworks
	g.GET("/frameworks", h.ListFrameworks)
	g.GET("/frameworks/available", h.ListAvailableFrameworks)
	g.POST("/frameworks/install", h.InstallFrameworkPlugin, rw)
	// CRITICAL: static TISAX routes must be registered BEFORE /frameworks/:id to avoid route conflict.
	g.GET("/frameworks/tisax/iso-mapping", h.GetTISAXISOMapping, license.Require(license.FeatureTISAX))
	g.GET("/frameworks/tisax/coverage-after-iso", h.GetTISAXCoverageAfterISO, license.Require(license.FeatureTISAX))
	g.GET("/frameworks/:id", h.GetFrameworkByID)
	// CRITICAL: CRA-specific enable route must be registered BEFORE the generic /:name/enable
	// to gate CRA behind a Pro license — FeatureCRA must be active to enable the CRA framework.
	g.POST("/frameworks/CRA/enable", h.EnableFramework, rw, license.Require(license.FeatureCRA))
	g.POST("/frameworks/:name/enable", h.EnableFramework, rw)
	g.DELETE("/frameworks/:id", h.DeleteFramework, rw)
	// CRITICAL: overdue-reviews and export/xlsx are static paths and must be registered BEFORE /controls/:id
	// to prevent Echo from matching them as param routes.
	g.GET("/controls/overdue-reviews", h.ListOverdueControls)
	g.GET("/controls/export/xlsx", h.ExportControlsXLSX)
	g.GET("/controls/:id", h.GetControlByID)
	g.GET("/frameworks/:id/report", h.GetReadinessReport)
	g.GET("/frameworks/:id/export-pdf", h.ExportFrameworkPDF, license.Require(license.FeatureAuditPDF))
	g.GET("/frameworks/:id/gaps", h.GetGapAnalysis)
	// CRITICAL: tisax-controls, tisax-gaps, and tisax-report-pdf must be registered BEFORE /frameworks/:id/controls
	// to avoid route ambiguity with the :id parameter.
	g.GET("/frameworks/:id/tisax-controls", h.GetTISAXControls, license.Require(license.FeatureTISAX))
	g.GET("/frameworks/:id/tisax-gaps", h.GetTISAXGapAnalysis, license.Require(license.FeatureTISAX))
	g.GET("/frameworks/:id/tisax-report-pdf", h.ExportTISAXReportPDF, license.Require(license.FeatureTISAX))
	// CRITICAL: soa.pdf and audit-package.zip must be registered before /frameworks/:id/controls to avoid route conflict.
	g.GET("/frameworks/:id/soa.pdf", h.ExportSoAPDF, license.Require(license.FeatureAuditPDF))
	g.GET("/frameworks/:id/audit-package.zip", h.ExportAuditPackage, license.Require(license.FeatureAuditPDF))
	g.GET("/frameworks/:id/controls", h.ListControls)
	g.POST("/frameworks/:id/auditor-link", h.CreateAuditorLink, rw)

	// SoA (Statement of Applicability) — cross-framework view
	// CRITICAL: /soa.csv must be registered BEFORE /soa/:control_id to avoid route conflict.
	g.GET("/soa", h.GetSoA)
	g.GET("/soa.csv", h.GetSoACSV)
	g.PATCH("/soa/:control_id", h.UpdateSoAApplicability, rw)

	// DSGVO Art. 32 TOM coverage
	g.GET("/dsgvo/tom-coverage", h.GetDSGVOTOMCoverage)

	// Framework Mappings (Story 28.2)
	g.GET("/framework-mappings", h.ListFrameworkMappings)
	g.DELETE("/framework-mappings/:id", h.DeleteFrameworkMapping, rw)

	// Controls
	// CRITICAL: /controls/:id/mappings and /controls/:id/changelog must be registered BEFORE /controls/:id to avoid route conflict.
	g.GET("/controls/:id/mappings", h.GetControlMappings)
	g.GET("/controls/:id/changelog", h.GetControlChangelog)
	g.PATCH("/controls/:id", h.UpdateControl, rw)
	g.PATCH("/controls/:id/soa", h.UpdateControlSoAMetadata, rw)
	g.POST("/controls/:id/evidence", h.AddEvidence, rw)
	g.POST("/controls/:id/evidence/upload", h.UploadEvidence, rw)
	g.GET("/controls/:id/evidence", h.ListEvidence)
	// Evidence file attachments (Migration 074)
	g.POST("/controls/:id/evidence-files", h.UploadEvidenceFile, rw)
	g.GET("/controls/:id/evidence-files", h.ListEvidenceFilesByControl)
	g.GET("/evidence/:eid/files", h.ListEvidenceFiles)
	g.GET("/evidence-files/:fid/download", h.DownloadEvidenceFile)
	g.DELETE("/evidence-files/:fid", h.DeleteEvidenceFile, rw)
	g.POST("/controls/:id/collect", h.CollectEvidence, rw)
	g.GET("/controls/:id/export", h.ExportEvidenceBundle)
	g.GET("/controls/:id/tasks", h.ListControlTasks)
	g.POST("/controls/:id/tasks", h.CreateControlTask, rw)
	g.PATCH("/controls/:id/tasks/:taskId", h.UpdateControlTask, rw)
	g.DELETE("/controls/:id/tasks/:taskId", h.DeleteControlTask, rw)
	// Control review cycles (Migration 075)
	g.POST("/controls/:id/review", h.RecordControlReview, rw)
	g.GET("/controls/:id/reviews", h.ListControlReviews)

	// Maßnahmen-Katalog (control measures)
	g.GET("/controls/:id/measures", h.ListMeasures)
	g.POST("/controls/:id/measures", h.CreateMeasure, rw)
	g.PATCH("/controls/:id/measures/:mid", h.UpdateMeasure, rw)
	g.DELETE("/controls/:id/measures/:mid", h.DeleteMeasure, rw)

	// Evidence review
	g.POST("/evidence/:id/review", h.ReviewEvidence, rw)

	// Evidence expiry alert
	g.GET("/evidence/expiring", h.GetExpiringEvidence)

	// Auditor link management
	g.GET("/auditor-links", h.ListAuditorLinks)
	g.DELETE("/auditor-links/:id", h.RevokeAuditorLink, rw)

	// Risk Assessment
	// CRITICAL: /risks/export/xlsx must be registered BEFORE /risks/:id to avoid route conflict.
	g.GET("/risks/export/xlsx", h.ExportRisksXLSX)
	g.GET("/risks", h.ListRisks)
	g.POST("/risks", h.CreateRisk, rw)
	g.GET("/risks/:id", h.GetRisk)
	g.PATCH("/risks/:id", h.UpdateRisk, rw)
	// CRITICAL: /risks/:id/treatment must be registered BEFORE /risks/:id/controls to avoid route conflict.
	g.PATCH("/risks/:id/treatment", h.UpdateRiskTreatment, rw)

	// Risk ↔ Control Links
	g.GET("/risks/:id/controls", h.ListRiskControls)
	g.POST("/risks/:id/controls", h.LinkRiskControl, rw)
	g.DELETE("/risks/:id/controls/:controlId", h.UnlinkRiskControl, rw)

	// Incident Register
	g.GET("/incidents", h.ListIncidents)
	g.POST("/incidents", h.CreateIncident, rw)
	g.GET("/incidents/:id", h.GetIncident)
	g.PATCH("/incidents/:id", h.UpdateIncident, rw)
	// CRITICAL: nis2/enabled must be registered BEFORE incidents/:id to avoid route conflict
	g.GET("/nis2/enabled", h.NIS2ReportingEnabled, license.Require(license.FeatureNIS2Reporting))
	g.POST("/incidents/:id/mark-reported", h.MarkDeadlineReported, rw, license.Require(license.FeatureNIS2Reporting))
	g.POST("/incidents/:id/assess-reportability", h.AssessReportability, rw, license.Require(license.FeatureNIS2Reporting))
	// CRITICAL: incidents/:id/reports must be before incidents/:id/report-pdf to avoid ambiguity
	g.GET("/incidents/:id/reports", h.ListIncidentReports, license.Require(license.FeatureNIS2Reporting))
	g.POST("/incidents/:id/reports", h.GenerateIncidentReportForm, rw, license.Require(license.FeatureNIS2Reporting))
	g.GET("/incidents/:id/report-pdf", h.IncidentReportPDF, license.Require(license.FeatureAuditPDF))
	// Report download (separate resource path)
	g.GET("/incident-reports/:reportId/pdf", h.DownloadIncidentReportPDF, license.Require(license.FeatureAuditPDF))

	// Supplier Register — Pro feature
	g.GET("/suppliers", h.ListSuppliers, license.Require(license.FeatureSupplierPortal))
	g.POST("/suppliers", h.CreateSupplier, rw, license.Require(license.FeatureSupplierPortal))
	// CRITICAL: static paths must be registered BEFORE /suppliers/:id to avoid route conflict
	g.GET("/suppliers/export", h.ExportSuppliers, license.Require(license.FeatureSupplierPortal))
	g.POST("/suppliers/import-csv", h.ImportSuppliersCSV, rw, license.Require(license.FeatureSupplierPortal))
	g.GET("/suppliers/:id", h.GetSupplier, license.Require(license.FeatureSupplierPortal))
	g.PATCH("/suppliers/:id", h.UpdateSupplier, rw, license.Require(license.FeatureSupplierPortal))
	g.DELETE("/suppliers/:id", h.DeleteSupplier, rw, license.Require(license.FeatureSupplierPortal))
	g.GET("/suppliers/:id/incidents", h.GetSupplierIncidents, license.Require(license.FeatureSupplierPortal))
	g.POST("/suppliers/:id/risks", h.LinkSupplierRisk, rw, license.Require(license.FeatureSupplierPortal))
	g.GET("/suppliers/:id/risks", h.ListSupplierRisks, license.Require(license.FeatureSupplierPortal))
	g.DELETE("/suppliers/:id/risks/:riskId", h.UnlinkSupplierRisk, rw, license.Require(license.FeatureSupplierPortal))
	// CRITICAL: static paths under /suppliers/:id must come before the bare /suppliers/:id param routes
	g.GET("/suppliers/:id/status", h.GetSupplierStatus, license.Require(license.FeatureSupplierPortal))

	// Supplier assessments (Story 29.3) — Pro feature
	g.POST("/suppliers/:id/assessments", h.CreateSupplierAssessment, rw, license.Require(license.FeatureSupplierPortal))
	g.GET("/suppliers/:id/assessments", h.ListSupplierAssessments, license.Require(license.FeatureSupplierPortal))

	// Assessment routes — CRITICAL: static sub-paths before bare :id to avoid route conflicts
	g.GET("/assessments/:id/answers", h.GetAssessmentAnswers)
	g.GET("/assessments/:id/report-pdf", h.GetAssessmentReportPDF, license.Require(license.FeatureAuditPDF))
	g.GET("/assessments/:id", h.GetAssessment)
	g.PATCH("/assessments/:id", h.UpdateAssessment, rw)
	g.PATCH("/assessments/:id/answers/:aid", h.ReviewAnswer, rw)

	// AI System Inventory — EU AI Act Pro feature
	g.GET("/ai-systems", h.ListAISystems, license.Require(license.FeatureEUAIAct))
	g.POST("/ai-systems", h.CreateAISystem, rw, license.Require(license.FeatureEUAIAct))
	// CRITICAL: static sub-paths before bare :id to avoid route conflicts
	g.GET("/ai-systems/:id/classifications", h.ListAIClassifications, license.Require(license.FeatureEUAIAct))
	g.POST("/ai-systems/:id/classify", h.ClassifyAISystem, rw, license.Require(license.FeatureEUAIAct))
	// CRITICAL: documentation/versions and documentation/export-pdf before documentation
	g.GET("/ai-systems/:id/documentation/versions", h.ListAIDocumentationVersions, license.Require(license.FeatureEUAIAct))
	g.GET("/ai-systems/:id/documentation/export-pdf", h.ExportAIDocumentationPDF, license.Require(license.FeatureEUAIAct))
	g.GET("/ai-systems/:id/documentation", h.GetLatestAIDocumentation, license.Require(license.FeatureEUAIAct))
	g.POST("/ai-systems/:id/documentation", h.SaveAIDocumentation, rw, license.Require(license.FeatureEUAIAct))
	g.GET("/ai-systems/:id", h.GetAISystem, license.Require(license.FeatureEUAIAct))
	g.PATCH("/ai-systems/:id", h.UpdateAISystem, rw, license.Require(license.FeatureEUAIAct))
	g.DELETE("/ai-systems/:id", h.DeleteAISystem, rw, license.Require(license.FeatureEUAIAct))

	// Org sector + authority directory (Story 31.4) — EU AI Act / NIS2 Pro feature
	g.GET("/org-sector", h.GetOrgSector, license.Require(license.FeatureEUAIAct))
	g.PATCH("/org-sector", h.UpdateOrgSector, rw, license.Require(license.FeatureEUAIAct))
	g.GET("/authorities", h.ListAuthorities, license.Require(license.FeatureEUAIAct))
	g.GET("/org-authorities", h.GetOrgAuthorities, license.Require(license.FeatureEUAIAct))

	// EU AI Act Dashboard (Story 30.4)
	// CRITICAL: eu-ai-act/report-pdf before eu-ai-act/dashboard to avoid route ambiguity
	g.GET("/eu-ai-act/report-pdf", h.GetEUAIActReportPDF, license.Require(license.FeatureEUAIAct))
	g.GET("/eu-ai-act/dashboard", h.GetEUAIActDashboard, license.Require(license.FeatureEUAIAct))

	// Policy Management
	g.GET("/policies", h.ListPolicies)
	g.POST("/policies", h.CreatePolicy, rw)
	// CRITICAL: /policies/generate-draft must be registered BEFORE /policies/:id to avoid route conflict.
	// /policies/generate-draft nutzt AI Copilot — Community-Feature seit v0.6.x.
	g.POST("/policies/generate-draft", h.GeneratePolicyDraft, rw)
	// CRITICAL: static sub-paths before bare :id to avoid route conflicts
	// CRITICAL: acceptance-campaigns/:cid/stats and /requests must be before /acceptance-campaigns
	g.GET("/policies/acceptance-campaigns/:cid/stats", h.GetCampaignStats)
	g.GET("/policies/acceptance-campaigns/:cid/requests", h.ListCampaignRequests)
	// CRITICAL: /policies/:id/versions/:v and /policies/:id/versions must be before /policies/:id
	g.GET("/policies/:id/versions", h.ListPolicyVersions)
	g.GET("/policies/:id/versions/:v", h.GetPolicyVersion)
	g.GET("/policies/:id", h.GetPolicy)
	g.PATCH("/policies/:id", h.UpdatePolicy, rw)
	g.POST("/policies/:id/acceptance-campaigns", h.CreateAcceptanceCampaign, rw)
	g.GET("/policies/:id/acceptance-campaigns", h.ListAcceptanceCampaigns)

	// Internal Audit Records
	g.GET("/audits", h.ListAuditRecords)
	g.POST("/audits", h.CreateAuditRecord, rw)
	g.GET("/audits/:id", h.GetAuditRecord)
	g.PATCH("/audits/:id", h.UpdateAuditRecord, rw)

	// Policy Templates
	g.GET("/policy-templates", h.ListPolicyTemplates)
	g.POST("/policy-templates/:id/apply", h.CreatePolicyFromTemplate, rw)

	// Resilience Tests (DORA Art. 24-27) — DORA Pro feature
	g.GET("/resilience-tests", h.ListResilienceTests, license.Require(license.FeatureDORA))
	g.POST("/resilience-tests", h.CreateResilienceTest, rw, license.Require(license.FeatureDORA))
	g.GET("/resilience-tests/:id", h.GetResilienceTest, license.Require(license.FeatureDORA))
	g.PATCH("/resilience-tests/:id", h.UpdateResilienceTest, rw, license.Require(license.FeatureDORA))
	g.DELETE("/resilience-tests/:id", h.DeleteResilienceTest, rw, license.Require(license.FeatureDORA))
	g.POST("/resilience-tests/:id/attachment", h.UploadResilienceTestAttachment, rw, license.Require(license.FeatureDORA))

	// DORA Dashboard (Story 27.5)
	g.GET("/dora/dashboard", h.GetDORADashboard, license.Require(license.FeatureDORA))
	g.GET("/dora/report-pdf", h.GetDORAPDF, license.Require(license.FeatureDORA))

	// Executive Summary PDF — cross-framework compliance overview
	// CRITICAL: /reports/executive-summary is a static path; registered before any dynamic /reports/:id routes.
	g.GET("/reports/executive-summary", h.GetExecutiveSummaryPDF, license.Require(license.FeatureAuditPDF))

	// CCM (Continuous Control Monitoring)
	g.GET("/ccm/checks", h.ListCCMChecks)
	g.POST("/ccm/checks", h.CreateCCMCheck, rw)
	g.DELETE("/ccm/checks/:id", h.DeleteCCMCheck, rw)
	g.PATCH("/ccm/checks/:id/toggle", h.ToggleCCMCheck, rw)
	g.POST("/ccm/checks/:id/run", h.TriggerCCMCheck, rw)
	g.GET("/ccm/checks/:id/results", h.ListCCMResults)

	// Questionnaire Builder (Story 29.2)
	// CRITICAL: /questionnaires/templates must be registered BEFORE /questionnaires/:id
	// and /questionnaires/:id/questions/reorder must be registered BEFORE /questionnaires/:id/questions/:qid
	g.GET("/questionnaires/templates", h.ListTemplates)
	g.GET("/questionnaires", h.ListQuestionnaires)
	g.POST("/questionnaires", h.CreateQuestionnaire, rw)
	g.GET("/questionnaires/:id", h.GetQuestionnaire)
	g.PATCH("/questionnaires/:id", h.UpdateQuestionnaire, rw)
	g.DELETE("/questionnaires/:id", h.DeleteQuestionnaire, rw)
	g.POST("/questionnaires/:id/questions/reorder", h.ReorderQuestions, rw)
	g.POST("/questionnaires/:id/questions", h.AddQuestion, rw)
	g.PATCH("/questionnaires/:id/questions/:qid", h.UpdateQuestion, rw)
	g.DELETE("/questionnaires/:id/questions/:qid", h.DeleteQuestion, rw)

	// Collaborative Tasks & Comments
	// NOTE: Routes use /collab-tasks and /comments to avoid conflicts with the existing
	// simple checklist /controls/:id/tasks routes.
	for _, entity := range []string{"controls", "risks", "incidents", "policies", "audits"} {
		et := urlEntityType[entity]
		g.GET("/"+entity+"/:id/collab-tasks", h.listTasksFor(et))
		g.POST("/"+entity+"/:id/collab-tasks", h.createTaskFor(et), rw)
		g.GET("/"+entity+"/:id/comments", h.listCommentsFor(et))
		g.POST("/"+entity+"/:id/comments", h.createCommentFor(et), rw)
	}
	g.PATCH("/collab-tasks/:tid", h.UpdateCollabTask, rw)
	g.DELETE("/collab-tasks/:tid", h.DeleteCollabTask, rw)
	g.DELETE("/comments/:cid", h.DeleteCollabComment, rw)

	// Audit Milestones / Certification Timeline (Migration 092)
	// CRITICAL: /milestones/next must be registered BEFORE /milestones/:id to avoid route conflict.
	g.GET("/milestones/next", h.GetNextMilestone)
	g.GET("/milestones", h.ListMilestones)
	g.POST("/milestones", h.CreateMilestone, rw)
	g.PUT("/milestones/:id", h.UpdateMilestone, rw)
	g.DELETE("/milestones/:id", h.DeleteMilestone, rw)

	// Score history — daily compliance trend data (Migration 093)
	g.GET("/score-history", h.GetScoreHistory)

	// CAPA (Corrective and Preventive Actions)
	g.GET("/capas", h.ListCAPAs)
	g.POST("/capas", h.CreateCAPA, rw)
	g.GET("/capas/:id", h.GetCAPA)
	g.PATCH("/capas/:id", h.UpdateCAPA, rw)
	g.DELETE("/capas/:id", h.DeleteCAPA, rw)
	// CRITICAL: /audits/:id/capas and /incidents/:id/capas must be registered BEFORE the bare
	// /audits/:id and /incidents/:id to avoid Echo route conflicts.
	g.GET("/audits/:id/capas", h.ListCAPAsForAudit)
	g.POST("/audits/:id/capas", h.CreateCAPAFromAudit, rw)
	g.GET("/incidents/:id/capas", h.ListCAPAsForIncident)
	g.POST("/incidents/:id/capas", h.CreateCAPAFromIncident, rw)

	// 4-Augen-Prinzip — Control status change approvals (Migration 092)
	// CRITICAL: static paths must be registered BEFORE param routes.
	// /approvals/count must come before /approvals/:id/approve and /approvals/:id/reject.
	g.POST("/controls/:id/approval-request", h.RequestControlApproval, rw)
	g.GET("/approvals", h.ListPendingApprovals)
	g.GET("/approvals/count", h.CountPendingApprovals)
	g.POST("/approvals/:id/approve", h.ApproveApproval, rw)
	g.POST("/approvals/:id/reject", h.RejectApproval, rw)

	// Org approval setting (admin-only toggle)
	g.GET("/org/approval-setting", h.GetApprovalSetting)
	g.PUT("/org/approval-setting", h.UpdateApprovalSetting, rw)

	registerAccessReviewRoutes(g, h)
	registerExceptionRoutes(g, h)
}

// RegisterAuditor registers read-only routes for external auditors.
// The provided group must already be behind the AuditorAuth middleware.
func RegisterAuditor(g *echo.Group, h *Handler) {
	g.GET("/frameworks", h.ListFrameworks)
	g.GET("/frameworks/:id", h.GetFrameworkByID)
	g.GET("/frameworks/:id/controls", h.ListControls)
	// SoA PDF export requires Pro FeatureAuditPDF — basic auditor view remains Community.
	g.GET("/frameworks/:id/soa.pdf", h.ExportSoAPDF, license.Require(license.FeatureAuditPDF))
	g.GET("/risks", h.ListRisks)
	g.GET("/incidents", h.ListIncidents)
	g.GET("/policies", h.ListPolicies)
	g.GET("/audits", h.ListAuditRecords)
}
