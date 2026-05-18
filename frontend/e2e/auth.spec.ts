import { test, expect } from '@playwright/test'

test.describe('Authentication', () => {
  test('redirects unauthenticated users to /login', async ({ page }) => {
    await page.goto('/')
    await expect(page).toHaveURL(/\/login/)
  })

  test('shows error on invalid credentials', async ({ page }) => {
    await page.route('**/api/v1/auth/login', route =>
      route.fulfill({ status: 401, contentType: 'application/json', body: JSON.stringify({ error: 'Invalid credentials' }) })
    )
    await page.goto('/login')
    await page.fill('input[type="email"]', 'wrong@example.com')
    await page.fill('input[type="password"]', 'wrongpassword')
    await page.click('button[type="submit"]')
    await expect(page.locator('text=Invalid credentials').or(page.locator('[role="alert"]'))).toBeVisible()
  })

  test('logs in successfully with valid credentials', async ({ page }) => {
    await page.route('**/api/v1/auth/login', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          token: 'v2.local.testtoken',
          user: { id: 'user-1', email: 'admin@example.com', role: 'admin', org_id: 'org-1', mfa_enabled: false },
        }),
      })
    )
    await page.route('**/api/v1/**', route => route.fulfill({ status: 200, contentType: 'application/json', body: '{}' }))

    await page.goto('/login')
    await page.fill('input[type="email"]', 'admin@example.com')
    await page.fill('input[type="password"]', 'correctpassword')
    await page.click('button[type="submit"]')
    await expect(page).not.toHaveURL(/\/login/, { timeout: 5000 })
  })

  test('shows forgot password link on login page', async ({ page }) => {
    await page.goto('/login')
    await expect(page.locator('a', { hasText: /vergessen|forgot/i })).toBeVisible()
  })
})
