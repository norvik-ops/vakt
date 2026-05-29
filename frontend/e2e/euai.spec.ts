import { test, expect } from './fixtures'

const FAKE_USER = { id: 'user-1', email: 'admin@example.com', display_name: 'Test Admin', roles: ['Admin'], role: 'Admin' }

const EU_AI_DASHBOARD = {
  total_systems: 0,
  systems_by_risk_class: {},
  systems_by_status: {},
  systems_without_documentation: 0,
  high_risk_deadline: '2026-08-02T00:00:00Z',
  high_risk_deadline_days_left: 70,
  iso27001_mappings: [],
}

const AI_SYSTEMS_LIST = { data: [], pagination: { page: 1, limit: 25, total: 0, total_pages: 1 } }

test.describe('EU AI Act (vaktcomply)', () => {
  test.beforeEach(async ({ page }) => {
    await page.addInitScript((u) => { localStorage.setItem('vakt_user', JSON.stringify(u)) }, FAKE_USER)
    await page.route('**/api/v1/**', route =>
      route.fulfill({ status: 200, contentType: 'application/json', body: '{"data":[],"pagination":{"page":1,"limit":25,"total":0,"total_pages":1}}' })
    )
  })

  test('EU AI Act Inventar lädt', async ({ page }) => {
    await page.route('**/api/v1/vaktcomply/ai-systems**', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(AI_SYSTEMS_LIST),
      })
    )
    await page.goto('/vaktcomply/ai-systems')
    await expect(page).toHaveURL(/vaktcomply\/ai-systems/)
  })

  test('KI-System anlegen Navigation', async ({ page }) => {
    await page.route('**/api/v1/vaktcomply/ai-systems**', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(AI_SYSTEMS_LIST),
      })
    )
    await page.goto('/vaktcomply/ai-systems')
    // Expect a button or link to create new AI system
    await expect(
      page.locator('button, a').filter({ hasText: /Neues|System|Anlegen|Hinzufügen|New/i }).first()
    ).toBeVisible({ timeout: 8000 })
  })

  test('EU AI Act Dashboard navigierbar', async ({ page }) => {
    await page.route('**/api/v1/vaktcomply/eu-ai-act/dashboard**', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(EU_AI_DASHBOARD),
      })
    )
    await page.goto('/vaktcomply/eu-ai-act/dashboard')
    await expect(page).toHaveURL(/vaktcomply\/eu-ai-act\/dashboard/)
  })
})
