import { Link } from 'react-router-dom'
import { useState, useEffect, useRef } from 'react'
import { createPortal } from 'react-dom'
import { LayoutDashboard, LayoutList, Settings, ShieldAlert, Plug, ChevronDown, ChevronRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { ProBadge } from './ProBadge'
import { isPathVisible } from '../moduleVisibility'
import { cn } from '../../lib/utils'
import { NAV_SECTIONS, SIDEBAR_SECTIONS_KEY, matchesPath, sectionForPath, type NavChild } from './sidebarSections'

// Storage ist eine Bequemlichkeit: privates Fenster, gesperrte Site-Daten oder ein
// werfender Zugriff dürfen die Navigation nie verhindern.
function loadManualSections(): Partial<Record<string, boolean>> {
  try {
    const raw = localStorage.getItem(SIDEBAR_SECTIONS_KEY)
    if (!raw) return {}
    const parsed: unknown = JSON.parse(raw)
    if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) return {}
    const out: Partial<Record<string, boolean>> = {}
    for (const [k, v] of Object.entries(parsed as Record<string, unknown>)) {
      if (typeof v === 'boolean') out[k] = v
    }
    return out
  } catch {
    return {}
  }
}

function saveManualSections(value: Partial<Record<string, boolean>>) {
  try {
    localStorage.setItem(SIDEBAR_SECTIONS_KEY, JSON.stringify(value))
  } catch {
    // bewusst still — siehe loadManualSections
  }
}

export interface SidebarNavProps {
  pathname: string
  collapsed: boolean
  isUserPro: boolean
  isAdminOrOwner: boolean
  overdueCount: number
  autoEvidenceCount: number
  pendingApprovalCount: number
  /** Wird bei jedem Link-Klick gerufen (schließt die Mobil-Leiste). */
  onNavigate: () => void
}

