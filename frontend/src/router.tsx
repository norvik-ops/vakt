import { createBrowserRouter, Navigate, Outlet } from 'react-router-dom'
import { useEffect, useState } from 'react'
import { apiFetch } from './api/client'
import { useAuthStore } from './shared/stores/auth'
import { useDemoMode } from './shared/hooks/useDemoMode'
import Layout from './shared/components/Layout'
import Setup from './pages/Setup'
import Login from './pages/Login'
import DemoLanding from './pages/DemoLanding'
import Dashboard from './pages/Dashboard'
import Settings from './pages/Settings'
import ScoreConfigPage from './pages/ScoreConfigPage'
import AlertingSettingsPage from './pages/AlertingSettingsPage'
import AccountSettingsPage from './pages/AccountSettingsPage'
import RetentionConfigPage from './pages/RetentionConfigPage'
import SessionsPage from './pages/SessionsPage'
import OrgBrandingPage from './pages/OrgBrandingPage'
import TrustPage from './pages/TrustPage'
import TrustCenterSettingsPage from './pages/TrustCenterSettingsPage'
import SupplierPortalPage from './pages/SupplierPortalPage'
import DSRPortalPage from './pages/DSRPortalPage'
import DSRPortalStatusPage from './pages/DSRPortalStatusPage'
import IntegrationsPage from './pages/IntegrationsPage'
import AuditorSettingsPage from './pages/AuditorSettingsPage'
import AuditorAcceptPage from './pages/AuditorAcceptPage'
import PolicyAcceptPage from './pages/PolicyAcceptPage'
import InviteAcceptPage from './pages/InviteAcceptPage'
import TeamSettingsPage from './pages/TeamSettingsPage'
import AuditLogPage from './pages/AuditLogPage'
import ForgotPasswordPage from './pages/ForgotPasswordPage'
import ResetPasswordPage from './pages/ResetPasswordPage'

// Lazy module pages — filled in by module agents
import { lazy, Suspense } from 'react'
import { ErrorBoundary } from './shared/components/ErrorBoundary'

const SecPulse    = lazy(() => import('./modules/secpulse/SecPulseRoutes'))
const SecVitals   = lazy(() => import('./modules/secvitals/SecVitalsRoutes'))
const SecVault    = lazy(() => import('./modules/secvault/SecVaultRoutes'))
const SecReflex   = lazy(() => import('./modules/secreflex/SecReflexRoutes'))
const SecPrivacy  = lazy(() => import('./modules/secprivacy/SecPrivacyRoutes'))
const HR          = lazy(() => import('./modules/hr/HRRoutes'))

function LoadingSpinner() {
  return (
    <div className="flex items-center justify-center h-64">
      <div className="w-6 h-6 border-2 border-blue-600 border-t-transparent rounded-full animate-spin" />
    </div>
  )
}

interface SetupStatus {
  setup_complete: boolean
}

// Renders children immediately; redirects to /setup only if setup is confirmed incomplete.
// No spinner — avoids flash on demo and normal instances where setup is already done.
function SetupGuard({ children }: { children: React.ReactNode }) {
  const [needsSetup, setNeedsSetup] = useState(false)

  useEffect(() => {
    apiFetch<SetupStatus>('/setup/status')
      .then((data) => { if (!data.setup_complete) setNeedsSetup(true) })
      .catch(() => {})
  }, [])

  if (needsSetup) return <Navigate to="/setup" replace />
  return <>{children}</>
}

// Prevents accessing /setup when setup is already complete — redirects to /login.
function SetupPageGuard() {
  const [ready, setReady] = useState(false)
  const [setupNeeded, setSetupNeeded] = useState(true)

  useEffect(() => {
    apiFetch<SetupStatus>('/setup/status')
      .then((data) => { setSetupNeeded(!data.setup_complete); setReady(true) })
      .catch(() => setReady(true))
  }, [])

  if (!ready) return <LoadingSpinner />
  if (!setupNeeded) return <Navigate to="/login" replace />
  return <Setup />
}

function AuthGuard() {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated())
  const demoMode = useDemoMode()
  if (!isAuthenticated) {
    if (demoMode === null) return <LoadingSpinner />
    return <Navigate to={demoMode ? '/demo' : '/login'} replace />
  }
  return <Outlet />
}

function ModuleShell({ children, moduleKey }: { children: React.ReactNode; moduleKey?: string }) {
  return (
    <ErrorBoundary key={moduleKey}>
      <Suspense fallback={<LoadingSpinner />}>{children}</Suspense>
    </ErrorBoundary>
  )
}

export const router = createBrowserRouter([
  {
    path: '/demo',
    element: <DemoLanding />,
  },
  {
    path: '/setup',
    element: <SetupPageGuard />,
  },
  {
    path: '/auditor/accept/:token',
    element: <AuditorAcceptPage />,
  },
  {
    path: '/invite/accept',
    element: <InviteAcceptPage />,
  },
  {
    path: '/policy/accept/:token',
    element: <PolicyAcceptPage />,
  },
  {
    path: '/trust/:slug',
    element: <TrustPage />,
  },
  {
    path: '/supplier/:token',
    element: <SupplierPortalPage />,
  },
  {
    path: '/dsr/status/:token',
    element: <DSRPortalStatusPage />,
  },
  {
    path: '/dsr/:slug',
    element: <DSRPortalPage />,
  },
  {
    path: '/login',
    element: (
      <SetupGuard>
        <Login />
      </SetupGuard>
    ),
  },
  {
    path: '/auth/forgot-password',
    element: <ForgotPasswordPage />,
  },
  {
    path: '/auth/reset-password',
    element: <ResetPasswordPage />,
  },
  {
    element: (
      <SetupGuard>
        <AuthGuard />
      </SetupGuard>
    ),
    children: [
      {
        element: <Layout />,
        children: [
          { path: '/', element: <Dashboard /> },
          { path: '/account', element: <AccountSettingsPage /> },
          { path: '/settings', element: <Settings /> },
          { path: '/settings/score-config', element: <ScoreConfigPage /> },
          { path: '/settings/alerting', element: <AlertingSettingsPage /> },
          { path: '/settings/retention', element: <RetentionConfigPage /> },
          { path: '/account/sessions', element: <SessionsPage /> },
          { path: '/settings/branding', element: <OrgBrandingPage /> },
          { path: '/settings/trust-center', element: <TrustCenterSettingsPage /> },
          { path: '/settings/auditors', element: <AuditorSettingsPage /> },
          { path: '/settings/team', element: <TeamSettingsPage /> },
          { path: '/settings/audit-log', element: <AuditLogPage /> },
          {
            path: '/secpulse/*',
            element: <ModuleShell moduleKey="secpulse"><SecPulse /></ModuleShell>,
          },
          {
            path: '/secvitals/*',
            element: <ModuleShell moduleKey="secvitals"><SecVitals /></ModuleShell>,
          },
          {
            path: '/secvault/*',
            element: <ModuleShell moduleKey="secvault"><SecVault /></ModuleShell>,
          },
          {
            path: '/secreflex/*',
            element: <ModuleShell moduleKey="secreflex"><SecReflex /></ModuleShell>,
          },
          {
            path: '/secprivacy/*',
            element: <ModuleShell moduleKey="secprivacy"><SecPrivacy /></ModuleShell>,
          },
          {
            path: '/hr/*',
            element: <ModuleShell moduleKey="hr"><HR /></ModuleShell>,
          },
          { path: '/integrations', element: <IntegrationsPage /> },
          { path: '*', element: <Navigate to="/" replace /> },
        ],
      },
    ],
  },
])
