import { test, expect } from '@playwright/test'

async function loginAsAdmin(page: import('@playwright/test').Page) {
  await page.goto('/login')
  await page.getByRole('textbox', { name: /e-mail/i }).fill(process.env.E2E_USER ?? 'admin@example.com')
  await page.getByLabel(/passwort|password/i).fill(process.env.E2E_PASS ?? 'changeme')
  await page.getByRole('button', { name: /anmelden|login/i }).click()
  await page.waitForURL('**/dashboard', { timeout: 10_000 })
}

test.describe('SecPulse — SLA Dashboard', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    await page.goto('/secpulse/sla')
  })

  test('SLA dashboard renders with filter tabs', async ({ page }) => {
    await expect(page.getByRole('tab', { name: /alle/i })).toBeVisible()
    await expect(page.getByRole('tab', { name: /überfällig/i })).toBeVisible()
    await expect(page.getByRole('tab', { name: /gefährdet/i })).toBeVisible()
  })
})
