import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { AllowPrivateTargetToggle } from './shared'

// R1-SA27-01 — the SSRF private-target opt-in must be reachable from the
// settings form and must carry a visible warning. Default is OFF; enabling it
// hands `true` up to the tab's save payload (allow_private_target).
describe('AllowPrivateTargetToggle', () => {
  it('renders unchecked when checked=false and shows the SSRF warning', () => {
    render(<AllowPrivateTargetToggle id="t1" checked={false} onChange={() => {}} />)
    expect(screen.getByRole('checkbox')).not.toBeChecked()
    expect(screen.getByText(/RFC1918/)).toBeTruthy()
    expect(screen.getByText(/SSRF-Schutz/)).toBeTruthy()
  })

  it('reflects checked=true (pre-fill of a previously allowed target)', () => {
    render(<AllowPrivateTargetToggle id="t2" checked onChange={() => {}} />)
    expect(screen.getByRole('checkbox')).toBeChecked()
  })

  it('emits the new value when toggled on', () => {
    const onChange = vi.fn()
    render(<AllowPrivateTargetToggle id="t3" checked={false} onChange={onChange} />)
    fireEvent.click(screen.getByRole('checkbox'))
    expect(onChange).toHaveBeenCalledWith(true)
  })
})
