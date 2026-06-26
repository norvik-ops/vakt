import { describe, it, expect, beforeEach } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithProviders } from '../test-utils'
import { useAuthStore } from '../shared/stores/auth'
import AdminHubPage from './AdminHubPage'

beforeEach(() => {
  useAuthStore.setState({ user: null, hydrating: false })
})

describe('AdminHubPage', () => {
  it('redirects to / for users without admin or owner role', () => {
    useAuthStore.setState({
      user: { id: '1', email: 'a@b.de', display_name: 'Reg', roles: ['analyst'] },
      hydrating: false,
    })
    renderWithProviders(<AdminHubPage />, { initialPath: '/admin' })
    // The component returns <Navigate to="/" />; no header rendered.
    expect(screen.queryByRole('heading', { name: /Administration/i })).toBeNull()
  })

  it('renders the three admin tiles for an admin', () => {
    useAuthStore.setState({
      user: { id: '1', email: 'a@b.de', display_name: 'Adm', roles: ['admin'] },
      hydrating: false,
    })
    renderWithProviders(<AdminHubPage />, { initialPath: '/admin' })
    expect(screen.getByRole('heading', { name: /Administration/i })).toBeTruthy()
    expect(screen.getByRole('link', { name: /System-Status/i })).toBeTruthy()
    expect(screen.getByRole('link', { name: /Sicherheitsereignisse/i })).toBeTruthy()
    expect(screen.getByRole('link', { name: /Audit-Log/i })).toBeTruthy()
  })

  it('also renders for owner role', () => {
    useAuthStore.setState({
      user: { id: '1', email: 'o@b.de', display_name: 'Own', roles: ['owner'] },
      hydrating: false,
    })
    renderWithProviders(<AdminHubPage />, { initialPath: '/admin' })
    expect(screen.getByRole('heading', { name: /Administration/i })).toBeTruthy()
  })

  // The backend emits role names as "Admin" / "Owner" (capitalized) — see
  // backend/internal/shared/demoseed/seed.go and auth/service.go. The role
  // check must be case-insensitive so the sidebar/admin-hub renders for the
  // demo-seeded admin user.
  it('renders for backend-canonical "Admin" capitalisation', () => {
    useAuthStore.setState({
      user: { id: '1', email: 'real@b.de', display_name: 'Real', roles: ['Admin'] },
      hydrating: false,
    })
    renderWithProviders(<AdminHubPage />, { initialPath: '/admin' })
    expect(screen.getByRole('heading', { name: /Administration/i })).toBeTruthy()
  })
})
