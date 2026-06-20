import { lazy, Suspense } from 'react'
import { Routes, Route, Navigate } from 'react-router-dom'
import { Spinner } from '../../components/Spinner'

// ponytail: lazy splits keep ReportsPage (Recharts) out of the entry chunk (S98-2)
const ScanOverviewPage = lazy(() => import('./pages/ScanOverviewPage'))
const AssetsPage = lazy(() => import('./pages/AssetsPage'))
const AssetDetailPage = lazy(() => import('./pages/AssetDetailPage'))
const FindingsPage = lazy(() => import('./pages/FindingsPage'))
const FindingDetailPage = lazy(() => import('./pages/FindingDetailPage'))
const ReportsPage = lazy(() => import('./pages/ReportsPage'))
const SLADashboardPage = lazy(() => import('./pages/SLADashboardPage'))
const EOLDashboardPage = lazy(() => import('./pages/EOLDashboardPage'))
const CertificatesPage = lazy(() => import('./pages/CertificatesPage'))

const fallback = <div className="flex h-full items-center justify-center"><Spinner size="lg" color="primary" /></div>

export default function SecPulseRoutes() {
  return (
    <Suspense fallback={fallback}>
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
    </Suspense>
  )
}
