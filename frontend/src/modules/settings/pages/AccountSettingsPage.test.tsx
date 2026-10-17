import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen, fireEvent, waitFor } from '@testing-library/react'
import { renderWithProviders } from '../../../test-utils'
import AccountSettingsPage from './AccountSettingsPage'
import { apiFetch } from '../../../api/client'

vi.mock('../../../api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../../api/client')>()
  return { ...actual, apiFetch: vi.fn() }
})

const SECRET = 'JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP'

function setupCalls(): number {
  return vi.mocked(apiFetch).mock.calls.filter(([path]) => path === '/auth/2fa/setup').length
}

beforeEach(() => {
  vi.mocked(apiFetch).mockReset()
  vi.mocked(apiFetch).mockImplementation((path: string) => {
    if (path === '/auth/2fa/status') return Promise.resolve({ enabled: false })
    if (path === '/auth/2fa/setup') return Promise.resolve({ secret: SECRET, uri: `otpauth://totp/Vakt?secret=${SECRET}` })
    return Promise.resolve([])
  })
})

// R1-KW-3: Der Dialog hat das Secret über onOpenChange geholt — das feuert bei einem
// von der Seite geöffneten (kontrollierten) Dialog nie. POST /2fa/setup ging nie raus.
describe('AccountSettingsPage — 2FA aktivieren (R1-KW-3)', () => {
  it('lädt beim Öffnen des Dialogs genau ein Secret und zeigt es an', async () => {
    renderWithProviders(<AccountSettingsPage />)
    const enable = await screen.findByRole('button', { name: /2FA aktivieren/i })
    await waitFor(() => { expect(enable).toBeEnabled() })
    fireEvent.click(enable)

    expect(await screen.findByText(SECRET)).toBeInTheDocument()
    expect(setupCalls()).toBe(1)
  })

  it('holt nach Schließen und erneutem Öffnen ein neues Secret', async () => {
    renderWithProviders(<AccountSettingsPage />)
    const enable = await screen.findByRole('button', { name: /2FA aktivieren/i })
    await waitFor(() => { expect(enable).toBeEnabled() })
    fireEvent.click(enable)
    await screen.findByText(SECRET)

    fireEvent.keyDown(document.activeElement ?? document.body, { key: 'Escape' })
    await waitFor(() => { expect(screen.queryByText(SECRET)).not.toBeInTheDocument() })

    fireEvent.click(screen.getByRole('button', { name: /2FA aktivieren/i }))
    await screen.findByText(SECRET)
    expect(setupCalls()).toBe(2)
  })
})
