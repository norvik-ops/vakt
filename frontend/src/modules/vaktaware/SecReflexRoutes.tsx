import { lazy, Suspense } from 'react'
import { Routes, Route, Navigate } from 'react-router-dom'
import { Spinner } from '../../components/Spinner'

const CampaignsPage = lazy(() => import('./pages/CampaignsPage'))
const CampaignDetailPage = lazy(() => import('./pages/CampaignDetailPage'))
const TemplatesPage = lazy(() => import('./pages/TemplatesPage'))
const TargetGroupsPage = lazy(() => import('./pages/TargetGroupsPage'))
const TrainingPage = lazy(() => import('./pages/TrainingPage'))
const PhishReportsPage = lazy(() => import('./pages/PhishReportsPage'))
const EnrollmentRulesPage = lazy(() => import('./pages/EnrollmentRulesPage'))
const TrainingReportPage = lazy(() => import('./pages/TrainingReportPage'))

const fallback = <div className="flex h-full items-center justify-center"><Spinner size="lg" color="primary" /></div>

export default function SecReflexRoutes() {
  return (
    <Suspense fallback={fallback}>
      <Routes>
        <Route index element={<Navigate to="campaigns" replace />} />
        <Route path="campaigns" element={<CampaignsPage />} />
        <Route path="campaigns/:id" element={<CampaignDetailPage />} />
        <Route path="templates" element={<TemplatesPage />} />
        <Route path="target-groups" element={<TargetGroupsPage />} />
        <Route path="training" element={<TrainingPage />} />
        <Route path="phish-reports" element={<PhishReportsPage />} />
        <Route path="enrollment-rules" element={<EnrollmentRulesPage />} />
        <Route path="training-report" element={<TrainingReportPage />} />
        <Route path="*" element={<Navigate to="campaigns" replace />} />
      </Routes>
    </Suspense>
  )
}
