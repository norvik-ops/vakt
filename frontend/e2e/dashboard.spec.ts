import { test, expect } from './fixtures'

const FAKE_USER = { id: 'user-1', email: 'admin@example.com', display_name: 'Test Admin', roles: ['Admin'], role: 'Admin' }

test.describe('Dashboard', () => {
  test.beforeEach(async ({ page }) => {
    await page.addInitScript((u) => {
      localStorage.setItem('vakt_user', JSON.stringify(u))
    }, FAKE_USER)
    await page.route('**/api/v1/**', route =>
      route.fulfill({ status: 200, contentType: 'application/json', body: '{"data":[],"pagination":{"page":1,"limit":25,"total":0,"total_pages":1}}' })
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
          score: 72,
          score_history: [
            { date: '2026-04-01', score: 60 },
            { date: '2026-05-01', score: 72 },
          ],
          framework_scores: [],
          open_capas: 0,
          overdue_controls: 0,
          overdue_tasks: 0,
          critical_risks: 0,
          top_risks: [],
          recent_activity: [],
          policies_total: 0,
          policies_approved: 0,
          components: { critical_findings: 0, high_findings: 0, open_breaches: 0, active_frameworks: 0 },
        }),
      })
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
