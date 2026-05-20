import { test, expect } from './fixtures'

const FAKE_TOKEN = 'v2.local.testtoken'

test.describe('Compliance (SecVitals)', () => {
  test.beforeEach(async ({ page }) => {
    await page.addInitScript((token) => {
      localStorage.setItem('vakt_token', token)
    }, FAKE_TOKEN)

    await page.route('**/api/v1/auth/me', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ id: 'user-1', email: 'admin@example.com', role: 'admin', org_id: 'org-1', mfa_enabled: false }),
      })
    )
    await page.route('**/api/v1/**', route =>
      route.fulfill({ status: 200, contentType: 'application/json', body: '{"data":[],"pagination":{"page":1,"limit":25,"total":0,"total_pages":1}}' })
    )
  })

  test('lists compliance frameworks', async ({ page }) => {
    await page.route('**/api/v1/secvitals/frameworks**', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([
          { id: 'fw-1', name: 'ISO 27001:2022', slug: 'ISO27001', total_controls: 93, implemented_controls: 45, score: 48 },
          { id: 'fw-2', name: 'NIS2', slug: 'NIS2', total_controls: 22, implemented_controls: 18, score: 82 },
        ]),
      })
    )

    await page.goto('/secvitals')
    await expect(page.locator('text=ISO 27001').or(page.locator('text=NIS2'))).toBeVisible({ timeout: 8000 })
  })

  test('navigates to controls list', async ({ page }) => {
    await page.route('**/api/v1/secvitals/frameworks**', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([
          { id: 'fw-1', name: 'ISO 27001:2022', slug: 'ISO27001', total_controls: 93, implemented_controls: 45, score: 48 },
        ]),
      })
    )

    await page.goto('/secvitals/fw-1/controls')
    await expect(page).toHaveURL(/secvitals/)
  })
})
