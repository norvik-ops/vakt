import { Routes, Route, Navigate } from 'react-router-dom'
import SecPrivacyOverviewPage from './pages/SecPrivacyOverviewPage'
import VVTPage from './pages/VVTPage'
import DPIAPage from './pages/DPIAPage'
import AVVPage from './pages/AVVPage'
import BreachPage from './pages/BreachPage'
import DSRPage from './pages/DSRPage'
import DSRPortalSettingsPage from './pages/DSRPortalSettingsPage'

export default function SecPrivacyRoutes() {
  return (
    <Routes>
      <Route index element={<SecPrivacyOverviewPage />} />
      <Route path="vvt" element={<VVTPage />} />
      <Route path="dpia" element={<DPIAPage />} />
      <Route path="avv" element={<AVVPage />} />
      <Route path="breach" element={<BreachPage />} />
      <Route path="dsr" element={<DSRPage />} />
      <Route path="dsr-portal-settings" element={<DSRPortalSettingsPage />} />
      <Route path="*" element={<Navigate to="/vaktprivacy" replace />} />
    </Routes>
  )
}
