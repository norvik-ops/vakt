import { Link, useLocation, useNavigate, Outlet } from 'react-router-dom'
import { useState, useEffect } from 'react'
import {
  Bug, FileCheck, Key, Fish, Eye, LayoutDashboard, LogOut, Sun, Moon, Settings,
  ShieldCheck, ShieldAlert, Siren, BookOpen, ClipboardList,
  FileText, FileSearch, Handshake, AlertTriangle, Users,
  Server, ScanSearch, BarChart2, Clock, Search, Bell,
  User, Trash2, MonitorSmartphone, Palette, Shield, Sparkles, FlaskConical,
  Building2, Bot, PackageX, Mail, GraduationCap, Target, Flag, LayoutTemplate, UserCog, Activity, UserCheck,
  Plug, ClipboardCheck, CalendarClock, Inbox, ExternalLink, Menu, X, ArrowUpCircle, ScrollText,
} from 'lucide-react'
import { useAuthStore } from '../stores/auth'
import { useThemeStore } from '../stores/theme'
import { cn } from '../../lib/utils'
import { NotificationBell } from './NotificationBell'
import { FeedbackWidget } from './FeedbackWidget'
import { useBackupStatus } from '../../hooks/useDashboard'
import { useDemoMode } from '../hooks/useDemoMode'
import { GlobalSearch } from './GlobalSearch'
import { VersionBanner } from './VersionBanner'
import { WhatsNewModal } from './WhatsNewModal'
import { useOverdueControls } from '../../modules/secvitals/hooks/useControlReviews'
import { useAutoEvidence } from '../../modules/secvitals/hooks/useEvidenceAuto'
import { useUpdateCheck } from '../hooks/useUpdateCheck'

interface NavItem {
  path: string
  label: string
  icon: React.ElementType
  exact?: boolean
  children?: { path: string; label: string; icon: React.ElementType }[]
}

