import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { ComplianceProgressCard } from './DashboardComponents'

// R1-KW-6 (Subreview PR #182): die Karte „Compliance-Fortschritt" summierte
// implemented/total der Frameworks selbst — ohne Teilgewicht. Sie zeigt jetzt
// die im Backend nach ADR-0086 gepoolte Gesamtzahl.
//
// 4 Kontrollen: umgesetzt, in Arbeit, nicht anwendbar, offen
// → Backend: covered 1, partial 1, applicable 3, (1 + 0,5) / 3 = 50 %.
// Alte Eigenrechnung aus framework_scores: 1 / 3 = 33 %.
describe('ComplianceProgressCard (R1-KW-6)', () => {
  it('zeigt die gepoolte Zahl aus dem Backend: „in Arbeit" zählt halb, nicht anwendbar zählt nicht', () => {
    render(
      <MemoryRouter>
        <ComplianceProgressCard
          scores={[{ framework_id: 'fw', framework_name: 'ISO27001', total_controls: 3, implemented_controls: 1, score_pct: 50 }]}
          overall={{ covered: 1, partial: 1, applicable: 3, score_pct: 50 }}
        />
      </MemoryRouter>,
    )
    expect(screen.getByText('50%')).toBeInTheDocument()
    expect(screen.queryByText('33%')).not.toBeInTheDocument()
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '50')
  })
})
