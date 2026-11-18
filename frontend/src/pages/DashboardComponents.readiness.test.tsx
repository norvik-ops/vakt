import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { ComplianceProgressCard, FrameworkProgress } from './DashboardComponents'

// UX-15: „NIS2 0 / 88 · 5%" — 9 Kontrollen in Bearbeitung zählen im Prozentwert
// halb, in der Zählung gar nicht. Die Zeile nennt sie jetzt, gleich formuliert
// wie Framework-Karte, Framework-Detail und Comply-Übersicht.
describe('Dashboard — Umsetzungsgrad mit „in Bearbeitung" (UX-15)', () => {
  it('Framework-Fortschritt nennt umgesetzt, in Bearbeitung und den Prozentwert', () => {
    render(
      <FrameworkProgress
        scores={[{ framework_id: 'n', framework_name: 'NIS2', total_controls: 88, implemented_controls: 0, partial_controls: 9, score_pct: (4.5 / 88) * 100 }]}
      />,
    )
    expect(screen.getByText('0 / 88 umgesetzt · 9 in Bearbeitung · 5%')).toBeInTheDocument()
    expect(screen.getByTestId('readiness-breakdown')).toHaveAttribute('title', expect.stringMatching(/zur Hälfte/))
  })

  it('rundet x,5 auf wie das Backend (12,5 % → 13 %), nicht zur geraden Zahl', () => {
    render(
      <FrameworkProgress
        scores={[{ framework_id: 'i', framework_name: 'ISO27001', total_controls: 4, implemented_controls: 0, partial_controls: 1, score_pct: 12.5 }]}
      />,
    )
    expect(screen.getByText('0 / 4 umgesetzt · 1 in Bearbeitung · 13%')).toBeInTheDocument()
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '13')
  })

  it('Compliance-Fortschritt: dieselbe Zeile, „offen" zählt in Bearbeitung nicht mit', () => {
    render(
      <MemoryRouter>
        <ComplianceProgressCard
          scores={[{ framework_id: 'n', framework_name: 'NIS2', total_controls: 214, implemented_controls: 1, partial_controls: 10, score_pct: 2.8 }]}
          overall={{ covered: 1, partial: 10, applicable: 214, score_pct: (6 / 214) * 100 }}
        />
      </MemoryRouter>,
    )
    expect(screen.getByText('1 / 214 umgesetzt · 10 in Bearbeitung · 3%')).toBeInTheDocument()
    expect(screen.getByText('203 offen')).toBeInTheDocument()
  })
})
