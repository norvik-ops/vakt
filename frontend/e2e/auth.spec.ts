import { test, expect } from './fixtures'

test.describe('Authentication', () => {
  test('redirects unauthenticated users to /login', async ({ page }) => {
    // F032: hydrate is gated on /auth/me. The fixture's default mock
    // returns 200 (most specs need an authenticated user), so to exercise
    // the redirect we need to override that with 401 first.
    await page.addInitScript(() => {
      const orig = window.fetch.bind(window)
      window.fetch = async (input, init) => {
        const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url
        if (url.includes('/api/v1/auth/me')) {
          return new Response('{}', { status: 401, headers: { 'Content-Type': 'application/json' } })
        }
        return orig(input, init)
      }
    })
    await page.goto('/')
    await expect(page).toHaveURL(/\/login/)
  })

  test('shows error on invalid credentials', async ({ page }) => {
    await page.route('**/api/v1/auth/login', route =>
      route.fulfill({ status: 400, contentType: 'application/json', body: JSON.stringify({ error: 'Invalid credentials' }) })
    )
    await page.goto('/login')
    await page.fill('input[type="email"]', 'wrong@example.com')
    await page.fill('input[type="password"]', 'wrongpassword')
    await page.click('button[type="submit"]')
    await expect(page.getByRole('alert').filter({ hasText: 'Invalid credentials' })).toBeVisible()
  })

  test('logs in successfully with valid credentials', async ({ page }) => {
    await page.route('**/api/v1/**', route => route.fulfill({ status: 200, contentType: 'application/json', body: '{}' }))
    await page.route('**/api/v1/auth/login', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          access_token: 'v4.local.testtoken',
          refresh_token: 'testrefresh',
          expires_in: 3600,
          user: { id: 'user-1', email: 'admin@example.com', display_name: 'Test Admin', roles: ['Admin'] },
          session_id: 'sess-1',
        }),
      })
    )

    await page.goto('/login')
    await page.fill('input[type="email"]', 'admin@example.com')
    await page.fill('input[type="password"]', 'correctpassword')
    await page.click('button[type="submit"]')
    await expect(page).not.toHaveURL(/\/login/, { timeout: 5000 })
  })

  test('shows forgot password link on login page', async ({ page }) => {
    await page.goto('/login')
    await expect(page.locator('a', { hasText: /vergessen|forgot/i })).toBeVisible()
  })
})
