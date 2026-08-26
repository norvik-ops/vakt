import { useEffect, useRef, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import {
  Search, Sun, Moon, Monitor, HelpCircle, BookOpen, ExternalLink,
  User, MonitorSmartphone, LogOut, ChevronDown,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useQueryClient } from '@tanstack/react-query'
import { apiFetch, ApiError, RateLimitedError } from '../../api/client'
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
 * Did the sign-out request leave the server session provably ended?
 *
 * Only two outcomes prove it: a 2xx (the token is now on the deny list and the
 * cookie was cleared by the response) or a 4xx (the server looked at the token
 * and did not accept it — an expired or already revoked session has nothing
 * left to revoke). Everything else — no answer at all, a rate limit, a 5xx —
 * means the token may still be valid, and crucially the HttpOnly cookie is
 * still sitting in the browser, because only the server's Set-Cookie can clear
 * it and that response never arrived.
 *
 * The distinction matters because the two are told apart nowhere else: warning
 * on a 4xx would cry wolf on the most common benign case (a session that had
 * already expired), while staying silent on a network failure is the case that
 * actually leaves someone signed in.
 */
function revocationUnconfirmed(err: unknown): boolean {
  if (err instanceof RateLimitedError) return true
  if (err instanceof ApiError) return err.status >= 500
  // A raw fetch rejection that survived apiFetch's retry budget: no request
  // ever reached the server.
  return true
}

/**
 * Ask the server to end this session, and say so out loud if it could not be
 * reached.
 *
 * window.alert is a blunt instrument and is chosen deliberately. /login is
 * rendered outside <Layout> (router.tsx), and <Toaster /> is mounted inside it
 * (Layout.tsx), so a toast raised here dies the moment the auth guard swaps the
 * shell out — and this message is only worth raising in the seconds *after*
 * that swap, once the request has finally failed. An alert survives the unmount
 * because it is not React, and it cannot be scrolled past, which is the right
 * severity for "you may still be signed in on this computer".
 *
 * The nicer form — a warning banner on the login page — needs Login.tsx, which
 * belongs to another track in this round; reported rather than built.
 */
async function endServerSession(): Promise<void> {
  try {
    await apiFetch('/auth/logout', { method: 'POST' })
  } catch (err) {
    if (!revocationUnconfirmed(err)) return
    window.alert(
      'Abmeldung nicht bestätigt: Der Server war nicht erreichbar. ' +
        'Diese Sitzung kann auf dem Server noch gültig sein. ' +
        'Bitte schließen Sie den Browser vollständig oder melden Sie sich erneut an ' +
        'und wiederholen Sie die Abmeldung.',
    )
  }
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
  const queryClient = useQueryClient()
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

  // R1-21-A01: this used to be clearAuth() + navigate() and nothing else — no
  // request ever left the browser. POST /auth/logout had zero callers in the
  // whole frontend (`git log -S auth/logout` finds no commit that added one),
  // so it was never wired rather than broken later.
  //
  // Client-only teardown cannot end this session: the access_token cookie is
  // HttpOnly, so JS cannot delete it, and clearAuth() only drops the in-memory
  // user. One reload re-ran /auth/me against the surviving cookie, got 200, and
  // put the user straight back in — on a shared machine the next person at the
  // keyboard is still logged in as them. Only the server can revoke the token
  // and expire the cookie, and only this call asks it to.
  //
  // The local teardown deliberately does NOT wait for the response. An expired
  // or already-revoked session answers 4xx, and on a dead network apiFetch
  // spends its whole retry budget (three attempts with backoff) before it gives
  // up — awaiting that would leave the user staring at a page that still looks
  // signed in for seconds after they asked to leave. Worse on a shared machine
  // than the failed revocation itself.
  //
  // What it must NOT do is swallow the outcome, which is what the first pass at
  // this fix did: a failed revocation left the user locally signed out, the
  // token still valid, the HttpOnly cookie still in the browser, and nothing on
  // screen to say so — one reload put them straight back in. endServerSession
  // tears down at the same speed and reports the failure whenever it lands.
  //
  // Not awaiting does not lose the request: a client-side route change does not
  // cancel an in-flight fetch, only a full page unload would.
  //
  // queryClient.clear() drops the cached authenticated responses — without it
  // the next colleague to sign in on the same tab sees the previous user's
  // data until each query refetches.
  function logout() {
    void endServerSession()
    clearAuth()
    queryClient.clear()
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
              href="https://github.com/norvik-ops/vakt/wiki"
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
