import { createRef } from 'react'
import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { DashboardLayout } from './DashboardLayout'
import { loadWidgets } from './WidgetConfigPanel'
import type { DashboardScore } from '../hooks/useDashboard'

// UX-37: die Karte oben links hieß „Security Score — Gesamtbewertung" und
// zeigte 70 + 10 je Framework: 70/100 auf einer leeren Instanz, 100/100 bei
// 1 von 214 umgesetzten Kontrollen. Sie zeigt jetzt den Umsetzungsgrad
// (Readiness nach ADR-0086) aus GET /dashboard/score.

function renderLayout(scoreData: Pick<DashboardScore, 'score' | 'has_data'> | undefined) {
  return render(
    <MemoryRouter>
      <DashboardLayout
        scoreLoading={false}
        scoreData={scoreData}
        scoreTrend={null}
        fwCount={1}
        fwLoading={false}
        editMode={false}
        setEditMode={vi.fn()}
        widgets={loadWidgets()}
        toggleWidget={vi.fn()}
        widgetMenuOpen={false}
        setWidgetMenuOpen={vi.fn()}
        widgetMenuRef={createRef<HTMLDivElement>()}
      />
    </MemoryRouter>,
  )
}

describe('DashboardLayout Umsetzungsgrad (UX-37)', () => {
  it('zeigt den Readiness-Wert mit neuer Beschriftung statt „Security Score"', () => {
    renderLayout({ score: 19, has_data: true })
    expect(screen.getByText('Umsetzungsgrad')).toBeInTheDocument()
    expect(screen.getByText('19 %')).toBeInTheDocument()
    expect(screen.getByText('Anteil umgesetzter Kontrollen')).toBeInTheDocument()
    expect(screen.queryByText(/Security Score/)).not.toBeInTheDocument()
    expect(screen.queryByText('Gesamtbewertung')).not.toBeInTheDocument()
    expect(screen.queryByText('/ 100')).not.toBeInTheDocument()
  })

  it('verlinkt auf die Frameworks, nicht mehr auf die Score-Konfiguration', () => {
    renderLayout({ score: 19, has_data: true })
    const link = screen.getByRole('link', { name: /Umsetzungsgrad: 19 Prozent/ })
    expect(link).toHaveAttribute('href', '/vaktcomply')
    expect(link.getAttribute('title')).toMatch(/Teilweise umgesetzte zählen halb/)
  })

  it('leere Instanz zeigt „Noch keine Daten", keine Zahl und schon gar nicht 70', () => {
    renderLayout({ score: 0, has_data: false })
    expect(screen.getByText('Noch keine Daten')).toBeInTheDocument()
    expect(screen.getByText('—')).toBeInTheDocument()
    expect(screen.queryByText(/70/)).not.toBeInTheDocument()
    expect(screen.queryByText('0 %')).not.toBeInTheDocument()
  })

  it('0 % mit Daten wird als 0 % angezeigt', () => {
    renderLayout({ score: 0, has_data: true })
    expect(screen.getByText('0 %')).toBeInTheDocument()
  })
})
