import { useState, useEffect } from 'react'
import { ShieldCheck, LogOut, Download, FileText, ChevronDown, ChevronRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { Badge } from '../components/ui/badge'
import {
  useAuditorFrameworks,
  useAuditorControls,
  useAuditorRisks,
  useAuditorIncidents,
  useAuditorPolicies,
  downloadAuditorZip,
  downloadAuditorFrameworkPDF,
  type AuditorFramework,
} from '../hooks/useAuditorPortal'

// ---------------------------------------------------------------------------
// Token storage key in sessionStorage
// ---------------------------------------------------------------------------
const TOKEN_KEY = 'auditor_session_token'

// ---------------------------------------------------------------------------
// Login form
// ---------------------------------------------------------------------------

interface LoginFormProps {
  onLogin: (token: string) => void
  error?: string
}

function LoginForm({ onLogin, error }: LoginFormProps) {
  const { t } = useTranslation()
  const [token, setToken] = useState('')

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    const trimmed = token.trim()
    if (trimmed) onLogin(trimmed)
  }

  return (
    <div className="min-h-screen bg-bg flex items-center justify-center p-6">
      <div className="w-full max-w-md space-y-6">
        <div className="flex flex-col items-center gap-3 text-center">
          <ShieldCheck className="w-12 h-12 text-brand" />
          <h1 className="text-2xl font-bold text-primary">{t('auditorPortal.loginTitle')}</h1>
          <p className="text-secondary text-sm">{t('auditorPortal.loginDescription')}</p>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4 rounded-lg border border-border bg-surface p-6">
          <div className="space-y-1">
            <Label htmlFor="auditor-token">{t('auditorPortal.tokenLabel')}</Label>
            <Input
              id="auditor-token"
              type="text"
              placeholder={t('auditorPortal.tokenPlaceholder')}
              value={token}
              onChange={(e) => { setToken(e.target.value) }}
              className="font-mono text-sm"
              autoComplete="off"
            />
          </div>
          {error && (
            <p className="text-sm text-destructive">{error}</p>
          )}
          <Button type="submit" className="w-full" disabled={!token.trim()}>
            {t('auditorPortal.loginButton')}
          </Button>
        </form>

        <p className="text-center text-xs text-secondary">{t('auditorPortal.poweredBy')}</p>
      </div>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Frameworks tab
// ---------------------------------------------------------------------------

interface FrameworkRowProps {
  fw: AuditorFramework
  token: string
}

function FrameworkRow({ fw, token }: FrameworkRowProps) {
  const { t } = useTranslation()
  const [expanded, setExpanded] = useState(false)
  const { data: controls = [], isLoading } = useAuditorControls(expanded ? fw.id : null, token)

  const readiness = fw.readiness_score

  return (
    <div className="rounded-lg border border-border bg-surface overflow-hidden">
      <div
        className="flex items-center justify-between p-4 cursor-pointer hover:bg-bg/60 transition-colors"
        onClick={() => { setExpanded((v) => !v) }}
      >
        <div className="flex items-center gap-3 min-w-0">
          {expanded ? <ChevronDown className="w-4 h-4 shrink-0 text-secondary" /> : <ChevronRight className="w-4 h-4 shrink-0 text-secondary" />}
          <div className="min-w-0">
            <p className="font-medium text-primary truncate">{fw.name}</p>
            <p className="text-xs text-secondary">{fw.version}</p>
          </div>
        </div>
        <div className="flex items-center gap-3 shrink-0 ml-4">
          <div className="text-right">
            <p className="text-xs text-secondary">{t('auditorPortal.readiness')}</p>
            <p className="font-semibold text-primary">{readiness}%</p>
          </div>
          <Button
            variant="outline"
            size="sm"
            onClick={(e) => { e.stopPropagation(); downloadAuditorFrameworkPDF(token, fw.id, fw.name) }}
            title={t('auditorPortal.downloadPdf')}
          >
            <FileText className="w-4 h-4 mr-1" />
            PDF
          </Button>
        </div>
      </div>

      {expanded && (
        <div className="border-t border-border">
          {isLoading ? (
            <p className="p-4 text-sm text-secondary">{t('auditorPortal.loading')}</p>
          ) : controls.length === 0 ? (
            <p className="p-4 text-sm text-secondary">{t('auditorPortal.noControls')}</p>
          ) : (
            <div className="divide-y divide-border">
              {controls.map((ctrl) => (
                <div key={ctrl.id} className="flex items-start gap-3 px-4 py-3">
                  <span className="text-xs font-mono text-secondary shrink-0 mt-0.5 w-32">{ctrl.control_id}</span>
                  <div className="min-w-0 flex-1">
                    <p className="text-sm font-medium text-primary truncate">{ctrl.title}</p>
                    {ctrl.domain && <p className="text-xs text-secondary">{ctrl.domain}</p>}
                  </div>
                  <ControlStatusBadge status={ctrl.manual_status || ctrl.status} />
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  )
}

function ControlStatusBadge({ status }: { status: string }) {
  const map: Record<string, { label: string; variant: 'default' | 'secondary' | 'destructive' | 'outline' }> = {
    implemented: { label: 'Implementiert', variant: 'default' },
    in_progress: { label: 'In Bearbeitung', variant: 'secondary' },
    not_implemented: { label: 'Offen', variant: 'destructive' },
    missing: { label: 'Fehlend', variant: 'destructive' },
    compliant: { label: 'Konform', variant: 'default' },
    not_applicable: { label: 'N/A', variant: 'outline' },
  }
  const { label, variant } = map[status] ?? { label: status || '—', variant: 'outline' as const }
  return <Badge variant={variant}>{label}</Badge>
}

// ---------------------------------------------------------------------------
// Risks tab
// ---------------------------------------------------------------------------

function RisksTab({ token }: { token: string }) {
  const { t } = useTranslation()
  const { data, isLoading } = useAuditorRisks(token)
  const risks = data?.data ?? []

  if (isLoading) return <p className="text-secondary text-sm p-4">{t('auditorPortal.loading')}</p>
  if (risks.length === 0) return <p className="text-secondary text-sm p-4">{t('auditorPortal.noRisks')}</p>

  return (
    <div className="space-y-2">
      {risks.map((r) => (
        <div key={r.id} className="rounded-lg border border-border bg-surface p-4 flex items-start justify-between gap-3">
          <div className="min-w-0">
            <p className="font-medium text-primary">{r.title}</p>
            {r.description && <p className="text-sm text-secondary mt-1 line-clamp-2">{r.description}</p>}
          </div>
          <Badge variant={r.treatment_status === 'implemented' ? 'default' : 'secondary'}>
            {r.treatment_status || '—'}
          </Badge>
        </div>
      ))}
    </div>
  )
}

// ---------------------------------------------------------------------------
// Incidents tab
// ---------------------------------------------------------------------------

function IncidentsTab({ token }: { token: string }) {
  const { t } = useTranslation()
  const { data, isLoading } = useAuditorIncidents(token)
  const incidents = data?.data ?? []

  const severityVariant = (s: string): 'default' | 'secondary' | 'destructive' | 'outline' => {
    if (s === 'critical' || s === 'high') return 'destructive'
    if (s === 'medium') return 'secondary'
    return 'outline'
  }

  if (isLoading) return <p className="text-secondary text-sm p-4">{t('auditorPortal.loading')}</p>
  if (incidents.length === 0) return <p className="text-secondary text-sm p-4">{t('auditorPortal.noIncidents')}</p>

  return (
    <div className="space-y-2">
      {incidents.map((inc) => (
        <div key={inc.id} className="rounded-lg border border-border bg-surface p-4 flex items-start justify-between gap-3">
          <div className="min-w-0">
            <p className="font-medium text-primary">{inc.title}</p>
            {inc.description && <p className="text-sm text-secondary mt-1 line-clamp-2">{inc.description}</p>}
          </div>
          <div className="flex gap-2 shrink-0">
            <Badge variant={severityVariant(inc.severity)}>{inc.severity}</Badge>
            <Badge variant="outline">{inc.status}</Badge>
          </div>
        </div>
      ))}
    </div>
  )
}

// ---------------------------------------------------------------------------
// Policies tab
// ---------------------------------------------------------------------------

function PoliciesTab({ token }: { token: string }) {
  const { t } = useTranslation()
  const { data, isLoading } = useAuditorPolicies(token)
  const policies = data?.data ?? []

  if (isLoading) return <p className="text-secondary text-sm p-4">{t('auditorPortal.loading')}</p>
  if (policies.length === 0) return <p className="text-secondary text-sm p-4">{t('auditorPortal.noPolicies')}</p>

  return (
    <div className="space-y-2">
      {policies.map((p) => (
        <div key={p.id} className="rounded-lg border border-border bg-surface p-4 flex items-start justify-between gap-3">
          <div className="min-w-0">
            <p className="font-medium text-primary">{p.title}</p>
            {p.category && <p className="text-xs text-secondary mt-0.5">{p.category}</p>}
          </div>
          <Badge variant={p.status === 'published' ? 'default' : 'secondary'}>{p.status}</Badge>
        </div>
      ))}
    </div>
  )
}

// ---------------------------------------------------------------------------
// Main portal
// ---------------------------------------------------------------------------

type Tab = 'frameworks' | 'risks' | 'incidents' | 'policies'

interface PortalProps {
  token: string
  onLogout: () => void
}

function Portal({ token, onLogout }: PortalProps) {
  const { t } = useTranslation()
  const [activeTab, setActiveTab] = useState<Tab>('frameworks')
  const { data: frameworks = [], isLoading, isError } = useAuditorFrameworks(token)

  const tabs: { key: Tab; label: string }[] = [
    { key: 'frameworks', label: t('auditorPortal.tabFrameworks') },
    { key: 'risks', label: t('auditorPortal.tabRisks') },
    { key: 'incidents', label: t('auditorPortal.tabIncidents') },
    { key: 'policies', label: t('auditorPortal.tabPolicies') },
  ]

  return (
    <div className="min-h-screen bg-bg">
      {/* Header */}
      <header className="border-b border-border bg-surface sticky top-0 z-10">
        <div className="max-w-5xl mx-auto px-6 py-4 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <ShieldCheck className="w-6 h-6 text-brand" />
            <div>
              <h1 className="text-base font-semibold text-primary">{t('auditorPortal.title')}</h1>
              <p className="text-xs text-secondary">{t('auditorPortal.subtitle')}</p>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => { downloadAuditorZip(token) }}
            >
              <Download className="w-4 h-4 mr-2" />
              {t('auditorPortal.downloadZip')}
            </Button>
            <Button variant="ghost" size="sm" onClick={onLogout}>
              <LogOut className="w-4 h-4 mr-2" />
              {t('auditorPortal.logoutButton')}
            </Button>
          </div>
        </div>
      </header>

      {/* Tabs */}
      <div className="border-b border-border bg-surface">
        <div className="max-w-5xl mx-auto px-6">
          <nav className="flex gap-1 -mb-px">
            {tabs.map((tab) => (
              <button
                key={tab.key}
                onClick={() => { setActiveTab(tab.key) }}
                className={`px-4 py-3 text-sm font-medium border-b-2 transition-colors ${
                  activeTab === tab.key
                    ? 'border-brand text-brand'
                    : 'border-transparent text-secondary hover:text-primary'
                }`}
              >
                {tab.label}
              </button>
            ))}
          </nav>
        </div>
      </div>

      {/* Content */}
      <main className="max-w-5xl mx-auto px-6 py-6">
        {isError && (
          <p className="text-destructive text-sm">{t('auditorPortal.invalidToken')}</p>
        )}

        {activeTab === 'frameworks' && (
          <div className="space-y-3">
            {isLoading && <p className="text-secondary text-sm">{t('auditorPortal.loading')}</p>}
            {!isLoading && frameworks.length === 0 && (
              <p className="text-secondary text-sm">{t('auditorPortal.noFrameworks')}</p>
            )}
            {frameworks.map((fw) => (
              <FrameworkRow key={fw.id} fw={fw} token={token} />
            ))}
          </div>
        )}

        {activeTab === 'risks' && <RisksTab token={token} />}
        {activeTab === 'incidents' && <IncidentsTab token={token} />}
        {activeTab === 'policies' && <PoliciesTab token={token} />}
      </main>

      <footer className="max-w-5xl mx-auto px-6 py-4 text-center text-xs text-secondary">
        {t('auditorPortal.poweredBy')}
      </footer>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Page entry point
// ---------------------------------------------------------------------------

export default function AuditorPortalPage() {
  const [token, setToken] = useState<string | null>(null)
  const [authError, setAuthError] = useState<string | undefined>()

  // Read token from sessionStorage on mount (persists within the browser tab).
  useEffect(() => {
    const stored = sessionStorage.getItem(TOKEN_KEY)
    if (stored) setToken(stored)
  }, [])

  // Also check URL search param ?token=... for deep links from AuditorAcceptPage.
  useEffect(() => {
    const params = new URLSearchParams(window.location.search)
    const urlToken = params.get('token')
    if (urlToken) {
      sessionStorage.setItem(TOKEN_KEY, urlToken)
      setToken(urlToken)
      // Clean the token from the URL to avoid sharing.
      const url = new URL(window.location.href)
      url.searchParams.delete('token')
      window.history.replaceState({}, '', url.pathname)
    }
  }, [])

  function handleLogin(newToken: string) {
    setAuthError(undefined)
    sessionStorage.setItem(TOKEN_KEY, newToken)
    setToken(newToken)
  }

  function handleLogout() {
    sessionStorage.removeItem(TOKEN_KEY)
    setToken(null)
    setAuthError(undefined)
  }

  if (!token) {
    return <LoginForm onLogin={handleLogin} error={authError} />
  }

  return (
    <Portal
      token={token}
      onLogout={handleLogout}
    />
  )
}
