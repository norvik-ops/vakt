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
    expect(screen.getByText('Vakt Pro')).toBeInTheDocument()
    // CTA link must be present
    const link = screen.getByRole('link')
    // Not merely "a link": it must lead to the quote form. It used to point at a
    // Polar checkout that quoted a VAT-inflated price the website never advertised.
    expect(link).toHaveAttribute('href', expect.stringContaining('/angebot'))
  })

  it('shows ProUpgradeUI when error is FeatureLockedError', () => {
    vi.mocked(useFeature).mockReturnValue({ enabled: true, loading: false })

    renderWithProviders(
      <ProGate error={new FeatureLockedError('audit_pdf')}>
        <p>Children</p>
      </ProGate>,
    )

    expect(screen.queryByText('Children')).not.toBeInTheDocument()
    expect(screen.getByText('Vakt Pro')).toBeInTheDocument()
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
})
