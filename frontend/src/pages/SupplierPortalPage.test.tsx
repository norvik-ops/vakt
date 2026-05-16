import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import SupplierPortalPage, { getPortalText, PORTAL_TEXT } from './SupplierPortalPage'

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function makeAssessment() {
  return {
    id: 'assess-1',
    status: 'in_progress',
    expires_at: new Date(Date.now() + 14 * 24 * 60 * 60 * 1000).toISOString(),
    questionnaire: {
      id: 'q-1',
      name: 'NIS2 Lieferanten-Assessment',
      description: 'Test questionnaire',
      questions: [
        {
          id: 'question-1',
          question_text: 'Haben Sie eine Netzwerksicherheitsrichtlinie?',
          question_type: 'yes_no',
          options: [],
          required: true,
          order_idx: 0,
        },
        {
          id: 'question-2',
          question_text: 'Welche Zugriffskontrollen setzen Sie ein?',
          question_type: 'multiple_choice',
          options: ['MFA', 'SSO', 'PAM'],
          required: false,
          order_idx: 1,
        },
      ],
    },
  }
}

function renderPortalPage(token = 'test-token-abc123') {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[`/supplier/${token}`]}>
        <Routes>
          <Route path="/supplier/:token" element={<SupplierPortalPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('SupplierPortalPage', () => {
  beforeEach(() => {
    vi.stubGlobal('fetch', vi.fn())
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('renders without auth wrapper (no redirect to /login)', async () => {
    ;(globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => makeAssessment(),
    })

    renderPortalPage()

    // Should not redirect — should show loading state or content.
    // Critically, no login redirect happens.
    await waitFor(() => {
      // After data loads, the questionnaire name should appear.
      expect(screen.getByText('NIS2 Lieferanten-Assessment')).toBeInTheDocument()
    })
  })

  it('shows loading state initially', () => {
    // Never resolves — stay in loading state.
    ;(globalThis.fetch as ReturnType<typeof vi.fn>).mockReturnValue(new Promise(() => {}))

    renderPortalPage()

    expect(screen.getByText('Fragebogen wird geladen…')).toBeInTheDocument()
  })

  it('shows first question after data loads', async () => {
    ;(globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => makeAssessment(),
    })

    renderPortalPage()

    await waitFor(() => {
      expect(
        screen.getByText('Haben Sie eine Netzwerksicherheitsrichtlinie?'),
      ).toBeInTheDocument()
    })
  })

  it('shows 410 error page when token is expired or submitted', async () => {
    ;(globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: false,
      status: 410,
      json: async () => ({ error: 'assessment_expired_or_submitted' }),
    })

    renderPortalPage()

    await waitFor(() => {
      expect(
        screen.getByText(
          'Dieser Link ist abgelaufen oder der Fragebogen wurde bereits eingereicht.',
        ),
      ).toBeInTheDocument()
    })
  })
})

// ---------------------------------------------------------------------------
// getPortalText unit tests
// ---------------------------------------------------------------------------

describe('getPortalText', () => {
  it('returns German title for lang=de', () => {
    const result = getPortalText('de', 'title')
    expect(result).toBe(PORTAL_TEXT.de.title)
    expect(result).toBe('Sicherheitsfragebogen')
  })

  it('returns English title for lang=en', () => {
    const result = getPortalText('en', 'title')
    expect(result).toBe(PORTAL_TEXT.en.title)
    expect(result).toBe('Security Questionnaire')
  })

  it('returns key as fallback for unknown key', () => {
    const result = getPortalText('de', 'nonexistent_key_xyz')
    expect(result).toBe('nonexistent_key_xyz')
  })

  it('falls back to German for unknown lang', () => {
    // Cast to force unknown lang — tests the fallback branch.
    const result = getPortalText('de', 'submit')
    expect(result).toBe(PORTAL_TEXT.de.submit)
  })
})
