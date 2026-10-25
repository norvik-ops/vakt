import { describe, it, expect, vi } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithProviders } from '../test-utils'
import InviteAcceptPage from './InviteAcceptPage'
import { useInviteInfo, type InviteInfo } from '../hooks/useTeam'

vi.mock('../hooks/useTeam', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../hooks/useTeam')>()
  return { ...actual, useInviteInfo: vi.fn() }
})

function withInvite(role: string) {
  vi.mocked(useInviteInfo).mockReturnValue({
    data: {
      id: 'inv-1', email: 'invitee@example.test', role, invited_by: 'admin@example.test',
      expires_at: '2099-01-01T00:00:00Z',
    } as InviteInfo,
    isLoading: false, isError: false,
  } as unknown as ReturnType<typeof useInviteInfo>)
}

// The invite page names the role with the same label as the team page's
// roleBadge (shared roleLabel). Before, it switched over the lowercase names
// only and showed platform names raw.
describe('InviteAcceptPage — role label', () => {
  it.each([
    ['SecurityAnalyst', 'Editor'],
    ['Viewer', 'Viewer'],
    ['InternalAuditor', 'InternalAuditor'],
    ['viewer', 'Viewer'],
  ])('labels %s like the team page', (role, label) => {
    withInvite(role)
    renderWithProviders(<InviteAcceptPage />, { initialPath: '/invite/accept?token=abc' })
    expect(screen.getByText(new RegExp(`als ${label} eingeladen`))).toBeInTheDocument()
  })
})
