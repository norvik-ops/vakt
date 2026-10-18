import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'

import { UserPermissionsEditor } from './UserPermissionsEditor'
import { useEnabledModules } from '../shared/moduleVisibility'
import type { ModulePermission } from '../hooks/useUserPermissions'

const mutate = vi.fn()
let loaded: ModulePermission[] = []

vi.mock('../hooks/useUserPermissions', () => ({
  useUserPermissions: () => ({ data: loaded, isLoading: false, error: null }),
  useUpdateUserPermissions: () => ({ mutate, isPending: false, error: null }),
}))

vi.mock('../shared/components/ProGate', () => ({
  ProGate: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}))

function enable(...modules: string[]) {
  useEnabledModules.setState({ modules: new Set(modules) })
}

describe('UserPermissionsEditor', () => {
  beforeEach(() => {
    mutate.mockReset()
    loaded = []
  })

  it('zeigt nur aktivierte Module', () => {
    enable('vaktcomply')
    render(<UserPermissionsEditor userId="u1" />)
    expect(screen.getByText('Vakt Comply')).toBeInTheDocument()
    for (const label of ['Vakt Scan', 'Vakt Vault', 'Vakt Aware', 'Vakt Privacy', 'Vakt HR']) {
      expect(screen.queryByText(label)).not.toBeInTheDocument()
    }
  })

  it('zeigt Vakt HR, wenn vakthr aktiviert ist', () => {
    enable('vaktcomply', 'vakthr')
    render(<UserPermissionsEditor userId="u1" />)
    expect(screen.getByText('Vakt HR')).toBeInTheDocument()
  })

  it('behält beim Speichern die Rechte abgeschalteter Module unverändert', () => {
    enable('vaktcomply')
    loaded = [
      { module: 'vaktcomply', can_read: true, can_write: false },
      { module: 'vaktscan', can_read: true, can_write: true },
    ]
    render(<UserPermissionsEditor userId="u1" />)
    fireEvent.click(screen.getByLabelText('Vakt Comply Schreiben'))
    fireEvent.click(screen.getByRole('button', { name: 'Berechtigungen speichern' }))

    expect(mutate).toHaveBeenCalledTimes(1)
    const payload = mutate.mock.calls[0][0] as ModulePermission[]
    expect(payload).toHaveLength(2)
    expect(payload).toContainEqual({ module: 'vaktcomply', can_read: true, can_write: true })
    expect(payload).toContainEqual({ module: 'vaktscan', can_read: true, can_write: true })
  })

  it('erfindet keine Zeilen für abgeschaltete Module ohne gespeicherte Rechte', () => {
    enable('vaktcomply', 'vakthr')
    render(<UserPermissionsEditor userId="u1" />)
    fireEvent.click(screen.getByRole('button', { name: 'Berechtigungen speichern' }))
    const payload = mutate.mock.calls[0][0] as ModulePermission[]
    expect(payload.map((p) => p.module).sort()).toEqual(['vaktcomply', 'vakthr'])
  })
})
