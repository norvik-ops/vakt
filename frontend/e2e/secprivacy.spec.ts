import { test, expect } from '@playwright/test'

async function loginAsAdmin(page: import('@playwright/test').Page) {
  await page.goto('/login')
  await page.getByRole('textbox', { name: /e-mail/i }).fill(process.env.E2E_USER ?? 'admin@example.com')
  await page.getByLabel(/passwort|password/i).fill(process.env.E2E_PASS ?? 'changeme')
  await page.getByRole('button', { name: /anmelden|login/i }).click()
  await page.waitForURL('**/dashboard', { timeout: 10_000 })
}

test.describe('SecPrivacy — DSR', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    await page.goto('/secprivacy/dsrs')
  })

  test('DSR list page renders', async ({ page }) => {
    await expect(page.getByText(/betroffenenanfragen|dsr/i)).toBeVisible()
  })

  test('can open create DSR dialog', async ({ page }) => {
    await page.getByRole('button', { name: /neue anfrage|erstellen|neu/i }).click()
    await expect(page.getByRole('dialog')).toBeVisible()
    await expect(page.getByLabel(/name/i)).toBeVisible()
    await expect(page.getByLabel(/e-mail/i)).toBeVisible()
  })

  test('export button downloads CSV', async ({ page }) => {
    const downloadPromise = page.waitForEvent('download')
    await page.getByRole('button', { name: /exportieren|export/i }).click()
    const download = await downloadPromise
    expect(download.suggestedFilename()).toMatch(/dsr-export.*\.csv/)
  })
})
