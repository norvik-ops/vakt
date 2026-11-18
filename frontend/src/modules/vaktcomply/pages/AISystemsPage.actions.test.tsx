/**
 * UX-33 (Kernweg 10): Die Inventarkarte zeigte ihre Aktionen nur als Symbole — der
 * zentrale Schritt „Risikoklasse bestimmen" war ein unbeschrifteter Kolben.
 * UX-34: Zwei Wege zur Risikoklasse (freie Auswahl im Dialog und der Assistent).
 * Der Dialog zeigt die Klasse jetzt nur an und verweist auf den Assistenten.
 */
import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, within } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import AISystemsPage from './AISystemsPage'
import type { AISystem } from '../types'

vi.mock('../../../shared/stores/auth', () => ({
  useAuthStore: (selector: (s: { token: string | null }) => unknown) =>
    selector({ token: 'test-token' }),
}))

const OPEN: AISystem = {
  id: 'ai-1', org_id: 'org-1', name: 'Bewerber-Vorauswahl', autonomy_level: 'assistive',
  status: 'under_review', created_at: '2024-01-01T00:00:00Z', updated_at: '2024-01-01T00:00:00Z',
}
const CLASSIFIED: AISystem = {
  ...OPEN, id: 'ai-2', name: 'Chatbot', risk_class: 'limited',
  classification_rationale: 'Art. 50: Interaktion mit Menschen', classified_by: 'KI-Beauftragte',
}

const createMutate = vi.fn()
const updateMutate = vi.fn()

vi.mock('../hooks/useAISystems', () => ({
  useAISystems: () => ({ data: [OPEN, CLASSIFIED], isLoading: false, isError: false }),
  useCreateAISystem: () => ({ mutate: createMutate, isPending: false }),
  useUpdateAISystem: () => ({ mutate: updateMutate, isPending: false }),
  useDeleteAISystem: () => ({ mutate: vi.fn(), isPending: false }),
}))

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <MemoryRouter initialEntries={['/vaktcomply/ai-systems']}>
      <QueryClientProvider client={qc}><AISystemsPage /></QueryClientProvider>
    </MemoryRouter>,
  )
}

describe('AISystemsPage — beschriftete Aktionen (UX-33)', () => {
  it('„Risikoklasse bestimmen" und „Technisches Dossier" sind sichtbar beschriftet', () => {
    renderPage()
    const classify = screen.getAllByTestId('classify-ai-system-btn')[0]
    expect(classify).toHaveTextContent('Risikoklasse bestimmen')
    expect(classify).toHaveAccessibleName('Risikoklasse bestimmen')
    const dossier = screen.getAllByRole('link', { name: 'Technisches Dossier' })[0]
    expect(dossier).toHaveTextContent('Technisches Dossier')
    // Relative Auflösung unter der Route ai-systems prüft AISystemsPage.link.test.tsx.
    expect(dossier.getAttribute('href')).toMatch(/ai-1\/documentation$/)
  })

  it('Bearbeiten und Löschen bleiben Symbole, tragen aber Tooltip und aria-label', () => {
    renderPage()
    const edit = screen.getByRole('button', { name: 'Bearbeiten: Bewerber-Vorauswahl' })
    expect(edit).toHaveAttribute('title', 'Bearbeiten')
    const del = screen.getByRole('button', { name: 'Löschen: Bewerber-Vorauswahl' })
    expect(del).toHaveAttribute('title', 'Löschen')
  })

  it('jede Aktionsschaltfläche der Karten hat einen zugänglichen Namen', () => {
    renderPage()
    for (const b of screen.getAllByRole('button')) {
      expect(b, b.outerHTML.slice(0, 120)).not.toHaveAccessibleName('')
    }
  })

  it('ohne Klasse: Karte sagt „noch nicht bestimmt"', () => {
    renderPage()
    expect(screen.getAllByTestId('risk-class-open')).toHaveLength(1)
    expect(screen.getByTestId('risk-class-open')).toHaveTextContent('Risikoklasse noch nicht bestimmt')
  })
})

describe('AISystemsPage — ein Weg zur Risikoklasse (UX-34)', () => {
  it('„KI-System erfassen": keine freie Auswahl der Risikoklasse, Verweis auf den Assistenten', () => {
    renderPage()
    fireEvent.click(screen.getAllByRole('button', { name: 'KI-System erfassen' })[0])
    const dlg = screen.getByRole('dialog')
    expect(within(dlg).queryByRole('combobox', { name: /Risikoklasse/ })).toBeNull()
    expect(within(dlg).queryByText('Minimales Risiko')).toBeNull()
    const info = within(dlg).getByTestId('ai-risk-class-info')
    expect(info).toHaveTextContent('Risikoklasse noch nicht bestimmt')
    expect(info).toHaveTextContent(/Assistent/)
    expect(within(dlg).queryByPlaceholderText('Klassifizierungsbegründung')).toBeNull()
  })

  it('Erfassen sendet keine Risikoklasse', () => {
    createMutate.mockClear()
    renderPage()
    fireEvent.click(screen.getAllByRole('button', { name: 'KI-System erfassen' })[0])
    const dlg = screen.getByRole('dialog')
    fireEvent.change(within(dlg).getByPlaceholderText('z.B. ChatGPT-Integration'), { target: { value: 'Neu' } })
    fireEvent.click(within(dlg).getByRole('button', { name: 'Erfassen' }))
    expect(createMutate).toHaveBeenCalledTimes(1)
    expect((createMutate.mock.calls[0][0] as { risk_class?: string }).risk_class).toBeUndefined()
  })

  it('Bearbeiten zeigt die gespeicherte Klasse nur an und gibt sie unverändert weiter', () => {
    updateMutate.mockClear()
    renderPage()
    fireEvent.click(screen.getByRole('button', { name: 'Bearbeiten: Chatbot' }))
    const dlg = screen.getByRole('dialog')
    const info = within(dlg).getByTestId('ai-risk-class-info')
    expect(info).toHaveTextContent('Begrenztes Risiko')
    expect(info).toHaveTextContent('Art. 50: Interaktion mit Menschen')
    expect(within(info).getByRole('button', { name: 'Risikoklasse bestimmen' })).toBeInTheDocument()
    fireEvent.click(within(dlg).getByRole('button', { name: 'Speichern' }))
    expect(updateMutate.mock.calls[0][0]).toMatchObject({
      risk_class: 'limited',
      classification_rationale: 'Art. 50: Interaktion mit Menschen',
      classified_by: 'KI-Beauftragte',
    })
  })
})
