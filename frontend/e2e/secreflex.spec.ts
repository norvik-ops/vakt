import { test, expect } from './fixtures'

const CAMPAIGN = {
  id: 'camp-1',
  org_id: 'org-1',
  name: 'Q2 Phishing Test',
  status: 'draft',
  template_id: 'tmpl-1',
  group_id: 'grp-1',
  landing_page_id: 'lp-1',
  track_opens: true,
  created_at: '2026-05-01T00:00:00Z',
  updated_at: '2026-05-01T00:00:00Z',
}

const TEMPLATE = {
  id: 'tmpl-1',
  name: 'IT-Support Passwort-Reset',
  subject: 'Dringend: Passwort zurücksetzen',
  body: '<p>Bitte setze dein Passwort zurück.</p>',
  preset: false,
  created_at: '2026-05-01T00:00:00Z',
}

const GROUP = {
  id: 'grp-1',
  name: 'Alle Mitarbeiter',
  source: 'manual',
  target_count: 42,
  created_at: '2026-05-01T00:00:00Z',
}

const STATS = {
  campaign_id: 'camp-1',
  sent: 42,
  opened: 15,
  clicked: 8,
  submitted: 3,
  reported: 5,
  open_rate: 0.357,
  click_rate: 0.190,
}

const MODULES = [
  { id: 'mod-1', title: 'Phishing erkennen', description: 'Grundlagen', duration_minutes: 15, assigned_count: 10, completed_count: 7, created_at: '2026-01-01T00:00:00Z' },
]

function mockHttp(page: import('@playwright/test').Page) {
  return page.route('**/api/v1/**', route => {
    const url = route.request().url()
    if (url.includes('/vaktaware/campaigns') && url.includes('camp-1/stats')) {
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(STATS) })
    }
    if (url.includes('/vaktaware/campaigns/camp-1')) {
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(CAMPAIGN) })
    }
    if (url.includes('/vaktaware/campaigns')) {
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ data: [CAMPAIGN], pagination: { page: 1, limit: 25, total: 1, total_pages: 1 } }) })
    }
    if (url.includes('/vaktaware/templates/presets')) {
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([]) })
    }
    if (url.includes('/vaktaware/templates')) {
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([TEMPLATE]) })
    }
    if (url.includes('/vaktaware/target-groups')) {
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([GROUP]) })
    }
    if (url.includes('/vaktaware/training-modules')) {
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ data: MODULES, pagination: { page: 1, limit: 25, total: 1, total_pages: 1 } }) })
    }
    return route.fulfill({ status: 200, contentType: 'application/json', body: '{}' })
  })
}

const FAKE_USER = { id: 'user-1', email: 'admin@example.com', display_name: 'Test Admin', roles: ['Admin'], role: 'Admin' }

async function login(page: import('@playwright/test').Page) {
  await page.addInitScript((u) => { localStorage.setItem('vakt_user', JSON.stringify(u)) }, FAKE_USER)
  await mockHttp(page)
}

test.describe('SecReflex — Campaigns', () => {
  test.beforeEach(async ({ page }) => {
    await login(page)
    await page.goto('/vaktaware/campaigns')
  })

  test('shows campaign list', async ({ page }) => {
    await expect(page.getByText('Q2 Phishing Test')).toBeVisible()
  })

  test('shows campaign status badge', async ({ page }) => {
    await expect(page.getByText(/draft|entwurf/i)).toBeVisible()
  })

  test('opens campaign detail', async ({ page }) => {
    const row = page.getByText('Q2 Phishing Test')
    await row.click()
    await expect(page).toHaveURL(/camp-1/)
  })
})

test.describe('SecReflex — Templates', () => {
  test.beforeEach(async ({ page }) => {
    await login(page)
    await page.goto('/vaktaware/templates')
  })

  test('shows template list', async ({ page }) => {
    await expect(page.getByText('IT-Support Passwort-Reset').or(page.getByText(/passwort/i)).first()).toBeVisible()
  })
})

test.describe('SecReflex — Training', () => {
  test.beforeEach(async ({ page }) => {
    await login(page)
    await page.goto('/vaktaware/training')
  })

  test('shows training module list', async ({ page }) => {
    await expect(page.getByText('Phishing erkennen').or(page.getByText(/training|modul/i)).first()).toBeVisible()
  })
})

test.describe('SecReflex — Navigation', () => {
  test('SecReflex is accessible from sidebar', async ({ page }) => {
    await login(page)
    await page.goto('/settings')
    const link = page.getByRole('link', { name: /aware|reflex|phishing|training/i })
    await expect(link.first()).toBeVisible()
  })
})