export function SidebarNav({
  pathname, collapsed, isUserPro, isAdminOrOwner,
  overdueCount, autoEvidenceCount, pendingApprovalCount, onNavigate,
}: SidebarNavProps) {
  const { t } = useTranslation()
  const [manual, setManual] = useState<Partial<Record<string, boolean>>>(loadManualSections)
  const activeSection = sectionForPath(pathname)

  // Icon-Modus: Bereichs-Icon öffnet ein Flyout mit den Einträgen (Hover oder Klick/Tastatur).
  const [flyoutKey, setFlyoutKey] = useState<string | null>(null)
  const [flyoutTop, setFlyoutTop] = useState(0)
  const flyoutTimerRef = useRef<ReturnType<typeof setTimeout>>()

  useEffect(() => () => { clearTimeout(flyoutTimerRef.current) }, [])
  useEffect(() => { if (!collapsed) setFlyoutKey(null) }, [collapsed])
  useEffect(() => {
    if (!flyoutKey) return
    function onKey(e: KeyboardEvent) { if (e.key === 'Escape') setFlyoutKey(null) }
    window.addEventListener('keydown', onKey)
    return () => { window.removeEventListener('keydown', onKey) }
  }, [flyoutKey])

  function isOpen(key: string): boolean {
    // Der Bereich der aktiven Route ist immer offen, sonst gilt die gemerkte Wahl.
    return key === activeSection || manual[key] === true
  }

  function toggleSection(key: string) {
    if (key === activeSection) return
    setManual((prev) => {
      const next = { ...prev, [key]: !(prev[key] === true) }
      saveManualSections(next)
      return next
    })
  }

  function openFlyout(key: string, el: HTMLElement) {
    clearTimeout(flyoutTimerRef.current)
    setFlyoutTop(el.getBoundingClientRect().top)
    setFlyoutKey(key)
  }

  function closeFlyoutSoon() {
    flyoutTimerRef.current = setTimeout(() => { setFlyoutKey(null) }, 120)
  }

  function handleNavigate() {
    setFlyoutKey(null)
    onNavigate()
  }

  function renderChildLink(c: NavChild, inFlyout = false) {
    const childActive = matchesPath(pathname, c.path)
    const isOverduePath = c.path === '/vaktcomply/overdue-reviews'
    const isAutoEvidencePath = c.path === '/vaktcomply/evidence'
    const isApprovalsPath = c.path === '/vaktcomply/approvals'
    const CIcon = c.icon
    return (
      <Link
        key={c.path}
        to={c.path}
        onClick={handleNavigate}
        aria-current={childActive ? 'page' : undefined}
        className={cn(
          'flex items-center gap-2 px-2 py-[6px] rounded-md text-[12px] font-medium transition-all duration-150',
          inFlyout && 'mx-1',
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
        {c.pro && !isUserPro && !isOverduePath && !isAutoEvidencePath && !isApprovalsPath && <ProBadge tier={c.tier} />}
      </Link>
    )
  }

  function topLinkClass(active: boolean) {
    return cn(
      'flex items-center rounded-md text-[13px] font-medium transition-all duration-150',
      collapsed ? 'justify-center p-2' : 'gap-2.5 px-3 py-[9px]',
      active
        ? 'bg-brand/10 dark:bg-muted/50 text-brand dark:text-primary'
        : 'text-secondary hover:bg-muted/50 hover:text-primary',
    )
  }

  const sections = NAV_SECTIONS
    .map((s) => ({ ...s, items: s.items.filter((c) => isPathVisible(c.path)) }))
    .filter((s) => s.items.length > 0)

  const systemNav: { to: string; icon: React.ElementType; label: string; exact?: boolean }[] = [
    { to: '/settings',     icon: Settings,    label: t('nav.settings'),      exact: true },
    { to: '/integrations', icon: Plug,        label: t('nav.integrations') },
    ...(isAdminOrOwner ? [{ to: '/admin', icon: ShieldAlert, label: t('nav.administration') }] : []),
  ]

  // Dashboard und Comply-Übersicht stehen über den Bereichen. Die Übersicht nutzt den
  // vorhandenen Katalog-Schlüssel „Übersicht“ (de) / „Overview“ (en).
  const topNav: { to: string; label: string; icon: React.ElementType }[] = [
    { to: '/',           label: 'nav.dashboard',       icon: LayoutDashboard },
    { to: '/vaktcomply', label: 'nav.comply.overview', icon: LayoutList },
  ]
  const flyoutSection = collapsed && flyoutKey ? sections.find((s) => s.key === flyoutKey) : undefined

  return (
    <>
      <div className="space-y-[2px] mb-4">
        {topNav.filter((n) => isPathVisible(n.to)).map(({ to, label, icon: Icon }) => {
          // Beide Einträge sind nur auf genau ihrer Route aktiv — /vaktcomply/risks gehört
          // zum Bereich „Betrieb“, nicht zur Comply-Übersicht.
          const active = pathname === to
          return (
            <Link
              key={to}
              to={to}
              onClick={handleNavigate}
              aria-current={active ? 'page' : undefined}
              title={collapsed ? t(label) : undefined}
              className={topLinkClass(active)}
            >
              <Icon className={cn('w-4 h-4 shrink-0', active ? 'text-brand' : '')} aria-hidden="true" />
              {!collapsed && t(label)}
            </Link>
          )
        })}

        {sections.map((s) => {
          const SIcon = s.icon
          const sectionActive = s.key === activeSection
          if (collapsed) {
            const flyoutId = `nav-flyout-${s.key}`
            const flyoutOpen = flyoutKey === s.key
            return (
              <button
                key={s.key}
                type="button"
                title={t(s.label)}
                aria-label={t(s.label)}
                aria-expanded={flyoutOpen}
                aria-controls={flyoutId}
                onClick={(e) => { if (flyoutOpen) setFlyoutKey(null); else openFlyout(s.key, e.currentTarget) }}
                onMouseEnter={(e) => { openFlyout(s.key, e.currentTarget) }}
                onMouseLeave={closeFlyoutSoon}
                className={cn('w-full', topLinkClass(sectionActive))}
              >
                <SIcon className={cn('w-4 h-4 shrink-0', sectionActive ? 'text-brand' : '')} aria-hidden="true" />
              </button>
            )
          }
          const open = isOpen(s.key)
          const listId = `nav-section-${s.key}`
          return (
            <div key={s.key} className="pt-2">
              <button
                type="button"
                onClick={() => { toggleSection(s.key) }}
                aria-expanded={open}
                aria-controls={listId}
                className={cn(
                  'w-full flex items-center gap-1.5 px-2 py-1 rounded-md text-[10px] font-semibold uppercase tracking-wider transition-colors hover:bg-muted/30',
                  sectionActive ? 'text-brand opacity-100' : 'text-secondary opacity-70 hover:opacity-100',
                )}
              >
                {open
                  ? <ChevronDown className="w-3 h-3 shrink-0" aria-hidden="true" />
                  : <ChevronRight className="w-3 h-3 shrink-0" aria-hidden="true" />}
                <span className="text-left leading-tight">{t(s.label)}</span>
                {!open && (
                  <span aria-hidden="true" className="ml-auto opacity-50 normal-case tracking-normal font-medium">
                    {s.items.length}
                  </span>
                )}
              </button>
              <div id={listId} hidden={!open} className="mt-0.5 ml-2 pl-2 border-l border-border space-y-[1px]">
                {s.items.map((c) => renderChildLink(c))}
              </div>
            </div>
          )
        })}
      </div>

      {!collapsed && (
        <p className="px-2 mb-1 text-[10px] font-semibold text-secondary uppercase tracking-wider opacity-60">
          {t('nav.system')}
        </p>
      )}
      <div className="space-y-[2px]">
        {systemNav.map(({ to, icon: Icon, label, exact }) => {
          const active = exact ? pathname === to : matchesPath(pathname, to)
          return (
            <Link
              key={to}
              to={to}
              onClick={handleNavigate}
              aria-current={active ? 'page' : undefined}
              title={collapsed ? label : undefined}
              className={topLinkClass(active)}
            >
              <Icon className={cn('w-4 h-4 shrink-0', active ? 'text-brand' : '')} aria-hidden="true" />
              {!collapsed && label}
            </Link>
          )
        })}
      </div>

      {/* Flyout im Icon-Modus. Portal, weil die Leiste per transform verschoben wird und
          fixed-Kinder sonst an ihr statt am Viewport hängen und vom Scroll-Container
          abgeschnitten werden. */}
      {flyoutSection && createPortal(
        <div
          id={`nav-flyout-${flyoutSection.key}`}
          role="navigation"
          aria-label={t(flyoutSection.label)}
          className="fixed left-[56px] z-40 bg-surface border border-border rounded-r-lg shadow-xl py-2 min-w-[200px]"
          style={{ top: flyoutTop }}
          onMouseEnter={() => { clearTimeout(flyoutTimerRef.current) }}
          onMouseLeave={closeFlyoutSoon}
        >
          <p className="px-3 pb-1.5 text-[10px] font-semibold text-secondary uppercase tracking-wider opacity-60">
            {t(flyoutSection.label)}
          </p>
          {flyoutSection.items.map((c) => renderChildLink(c, true))}
        </div>,
        document.body,
      )}
    </>
  )
}
