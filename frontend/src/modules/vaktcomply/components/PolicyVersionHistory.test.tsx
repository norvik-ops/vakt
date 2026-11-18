import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import PolicyVersionHistory from './PolicyVersionHistory'

// UX-22: Die Historie zeigte nur „v1", die Detailseite „Revision 2" — zwei Namen
// für dieselbe Nummer, und der aktuelle Stand fehlte in der Liste.
vi.mock('../hooks/usePolicyVersions', () => ({
  usePolicyVersions: () => ({
    data: [{
      id: 'pv-1', org_id: 'org-1', policy_id: 'p-1', version: 1, title: 'Passwortrichtlinie',
      content: '', status: 'draft', version_note: 'Erstfassung', updated_by: 'Erika', created_at: '2026-01-01T00:00:00Z',
    }],
    isLoading: false,
    isError: false,
  }),
}))

describe('PolicyVersionHistory — Revisionen (UX-22)', () => {
  it('nennt alle Stände „Revision" und führt den aktuellen Stand oben', () => {
    render(<PolicyVersionHistory policyId="p-1" currentVersion={2} />)

    expect(screen.getByTestId('policy-revision-current')).toHaveTextContent('Revision 2aktueller Stand')
    expect(screen.getByText('Revision 1')).toBeInTheDocument()
    expect(screen.queryByText(/^v\d/)).toBeNull()
  })
})
