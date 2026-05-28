import { describe, it, expect, vi, beforeEach } from 'vitest'
import { fireEvent, screen, waitFor } from '@testing-library/react'
import { renderWithProviders } from '../../test-utils'
import { useAuthStore } from '../stores/auth'
import { TopBar } from './TopBar'

// Heavy children that would pull in hooks/fetches the test does not care about.
vi.mock('./NotificationBell', () => ({
  NotificationBell: () => <div data-testid="notif-bell" />,
}))
vi.mock('./ChangelogPopover', () => ({
  ChangelogPopover: () => <div data-testid="changelog" />,
}))
// Stub react-i18next so tests render the translation keys directly without
// needing a configured i18n instance.
vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

function seedUser() {
  useAuthStore.setState({
    user: { id: '1', email: 'kim@example.com', display_name: 'Kim Tester', roles: ['admin'] },
    hydrating: false,
  })
}

beforeEach(() => {
  useAuthStore.setState({ user: null, hydrating: false })
})

describe('TopBar', () => {
  it('renders the search trigger and right-cluster utilities', () => {
    seedUser()
    renderWithProviders(<TopBar onOpenSearch={vi.fn()} onOpenShortcuts={vi.fn()} />)

    expect(screen.getByRole('button', { name: /Globale Suche/i })).toBeTruthy()
    expect(screen.getByTestId('notif-bell')).toBeTruthy()
    expect(screen.getByTestId('changelog')).toBeTruthy()
    expect(screen.getByRole('button', { name: /Tastaturkürzel/i })).toBeTruthy()
  })

  it('fires onOpenSearch when the search trigger is clicked', () => {
    seedUser()
    const onOpenSearch = vi.fn()
    renderWithProviders(<TopBar onOpenSearch={onOpenSearch} onOpenShortcuts={vi.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: /Globale Suche/i }))
    expect(onOpenSearch).toHaveBeenCalledTimes(1)
  })

  it('opens the user menu and exposes account/logout entries', async () => {
    seedUser()
    renderWithProviders(<TopBar onOpenSearch={vi.fn()} onOpenShortcuts={vi.fn()} />)

    // Menu is closed initially.
    expect(screen.queryByRole('menu')).toBeNull()

    fireEvent.click(screen.getByRole('button', { name: /Benutzermenü/i }))

    await waitFor(() => {
      expect(screen.getByRole('menu')).toBeTruthy()
    })

    // E-Mail in header line
    expect(screen.getByText('kim@example.com')).toBeTruthy()
    expect(screen.getByRole('menuitem', { name: /nav\.account/i })).toBeTruthy()
    expect(screen.getByRole('menuitem', { name: /nav\.sessions/i })).toBeTruthy()
    expect(screen.getByRole('menuitem', { name: /nav\.documentation/i })).toBeTruthy()
    expect(screen.getByRole('menuitem', { name: /auth\.logout/i })).toBeTruthy()
  })

  it('clearAuth runs when Logout is clicked', async () => {
    seedUser()
    renderWithProviders(<TopBar onOpenSearch={vi.fn()} onOpenShortcuts={vi.fn()} />)

    fireEvent.click(screen.getByRole('button', { name: /Benutzermenü/i }))
    await waitFor(() => { expect(screen.getByRole('menu')).toBeTruthy(); })
    fireEvent.click(screen.getByRole('menuitem', { name: /auth\.logout/i }))

    await waitFor(() => {
      expect(useAuthStore.getState().user).toBeNull()
    })
  })
})
