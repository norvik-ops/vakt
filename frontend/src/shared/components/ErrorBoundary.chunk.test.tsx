import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { ErrorBoundary } from './ErrorBoundary'
import { resetChunkReloadStateForTests } from '../chunkReload'

function Thrower(): never {
  throw new TypeError('Failed to fetch dynamically imported module: /assets/Risk-abc.js')
}

// Modul-Seiten (ModuleShell) liegen hinter dieser Boundary, nicht hinter dem
// errorElement des Routers — sie muss Chunk-Fehler genauso behandeln.
describe('ErrorBoundary bei Chunk-Ladefehler', () => {
  beforeEach(() => {
    resetChunkReloadStateForTests()
    vi.spyOn(console, 'error').mockImplementation(() => {})
  })

  it('zeigt den Chunk-Hinweis statt der Rohmeldung und meldet nichts ans Backend', async () => {
    window.sessionStorage.setItem('vakt_chunk_reload_at', String(Date.now()))
    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(null))
    render(
      <MemoryRouter>
        <ErrorBoundary><Thrower /></ErrorBoundary>
      </MemoryRouter>,
    )
    expect(await screen.findByRole('alert')).toHaveTextContent('Seite konnte nicht geladen werden')
    expect(screen.queryByText(/Failed to fetch dynamically imported module/)).not.toBeInTheDocument()
    expect(fetchSpy).not.toHaveBeenCalled()
  })
})
