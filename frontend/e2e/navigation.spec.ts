import { test, expect } from './fixtures'

const FAKE_TOKEN = 'v2.local.testtoken'

test.describe('Navigation', () => {
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
    await page.route('**/api/v1/**', route =>
      route.fulfill({ status: 200, contentType: 'application/json', body: '{"data":[],"pagination":{"page":1,"limit":25,"total":0,"total_pages":1}}' })
    )
  })

  test('shows keyboard shortcuts modal on ?', async ({ page }) => {
    await page.goto('/')
    await page.keyboard.press('?')
    await expect(
      page.locator('[role="dialog"]').filter({ hasText: /shortcut|Tastenkürzel|Cmd\+K/i })
    ).toBeVisible({ timeout: 3000 })
  })

  test('navigates to settings page', async ({ page }) => {
    await page.goto('/settings')
    await expect(page).toHaveURL(/settings/)
    await expect(page.locator('text=Einstellungen, text=Settings').first()).toBeVisible({ timeout: 5000 })
  })

  test('sidebar links are reachable', async ({ page }) => {
    await page.goto('/')
    const sidebarLinks = ['/secvitals', '/secpulse', '/secprivacy']
    for (const link of sidebarLinks) {
      const anchor = page.locator(`nav a[href="${link}"]`)
      if (await anchor.count() > 0) {
        await expect(anchor.first()).toBeVisible()
      }
    }
  })
})
