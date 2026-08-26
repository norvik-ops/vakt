import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import AssetDetailPage from './AssetDetailPage'
import { mockApiServer, type ApiServerHandle } from '../../../test-utils/apiServer'

/**
 * R1-19-W07 — "Scan starten" could never have worked.
 *
 * The mutation posted no body; the handler requires `scanner`
 * (`oneof=trivy nuclei openvas`), so every click was a 422. There was also no
 * scanner selection anywhere in the frontend, so this was not a user error
 * waiting to be made — no sequence of clicks could produce a valid request.
 *
 * The check that matters is what leaves the page, so these tests read the
 * request body off a stubbed fetch rather than asserting on a mocked hook.
 */

let server: ApiServerHandle

const ASSET = {
  id: 'a1',
  org_id: 'o1',
  name: 'My Web App',
  type: 'web_app',
  criticality: 'high',
  tags: [],
  external_url: 'https://app.example.com',
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

function armServer(scanners: Record<string, boolean>) {
  server = mockApiServer({
    'GET /vaktscan/assets/a1': { body: ASSET },
    'GET /vaktscan/scanner-status': { body: { trivy: false, nuclei: false, openvas: false, ...scanners } },
    'GET /vaktscan/findings': { body: { data: [], pagination: { limit: 25, next_cursor: '', has_more: false } } },
    'POST /vaktscan/assets/a1/scans': { status: 202, body: { id: 's1', status: 'queued' } },
  })
}

function renderPage() {
  return render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <MemoryRouter initialEntries={['/vaktscan/assets/a1']}>
        <Routes>
          <Route path="/vaktscan/assets/:id" element={<AssetDetailPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

// Radix Select calls these on open; jsdom implements none of them, and the
// resulting TypeError tears the page down mid-render so every later query
// fails for a reason unrelated to the defect under test.
beforeEach(() => {
  vi.clearAllMocks()
  Element.prototype.scrollIntoView = vi.fn()
  Element.prototype.hasPointerCapture = vi.fn(() => false)
  Element.prototype.releasePointerCapture = vi.fn()
})
afterEach(() => { server.restore(); vi.unstubAllGlobals() })

describe('AssetDetailPage — start scan (R1-19-W07)', () => {
  it('sends the chosen scanner, which the handler requires', async () => {
    armServer({ trivy: true })
    renderPage()

    const start = await screen.findByRole('button', { name: /Scan starten/i })
    // Nothing can be sent before a scanner is picked — that is the whole point.
    expect(start).toBeDisabled()

    fireEvent.click(await screen.findByRole('combobox', { name: /Scanner/i }))
    fireEvent.click(await screen.findByRole('option', { name: 'trivy' }))
    await waitFor(() => { expect(start).not.toBeDisabled() })
    fireEvent.click(start)

    await waitFor(() => {
      const post = server.requests.find((r) => r.method === 'POST')
      expect(post).toBeDefined()
      expect(post?.body).toMatchObject({ scanner: 'trivy' })
    })
  })

  it('passes the asset URL as the target so the worker does not scan the asset NAME', async () => {
    // With no target the worker falls back to payload.AssetName and then
    // rejects anything containing a slash — "My Web App" would be the target.
    armServer({ nuclei: true })
    renderPage()

    fireEvent.click(await screen.findByRole('combobox', { name: /Scanner/i }))
    fireEvent.click(await screen.findByRole('option', { name: 'nuclei' }))
    fireEvent.click(await screen.findByRole('button', { name: /Scan starten/i }))

    await waitFor(() => {
      const post = server.requests.find((r) => r.method === 'POST')
      expect(post?.body).toMatchObject({ target_url: 'https://app.example.com' })
    })
  })

  it('offers only the scanners this instance actually has installed', async () => {
    armServer({ trivy: true, openvas: true })
    renderPage()

    fireEvent.click(await screen.findByRole('combobox', { name: /Scanner/i }))
    expect(await screen.findByRole('option', { name: 'trivy' })).toBeTruthy()
    expect(screen.getByRole('option', { name: 'openvas' })).toBeTruthy()
    expect(screen.queryByRole('option', { name: 'nuclei' })).toBeNull()
  })

  it('says so when no scanner is installed instead of offering a button that 422s', async () => {
    armServer({})
    renderPage()

    expect(await screen.findByText(/kein Scanner installiert/i)).toBeTruthy()
    expect(await screen.findByRole('button', { name: /Scan starten/i })).toBeDisabled()
  })

  it('has a translation for the failure message — the key used to be missing everywhere', async () => {
    armServer({ trivy: true })
    server.route('POST /vaktscan/assets/a1/scans', {
      status: 422, body: { error: 'Ungültige Eingabe', code: 'VALIDATION_ERROR' },
    })
    renderPage()

    fireEvent.click(await screen.findByRole('combobox', { name: /Scanner/i }))
    fireEvent.click(await screen.findByRole('option', { name: 'trivy' }))
    fireEvent.click(await screen.findByRole('button', { name: /Scan starten/i }))

    // vaktscan.assetDetail.scanFailed existed in no locale file, so the page
    // printed the raw key at the user. Rendering the key itself is the failure.
    const msg = await screen.findByText(/Scan konnte nicht gestartet werden/i)
    expect(msg.textContent).not.toContain('vaktscan.assetDetail')
  })
})
