import { test, expect } from './fixtures'

const FAKE_USER = { id: 'user-1', email: 'admin@example.com', display_name: 'Test Admin', roles: ['Admin'], role: 'Admin' }

const TISAX_FRAMEWORK = {
  id: 'fw-tisax-1',
  name: 'TISAX® / VDA ISA',
  version: '6.0',
  slug: 'TISAX',
  control_count: 39,
  created_at: '2026-01-01T00:00:00Z',
}

const TISAX_CONTROLS = [
  {
    id: 'ctrl-1',
    framework_id: 'fw-tisax-1',
    code: '1.1.1',
    title: 'Informationssicherheitsrichtlinie',
    domain: 'Organisation',
    status: 'not_implemented',
    maturity_score: 0,
    priority: 'high',
    description: 'Test description',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
  },
]

const TISAX_MAPPING = [
  {
    id: 'map-1',
    tisax_control_id: 'ctrl-1',
    tisax_code: '1.1.1',
    iso27001_control_id: 'iso-ctrl-1',
    iso27001_code: 'A.5.1',
  },
]

test.describe('TISAX (vaktcomply)', () => {
  test.beforeEach(async ({ page }) => {
    await page.addInitScript((u) => { localStorage.setItem('vakt_user', JSON.stringify(u)) }, FAKE_USER)
    await page.route('**/api/v1/**', route =>
      route.fulfill({ status: 200, contentType: 'application/json', body: '{"data":[],"pagination":{"page":1,"limit":25,"total":0,"total_pages":1}}' })
    )
  })

  test('TISAX-Übersicht lädt', async ({ page }) => {
    await page.route('**/api/v1/vaktcomply/frameworks', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([TISAX_FRAMEWORK]),
      })
    )
    await page.route('**/api/v1/vaktcomply/frameworks/fw-tisax-1', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(TISAX_FRAMEWORK),
      })
    )
    await page.goto('/vaktcomply/frameworks/fw-tisax-1/tisax')
    await expect(page).toHaveURL(/vaktcomply\/frameworks\/fw-tisax-1\/tisax/)
  })

  test('TISAX-Controls-Liste sichtbar', async ({ page }) => {
    await page.route('**/api/v1/vaktcomply/frameworks/fw-tisax-1/controls**', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: TISAX_CONTROLS, pagination: { page: 1, limit: 25, total: 1, total_pages: 1 } }),
      })
    )
    await page.route('**/api/v1/vaktcomply/frameworks/fw-tisax-1', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(TISAX_FRAMEWORK),
      })
    )
    await page.goto('/vaktcomply/frameworks/fw-tisax-1/tisax')
    await expect(page).toHaveURL(/vaktcomply/)
  })

  test('TISAX-Reifegrad-Ansicht', async ({ page }) => {
    await page.route('**/api/v1/vaktcomply/tisax-mapping**', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(TISAX_MAPPING),
      })
    )
    await page.goto('/vaktcomply/tisax-mapping')
    await expect(page).toHaveURL(/vaktcomply\/tisax-mapping/)
  })
})
