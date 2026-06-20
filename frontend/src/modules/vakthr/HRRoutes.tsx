import { lazy, Suspense } from 'react'
import { Routes, Route, Navigate } from 'react-router-dom'
import { Spinner } from '../../components/Spinner'

const EmployeesPage = lazy(() => import('./pages/EmployeesPage'))
const ChecklistsPage = lazy(() => import('./pages/ChecklistsPage'))
const ChecklistRunPage = lazy(() => import('./pages/ChecklistRunPage'))
const AccessConceptsPage = lazy(() => import('./pages/AccessConceptsPage'))
const MoverEventsPage = lazy(() => import('./pages/MoverEventsPage'))
const ContractorsPage = lazy(() => import('./pages/ContractorsPage'))

const fallback = <div className="flex h-full items-center justify-center"><Spinner size="lg" color="primary" /></div>

export default function HRRoutes() {
  return (
    <Suspense fallback={fallback}>
      <Routes>
        <Route index element={<Navigate to="employees" replace />} />
        <Route path="employees" element={<EmployeesPage />} />
        <Route path="checklists" element={<ChecklistsPage />} />
        <Route path="checklist-runs/:id" element={<ChecklistRunPage />} />
        <Route path="access-concepts" element={<AccessConceptsPage />} />
        <Route path="mover-events" element={<MoverEventsPage />} />
        <Route path="contractors" element={<ContractorsPage />} />
        <Route path="*" element={<Navigate to="/vakthr/employees" replace />} />
      </Routes>
    </Suspense>
  )
}
