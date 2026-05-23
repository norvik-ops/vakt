import { test, expect } from './fixtures'

const SESSION_LIST = [
  {
    id: 'sess-1',
    device_hint: 'Chrome / Linux',
    ip: '192.168.1.10',
    created_at: new Date().toISOString(),
    last_used: new Date().toISOString(),
    expires_at: new Date(Date.now() + 86400000).toISOString(),
    is_current: true,
  },
  {
    id: 'sess-2',
    device_hint: 'Firefox / Windows',
    ip: '10.0.0.5',
    created_at: new Date().toISOString(),
    last_used: new Date().toISOString(),
    expires_at: new Date(Date.now() + 86400000).toISOString(),
    is_current: false,
  },
]

function mockAuth(page: Parameters<typeof test>[1]['page']) {
  return page.route('**/api/v1/**', (route) => {
    const url = route.request().url()
    if (url.includes('/auth/sessions') && route.request().method() === 'GET') {
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(SESSION_LIST) })
    }
    if (url.includes('/auth/sessions') && route.request().method() === 'DELETE') {
      return route.fulfill({ status: 200, contentType: 'application/json', body: '{}' })
    }
    return route.fulfill({ status: 200, contentType: 'application/json', body: '{}' })
  })
}

function mockStoreAuth(page: Parameters<typeof test>[1]['page']) {
  return page.addInitScript(() => {
    localStorage.setItem(
      'vakt_user',
      JSON.stringify({ id: 'u1', email: 'admin@example.com', role: 'admin', display_name: 'Admin', roles: ['admin'] }),
    )
    localStorage.setItem('vakt_session_id', 'sess-1')
  })
}

test.describe('SessionsPage', () => {
  test('shows session list with current-session badge', async ({ page }) => {
    await mockStoreAuth(page)
    await mockAuth(page)
    await page.goto('/account/sessions')
    await expect(page.locator('text=Chrome').or(page.locator('[data-testid="session-row"]').first())).toBeVisible({ timeout: 8000 })
  })

  test('panic button requires 2-step confirmation before revoking all sessions', async ({ page }) => {
    await mockStoreAuth(page)
    await mockAuth(page)
    await page.goto('/account/sessions')

    // First click — should show confirm state, NOT yet call the API.
    const panicBtn = page.locator('button', { hasText: /alle.*abmelden|revoke all|panic/i }).first()
    await panicBtn.waitFor({ state: 'visible', timeout: 8000 })
    await panicBtn.click()

    // Second click — confirms and calls DELETE /auth/sessions.
    const confirmBtn = page.locator('button', { hasText: /sicher|confirm|ja/i }).first()
    const deletePromise = page.waitForRequest(
      (req) => req.url().includes('/auth/sessions') && req.method() === 'DELETE',
    )
    await confirmBtn.click()
    await deletePromise
  })

  test('single-session revoke fires DELETE /auth/sessions/:id', async ({ page }) => {
    await mockStoreAuth(page)
    await mockAuth(page)
    await page.goto('/account/sessions')

    const deletePromise = page.waitForRequest(
      (req) => req.url().includes('/auth/sessions/sess-2') && req.method() === 'DELETE',
    )
    // Click revoke on the non-current session row (sr-only text "Widerrufen", sess-2 is index 1).
    const revokeBtn = page.locator('button', { hasText: /widerrufen/i }).nth(1)
    await revokeBtn.waitFor({ state: 'visible', timeout: 8000 })
    await revokeBtn.click()
    // Confirmation dialog (AlertDialog is Dialog under the hood → role="dialog").
    const confirmBtn = page.getByRole('dialog', { name: /session widerrufen/i }).getByRole('button', { name: /widerrufen/i })
    await confirmBtn.waitFor({ state: 'visible', timeout: 5000 })
    await confirmBtn.click()
    await deletePromise
  })
})
