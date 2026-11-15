import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { act, render, screen } from '@testing-library/react'
import { createMemoryRouter, MemoryRouter, RouterProvider } from 'react-router-dom'
import { installChunkReloadListener, resetChunkReloadStateForTests } from './chunkReload'
import { RouteErrorPage } from './components/RouteErrorPage'
import { ErrorBoundary } from './components/ErrorBoundary'

// Hauptweg mit den echten Abhängigkeiten: leerer sessionStorage,
// window.location.reload gemockt. Der erste Chunk-Fehler muss genau einen
// Reload auslösen — egal, ob ihn der vite:preloadError-Listener, das
// errorElement des Routers oder die ErrorBoundary zuerst sieht.

const chunkError = () => new TypeError('Importing a module script failed.')

function Thrower(): never {
  throw chunkError()
}

function preloadErrorEvent(payload: unknown): VitePreloadErrorEvent {
  const event = new Event('vite:preloadError', { cancelable: true }) as VitePreloadErrorEvent
  event.payload = payload as Error
  return event
}

let reload: ReturnType<typeof vi.fn>

beforeEach(() => {
  resetChunkReloadStateForTests()
  window.sessionStorage.clear()
  reload = vi.fn()
  // jsdom erlaubt kein spyOn auf location.reload (nicht konfigurierbar).
  vi.stubGlobal('location', { href: window.location.href, origin: window.location.origin, reload })
  vi.spyOn(console, 'error').mockImplementation(() => {})
})

afterEach(() => {
  vi.unstubAllGlobals()
  vi.useRealTimers()
})

describe('vite:preloadError-Listener', () => {
  it('lädt beim ersten Chunk-Fehler genau einmal neu', () => {
    const uninstall = installChunkReloadListener()
    try {
      window.dispatchEvent(preloadErrorEvent(chunkError()))
      window.dispatchEvent(preloadErrorEvent(chunkError()))
      expect(reload).toHaveBeenCalledTimes(1)
      expect(window.sessionStorage.getItem('vakt_chunk_reload_at')).not.toBeNull()
    } finally {
      uninstall()
    }
  })

  it('lädt bei anderen Fehlern nicht neu', () => {
    const uninstall = installChunkReloadListener()
    try {
      window.dispatchEvent(preloadErrorEvent(new SyntaxError('Unexpected token')))
      expect(reload).not.toHaveBeenCalled()
    } finally {
      uninstall()
    }
  })
})

describe('Error-Boundaries beim ersten Chunk-Fehler', () => {
  it('RouteErrorPage löst genau einen Reload aus', () => {
    const router = createMemoryRouter(
      [{ errorElement: <RouteErrorPage />, children: [{ path: '/', element: <Thrower /> }] }],
      { initialEntries: ['/'] },
    )
    render(<RouterProvider router={router} />)
    expect(screen.getByRole('status')).toHaveTextContent('Neue Version wird geladen')
    expect(reload).toHaveBeenCalledTimes(1)
  })

  it('ErrorBoundary löst genau einen Reload aus', () => {
    render(
      <MemoryRouter>
        <ErrorBoundary><Thrower /></ErrorBoundary>
      </MemoryRouter>,
    )
    expect(screen.getByRole('status')).toHaveTextContent('Neue Version wird geladen')
    expect(reload).toHaveBeenCalledTimes(1)
  })

  it('zeigt nach 3 s auch im Lade-Zustand den „Neu laden“-Knopf, falls der Reload ausbleibt', () => {
    vi.useFakeTimers()
    render(
      <MemoryRouter>
        <ErrorBoundary><Thrower /></ErrorBoundary>
      </MemoryRouter>,
    )
    expect(screen.queryByRole('button', { name: 'Neu laden' })).not.toBeInTheDocument()
    act(() => { vi.advanceTimersByTime(3_000) })
    const button = screen.getByRole('button', { name: 'Neu laden' })
    act(() => { button.click() })
    expect(reload).toHaveBeenCalledTimes(2) // einmal automatisch, einmal per Knopf
  })
})
