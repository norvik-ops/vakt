import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { AssessmentReviewView } from './AssessmentReviewView'
import type { AnswerWithReview } from '../types'

const mockReviewMutate = vi.fn()

const mockAnswers: AnswerWithReview[] = [
  {
    id: 'ans-1',
    question_text: 'Haben Sie eine Datenschutzrichtlinie?',
    answer_text: 'Ja, vorhanden.',
    file_url: '',
    review_status: undefined,
    rework_note: undefined,
    control_id: 'ctrl-1',
  },
  {
    id: 'ans-2',
    question_text: 'Ist ein ISMS implementiert?',
    answer_text: 'Nein.',
    file_url: '',
    review_status: undefined,
  },
]

vi.mock('../hooks/useAssessments', () => ({
  useAssessmentAnswers: () => ({ data: mockAnswers, isLoading: false }),
  useReviewAnswer: () => ({ mutate: mockReviewMutate, isPending: false }),
  useFinalizeAssessment: () => ({ mutate: vi.fn(), isPending: false }),
  statusToVariant: (s: string) => s,
}))

function renderReview(assessmentId = 'test-assess-1') {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[`/vaktcomply/assessments/${assessmentId}/review`]}>
        <Routes>
          <Route path="/vaktcomply/assessments/:id/review" element={<AssessmentReviewView />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('AssessmentReviewView', () => {
  it('renders question texts', () => {
    renderReview()
    expect(screen.getByText('Haben Sie eine Datenschutzrichtlinie?')).toBeTruthy()
    expect(screen.getByText('Ist ein ISMS implementiert?')).toBeTruthy()
  })

  it('Accept button calls mutate with review_status=accepted', () => {
    mockReviewMutate.mockClear()
    renderReview()
    const acceptButtons = screen.getAllByText('Akzeptieren')
    fireEvent.click(acceptButtons[0])
    expect(mockReviewMutate).toHaveBeenCalledWith({
      answerId: 'ans-1',
      input: { review_status: 'accepted' },
    })
  })

  it('Needs-Rework button opens the rework dialog', () => {
    renderReview()
    const reworkButtons = screen.getAllByText('Nacharbeit nötig')
    fireEvent.click(reworkButtons[0])
    expect(screen.getByText('Nacharbeit erforderlich')).toBeTruthy()
    expect(screen.getByPlaceholderText(/Begründung/)).toBeTruthy()
  })
})
