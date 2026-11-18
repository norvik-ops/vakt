import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { FrameworkProgress } from './DashboardComponents'
import { FrameworkSetupWizard } from '../modules/vaktcomply/components/FrameworkSetupWizard'

// Das Dashboard zeigte die Balken „EUAIACT · ISO27001", der Einrichtungs-
// Assistent „Willkommen bei EUAIACT". Beide nutzen jetzt den Katalognamen.

describe('FrameworkProgress — Anzeigename', () => {
  it('beschriftet die Balken mit dem Katalognamen statt mit dem Schlüssel', () => {
    render(
      <FrameworkProgress
        scores={[
          { framework_id: 'a', framework_name: 'EUAIACT', total_controls: 17, implemented_controls: 0, score_pct: 0 },
          { framework_id: 'b', framework_name: 'ISO27001', total_controls: 93, implemented_controls: 1, score_pct: 1 },
        ]}
      />,
    )
    expect(screen.getByText('EU AI Act')).toBeInTheDocument()
    expect(screen.getByText('ISO 27001')).toBeInTheDocument()
    expect(screen.getByRole('progressbar', { name: 'ISO 27001' })).toBeInTheDocument()
    expect(screen.queryByText('EUAIACT')).not.toBeInTheDocument()
    expect(screen.queryByText('ISO27001')).not.toBeInTheDocument()
  })
})

describe('FrameworkSetupWizard — Anzeigename', () => {
  it('begrüßt mit dem Katalognamen', () => {
    render(
      <MemoryRouter>
        <FrameworkSetupWizard framework={{ id: 'fw-ai', name: 'EUAIACT' }} onClose={() => {}} />
      </MemoryRouter>,
    )
    expect(screen.getByRole('dialog', { name: 'Willkommen bei EU AI Act' })).toBeInTheDocument()
    expect(screen.queryByText(/EUAIACT/)).not.toBeInTheDocument()
  })
})
