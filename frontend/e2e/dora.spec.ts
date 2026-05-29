import { test, expect } from './fixtures'

const FAKE_USER = { id: 'user-1', email: 'admin@example.com', display_name: 'Test Admin', roles: ['Admin'], role: 'Admin' }

const DORA_DASHBOARD = {
  readiness_percent: 42,
  controls_total: 15,
  controls_implemented: 6,
  controls_in_progress: 3,
  incidents_open: 1,
  incidents_overdue: 0,
  third_parties_total: 4,
  third_parties_assessed: 2,
  resilience_tests_total: 2,
  resilience_tests_passed: 1,
  next_deadline_at: null,
}

const DORA_INCIDENTS = { data: [], pagination: { page: 1, limit: 25, total: 0, total_pages: 1 } }

const DORA_THIRD_PARTIES = []

test.describe('DORA (vaktcomply)', () => {
  test.beforeEach(async ({ page }) => {
    await page.addInitScript((u) => { localStorage.setItem('vakt_user', JSON.stringify(u)) }, FAKE_USER)
    // Catch-all for all API calls
    await page.route('**/api/v1/**', route =>
      route.fulfill({ status: 200, contentType: 'application/json', body: '{"data":[],"pagination":{"page":1,"limit":25,"total":0,"total_pages":1}}' })
    )
  })

  test('DORA-Übersicht lädt', async ({ page }) => {
    await page.route('**/api/v1/vaktcomply/dora/dashboard', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(DORA_DASHBOARD),
      })
    )
    await page.goto('/vaktcomply/dora/dashboard')
    // Expect a heading or key content element
    await expect(
      page.locator('h1, h2, [data-testid="dora-heading"]').or(page.locator('text=DORA')).first()
    ).toBeVisible({ timeout: 8000 })
  })

  test('IKT-Incident-Liste navigierbar', async ({ page }) => {
    await page.route('**/api/v1/vaktcomply/incidents**', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(DORA_INCIDENTS),
      })
    )
    await page.goto('/vaktcomply/incidents')
    await expect(page).toHaveURL(/vaktcomply\/incidents/)
  })

  test('Drittanbieter-Register öffnet', async ({ page }) => {
    await page.route('**/api/v1/vaktcomply/dora/third-parties**', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(DORA_THIRD_PARTIES),
      })
    )
    await page.goto('/vaktcomply/dora/third-parties')
    await expect(page).toHaveURL(/vaktcomply\/dora\/third-parties/)
  })
})
