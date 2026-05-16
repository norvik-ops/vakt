import { Routes, Route, Navigate } from 'react-router-dom'
import CampaignsPage from './pages/CampaignsPage'
import CampaignDetailPage from './pages/CampaignDetailPage'
import TemplatesPage from './pages/TemplatesPage'
import TargetGroupsPage from './pages/TargetGroupsPage'
import TrainingPage from './pages/TrainingPage'
import PhishReportsPage from './pages/PhishReportsPage'

export default function SecReflexRoutes() {
  return (
    <Routes>
      <Route index element={<Navigate to="campaigns" replace />} />
      <Route path="campaigns" element={<CampaignsPage />} />
      <Route path="campaigns/:id" element={<CampaignDetailPage />} />
      <Route path="templates" element={<TemplatesPage />} />
      <Route path="target-groups" element={<TargetGroupsPage />} />
      <Route path="training" element={<TrainingPage />} />
      <Route path="phish-reports" element={<PhishReportsPage />} />
      <Route path="*" element={<Navigate to="campaigns" replace />} />
    </Routes>
  )
}
