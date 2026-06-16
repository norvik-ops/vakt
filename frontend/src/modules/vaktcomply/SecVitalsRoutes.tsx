import { lazy, Suspense } from 'react'
import { Routes, Route, Navigate } from 'react-router-dom'

const SecVitalsOverviewPage = lazy(() => import('./pages/SecVitalsOverviewPage'))
const FrameworksPage = lazy(() => import('./pages/FrameworksPage'))
const FrameworkDetailPage = lazy(() => import('./pages/FrameworkDetailPage'))
const ControlDetailPage = lazy(() => import('./pages/ControlDetailPage'))
const RisksPage = lazy(() => import('./pages/RisksPage'))
const RiskDetailPage = lazy(() => import('./pages/RiskDetailPage'))
const IncidentsPage = lazy(() => import('./pages/IncidentsPage'))
const IncidentDetailPage = lazy(() => import('./pages/IncidentDetailPage'))
const PoliciesPage = lazy(() => import('./pages/PoliciesPage'))
const PolicyDetailPage = lazy(() => import('./pages/PolicyDetailPage'))
const AuditsPage = lazy(() => import('./pages/AuditsPage'))
const AuditDetailPage = lazy(() => import('./pages/AuditDetailPage'))
const NIS2ChecklistPage = lazy(() => import('./pages/NIS2ChecklistPage'))
const NIS2AssistantPage = lazy(() => import('./pages/NIS2AssistantPage'))
const NIS2ReassessmentPage = lazy(() => import('./pages/NIS2ReassessmentPage'))
const ISO27001ChecklistPage = lazy(() => import('./pages/ISO27001ChecklistPage'))
const BSIGrundschutzPage = lazy(() => import('./pages/BSIGrundschutzPage'))
const AIReportPage = lazy(() => import('./pages/AIReportPage'))
const AIAgentPage = lazy(() => import('./pages/AIAgentPage'))
const SuppliersPage = lazy(() => import('./pages/SuppliersPage'))
const AISystemsPage = lazy(() => import('./pages/AISystemsPage'))
const AIDocumentationPage = lazy(() => import('./pages/AIDocumentationPage'))
const EUAIActDashboardPage = lazy(() => import('./pages/EUAIActDashboardPage'))
const DORAPage = lazy(() => import('./pages/DORAPage'))
const DORADashboardPage = lazy(() => import('./pages/DORADashboardPage'))
const DORAThirdPartiesPage = lazy(() => import('./pages/DORAThirdPartiesPage'))
const ResilienceTestsPage = lazy(() => import('./pages/ResilienceTestsPage'))
const TISAXPage = lazy(() => import('./pages/TISAXPage'))
const TISAXMappingPage = lazy(() => import('./pages/TISAXMappingPage'))
const QuestionnairePage = lazy(() => import('./pages/QuestionnairePage'))
const AssessmentReviewView = lazy(() =>
  import('./components/AssessmentReviewView').then(m => ({ default: m.AssessmentReviewView }))
)
const AuthorityDirectoryPage = lazy(() => import('./pages/AuthorityDirectoryPage'))
const DSGVOTOMPage = lazy(() => import('./pages/DSGVOTOMPage'))
const CCMPage = lazy(() => import('./pages/CCMPage'))
const PolicyAcceptancePage = lazy(() => import('./pages/PolicyAcceptancePage'))
const CAPAsPage = lazy(() => import('./pages/CAPAsPage'))
const OverdueReviewsPage = lazy(() => import('./pages/OverdueReviewsPage'))
const EvidenceAutoPage = lazy(() => import('./pages/EvidenceAutoPage'))
const ApprovalsPage = lazy(() => import('./pages/ApprovalsPage'))
const CertificationTimelinePage = lazy(() => import('./pages/CertificationTimelinePage'))
const CISControlsPage = lazy(() => import('./pages/CISControlsPage'))
const MappingCoveragePage = lazy(() => import('./pages/MappingCoveragePage'))
const SoAPage = lazy(() => import('./pages/SoAPage'))
const AccessReviewsPage = lazy(() => import('./pages/AccessReviewsPage'))
const ExceptionsPage = lazy(() => import('./pages/ExceptionsPage'))
const PolicyTemplatesPage = lazy(() => import('./pages/PolicyTemplatesPage'))
const BCPPage = lazy(() => import('./pages/BCPPage'))
const ProtectionNeedsPage = lazy(() => import('./pages/ProtectionNeedsPage'))
const ISMSScopePage = lazy(() => import('./pages/ISMSScopePage'))
const PentestsPage = lazy(() => import('./pages/PentestsPage'))
const BSIModelingPage = lazy(() => import('./pages/BSIModelingPage'))
const BSITargetObjectsPage = lazy(() => import('./pages/BSITargetObjectsPage'))
const BSICheckSheetPage = lazy(() => import('./pages/BSICheckSheetPage'))
const BSICockpitPage = lazy(() => import('./pages/BSICockpitPage'))
const BSIReportsPage = lazy(() => import('./pages/BSIReportsPage'))
const ManagementReviewsPage = lazy(() => import('./pages/ManagementReviewsPage'))
const KPIDashboardPage = lazy(() => import('./pages/KPIDashboardPage'))
const CryptoKeysPage = lazy(() => import('./pages/CryptoKeysPage'))
const InterestedPartiesPage = lazy(() => import('./pages/InterestedPartiesPage'))
const AuditProgramPage = lazy(() => import('./pages/AuditProgramPage'))
const BCMDashboardPage = lazy(() => import('./pages/BCMDashboardPage'))
const BIAPage = lazy(() => import('./pages/BIAPage'))
const RecoveryPlansPage = lazy(() => import('./pages/RecoveryPlansPage'))
const EmergencyContactsPage = lazy(() => import('./pages/EmergencyContactsPage'))
const BackupEvidencePage = lazy(() => import('./pages/BackupEvidencePage'))

