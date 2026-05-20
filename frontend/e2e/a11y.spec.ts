import { test, expect } from './fixtures'
import AxeBuilder from '@axe-core/playwright'

// Accessibility smoke tests using axe-core via Playwright.
// Runs on public, unauthenticated routes to catch WCAG-blocking issues
// before they reach production.

test.describe('Accessibility (axe-playwright)', () => {
  test('login page passes axe scan (WCAG 2.1 AA)', async ({ page }) => {
    await page.goto('/login')
    await page.waitForLoadState('networkidle')

    const results = await new AxeBuilder({ page })
      .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
      .analyze()

    expect(results.violations).toEqual([])
  })

  test('setup page passes axe scan', async ({ page }) => {
    await page.goto('/setup')
    await page.waitForLoadState('networkidle')

    const results = await new AxeBuilder({ page })
      .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
      .analyze()

    expect(results.violations).toEqual([])
  })
})