const MODULES_NAV: NavItem[] = [
  { path: '/',            label: 'Dashboard',  icon: LayoutDashboard, exact: true },
  {
    path: '/secpulse',
    label: 'Vakt Scan',
    icon: Bug,
    children: [
      { path: '/secpulse/assets',   label: 'Assets',        icon: Server },
      { path: '/secpulse/findings', label: 'Findings',      icon: ScanSearch },
      { path: '/secpulse/sla',      label: 'SLA-Dashboard', icon: Clock },
      { path: '/secpulse/reports',  label: 'Berichte',      icon: BarChart2 },
      { path: '/secpulse/eol',      label: 'EOL-Dashboard', icon: PackageX },
    ],
  },
  {
    path: '/secvitals',
    label: 'Vakt Comply',
    icon: FileCheck,
    children: [
      { path: '/secvitals/frameworks', label: 'Frameworks',      icon: ShieldCheck },
      { path: '/secvitals/risks',      label: 'Risiken',         icon: ShieldAlert },
      { path: '/secvitals/incidents',  label: 'Vorfälle',        icon: Siren },
      { path: '/secvitals/policies',   label: 'Richtlinien',     icon: BookOpen },
      { path: '/secvitals/audits',     label: 'Audits',          icon: ClipboardList },
      { path: '/secvitals/suppliers',       label: 'Lieferanten',       icon: Building2 },
      { path: '/secvitals/ai-systems',        label: 'KI-Systeme',           icon: Bot },
      { path: '/secvitals/resilience-tests', label: 'Resilience-Tests',    icon: FlaskConical },
      { path: '/secvitals/eu-ai-act/dashboard', label: 'EU AI Act',          icon: Bot },
      { path: '/secvitals/dora/dashboard',   label: 'DORA Dashboard',       icon: ShieldCheck },
      { path: '/secvitals/nis2',             label: 'NIS2-Anforderungen',  icon: ShieldCheck },
      { path: '/secvitals/nis2-assistant', label: 'NIS2-Assistent',    icon: Shield },
      { path: '/secvitals/iso27001',       label: 'ISO 27001 Annex A', icon: Shield },
      { path: '/secvitals/grundschutz',    label: 'BSI Grundschutz',   icon: Shield },
      { path: '/secvitals/ai-report',      label: 'KI-Bericht',        icon: Sparkles },
      { path: '/secvitals/ccm',            label: 'CCM',               icon: Activity },
      { path: '/secvitals/capas',          label: 'Korrekturmaßnahmen', icon: ClipboardCheck },
      { path: '/secvitals/overdue-reviews', label: 'Überfällige Kontrollen', icon: CalendarClock },
      { path: '/secvitals/evidence/auto', label: 'Nachweise-Eingang', icon: Inbox },
    ],
  },
  { path: '/secvault',   label: 'Vakt Vault',    icon: Key },
  {
    path: '/secreflex',
    label: 'Vakt Aware',
    icon: Fish,
    children: [
      { path: '/secreflex/campaigns',     label: 'Kampagnen',     icon: Mail },
      { path: '/secreflex/templates',     label: 'Vorlagen',      icon: LayoutTemplate },
      { path: '/secreflex/target-groups', label: 'Zielgruppen',   icon: Target },
      { path: '/secreflex/training',      label: 'Training',      icon: GraduationCap },
      { path: '/secreflex/phish-reports', label: 'Phish-Berichte', icon: Flag },
    ],
  },
  {
    path: '/secprivacy',
    label: 'Vakt Privacy',
    icon: Eye,
    children: [
      { path: '/secprivacy/vvt',    label: 'VVT',         icon: FileText },
      { path: '/secprivacy/dpia',   label: 'DPIA',        icon: FileSearch },
      { path: '/secprivacy/avv',    label: 'AVV',         icon: Handshake },
      { path: '/secprivacy/breach', label: 'Datenpannen', icon: AlertTriangle },
      { path: '/secprivacy/dsr',    label: 'DSR',         icon: Users },
    ],
  },
  {
    path: '/hr',
    label: 'HR',
    icon: UserCog,
    children: [
      { path: '/hr/employees',  label: 'Mitarbeiter', icon: Users },
      { path: '/hr/checklists', label: 'Checklisten', icon: ClipboardList },
    ],
  },
  { path: '/integrations', label: 'Integrationen', icon: Plug },
]

