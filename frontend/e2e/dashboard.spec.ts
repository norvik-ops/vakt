import { test, expect } from '@playwright/test'

// Helper: authenticate and store session
async function loginAsAdmin(page: import('@playwright/test').Page) {
  await page.goto('/login')
  await page.getByRole('textbox', { name: /e-mail/i }).fill(process.env.E2E_USER ?? 'admin@example.com')
  await page.getByLabel(/passwort|password/i).fill(process.env.E2E_PASS ?? 'changeme')
  await page.getByRole('button', { name: /anmelden|login/i }).click()
  await page.waitForURL('**/dashboard', { timeout: 10_000 })
}

test.describe('Dashboard', () => {
  test('redirects unauthenticated users to login', async ({ page }) => {
    await page.goto('/dashboard')
    await expect(page).toHaveURL(/\/login/)
  })

  test('shows security score after login', async ({ page }) => {
    await loginAsAdmin(page)
    await expect(page.getByText(/security score|gesamtbewertung/i)).toBeVisible()
    // Score is a number 0-100
    await expect(page.locator('text=/^\\d+$/')).toBeVisible()
  })

  test('shows five module cards', async ({ page }) => {
    await loginAsAdmin(page)
    const modules = ['SecPulse', 'SecVitals', 'SecVault', 'SecReflex', 'SecPrivacy']
    for (const mod of modules) {
      await expect(page.getByText(mod)).toBeVisible()
    }
  })
})
