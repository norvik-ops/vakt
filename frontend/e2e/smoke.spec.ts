import { test, expect } from '@playwright/test'

/**
 * Smoke tests — laufen gegen eine lokale Instanz (E2E_DEMO_URL, default: http://localhost).
 * Jeder Test prüft: Seite lädt, kein Error-Boundary, kein JS-Absturz,
 * main-Element sichtbar.
 *
 * Auth kommt aus .playwright/auth.json, erzeugt von smoke-setup.ts.
 */

const MODULES = [
  { name: 'Dashboard',    path: '/' },
  { name: 'Vakt Comply',  path: '/vaktcomply' },
  { name: 'Vakt Scan',    path: '/vaktscan' },
  { name: 'Vakt Vault',   path: '/vaktvault' },
  { name: 'Vakt Aware',   path: '/vaktaware' },
  { name: 'Vakt Privacy', path: '/vaktprivacy' },
  { name: 'Vakt HR',      path: '/vakthr' },
  { name: 'Settings',     path: '/settings' },
]

for (const mod of MODULES) {
  test(`${mod.name} — lädt ohne Fehler`, async ({ page }) => {
    const jsErrors: string[] = []
    page.on('pageerror', (err) => jsErrors.push(err.message))

    await page.goto(mod.path, { waitUntil: 'load' })

    // Kein Error-Boundary
    await expect(
      page.getByText(/something went wrong|unexpected application error|ein fehler ist aufgetreten/i)
    ).not.toBeVisible()

    // Kein JS-Absturz
    expect(jsErrors, `JS-Fehler auf ${mod.path}: ${jsErrors.join(' | ')}`).toHaveLength(0)

    // Seite hat sichtbaren Inhalt
    await expect(page.locator('main').first()).toBeVisible({ timeout: 5_000 })

    // Session ist noch aktiv (kein Redirect zur Login-Seite)
    expect(page.url()).not.toContain('/login')
  })
}

test('Login-Seite nach Logout erreichbar', async ({ page }) => {
  await page.goto('/login')
  await expect(page.locator('#email')).toBeVisible({ timeout: 5_000 })
})