export default function Layout() {
  const location = useLocation()
  const navigate = useNavigate()
  const { user, clearAuth } = useAuthStore()
  const { theme, toggle } = useThemeStore()
  const { data: backupStatus } = useBackupStatus()
  const [backupDismissed, setBackupDismissed] = useState(false)
  const [demoBannerDismissed, setDemoBannerDismissed] = useState(false)
  const [updateDismissed, setUpdateDismissed] = useState(false)
  const [sidebarOpen, setSidebarOpen] = useState(false)
  const { data: updateInfo } = useUpdateCheck()
  const isAdminOrOwner = user?.roles?.includes('admin') || user?.roles?.includes('owner')
  const demoMode = useDemoMode()
  const { data: overdueControls } = useOverdueControls()
  const overdueCount = overdueControls?.length ?? 0
  const { data: autoEvidence } = useAutoEvidence()
  const autoEvidenceCount = autoEvidence?.length ?? 0

  useEffect(() => {
    if (demoMode === true) document.title = 'Vakt Demo'
  }, [demoMode])

  function logout() {
    clearAuth()
    navigate('/login')
  }

  function isActive(path: string, exact?: boolean) {
    if (exact) return location.pathname === path
    return location.pathname === path || location.pathname.startsWith(path + '/')
  }

  return (
    <div className="flex flex-col h-screen bg-bg">
      {demoMode && !demoBannerDismissed && (
        <div className="bg-brand/10 border-b border-brand/30 px-4 py-2 flex items-center justify-between text-sm shrink-0">
          <span className="text-brand flex items-center gap-2">
            <FlaskConical className="w-4 h-4 shrink-0" />
            <strong>Demo-Umgebung</strong> — Alle Daten sind Beispieldaten. Login: <code className="mx-1 bg-brand/10 px-1 rounded">admin@vakt.local</code> / <code className="mx-1 bg-brand/10 px-1 rounded">admin1234</code>
          </span>
          <button onClick={() => setDemoBannerDismissed(true)} aria-label="Schließen" className="text-brand/60 hover:text-brand ml-4">✕</button>
        </div>
      )}
      {backupStatus?.stale && !backupDismissed && !demoMode && (
        <div className="bg-amber-50 border-b border-amber-200 px-4 py-2 flex items-center justify-between text-sm shrink-0">
          <span className="text-amber-800">
            ⚠ Kein Backup in den letzten 7 Tagen — <code>make backup</code> ausführen
          </span>
          <button onClick={() => setBackupDismissed(true)} aria-label="Schließen" className="text-amber-600 hover:text-amber-800 ml-4">✕</button>
        </div>
      )}
      <VersionBanner />
      {isAdminOrOwner && updateInfo?.update_available && !updateDismissed && (
        <div className="bg-amber-50 dark:bg-amber-950/30 border-b border-amber-200 dark:border-amber-800 px-4 py-2 flex items-center justify-between text-sm shrink-0">
          <span className="text-amber-800 dark:text-amber-300 flex items-center gap-2">
            <ArrowUpCircle className="w-4 h-4 shrink-0" />
            <span>
              <strong>Vakt {updateInfo.latest_version}</strong> ist verfügbar —{' '}
              {updateInfo.release_url ? (
                <a
                  href={updateInfo.release_url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="underline hover:text-amber-900 dark:hover:text-amber-200 font-medium"
                >
                  Jetzt aktualisieren →
                </a>
              ) : (
                <span className="font-medium">Jetzt aktualisieren</span>
              )}
            </span>
          </span>
          <button
            onClick={() => setUpdateDismissed(true)}
            aria-label="Schließen"
            className="text-amber-600 dark:text-amber-400 hover:text-amber-800 dark:hover:text-amber-200 ml-4"
          >
            ✕
          </button>
        </div>
      )}
      <div className="flex flex-1 min-h-0">
      {/* Mobile backdrop */}
      {sidebarOpen && (
        <div
          className="fixed inset-0 z-20 bg-black/40 lg:hidden"
          onClick={() => setSidebarOpen(false)}
        />
      )}
      {/* Sidebar */}
      <aside className={cn(
        'shrink-0 bg-surface border-r border-border flex flex-col',
        'fixed inset-y-0 left-0 z-30 w-[210px] transition-transform duration-200 lg:static lg:translate-x-0',
        sidebarOpen ? 'translate-x-0' : '-translate-x-full lg:translate-x-0',
      )}>

        {/* Brand */}
        <div className="px-3 pt-5 pb-4">
          <div className="flex items-center gap-2.5 px-2 mb-1">
            <img src="/logo.svg" alt="Vakt" className="w-7 h-7 shrink-0" />
            <span className="font-bold text-[18px] text-brand leading-none">Vakt</span>
            <button
              className="ml-auto lg:hidden text-secondary hover:text-primary p-1 rounded"
              onClick={() => setSidebarOpen(false)}
              aria-label="Menü schließen"
            >
              <X className="w-4 h-4" />
            </button>
          </div>
          <p className="text-[11px] text-secondary px-2">Security Platform</p>
        </div>

        {/* Search trigger */}
        <div className="px-3 pb-2">
          <button
            onClickCapture={() => window.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', metaKey: true, bubbles: true }))}
            className="w-full flex items-center gap-2 text-xs text-secondary border border-border rounded-md px-3 py-1.5 hover:border-brand/40 transition-colors"
          >
            <Search className="w-3 h-3" />
            <span>Suchen</span>
            <kbd className="ml-auto opacity-60">⌘K</kbd>
          </button>
        </div>

        {/* Nav */}
        <nav className="flex-1 px-3 overflow-y-auto">
          <p className="px-2 mb-1 text-[10px] font-semibold text-secondary uppercase tracking-wider opacity-60">
            Module
          </p>
          <div className="space-y-[2px] mb-4">
            {MODULES_NAV.map(({ path, label, icon: Icon, exact, children }) => {
              const active = isActive(path, exact)
              const expanded = active && children && children.length > 0
              return (
                <div key={path}>
                  <Link
                    to={path}
                    className={cn(
                      'flex items-center gap-2.5 px-3 py-[9px] rounded-md text-[13px] font-medium transition-all duration-150',
                      active
                        ? 'bg-[#eef2ff] dark:bg-[#1E2235] text-brand dark:text-primary'
                        : 'text-secondary hover:bg-[#f1f5f9] dark:hover:bg-[#1E2235] hover:text-primary',
                    )}
                  >
                    <Icon className={cn('w-4 h-4 shrink-0', active ? 'text-brand' : '')} />
                    {label}
                  </Link>
                  {expanded && (
                    <div className="ml-3 mt-0.5 mb-1 pl-3 border-l border-border space-y-[1px]">
                      {children.map(({ path: cp, label: cl, icon: CIcon }) => {
                        const childActive = location.pathname === cp || location.pathname.startsWith(cp + '/')
                        const isOverduePath = cp === '/secvitals/overdue-reviews'
                        const isAutoEvidencePath = cp === '/secvitals/evidence/auto'
                        return (
                          <Link
                            key={cp}
                            to={cp}
                            className={cn(
                              'flex items-center gap-2 px-2 py-[6px] rounded-md text-[12px] font-medium transition-all duration-150',
                              childActive
                                ? 'text-brand bg-[#eef2ff] dark:bg-[#1E2235]'
                                : 'text-secondary hover:text-primary hover:bg-[#f1f5f9] dark:hover:bg-[#1E2235]',
                            )}
                          >
                            <CIcon className="w-3.5 h-3.5 shrink-0" />
                            <span className="flex-1">{cl}</span>
                            {isOverduePath && overdueCount > 0 && (
                              <span className="ml-auto text-[10px] font-semibold bg-destructive text-destructive-foreground rounded-full px-1.5 py-0.5 leading-none">
                                {overdueCount}
                              </span>
                            )}
                            {isAutoEvidencePath && autoEvidenceCount > 0 && (
                              <span className="ml-auto text-[10px] font-semibold bg-brand text-white rounded-full px-1.5 py-0.5 leading-none">
                                {autoEvidenceCount}
                              </span>
                            )}
                          </Link>
                        )
                      })}
                    </div>
                  )}
                </div>
              )
            })}
          </div>

          <p className="px-2 mb-1 text-[10px] font-semibold text-secondary uppercase tracking-wider opacity-60">
            System
          </p>
          <div className="space-y-[2px]">
            <Link
              to="/settings"
              className={cn(
                'flex items-center gap-2.5 px-3 py-[9px] rounded-md text-[13px] font-medium transition-all duration-150',
                location.pathname === '/settings'
                  ? 'bg-[#eef2ff] dark:bg-[#1E2235] text-brand dark:text-primary'
                  : 'text-secondary hover:bg-[#f1f5f9] dark:hover:bg-[#1E2235] hover:text-primary',
              )}
            >
              <Settings className={cn('w-4 h-4 shrink-0', location.pathname === '/settings' ? 'text-brand' : '')} />
              Einstellungen
            </Link>
            <Link
              to="/settings/alerting"
              className={cn(
                'flex items-center gap-2.5 px-3 py-[9px] rounded-md text-[13px] font-medium transition-all duration-150',
                isActive('/settings/alerting')
                  ? 'bg-[#eef2ff] dark:bg-[#1E2235] text-brand dark:text-primary'
                  : 'text-secondary hover:bg-[#f1f5f9] dark:hover:bg-[#1E2235] hover:text-primary',
              )}
            >
              <Bell className={cn('w-4 h-4 shrink-0', isActive('/settings/alerting') ? 'text-brand' : '')} />
              Benachrichtigungen
            </Link>
            <Link
              to="/settings/retention"
              className={cn(
                'flex items-center gap-2.5 px-3 py-[9px] rounded-md text-[13px] font-medium transition-all duration-150',
                isActive('/settings/retention')
                  ? 'bg-[#eef2ff] dark:bg-[#1E2235] text-brand dark:text-primary'
                  : 'text-secondary hover:bg-[#f1f5f9] dark:hover:bg-[#1E2235] hover:text-primary',
              )}
            >
              <Trash2 className={cn('w-4 h-4 shrink-0', isActive('/settings/retention') ? 'text-brand' : '')} />
              Datenpflege
            </Link>
            <Link
              to="/settings/branding"
              className={cn(
                'flex items-center gap-2.5 px-3 py-[9px] rounded-md text-[13px] font-medium transition-all duration-150',
                isActive('/settings/branding')
                  ? 'bg-[#eef2ff] dark:bg-[#1E2235] text-brand dark:text-primary'
                  : 'text-secondary hover:bg-[#f1f5f9] dark:hover:bg-[#1E2235] hover:text-primary',
              )}
            >
              <Palette className={cn('w-4 h-4 shrink-0', isActive('/settings/branding') ? 'text-brand' : '')} />
              Branding
            </Link>
            <Link
              to="/settings/trust-center"
              className={cn(
                'flex items-center gap-2.5 px-3 py-[9px] rounded-md text-[13px] font-medium transition-all duration-150',
                isActive('/settings/trust-center')
                  ? 'bg-[#eef2ff] dark:bg-[#1E2235] text-brand dark:text-primary'
                  : 'text-secondary hover:bg-[#f1f5f9] dark:hover:bg-[#1E2235] hover:text-primary',
              )}
            >
              <Shield className={cn('w-4 h-4 shrink-0', isActive('/settings/trust-center') ? 'text-brand' : '')} />
              Trust Center
            </Link>
            <Link
              to="/settings/auditors"
              className={cn(
                'flex items-center gap-2.5 px-3 py-[9px] rounded-md text-[13px] font-medium transition-all duration-150',
                isActive('/settings/auditors')
                  ? 'bg-[#eef2ff] dark:bg-[#1E2235] text-brand dark:text-primary'
                  : 'text-secondary hover:bg-[#f1f5f9] dark:hover:bg-[#1E2235] hover:text-primary',
              )}
            >
              <UserCheck className={cn('w-4 h-4 shrink-0', isActive('/settings/auditors') ? 'text-brand' : '')} />
              Auditoren
            </Link>
            <Link
              to="/settings/team"
              className={cn(
                'flex items-center gap-2.5 px-3 py-[9px] rounded-md text-[13px] font-medium transition-all duration-150',
                isActive('/settings/team')
                  ? 'bg-[#eef2ff] dark:bg-[#1E2235] text-brand dark:text-primary'
                  : 'text-secondary hover:bg-[#f1f5f9] dark:hover:bg-[#1E2235] hover:text-primary',
              )}
            >
              <Users className={cn('w-4 h-4 shrink-0', isActive('/settings/team') ? 'text-brand' : '')} />
              Team
            </Link>
            {isAdminOrOwner && (
              <Link
                to="/settings/audit-log"
                className={cn(
                  'flex items-center gap-2.5 px-3 py-[9px] rounded-md text-[13px] font-medium transition-all duration-150',
                  isActive('/settings/audit-log')
                    ? 'bg-[#eef2ff] dark:bg-[#1E2235] text-brand dark:text-primary'
                    : 'text-secondary hover:bg-[#f1f5f9] dark:hover:bg-[#1E2235] hover:text-primary',
                )}
              >
                <ScrollText className={cn('w-4 h-4 shrink-0', isActive('/settings/audit-log') ? 'text-brand' : '')} />
                Audit-Log
              </Link>
            )}
            <Link
              to="/account"
              className={cn(
                'flex items-center gap-2.5 px-3 py-[9px] rounded-md text-[13px] font-medium transition-all duration-150',
                isActive('/account')
                  ? 'bg-[#eef2ff] dark:bg-[#1E2235] text-brand dark:text-primary'
                  : 'text-secondary hover:bg-[#f1f5f9] dark:hover:bg-[#1E2235] hover:text-primary',
              )}
            >
              <User className={cn('w-4 h-4 shrink-0', isActive('/account') ? 'text-brand' : '')} />
              Mein Account
            </Link>
            <Link
              to="/account/sessions"
              className={cn(
                'flex items-center gap-2.5 px-3 py-[9px] rounded-md text-[13px] font-medium transition-all duration-150',
                isActive('/account/sessions')
                  ? 'bg-[#eef2ff] dark:bg-[#1E2235] text-brand dark:text-primary'
                  : 'text-secondary hover:bg-[#f1f5f9] dark:hover:bg-[#1E2235] hover:text-primary',
              )}
            >
              <MonitorSmartphone className={cn('w-4 h-4 shrink-0', isActive('/account/sessions') ? 'text-brand' : '')} />
              Sitzungen
            </Link>
          </div>
        </nav>

        {/* Bottom */}
        <div className="px-3 pb-4 border-t border-border pt-3 space-y-[2px]">
          <div className="flex items-center px-3 py-[9px]">
            <NotificationBell />
          </div>
          <a
            href="https://github.com/norvik-ops/vakt/wiki"
            target="_blank"
            rel="noopener noreferrer"
            className="w-full flex items-center gap-2.5 px-3 py-[9px] rounded-md text-[13px] text-secondary hover:bg-[#f1f5f9] dark:hover:bg-[#1E2235] hover:text-primary transition-all duration-150"
          >
            <BookOpen className="w-4 h-4 shrink-0" />
            Dokumentation
            <ExternalLink className="w-3 h-3 ml-auto opacity-40" />
          </a>
          <button
            onClick={toggle}
            aria-label="Theme wechseln"
            className="w-full flex items-center gap-2.5 px-3 py-[9px] rounded-md text-[13px] text-secondary hover:bg-[#f1f5f9] dark:hover:bg-[#1E2235] hover:text-primary transition-all duration-150"
          >
            {theme === 'dark'
              ? <><Sun className="w-4 h-4 shrink-0" />Heller Modus</>
              : <><Moon className="w-4 h-4 shrink-0" />Dunkler Modus</>
            }
          </button>
          {user?.email && (
            <div className="px-3 py-1">
              <p className="text-[11px] text-secondary truncate">{user.email}</p>
            </div>
          )}
          <button
            onClick={logout}
            className="w-full flex items-center gap-2.5 px-3 py-[9px] rounded-md text-[13px] text-secondary hover:bg-[#f1f5f9] dark:hover:bg-[#1E2235] hover:text-red-500 transition-all duration-150"
          >
            <LogOut className="w-4 h-4 shrink-0" />
            Abmelden
          </button>
          <div className="px-3 py-2 border-t border-border mt-1">
            <p className="text-[10px] text-secondary/50">© 2026 NorvikOps · ELv2</p>
          </div>
        </div>
      </aside>

      {/* Main */}
      <main className="flex-1 overflow-auto bg-bg flex flex-col min-w-0">
        {/* Mobile top bar with hamburger */}
        <div className="lg:hidden flex items-center gap-3 px-4 py-3 border-b border-border bg-surface shrink-0">
          <button
            onClick={() => setSidebarOpen(true)}
            aria-label="Menü öffnen"
            className="text-secondary hover:text-primary p-1 rounded"
          >
            <Menu className="w-5 h-5" />
          </button>
          <div className="flex items-center gap-2">
            <img src="/logo.svg" alt="Vakt" className="w-5 h-5 shrink-0" />
            <span className="font-bold text-[15px] text-brand leading-none">Vakt</span>
          </div>
        </div>
        <div className="flex-1 overflow-auto">
          <Outlet />
        </div>
      </main>
      </div>
      <GlobalSearch />
      {demoMode && <FeedbackWidget />}
      <WhatsNewModal />
    </div>
  )
}
