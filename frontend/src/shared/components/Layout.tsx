import { Link, useLocation, useNavigate, Outlet } from 'react-router-dom'
import { useState, useEffect } from 'react'
import {
  Bug, FileCheck, Key, Fish, Eye, LayoutDashboard, LogOut, Sun, Moon, Monitor, Settings,
  ShieldCheck, ShieldAlert, Siren, BookOpen, ClipboardList,
  FileText, FileSearch, Handshake, AlertTriangle, Users,
  Server, ScanSearch, BarChart2, Clock, Search, Bell,
  User, Trash2, MonitorSmartphone, Palette, Shield, FlaskConical,
  Building2, Bot, PackageX, Mail, GraduationCap, Target, Flag, LayoutTemplate, UserCog, Activity, UserCheck,
  Plug, ClipboardCheck, CalendarClock, Inbox, ExternalLink, Menu, X, ArrowUpCircle, ScrollText, HeartPulse, CalendarDays,
  ChevronLeft, ChevronRight, HelpCircle, Webhook, FileBarChart2,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '../stores/auth'
import { useThemeStore } from '../stores/theme'
import { cn } from '../../lib/utils'
import { NotificationBell } from './NotificationBell'
import { FeedbackWidget } from './FeedbackWidget'
import { useBackupStatus } from '../../hooks/useDashboard'
import { useDemoMode } from '../hooks/useDemoMode'
import { GlobalSearch } from './GlobalSearch'
import { VersionBanner } from './VersionBanner'
import { LicenseExpiryBanner } from './LicenseExpiryBanner'
import { WhatsNewModal } from './WhatsNewModal'
import { useOverdueControls } from '../../modules/secvitals/hooks/useControlReviews'
import { useAutoEvidence } from '../../modules/secvitals/hooks/useEvidenceAuto'
import { usePendingApprovalCount } from '../../modules/secvitals/hooks/useApprovals'
import { useUpdateCheck } from '../hooks/useUpdateCheck'
import { Toaster } from './Toaster'
import { PWAInstallPrompt } from './PWAInstallPrompt'
import { PageTransition } from './PageTransition'
import { useKeyboardShortcuts } from '../hooks/useKeyboardShortcuts'
import { KeyboardShortcutsModal } from './KeyboardShortcutsModal'
import { AppTour } from './AppTour'
import { ChangelogPopover } from './ChangelogPopover'

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
      { path: '/secvitals/soa',        label: 'Statement of Applicability', icon: FileCheck },
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
      { path: '/secvitals/cis-controls',  label: 'CIS Controls v8',    icon: Shield },
      { path: '/secvitals/ccm',            label: 'CCM',               icon: Activity },
      { path: '/secvitals/capas',          label: 'Korrekturmaßnahmen', icon: ClipboardCheck },
      { path: '/secvitals/overdue-reviews', label: 'Überfällige Kontrollen', icon: CalendarClock },
      { path: '/secvitals/evidence/auto', label: 'Nachweise-Eingang', icon: Inbox },
      { path: '/secvitals/approvals',     label: 'Genehmigungen',     icon: UserCheck },
      { path: '/secvitals/certification-timeline', label: 'Zertifizierungs-Timeline', icon: CalendarDays },
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

const SIDEBAR_COLLAPSED_KEY = 'vakt_sidebar_collapsed'

export default function Layout() {
  const { t } = useTranslation()
  const location = useLocation()
  const navigate = useNavigate()
  const { user, clearAuth } = useAuthStore()
  const { theme, toggle } = useThemeStore()
  const { data: backupStatus } = useBackupStatus()
  const [backupDismissed, setBackupDismissed] = useState(false)
  const [demoBannerDismissed, setDemoBannerDismissed] = useState(false)
  const [updateDismissed, setUpdateDismissed] = useState(false)
  const [sidebarOpen, setSidebarOpen] = useState(false)
  const [sidebarCollapsed, setSidebarCollapsed] = useState(
    () => localStorage.getItem(SIDEBAR_COLLAPSED_KEY) === 'true',
  )
  const [shortcutsOpen, setShortcutsOpen] = useState(false)
  const { data: updateInfo } = useUpdateCheck()
  const isAdminOrOwner = user?.roles?.includes('admin') || user?.roles?.includes('owner')
  const demoMode = useDemoMode()
  const { data: overdueControls } = useOverdueControls()
  const overdueCount = overdueControls?.length ?? 0
  const { data: autoEvidence } = useAutoEvidence()
  const autoEvidenceCount = autoEvidence?.length ?? 0
  const { data: pendingApprovalData } = usePendingApprovalCount()
  const pendingApprovalCount = pendingApprovalData?.count ?? 0

  useKeyboardShortcuts({ onOpenHelp: () => setShortcutsOpen(true) })

  function toggleSidebarCollapsed() {
    setSidebarCollapsed((prev) => {
      const next = !prev
      localStorage.setItem(SIDEBAR_COLLAPSED_KEY, String(next))
      return next
    })
  }

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
      {/* Skip to main content */}
      <a
        href="#main-content"
        className="sr-only focus:not-sr-only focus:absolute focus:top-4 focus:left-4 z-50 bg-background px-4 py-2 rounded-lg border font-medium"
      >
        {t('nav.skipToContent')}
      </a>

      {demoMode && !demoBannerDismissed && (
        <div className="bg-brand/10 border-b border-brand/30 px-4 py-2 flex items-center justify-between text-sm shrink-0">
          <span className="text-brand flex items-center gap-2">
            <FlaskConical className="w-4 h-4 shrink-0" />
            <strong>{t('demo.banner')}</strong> — {t('demo.description')} Login: <code className="mx-1 bg-brand/10 px-1 rounded">admin@vakt.local</code> / <code className="mx-1 bg-brand/10 px-1 rounded">admin1234</code>
          </span>
          <button onClick={() => setDemoBannerDismissed(true)} aria-label={t('common.close')} className="text-brand/60 hover:text-brand ml-4">✕</button>
        </div>
      )}
      {backupStatus?.stale && !backupDismissed && !demoMode && (
        <div className="bg-amber-50 border-b border-amber-200 px-4 py-2 flex items-center justify-between text-sm shrink-0">
          <span className="text-amber-800">
            ⚠ {t('backup.staleWarning')} — <code>make backup</code> ausführen
          </span>
          <button onClick={() => setBackupDismissed(true)} aria-label={t('common.close')} className="text-amber-600 hover:text-amber-800 ml-4">✕</button>
        </div>
      )}
      <VersionBanner />
      {isAdminOrOwner && updateInfo?.update_available && !updateDismissed && (
        <div className="bg-amber-50 dark:bg-amber-950/30 border-b border-amber-200 dark:border-amber-800 px-4 py-2 flex items-center justify-between text-sm shrink-0">
          <span className="text-amber-800 dark:text-amber-300 flex items-center gap-2">
            <ArrowUpCircle className="w-4 h-4 shrink-0" />
            <span>
              <strong>Vakt {updateInfo.latest_version}</strong> {t('update.available')} —{' '}
              {updateInfo.release_url ? (
                <a
                  href={updateInfo.release_url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="underline hover:text-amber-900 dark:hover:text-amber-200 font-medium"
                >
                  {t('update.updateNow')}
                </a>
              ) : (
                <span className="font-medium">{t('update.updateNowLabel')}</span>
              )}
            </span>
          </span>
          <button
            onClick={() => setUpdateDismissed(true)}
            aria-label={t('common.close')}
            className="text-amber-600 dark:text-amber-400 hover:text-amber-800 dark:hover:text-amber-200 ml-4"
          >
            ✕
          </button>
        </div>
      )}
      <LicenseExpiryBanner />
      <div className="flex flex-1 min-h-0">
      {/* Mobile backdrop */}
      {sidebarOpen && (
        /* WCAG 2.1.1: keyboard-accessible dismiss — tabIndex={-1} keeps it out of tab order
           but allows Escape to close via the document-level keydown listener */
        <div
          className="fixed inset-0 z-20 bg-black/40 lg:hidden"
          onClick={() => setSidebarOpen(false)}
          aria-hidden="true"
        />
      )}
      {/* Sidebar */}
      <aside
        aria-expanded={!sidebarCollapsed}
        className={cn(
          'shrink-0 bg-surface border-r border-border flex flex-col',
          'fixed inset-y-0 left-0 z-30 transition-all duration-200 lg:static lg:translate-x-0',
          sidebarOpen ? 'translate-x-0' : '-translate-x-full lg:translate-x-0',
          sidebarCollapsed ? 'w-[56px]' : 'w-[210px]',
        )}
      >

        {/* Brand */}
        <div className={cn('px-3 pt-5 pb-4', sidebarCollapsed && 'px-2')}>
          <div className={cn('flex items-center gap-2.5 px-2 mb-1', sidebarCollapsed && 'justify-center px-0')}>
            <img src="/logo.svg" alt="Vakt" className="w-7 h-7 shrink-0" title="Vakt" />
            {!sidebarCollapsed && <span className="font-bold text-[18px] text-brand leading-none">Vakt</span>}
            {!sidebarCollapsed && (
              <button
                className="ml-auto lg:hidden text-secondary hover:text-primary p-1 rounded"
                onClick={() => setSidebarOpen(false)}
                aria-label={t('nav.closeMenu')}
              >
                <X className="w-4 h-4" aria-hidden="true" />
              </button>
            )}
          </div>
          {!sidebarCollapsed && <p className="text-[11px] text-secondary px-2">Security Platform</p>}
        </div>

        {/* Search trigger */}
        {!sidebarCollapsed && (
          <div className="px-3 pb-2">
            <button
              type="button"
              aria-label="Globale Suche öffnen (Cmd+K)"
              onClickCapture={() => window.dispatchEvent(new CustomEvent('vakt:open-search'))}
              className="w-full flex items-center gap-2 text-xs text-secondary border border-border rounded-md px-3 py-1.5 hover:border-brand/40 transition-colors"
            >
              {/* WCAG 1.1.1: search icon is decorative, button is named by aria-label */}
              <Search className="w-3 h-3" aria-hidden="true" />
              <span>{t('nav.search')}</span>
              <kbd className="ml-auto opacity-60" aria-hidden="true">⌘K</kbd>
            </button>
          </div>
        )}
        {sidebarCollapsed && (
          <div className="px-2 pb-2">
            <button
              type="button"
              aria-label="Globale Suche öffnen (Cmd+K)"
              title="Suche (⌘K)"
              onClickCapture={() => window.dispatchEvent(new CustomEvent('vakt:open-search'))}
              className="w-full flex items-center justify-center p-2 text-secondary border border-border rounded-md hover:border-brand/40 transition-colors"
            >
              <Search className="w-4 h-4" aria-hidden="true" />
            </button>
          </div>
        )}

        {/* Nav */}
        <nav role="navigation" aria-label={t('nav.mainNav')} className={cn('flex-1 overflow-y-auto', sidebarCollapsed ? 'px-2' : 'px-3')}>
          {!sidebarCollapsed && (
            <p className="px-2 mb-1 text-[10px] font-semibold text-secondary uppercase tracking-wider opacity-60">
              {t('nav.modules')}
            </p>
          )}
          <div className="space-y-[2px] mb-4">
            {MODULES_NAV.map(({ path, label, icon: Icon, exact, children }) => {
              const active = isActive(path, exact)
              const expanded = active && children && children.length > 0
              return (
                <div key={path}>
                  {/* WCAG 2.4.4 + 4.1.2: aria-current="page" identifies the active link */}
                  <Link
                    to={path}
                    onClick={() => setSidebarOpen(false)}
                    aria-current={active ? 'page' : undefined}
                    title={sidebarCollapsed ? label : undefined}
                    className={cn(
                      'flex items-center rounded-md text-[13px] font-medium transition-all duration-150',
                      sidebarCollapsed ? 'justify-center p-2' : 'gap-2.5 px-3 py-[9px]',
                      active
                        ? 'bg-brand/10 dark:bg-muted/50 text-brand dark:text-primary'
                        : 'text-secondary hover:bg-muted/50 hover:text-primary',
                    )}
                  >
                    {/* WCAG 1.1.1: nav icons are decorative — label comes from text */}
                    <Icon className={cn('w-4 h-4 shrink-0', active ? 'text-brand' : '')} aria-hidden="true" />
                    {!sidebarCollapsed && label}
                  </Link>
                  {expanded && !sidebarCollapsed && (
                    <div className="ml-3 mt-0.5 mb-1 pl-3 border-l border-border space-y-[1px]">
                      {children.map(({ path: cp, label: cl, icon: CIcon }) => {
                        const childActive = location.pathname === cp || location.pathname.startsWith(cp + '/')
                        const isOverduePath = cp === '/secvitals/overdue-reviews'
                        const isAutoEvidencePath = cp === '/secvitals/evidence/auto'
                        const isApprovalsPath = cp === '/secvitals/approvals'
                        return (
                          <Link
                            key={cp}
                            to={cp}
                            onClick={() => setSidebarOpen(false)}
                            aria-current={childActive ? 'page' : undefined}
                            className={cn(
                              'flex items-center gap-2 px-2 py-[6px] rounded-md text-[12px] font-medium transition-all duration-150',
                              childActive
                                ? 'text-brand bg-brand/10 dark:bg-muted/50'
                                : 'text-secondary hover:text-primary hover:bg-muted/50',
                            )}
                          >
                            <CIcon className="w-3.5 h-3.5 shrink-0" aria-hidden="true" />
                            <span className="flex-1">{cl}</span>
                            {isOverduePath && overdueCount > 0 && (
                              <span
                                className="ml-auto text-[10px] font-semibold bg-destructive text-destructive-foreground rounded-full px-1.5 py-0.5 leading-none"
                                aria-label={`${overdueCount} überfällige Kontrollen`}
                              >
                                {overdueCount}
                              </span>
                            )}
                            {isAutoEvidencePath && autoEvidenceCount > 0 && (
                              <span
                                className="ml-auto text-[10px] font-semibold bg-brand text-white rounded-full px-1.5 py-0.5 leading-none"
                                aria-label={`${autoEvidenceCount} neue Nachweise`}
                              >
                                {autoEvidenceCount}
                              </span>
                            )}
                            {isApprovalsPath && pendingApprovalCount > 0 && (
                              <span
                                className="ml-auto text-[10px] font-semibold bg-amber-500 text-white rounded-full px-1.5 py-0.5 leading-none"
                                aria-label={`${pendingApprovalCount} ausstehende Genehmigungen`}
                              >
                                {pendingApprovalCount}
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

          {!sidebarCollapsed && (
            <p className="px-2 mb-1 text-[10px] font-semibold text-secondary uppercase tracking-wider opacity-60">
              {t('nav.system')}
            </p>
          )}
          {/* WCAG 2.4.4 + 4.1.2: aria-current="page" on each active system link */}
          <div className="space-y-[2px]">
            {[
              { to: '/settings', icon: Settings, label: t('nav.settings'), exact: true },
              { to: '/settings/alerting', icon: Bell, label: t('nav.alerting') },
              { to: '/settings/retention', icon: Trash2, label: t('nav.retention') },
              { to: '/settings/branding', icon: Palette, label: 'Branding' },
              { to: '/settings/trust-center', icon: Shield, label: 'Trust Center' },
              { to: '/settings/auditors', icon: UserCheck, label: 'Auditoren' },
              { to: '/settings/team', icon: Users, label: 'Team' },
              { to: '/settings/webhooks', icon: Webhook, label: 'Webhooks' },
              { to: '/settings/reports', icon: FileBarChart2, label: 'Geplante Berichte' },
              { to: '/settings/notifications', icon: Bell, label: 'Benachrichtigungen' },
              ...(isAdminOrOwner ? [
                { to: '/settings/audit-log', icon: ScrollText, label: 'Audit-Log' },
                { to: '/admin/health', icon: HeartPulse, label: 'System-Status' },
                { to: '/admin/tenants', icon: Building2, label: 'Mandanten' },
                { to: '/admin/security', icon: ShieldAlert, label: 'Sicherheitsereignisse' },
              ] : []),
              { to: '/account', icon: User, label: t('nav.account') },
              { to: '/account/sessions', icon: MonitorSmartphone, label: t('nav.sessions') },
            ].map(({ to, icon: Icon, label, exact }) => {
              const active = exact ? location.pathname === to : isActive(to)
              return (
                <Link
                  key={to}
                  to={to}
                  onClick={() => setSidebarOpen(false)}
                  aria-current={active ? 'page' : undefined}
                  title={sidebarCollapsed ? label : undefined}
                  className={cn(
                    'flex items-center rounded-md text-[13px] font-medium transition-all duration-150',
                    sidebarCollapsed ? 'justify-center p-2' : 'gap-2.5 px-3 py-[9px]',
                    active
                      ? 'bg-brand/10 dark:bg-muted/50 text-brand dark:text-primary'
                      : 'text-secondary hover:bg-muted/50 hover:text-primary',
                  )}
                >
                  <Icon className={cn('w-4 h-4 shrink-0', active ? 'text-brand' : '')} aria-hidden="true" />
                  {!sidebarCollapsed && label}
                </Link>
              )
            })}
          </div>
        </nav>

        {/* Bottom */}
        <div className={cn('pb-4 border-t border-border pt-3 space-y-[2px]', sidebarCollapsed ? 'px-2' : 'px-3')}>
          {/* Collapse toggle */}
          <button
            onClick={toggleSidebarCollapsed}
            aria-label={sidebarCollapsed ? 'Sidebar ausklappen' : 'Sidebar einklappen'}
            title={sidebarCollapsed ? 'Sidebar ausklappen' : 'Sidebar einklappen'}
            className={cn(
              'w-full flex items-center rounded-md text-[13px] text-secondary hover:bg-muted/50 hover:text-primary transition-all duration-150',
              sidebarCollapsed ? 'justify-center p-2' : 'gap-2.5 px-3 py-[9px]',
            )}
          >
            {sidebarCollapsed
              ? <ChevronRight className="w-4 h-4 shrink-0" aria-hidden="true" />
              : <><ChevronLeft className="w-4 h-4 shrink-0" aria-hidden="true" /><span>Einklappen</span></>
            }
          </button>

          {/* Help / keyboard shortcuts */}
          <button
            onClick={() => setShortcutsOpen(true)}
            aria-label="Tastaturkürzel anzeigen"
            title="Tastaturkürzel (?)"
            className={cn(
              'w-full flex items-center rounded-md text-[13px] text-secondary hover:bg-muted/50 hover:text-primary transition-all duration-150',
              sidebarCollapsed ? 'justify-center p-2' : 'gap-2.5 px-3 py-[9px]',
            )}
          >
            <HelpCircle className="w-4 h-4 shrink-0" aria-hidden="true" />
            {!sidebarCollapsed && 'Tastaturkürzel'}
          </button>

          <div className={cn('flex items-center gap-1 py-[9px]', sidebarCollapsed ? 'justify-center' : 'px-3')}>
            <NotificationBell />
            <ChangelogPopover />
          </div>

          {!sidebarCollapsed && (
            <a
              href="https://github.com/norvik-ops/vakt/wiki"
              target="_blank"
              rel="noopener noreferrer"
              className="w-full flex items-center gap-2.5 px-3 py-[9px] rounded-md text-[13px] text-secondary hover:bg-muted/50 hover:text-primary transition-all duration-150"
            >
              <BookOpen className="w-4 h-4 shrink-0" aria-hidden="true" />
              {t('nav.documentation')}
              {/* WCAG 2.4.4: external-link icon is decorative; label names the link */}
              <ExternalLink className="w-3 h-3 ml-auto opacity-40" aria-hidden="true" />
            </a>
          )}
          {sidebarCollapsed && (
            <a
              href="https://github.com/norvik-ops/vakt/wiki"
              target="_blank"
              rel="noopener noreferrer"
              title="Dokumentation"
              className="w-full flex items-center justify-center p-2 rounded-md text-secondary hover:bg-muted/50 hover:text-primary transition-all duration-150"
            >
              <BookOpen className="w-4 h-4 shrink-0" aria-hidden="true" />
            </a>
          )}

          <button
            onClick={toggle}
            aria-label={
              theme === 'light'
                ? 'Zu dunklem Modus wechseln'
                : theme === 'dark'
                ? 'Zu System-Modus wechseln'
                : 'Zu hellem Modus wechseln'
            }
            title={theme === 'light' ? 'Dunkel' : theme === 'dark' ? 'System' : 'Hell'}
            className={cn(
              'w-full flex items-center rounded-md text-[13px] text-secondary hover:bg-muted/50 hover:text-primary transition-all duration-150',
              sidebarCollapsed ? 'justify-center p-2' : 'gap-2.5 px-3 py-[9px]',
            )}
          >
            {theme === 'light' ? (
              <><Moon className="w-4 h-4 shrink-0" aria-hidden="true" />{!sidebarCollapsed && t('theme.dark')}</>
            ) : theme === 'dark' ? (
              <><Monitor className="w-4 h-4 shrink-0" aria-hidden="true" />{!sidebarCollapsed && 'System'}</>
            ) : (
              <><Sun className="w-4 h-4 shrink-0" aria-hidden="true" />{!sidebarCollapsed && t('theme.light')}</>
            )}
          </button>
          {!sidebarCollapsed && user?.email && (
            <div className="px-3 py-1">
              <p className="text-[11px] text-secondary truncate">{user.email}</p>
            </div>
          )}
          <button
            onClick={logout}
            title="Abmelden"
            className={cn(
              'w-full flex items-center rounded-md text-[13px] text-secondary hover:bg-muted/50 hover:text-red-500 transition-all duration-150',
              sidebarCollapsed ? 'justify-center p-2' : 'gap-2.5 px-3 py-[9px]',
            )}
          >
            <LogOut className="w-4 h-4 shrink-0" aria-hidden="true" />
            {!sidebarCollapsed && t('auth.logout')}
          </button>
          {!sidebarCollapsed && (
            <div className="px-3 py-2 border-t border-border mt-1">
              <p className="text-[10px] text-secondary/50">© 2026 NorvikOps · ELv2</p>
            </div>
          )}
        </div>
      </aside>

      {/* Main */}
      <main id="main-content" role="main" className="flex-1 overflow-auto bg-bg flex flex-col min-w-0 pb-16 md:pb-0">
        {/* Mobile top bar with hamburger */}
        <div className="lg:hidden flex items-center gap-3 px-4 py-3 border-b border-border bg-surface shrink-0">
          <button
            onClick={() => setSidebarOpen(true)}
            aria-label={t('nav.openMenu')}
            className="text-secondary hover:text-primary p-1 rounded"
          >
            {/* WCAG 1.1.1: icon is decorative, button is named by aria-label */}
            <Menu className="w-5 h-5" aria-hidden="true" />
          </button>
          <div className="flex items-center gap-2 flex-1">
            <img src="/logo.svg" alt="Vakt" className="w-5 h-5 shrink-0" />
            <span className="font-bold text-[15px] text-brand leading-none">Vakt</span>
          </div>
          {/* Changelog — mobile only */}
          <ChangelogPopover />
          {/* Theme toggle — mobile only */}
          <button
            onClick={toggle}
            aria-label={
              theme === 'light'
                ? 'Zu dunklem Modus wechseln'
                : theme === 'dark'
                ? 'Zu System-Modus wechseln'
                : 'Zu hellem Modus wechseln'
            }
            title={theme === 'light' ? 'Dunkel' : theme === 'dark' ? 'System' : 'Hell'}
            className="p-1.5 rounded-lg text-secondary hover:text-primary hover:bg-surface2"
          >
            {theme === 'light' ? (
              <Sun className="w-4 h-4" aria-hidden="true" />
            ) : theme === 'dark' ? (
              <Moon className="w-4 h-4" aria-hidden="true" />
            ) : (
              <Monitor className="w-4 h-4" aria-hidden="true" />
            )}
          </button>
        </div>
        <div className="flex-1 overflow-auto">
          <PageTransition>
            <Outlet />
          </PageTransition>
        </div>
      </main>
      </div>
      {/* Mobile bottom navigation */}
      <nav
        aria-label="Mobile Navigation"
        className="md:hidden fixed bottom-0 left-0 right-0 bg-surface border-t border-border z-30 flex"
      >
        {[
          { label: 'Comply', path: '/secvitals', icon: ShieldCheck },
          { label: 'Scan', path: '/secpulse', icon: Bug },
          { label: 'Privacy', path: '/secprivacy', icon: Eye },
          { label: 'HR', path: '/hr', icon: Users },
        ].map(({ label, path, icon: Icon }) => {
          const active = isActive(path)
          return (
            <Link
              key={path}
              to={path}
              aria-current={active ? 'page' : undefined}
              className={`flex-1 flex flex-col items-center py-2 text-xs transition-colors ${
                active ? 'text-brand' : 'text-secondary hover:text-brand'
              }`}
            >
              <Icon className="h-5 w-5 mb-1" aria-hidden="true" />
              {label}
            </Link>
          )
        })}
      </nav>
      <GlobalSearch />
      <KeyboardShortcutsModal open={shortcutsOpen} onClose={() => setShortcutsOpen(false)} />
      {demoMode && <FeedbackWidget />}
      <WhatsNewModal />
      <Toaster />
      <PWAInstallPrompt />
      <AppTour />
    </div>
  )
}
