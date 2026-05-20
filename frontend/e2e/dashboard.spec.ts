import { test, expect } from './fixtures'

const FAKE_TOKEN = 'v2.local.testtoken'

test.describe('Dashboard', () => {
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
    await page.route('**/api/v1/dashboard**', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          compliance_score: 72,
          open_findings: 14,
          open_risks: 5,
          active_incidents: 2,
          score_history: [
            { date: '2026-04-01', score: 60 },
            { date: '2026-05-01', score: 72 },
          ],
        }),
      })
    )
    await page.route('**/api/v1/**', route =>
      route.fulfill({ status: 200, contentType: 'application/json', body: '{"data":[],"pagination":{"page":1,"limit":25,"total":0,"total_pages":1}}' })
    )
  })

  test('renders dashboard with score widget', async ({ page }) => {
    await page.goto('/')
    await expect(page.locator('text=72').or(page.locator('text=Compliance'))).toBeVisible({ timeout: 8000 })
  })

  test('opens global search with Ctrl+K', async ({ page }) => {
    await page.goto('/')
    await page.keyboard.press('Control+k')
    await expect(page.locator('[role="dialog"][aria-label="Suche"]').or(page.locator('input[aria-label="Globale Suche"]'))).toBeVisible({ timeout: 3000 })
  })
})
