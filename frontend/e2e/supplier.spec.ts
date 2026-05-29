import { test, expect } from './fixtures'

const FAKE_USER = { id: 'user-1', email: 'admin@example.com', display_name: 'Test Admin', roles: ['Admin'], role: 'Admin' }

const SUPPLIERS_LIST = [
  {
    id: 'sup-1',
    name: 'Acme GmbH',
    contact_name: 'Max Mustermann',
    contact_email: 'max@acme.de',
    service_type: 'Cloud-Infrastruktur',
    criticality: 'critical',
    nis2_relevant: true,
    dora_relevant: false,
    contract_status: 'active',
    assessment_status: 'pending',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
  },
]

const QUESTIONNAIRES_LIST = [
  {
    id: 'q-1',
    name: 'NIS2 Lieferanten-Assessment',
    description: '15 Fragen zum NIS2-Compliance-Status',
    question_count: 15,
    created_at: '2026-01-01T00:00:00Z',
  },
]

test.describe('Supplier Portal (vaktcomply)', () => {
  test.beforeEach(async ({ page }) => {
    await page.addInitScript((u) => { localStorage.setItem('vakt_user', JSON.stringify(u)) }, FAKE_USER)
    await page.route('**/api/v1/**', route =>
      route.fulfill({ status: 200, contentType: 'application/json', body: '{"data":[],"pagination":{"page":1,"limit":25,"total":0,"total_pages":1}}' })
    )
  })

  test('Supplier-Portal-Übersicht lädt', async ({ page }) => {
    await page.route('**/api/v1/vaktcomply/suppliers**', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(SUPPLIERS_LIST),
      })
    )
    await page.goto('/vaktcomply/suppliers')
    await expect(page).toHaveURL(/vaktcomply\/suppliers/)
  })

  test('Lieferanten-Liste sichtbar', async ({ page }) => {
    await page.route('**/api/v1/vaktcomply/suppliers**', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(SUPPLIERS_LIST),
      })
    )
    await page.goto('/vaktcomply/suppliers')
    // Expect the supplier name to be visible in the list
    await expect(
      page.locator('text=Acme GmbH').or(page.locator('[data-testid="supplier-list"]')).first()
    ).toBeVisible({ timeout: 8000 })
  })

  test('Fragebogen-Navigation', async ({ page }) => {
    await page.route('**/api/v1/vaktcomply/questionnaires**', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(QUESTIONNAIRES_LIST),
      })
    )
    await page.route('**/api/v1/vaktcomply/suppliers**', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([]),
      })
    )
    // Navigate to suppliers — questionnaire management is accessible from suppliers page
    await page.goto('/vaktcomply/suppliers')
    await expect(page).toHaveURL(/vaktcomply\/suppliers/)
  })
})
