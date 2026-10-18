import { useState, useCallback, useEffect } from 'react'
import { isPathVisible } from '../moduleVisibility'

const STORAGE_KEY = 'vakt_recent_pages'
const MAX_ENTRIES = 5

export interface RecentPage {
  path: string
  label: string
  icon: string
  visitedAt: number
}

function loadPages(): RecentPage[] {
  try {
    return JSON.parse(localStorage.getItem(STORAGE_KEY) ?? '[]') as RecentPage[]
  } catch {
    return []
  }
}

function savePages(pages: RecentPage[]): void {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(pages))
  } catch {
    // localStorage may be unavailable
  }
}

export function trackPage(path: string, label: string, icon: string): void {
  const prev = loadPages().filter((p) => p.path !== path)
  const next: RecentPage[] = [{ path, label, icon, visitedAt: Date.now() }, ...prev].slice(0, MAX_ENTRIES)
  savePages(next)
  // Notify any mounted hooks via storage event (cross-tab + same-tab via custom event)
  window.dispatchEvent(new CustomEvent('vakt:recent-pages-updated'))
}

export function useRecentPages(): RecentPage[] {
  const [pages, setPages] = useState<RecentPage[]>(() => loadPages())

  // Keep up-to-date when trackPage is called from the same tab
  const refresh = useCallback(() => { setPages(loadPages()); }, [])

  // Listen for updates
  useEffect(() => {
    window.addEventListener('vakt:recent-pages-updated', refresh)
    return () => { window.removeEventListener('vakt:recent-pages-updated', refresh); }
  }, [refresh])

  // Einträge aus einem inzwischen abgeschalteten Modul bleiben gespeichert (kommt das Modul zurück,
  // erscheinen sie wieder), werden aber nicht angezeigt.
  return pages.filter((p) => isPathVisible(p.path))
}
