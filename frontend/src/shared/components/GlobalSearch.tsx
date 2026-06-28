import { useState, useEffect, useRef, useCallback, type RefObject } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import {
  Search, Shield, AlertTriangle, FileText, Siren, ClipboardCheck,
  Server, Bug, Users, Clock, Loader2, LayoutDashboard, ShieldAlert,
  BookOpen, Eye,
} from 'lucide-react'
import { useSearch, SearchResult } from '../../hooks/useSearch'
import { useDebounce } from '../../hooks/useDebounce'
import { useFocusTrap } from '../hooks/useFocusTrap'

interface QuickNavItem {
  label: string
  path: string
  icon: React.ReactNode
}

const QUICK_NAV: QuickNavItem[] = [
  { label: 'Dashboard',     path: '/',                       icon: <LayoutDashboard className="w-3.5 h-3.5 text-secondary shrink-0" /> },
  { label: 'Frameworks',     path: '/vaktcomply/frameworks',   icon: <Shield className="w-3.5 h-3.5 text-brand shrink-0" /> },
  { label: 'Risiken',       path: '/vaktcomply/risks',        icon: <ShieldAlert className="w-3.5 h-3.5 text-amber-500 shrink-0" /> },
  { label: 'Vorfälle',      path: '/vaktcomply/incidents',    icon: <Siren className="w-3.5 h-3.5 text-red-500 shrink-0" /> },
  { label: 'Richtlinien',   path: '/vaktcomply/policies',     icon: <BookOpen className="w-3.5 h-3.5 text-blue-500 shrink-0" /> },
  { label: 'Findings',      path: '/vaktscan/findings',      icon: <Bug className="w-3.5 h-3.5 text-orange-500 shrink-0" /> },
  { label: 'Board-Bericht', path: '/vaktcomply/board-report', icon: <Eye className="w-3.5 h-3.5 text-purple-500 shrink-0" /> },
]

const ENTITY_ICONS: Record<string, React.ReactNode> = {
  control:  <Shield        className="w-3.5 h-3.5 text-brand shrink-0" />,
  risk:     <AlertTriangle className="w-3.5 h-3.5 text-amber-500 shrink-0" />,
  policy:   <FileText      className="w-3.5 h-3.5 text-blue-500 shrink-0" />,
  incident: <Siren         className="w-3.5 h-3.5 text-red-500 shrink-0" />,
  capa:     <ClipboardCheck className="w-3.5 h-3.5 text-green-500 shrink-0" />,
  asset:    <Server        className="w-3.5 h-3.5 text-purple-500 shrink-0" />,
  finding:  <Bug           className="w-3.5 h-3.5 text-orange-500 shrink-0" />,
  dsr:      <Users         className="w-3.5 h-3.5 text-cyan-500 shrink-0" />,
  breach:   <AlertTriangle className="w-3.5 h-3.5 text-destructive shrink-0" />,
}

const ENTITY_LABELS: Record<string, string> = {
  control:  'Kontrolle',
  risk:     'Risiko',
  policy:   'Richtlinie',
  incident: 'Vorfall',
  capa:     'Korrekturmaßnahme',
  asset:    'Asset',
  finding:  'Finding',
  dsr:      'DSR',
  breach:   'Datenpanne',
}

const RECENT_KEY = 'vakt_search_recent'
const MAX_RECENT = 5

function loadRecent(): SearchResult[] {
  try {
    return JSON.parse(localStorage.getItem(RECENT_KEY) ?? '[]') as SearchResult[]
  } catch {
    return []
  }
}

function saveRecent(result: SearchResult) {
  const prev = loadRecent().filter(
    r => !(r.id === result.id && r.entity_type === result.entity_type),
  )
  try {
    localStorage.setItem(RECENT_KEY, JSON.stringify([result, ...prev].slice(0, MAX_RECENT)))
  } catch {
    // localStorage may be unavailable
  }
}

