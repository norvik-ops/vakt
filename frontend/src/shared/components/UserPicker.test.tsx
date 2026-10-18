import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen, fireEvent } from '@testing-library/react'
import { renderWithProviders } from '../../test-utils'
import { UserPicker } from './UserPicker'
import { useTeamMembers } from '../../hooks/useTeam'

vi.mock('../../hooks/useTeam', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../hooks/useTeam')>()
  return { ...actual, useTeamMembers: vi.fn() }
})

function member(id: string, name: string, email: string) {
  return { id, name, email, role: 'editor', platform_role: 'Editor', created_at: '2026-10-18T00:00:00Z' }
}

describe('UserPicker', () => {
  beforeEach(() => {
    vi.mocked(useTeamMembers).mockReturnValue({
      data: [member('1', 'Anna Admin', 'anna@example.org'), member('2', '', 'direkt@example.org')],
      isLoading: false,
    } as unknown as ReturnType<typeof useTeamMembers>)
  })

  // R1-KW-9: Ein per „Direkt anlegen“ erzeugter Nutzer hat keinen Anzeigenamen. Radix verbietet
  // <SelectItem value="">, das Öffnen der Liste warf und riss „Risiko erfassen“ mit.
  it('öffnet die Liste auch mit einem Teammitglied ohne Namen und zeigt dessen E-Mail', () => {
    renderWithProviders(<UserPicker value={undefined} onChange={() => {}} />)
    fireEvent.click(screen.getByRole('combobox'))
    expect(screen.getByRole('option', { name: 'Anna Admin' })).toBeInTheDocument()
    expect(screen.getByRole('option', { name: 'direkt@example.org' })).toBeInTheDocument()
  })

  it('gibt für ein Mitglied ohne Namen die E-Mail als Verantwortlichen zurück', () => {
    const onChange = vi.fn()
    renderWithProviders(<UserPicker value={undefined} onChange={onChange} />)
    fireEvent.click(screen.getByRole('combobox'))
    fireEvent.click(screen.getByRole('option', { name: 'direkt@example.org' }))
    expect(onChange).toHaveBeenCalledWith('direkt@example.org')
  })
})
