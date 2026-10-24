import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import AssetsPage from './AssetsPage'
import { mockApiServer, type ApiServerHandle } from '../../../test-utils/apiServer'

/**
 * R23-02 — the asset CSV import once had one dialog nobody could open and one
 * that pointed at a Pro route a Community licence could not use.
 *
 * Since 2026-10-23 the extended import (POST /vaktcomply/assets/import/csv) is
 * Community: its gate vaktscan_advanced left with Vakt Scan. These tests assert
 * the URL the upload actually goes to — the same one on every licence, including
 * an old Pro key that still lists vaktscan_advanced.
 */

let server: ApiServerHandle

function armServer(isPro: boolean) {
  server = mockApiServer({
    'GET /license': {
      body: { tier: isPro ? 'pro' : 'community', is_pro: isPro, features: isPro ? ['vaktscan_advanced'] : [] },
    },
    'GET /vaktcomply/assets': {
      body: { data: [], pagination: { page: 1, limit: 25, total: 0, total_pages: 1 } },
    },
    'POST /vaktcomply/assets/import': { body: { inserted: 2, errored: 0, errors: [] } },
    'POST /vaktcomply/assets/import/csv': { body: { imported: 2, skipped: 0, errors: [] } },
  })
}

function renderPage() {
  return render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <MemoryRouter>
        <AssetsPage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

/** Drive the dialog from the button to a completed upload. */
async function uploadThroughDialog() {
  fireEvent.click(await screen.findByRole('button', { name: /CSV|Import/i }))

  const file = new File(['name,type\nsrv-1,server\nsrv-2,server\n'], 'assets.csv', { type: 'text/csv' })
  const input = document.querySelector('input[type="file"]')
  expect(input).not.toBeNull()
  fireEvent.change(input as HTMLInputElement, { target: { files: [file] } })

  const upload = await screen.findByRole('button', { name: /Import|hochladen|starten/i })
  fireEvent.click(upload)
}

beforeEach(() => { vi.clearAllMocks() })
afterEach(() => { server.restore(); vi.unstubAllGlobals() })

describe('AssetsPage — CSV import reachability (R23-02)', () => {
  it.each([
    ['a Community licence', false],
    ['a Pro licence', true],
  ])('uploads to the Community CSV route on %s', async (_label, isPro) => {
    armServer(isPro)
    renderPage()
    await uploadThroughDialog()

    await waitFor(() => {
      const posts = server.requests.filter((r) => r.method === 'POST')
      expect(posts).toHaveLength(1)
      expect(posts[0].url).toContain('/vaktcomply/assets/import/csv')
    })
  })
})
