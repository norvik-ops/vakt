import { test, expect } from './fixtures'

const FAKE_USER = { id: 'user-1', email: 'admin@example.com', display_name: 'Test Admin', roles: ['Admin'], role: 'Admin' }

test.describe('Assets (SecPulse)', () => {
  test.beforeEach(async ({ page }) => {
    await page.addInitScript((u) => { localStorage.setItem('vakt_user', JSON.stringify(u)) }, FAKE_USER)
  })

  test('shows empty state when no assets exist', async ({ page }) => {
    await page.route('**/api/v1/**', route =>
      route.fulfill({ status: 200, contentType: 'application/json', body: '{}' })
    )
    await page.route('**/api/v1/secpulse/assets**', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: [], pagination: { page: 1, limit: 25, total: 0, total_pages: 1 } }),
      })
    )

    await page.goto('/secpulse/assets')
    await expect(page.getByText('Noch kein Asset angelegt').or(page.getByText('No Asset').or(page.getByText(/asset angelegt/i)))).toBeVisible({ timeout: 8000 })
  })

  test('opens create asset dialog', async ({ page }) => {
    await page.route('**/api/v1/**', route =>
      route.fulfill({ status: 200, contentType: 'application/json', body: '{}' })
    )
    await page.route('**/api/v1/secpulse/assets**', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: [], pagination: { page: 1, limit: 25, total: 0, total_pages: 1 } }),
      })
    )

    await page.goto('/secpulse/assets')
    await page.getByRole('button', { name: /neues asset|new asset|asset anlegen/i }).first().click()
    await expect(page.locator('[role="dialog"]')).toBeVisible({ timeout: 3000 })
    await expect(page.locator('input[id="asset-name"]')).toBeVisible()
  })
})
