import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { TopBar } from './TopBar'
import { useAuthStore } from '../stores/auth'
import { mockApiServer, type ApiServerHandle } from '../../test-utils/apiServer'
import type { ApiResponse } from '../../api/contract'

/**
 * R1-21-A01 — signing out must actually end the server session.
 *
 * These tests deliberately do NOT mock apiFetch or the auth store's network
 * layer. They stub `fetch` and let the real apiFetch run, because the defect
 * was precisely that no request was ever made: a test that mocks the request
 * layer one level higher cannot tell "logout posted nothing" apart from
 * "logout posted correctly".
 *
 * Falsification check performed on this file: with the `await apiFetch(...)`
 * line removed from TopBar.logout(), "posts to /auth/logout" fails with
 * `expected [] to have length 1`. The assertion carries the claim.
 */

const navigateMock = vi.fn()
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return { ...actual, useNavigate: () => navigateMock }
})

// NotificationBell opens an SSE stream on mount; it is irrelevant here and
// EventSource does not exist in jsdom.
vi.mock('./NotificationBell', () => ({ NotificationBell: () => null }))
vi.mock('./ChangelogPopover', () => ({ ChangelogPopover: () => null }))

function renderTopBar(client: QueryClient) {
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <TopBar onOpenSearch={vi.fn()} onOpenShortcuts={vi.fn()} />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

async function openUserMenuAndSignOut() {
  fireEvent.click(screen.getByRole('button', { name: /Benutzermenü/i }))
  const signOut = await screen.findByRole('menuitem', { name: /auth\.logout|abmelden|log ?out/i })
  fireEvent.click(signOut)
}

describe('TopBar — sign out (R1-21-A01)', () => {
  let server: ApiServerHandle
  let alertSpy: ReturnType<typeof vi.fn>

  beforeEach(() => {
    navigateMock.mockClear()
    // jsdom has no alert; without the stub it logs "Not implemented" and the
    // assertion below would have nothing to read.
    alertSpy = vi.spyOn(window, 'alert').mockImplementation(() => undefined)
    useAuthStore.setState({
      user: { id: 'u1', email: 't@example.com', display_name: 'Testnutzer', roles: ['Admin'] },
      hydrating: false,
    })
    const ok: ApiResponse<'/auth/logout', 'post'> = { status: 'logged out' }
    server = mockApiServer({ 'POST /auth/logout': { body: ok } })
  })

  afterEach(() => {
    server.restore()
    alertSpy.mockRestore()
    vi.unstubAllGlobals()
  })

  it('posts to /auth/logout — the access_token cookie is HttpOnly, only the server can end the session', async () => {
    renderTopBar(new QueryClient())
    await openUserMenuAndSignOut()

    await waitFor(() => {
      const logoutCalls = server.requests.filter(
        (r) => r.method === 'POST' && r.url.endsWith('/auth/logout'),
      )
      expect(logoutCalls).toHaveLength(1)
    })
  })

  it('clears the local user and redirects even when the server call fails', async () => {
    // An expired or already-revoked session answers 4xx. Leaving the user on a
    // page that still looks authenticated would be worse than the failed revoke.
    server.route('POST /auth/logout', { status: 400, body: { error: 'missing authorization header' } })

    renderTopBar(new QueryClient())
    await openUserMenuAndSignOut()

    await waitFor(() => { expect(useAuthStore.getState().user).toBeNull() })
    expect(navigateMock).toHaveBeenCalledWith('/login')
  })

  it('says so when the revocation could not be confirmed — the cookie is still there', async () => {
    // A 5xx (like a network failure, unlike a 4xx) means the server never
    // accepted the revocation AND never sent the Set-Cookie that clears the
    // HttpOnly access_token. The session may still be live and one reload puts
    // this user back in, so staying silent here is the thing that leaves
    // someone signed in on a shared machine.
    server.route('POST /auth/logout', { status: 503, body: { error: 'upstream unavailable' } })

    renderTopBar(new QueryClient())
    await openUserMenuAndSignOut()

    await waitFor(() => { expect(alertSpy).toHaveBeenCalledTimes(1) })
    expect(alertSpy.mock.calls[0][0]).toMatch(/noch gültig/i)

    // The warning does not come at the cost of the teardown — both happen.
    expect(useAuthStore.getState().user).toBeNull()
    expect(navigateMock).toHaveBeenCalledWith('/login')
  })

  it('stays silent when the server answered 4xx — that session was already gone', async () => {
    // Guards the other half of the classification. A warning on every expired
    // session trains people to click it away, and then the one that matters
    // gets clicked away too.
    server.route('POST /auth/logout', { status: 401, body: { error: 'invalid token' } })

    renderTopBar(new QueryClient())
    await openUserMenuAndSignOut()

    await waitFor(() => { expect(useAuthStore.getState().user).toBeNull() })
    expect(alertSpy).not.toHaveBeenCalled()
  })

  it('stays silent on a clean sign-out', async () => {
    renderTopBar(new QueryClient())
    await openUserMenuAndSignOut()

    await waitFor(() => { expect(navigateMock).toHaveBeenCalledWith('/login') })
    expect(alertSpy).not.toHaveBeenCalled()
  })

  it('empties the query cache so the next user on this tab sees no stale data', async () => {
    const client = new QueryClient()
    client.setQueryData(['vaktcomply', 'controls'], [{ id: 'c1', title: 'geheim' }])

    renderTopBar(client)
    await openUserMenuAndSignOut()

    await waitFor(() => {
      expect(client.getQueryData(['vaktcomply', 'controls'])).toBeUndefined()
    })
  })
})
