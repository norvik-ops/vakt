import { useState, useEffect, useCallback, useRef } from 'react'

const STORAGE_PREFIX = 'vakt_filters_'
const DEBOUNCE_MS = 300

function readFromStorage<T extends object>(key: string, defaults: T): T {
  try {
    const raw = localStorage.getItem(STORAGE_PREFIX + key)
    if (!raw) return defaults
    const parsed = JSON.parse(raw) as Partial<T>
    // Merge with defaults so newly added filter fields are always present
    return { ...defaults, ...parsed }
  } catch {
    return defaults
  }
}

export function useSavedFilters<T extends object>(
  key: string,
  defaults: T,
): [T, (v: T | ((prev: T) => T)) => void] {
  const [filters, setFiltersState] = useState<T>(() =>
    readFromStorage(key, defaults),
  )

  // Keep a stable ref to the current filters for the debounced write
  const filtersRef = useRef(filters)
  filtersRef.current = filters

  // Debounced persistence
  const debounceTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  const persistFilters = useCallback(
    (next: T) => {
      if (debounceTimer.current !== null) {
        clearTimeout(debounceTimer.current)
      }
      debounceTimer.current = setTimeout(() => {
        try {
          localStorage.setItem(STORAGE_PREFIX + key, JSON.stringify(next))
        } catch {
          // localStorage may be unavailable (private mode, quota exceeded)
        }
      }, DEBOUNCE_MS)
    },
    [key],
  )

  const setFilters = useCallback(
    (v: T | ((prev: T) => T)) => {
      setFiltersState((prev) => {
        const next = typeof v === 'function' ? (v as (p: T) => T)(prev) : v
        persistFilters(next)
        return next
      })
    },
    [persistFilters],
  )

  // Re-read when key changes (e.g. navigating between pages with same hook)
  useEffect(() => {
    setFiltersState(readFromStorage(key, defaults))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key])

  // Cleanup timer on unmount
  useEffect(() => {
    return () => {
      if (debounceTimer.current !== null) {
        clearTimeout(debounceTimer.current)
      }
    }
  }, [])

  return [filters, setFilters]
}
