// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"github.com/labstack/echo/v4"

	"github.com/matharnica/vakt/internal/auth"
	"github.com/matharnica/vakt/internal/shared/platform/features"
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
	g.GET("/frameworks/mapping-coverage", h.GetMappingCoverage)
	g.GET("/frameworks/tisax/iso-mapping", h.GetTISAXISOMapping, features.Require(features.FeatureTISAX))
	g.GET("/frameworks/tisax/coverage-after-iso", h.GetTISAXCoverageAfterISO, features.Require(features.FeatureTISAX))
	g.GET("/frameworks/:id", h.GetFrameworkByID)
	// CRITICAL: CRA-specific enable route must be registered BEFORE the generic /:name/enable
	// to gate CRA behind a Pro license — FeatureCRA must be active to enable the CRA framework.
	g.POST("/frameworks/CRA/enable", h.EnableFramework, rw, features.Require(features.FeatureCRA))
	g.POST("/frameworks/:name/enable", h.EnableFramework, rw)
	// CRITICAL: dora/variant must be registered BEFORE /:id to avoid route collision.
	g.PUT("/frameworks/dora/variant", h.SwitchDORAVariant, rw)
	g.DELETE("/frameworks/:id", h.DeleteFramework, rw)
	// CRITICAL: overdue-reviews and export/xlsx are static paths and must be registered BEFORE /controls/:id
	// to prevent Echo from matching them as param routes.
	g.GET("/controls/overdue-reviews", h.ListOverdueControls)
	g.GET("/controls/export/xlsx", h.ExportControlsXLSX)
	g.GET("/controls/:id", h.GetControlByID)
	g.GET("/frameworks/:id/report", h.GetReadinessReport)
	g.GET("/frameworks/:id/export-pdf", h.ExportFrameworkPDF, features.Require(features.FeatureAuditPDF))
	g.GET("/frameworks/:id/gaps", h.GetGapAnalysis)
	// CRITICAL: tisax-controls, tisax-gaps, and tisax-report-pdf must be registered BEFORE /frameworks/:id/controls
	// to avoid route ambiguity with the :id parameter.
	g.GET("/frameworks/:id/tisax-controls", h.GetTISAXControls, features.Require(features.FeatureTISAX))
	g.GET("/frameworks/:id/tisax-gaps", h.GetTISAXGapAnalysis, features.Require(features.FeatureTISAX))
	g.GET("/frameworks/:id/tisax-report-pdf", h.ExportTISAXReportPDF, features.Require(features.FeatureTISAX))
	// CRITICAL: soa.pdf and audit-package.zip must be registered before /frameworks/:id/controls to avoid route conflict.
	g.GET("/frameworks/:id/soa.pdf", h.ExportSoAPDF, features.Require(features.FeatureAuditPDF))
	g.GET("/frameworks/:id/audit-package.zip", h.ExportAuditPackage, features.Require(features.FeatureAuditPDF))
	g.GET("/frameworks/:id/controls", h.ListControls)
	g.GET("/frameworks/:id/implementation-path", h.GetImplementationPath)
	g.POST("/frameworks/:id/auditor-link", h.CreateAuditorLink, rw)

	// SoA (Statement of Applicability) — cross-framework view (legacy, column-based)
	// CRITICAL: /soa.csv must be registered BEFORE /soa/:control_id to avoid route conflict.
	g.GET("/soa", h.GetSoA)
	g.GET("/soa.csv", h.GetSoACSV)
	g.PATCH("/soa/:control_id", h.UpdateSoAApplicability, rw)

	// SoA dedicated tables (S68-1) — ISO 27001:2022 Annex A, versioned, approval workflow
	// CRITICAL: static paths (/soa/init, /soa/approve, /soa/versions, /soa/summary, /soa/export, /soa/entries)
	// MUST be registered BEFORE the param route /soa/entries/:control_ref.
	g.POST("/soa/init", h.InitDedicatedSoA, rw)
	g.POST("/soa/approve", h.ApproveDedicatedSoA, rw)
	g.GET("/soa/versions", h.GetDedicatedSoAVersions)
	g.GET("/soa/summary", h.GetDedicatedSoASummary)
	g.GET("/soa/export", h.ExportDedicatedSoA)
	g.GET("/soa/entries", h.GetDedicatedSoAEntries)
	g.GET("/soa/entries/:control_ref", h.GetDedicatedSoAEntry)
	g.PUT("/soa/entries/:control_ref", h.UpdateDedicatedSoAEntry, rw)

	// Interessierte Parteien — ISO 27001 Clause 4.2 (S68-3)
	// CRITICAL: static sub-paths (/seed-defaults, /export) must be before /:id
	g.GET("/interested-parties", h.ListInterestedParties)
	g.POST("/interested-parties", h.CreateInterestedParty, rw)
	g.POST("/interested-parties/seed-defaults", h.SeedDefaultInterestedParties, rw)
	g.GET("/interested-parties/export", h.ExportInterestedPartiesPDF)
	g.PUT("/interested-parties/:id", h.UpdateInterestedParty, rw)
	g.DELETE("/interested-parties/:id", h.DeleteInterestedParty, rw)

	// Audit-Programm — ISO 27001 Clause 9.2 (S68-4)
	// CRITICAL: /audit-program/summary must be before /audit-program/:id
	g.GET("/audit-plans", h.ListAuditPlans)
	g.POST("/audit-plans", h.CreateAuditPlan, rw)
	g.PUT("/audit-plans/:id", h.UpdateAuditPlan, rw)
	g.GET("/audit-program/summary", h.GetAuditProgramSummary)
	g.GET("/audit-program", h.ListAuditProgramAudits)
	g.POST("/audit-program", h.CreateAuditProgramAudit, rw)
	g.GET("/audit-program/:id", h.GetAuditProgramAudit)
	g.PUT("/audit-program/:id", h.UpdateAuditProgramAudit, rw)
	g.PATCH("/audit-program/:id/complete", h.CompleteAuditProgramAudit, rw)
	g.GET("/audit-program/:id/findings", h.ListAuditFindings)
	g.POST("/audit-program/:id/findings", h.CreateAuditFinding, rw)
	g.GET("/audit-program/:id/export", h.ExportAuditProgramReport)

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
	g.DELETE("/risks/:id", h.DeleteRisk, rw)
	// CRITICAL: /risks/:id/treatment, /risks/:id/residual and /risks/:id/accept must be registered
	// BEFORE /risks/:id/controls to avoid route conflict.
	g.PATCH("/risks/:id/treatment", h.UpdateRiskTreatment, rw)
	// S61-4: Residualrisiko-Berechnung
	g.PATCH("/risks/:id/residual", h.UpdateRiskResidualFields, rw)
	g.POST("/risks/:id/accept", h.AcceptRisk, rw)

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
	g.GET("/nis2/enabled", h.NIS2ReportingEnabled, features.Require(features.FeatureNIS2Reporting))
	g.POST("/incidents/:id/mark-reported", h.MarkDeadlineReported, rw, features.Require(features.FeatureNIS2Reporting))
	g.POST("/incidents/:id/assess-reportability", h.AssessReportability, rw, features.Require(features.FeatureNIS2Reporting))
	// S39-1: BSI-Meldepflicht-Klassifizierung (new 3-question wizard with "probably/none/unclear" output)
	g.POST("/incidents/:id/classify-reporting", h.ClassifyReportingObligation, rw, features.Require(features.FeatureNIS2Reporting))
	// CRITICAL: incidents/:id/reports must be before incidents/:id/report-pdf to avoid ambiguity
	g.GET("/incidents/:id/reports", h.ListIncidentReports, features.Require(features.FeatureNIS2Reporting))
	g.POST("/incidents/:id/reports", h.GenerateIncidentReportForm, rw, features.Require(features.FeatureNIS2Reporting))
	g.GET("/incidents/:id/report-pdf", h.IncidentReportPDF, features.Require(features.FeatureAuditPDF))
	// Report download (separate resource path)
	g.GET("/incident-reports/:reportId/pdf", h.DownloadIncidentReportPDF, features.Require(features.FeatureAuditPDF))
	// S67-1: NIS2 Art.23 stage-based reporting workflow
	g.POST("/incidents/:id/nis2/assess", h.NIS2AssessReportability, rw, features.Require(features.FeatureNIS2Reporting))
	g.GET("/incidents/:id/nis2-status", h.NIS2Status, features.Require(features.FeatureNIS2Reporting))
	g.POST("/incidents/:id/nis2/submit/:stage", h.NIS2SubmitStage, rw, features.Require(features.FeatureNIS2Reporting))
	g.GET("/authority-contacts", h.ListAuthorityContacts)
	g.POST("/authority-contacts", h.CreateAuthorityContact, rw)

	// Supplier Register — Pro feature
	g.GET("/suppliers", h.ListSuppliers, features.Require(features.FeatureSupplierPortal))
	g.POST("/suppliers", h.CreateSupplier, rw, features.Require(features.FeatureSupplierPortal))
	// CRITICAL: static paths must be registered BEFORE /suppliers/:id to avoid route conflict
	g.GET("/suppliers/export", h.ExportSuppliers, features.Require(features.FeatureSupplierPortal))
	g.POST("/suppliers/import-csv", h.ImportSuppliersCSV, rw, features.Require(features.FeatureSupplierPortal))
	g.GET("/suppliers/:id", h.GetSupplier, features.Require(features.FeatureSupplierPortal))
	g.PATCH("/suppliers/:id", h.UpdateSupplier, rw, features.Require(features.FeatureSupplierPortal))
	g.DELETE("/suppliers/:id", h.DeleteSupplier, rw, features.Require(features.FeatureSupplierPortal))
	g.GET("/suppliers/:id/incidents", h.GetSupplierIncidents, features.Require(features.FeatureSupplierPortal))
	g.POST("/suppliers/:id/risks", h.LinkSupplierRisk, rw, features.Require(features.FeatureSupplierPortal))
	g.GET("/suppliers/:id/risks", h.ListSupplierRisks, features.Require(features.FeatureSupplierPortal))
	g.DELETE("/suppliers/:id/risks/:riskId", h.UnlinkSupplierRisk, rw, features.Require(features.FeatureSupplierPortal))
	// CRITICAL: static paths under /suppliers/:id must come before the bare /suppliers/:id param routes
	g.GET("/suppliers/:id/status", h.GetSupplierStatus, features.Require(features.FeatureSupplierPortal))

	// Supplier assessments (Story 29.3) — Pro feature
	g.POST("/suppliers/:id/assessments", h.CreateSupplierAssessment, rw, features.Require(features.FeatureSupplierPortal))
	g.GET("/suppliers/:id/assessments", h.ListSupplierAssessments, features.Require(features.FeatureSupplierPortal))

	// Assessment routes — CRITICAL: static sub-paths before bare :id to avoid route conflicts
	g.GET("/assessments/:id/answers", h.GetAssessmentAnswers)
	g.GET("/assessments/:id/report-pdf", h.GetAssessmentReportPDF, features.Require(features.FeatureAuditPDF))
	g.GET("/assessments/:id", h.GetAssessment)
	g.PATCH("/assessments/:id", h.UpdateAssessment, rw)
	g.PATCH("/assessments/:id/answers/:aid", h.ReviewAnswer, rw)

	// AI System Inventory — EU AI Act Pro feature
	g.GET("/ai-systems", h.ListAISystems, features.Require(features.FeatureEUAIAct))
	g.POST("/ai-systems", h.CreateAISystem, rw, features.Require(features.FeatureEUAIAct))
	// CRITICAL: static sub-paths before bare :id to avoid route conflicts
	g.GET("/ai-systems/:id/classifications", h.ListAIClassifications, features.Require(features.FeatureEUAIAct))
	g.POST("/ai-systems/:id/classify", h.ClassifyAISystem, rw, features.Require(features.FeatureEUAIAct))
	// CRITICAL: documentation/versions and documentation/export-pdf before documentation
	g.GET("/ai-systems/:id/documentation/versions", h.ListAIDocumentationVersions, features.Require(features.FeatureEUAIAct))
	g.GET("/ai-systems/:id/documentation/export-pdf", h.ExportAIDocumentationPDF, features.Require(features.FeatureEUAIAct))
	g.GET("/ai-systems/:id/documentation", h.GetLatestAIDocumentation, features.Require(features.FeatureEUAIAct))
	g.POST("/ai-systems/:id/documentation", h.SaveAIDocumentation, rw, features.Require(features.FeatureEUAIAct))
	g.GET("/ai-systems/:id", h.GetAISystem, features.Require(features.FeatureEUAIAct))
	g.PATCH("/ai-systems/:id", h.UpdateAISystem, rw, features.Require(features.FeatureEUAIAct))
	g.DELETE("/ai-systems/:id", h.DeleteAISystem, rw, features.Require(features.FeatureEUAIAct))

	// Org sector + authority directory (Story 31.4) — EU AI Act / NIS2 Pro feature
	g.GET("/org-sector", h.GetOrgSector, features.Require(features.FeatureEUAIAct))
	g.PATCH("/org-sector", h.UpdateOrgSector, rw, features.Require(features.FeatureEUAIAct))
	g.GET("/authorities", h.ListAuthorities, features.Require(features.FeatureEUAIAct))
	g.GET("/org-authorities", h.GetOrgAuthorities, features.Require(features.FeatureEUAIAct))

	// EU AI Act Dashboard (Story 30.4)
	// CRITICAL: eu-ai-act/report-pdf before eu-ai-act/dashboard to avoid route ambiguity
	g.GET("/eu-ai-act/report-pdf", h.GetEUAIActReportPDF, features.Require(features.FeatureEUAIAct))
	g.GET("/eu-ai-act/dashboard", h.GetEUAIActDashboard, features.Require(features.FeatureEUAIAct))

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
	g.GET("/resilience-tests", h.ListResilienceTests, features.Require(features.FeatureDORA))
	g.POST("/resilience-tests", h.CreateResilienceTest, rw, features.Require(features.FeatureDORA))
	g.GET("/resilience-tests/:id", h.GetResilienceTest, features.Require(features.FeatureDORA))
	g.PATCH("/resilience-tests/:id", h.UpdateResilienceTest, rw, features.Require(features.FeatureDORA))
	g.DELETE("/resilience-tests/:id", h.DeleteResilienceTest, rw, features.Require(features.FeatureDORA))
	g.POST("/resilience-tests/:id/attachment", h.UploadResilienceTestAttachment, rw, features.Require(features.FeatureDORA))
	g.POST("/resilience-tests/:id/link-evidence", h.LinkResilienceTestAsEvidence, rw, features.Require(features.FeatureDORA))

	// DORA Dashboard (Story 27.5)
	g.GET("/dora/dashboard", h.GetDORADashboard, features.Require(features.FeatureDORA))
	g.GET("/dora/report-pdf", h.GetDORAPDF, features.Require(features.FeatureDORA))

	// DORA IKT-Drittanbieter-Register (Art. 28-44 / S38-1)
	// CRITICAL: static sub-paths (/third-parties) before param routes.
	g.GET("/dora/third-parties", h.ListDORAThirdParties, features.Require(features.FeatureDORA))
	g.POST("/dora/third-parties", h.CreateDORAThirdParty, rw, features.Require(features.FeatureDORA))
	g.GET("/dora/third-parties/:id", h.GetDORAThirdParty, features.Require(features.FeatureDORA))
	g.PATCH("/dora/third-parties/:id", h.UpdateDORAThirdParty, rw, features.Require(features.FeatureDORA))
	g.DELETE("/dora/third-parties/:id", h.DeleteDORAThirdParty, rw, features.Require(features.FeatureDORA))
	g.POST("/dora/third-parties/:id/controls", h.LinkDORAThirdPartyControl, rw, features.Require(features.FeatureDORA))
	g.DELETE("/dora/third-parties/:id/controls/:controlId", h.UnlinkDORAThirdPartyControl, rw, features.Require(features.FeatureDORA))

	// Executive Summary PDF — cross-framework compliance overview
	// CRITICAL: /reports/executive-summary is a static path; registered before any dynamic /reports/:id routes.
	g.GET("/reports/executive-summary", h.GetExecutiveSummaryPDF, features.Require(features.FeatureAuditPDF))

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
	// CRITICAL: static sub-paths (/capas/:id/nc-fields and /capas/:id/effectiveness-check)
	// must be registered BEFORE the bare /capas/:id routes above.
	// (Echo route registration order: most-specific first for same-prefix paths.)
	g.PATCH("/capas/:id/nc-fields", h.UpdateCAPANCFields, rw)
	g.POST("/capas/:id/effectiveness-check", h.CompleteEffectivenessCheck, rw)
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

	// BCP / Notfallhandbuch (S60)
	// CRITICAL: /bcp/plans/:id/tests and /bcp/plans/:id/evidence must be registered BEFORE /bcp/plans/:id
	g.GET("/bcp/plans", h.ListBCPPlans)
	g.POST("/bcp/plans", h.CreateBCPPlan, rw)
	g.GET("/bcp/plans/:id/tests", h.ListBCPTests)
	g.POST("/bcp/plans/:id/tests", h.AddBCPTest, rw)
	g.POST("/bcp/plans/:id/evidence", h.LinkBCPPlanAsEvidence, rw)
	g.GET("/bcp/plans/:id", h.GetBCPPlan)
	g.PATCH("/bcp/plans/:id", h.UpdateBCPPlan, rw)
	g.DELETE("/bcp/plans/:id", h.DeleteBCPPlan, rw)

	// Schutzbedarfsfeststellung (S60)
	// CRITICAL: sub-resource routes must be registered BEFORE /:id to avoid shadowing
	g.GET("/protection-needs/assessments", h.ListProtectionNeedAssessments)
	g.POST("/protection-needs/assessments", h.CreateProtectionNeedAssessment, rw)
	g.POST("/protection-needs/assessments/:id/finalize", h.FinalizeProtectionNeedAssessment, rw)
	g.PATCH("/protection-needs/assessments/:id/asset-link", h.LinkPNAAsset, rw)
	g.GET("/protection-needs/assessments/:id", h.GetProtectionNeedAssessment)
	g.PATCH("/protection-needs/assessments/:id", h.UpdateProtectionNeedAssessment, rw)
	g.DELETE("/protection-needs/assessments/:id", h.DeleteProtectionNeedAssessment, rw)

	// ISMS Scope (S61-1)
	// CRITICAL: /isms-scope/versions and /isms-scope/approve and /isms-scope/export-pdf
	// must be registered BEFORE /isms-scope to avoid route conflicts.
	g.GET("/isms-scope/versions", h.ListISMSScopeVersions)
	g.POST("/isms-scope/approve", h.ApproveISMSScope, rw)
	g.GET("/isms-scope/export-pdf", h.ExportISMSScopePDF)
	g.GET("/isms-scope", h.GetISMSScope)
	g.POST("/isms-scope", h.CreateOrUpdateISMSScope, rw)

	// Pentest Tracking (S61-6)
	// CRITICAL: sub-resource routes before /:id to avoid shadowing
	g.GET("/pentests", h.ListPentests)
	g.POST("/pentests", h.CreatePentest, rw)
	g.GET("/pentests/:id", h.GetPentest)
	g.PATCH("/pentests/:id", h.UpdatePentest, rw)
	g.DELETE("/pentests/:id", h.DeletePentest, rw)
	g.POST("/pentests/:id/report", h.UploadPentestReport, rw)
	g.POST("/pentests/:id/link-evidence", h.LinkPentestAsEvidence, rw)

	// BSI Modeling (S61-5)
	// CRITICAL: static paths before /:id to avoid route conflict.
	g.GET("/bsi-modeling", h.GetBSIModelingMatrix)
	g.POST("/bsi-modeling", h.CreateBSIModeling, rw)
	g.GET("/bsi-modeling/stats", h.GetBSIModelingStats)
	g.GET("/bsi-modeling/suggestions", h.GetBSIBausteinSuggestions)
	g.GET("/bsi-modeling/export-pdf", h.ExportBSIModelingPDF)
	g.GET("/bsi-modeling/export-xlsx", h.ExportBSIModelingXLSX)
	g.PATCH("/bsi-modeling/:id", h.UpdateBSIModeling, rw)
	g.DELETE("/bsi-modeling/:id", h.DeleteBSIModeling, rw)

	// Management Reviews (S61-2)
	// CRITICAL: sub-resource routes before /:id to avoid route conflicts
	g.GET("/management-reviews", h.ListManagementReviews)
	g.POST("/management-reviews", h.CreateManagementReview, rw)
	g.GET("/management-reviews/:id", h.GetManagementReview)
	g.PATCH("/management-reviews/:id/inputs", h.UpdateManagementReviewInputs, rw)
	g.PATCH("/management-reviews/:id/outputs", h.UpdateManagementReviewOutputs, rw)
	g.POST("/management-reviews/:id/approve", h.ApproveManagementReview, rw)
	g.GET("/management-reviews/:id/export-pdf", h.ExportManagementReviewPDF)

	// ISMS KPI Dashboard (S61-7)
	// CRITICAL: /kpi-dashboard/export-pdf before /kpi-dashboard to avoid route conflict.
	g.GET("/kpi-dashboard/export-pdf", h.ExportKPIReportPDF)
	g.GET("/kpi-dashboard", h.GetKPIDashboard)

	// S67-4: Evidence staleness compliance score
	g.GET("/compliance-score", h.GetComplianceScore)
	g.GET("/controls/stale", h.ListStaleControls)

	// S67-6: Kryptographie-Schlüssel-Lifecycle (A.8.24)
	g.GET("/crypto-keys", h.ListCryptoKeys)
	g.POST("/crypto-keys", h.CreateCryptoKey, rw)
	g.POST("/crypto-keys/:id/rotate", h.RotateCryptoKey, rw)
	g.DELETE("/crypto-keys/:id", h.DeleteCryptoKey, rw)

	// S74-1: IT-Grundschutz-Check — Zielobjekte (Strukturanalyse)
	// CRITICAL: static sub-paths (/check/summary, /check/bulk, /risks/summary, /modeling) must be
	// registered BEFORE param routes (/check/:anforderungId, /risks/:riskId) to avoid Echo conflicts.
	g.GET("/bsi/target-objects", h.ListBSITargetObjects)
	g.POST("/bsi/target-objects", h.CreateBSITargetObject, rw)
	g.GET("/bsi/target-objects/:id/check/summary", h.GetBSICheckSummary)
	g.POST("/bsi/target-objects/:id/check/bulk", h.BulkSetBSICheckResults, rw)
	g.GET("/bsi/target-objects/:id/check", h.GetBSICheckSheet)
	g.PUT("/bsi/target-objects/:id/check/:anforderungId", h.SetBSICheckResult, rw)
	g.GET("/bsi/target-objects/:id/risks/summary", h.GetBSIRiskSummary)
	g.GET("/bsi/target-objects/:id/risks", h.ListBSIRisks)
	g.POST("/bsi/target-objects/:id/risks", h.CreateBSIRisk, rw)
	g.PUT("/bsi/target-objects/:id/risks/:riskId", h.UpdateBSIRisk, rw)
	g.DELETE("/bsi/target-objects/:id/risks/:riskId", h.DeleteBSIRisk, rw)
	g.POST("/bsi/target-objects/:id/modeling", h.AssignBausteinToTargetObject, rw)
	g.DELETE("/bsi/target-objects/:id/modeling/:bausteinId", h.RemoveBausteinFromTargetObject, rw)
	g.GET("/bsi/target-objects/:id", h.GetBSITargetObject)
	g.PUT("/bsi/target-objects/:id", h.UpdateBSITargetObject, rw)
	g.DELETE("/bsi/target-objects/:id", h.DeleteBSITargetObject, rw)

	// S74-2: Grundschutz-Cockpit & GAP-Report
	g.GET("/bsi/cockpit", h.GetBSICockpit)
	g.GET("/bsi/gap-report", h.GetBSIGapReport)

	// S74-3: Risikobewertung BSI 200-3 — Gefährdungskatalog
	g.GET("/bsi/threats", h.ListBSIThreats)

	// S74-4: Referenzberichte A1–A6
	// CRITICAL: /bsi/reports/:type/preview must be before /bsi/reports/:type to avoid route conflict.
	g.GET("/bsi/reports", h.ListBSIReportExports)
	g.GET("/bsi/reports/:type/preview", h.GetBSIReportPreview)
	g.GET("/bsi/reports/:type", h.GenerateBSIReport, features.Require(features.FeatureAuditPDF))

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
	g.GET("/frameworks/:id/soa.pdf", h.ExportSoAPDF, features.Require(features.FeatureAuditPDF))
	// Framework compliance report PDF for external auditors (S73-2).
	g.GET("/frameworks/:id/report.pdf", h.ExportFrameworkPDF, features.Require(features.FeatureAuditPDF))
	g.GET("/risks", h.ListRisks)
	g.GET("/incidents", h.ListIncidents)
	g.GET("/policies", h.ListPolicies)
	g.GET("/audits", h.ListAuditRecords)
	g.GET("/export.zip", h.AuditorExportZIP)
}
