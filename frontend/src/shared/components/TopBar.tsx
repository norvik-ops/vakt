import { useEffect, useRef, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import {
  Search, Sun, Moon, Monitor, HelpCircle, BookOpen, ExternalLink,
  User, MonitorSmartphone, LogOut, ChevronDown,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '../stores/auth'
import { useThemeStore } from '../stores/theme'
import { cn } from '../../lib/utils'
import { NotificationBell } from './NotificationBell'
import { ChangelogPopover } from './ChangelogPopover'

interface TopBarProps {
  onOpenSearch: () => void
  onOpenShortcuts: () => void
}

/**
 * Desktop top bar with global utilities — search, notifications, changelog,
 * help, theme, and the user menu. Hidden on mobile (the existing mobile top
 * bar in Layout.tsx handles those breakpoints).
 *
 * Lives inside <main>, so it aligns with content edges, not with the sidebar.
 */
export function TopBar({ onOpenSearch, onOpenShortcuts }: TopBarProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { user, clearAuth } = useAuthStore()
  const { theme, toggle: toggleTheme } = useThemeStore()
  const [userMenuOpen, setUserMenuOpen] = useState(false)
  const userMenuRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!userMenuOpen) return
    function handler(e: MouseEvent) {
      if (userMenuRef.current && !userMenuRef.current.contains(e.target as Node)) {
        setUserMenuOpen(false)
      }
    }
    function escHandler(e: KeyboardEvent) {
      if (e.key === 'Escape') setUserMenuOpen(false)
    }
    document.addEventListener('mousedown', handler)
    document.addEventListener('keydown', escHandler)
    return () => {
      document.removeEventListener('mousedown', handler)
      document.removeEventListener('keydown', escHandler)
    }
  }, [userMenuOpen])

  function logout() {
    clearAuth()
    navigate('/login')
  }

  const ThemeIcon = theme === 'light' ? Moon : theme === 'dark' ? Monitor : Sun
  const themeLabel =
    theme === 'light' ? t('theme.dark') :
    theme === 'dark' ? 'System' :
    t('theme.light')

  const initial = (user?.display_name || user?.email || '?').charAt(0).toUpperCase()

  return (
    <div
      role="toolbar"
      aria-label="Hauptwerkzeugleiste"
      className="hidden lg:flex h-12 items-center gap-1 px-4 border-b border-border bg-surface shrink-0"
    >
      {/* Search trigger — primary action, takes the left space */}
      <button
        type="button"
        onClick={onOpenSearch}
        aria-label="Globale Suche öffnen (Cmd+K)"
        className="flex items-center gap-2 text-xs text-secondary border border-border rounded-md px-3 py-1.5 hover:border-brand/40 hover:text-primary transition-colors min-w-[260px]"
      >
        <Search className="w-3.5 h-3.5" aria-hidden="true" />
        <span>{t('nav.search')}</span>
        <kbd className="ml-auto opacity-60 text-[10px]" aria-hidden="true">⌘K</kbd>
      </button>

      <div className="flex-1" />

      {/* Right cluster */}
      <NotificationBell />
      <ChangelogPopover />

      <button
        type="button"
        onClick={onOpenShortcuts}
        aria-label="Tastaturkürzel anzeigen"
        title="Tastaturkürzel (?)"
        className="p-2 rounded-md text-secondary hover:bg-muted/50 hover:text-primary transition-colors"
      >
        <HelpCircle className="w-4 h-4" aria-hidden="true" />
      </button>

      <button
        type="button"
        onClick={toggleTheme}
        aria-label={`Theme: ${themeLabel}`}
        title={themeLabel}
        className="p-2 rounded-md text-secondary hover:bg-muted/50 hover:text-primary transition-colors"
      >
        <ThemeIcon className="w-4 h-4" aria-hidden="true" />
      </button>

      {/* User menu */}
      <div className="relative ml-1" ref={userMenuRef}>
        <button
          type="button"
          onClick={() => { setUserMenuOpen((v) => !v); }}
          aria-haspopup="menu"
          aria-expanded={userMenuOpen}
          aria-label="Benutzermenü"
          className={cn(
            'flex items-center gap-1.5 pl-1 pr-2 py-1 rounded-md text-sm transition-colors',
            'hover:bg-muted/50',
            userMenuOpen && 'bg-muted/50',
          )}
        >
          <span
            aria-hidden="true"
            className="w-7 h-7 rounded-full bg-brand/15 text-brand flex items-center justify-center text-xs font-semibold"
          >
            {initial}
          </span>
          <ChevronDown className="w-3.5 h-3.5 text-secondary" aria-hidden="true" />
        </button>

        {userMenuOpen && (
          <div
            role="menu"
            className="absolute right-0 top-full mt-1 w-64 bg-surface border border-border rounded-lg shadow-lg z-40 py-1"
          >
            {(user?.email || user?.display_name) && (
              <div className="px-3 py-2 border-b border-border">
                {user.display_name && (
                  <p className="text-sm font-medium text-primary truncate">{user.display_name}</p>
                )}
                {user.email && (
                  <p className="text-[11px] text-secondary truncate">{user.email}</p>
                )}
              </div>
            )}
            <Link
              to="/account"
              onClick={() => { setUserMenuOpen(false); }}
              role="menuitem"
              className="flex items-center gap-2.5 px-3 py-2 text-sm text-secondary hover:bg-muted/50 hover:text-primary transition-colors"
            >
              <User className="w-4 h-4" aria-hidden="true" />
              {t('nav.account')}
            </Link>
            <Link
              to="/account/sessions"
              onClick={() => { setUserMenuOpen(false); }}
              role="menuitem"
              className="flex items-center gap-2.5 px-3 py-2 text-sm text-secondary hover:bg-muted/50 hover:text-primary transition-colors"
            >
              <MonitorSmartphone className="w-4 h-4" aria-hidden="true" />
              {t('nav.sessions')}
            </Link>
            <a
              href="https://github.com/norvik-ops/vatk/wiki"
              target="_blank"
              rel="noopener noreferrer"
              onClick={() => { setUserMenuOpen(false); }}
              role="menuitem"
              className="flex items-center gap-2.5 px-3 py-2 text-sm text-secondary hover:bg-muted/50 hover:text-primary transition-colors"
            >
              <BookOpen className="w-4 h-4" aria-hidden="true" />
              <span className="flex-1">{t('nav.documentation')}</span>
              <ExternalLink className="w-3 h-3 opacity-40" aria-hidden="true" />
            </a>
            <div className="border-t border-border my-1" />
            <button
              type="button"
              onClick={logout}
              role="menuitem"
              className="w-full flex items-center gap-2.5 px-3 py-2 text-sm text-secondary hover:bg-muted/50 hover:text-red-500 transition-colors"
            >
              <LogOut className="w-4 h-4" aria-hidden="true" />
              {t('auth.logout')}
            </button>
          </div>
        )}
      </div>
    </div>
  )
}
