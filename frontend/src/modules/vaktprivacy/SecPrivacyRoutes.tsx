import { lazy, Suspense } from 'react'
import { Routes, Route, Navigate } from 'react-router-dom'
import { Spinner } from '../../components/Spinner'

const SecPrivacyOverviewPage = lazy(() => import('./pages/SecPrivacyOverviewPage'))
const VVTPage = lazy(() => import('./pages/VVTPage'))
const DPIAPage = lazy(() => import('./pages/DPIAPage'))
const AVVPage = lazy(() => import('./pages/AVVPage'))
const BreachPage = lazy(() => import('./pages/BreachPage'))
const DSRPage = lazy(() => import('./pages/DSRPage'))
const DSRPortalSettingsPage = lazy(() => import('./pages/DSRPortalSettingsPage'))
const DeletionRemindersPage = lazy(() => import('./pages/DeletionRemindersPage'))
const TransfersPage = lazy(() => import('./pages/TransfersPage'))
const PrivacyDesignPage = lazy(() => import('./pages/PrivacyDesignPage'))

const fallback = <div className="flex h-full items-center justify-center"><Spinner size="lg" color="primary" /></div>

export default function SecPrivacyRoutes() {
  return (
    <Suspense fallback={fallback}>
      <Routes>
        <Route index element={<SecPrivacyOverviewPage />} />
        <Route path="vvt" element={<VVTPage />} />
        <Route path="dpia" element={<DPIAPage />} />
        <Route path="avv" element={<AVVPage />} />
        <Route path="breach" element={<BreachPage />} />
        <Route path="dsr" element={<DSRPage />} />
        <Route path="dsr-portal-settings" element={<DSRPortalSettingsPage />} />
        <Route path="deletion-reminders" element={<DeletionRemindersPage />} />
        <Route path="transfers" element={<TransfersPage />} />
        <Route path="privacy-design" element={<PrivacyDesignPage />} />
        <Route path="*" element={<Navigate to="/vaktprivacy" replace />} />
      </Routes>
    </Suspense>
  )
}
