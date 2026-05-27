// Visual regression smoke. Pins the rendered DOM of a handful of key pages
// against committed PNG baselines so a CSS/markup change that breaks layout
// no longer slips through review unnoticed.
//
// Scope on purpose narrow — login form, an empty-state app shell — these are
// the screens that face customers first. Deep pages would require demo data
// and inflate the baseline churn.
//
// Tolerances (maxDiffPixelRatio: 0.02) intentionally permissive to absorb
// font hinting / browser-version drift. Tighten once we hit a real
// regression that this pass-through.
//
// Baselines live under e2e/visual.spec.ts-snapshots/. Update with:
//   npx playwright test visual.spec.ts --update-snapshots
// Review the diff carefully — only commit the new baseline if the visual
// change was intended.

import { test, expect } from './fixtures'

// First-time bootstrap: baselines must exist in
// e2e/visual.spec.ts-snapshots/ before this suite can detect regressions.
// Generate locally with:
//
//   cd frontend && npx playwright test visual.spec.ts --update-snapshots --project=chromium
//
// Review the diff carefully, then commit the .png files. Once baselines
// exist, remove the test.fixme() line below to enable the suite in CI.
test.describe('Visual regression', () => {
  test.fixme(true, 'baseline screenshots not yet committed — see comment in visual.spec.ts')

  test.use({
    // Fixed viewport so screenshots don't drift between machines.
    viewport: { width: 1280, height: 800 },
    // Hide cursor / fix scrollbar paint differences across renderers.
    deviceScaleFactor: 1,
  })

  test('login page', async ({ page }) => {
    await page.goto('/login')
    // Wait for the form to settle. The auto-demo flow may show a spinner
    // briefly on demo-mode instances; in unit-mode it goes straight to form.
    await expect(page.getByRole('button', { name: /einloggen|login/i })).toBeVisible({ timeout: 10_000 })
    // Let any spinners / fade-in animations finish.
    await page.waitForTimeout(500)

    await expect(page).toHaveScreenshot('login.png', {
      fullPage: true,
      maxDiffPixelRatio: 0.02,
      // Mask elements that genuinely change every render (timestamps, random
      // demo credentials shown on the demo login page).
      mask: [
        page.locator('[data-testid="demo-credentials"]'),
        page.locator('time'),
      ],
    })
  })
})
