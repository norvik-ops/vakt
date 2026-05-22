import { createBrowserRouter, Navigate, Outlet } from 'react-router-dom'
import { useEffect, useState, lazy, Suspense } from 'react'
import { apiFetch } from './api/client'
import { useAuthStore } from './shared/stores/auth'
import Layout from './shared/components/Layout'
import { ErrorBoundary } from './shared/components/ErrorBoundary'
import { Spinner } from './components/Spinner'

// Eager-loaded: Auth-Flows (Login/Setup), Dashboard (initial Landing),
// öffentliche Pages mit Magic-Link-Tokens (auditor/policy/invite/dsr —
// die Token-Validierung soll ohne Code-Split-Wartezeit laufen),
// NotFound (Fallback ohne Suspense-Spinner).
import Setup from './pages/Setup'
import Login from './pages/Login'
import Dashboard from './pages/Dashboard'
import TrustPage from './pages/TrustPage'
import NIS2WizardPage from './pages/NIS2WizardPage'
import MultiFrameworkWizardPage from './pages/MultiFrameworkWizardPage'
import AuditorAcceptPage from './pages/AuditorAcceptPage'
import PolicyAcceptPage from './pages/PolicyAcceptPage'
import InviteAcceptPage from './pages/InviteAcceptPage'
import DSRPortalPage from './pages/DSRPortalPage'
import DSRPortalStatusPage from './pages/DSRPortalStatusPage'
import SupplierPortalPage from './pages/SupplierPortalPage'
import ForgotPasswordPage from './pages/ForgotPasswordPage'
import ResetPasswordPage from './pages/ResetPasswordPage'
import NotFoundPage from './pages/NotFoundPage'

// Sprint 16 S16-3: alle anderen Page-Imports lazy. Reduziert das Initial-
// Bundle um die Settings-/Admin-/Audit-Sektionen, die nach Login selten
// auf den ersten Klick gebraucht werden.
const Settings                   = lazy(() => import('./modules/settings/pages/Settings'))
const ScoreConfigPage            = lazy(() => import('./pages/ScoreConfigPage'))
const AlertingSettingsPage       = lazy(() => import('./modules/settings/pages/AlertingSettingsPage'))
const AccountSettingsPage        = lazy(() => import('./modules/settings/pages/AccountSettingsPage'))
const RetentionConfigPage        = lazy(() => import('./pages/RetentionConfigPage'))
const SessionsPage               = lazy(() => import('./pages/SessionsPage'))
const OrgBrandingPage            = lazy(() => import('./pages/OrgBrandingPage'))
const TrustCenterSettingsPage    = lazy(() => import('./modules/settings/pages/TrustCenterSettingsPage'))
const IntegrationsPage           = lazy(() => import('./pages/IntegrationsPage'))
const AuditorSettingsPage        = lazy(() => import('./modules/settings/pages/AuditorSettingsPage'))
const TeamSettingsPage           = lazy(() => import('./modules/settings/pages/TeamSettingsPage'))
const AuditLogPage               = lazy(() => import('./pages/AuditLogPage'))
const ApiKeysPage                = lazy(() => import('./pages/ApiKeysPage'))
const AdminHealthPage            = lazy(() => import('./pages/AdminHealthPage'))
const AdminTenantsPage           = lazy(() => import('./pages/AdminTenantsPage'))
const AdminSecurityPage          = lazy(() => import('./pages/AdminSecurityPage'))
const WebhooksPage               = lazy(() => import('./pages/WebhooksPage'))
const ScheduledReportsPage       = lazy(() => import('./pages/ScheduledReportsPage'))
const NotificationPreferencesPage = lazy(() => import('./pages/NotificationPreferencesPage'))

const SecPulse    = lazy(() => import('./modules/secpulse/SecPulseRoutes'))
const SecVitals   = lazy(() => import('./modules/secvitals/SecVitalsRoutes'))
const SecVault    = lazy(() => import('./modules/secvault/SecVaultRoutes'))
const SecReflex   = lazy(() => import('./modules/secreflex/SecReflexRoutes'))
const SecPrivacy  = lazy(() => import('./modules/secprivacy/SecPrivacyRoutes'))
const HR          = lazy(() => import('./modules/hr/HRRoutes'))

function LoadingSpinner() {
  return (
    <div className="flex items-center justify-center h-64">
      <Spinner size="lg" />
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
  if (!isAuthenticated) {
    return <Navigate to="/login" replace />
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
    // Sprint 19 S19-4: Public NIS2-Wizard — kein Layout, kein Auth.
    // Top-of-Funnel-Akquise-Asset.
    path: '/nis2-check',
    element: <NIS2WizardPage />,
  },
  {
    // Sprint 28 S28-4: Multi-Framework-Assessment (NIS2 + ISO27001 + DSGVO-TOM).
    // ProGate: FeatureNIS2Reporting. Kein Layout-Wrapper, kein Setup-Guard.
    path: '/nis2-check/multi',
    element: <MultiFrameworkWizardPage />,
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
          { path: '/settings/api-keys', element: <ApiKeysPage /> },
          { path: '/settings/webhooks', element: <WebhooksPage /> },
          { path: '/settings/reports', element: <ScheduledReportsPage /> },
          { path: '/settings/notifications', element: <NotificationPreferencesPage /> },
          { path: '/admin/health', element: <AdminHealthPage /> },
          { path: '/admin/tenants', element: <AdminTenantsPage /> },
          { path: '/admin/security', element: <AdminSecurityPage /> },
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
          { path: '*', element: <NotFoundPage /> },
        ],
      },
    ],
  },
])
