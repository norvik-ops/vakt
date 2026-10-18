import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { renderHook } from '@testing-library/react'

import { useRecentPages } from './useRecentPages'
import { useEnabledModules } from '../moduleVisibility'

const STORAGE_KEY = 'vakt_recent_pages'

describe('useRecentPages', () => {
  beforeEach(() => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify([
      { path: '/vaktscan/findings/1', label: 'Finding', icon: 'x', visitedAt: 3 },
      { path: '/vaktcomply/risks/2', label: 'Risiko', icon: 'x', visitedAt: 2 },
      { path: '/vaktscan/assets/3', label: 'Asset', icon: 'x', visitedAt: 1 },
    ]))
  })

  afterEach(() => {
    localStorage.removeItem(STORAGE_KEY)
    useEnabledModules.setState({ modules: null })
  })

  it('blendet Seiten aus abgeschalteten Modulen aus, behält sie aber im Speicher', () => {
    useEnabledModules.setState({ modules: new Set(['vaktcomply']) })
    const { result } = renderHook(() => useRecentPages())
    expect(result.current.map((p) => p.path)).toEqual(['/vaktcomply/risks/2', '/vaktscan/assets/3'])
    expect(JSON.parse(localStorage.getItem(STORAGE_KEY) ?? '[]')).toHaveLength(3)
  })

  it('zeigt alle Seiten, wenn das Modul aktiviert ist', () => {
    useEnabledModules.setState({ modules: new Set(['vaktcomply', 'vaktscan']) })
    const { result } = renderHook(() => useRecentPages())
    expect(result.current).toHaveLength(3)
  })
})
