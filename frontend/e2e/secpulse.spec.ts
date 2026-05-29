import { test, expect } from './fixtures'

const FAKE_USER = { id: 'user-1', email: 'admin@example.com', display_name: 'Test Admin', roles: ['Admin'], role: 'Admin' }

async function authenticate(page: import('@playwright/test').Page) {
  await page.addInitScript((u) => { localStorage.setItem('vakt_user', JSON.stringify(u)) }, FAKE_USER)
  await page.route('**/api/v1/**', route =>
    route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ data: [], pagination: { page: 1, limit: 25, total: 0, total_pages: 1 } }) })
  )
}

test.describe('SecPulse — SLA Dashboard', () => {
  test.beforeEach(async ({ page }) => {
    await authenticate(page)
    await page.goto('/vaktscan/sla')
  })

  test('SLA dashboard renders with filter tabs', async ({ page }) => {
    await expect(page.getByRole('button', { name: /alle/i })).toBeVisible()
    await expect(page.getByRole('button', { name: /überfällig/i })).toBeVisible()
    await expect(page.getByRole('button', { name: /gefährdet/i })).toBeVisible()
  })
})
