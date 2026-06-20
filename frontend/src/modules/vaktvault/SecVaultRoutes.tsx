import { lazy, Suspense } from 'react'
import { Routes, Route, Navigate } from 'react-router-dom'
import { Spinner } from '../../components/Spinner'

const ProjectsPage = lazy(() => import('./pages/ProjectsPage'))
const ProjectDetailPage = lazy(() => import('./pages/ProjectDetailPage'))
const GitScansPage = lazy(() => import('./pages/GitScansPage'))
const TokensPage = lazy(() => import('./pages/TokensPage'))
const AccessReviewsPage = lazy(() => import('./pages/AccessReviewsPage'))

const fallback = <div className="flex h-full items-center justify-center"><Spinner size="lg" color="primary" /></div>

export default function SecVaultRoutes() {
  return (
    <Suspense fallback={fallback}>
      <Routes>
        <Route index element={<Navigate to="projects" replace />} />
        <Route path="projects" element={<ProjectsPage />} />
        <Route path="projects/:id" element={<ProjectDetailPage />} />
        <Route path="git-scans" element={<GitScansPage />} />
        <Route path="tokens" element={<TokensPage />} />
        <Route path="access-reviews" element={<AccessReviewsPage />} />
        <Route path="*" element={<Navigate to="projects" replace />} />
      </Routes>
    </Suspense>
  )
}
