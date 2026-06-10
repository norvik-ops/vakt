import { Routes, Route, Navigate } from 'react-router-dom'
import ScanOverviewPage from './pages/ScanOverviewPage'
import AssetsPage from './pages/AssetsPage'
import AssetDetailPage from './pages/AssetDetailPage'
import FindingsPage from './pages/FindingsPage'
import FindingDetailPage from './pages/FindingDetailPage'
import ReportsPage from './pages/ReportsPage'
import SLADashboardPage from './pages/SLADashboardPage'
import EOLDashboardPage from './pages/EOLDashboardPage'
import CertificatesPage from './pages/CertificatesPage'

export default function SecPulseRoutes() {
  return (
    <Routes>
      <Route index element={<ScanOverviewPage />} />
      <Route path="assets" element={<AssetsPage />} />
      <Route path="assets/:id" element={<AssetDetailPage />} />
      <Route path="findings" element={<FindingsPage />} />
      <Route path="findings/:id" element={<FindingDetailPage />} />
      <Route path="reports" element={<ReportsPage />} />
      <Route path="sla" element={<SLADashboardPage />} />
      <Route path="eol" element={<EOLDashboardPage />} />
      <Route path="certificates" element={<CertificatesPage />} />
      <Route path="*" element={<Navigate to="assets" replace />} />
    </Routes>
  )
}
