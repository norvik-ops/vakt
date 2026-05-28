import { test, expect } from './fixtures'

const DEMO_CREDS = {
  admin_email: 'admin@demo-abc123.demo',
  admin_password: 'abcdef1234567890', // gitleaks:allow
  analyst_email: 'analyst@demo-abc123.demo',
  analyst_password: '1234567890abcdef', // gitleaks:allow
  expires_in: 14400,
}

test.describe('Demo Mode', () => {
  test.beforeEach(async ({ page }) => {
    // Layer on top of the fixture's fetch override. This script runs second
    // (fixture registers first), so it wraps the fixture's patched fetch.
    // Requests to /health get demo:true; everything else falls through to
    // the fixture's version (which handles /api/v1/setup/status and real network).
    await page.addInitScript(() => {
      const origFetch = window.fetch.bind(window)
      window.fetch = async (input, init) => {
        const url =
          typeof input === 'string'
            ? input
            : input instanceof URL
              ? input.toString()
              : input.url
        if (url.endsWith('/health')) {
          return new Response(
            JSON.stringify({ demo: true, version: 'e2e-test', sso_enabled: false }),
            { status: 200, headers: { 'Content-Type': 'application/json' } },
          )
        }
        return origFetch(input, init)
      }
    })

    await page.route('**/api/v1/demo/start', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(DEMO_CREDS),
      })
    )
  })

  test('shows demo banner and credentials card', async ({ page }) => {
    await page.goto('/login')
    await expect(page.locator('text=Demo-Umgebung')).toBeVisible({ timeout: 5000 })
    await expect(page.locator('button', { hasText: 'Admin' })).toBeVisible({ timeout: 5000 })
    await expect(page.locator('button', { hasText: 'Analyst' })).toBeVisible()
  })

  test('one-click demo login navigates to dashboard (F041)', async ({ page }) => {
    // F041: clicking a demo-user button no longer pre-fills the form;
    // it POSTs to /demo/login server-side and navigates home. Mock the
    // endpoint to return the standard LoginResponse shape and assert the
    // SPA actually navigates away from /login.
    await page.route('**/api/v1/demo/login', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          access_token: 'v4.local.demo-test',
          refresh_token: 'demo-refresh',
          expires_in: 3600,
          session_id: 'demo-session-1',
          user: { id: 'demo-1', email: DEMO_CREDS.admin_email, display_name: 'Demo Admin', roles: ['Admin'] },
        }),
      })
    )
    await page.goto('/login')
    await expect(page.locator('button', { hasText: 'Admin' })).toBeVisible({ timeout: 5000 })
    await page.locator('button', { hasText: 'Admin' }).click()
    await expect(page).not.toHaveURL(/\/login/, { timeout: 5000 })
  })
})
