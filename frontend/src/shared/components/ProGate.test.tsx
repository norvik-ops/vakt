import { describe, it, expect, vi } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithProviders } from '../../test-utils'
import { ProGate } from './ProGate'
import { FeatureLockedError } from '../../api/client'

vi.mock('../hooks/useFeature', () => ({
  useFeature: vi.fn().mockReturnValue({ enabled: true, loading: false }),
}))

import { useFeature } from '../hooks/useFeature'

describe('ProGate', () => {
  it('renders children when no error and feature enabled', () => {
    vi.mocked(useFeature).mockReturnValue({ enabled: true, loading: false })

    renderWithProviders(
      <ProGate error={null}>
        <p>Protected content</p>
      </ProGate>,
    )

    expect(screen.getByText('Protected content')).toBeInTheDocument()
  })

  it('shows ProUpgradeUI immediately when feature is locked via license (no API roundtrip)', () => {
    vi.mocked(useFeature).mockReturnValue({ enabled: false, loading: false })

    renderWithProviders(
      <ProGate error={null} feature="audit_pdf">
        <p>Should not render</p>
      </ProGate>,
    )

    expect(screen.queryByText('Should not render')).not.toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Pro-Feature' })).toBeInTheDocument()
    // Not merely "a link": it must lead to the quote form. It used to point at a
    // Polar checkout that quoted a VAT-inflated price the website never advertised.
    const link = screen.getByRole('link', { name: /Jetzt upgraden/ })
    expect(link).toHaveAttribute('href', expect.stringContaining('/angebot'))
    // Wer schon einen Schlüssel hat, kommt direkt zur Lizenz-Eingabe.
    expect(screen.getByRole('link', { name: 'Lizenz aktivieren' })).toHaveAttribute('href', '/settings')
  })

  it('shows ProUpgradeUI when error is FeatureLockedError', () => {
    vi.mocked(useFeature).mockReturnValue({ enabled: true, loading: false })

    renderWithProviders(
      <ProGate error={new FeatureLockedError('audit_pdf')}>
        <p>Children</p>
      </ProGate>,
    )

    expect(screen.queryByText('Children')).not.toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Pro-Feature' })).toBeInTheDocument()
  })

  it('shows ErrorState when error is a generic server error', () => {
    vi.mocked(useFeature).mockReturnValue({ enabled: true, loading: false })

    renderWithProviders(
      <ProGate error={new Error('Internal Server Error')}>
        <p>Children</p>
      </ProGate>,
    )

    expect(screen.queryByText('Children')).not.toBeInTheDocument()
    expect(screen.getByText(/Internal Server Error/)).toBeInTheDocument()
  })

  it('renders children while feature check is loading (no flicker)', () => {
    vi.mocked(useFeature).mockReturnValue({ enabled: true, loading: true })

    renderWithProviders(
      <ProGate error={null} feature="audit_pdf">
        <p>Loading children</p>
      </ProGate>,
    )

    expect(screen.getByText('Loading children')).toBeInTheDocument()
  })

  // UX-01: DORA, TISAX und ISO 42001 liegen in keinem Tarif. Ein Pro-Kunde, der dort
  // „Jetzt upgraden“ liest, soll für etwas zahlen, das kein Kauf freischaltet.
  it('zeigt „Nicht im Angebot“ ohne Kauf-Link, wenn die API ein unverkauftes Merkmal sperrt', () => {
    vi.mocked(useFeature).mockReturnValue({ enabled: true, loading: false })

    renderWithProviders(
      <ProGate error={new FeatureLockedError('dora')}>
        <p>Children</p>
      </ProGate>,
    )

    expect(screen.getByRole('heading', { name: 'Nicht im Angebot' })).toBeInTheDocument()
    expect(screen.queryByRole('link')).not.toBeInTheDocument()
    expect(screen.queryByText(/upgraden/i)).not.toBeInTheDocument()
    expect(screen.queryByText(/Community Edition/)).not.toBeInTheDocument()
  })

  it('zeigt „Nicht im Angebot“ schon bei der Lizenz-Vorprüfung eines unverkauften Merkmals', () => {
    vi.mocked(useFeature).mockReturnValue({ enabled: false, loading: false })

    renderWithProviders(
      <ProGate error={null} feature="tisax">
        <p>Should not render</p>
      </ProGate>,
    )

    expect(screen.getByTestId('feature-lock-unsold')).toBeInTheDocument()
    expect(screen.queryByRole('link')).not.toBeInTheDocument()
  })
})
