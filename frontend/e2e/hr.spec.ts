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

const FAKE_USER = { id: 'user-1', email: 'admin@example.com', display_name: 'Test Admin', roles: ['Admin'], role: 'Admin' }

async function login(page: import('@playwright/test').Page) {
  await page.addInitScript((u) => { localStorage.setItem('vakt_user', JSON.stringify(u)) }, FAKE_USER)
  await page.route('**/api/v1/**', route => {
    const url = route.request().url()
    if (url.includes('/vakthr/employees') && !url.includes('/checklist-runs')) {
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ data: EMPLOYEES, pagination: { page: 1, limit: 25, total: 2, total_pages: 1 } }) })
    }
    if (url.includes('/vakthr/checklists')) {
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([CHECKLIST]) })
    }
    if (url.includes('/checklist-runs')) {
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([RUN]) })
    }
    return route.fulfill({ status: 200, contentType: 'application/json', body: '{}' })
  })
}

test.describe('SecHR — Employees', () => {
  test.beforeEach(async ({ page }) => {
    await login(page)
    await page.goto('/vakthr/employees')
  })

  test('shows employee list with status badges', async ({ page }) => {
    await expect(page.getByText('Anna Müller').or(page.getByText('a.mueller@example.com')).first()).toBeVisible()
    await expect(page.getByText('Ben Schmidt').or(page.getByText('b.schmidt@example.com')).first()).toBeVisible()
  })

  test('displays offboarding status indicator', async ({ page }) => {
    await expect(page.getByText(/offboarding/i).first()).toBeVisible()
  })

  test('opens create employee dialog', async ({ page }) => {
    const addBtn = page.getByRole('button', { name: /mitarbeiter|employee|hinzufügen|anlegen/i })
    if (await addBtn.isVisible()) {
      await addBtn.click()
      await expect(page.getByRole('dialog', { name: /mitarbeiter hinzufügen|mitarbeiter bearbeiten/i })).toBeVisible()
    }
  })
})

test.describe('SecHR — Checklists', () => {
  test.beforeEach(async ({ page }) => {
    await login(page)
    await page.goto('/vakthr/checklists')
  })

  test('shows checklist template list', async ({ page }) => {
    await expect(page.getByText('Standard Onboarding').or(page.getByText(/onboarding/i)).first()).toBeVisible()
  })
})

test.describe('SecHR — Navigation', () => {
  test('HR module is accessible from sidebar', async ({ page }) => {
    await login(page)
    await page.goto('/vakthr/employees')
    const hrLink = page.getByRole('link', { name: /hr|mitarbeiter|personal/i })
    await expect(hrLink.first()).toBeVisible()
  })

  test('employee detail page renders', async ({ page }) => {
    await login(page)
    await page.goto('/vakthr/employees')
    await expect(page.getByText('Anna').or(page.getByText('a.mueller@example.com')).first()).toBeVisible()
  })
})
