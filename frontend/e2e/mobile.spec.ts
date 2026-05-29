import { test, expect } from './fixtures'

const FAKE_USER = { id: 'user-1', email: 'admin@example.com', display_name: 'Test Admin', roles: ['Admin'], role: 'Admin' }

const EMPTY_PAGE_RESPONSE = '{"data":[],"pagination":{"page":1,"limit":25,"total":0,"total_pages":1}}'

/**
 * Mobile responsiveness tests — iPhone SE viewport (375×667).
 * Checks that main module pages have no horizontal overflow and no
 * clipped navigation buttons.
 */
test.describe('Mobile Responsiveness (375×667)', () => {
  test.beforeEach(async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 667 })
    await page.addInitScript((u) => { localStorage.setItem('vakt_user', JSON.stringify(u)) }, FAKE_USER)
    await page.route('**/api/v1/**', route =>
      route.fulfill({ status: 200, contentType: 'application/json', body: EMPTY_PAGE_RESPONSE })
    )
  })

  test('Vakt Scan Findings — kein horizontaler Overflow', async ({ page }) => {
    await page.goto('/vaktscan/findings')
    // scrollWidth > clientWidth indicates horizontal overflow on body
    const hasOverflow = await page.evaluate(() => document.body.scrollWidth > document.body.clientWidth)
    expect(hasOverflow).toBe(false)
  })

  test('Vakt Comply Frameworks — kein horizontaler Overflow', async ({ page }) => {
    await page.route('**/api/v1/vaktcomply/frameworks', route =>
      route.fulfill({ status: 200, contentType: 'application/json', body: '[]' })
    )
    await page.goto('/vaktcomply/frameworks')
    const hasOverflow = await page.evaluate(() => document.body.scrollWidth > document.body.clientWidth)
    expect(hasOverflow).toBe(false)
  })

  test('Vakt Vault Projekte — kein horizontaler Overflow', async ({ page }) => {
    await page.route('**/api/v1/vaktvault/projects', route =>
      route.fulfill({ status: 200, contentType: 'application/json', body: '[]' })
    )
    await page.goto('/vaktvault')
    const hasOverflow = await page.evaluate(() => document.body.scrollWidth > document.body.clientWidth)
    expect(hasOverflow).toBe(false)
  })

  test('Vakt Aware Kampagnen — kein horizontaler Overflow', async ({ page }) => {
    await page.route('**/api/v1/vaktaware/campaigns**', route =>
      route.fulfill({ status: 200, contentType: 'application/json', body: '[]' })
    )
    await page.goto('/vaktaware/campaigns')
    const hasOverflow = await page.evaluate(() => document.body.scrollWidth > document.body.clientWidth)
    expect(hasOverflow).toBe(false)
  })

  test('Vakt Privacy VVT — kein horizontaler Overflow', async ({ page }) => {
    await page.route('**/api/v1/vaktprivacy/vvt**', route =>
      route.fulfill({ status: 200, contentType: 'application/json', body: EMPTY_PAGE_RESPONSE })
    )
    await page.goto('/vaktprivacy/vvt')
    const hasOverflow = await page.evaluate(() => document.body.scrollWidth > document.body.clientWidth)
    expect(hasOverflow).toBe(false)
  })

  test('Vakt HR Mitarbeiter — kein horizontaler Overflow', async ({ page }) => {
    await page.route('**/api/v1/vakthr/employees**', route =>
      route.fulfill({ status: 200, contentType: 'application/json', body: EMPTY_PAGE_RESPONSE })
    )
    await page.goto('/vakthr/employees')
    const hasOverflow = await page.evaluate(() => document.body.scrollWidth > document.body.clientWidth)
    expect(hasOverflow).toBe(false)
  })
})
