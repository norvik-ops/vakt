import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen, fireEvent, within } from '@testing-library/react'
import { renderWithProviders } from '../../../test-utils'
import TeamSettingsPage from './TeamSettingsPage'
import {
  useTeamMembers,
  useUpdateRole,
  useInvitations,
  type TeamMember,
} from '../../../hooks/useTeam'

vi.mock('../../../hooks/useTeam', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../../hooks/useTeam')>()
  return {
    ...actual,
    useTeamMembers: vi.fn(),
    useUpdateRole: vi.fn(),
    useRemoveUser: vi.fn(() => ({ mutate: vi.fn(), isPending: false })),
    useInvitations: vi.fn(),
    useCreateInvitation: vi.fn(() => ({ mutate: vi.fn(), isPending: false, error: null })),
    useRevokeInvitation: vi.fn(() => ({ mutate: vi.fn(), isPending: false })),
    useCreateUser: vi.fn(() => ({ mutate: vi.fn(), isPending: false, error: null })),
  }
})

// ── fixtures ──────────────────────────────────────────────────────────────────

// Two admins so the last-admin guard never hides the role select, and one member
// whose users.role cache says "viewer" while org_members says InternalAuditor —
// the state a granted SoD role actually produces (ADR-0077: AuditorReadOnly and
// InternalAuditor both cache as "viewer").
const ADMIN: TeamMember = {
  id: 'u-admin', email: 'admin@example.test', name: 'Admin Eins',
  role: 'admin', platform_role: 'Admin', created_at: '2026-01-01T00:00:00Z',
}
const ADMIN2: TeamMember = {
  id: 'u-admin2', email: 'admin2@example.test', name: 'Admin Zwei',
  role: 'admin', platform_role: 'Admin', created_at: '2026-01-02T00:00:00Z',
}
const AUDITOR: TeamMember = {
  id: 'u-auditor', email: 'auditor@example.test', name: 'Anna Auditor',
  role: 'viewer', platform_role: 'InternalAuditor', created_at: '2026-01-03T00:00:00Z',
}

const mockUpdate = vi.fn()

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(useTeamMembers).mockReturnValue({
    data: [ADMIN, ADMIN2, AUDITOR], isLoading: false, isError: false, refetch: vi.fn(),
  } as unknown as ReturnType<typeof useTeamMembers>)
  vi.mocked(useInvitations).mockReturnValue({
    data: [], isLoading: false, isError: false, refetch: vi.fn(),
  } as unknown as ReturnType<typeof useInvitations>)
  vi.mocked(useUpdateRole).mockReturnValue({
    mutate: mockUpdate, isPending: false,
  } as unknown as ReturnType<typeof useUpdateRole>)
})

// ESK-13: an organisation could not give a second user the InternalAuditor role
// through the product at all — this select only offered admin/editor/viewer,
// which are not the words auth.RequireRole checks. Without a way to grant it, the
// ISO 27001 9.2 chain "create audit -> complete audit" is not walkable in a
// single-admin org, and the obvious self-help is an UPDATE against the database.
describe('TeamSettingsPage — assigning the InternalAuditor role', () => {
  it('offers InternalAuditor in the member role select', () => {
    renderWithProviders(<TeamSettingsPage />)

    const row = screen.getByText('auditor@example.test').closest('tr')
    expect(row).not.toBeNull()
    fireEvent.click(within(row as HTMLElement).getByRole('combobox'))

    expect(screen.getByRole('option', { name: /InternalAuditor/ })).toBeInTheDocument()
    expect(screen.getByRole('option', { name: /AuditorReadOnly/ })).toBeInTheDocument()
  })

  it('sends the platform role name the backend guards check', () => {
    renderWithProviders(<TeamSettingsPage />)

    const row = screen.getByText('admin2@example.test').closest('tr')
    fireEvent.click(within(row as HTMLElement).getByRole('combobox'))
    fireEvent.click(screen.getByRole('option', { name: /InternalAuditor/ }))

    expect(mockUpdate).toHaveBeenCalledWith(
      expect.objectContaining({ id: 'u-admin2', role: 'InternalAuditor' }),
      expect.anything(),
    )
  })

  it('shows the authoritative role, not the lossy users.role cache', () => {
    renderWithProviders(<TeamSettingsPage />)

    // AUTHORITATIVE: platform_role InternalAuditor, cache role 'viewer'. Reading
    // the cache would show the org's internal auditor as a plain viewer, and the
    // select would offer to "change" them to the role they already hold.
    const row = screen.getByText('auditor@example.test').closest('tr')
    expect(within(row as HTMLElement).getByRole('combobox')).toHaveTextContent('InternalAuditor')
  })

  // The four strings behind these keys existed in all four locales but were
  // rendered nowhere (REV-ESK13 §2.3). They are the only place the product says
  // what separates AuditorReadOnly from InternalAuditor from Viewer — the names
  // alone do not, and picking wrongly is what ADR-0055's segregation of duties
  // turns on.
  it('explains what the two audit roles do', () => {
    renderWithProviders(<TeamSettingsPage />)

    expect(screen.getByText(/Audits abschließen und Management-Reviews genehmigen/)).toBeInTheDocument()
    expect(screen.getByText(/Nur lesen, für externe Prüfer/)).toBeInTheDocument()
  })

  it('counts admins by the authoritative role for the last-admin lock', () => {
    // DRIFTED: platform_role Admin, cache role 'viewer'. This is the state every
    // role change made before the backend fix produced — the endpoint wrote only
    // users.role. A UI that counts the cache sees zero admins here, offers to
    // re-role the org's only Admin, and the backend's last-admin guard is what
    // stands between the operator and locking themselves out. That guard now
    // counts the same column requireAdmin authorises over, so this drifted admin
    // really is protected — see
    // TestRoleSourceOfTruth_guardAndGateReadTheSameColumn on the backend side.
    const DRIFTED_ADMIN: TeamMember = { ...ADMIN, role: 'viewer', platform_role: 'Admin' }
    vi.mocked(useTeamMembers).mockReturnValue({
      data: [DRIFTED_ADMIN, AUDITOR], isLoading: false, isError: false, refetch: vi.fn(),
    } as unknown as ReturnType<typeof useTeamMembers>)

    renderWithProviders(<TeamSettingsPage />)

    const adminRow = screen.getByText('admin@example.test').closest('tr')
    expect(within(adminRow as HTMLElement).queryByRole('combobox')).toBeNull()

    // Control: the non-admin row is still editable, so the assertion above is not
    // green because the whole table lost its selects.
    const auditorRow = screen.getByText('auditor@example.test').closest('tr')
    expect(within(auditorRow as HTMLElement).getByRole('combobox')).toBeInTheDocument()
  })
})
