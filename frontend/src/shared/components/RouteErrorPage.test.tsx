import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { createMemoryRouter, RouterProvider } from 'react-router-dom'
import { RouteErrorPage } from './RouteErrorPage'
import { reloadOnceAfterChunkError, resetChunkReloadStateForTests } from '../chunkReload'

function Thrower({ error }: { error: Error }): never {
  throw error
}

function renderWithError(error: Error) {
  const router = createMemoryRouter(
    [{ errorElement: <RouteErrorPage />, children: [{ path: '/', element: <Thrower error={error} /> }] }],
    { initialEntries: ['/'] },
  )
  return render(<RouterProvider router={router} />)
}

const chunkError = new TypeError('Importing a module script failed.')

describe('RouteErrorPage', () => {
  beforeEach(() => {
    resetChunkReloadStateForTests()
    window.sessionStorage.clear()
    vi.spyOn(console, 'error').mockImplementation(() => {})
  })

  it('zeigt bei einem Chunk-Fehler „Neue Version wird geladen“, solange der Reload läuft', () => {
    // Reload wurde schon (z. B. vom vite:preloadError-Listener) ausgelöst.
    reloadOnceAfterChunkError({ now: () => Date.now(), getStorage: () => window.sessionStorage, reload: () => {} })
    renderWithError(chunkError)
    expect(screen.getByRole('status')).toHaveTextContent('Neue Version wird geladen')
    expect(screen.queryByText(/Unexpected Application Error/)).not.toBeInTheDocument()
  })

  it('zeigt nach einem kurz zuvor erfolgten Reload den deutschen Hinweis mit „Neu laden“', async () => {
    window.sessionStorage.setItem('vakt_chunk_reload_at', String(Date.now()))
    renderWithError(chunkError)
    expect(await screen.findByRole('alert')).toHaveTextContent('Seite konnte nicht geladen werden')
    expect(screen.getByRole('button', { name: 'Neu laden' })).toBeInTheDocument()
  })

  it('zeigt andere Fehler weiter mit ihrer Meldung an', () => {
    renderWithError(new Error('kaputt'))
    expect(screen.getByText('Etwas ist schiefgelaufen')).toBeInTheDocument()
    expect(screen.getByText('kaputt')).toBeInTheDocument()
  })
})
