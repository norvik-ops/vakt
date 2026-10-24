import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen, fireEvent } from '@testing-library/react'
import { renderWithProviders } from '../test-utils'
import type { ScoreConfig } from '../hooks/useDashboard'

// A stored configuration with non-default values in every field, so the test
// can tell "sent back unchanged" from "overwritten with the default".
const STORED: ScoreConfig = {
  base_score: 60,
  crit_penalty: 7,
  crit_penalty_cap: 33,
  high_penalty: 3,
  high_penalty_cap: 12,
  breach_penalty: 25,
  breach_penalty_cap: 40,
  fw_bonus: 8,
  fw_bonus_cap: 24,
}

const mutate = vi.fn()

vi.mock('../hooks/useDashboard', () => ({
  useScoreConfig: () => ({ data: STORED, isLoading: false }),
  useUpdateScoreConfig: () => ({ mutate, isPending: false }),
}))

import ScoreConfigPage from './ScoreConfigPage'

beforeEach(() => { mutate.mockReset() })

describe('ScoreConfigPage', () => {
  it('shows only the weights that still reach the score', () => {
    renderWithProviders(<ScoreConfigPage />)
    // base_score, fw_bonus, fw_bonus_cap
    expect(screen.getAllByRole('spinbutton')).toHaveLength(3)
    expect(screen.queryByDisplayValue('25')).toBeNull() // breach_penalty
    expect(screen.queryByDisplayValue('33')).toBeNull() // crit_penalty_cap
  })

  it('sends the hidden penalties back unchanged on save', () => {
    renderWithProviders(<ScoreConfigPage />)
    fireEvent.change(screen.getByDisplayValue('60'), { target: { value: '65' } })
    fireEvent.click(screen.getByRole('button', { name: /speichern|save/i }))
    expect(mutate).toHaveBeenCalledTimes(1)
    expect(mutate.mock.calls[0][0]).toEqual({ ...STORED, base_score: 65 })
  })

  it('reset restores only the visible weights', () => {
    renderWithProviders(<ScoreConfigPage />)
    fireEvent.click(screen.getByRole('button', { name: /zurücksetzen|reset/i }))
    fireEvent.click(screen.getByRole('button', { name: /speichern|save/i }))
    expect(mutate.mock.calls[0][0]).toEqual({
      ...STORED, base_score: 70, fw_bonus: 10, fw_bonus_cap: 30,
    })
  })
})
