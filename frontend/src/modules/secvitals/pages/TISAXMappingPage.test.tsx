import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import { createElement } from 'react'
import TISAXMappingPage from './TISAXMappingPage'
import type { MappingResult } from '../types'

vi.mock('../hooks/useTISAXMapping', () => ({
  useTISAXISOMapping: vi.fn(),
  useTISAXGapsAfterISO: vi.fn(),
}))

import { useTISAXISOMapping } from '../hooks/useTISAXMapping'

const mockResults: MappingResult[] = [
  {
    tisax_control_id: 'TISAX-1.1.1',
    tisax_control_title: 'IS-Politik und -Ziele definiert',
    iso_control_id: 'A.5.1.1',
    iso_control_title: 'Policies for information security',
    covered: true,
  },
  {
    tisax_control_id: 'TISAX-2.1.1',
    tisax_control_title: 'Rollen und Verantwortlichkeiten IS',
    iso_control_id: 'A.6.1.1',
    iso_control_title: 'Information security roles',
    covered: false,
  },
  {
    tisax_control_id: 'TISAX-3.1.1',
    tisax_control_title: 'Überprüfung vor der Anstellung',
    iso_control_id: 'A.7.1.1',
    iso_control_title: 'Screening',
    covered: false,
  },
]

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    createElement(
      QueryClientProvider,
      { client: queryClient },
      createElement(MemoryRouter, null, createElement(TISAXMappingPage)),
    ),
  )
}

describe('TISAXMappingPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders covered badge (green) for covered=true entries', async () => {
    vi.mocked(useTISAXISOMapping).mockReturnValue({
      data: mockResults,
      isLoading: false,
      isSuccess: true,
    } as unknown as ReturnType<typeof useTISAXISOMapping>)

    renderPage()

    await waitFor(() => {
      // "Abgedeckt" badge text should appear for covered item
      const badges = screen.getAllByText('Abgedeckt')
      // At least one for the covered result row (plus summary badge)
      expect(badges.length).toBeGreaterThanOrEqual(1)
    })
  })

  it('renders gap badge (red/destructive) for covered=false entries', async () => {
    vi.mocked(useTISAXISOMapping).mockReturnValue({
      data: mockResults,
      isLoading: false,
      isSuccess: true,
    } as unknown as ReturnType<typeof useTISAXISOMapping>)

    renderPage()

    await waitFor(() => {
      const gapBadges = screen.getAllByText('Lücke')
      expect(gapBadges.length).toBeGreaterThanOrEqual(1)
    })
  })

  it('shows all rows by default', async () => {
    vi.mocked(useTISAXISOMapping).mockReturnValue({
      data: mockResults,
      isLoading: false,
      isSuccess: true,
    } as unknown as ReturnType<typeof useTISAXISOMapping>)

    renderPage()

    await waitFor(() => {
      expect(screen.getByText('IS-Politik und -Ziele definiert')).toBeInTheDocument()
      expect(screen.getByText('Rollen und Verantwortlichkeiten IS')).toBeInTheDocument()
      expect(screen.getByText('Überprüfung vor der Anstellung')).toBeInTheDocument()
    })
  })

  it('hides covered rows when "Nur Lücken" toggle is enabled', async () => {
    vi.mocked(useTISAXISOMapping).mockReturnValue({
      data: mockResults,
      isLoading: false,
      isSuccess: true,
    } as unknown as ReturnType<typeof useTISAXISOMapping>)

    renderPage()

    // Wait for initial render
    await waitFor(() => {
      expect(screen.getByText('IS-Politik und -Ziele definiert')).toBeInTheDocument()
    })

    // Click the toggle
    const toggle = screen.getByRole('switch')
    fireEvent.click(toggle)

    await waitFor(() => {
      // Covered item should be hidden
      expect(screen.queryByText('IS-Politik und -Ziele definiert')).not.toBeInTheDocument()
      // Gap items should still be visible
      expect(screen.getByText('Rollen und Verantwortlichkeiten IS')).toBeInTheDocument()
      expect(screen.getByText('Überprüfung vor der Anstellung')).toBeInTheDocument()
    })
  })

  it('shows info banner when no data is returned', async () => {
    vi.mocked(useTISAXISOMapping).mockReturnValue({
      data: [],
      isLoading: false,
      isSuccess: true,
    } as unknown as ReturnType<typeof useTISAXISOMapping>)

    renderPage()

    await waitFor(() => {
      expect(screen.getByText(/ISO 27001 noch nicht aktiviert/)).toBeInTheDocument()
    })
  })

  it('shows loading spinner while fetching', () => {
    vi.mocked(useTISAXISOMapping).mockReturnValue({
      data: undefined,
      isLoading: true,
      isSuccess: false,
    } as unknown as ReturnType<typeof useTISAXISOMapping>)

    renderPage()

    // Should have a spinner (animate-spin element)
    const spinner = document.querySelector('.animate-spin')
    expect(spinner).toBeInTheDocument()
  })
})
