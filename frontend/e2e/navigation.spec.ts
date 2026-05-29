import { test, expect } from './fixtures'

const FAKE_USER = { id: 'user-1', email: 'admin@example.com', display_name: 'Test Admin', roles: ['Admin'], role: 'Admin' }

test.describe('Navigation', () => {
  test.beforeEach(async ({ page }) => {
    await page.addInitScript((u) => {
      localStorage.setItem('vakt_user', JSON.stringify(u))
    }, FAKE_USER)

    await page.route('**/api/v1/**', route =>
      route.fulfill({ status: 200, contentType: 'application/json', body: '{"data":[],"pagination":{"page":1,"limit":25,"total":0,"total_pages":1}}' })
    )
  })

  test('shows keyboard shortcuts modal on ?', async ({ page }) => {
    await page.goto('/settings')
    // Wait for layout to mount and register keyboard handlers
    await page.waitForSelector('button[aria-label="Tastaturkürzel anzeigen"]')
    await page.keyboard.press('?')
    await expect(
      page.locator('[role="dialog"]').filter({ hasText: /shortcut|Tastaturkürzel|Tastenkürzel|Cmd\+K/i })
    ).toBeVisible({ timeout: 3000 })
  })

  test('navigates to settings page', async ({ page }) => {
    await page.goto('/settings')
    await expect(page).toHaveURL(/settings/)
    await expect(page.getByText('Einstellungen').or(page.getByText('Settings')).first()).toBeVisible({ timeout: 5000 })
  })

  test('sidebar links are reachable', async ({ page }) => {
    await page.goto('/settings')
    const sidebarLinks = ['/vaktcomply', '/vaktscan', '/vaktprivacy']
    for (const link of sidebarLinks) {
      const anchor = page.locator(`nav a[href="${link}"]`)
      if (await anchor.count() > 0) {
        await expect(anchor.first()).toBeVisible()
      }
    }
  })
})
