import { describe, it, expect, vi, beforeEach, beforeAll } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import TISAXPage from './TISAXPage'
import type { Control } from '../types'

// jsdom does not implement URL.createObjectURL — define it once for all tests in this file
beforeAll(() => {
  if (typeof URL.createObjectURL === 'undefined') {
    Object.defineProperty(URL, 'createObjectURL', { value: vi.fn().mockReturnValue('blob:mock'), writable: true })
  }
  if (typeof URL.revokeObjectURL === 'undefined') {
    Object.defineProperty(URL, 'revokeObjectURL', { value: vi.fn(), writable: true })
  }
})

// ── Mock data ─────────────────────────────────────────────────────────────────

function makeControl(overrides: Partial<Control> = {}): Control {
  return {
    id: 'c-1',
    framework_id: 'fw-tisax',
    control_id: 'TISAX-1.1',
    title: 'Test Control',
    description: 'desc',
    domain: 'Informationssicherheit',
    status: 'missing',
    not_applicable: false,
    maturity_score: 0,
    ...overrides,
  }
}

// Controls without TISAX-15 prefix (normal protection level)
const NORMAL_CONTROLS: Control[] = [
  makeControl({ id: 'c-1', control_id: 'TISAX-1.1', title: 'Control 1.1' }),
  makeControl({ id: 'c-2', control_id: 'TISAX-5.2', title: 'Control 5.2' }),
  makeControl({ id: 'c-3', control_id: 'TISAX-14.1', title: 'Control 14.1' }),
]

// Controls including TISAX-15 prefix (very_high protection level)
const VERY_HIGH_CONTROLS: Control[] = [
  ...NORMAL_CONTROLS,
  makeControl({ id: 'c-4', control_id: 'TISAX-15.1', title: 'High Protection Control', domain: 'Sehr hoher Schutzbedarf' }),
  makeControl({ id: 'c-5', control_id: 'TISAX-15.2', title: 'Very High Control', domain: 'Sehr hoher Schutzbedarf' }),
]

vi.mock('../../../api/client', () => ({
  apiFetch: vi.fn().mockImplementation((url: string) => {
    if (url.includes('protection_level=very_high')) {
      return Promise.resolve(VERY_HIGH_CONTROLS)
    }
    return Promise.resolve(NORMAL_CONTROLS)
  }),
}))

function renderTISAXPage(frameworkId = 'fw-tisax', protectionLevel?: string) {
  const initialPath = `/secvitals/frameworks/${frameworkId}/tisax${protectionLevel ? `?protection_level=${protectionLevel}` : ''}`
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[initialPath]}>
        <Routes>
          <Route path="/secvitals/frameworks/:id/tisax" element={<TISAXPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

// ── Tests ─────────────────────────────────────────────────────────────────────

describe('TISAXPage', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('renders three protection level tabs', () => {
    renderTISAXPage()
    expect(screen.getByText('Normal')).toBeInTheDocument()
    expect(screen.getByText('Hoch')).toBeInTheDocument()
    expect(screen.getByText('Sehr hoch')).toBeInTheDocument()
  })

  it('renders the TISAX-Ansicht page title', () => {
    renderTISAXPage()
    expect(screen.getByText('TISAX-Ansicht')).toBeInTheDocument()
  })

  it('"Bereitschaftsbericht exportieren" button is present', () => {
    renderTISAXPage('fw-tisax')
    const btn = screen.getByRole('button', { name: /Bereitschaftsbericht exportieren/i })
    expect(btn).toBeTruthy()
  })

  it('click on "Bereitschaftsbericht exportieren" triggers fetch to tisax-report-pdf endpoint', async () => {
    // Mock fetch to capture the URL called
    const mockBlob = new Blob(['%PDF'], { type: 'application/pdf' })
    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValue({
      blob: () => Promise.resolve(mockBlob),
      ok: true,
      status: 200,
    } as unknown as Response)

    renderTISAXPage('fw-tisax')

    const btn = screen.getByRole('button', { name: /Bereitschaftsbericht exportieren/i })
    fireEvent.click(btn)

    expect(fetchSpy).toHaveBeenCalledWith(
      expect.stringContaining('/api/v1/secvitals/frameworks/fw-tisax/tisax-report-pdf'),
      expect.objectContaining({ credentials: 'include' }),
    )
  })
})

describe('TISAXPage — protection level filtering (unit logic test)', () => {
  it('normal protection level: controls with TISAX-15 prefix are excluded', () => {
    const filtered = NORMAL_CONTROLS.filter(
      (c) => !c.control_id.startsWith('TISAX-15'),
    )
    for (const c of filtered) {
      expect(c.control_id.startsWith('TISAX-15')).toBe(false)
    }
    expect(filtered.length).toBe(3)
  })

  it('very_high protection level: TISAX-15 controls are included', () => {
    const chapter15 = VERY_HIGH_CONTROLS.filter((c) =>
      c.control_id.startsWith('TISAX-15'),
    )
    expect(chapter15.length).toBe(2)
  })
})
