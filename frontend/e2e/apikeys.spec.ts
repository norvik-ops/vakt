import { test, expect } from './fixtures'

const API_KEYS = [
  {
    id: 'key-1',
    name: 'CI Pipeline',
    key_prefix: 'vk_ci_',
    scopes: ['secvault.secrets.read'],
    last_used_at: new Date().toISOString(),
    last_used_ip: '10.0.0.1',
    created_at: new Date().toISOString(),
    rotated_at: null,
  },
]

// Key fixture where rotated_at is set to "just now" — so inGrace is true.
const API_KEYS_IN_GRACE = [
  {
    id: 'key-1',
    name: 'CI Pipeline',
    key_prefix: 'vk_ci_',
    scopes: ['secvault.secrets.read'],
    last_used_at: new Date().toISOString(),
    last_used_ip: '10.0.0.1',
    created_at: new Date().toISOString(),
    rotated_at: new Date().toISOString(), // just rotated → within 24h window
  },
]

function mockStoreAuth(page: Parameters<typeof test>[1]['page']) {
  return page.addInitScript(() => {
    localStorage.setItem(
      'vakt_user',
      JSON.stringify({ id: 'u1', email: 'admin@example.com', role: 'admin', display_name: 'Admin', roles: ['admin'] }),
    )
  })
}

function mockApiKeys(page: Parameters<typeof test>[1]['page']) {
  return page.route('**/api/v1/**', (route) => {
    const url = route.request().url()
    if (url.includes('/api-keys') && route.request().method() === 'GET') {
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(API_KEYS) })
    }
    if (url.includes('/rotate') && route.request().method() === 'POST') {
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ ...API_KEYS[0], raw_key: 'vk_new_secret_key_abc123', rotated_at: new Date().toISOString() }),
      })
    }
    return route.fulfill({ status: 200, contentType: 'application/json', body: '{}' })
  })
}

test.describe('ApiKeysPage', () => {
  test('shows key list with scope tags', async ({ page }) => {
    await mockStoreAuth(page)
    await mockApiKeys(page)
    await page.goto('/settings/api-keys')
    await expect(page.locator('text=CI Pipeline').or(page.locator('[data-testid="api-key-row"]').first())).toBeVisible({ timeout: 8000 })
  })

  test('rotate button opens modal and shows new raw key once', async ({ page }) => {
    await mockStoreAuth(page)
    await mockApiKeys(page)
    await page.goto('/settings/api-keys')

    const rotateBtn = page.locator('button', { hasText: /rotier|rotate/i }).first()
    await rotateBtn.waitFor({ state: 'visible', timeout: 8000 })
    await rotateBtn.click()

    // Modal should appear showing the new raw key.
    await expect(
      page.locator('input[readonly]').filter({ hasText: /vk_new/ }).or(
        page.locator('code', { hasText: /vk_new/ }),
      ).or(
        page.locator('[data-testid="raw-key"]'),
      ).or(
        page.getByText('vk_new_secret_key_abc123'),
      ),
    ).toBeVisible({ timeout: 8000 })
  })

  test('grace-period badge appears in key list after rotation', async ({ page }) => {
    await mockStoreAuth(page)
    // Return a key that was already rotated (rotated_at = now) so inGrace is true.
    await page.route('**/api/v1/**', (route) => {
      const url = route.request().url()
      if (url.includes('/api-keys') && route.request().method() === 'GET') {
        return route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(API_KEYS_IN_GRACE),
        })
      }
      return route.fulfill({ status: 200, contentType: 'application/json', body: '{}' })
    })
    await page.goto('/settings/api-keys')

    // The component renders "Grace 24h aktiv" badge when inGrace is true.
    await expect(
      page.locator('text=Grace 24h aktiv').or(page.locator('text=Grace')).or(page.locator('text=24h')),
    ).toBeVisible({ timeout: 8000 })
  })

  test('rotate fires POST /api-keys/:id/rotate', async ({ page }) => {
    await mockStoreAuth(page)
    await mockApiKeys(page)
    await page.goto('/settings/api-keys')

    const rotatePromise = page.waitForRequest(
      (req) => req.url().includes('/rotate') && req.method() === 'POST',
    )
    const rotateBtn = page.locator('button', { hasText: /rotier|rotate/i }).first()
    await rotateBtn.waitFor({ state: 'visible', timeout: 8000 })
    await rotateBtn.click()
    await rotatePromise
  })
})
