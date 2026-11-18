import { test, expect } from './fixtures'
import type { Page } from '@playwright/test'

/**
 * UX-39 — Handy-Breite 390 px: Seitenköpfe mit Aktionen liefen aus dem Bild,
 * die Seite bekam eine horizontale Scroll-Leiste.
 *
 * Prüft je Kernweg-Seite (mit leeren API-Antworten):
 *   - weder das Dokument noch der Scroll-Container der Seite ist breiter als der Bildschirm,
 *   - der Seitenkopf und seine Aktionen bleiben im Bild.
 * Und: Die Seitenleiste öffnet auf dem Handy als volle Leiste (auch wenn sie am
 * Desktop eingeklappt gespeichert ist) und schließt mit Escape.
 *
 * Was der Test NICHT prüft: Seiten mit echten Daten (lange Titel, volle Tabellen) —
 * das deckt der Live-Durchlauf gegen eine Kernweg-Instanz ab, siehe PR-Beschreibung.
 * Läuft gegen den Vite-Dev-Server oder per E2E_BASE_URL gegen `vite preview` (Build).
 */

const EMPTY_PAGE = '{"data":[],"pagination":{"page":1,"limit":25,"total":0,"total_pages":1}}'

async function mockApi(page: Page) {
  await page.route('**/api/v1/**', (route) =>
    route.fulfill({ status: 200, contentType: 'application/json', body: EMPTY_PAGE }),
  )
  for (const path of ['**/api/v1/vaktcomply/frameworks', '**/api/v1/vaktcomply/risks**', '**/api/v1/vaktcomply/evidence**']) {
    await page.route(path, (route) => route.fulfill({ status: 200, contentType: 'application/json', body: '[]' }))
  }
}

async function overflow(page: Page) {
  return page.evaluate(() => {
    const vw = document.documentElement.clientWidth
    const scrollers = [document.documentElement, ...Array.from(document.querySelectorAll('#main-content, #main-content > div'))]
      .filter((el) => el.scrollWidth > el.clientWidth + 1)
      .map((el) => `${el.id || el.tagName}: ${el.scrollWidth} > ${el.clientWidth}`)
    const header = document.querySelector('[data-testid="page-header"]')
    const actions = document.querySelector('[data-testid="page-header-actions"]')
    return {
      scrollers,
      headerRight: header ? Math.round(header.getBoundingClientRect().right) : null,
      actionsRight: actions ? Math.round(actions.getBoundingClientRect().right) : null,
      vw,
    }
  })
}

const PAGES: [string, string][] = [
  ['Dashboard', '/'],
  ['Frameworks', '/vaktcomply/frameworks'],
  ['Nachweise', '/vaktcomply/evidence'],
  ['SoA', '/vaktcomply/soa'],
  ['Risiken', '/vaktcomply/risks'],
  ['Richtlinien', '/vaktcomply/policies'],
  ['Inventar', '/vaktcomply/assets'],
  ['Einstellungen', '/settings'],
]

test.describe('Handy 390 px (UX-39)', () => {
  test.beforeEach(async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 })
    await mockApi(page)
  })

  for (const [name, path] of PAGES) {
    test(`${name}: keine horizontale Seitenscroll-Leiste, Kopf bleibt im Bild`, async ({ page }) => {
      await page.goto(path)
      await page.waitForTimeout(800)
      const o = await overflow(page)
      expect(o.scrollers, `${name}: zu breite Scroll-Container`).toEqual([])
      if (o.actionsRight !== null) expect(o.actionsRight).toBeLessThanOrEqual(o.vw)
      if (o.headerRight !== null) expect(o.headerRight).toBeLessThanOrEqual(o.vw)
    })
  }

  test('Seitenleiste: „Mehr" öffnet die volle Leiste, auch wenn sie am Desktop eingeklappt ist; Escape schließt', async ({ page }) => {
    await page.addInitScript(() => { localStorage.setItem('vakt_sidebar_collapsed', 'true') })
    await page.goto('/vaktcomply/risks')
    const aside = page.getByTestId('app-sidebar')
    await expect(aside).toHaveAttribute('data-open', 'false')
    await page.getByRole('navigation', { name: /mobil|mobile/i }).getByRole('button').click()
    await expect(aside).toHaveAttribute('data-open', 'true')
    await expect.poll(async () => (await aside.boundingBox())?.width ?? 0).toBeGreaterThan(150)
    await expect(aside.getByRole('button', { name: /Sidebar (ein|aus)klappen/ })).toBeHidden()
    await page.keyboard.press('Escape')
    await expect(aside).toHaveAttribute('data-open', 'false')
  })
})
