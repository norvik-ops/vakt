import { Link, useLocation, Outlet } from 'react-router-dom'
import { useState, useEffect, useRef, Suspense } from 'react'
import {
  Bug, FileCheck, Key, Fish, Eye, LayoutDashboard, Sun, Moon, Monitor, Settings,
  ShieldCheck, ShieldAlert, Siren, BookOpen, ClipboardList,
  FileText, FileSearch, Handshake, AlertTriangle, Users,
  Server, ScanSearch, BarChart2, Clock, Search,
  Shield, FlaskConical,
  Building2, Bot, PackageX, Mail, GraduationCap, Target, Flag, LayoutTemplate, UserCog, UserCheck,
  Plug, ClipboardCheck, CalendarClock, Inbox, Menu, X, ArrowUpCircle, ScrollText, CalendarDays,
  ChevronLeft, ChevronRight, Cpu, Landmark, ListChecks, Cloud, Banknote, ChevronDown,
  LayoutGrid, FileBarChart, Globe, ActivitySquare, Phone, DatabaseBackup,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { ProBadge } from './ProBadge'
import { BetaBadge } from './BetaBadge'
import { useAuthStore } from '../stores/auth'
import { useThemeStore } from '../stores/theme'
import { cn } from '../../lib/utils'
import { FeedbackWidget } from './FeedbackWidget'
import { useBackupStatus } from '../../hooks/useDashboard'
import { useDemoMode } from '../hooks/useDemoMode'
import { GlobalSearch } from './GlobalSearch'
import { VersionBanner } from './VersionBanner'
import { LicenseExpiryBanner } from './LicenseExpiryBanner'
import { WhatsNewModal } from './WhatsNewModal'
import { useOverdueControls } from '../../modules/vaktcomply/hooks/useControlReviews'
import { useAutoEvidence } from '../../modules/vaktcomply/hooks/useEvidenceAuto'
import { usePendingApprovalCount } from '../hooks/useApprovals'
import { useUpdateCheck } from '../hooks/useUpdateCheck'
import { Toaster } from './Toaster'
import { PWAInstallPrompt } from './PWAInstallPrompt'
import { PageTransition } from './PageTransition'
import { useKeyboardShortcuts } from '../hooks/useKeyboardShortcuts'
import { KeyboardShortcutsModal } from './KeyboardShortcutsModal'
import { AppTour } from './AppTour'
import { Spinner } from '../../components/Spinner'
import { TopBar } from './TopBar'

interface NavChild {
  path: string
  label: string
  icon: React.ElementType
  pro?: boolean
}

interface NavGroup {
  label: string
  items: NavChild[]
}

interface NavItem {
  path: string
  label: string
  icon: React.ElementType
  exact?: boolean
  children?: NavChild[]
  childGroups?: NavGroup[]
}

const MODULES_NAV: NavItem[] = [
  { path: '/',            label: 'nav.dashboard',  icon: LayoutDashboard, exact: true },
  {
    path: '/vaktscan',
    label: 'nav.scan.root',
    icon: Bug,
    children: [
      { path: '/vaktscan/assets',       label: 'nav.scan.assets',       icon: Server },
      { path: '/vaktscan/findings',     label: 'nav.scan.findings',     icon: ScanSearch },
      { path: '/vaktscan/sla',          label: 'nav.scan.sla',          icon: Clock },
      { path: '/vaktscan/reports',      label: 'nav.scan.reports',      icon: BarChart2 },
      { path: '/vaktscan/eol',          label: 'nav.scan.eol',          icon: PackageX },
      { path: '/vaktscan/certificates', label: 'nav.scan.certificates', icon: ShieldCheck },
    ],
  },
  {
    path: '/vaktcomply',
    label: 'nav.comply.root',
    icon: FileCheck,
    childGroups: [
      {
        label: 'nav.comply.group.frameworks',
        items: [
          { path: '/vaktcomply/frameworks',          label: 'nav.comply.overview',     icon: ShieldCheck },
          { path: '/vaktcomply/nis2',                label: 'nav.comply.nis2',          icon: Shield },
          { path: '/vaktcomply/iso27001',            label: 'nav.comply.iso27001',      icon: FileCheck },
          { path: '/vaktcomply/grundschutz',         label: 'nav.comply.bsi',           icon: Landmark },
          { path: '/vaktcomply/bsi/target-objects',  label: 'nav.comply.bsiCheck',      icon: ClipboardCheck, pro: true },
          { path: '/vaktcomply/bsi/cockpit',         label: 'nav.comply.bsiCockpit',    icon: LayoutGrid, pro: true },
          { path: '/vaktcomply/bsi/reports',         label: 'nav.comply.bsiReports',    icon: FileBarChart, pro: true },
          { path: '/vaktcomply/cis-controls',        label: 'nav.comply.cisv8',         icon: ListChecks },
          { path: '/vaktcomply/ccm',                 label: 'nav.comply.ccm',           icon: Cloud, pro: true },
          { path: '/vaktcomply/dora/dashboard',      label: 'nav.comply.dora',          icon: Banknote, pro: true },
          { path: '/vaktcomply/eu-ai-act/dashboard', label: 'nav.comply.euAiAct',       icon: Bot, pro: true },
        ],
      },
      {
        label: 'nav.comply.group.operations',
        items: [
          { path: '/vaktcomply/risks',           label: 'nav.comply.risks',       icon: ShieldAlert },
          { path: '/vaktcomply/incidents',       label: 'nav.comply.incidents',   icon: Siren },
          { path: '/vaktcomply/audits',          label: 'nav.comply.audits',      icon: ClipboardList },
          { path: '/vaktcomply/capas',           label: 'nav.comply.capas',       icon: ClipboardCheck },
          { path: '/vaktcomply/approvals',       label: 'nav.comply.approvals',   icon: UserCheck },
          { path: '/vaktcomply/overdue-reviews', label: 'nav.comply.overdue',     icon: CalendarClock },
        ],
      },
      {
        label: 'nav.comply.group.documentation',
        items: [
          { path: '/vaktcomply/policies',               label: 'nav.comply.policies',     icon: BookOpen },
          { path: '/vaktcomply/soa',                    label: 'nav.comply.soa',           icon: ScrollText },
          { path: '/vaktcomply/evidence/auto',          label: 'nav.comply.evidence',      icon: Inbox },
          { path: '/vaktcomply/crypto-keys',            label: 'nav.comply.cryptography',  icon: Key },
          { path: '/vaktcomply/certification-timeline', label: 'nav.comply.certTimeline',  icon: CalendarDays },
        ],
      },
      {
        label: 'nav.comply.group.thirdParty',
        items: [
          { path: '/vaktcomply/suppliers',        label: 'nav.comply.suppliers',  icon: Building2 },
          { path: '/vaktcomply/ai-systems',       label: 'nav.comply.aiSystems',  icon: Cpu },
          { path: '/vaktcomply/resilience-tests', label: 'nav.comply.resilience', icon: FlaskConical },
        ],
      },
      {
        label: 'nav.comply.group.bcm',
        items: [
          { path: '/vaktcomply/bcm',                      label: 'nav.comply.bcmDashboard',       icon: ShieldCheck, pro: true },
          { path: '/vaktcomply/bcm/bia',                  label: 'nav.comply.bcmBia',             icon: ActivitySquare, pro: true },
          { path: '/vaktcomply/bcm/recovery-plans',       label: 'nav.comply.bcmRecoveryPlans',   icon: ListChecks, pro: true },
          { path: '/vaktcomply/bcm/emergency-contacts',   label: 'nav.comply.bcmEmergencyContacts', icon: Phone, pro: true },
          { path: '/vaktcomply/backup',                   label: 'nav.comply.backup',             icon: DatabaseBackup },
        ],
      },
    ],
  },
  {
    path: '/vaktvault',
    label: 'nav.vault.root',
    icon: Key,
    children: [
      { path: '/vaktvault/projects',       label: 'nav.vault.projects',      icon: Key },
      { path: '/vaktvault/tokens',         label: 'nav.vault.tokens',        icon: Shield },
      { path: '/vaktvault/git-scans',      label: 'nav.vault.gitScans',      icon: ScanSearch, pro: true },
      { path: '/vaktvault/access-reviews', label: 'nav.vault.accessReviews', icon: UserCheck, pro: true },
    ],
  },
  {
    path: '/vaktaware',
    label: 'nav.aware.root',
    icon: Fish,
    children: [
      { path: '/vaktaware/campaigns',     label: 'nav.aware.campaigns',    icon: Mail },
      { path: '/vaktaware/templates',     label: 'nav.aware.templates',    icon: LayoutTemplate },
      { path: '/vaktaware/target-groups', label: 'nav.aware.targetGroups', icon: Target },
      { path: '/vaktaware/training',      label: 'nav.aware.training',     icon: GraduationCap },
      { path: '/vaktaware/phish-reports', label: 'nav.aware.phishReports', icon: Flag },
    ],
  },
  {
    path: '/vaktprivacy',
    label: 'nav.privacy.root',
    icon: Eye,
    children: [
      { path: '/vaktprivacy/vvt',               label: 'nav.privacy.vvt',            icon: FileText },
      { path: '/vaktprivacy/dpia',              label: 'nav.privacy.dpia',           icon: FileSearch, pro: true },
      { path: '/vaktprivacy/avv',               label: 'nav.privacy.avv',            icon: Handshake, pro: true },
      { path: '/vaktprivacy/breach',            label: 'nav.privacy.breach',         icon: AlertTriangle },
      { path: '/vaktprivacy/dsr',               label: 'nav.privacy.dsr',            icon: Users },
      { path: '/vaktprivacy/transfers',         label: 'nav.privacy.tia',            icon: Globe, pro: true },
      { path: '/vaktprivacy/deletion-reminders', label: 'nav.privacy.deletionReminders', icon: PackageX, pro: true },
      { path: '/vaktprivacy/privacy-design',    label: 'nav.privacy.privacyDesign',  icon: ClipboardCheck, pro: true },
    ],
  },
  {
    path: '/vakthr',
    label: 'nav.hr.root',
    icon: UserCog,
    children: [
      { path: '/vakthr/employees',   label: 'nav.hr.employees',   icon: Users },
      { path: '/vakthr/checklists',  label: 'nav.hr.checklists',  icon: ClipboardList },
      { path: '/vakthr/contractors', label: 'nav.hr.contractors', icon: Building2 },
    ],
  },
]

const SIDEBAR_COLLAPSED_KEY = 'vakt_sidebar_collapsed'

export default function Layout() {
  const { t } = useTranslation()
  const location = useLocation()
  const { user } = useAuthStore()
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
  // Accordion state for the Vakt Comply childGroups. Exactly zero or one
  // group is rendered open at any time. The group that contains the active
  // path is auto-opened; clicking a group header swaps to that group (or
  // closes it if it's already open). The lazy initializer prevents the
  // one-frame flash where the active group would otherwise render closed.
  const [expandedGroup, setExpandedGroup] = useState<string | null>(() => {
    if (typeof window === 'undefined') return null
    const path = window.location.pathname
    for (const mod of MODULES_NAV) {
      if (!mod.childGroups) continue
      for (const g of mod.childGroups) {
        if (g.items.some((c) => path === c.path || path.startsWith(c.path + '/'))) {
          return g.label
        }
      }
    }
    return null
  })
  const { data: updateInfo } = useUpdateCheck()
  const isAdminOrOwner = user?.roles.some((r) => r.toLowerCase() === 'admin' || r.toLowerCase() === 'owner') ?? false
  const demoMode = useDemoMode()

  // Collapsed-sidebar flyout: which module path is currently hovered
  const [flyoutPath, setFlyoutPath] = useState<string | null>(null)
  const [flyoutTop, setFlyoutTop] = useState(0)
  const flyoutTimerRef = useRef<ReturnType<typeof setTimeout>>()
  const { data: overdueControls } = useOverdueControls()
  const overdueCount = overdueControls?.length ?? 0
  const { data: autoEvidence } = useAutoEvidence()
  const autoEvidenceCount = autoEvidence?.length ?? 0
  const { data: pendingApprovalData } = usePendingApprovalCount()
  const pendingApprovalCount = pendingApprovalData?.count ?? 0

  useKeyboardShortcuts({ onOpenHelp: () => { setShortcutsOpen(true); } })

  function toggleSidebarCollapsed() {
    setSidebarCollapsed((prev) => {
      const next = !prev
      localStorage.setItem(SIDEBAR_COLLAPSED_KEY, String(next))
      return next
    })
  }

  function openSearch() {
    window.dispatchEvent(new CustomEvent('vakt:open-search'))
  }

  function openFlyout(path: string, e: React.MouseEvent) {
    clearTimeout(flyoutTimerRef.current)
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect()
    setFlyoutTop(rect.top)
    setFlyoutPath(path)
  }

  function closeFlyout() {
    flyoutTimerRef.current = setTimeout(() => { setFlyoutPath(null) }, 120)
  }

  useEffect(() => () => { clearTimeout(flyoutTimerRef.current) }, [])

  useEffect(() => {
    if (demoMode === true) document.title = 'Vakt Demo'
  }, [demoMode])

  function isActive(path: string, exact?: boolean) {
    if (exact) return location.pathname === path
    return location.pathname === path || location.pathname.startsWith(path + '/')
  }

  /**
   * Returns the label of the childGroup that contains a child whose path
   * matches the current location, scanning all MODULES_NAV entries. Returns
   * null when on a hub root (e.g. /vaktcomply) or on a module without groups.
   */
  function findAutoActiveGroup(): string | null {
    for (const mod of MODULES_NAV) {
      if (!mod.childGroups) continue
      for (const group of mod.childGroups) {
        if (group.items.some((c) => location.pathname === c.path || location.pathname.startsWith(c.path + '/'))) {
          return group.label
        }
      }
    }
    return null
  }

  // Resync the open group whenever navigation happens. If the user is now
  // on a page inside a group, open that group. Otherwise (hub root, other
  // module) leave the manual choice in place but null is the safe default.
  useEffect(() => {
    const auto = findAutoActiveGroup()
    if (auto) setExpandedGroup(auto)
    // We intentionally don't reset to null on hub roots — that way the last
    // expanded group stays open while the user is on /vaktcomply itself.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [location.pathname])

  function renderChildLink(c: NavChild) {
    const childActive = location.pathname === c.path || location.pathname.startsWith(c.path + '/')
    const isOverduePath = c.path === '/vaktcomply/overdue-reviews'
    const isAutoEvidencePath = c.path === '/vaktcomply/evidence/auto'
    const isApprovalsPath = c.path === '/vaktcomply/approvals'
    const CIcon = c.icon
    return (
      <Link
        key={c.path}
        to={c.path}
        onClick={() => { setSidebarOpen(false); }}
        aria-current={childActive ? 'page' : undefined}
        className={cn(
          'flex items-center gap-2 px-2 py-[6px] rounded-md text-[12px] font-medium transition-all duration-150',
          childActive
            ? 'text-brand bg-brand/10 dark:bg-muted/50'
            : 'text-secondary hover:text-primary hover:bg-muted/50',
        )}
      >
        <CIcon className="w-3.5 h-3.5 shrink-0" aria-hidden="true" />
        <span className="flex-1 truncate">{t(c.label)}</span>
        {isOverduePath && overdueCount > 0 && (
          <span
            className="ml-auto text-[10px] font-semibold bg-destructive text-destructive-foreground rounded-full px-1.5 py-0.5 leading-none"
            aria-label={`${String(overdueCount)} überfällige Kontrollen`}
          >
            {overdueCount}
          </span>
        )}
        {isAutoEvidencePath && autoEvidenceCount > 0 && (
          <span
            className="ml-auto text-[10px] font-semibold bg-brand text-white rounded-full px-1.5 py-0.5 leading-none"
            aria-label={`${String(autoEvidenceCount)} neue Nachweise`}
          >
            {autoEvidenceCount}
          </span>
        )}
        {isApprovalsPath && pendingApprovalCount > 0 && (
          <span
            className="ml-auto text-[10px] font-semibold bg-amber-500 text-white rounded-full px-1.5 py-0.5 leading-none"
            aria-label={`${String(pendingApprovalCount)} ausstehende Genehmigungen`}
          >
            {pendingApprovalCount}
          </span>
        )}
        {c.pro && !isOverduePath && !isAutoEvidencePath && !isApprovalsPath && <ProBadge />}
      </Link>
    )
  }

  const systemNav: { to: string; icon: React.ElementType; label: string; exact?: boolean }[] = [
    { to: '/settings',     icon: Settings,    label: t('nav.settings'),      exact: true },
    { to: '/integrations', icon: Plug,        label: t('nav.integrations') },
    ...(isAdminOrOwner ? [{ to: '/admin', icon: ShieldAlert, label: t('nav.administration') }] : []),
  ]

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
            <strong>{t('demo.banner')}</strong> — {t('demo.description')}
          </span>
          <button onClick={() => { setDemoBannerDismissed(true); }} aria-label={t('common.close')} className="text-brand/60 hover:text-brand ml-4">✕</button>
        </div>
      )}
      {backupStatus?.stale && !backupDismissed && !demoMode && (
        <div className="bg-amber-50 border-b border-amber-200 px-4 py-2 flex items-center justify-between text-sm shrink-0">
          <span className="text-amber-800">
            ⚠ {t('backup.staleWarning')} —{' '}
            <Link to="/settings?tab=system" className="underline font-medium hover:text-amber-900">
              {t('backup.staleAction')}
            </Link>
          </span>
          <button onClick={() => { setBackupDismissed(true); }} aria-label={t('common.close')} className="text-amber-600 hover:text-amber-800 ml-4">✕</button>
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
            onClick={() => { setUpdateDismissed(true); }}
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
        <div
          className="fixed inset-0 z-20 bg-black/40 lg:hidden"
          onClick={() => { setSidebarOpen(false); }}
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
                onClick={() => { setSidebarOpen(false); }}
                aria-label={t('nav.closeMenu')}
              >
                <X className="w-4 h-4" aria-hidden="true" />
              </button>
            )}
          </div>
          {!sidebarCollapsed && (
            <div className="flex items-center gap-2 px-2">
              <p className="text-[11px] text-secondary">Security Platform</p>
              <BetaBadge />
            </div>
          )}
          {sidebarCollapsed && <BetaBadge collapsed />}
        </div>

        {/* Search trigger — only when sidebar is collapsed (TopBar covers expanded case) */}
        {sidebarCollapsed && (
          <div className="px-2 pb-2 hidden lg:block">
            <button
              type="button"
              aria-label={t('layout.searchOpenDesktop')}
              title={t('layout.searchTitle')}
              onClickCapture={openSearch}
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
            {MODULES_NAV.map(({ path, label, icon: Icon, exact, children, childGroups }) => {
              const active = isActive(path, exact)
              const hasChildren = (children?.length ?? 0) > 0 || (childGroups?.length ?? 0) > 0
              const expanded = active && hasChildren
              return (
                <div key={path}>
                  <Link
                    to={path}
                    onClick={() => { setSidebarOpen(false); setFlyoutPath(null) }}
                    aria-current={active ? 'page' : undefined}
                    title={sidebarCollapsed ? t(label) : undefined}
                    onMouseEnter={sidebarCollapsed && hasChildren ? (e) => { openFlyout(path, e) } : undefined}
                    onMouseLeave={sidebarCollapsed && hasChildren ? closeFlyout : undefined}
                    className={cn(
                      'flex items-center rounded-md text-[13px] font-medium transition-all duration-150',
                      sidebarCollapsed ? 'justify-center p-2' : 'gap-2.5 px-3 py-[9px]',
                      active
                        ? 'bg-brand/10 dark:bg-muted/50 text-brand dark:text-primary'
                        : 'text-secondary hover:bg-muted/50 hover:text-primary',
                    )}
                  >
                    <Icon className={cn('w-4 h-4 shrink-0', active ? 'text-brand' : '')} aria-hidden="true" />
                    {!sidebarCollapsed && t(label)}
                  </Link>
                  {expanded && !sidebarCollapsed && (
                    <div className="ml-3 mt-0.5 mb-1 pl-3 border-l border-border space-y-[1px]">
                      {children?.map(renderChildLink)}
                      {childGroups?.map((group) => {
                        const open = expandedGroup === group.label
                        return (
                          <div key={group.label} className="pt-1 first:pt-0">
                            <button
                              type="button"
                              onClick={() => { setExpandedGroup(open ? null : group.label); }}
                              aria-expanded={open}
                              className="w-full flex items-center gap-1 px-2 py-1 rounded-md text-[9px] font-semibold text-secondary uppercase tracking-wider opacity-70 hover:opacity-100 hover:bg-muted/30 transition-colors"
                            >
                              {open
                                ? <ChevronDown className="w-2.5 h-2.5 shrink-0" aria-hidden="true" />
                                : <ChevronRight className="w-2.5 h-2.5 shrink-0" aria-hidden="true" />}
                              <span>{t(group.label)}</span>
                              {!open && (
                                <span className="ml-auto opacity-50 normal-case tracking-normal font-medium text-[9px]">
                                  {group.items.length}
                                </span>
                              )}
                            </button>
                            {open && (
                              <div className="mt-0.5">
                                {group.items.map(renderChildLink)}
                              </div>
                            )}
                          </div>
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
          <div className="space-y-[2px]">
            {systemNav.map(({ to, icon: Icon, label, exact }) => {
              const active = exact ? location.pathname === to : isActive(to)
              return (
                <Link
                  key={to}
                  to={to}
                  onClick={() => { setSidebarOpen(false); }}
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

        {/* Bottom — user row + collapse + © */}
        <div className={cn('pb-4 border-t border-border pt-3 space-y-[2px]', sidebarCollapsed ? 'px-2' : 'px-3')}>
          {/* User identity link */}
          {user && (
            <Link
              to="/account"
              onClick={() => { setSidebarOpen(false) }}
              title={sidebarCollapsed ? (user.display_name || user.email) : undefined}
              aria-label={t('nav.userProfile')}
              className={cn(
                'flex items-center rounded-md transition-all duration-150 text-secondary hover:bg-muted/50 hover:text-primary',
                sidebarCollapsed ? 'justify-center p-2' : 'gap-2.5 px-3 py-2',
              )}
            >
              <span
                aria-hidden="true"
                className="w-6 h-6 rounded-full bg-brand/15 text-brand flex items-center justify-center text-[11px] font-semibold shrink-0"
              >
                {(user.display_name || user.email || '?').charAt(0).toUpperCase()}
              </span>
              {!sidebarCollapsed && (
                <span className="flex-1 min-w-0">
                  <span className="block text-[12px] font-medium truncate text-primary">
                    {user.display_name || user.email}
                  </span>
                  {user.display_name && user.email && (
                    <span className="block text-[10px] text-secondary truncate">{user.email}</span>
                  )}
                </span>
              )}
            </Link>
          )}
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
              : <><ChevronLeft className="w-4 h-4 shrink-0" aria-hidden="true" /><span>{t('nav.collapse')}</span></>
            }
          </button>
          {!sidebarCollapsed && (
            <div className="px-3 pt-2">
              <p className="text-[10px] text-secondary/50">© 2026 NorvikOps · ELv2</p>
            </div>
          )}
        </div>
      </aside>

      {/* Collapsed-sidebar flyout panel */}
      {sidebarCollapsed && flyoutPath && (() => {
        const item = MODULES_NAV.find((m) => m.path === flyoutPath)
        if (!item) return null
        const allItems: NavChild[] = [
          ...(item.children ?? []),
          ...(item.childGroups?.flatMap((g) => g.items) ?? []),
        ]
        if (allItems.length === 0) return null
        return (
          <div
            role="navigation"
            aria-label={`${t(item.label)} Unternavigation`}
            className="fixed left-[56px] z-40 bg-surface border border-border rounded-r-lg shadow-xl py-2 min-w-[200px]"
            style={{ top: flyoutTop }}
            onMouseEnter={() => { clearTimeout(flyoutTimerRef.current) }}
            onMouseLeave={closeFlyout}
          >
            <p className="px-3 pb-1.5 text-[10px] font-semibold text-secondary uppercase tracking-wider opacity-60">
              {t(item.label)}
            </p>
            {/* Group headers for childGroups modules */}
            {item.childGroups ? item.childGroups.map((group) => (
              <div key={group.label}>
                <p className="px-3 pt-2 pb-0.5 text-[9px] font-semibold text-secondary uppercase tracking-wider opacity-50">
                  {t(group.label)}
                </p>
                {group.items.map((c) => {
                  const CIcon = c.icon
                  const childActive = location.pathname === c.path || location.pathname.startsWith(c.path + '/')
                  return (
                    <Link
                      key={c.path}
                      to={c.path}
                      onClick={() => { setSidebarOpen(false); setFlyoutPath(null) }}
                      aria-current={childActive ? 'page' : undefined}
                      className={cn(
                        'flex items-center gap-2 mx-1 px-2 py-[6px] rounded-md text-[12px] font-medium transition-all duration-150',
                        childActive
                          ? 'text-brand bg-brand/10 dark:bg-muted/50'
                          : 'text-secondary hover:text-primary hover:bg-muted/50',
                      )}
                    >
                      <CIcon className="w-3.5 h-3.5 shrink-0" aria-hidden="true" />
                      <span className="flex-1 truncate">{t(c.label)}</span>
                      {c.pro && <ProBadge />}
                    </Link>
                  )
                })}
              </div>
            )) : allItems.map((c) => {
              const CIcon = c.icon
              const childActive = location.pathname === c.path || location.pathname.startsWith(c.path + '/')
              return (
                <Link
                  key={c.path}
                  to={c.path}
                  onClick={() => { setSidebarOpen(false); setFlyoutPath(null) }}
                  aria-current={childActive ? 'page' : undefined}
                  className={cn(
                    'flex items-center gap-2 mx-1 px-2 py-[6px] rounded-md text-[12px] font-medium transition-all duration-150',
                    childActive
                      ? 'text-brand bg-brand/10 dark:bg-muted/50'
                      : 'text-secondary hover:text-primary hover:bg-muted/50',
                  )}
                >
                  <CIcon className="w-3.5 h-3.5 shrink-0" aria-hidden="true" />
                  <span className="flex-1 truncate">{t(c.label)}</span>
                  {c.pro && <ProBadge />}
                </Link>
              )
            })}
          </div>
        )
      })()}

      {/* Main */}
      <main id="main-content" role="main" className="flex-1 overflow-auto bg-bg flex flex-col min-w-0 pb-16 md:pb-0">
        {/* Mobile top bar with hamburger + search */}
        <div className="lg:hidden flex items-center gap-3 px-4 py-3 border-b border-border bg-surface shrink-0">
          <button
            onClick={() => { setSidebarOpen(true); }}
            aria-label={t('nav.openMenu')}
            className="text-secondary hover:text-primary p-1 rounded"
          >
            <Menu className="w-5 h-5" aria-hidden="true" />
          </button>
          <div className="flex items-center gap-2 flex-1">
            <img src="/logo.svg" alt="Vakt" className="w-5 h-5 shrink-0" />
            <span className="font-bold text-[15px] text-brand leading-none">Vakt</span>
          </div>
          {/* Search trigger — mobile */}
          <button
            type="button"
            onClick={openSearch}
            aria-label={t('layout.searchOpenMobile')}
            className="p-1.5 rounded-lg text-secondary hover:text-primary hover:bg-surface2"
          >
            <Search className="w-4 h-4" aria-hidden="true" />
          </button>
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

        {/* Desktop TopBar */}
        <TopBar
          onOpenSearch={openSearch}
          onOpenShortcuts={() => { setShortcutsOpen(true); }}
        />

        <div className="flex-1 overflow-auto">
          <Suspense
            fallback={
              <div className="flex items-center justify-center h-64">
                <Spinner size="lg" />
              </div>
            }
          >
            <PageTransition>
              <Outlet />
            </PageTransition>
          </Suspense>
        </div>
      </main>
      </div>
      {/* Mobile bottom navigation — 4 core modules + More-Drawer */}
      <nav
        aria-label={t('layout.mobileNav')}
        className="md:hidden fixed bottom-0 left-0 right-0 bg-surface border-t border-border z-30 flex"
      >
        {[
          { label: 'Home',    path: '/',           icon: LayoutDashboard, exact: true },
          { label: 'Comply',  path: '/vaktcomply',  icon: ShieldCheck },
          { label: 'Scan',    path: '/vaktscan',   icon: Bug },
          { label: 'Privacy', path: '/vaktprivacy', icon: Eye },
        ].map(({ label, path, icon: Icon, exact }) => {
          const active = isActive(path, exact)
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
        <button
          type="button"
          onClick={() => { setSidebarOpen(true); }}
          aria-label={t('layout.moreModules')}
          className="flex-1 flex flex-col items-center py-2 text-xs transition-colors text-secondary hover:text-brand"
        >
          <Menu className="h-5 w-5 mb-1" aria-hidden="true" />
          {t('layout.more')}
        </button>
      </nav>
      <GlobalSearch />
      <KeyboardShortcutsModal open={shortcutsOpen} onClose={() => { setShortcutsOpen(false); }} />
      {demoMode && <FeedbackWidget />}
      <WhatsNewModal />
      <Toaster />
      <PWAInstallPrompt />
      <AppTour />
    </div>
  )
}
