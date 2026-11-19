import { describe, it, expect } from 'vitest'
import { pwaOptions } from './pwaOptions'

// Nach einem Deploy muss der nächste Seitenaufruf den neuen Stand zeigen.
// Was dieser Test NICHT prüft: das echte Verhalten im Browser (Service-Worker-
// Lebenszyklus, Safari). Das zeigt nur ein Lauf gegen zwei echte Builds.

const workbox = pwaOptions.workbox ?? {}

function match(path: string, mode: RequestMode): boolean {
  const route = (workbox.runtimeCaching ?? []).find((r) => r.handler === 'NetworkFirst')
  if (!route) throw new Error('keine NetworkFirst-Route')
  const url = new URL(path, 'https://vakt.example')
  if (typeof route.urlPattern !== 'function') throw new Error('urlPattern ist keine Funktion')
  const opts = { url, request: { mode } as Request, sameOrigin: true, event: {} }
  return Boolean(route.urlPattern(opts))
}

describe('Service Worker: neuer Stand nach einem Deploy', () => {
  it('übernimmt sofort und räumt alte Caches ab', () => {
    expect(pwaOptions.registerType).toBe('autoUpdate')
    expect(workbox.skipWaiting).toBe(true)
    expect(workbox.clientsClaim).toBe(true)
    expect(workbox.cleanupOutdatedCaches).toBe(true)
  })

  it('beantwortet Seitenaufrufe nicht aus dem Precache (kein html, kein navigateFallback)', () => {
    // null, nicht undefined: undefined ersetzt vite-plugin-pwa durch 'index.html'.
    expect(workbox.navigateFallback).toBeNull()
    for (const p of workbox.globPatterns ?? []) expect(p).not.toMatch(/html/)
  })

  it('holt Seitenaufrufe zuerst aus dem Netz, /api/ und Nicht-Navigationen nie über diese Route', () => {
    expect(match('/', 'navigate')).toBe(true)
    expect(match('/comply/risks', 'navigate')).toBe(true)
    expect(match('/api/v1/auth/oidc/callback', 'navigate')).toBe(false)
    expect(match('/assets/index-abc.js', 'cors')).toBe(false)
  })
})