export function GlobalSearch() {
  const [open, setOpen]         = useState(false)
  const [query, setQuery]       = useState('')
  const [recent, setRecent]     = useState<SearchResult[]>([])
  const [activeIdx, setActiveIdx] = useState(-1)

  const navigate   = useNavigate()
  const { t } = useTranslation()
  const inputRef   = useRef<HTMLInputElement>(null)
  const listRef    = useRef<HTMLUListElement>(null)

  const debouncedQuery          = useDebounce(query, 300)
  const { data, isFetching }    = useSearch(debouncedQuery)

  const results   = data?.results ?? []
  const showRecent = debouncedQuery.length < 2 && recent.length > 0
  const displayList: SearchResult[] = debouncedQuery.length >= 2 ? results : (showRecent ? recent : [])

  // Reset keyboard selection when list changes.
  useEffect(() => { setActiveIdx(-1) }, [displayList.length, debouncedQuery])

  // Global keyboard shortcut: Cmd/Ctrl+K (also via custom event from Layout/useKeyboardShortcuts)
  useEffect(() => {
    function onKeyDown(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault()
        setOpen(prev => !prev)
      }
      if (e.key === 'Escape') setOpen(false)
    }
    function onOpenSearch() {
      setOpen(true)
    }
    window.addEventListener('keydown', onKeyDown)
    window.addEventListener('vakt:open-search', onOpenSearch)
    return () => {
      window.removeEventListener('keydown', onKeyDown)
      window.removeEventListener('vakt:open-search', onOpenSearch)
    }
  }, [])

  // Focus input & load recent when opened.
  useEffect(() => {
    if (open) {
      setRecent(loadRecent())
      setQuery('')
      setActiveIdx(-1)
      setTimeout(() => inputRef.current?.focus(), 50)
    }
  }, [open])

  const handleSelect = useCallback((result: SearchResult) => {
    saveRecent(result)
    navigate(result.url)
    setOpen(false)
  }, [navigate])

  function handleKeyDown(e: React.KeyboardEvent) {
    if (displayList.length === 0) return
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      setActiveIdx(i => Math.min(i + 1, displayList.length - 1))
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      setActiveIdx(i => Math.max(i - 1, 0))
    } else if (e.key === 'Enter' && activeIdx >= 0) {
      e.preventDefault()
      handleSelect(displayList[activeIdx])
    }
  }

  // Scroll active item into view.
  useEffect(() => {
    if (activeIdx < 0 || !listRef.current) return
    const item = listRef.current.children[activeIdx] as HTMLElement | undefined
    item?.scrollIntoView({ block: 'nearest' })
  }, [activeIdx])

  const trapRef = useFocusTrap<HTMLDivElement>(open, () => { setOpen(false); })

  if (!open) return null

  return (
    <div
      className="fixed inset-0 z-50 flex items-start justify-center pt-24 bg-black/40"
      onClick={() => { setOpen(false); }}
    >
      <div
        ref={trapRef as RefObject<HTMLDivElement>}
        role="dialog"
        aria-label={t('nav.search')}
        aria-modal="true"
        className="w-full max-w-lg bg-background dark:bg-card rounded-xl shadow-2xl border border-border overflow-hidden"
        onClick={e => { e.stopPropagation(); }}
      >
        {/* Input row */}
        <div className="flex items-center gap-2 px-4 py-3 border-b border-border">
          {/* WCAG 1.1.1: search icon is decorative, input has its own aria-label */}
          <Search className="w-4 h-4 text-secondary shrink-0" aria-hidden="true" />
          <input
            ref={inputRef}
            value={query}
            onChange={e => { setQuery(e.target.value); setActiveIdx(-1) }}
            onKeyDown={handleKeyDown}
            placeholder={t('globalSearch.placeholder')}
            aria-label={t('globalSearch.ariaLabel')}
            aria-autocomplete="list"
            aria-controls="global-search-results"
            aria-activedescendant={activeIdx >= 0 ? `search-result-${String(activeIdx)}` : undefined}
            role="combobox"
            aria-expanded={displayList.length > 0}
            className="flex-1 bg-transparent text-sm outline-none text-primary placeholder:text-secondary"
          />
          {isFetching
            ? <Loader2 className="w-3.5 h-3.5 text-secondary animate-spin shrink-0" aria-label={t('states.loading')} />
            : null
          }
          <kbd className="text-xs text-secondary border border-border rounded px-1 shrink-0" aria-label={t('globalSearch.escAria')}>Esc</kbd>
        </div>

        {/* Results / recent / empty states */}
        {showRecent && (
          <div className="px-4 pt-3 pb-1 text-xs text-secondary font-medium flex items-center gap-1">
            <Clock className="w-3 h-3" />
            {t('globalSearch.recentlyViewed')}
          </div>
        )}

        {debouncedQuery.length >= 2 && !isFetching && results.length > 0 && (
          <div className="px-4 pt-2.5 pb-1 text-xs text-secondary font-medium">
            {t('globalSearch.resultsFor', { count: results.length, query: debouncedQuery })}
          </div>
        )}

        {displayList.length > 0 && (
          <ul ref={listRef} id="global-search-results" role="listbox" aria-label={t('globalSearch.resultsAria')} className="max-h-80 overflow-y-auto py-1.5">
            {displayList.map((r, idx) => (
              <li key={r.id + r.entity_type} role="option" aria-selected={idx === activeIdx} id={`search-result-${String(idx)}`}>
                <button
                  onClick={() => { handleSelect(r); }}
                  className={[
                    'w-full flex items-center gap-3 px-4 py-2 text-left transition-colors',
                    idx === activeIdx
                      ? 'bg-brand/10 dark:bg-muted/50'
                      : 'hover:bg-muted/50',
                  ].join(' ')}
                >
                  {ENTITY_ICONS[r.entity_type] ?? <FileText className="w-3.5 h-3.5 text-secondary shrink-0" />}
                  <span className="flex-1 min-w-0">
                    <span className="block text-sm font-medium text-primary truncate">{r.title}</span>
                    {r.subtitle && (
                      <span className="block text-xs text-secondary truncate">{r.subtitle}</span>
                    )}
                  </span>
                  <span className="text-xs text-secondary shrink-0 ml-2">
                    {ENTITY_LABELS[r.entity_type] ?? r.entity_type}
                  </span>
                </button>
              </li>
            ))}
          </ul>
        )}

        {debouncedQuery.length >= 2 && !isFetching && results.length === 0 && (
          <div className="px-4 py-6 text-center text-sm text-secondary">
            {t('globalSearch.noResults', { query: debouncedQuery })}
          </div>
        )}

        {debouncedQuery.length < 2 && !showRecent && (
          <div className="px-4 pt-3 pb-1">
            <p className="text-xs text-secondary font-medium mb-2">{t('globalSearch.quickNav')}</p>
            <div className="grid grid-cols-2 gap-1">
              {QUICK_NAV.map((item) => (
                <button
                  key={item.path}
                  onClick={() => { navigate(item.path); setOpen(false) }}
                  className="flex items-center gap-2 px-3 py-2 rounded-md text-left text-sm text-primary hover:bg-muted/50 transition-colors"
                >
                  {item.icon}
                  <span className="truncate">{item.label}</span>
                </button>
              ))}
            </div>
          </div>
        )}

        {/* Footer */}
        <div className="px-4 py-2 border-t border-border flex justify-between text-xs text-secondary">
          <span>{t('globalSearch.footerLeft')}</span>
          <span>{t('globalSearch.footerRight')}</span>
        </div>
      </div>
    </div>
  )
}
