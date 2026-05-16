import { test, expect } from '@playwright/test'

test.describe('Authentication', () => {
  test('login page renders and shows form', async ({ page }) => {
    await page.goto('/login')
    await expect(page.getByRole('heading', { name: /anmelden|login|willkommen/i })).toBeVisible()
    await expect(page.getByRole('textbox', { name: /e-mail/i })).toBeVisible()
    await expect(page.getByRole('button', { name: /anmelden|login/i })).toBeVisible()
  })

  test('invalid credentials show error', async ({ page }) => {
    await page.goto('/login')
    await page.getByRole('textbox', { name: /e-mail/i }).fill('invalid@example.com')
    await page.getByLabel(/passwort|password/i).fill('wrongpassword')
    await page.getByRole('button', { name: /anmelden|login/i }).click()
    await expect(page.getByText(/ungültig|invalid|fehler|error/i)).toBeVisible({ timeout: 5_000 })
  })
})
