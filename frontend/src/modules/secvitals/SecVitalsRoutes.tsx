import { Routes, Route, Navigate } from 'react-router-dom'
import SecVitalsOverviewPage from './pages/SecVitalsOverviewPage'
import FrameworksPage from './pages/FrameworksPage'
import FrameworkDetailPage from './pages/FrameworkDetailPage'
import ControlDetailPage from './pages/ControlDetailPage'
import RisksPage from './pages/RisksPage'
import RiskDetailPage from './pages/RiskDetailPage'
import IncidentsPage from './pages/IncidentsPage'
import IncidentDetailPage from './pages/IncidentDetailPage'
import PoliciesPage from './pages/PoliciesPage'
import PolicyDetailPage from './pages/PolicyDetailPage'
import AuditsPage from './pages/AuditsPage'
import AuditDetailPage from './pages/AuditDetailPage'
import NIS2ChecklistPage from './pages/NIS2ChecklistPage'
import NIS2AssistantPage from './pages/NIS2AssistantPage'
import ISO27001ChecklistPage from './pages/ISO27001ChecklistPage'
import BSIGrundschutzPage from './pages/BSIGrundschutzPage'
import AIReportPage from './pages/AIReportPage'
import AIAgentPage from './pages/AIAgentPage'
import SuppliersPage from './pages/SuppliersPage'
import AISystemsPage from './pages/AISystemsPage'
import AIDocumentationPage from './pages/AIDocumentationPage'
import EUAIActDashboardPage from './pages/EUAIActDashboardPage'
import DORAPage from './pages/DORAPage'
import DORADashboardPage from './pages/DORADashboardPage'
import ResilienceTestsPage from './pages/ResilienceTestsPage'
import TISAXPage from './pages/TISAXPage'
import TISAXMappingPage from './pages/TISAXMappingPage'
import QuestionnairePage from './pages/QuestionnairePage'
import { AssessmentReviewView } from './components/AssessmentReviewView'
import AuthorityDirectoryPage from './pages/AuthorityDirectoryPage'
import DSGVOTOMPage from './pages/DSGVOTOMPage'
import CCMPage from './pages/CCMPage'
import PolicyAcceptancePage from './pages/PolicyAcceptancePage'
import CAPAsPage from './pages/CAPAsPage'
import OverdueReviewsPage from './pages/OverdueReviewsPage'
import EvidenceAutoPage from './pages/EvidenceAutoPage'
import ApprovalsPage from './pages/ApprovalsPage'
import CertificationTimelinePage from './pages/CertificationTimelinePage'
import CISControlsPage from './pages/CISControlsPage'
import SoAPage from './pages/SoAPage'
import AccessReviewsPage from './pages/AccessReviewsPage'
import ExceptionsPage from './pages/ExceptionsPage'
import PolicyTemplatesPage from './pages/PolicyTemplatesPage'

export default function SecVitalsRoutes() {
  return (
    <Routes>
      <Route index element={<SecVitalsOverviewPage />} />
      <Route path="frameworks" element={<FrameworksPage />} />
      <Route path="frameworks/:id" element={<FrameworkDetailPage />} />
      {/* CRITICAL: tisax route must be before the :id/controls catch-all */}
      <Route path="frameworks/:id/tisax" element={<TISAXPage />} />
      <Route path="cis-controls" element={<CISControlsPage />} />
      <Route path="tisax-mapping" element={<TISAXMappingPage />} />
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
      <Route path="iso27001" element={<ISO27001ChecklistPage />} />
      <Route path="grundschutz" element={<BSIGrundschutzPage />} />
      <Route path="ai-report" element={<AIReportPage />} />
      <Route path="ai/agent" element={<AIAgentPage />} />
      <Route path="suppliers" element={<SuppliersPage />} />
      <Route path="ai-systems" element={<AISystemsPage />} />
      <Route path="ai-systems/:id/documentation" element={<AIDocumentationPage />} />
      <Route path="eu-ai-act/dashboard" element={<EUAIActDashboardPage />} />
      {/* CRITICAL: dora/dashboard must be before dora/:frameworkId to avoid catch-all match */}
      <Route path="dora/dashboard" element={<DORADashboardPage />} />
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
      <Route path="*" element={<Navigate to="/secvitals" replace />} />
    </Routes>
  )
}
