import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import QuestionnairePage, { buildReorderPayload } from './QuestionnairePage'
import type { Questionnaire } from '../types'

// ─── Fixtures ─────────────────────────────────────────────────────────────────

const mockQuestionnaire: Questionnaire = {
  id: 'qnr-1',
  org_id: 'org-1',
  name: 'NIS2 Lieferanten-Assessment',
  description: 'Test template',
  is_template: true,
  questions: [
    {
      id: 'q-1',
      questionnaire_id: 'qnr-1',
      order_idx: 0,
      question_text: 'Netzwerksicherheit',
      question_type: 'yes_no',
      required: true,
      created_at: '2025-01-01T00:00:00Z',
      updated_at: '2025-01-01T00:00:00Z',
    },
    {
      id: 'q-2',
      questionnaire_id: 'qnr-1',
      order_idx: 1,
      question_text: 'Zugriffskontrollen',
      question_type: 'yes_no',
      required: true,
      created_at: '2025-01-01T00:00:00Z',
      updated_at: '2025-01-01T00:00:00Z',
    },
    {
      id: 'q-3',
      questionnaire_id: 'qnr-1',
      order_idx: 2,
      question_text: 'Backup-Strategie wählen',
      question_type: 'multiple_choice',
      options: ['Täglich', 'Wöchentlich', 'Monatlich'],
      required: false,
      created_at: '2025-01-01T00:00:00Z',
      updated_at: '2025-01-01T00:00:00Z',
    },
  ],
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-01-01T00:00:00Z',
}

// ─── Mocks ────────────────────────────────────────────────────────────────────

vi.mock('../hooks/useQuestionnaires', () => ({
  useQuestionnaire: vi.fn(),
  useReorderQuestions: vi.fn(() => ({ mutate: vi.fn(), isPending: false })),
  useAddQuestion: vi.fn(() => ({ mutate: vi.fn(), isPending: false })),
}))

import { useQuestionnaire, useReorderQuestions } from '../hooks/useQuestionnaires'

// ─── Helper ───────────────────────────────────────────────────────────────────

function renderPage(id = 'qnr-1') {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[`/secvitals/questionnaires/${id}`]}>
        <Routes>
          <Route path="/secvitals/questionnaires/:id" element={<QuestionnairePage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

// ─── Tests: page rendering ────────────────────────────────────────────────────

describe('QuestionnairePage', () => {
  it('renders questionnaire name as heading', () => {
    vi.mocked(useQuestionnaire).mockReturnValue({
      data: mockQuestionnaire,
      isLoading: false,
      isError: false,
    } as ReturnType<typeof useQuestionnaire>)

    renderPage()
    expect(screen.getByText('NIS2 Lieferanten-Assessment')).toBeTruthy()
  })

  it('renders all question texts', () => {
    vi.mocked(useQuestionnaire).mockReturnValue({
      data: mockQuestionnaire,
      isLoading: false,
      isError: false,
    } as ReturnType<typeof useQuestionnaire>)

    renderPage()
    expect(screen.getByText('Netzwerksicherheit')).toBeTruthy()
    expect(screen.getByText('Zugriffskontrollen')).toBeTruthy()
    expect(screen.getByText('Backup-Strategie wählen')).toBeTruthy()
  })

  it('renders drag handles for each question', () => {
    vi.mocked(useQuestionnaire).mockReturnValue({
      data: mockQuestionnaire,
      isLoading: false,
      isError: false,
    } as ReturnType<typeof useQuestionnaire>)

    renderPage()
    const handles = screen.getAllByTestId('drag-handle')
    expect(handles.length).toBe(3)
  })

  it('renders question type badges', () => {
    vi.mocked(useQuestionnaire).mockReturnValue({
      data: mockQuestionnaire,
      isLoading: false,
      isError: false,
    } as ReturnType<typeof useQuestionnaire>)

    renderPage()
    // yes_no badge appears twice
    const yesNoBadges = screen.getAllByText('Ja/Nein')
    expect(yesNoBadges.length).toBe(2)
    expect(screen.getByText('Mehrfachauswahl')).toBeTruthy()
  })

  it('renders Frage hinzufügen button', () => {
    vi.mocked(useQuestionnaire).mockReturnValue({
      data: mockQuestionnaire,
      isLoading: false,
      isError: false,
    } as ReturnType<typeof useQuestionnaire>)

    renderPage()
    expect(screen.getByText('Frage hinzufügen')).toBeTruthy()
  })

  it('shows loading state when isLoading is true', () => {
    vi.mocked(useQuestionnaire).mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
    } as ReturnType<typeof useQuestionnaire>)

    renderPage()
    expect(screen.getByText('Lade Fragebogen...')).toBeTruthy()
  })

  it('shows not found state when data is undefined', () => {
    vi.mocked(useQuestionnaire).mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: false,
    } as ReturnType<typeof useQuestionnaire>)

    renderPage()
    expect(screen.getByText('Fragebogen nicht gefunden.')).toBeTruthy()
  })

  it('calls useReorderQuestions hook (hook is wired)', () => {
    vi.mocked(useQuestionnaire).mockReturnValue({
      data: mockQuestionnaire,
      isLoading: false,
      isError: false,
    } as ReturnType<typeof useQuestionnaire>)

    renderPage()
    expect(vi.mocked(useReorderQuestions)).toHaveBeenCalledWith('qnr-1')
  })
})

// ─── Tests: buildReorderPayload ───────────────────────────────────────────────

describe('buildReorderPayload', () => {
  const order = ['q-1', 'q-2', 'q-3', 'q-4', 'q-5']

  it('moves first item to last position', () => {
    // Drag q-1 before nothing (target = q-5, meaning q-1 goes before q-5)
    const result = buildReorderPayload('q-1', 'q-5', order)
    expect(result).toEqual(['q-2', 'q-3', 'q-4', 'q-1', 'q-5'])
  })

  it('moves last item to first position', () => {
    // Drag q-5 before q-1
    const result = buildReorderPayload('q-5', 'q-1', order)
    expect(result).toEqual(['q-5', 'q-1', 'q-2', 'q-3', 'q-4'])
  })

  it('moves middle item forward', () => {
    // Drag q-2 before q-4
    const result = buildReorderPayload('q-2', 'q-4', order)
    expect(result).toEqual(['q-1', 'q-3', 'q-2', 'q-4', 'q-5'])
  })

  it('moves middle item backward', () => {
    // Drag q-4 before q-2
    const result = buildReorderPayload('q-4', 'q-2', order)
    expect(result).toEqual(['q-1', 'q-4', 'q-2', 'q-3', 'q-5'])
  })

  it('returns same order when draggedId equals targetId', () => {
    const result = buildReorderPayload('q-3', 'q-3', order)
    expect(result).toEqual(order)
  })

  it('handles two-element order', () => {
    const twoOrder = ['q-1', 'q-2']
    const result = buildReorderPayload('q-2', 'q-1', twoOrder)
    expect(result).toEqual(['q-2', 'q-1'])
  })

  it('preserves all IDs in output', () => {
    const result = buildReorderPayload('q-3', 'q-1', order)
    expect(result.sort()).toEqual([...order].sort())
  })
})
