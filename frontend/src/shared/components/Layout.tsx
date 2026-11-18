import { Link, useLocation, Outlet } from 'react-router-dom'
import { useState, useEffect, Suspense } from 'react'
import {
  LayoutDashboard, Sun, Moon, Monitor, ShieldCheck, Search, FlaskConical, Menu, X, ArrowUpCircle, ChevronLeft, ChevronRight,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { isPathVisible } from '../moduleVisibility'
import { useAuthStore } from '../stores/auth'
import { useThemeStore } from '../stores/theme'
import { cn } from '../../lib/utils'
import { FeedbackWidget } from './FeedbackWidget'
import { useBackupStatus } from '../../hooks/useDashboard'
import { useDemoMode } from '../hooks/useDemoMode'
import { GlobalSearch } from './GlobalSearch'
import { LicenseExpiryBanner } from './LicenseExpiryBanner'
import { BackupBanner } from './BackupBanner'
import { WhatsNewModal } from './WhatsNewModal'
import { useOverdueControls } from '../../modules/vaktcomply/hooks/useControlReviews'
import { useAutoEvidence } from '../../modules/vaktcomply/hooks/useEvidenceAuto'
import { usePendingApprovalCount } from '../hooks/useApprovals'
import { useUpdateCheck } from '../hooks/useUpdateCheck'
import { useLicenseStatus } from '../hooks/useFeature'
import { Toaster } from './Toaster'
import { PWAInstallPrompt } from './PWAInstallPrompt'
import { PageTransition } from './PageTransition'
import { useKeyboardShortcuts } from '../hooks/useKeyboardShortcuts'
import { KeyboardShortcutsModal } from './KeyboardShortcutsModal'
import { AppTour } from './AppTour'
import { Spinner } from '../../components/Spinner'
import { TopBar } from './TopBar'
import { SidebarNav } from './SidebarNav'

const SIDEBAR_COLLAPSED_KEY = 'vakt_sidebar_collapsed'
const DESKTOP_QUERY = '(min-width: 1024px)' // Tailwind `lg`, ab da ist die Seitenleiste fest

function useIsDesktop(): boolean {
  const [isDesktop, setIsDesktop] = useState(() =>
    typeof window !== 'undefined' && typeof window.matchMedia === 'function'
      ? window.matchMedia(DESKTOP_QUERY).matches
      : true,
  )
  useEffect(() => {
    if (typeof window.matchMedia !== 'function') return
    const mq = window.matchMedia(DESKTOP_QUERY)
    const onChange = () => { setIsDesktop(mq.matches) }
    onChange()
    mq.addEventListener('change', onChange)
    return () => { mq.removeEventListener('change', onChange) }
  }, [])
  return isDesktop
}

export default function Layout() {
  const { t } = useTranslation()
  const location = useLocation()
  const { user } = useAuthStore()
  const { theme, toggle } = useThemeStore()
  const { data: backupStatus } = useBackupStatus()
  const [demoBannerDismissed, setDemoBannerDismissed] = useState(false)
  const [updateDismissed, setUpdateDismissed] = useState(false)
  const [sidebarOpen, setSidebarOpen] = useState(false)
  const [sidebarCollapsed, setSidebarCollapsed] = useState(() => {
    try { return localStorage.getItem(SIDEBAR_COLLAPSED_KEY) === 'true' } catch { return false }
  })
  const [shortcutsOpen, setShortcutsOpen] = useState(false)
  // UX-39: Auf dem Handy ist die Seitenleiste ein Drawer. Ein auf dem Desktop
  // gespeichertes „eingeklappt" (56 px, nur Icons) gilt dort nicht — sonst öffnet
  // „Mehr" eine schmale Icon-Leiste ohne Beschriftung.
  const isDesktop = useIsDesktop()
  const collapsed = sidebarCollapsed && isDesktop
  const { data: updateInfo } = useUpdateCheck()
  const { data: licenseStatus } = useLicenseStatus()
  const isUserPro = licenseStatus?.is_pro ?? false
  const isAdminOrOwner = user?.roles.some((r) => r.toLowerCase() === 'admin' || r.toLowerCase() === 'owner') ?? false
  const demoMode = useDemoMode()

  const { data: overdueControls } = useOverdueControls()
  const overdueCount = overdueControls?.length ?? 0
  const { data: autoEvidence } = useAutoEvidence()
  const autoEvidenceCount = autoEvidence?.length ?? 0
  const { data: pendingApprovalData } = usePendingApprovalCount()
  const pendingApprovalCount = pendingApprovalData?.count ?? 0

  useKeyboardShortcuts({ onOpenHelp: () => { setShortcutsOpen(true); } })

  // Drawer mit Escape schließen (Handy/Tablet).
  useEffect(() => {
    if (!sidebarOpen) return
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') setSidebarOpen(false) }
    window.addEventListener('keydown', onKey)
    return () => { window.removeEventListener('keydown', onKey) }
  }, [sidebarOpen])

  function toggleSidebarCollapsed() {
    setSidebarCollapsed((prev) => {
      const next = !prev
      try { localStorage.setItem(SIDEBAR_COLLAPSED_KEY, String(next)) } catch { /* Storage optional */ }
      return next
    })
  }

  function openSearch() {
    window.dispatchEvent(new CustomEvent('vakt:open-search'))
  }

  useEffect(() => {
    if (demoMode === true) document.title = 'Vakt Demo'
  }, [demoMode])

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
            <strong>{t('demo.banner')}</strong> — {t('demo.description')}
          </span>
          <button onClick={() => { setDemoBannerDismissed(true); }} aria-label={t('common.close')} className="text-brand/60 hover:text-brand ml-4">✕</button>
        </div>
      )}
      {!demoMode && <BackupBanner status={backupStatus} />}
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
        aria-expanded={!collapsed}
        data-testid="app-sidebar"
        data-open={sidebarOpen ? 'true' : 'false'}
        className={cn(
          'shrink-0 bg-surface border-r border-border flex flex-col',
          'fixed inset-y-0 left-0 z-30 transition-all duration-200 lg:static lg:translate-x-0',
          sidebarOpen ? 'translate-x-0' : '-translate-x-full lg:translate-x-0',
          collapsed ? 'w-[56px]' : 'w-[210px]',
        )}
      >

        {/* Brand */}
        <div className={cn('px-3 pt-5 pb-4', collapsed && 'px-2')}>
          <div className={cn('flex items-center gap-2.5 px-2 mb-1', collapsed && 'justify-center px-0')}>
            <img src="/logo.svg" alt="Vakt" className="w-7 h-7 shrink-0" title="Vakt" />
            {!collapsed && <span className="font-bold text-[18px] text-brand leading-none">Vakt</span>}
            {!collapsed && (
              <button
                className="ml-auto lg:hidden text-secondary hover:text-primary p-1 rounded"
                onClick={() => { setSidebarOpen(false); }}
                aria-label={t('nav.closeMenu')}
              >
                <X className="w-4 h-4" aria-hidden="true" />
              </button>
            )}
          </div>
          {!collapsed && (
            <div className="flex items-center gap-2 px-2">
              <p className="text-[11px] text-secondary">Security Platform</p>
            </div>
          )}
        </div>

        {/* Search trigger — only when sidebar is collapsed (TopBar covers expanded case) */}
        {collapsed && (
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
        <nav role="navigation" aria-label={t('nav.mainNav')} className={cn('flex-1 overflow-y-auto', collapsed ? 'px-2' : 'px-3')}>
          <SidebarNav
            pathname={location.pathname}
            collapsed={collapsed}
            isUserPro={isUserPro}
            isAdminOrOwner={isAdminOrOwner}
            overdueCount={overdueCount}
            autoEvidenceCount={autoEvidenceCount}
            pendingApprovalCount={pendingApprovalCount}
            onNavigate={() => { setSidebarOpen(false) }}
          />
        </nav>

        {/* Bottom — user row + collapse + © */}
        <div className={cn('pb-4 border-t border-border pt-3 space-y-[2px]', collapsed ? 'px-2' : 'px-3')}>
          {/* User identity link */}
          {user && (
            <Link
              to="/account"
              onClick={() => { setSidebarOpen(false) }}
              title={collapsed ? (user.display_name || user.email) : undefined}
              aria-label={t('nav.userProfile')}
              className={cn(
                'flex items-center rounded-md transition-all duration-150 text-secondary hover:bg-muted/50 hover:text-primary',
                collapsed ? 'justify-center p-2' : 'gap-2.5 px-3 py-2',
              )}
            >
              <span
                aria-hidden="true"
                className="w-6 h-6 rounded-full bg-brand/15 text-brand flex items-center justify-center text-[11px] font-semibold shrink-0"
              >
                {(user.display_name || user.email || '?').charAt(0).toUpperCase()}
              </span>
              {!collapsed && (
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
            aria-label={collapsed ? 'Sidebar ausklappen' : 'Sidebar einklappen'}
            title={collapsed ? 'Sidebar ausklappen' : 'Sidebar einklappen'}
            className={cn(
              'hidden lg:flex w-full items-center rounded-md text-[13px] text-secondary hover:bg-muted/50 hover:text-primary transition-all duration-150',
              collapsed ? 'justify-center p-2' : 'gap-2.5 px-3 py-[9px]',
            )}
          >
            {collapsed
              ? <ChevronRight className="w-4 h-4 shrink-0" aria-hidden="true" />
              : <><ChevronLeft className="w-4 h-4 shrink-0" aria-hidden="true" /><span>{t('nav.collapse')}</span></>
            }
          </button>
          {!collapsed && (
            <div className="px-3 pt-2">
              <p className="text-[10px] text-secondary/50">© 2026 NorvikOps · ELv2</p>
            </div>
          )}
        </div>
      </aside>

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
        ].filter(({ path }) => isPathVisible(path)).map(({ label, path, icon: Icon, exact }) => {
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
