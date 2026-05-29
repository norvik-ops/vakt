import { test, expect } from './fixtures'

const FAKE_USER = { id: 'user-1', email: 'admin@example.com', display_name: 'Test Admin', roles: ['Admin'], role: 'Admin' }

test.describe('Compliance (SecVitals)', () => {
  test.beforeEach(async ({ page }) => {
    await page.addInitScript((u) => { localStorage.setItem('vakt_user', JSON.stringify(u)) }, FAKE_USER)
    await page.route('**/api/v1/**', route =>
      route.fulfill({ status: 200, contentType: 'application/json', body: '{"data":[],"pagination":{"page":1,"limit":25,"total":0,"total_pages":1}}' })
    )
  })

  test('lists compliance frameworks', async ({ page }) => {
    await page.route('**/api/v1/vaktcomply/frameworks**', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([
          { id: 'fw-1', name: 'ISO 27001:2022', slug: 'ISO27001', total_controls: 93, implemented_controls: 45, score: 48 },
          { id: 'fw-2', name: 'NIS2', slug: 'NIS2', total_controls: 22, implemented_controls: 18, score: 82 },
        ]),
      })
    )

    await page.goto('/vaktcomply')
    await expect(page.locator('text=ISO 27001').or(page.locator('text=NIS2')).first()).toBeVisible({ timeout: 8000 })
  })

  test('navigates to controls list', async ({ page }) => {
    await page.route('**/api/v1/vaktcomply/frameworks**', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([
          { id: 'fw-1', name: 'ISO 27001:2022', slug: 'ISO27001', total_controls: 93, implemented_controls: 45, score: 48 },
        ]),
      })
    )

    await page.goto('/vaktcomply/fw-1/controls')
    await expect(page).toHaveURL(/vaktcomply/)
  })
})
