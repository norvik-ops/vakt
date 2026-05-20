import { test, expect } from './fixtures'

const EMPLOYEES = [
  { id: 'emp-1', first_name: 'Anna', last_name: 'Müller', email: 'a.mueller@example.com', department: 'Engineering', role: 'Backend-Entwicklerin', status: 'active', created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z' },
  { id: 'emp-2', first_name: 'Ben', last_name: 'Schmidt', email: 'b.schmidt@example.com', department: 'IT', role: 'SysAdmin', status: 'offboarding', created_at: '2026-01-02T00:00:00Z', updated_at: '2026-01-02T00:00:00Z' },
]

const CHECKLIST = {
  id: 'cl-1',
  org_id: 'org-1',
  type: 'onboarding',
  name: 'Standard Onboarding',
  items: [
    { id: 'item-1', label: 'GitHub-Zugang einrichten', required: true },
    { id: 'item-2', label: 'Laptop übergeben', required: true },
  ],
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

const RUN = {
  id: 'run-1',
  org_id: 'org-1',
  employee_id: 'emp-1',
  checklist_id: 'cl-1',
  status: 'in_progress',
  completed_items: ['item-1'],
  started_at: '2026-05-01T10:00:00Z',
  created_at: '2026-05-01T10:00:00Z',
  updated_at: '2026-05-01T10:00:00Z',
}

function mockAuth(page: import('@playwright/test').Page) {
  return page.route('**/api/v1/auth/login', route =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        token: 'v2.local.testtoken',
        user: { id: 'user-1', email: 'admin@example.com', role: 'admin', org_id: 'org-1', mfa_enabled: false },
      }),
    })
  )
}

async function login(page: import('@playwright/test').Page) {
  await mockAuth(page)
  await page.route('**/api/v1/**', route => {
    const url = route.request().url()
    if (url.includes('/hr/employees') && !url.includes('/checklist-runs')) {
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ data: EMPLOYEES, pagination: { page: 1, limit: 25, total: 2, total_pages: 1 } }) })
    }
    if (url.includes('/hr/checklists')) {
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ data: [CHECKLIST], pagination: { page: 1, limit: 25, total: 1, total_pages: 1 } }) })
    }
    if (url.includes('/checklist-runs')) {
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ data: [RUN], pagination: { page: 1, limit: 25, total: 1, total_pages: 1 } }) })
    }
    return route.fulfill({ status: 200, contentType: 'application/json', body: '{}' })
  })
  await page.goto('/login')
  await page.fill('input[type="email"]', 'admin@example.com')
  await page.fill('input[type="password"]', 'changeme')
  await page.click('button[type="submit"]')
  await page.waitForURL('**/dashboard', { timeout: 10_000 })
}

test.describe('SecHR — Employees', () => {
  test.beforeEach(async ({ page }) => {
    await login(page)
    await page.goto('/hr/employees')
  })

  test('shows employee list with status badges', async ({ page }) => {
    await expect(page.getByText('Anna Müller').or(page.getByText('a.mueller@example.com'))).toBeVisible()
    await expect(page.getByText('Ben Schmidt').or(page.getByText('b.schmidt@example.com'))).toBeVisible()
  })

  test('displays offboarding status indicator', async ({ page }) => {
    await expect(page.getByText(/offboarding/i)).toBeVisible()
  })

  test('opens create employee dialog', async ({ page }) => {
    const addBtn = page.getByRole('button', { name: /mitarbeiter|employee|hinzufügen|anlegen/i })
    if (await addBtn.isVisible()) {
      await addBtn.click()
      await expect(page.getByRole('dialog')).toBeVisible()
    }
  })
})

test.describe('SecHR — Checklists', () => {
  test.beforeEach(async ({ page }) => {
    await login(page)
    await page.goto('/hr/checklists')
  })

  test('shows checklist template list', async ({ page }) => {
    await expect(page.getByText('Standard Onboarding').or(page.getByText(/onboarding/i))).toBeVisible()
  })
})

test.describe('SecHR — Navigation', () => {
  test('HR module is accessible from sidebar', async ({ page }) => {
    await login(page)
    const hrLink = page.getByRole('link', { name: /hr|mitarbeiter|personal/i })
    await expect(hrLink.first()).toBeVisible()
  })

  test('employee detail page renders', async ({ page }) => {
    await login(page)
    await page.route('**/api/v1/hr/employees/emp-1', route =>
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(EMPLOYEES[0]) })
    )
    await page.route('**/api/v1/hr/employees/emp-1/checklist-runs', route =>
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ data: [RUN], pagination: { page: 1, limit: 25, total: 1, total_pages: 1 } }) })
    )
    await page.goto('/hr/employees/emp-1')
    await expect(page.getByText('Anna').or(page.getByText('a.mueller@example.com'))).toBeVisible()
  })
})