export default function SecVitalsRoutes() {
  return (
    <Suspense fallback={null}>
      <Routes>
        <Route index element={<SecVitalsOverviewPage />} />
        <Route path="frameworks" element={<FrameworksPage />} />
        <Route path="frameworks/:id" element={<FrameworkDetailPage />} />
        {/* CRITICAL: tisax route must be before the :id/controls catch-all */}
        <Route path="frameworks/:id/tisax" element={<TISAXPage />} />
        <Route path="cis-controls" element={<CISControlsPage />} />
        <Route path="tisax-mapping" element={<TISAXMappingPage />} />
        <Route path="mapping-coverage" element={<MappingCoveragePage />} />
        {/* CRITICAL: overdue-reviews must be before controls/:id to avoid catch-all match */}
        <Route path="overdue-reviews" element={<OverdueReviewsPage />} />
        <Route path="evidence/auto" element={<EvidenceAutoPage />} />
        <Route path="controls/:id" element={<ControlDetailPage />} />
        <Route path="risks" element={<RisksPage />} />
        <Route path="risks/:id" element={<RiskDetailPage />} />
        <Route path="incidents" element={<IncidentsPage />} />
        <Route path="incidents/:id" element={<IncidentDetailPage />} />
        <Route path="policies" element={<PoliciesPage />} />
        <Route path="policies/:id" element={<PolicyDetailPage />} />
        <Route path="policies/:id/acceptance" element={<PolicyAcceptancePage />} />
        <Route path="audits" element={<AuditsPage />} />
        <Route path="audits/:id" element={<AuditDetailPage />} />
        <Route path="nis2" element={<NIS2ChecklistPage />} />
        <Route path="nis2-assistant" element={<NIS2AssistantPage />} />
        {/* Sprint 28 / S28-3: Re-Assessment History — ProGate: FeatureNIS2Reporting */}
        <Route path="nis2-history" element={<NIS2ReassessmentPage />} />
        <Route path="iso27001" element={<ISO27001ChecklistPage />} />
        <Route path="grundschutz" element={<BSIGrundschutzPage />} />
        <Route path="ai-report" element={<AIReportPage />} />
        <Route path="ai/agent" element={<AIAgentPage />} />
        <Route path="suppliers" element={<SuppliersPage />} />
        <Route path="ai-systems" element={<AISystemsPage />} />
        <Route path="ai-systems/:id/documentation" element={<AIDocumentationPage />} />
        <Route path="eu-ai-act/dashboard" element={<EUAIActDashboardPage />} />
        {/* CRITICAL: dora/dashboard and dora/third-parties must be before dora/:frameworkId to avoid catch-all match */}
        <Route path="dora/dashboard" element={<DORADashboardPage />} />
        <Route path="dora/third-parties" element={<DORAThirdPartiesPage />} />
        <Route path="dora/:frameworkId" element={<DORAPage />} />
        <Route path="resilience-tests" element={<ResilienceTestsPage />} />
        <Route path="questionnaires/:id" element={<QuestionnairePage />} />
        {/* CRITICAL: assessments/:id/review must be before any catch-all */}
        <Route path="assessments/:id/review" element={<AssessmentReviewView />} />
        <Route path="authorities" element={<AuthorityDirectoryPage />} />
        <Route path="dsgvo/tom" element={<DSGVOTOMPage />} />
        <Route path="ccm" element={<CCMPage />} />
        <Route path="capas" element={<CAPAsPage />} />
        <Route path="approvals" element={<ApprovalsPage />} />
        <Route path="certification-timeline" element={<CertificationTimelinePage />} />
        <Route path="soa" element={<SoAPage />} />
        <Route path="access-reviews" element={<AccessReviewsPage />} />
        <Route path="exceptions" element={<ExceptionsPage />} />
        <Route path="policy-templates" element={<PolicyTemplatesPage />} />
        <Route path="bcp" element={<BCPPage />} />
        <Route path="protection-needs" element={<ProtectionNeedsPage />} />
        <Route path="isms-scope" element={<ISMSScopePage />} />
        <Route path="pentests" element={<PentestsPage />} />
        <Route path="bsi-modeling" element={<BSIModelingPage />} />
        <Route path="bsi/target-objects" element={<BSITargetObjectsPage />} />
        <Route path="bsi/check/:id" element={<BSICheckSheetPage />} />
        <Route path="bsi/cockpit" element={<BSICockpitPage />} />
        <Route path="bsi/reports" element={<BSIReportsPage />} />
        <Route path="management-reviews" element={<ManagementReviewsPage />} />
        <Route path="kpi-dashboard" element={<KPIDashboardPage />} />
        <Route path="crypto-keys" element={<CryptoKeysPage />} />
        <Route path="interested-parties" element={<InterestedPartiesPage />} />
        <Route path="audit-program" element={<AuditProgramPage />} />
        <Route path="bcm" element={<BCMDashboardPage />} />
        <Route path="bcm/bia" element={<BIAPage />} />
        <Route path="bcm/recovery-plans" element={<RecoveryPlansPage />} />
        <Route path="bcm/emergency-contacts" element={<EmergencyContactsPage />} />
        <Route path="backup" element={<BackupEvidencePage />} />
        <Route path="*" element={<Navigate to="/vaktcomply" replace />} />
      </Routes>
    </Suspense>
  )
}
