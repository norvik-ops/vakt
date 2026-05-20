import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { axe } from 'vitest-axe'

import { SlideOver } from './SlideOver'

describe('SlideOver', () => {
  it('renders title and content when open', () => {
    render(
      <SlideOver open onClose={() => undefined} title="Test Panel">
        <p>Body content</p>
      </SlideOver>,
    )
    expect(screen.getByRole('dialog', { name: 'Test Panel' })).toBeInTheDocument()
    expect(screen.getByText('Body content')).toBeInTheDocument()
  })

  it('renders nothing when closed', () => {
    render(
      <SlideOver open={false} onClose={() => undefined} title="Test Panel">
        <p>Body content</p>
      </SlideOver>,
    )
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('calls onClose when the close button is clicked', () => {
    const onClose = vi.fn()
    render(
      <SlideOver open onClose={onClose} title="Test Panel">
        <p>Body</p>
      </SlideOver>,
    )
    fireEvent.click(screen.getByRole('button', { name: 'Schließen' }))
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('has no a11y violations when open', async () => {
    const { container } = render(
      <SlideOver open onClose={() => undefined} title="Test Panel" description="Subtitle">
        <p>Body</p>
      </SlideOver>,
    )
    expect(await axe(container)).toHaveNoViolations()
  })
})
