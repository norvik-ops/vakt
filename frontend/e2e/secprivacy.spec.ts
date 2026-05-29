import { test, expect } from './fixtures'

const FAKE_USER = { id: 'user-1', email: 'admin@example.com', display_name: 'Test Admin', roles: ['Admin'], role: 'Admin' }

async function authenticate(page: import('@playwright/test').Page) {
  await page.addInitScript((u) => { localStorage.setItem('vakt_user', JSON.stringify(u)) }, FAKE_USER)
  await page.route('**/api/v1/**', route =>
    route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ data: [], pagination: { page: 1, limit: 25, total: 0, total_pages: 1 } }) })
  )
}

test.describe('SecPrivacy — DSR', () => {
  test.beforeEach(async ({ page }) => {
    await authenticate(page)
    await page.goto('/vaktprivacy/dsr')
  })

  test('DSR list page renders', async ({ page }) => {
    await expect(page.getByRole('heading', { name: /betroffenenanfragen|dsr/i })).toBeVisible()
  })

  test('can open create DSR dialog', async ({ page }) => {
    await page.getByRole('button', { name: /dsr anlegen|anlegen|neue anfrage|erstellen/i }).first().click()
    await expect(page.getByRole('dialog', { name: 'Datenschutzanfrage anlegen' })).toBeVisible()
    await expect(page.getByPlaceholder(/mustermann|name/i)).toBeVisible()
    await expect(page.locator('input[type="email"]')).toBeVisible()
  })

  test('export button is visible and clickable', async ({ page }) => {
    const exportBtn = page.getByRole('button', { name: /exportieren|export/i })
    await expect(exportBtn).toBeVisible()
  })
})
