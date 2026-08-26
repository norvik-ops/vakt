import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import Login, { secondFactorBody } from './Login'
import { mockApiServer, type ApiServerHandle } from '../test-utils/apiServer'

/**
 * R1-14cA-03 — backup codes must actually be redeemable at login.
 *
 * The page sent `{mfa_token, code}` for everything the user typed. The backend
 * validates `code` as a TOTP and `backup_code` against the bcrypt hashes, so a
 * backup code in the `code` field went down the TOTP branch and came back 422
 * — locking out precisely the person who lost their authenticator, which is
 * the one situation these codes exist for.
 *
 * The assertions below read the request that LEFT the page (via a stubbed
 * fetch, with the real apiFetch in the path), not a mocked hook. A test that
 * mocked the submit helper would have passed against the broken version too.
 */

const navigateMock = vi.fn()
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return { ...actual, useNavigate: () => navigateMock }
})

describe('secondFactorBody — which field the typed value goes into', () => {
  it('sends a six-digit TOTP as `code`', () => {
    expect(secondFactorBody('tok', '123456')).toEqual({ mfa_token: 'tok', code: '123456' })
  })

  it('sends a backup code as `backup_code`, not `code`', () => {
    expect(secondFactorBody('tok', 'AB12-CD34')).toEqual({
      mfa_token: 'tok',
      backup_code: 'AB12-CD34',
    })
  })

  it('upper-cases and re-inserts the dash — bcrypt compares the raw string', () => {
    expect(secondFactorBody('tok', 'ab12cd34')).toEqual({
      mfa_token: 'tok',
      backup_code: 'AB12-CD34',
    })
    expect(secondFactorBody('tok', '  ab12-cd34  ')).toEqual({
      mfa_token: 'tok',
      backup_code: 'AB12-CD34',
    })
  })

  it('leaves anything it does not recognise in `code` for the server to reject', () => {
    // Not a decision this function should make on its own — 8 non-hex chars or
    // a truncated code belong to the backend's error message, not to a guess here.
    expect(secondFactorBody('tok', 'ZZZZ-ZZZZ')).toEqual({ mfa_token: 'tok', code: 'ZZZZ-ZZZZ' })
    expect(secondFactorBody('tok', '1234')).toEqual({ mfa_token: 'tok', code: '1234' })
  })

  it('does not mistake a numeric backup code for a TOTP', () => {
    // Backup codes are hex, so an all-digit one is possible. Eight characters
    // is never a TOTP.
    expect(secondFactorBody('tok', '1234-5678')).toEqual({
      mfa_token: 'tok',
      backup_code: '1234-5678',
    })
  })
})

describe('Login — second factor over the wire (R1-14cA-03)', () => {
  let server: ApiServerHandle

  function renderLogin() {
    return render(
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter>
          <Login />
        </MemoryRouter>
      </QueryClientProvider>,
    )
  }

  async function loginUntilMfaPrompt() {
    fireEvent.change(screen.getByLabelText('E-Mail'), {
      target: { value: 'kim@example.com' },
    })
    fireEvent.change(screen.getByLabelText('Passwort'), {
      target: { value: 'correct horse battery staple' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Anmelden' }))
    return screen.findByLabelText('Authentifizierungscode')
  }

  beforeEach(() => {
    navigateMock.mockClear()
    server = mockApiServer({
      'GET /health': { body: { status: 'ok', demo: false, sso_enabled: false } },
      'POST /auth/login': { body: { mfa_required: true, mfa_token: 'pending-token' } },
      'POST /auth/2fa/login-verify': {
        body: {
          user: { id: 'u1', email: 'kim@example.com', display_name: 'Kim', roles: ['Admin'] },
          csrf_token: 'csrf-1',
          session_id: 's1',
        },
      },
    })
  })

  afterEach(() => {
    server.restore()
    vi.unstubAllGlobals()
  })

  it('puts a backup code in the `backup_code` field of the login-verify request', async () => {
    renderLogin()
    const field = await loginUntilMfaPrompt()

    fireEvent.change(field, { target: { value: 'AB12-CD34' } })
    fireEvent.click(screen.getByRole('button', { name: 'Bestätigen' }))

    await waitFor(() => {
      const verify = server.requests.find((r) => r.url.endsWith('/auth/2fa/login-verify'))
      expect(verify).toBeDefined()
      expect(verify?.body).toEqual({ mfa_token: 'pending-token', backup_code: 'AB12-CD34' })
    })
  })

  it('still puts a TOTP in the `code` field', async () => {
    renderLogin()
    const field = await loginUntilMfaPrompt()

    fireEvent.change(field, { target: { value: '654321' } })
    fireEvent.click(screen.getByRole('button', { name: 'Bestätigen' }))

    await waitFor(() => {
      const verify = server.requests.find((r) => r.url.endsWith('/auth/2fa/login-verify'))
      expect(verify?.body).toEqual({ mfa_token: 'pending-token', code: '654321' })
    })
  })

  it('tells the user backup codes are accepted here', async () => {
    // The account page hands these codes out promising they work "falls Sie
    // keinen Zugriff auf Ihre Authenticator-App haben". This prompt used to
    // say only "6-stelliger Code aus Ihrer Authenticator-App".
    renderLogin()
    await loginUntilMfaPrompt()
    expect(screen.getByText(/Backup-Codes/i)).toBeTruthy()
  })
})
