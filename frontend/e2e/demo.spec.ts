import { test, expect } from './fixtures'

const DEMO_CREDS = {
  admin_email: 'admin@demo-abc123.demo',
  admin_password: 'abcdef1234567890',
  analyst_email: 'analyst@demo-abc123.demo',
  analyst_password: '1234567890abcdef',
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

  test('pre-fills credentials when clicking a demo user button', async ({ page }) => {
    await page.goto('/login')
    await expect(page.locator('button', { hasText: 'Admin' })).toBeVisible({ timeout: 5000 })
    await page.locator('button', { hasText: 'Admin' }).click()

    await expect(page.locator('input[type="email"]')).toHaveValue(DEMO_CREDS.admin_email)
    await expect(page.locator('input[type="password"]')).toHaveValue(DEMO_CREDS.admin_password)
  })
})
