import { Routes, Route, Navigate } from 'react-router-dom'
import ProjectsPage from './pages/ProjectsPage'
import ProjectDetailPage from './pages/ProjectDetailPage'
import GitScansPage from './pages/GitScansPage'
import TokensPage from './pages/TokensPage'
import AccessReviewsPage from './pages/AccessReviewsPage'

export default function SecVaultRoutes() {
  return (
    <Routes>
      <Route index element={<Navigate to="projects" replace />} />
      <Route path="projects" element={<ProjectsPage />} />
      <Route path="projects/:id" element={<ProjectDetailPage />} />
      <Route path="git-scans" element={<GitScansPage />} />
      <Route path="tokens" element={<TokensPage />} />
      <Route path="access-reviews" element={<AccessReviewsPage />} />
      <Route path="*" element={<Navigate to="projects" replace />} />
    </Routes>
  )
}
