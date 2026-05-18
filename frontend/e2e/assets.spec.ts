import { test, expect } from '@playwright/test'

const FAKE_TOKEN = 'v2.local.testtoken'

test.describe('Assets (SecPulse)', () => {
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
  })

  test('shows empty state when no assets exist', async ({ page }) => {
    await page.route('**/api/v1/secpulse/assets**', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: [], pagination: { page: 1, limit: 25, total: 0, total_pages: 1 } }),
      })
    )
    await page.route('**/api/v1/**', route =>
      route.fulfill({ status: 200, contentType: 'application/json', body: '{}' })
    )

    await page.goto('/secpulse/assets')
    await expect(page.locator('[data-testid="empty-state"]').or(page.locator('text=Kein Asset').or(page.locator('text=No assets')))).toBeVisible({ timeout: 8000 })
  })

  test('opens create asset dialog', async ({ page }) => {
    await page.route('**/api/v1/secpulse/assets**', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: [], pagination: { page: 1, limit: 25, total: 0, total_pages: 1 } }),
      })
    )
    await page.route('**/api/v1/**', route =>
      route.fulfill({ status: 200, contentType: 'application/json', body: '{}' })
    )

    await page.goto('/secpulse/assets')
    await page.click('button:has-text("Neu"), button:has-text("New"), button:has-text("Asset")')
    await expect(page.locator('[role="dialog"]')).toBeVisible({ timeout: 3000 })
    await expect(page.locator('input[id="asset-name"]').or(page.locator('label:has-text("Name") + input, label:has-text("Name") ~ * input'))).toBeVisible()
  })
})
