import { Routes, Route, Navigate } from 'react-router-dom'
import EmployeesPage from './pages/EmployeesPage'
import ChecklistsPage from './pages/ChecklistsPage'
import ChecklistRunPage from './pages/ChecklistRunPage'
import AccessConceptsPage from './pages/AccessConceptsPage'

export default function HRRoutes() {
  return (
    <Routes>
      <Route index element={<Navigate to="employees" replace />} />
      <Route path="employees" element={<EmployeesPage />} />
      <Route path="checklists" element={<ChecklistsPage />} />
      <Route path="checklist-runs/:id" element={<ChecklistRunPage />} />
      <Route path="access-concepts" element={<AccessConceptsPage />} />
      <Route path="*" element={<Navigate to="/vakthr/employees" replace />} />
    </Routes>
  )
}
