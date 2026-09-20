import fs from 'node:fs'
import path from 'node:path'
import { test, expect, type Page } from '@playwright/test'
import { KW, compose, openAs, readProKey, signLicense, waitForHealthy, closeContexts } from './lib/kw'

/**
 * Kernweg 1 — Kauf & Lizenz (docs/launch-gate.md)
 * Erfüllt, wenn: Bestellung → Rechnung (Lexware) → Lizenzschlüssel kommt an →
 * Pro-Funktionen schalten frei → Verlängerung läuft.
 *
 * GRENZE: Bestellung, Rechnung (Lexware) und Versand der Schlüssel-Mail laufen im
 * Billing-Dienst auf Norvik-Infrastruktur und gegen die echte Lexware-API — beides
 * ist hier tabu. Automatisiert ist alles AB „Schlüssel liegt vor":
 *   - der Schlüssel kommt aus dem offiziellen Werkzeug internal/license/generator,
 *     signiert mit einem pro Lauf erzeugten Test-Schlüsselpaar (scripts/kernwege/
 *     api-testkey.Dockerfile setzt den öffentlichen Teil ins API-Binary);
 *   - die Verlängerung holt die Instanz selbst — gegen einen lokalen Stub
 *     (scripts/kernwege/licstub.Caddyfile) statt api.norvikops.de.
 * Der Rest steht als manueller Schritt in docs/launch-gate.md.
 */

const PRO_FEATURES = [
  'eu_ai_act', 'cra', 'ai_advisor', 'audit_pdf', 'sso', 'api_access', 'vaktaware_advanced',
  'vaktscan_advanced', 'vaktvault_advanced', 'vaktprivacy_advanced', 'bsi_grundschutz',
  'granular_permissions', 'supplier_portal', 'nis2_reporting', 'saml_auth', 'agent_write_tools',
  'scim_provisioning', 'siem_export', 'multi_framework',
]
const DAY = 86_400

function deDate(unixSeconds: number): string {
  return new Date(unixSeconds * 1000).toLocaleDateString('de-DE', {
    day: '2-digit', month: '2-digit', year: 'numeric', timeZone: 'Europe/Berlin',
  })
}

async function licenseCard(page: Page) {
  await page.goto('/settings')
  // Die Lizenz-Karte liegt im Reiter „Plattform" der Einstellungen.
  const card = page.getByRole('tabpanel', { name: 'Plattform' })
  await expect(card.getByRole('heading', { name: 'Lizenz', level: 2 })).toBeVisible()
  return card
}

async function activate(page: Page, key: string): Promise<void> {
  const card = await licenseCard(page)
  await card.getByPlaceholder('Ihr Lizenzschlüssel').fill(key)
  await card.getByRole('button', { name: 'Aktivieren' }).click()
  await expect(card.getByText('Key aktiviert — Lizenz aktualisiert.')).toBeVisible()
}

test.describe('KW1 Kauf & Lizenz', () => {
  test.afterEach(closeContexts)

  test.skip(!fs.existsSync(path.join(KW.dir, 'license-pro.key')),
    'kein Test-Pro-Schlüssel — internal/license/generator fehlt im Baum (Public Mirror)')

  test('KW1 Community-Instanz: Pro-Funktion ist gesperrt und nennt den Weg zur Lizenz', async ({ browser }) => {
    const page = await openAs(browser, 'admin')
    const card = await licenseCard(page)
    await expect(card.getByText('Community', { exact: true })).toBeVisible()

    // Sperre gegen Rückkehr von R1-B1-N2 (GATE-5, gefixt): der verkaufte
    // Multi-Framework-Check ist aus der Navigation erreichbar, nicht nur per URL.
    await page.goto('/')
    const nav = page.getByRole('navigation', { name: 'Hauptnavigation' })
    await nav.getByRole('link', { name: 'Vakt Comply' }).click()
    // Die Gruppe „Frameworks" klappt ein Mensch auf, wenn sie zu ist.
    const group = nav.getByRole('button', { name: /^Frameworks/ })
    if ((await group.getAttribute('aria-expanded')) !== 'true') await group.click()
    await expect(nav.getByRole('link', { name: /Multi-Framework/ })).toBeVisible()
    await nav.getByRole('link', { name: /Multi-Framework/ }).click()
    await expect(page.getByRole('heading', { name: 'Pro-Feature' })).toBeVisible()
    await expect(page.getByRole('link', { name: 'Lizenz aktivieren' })).toBeVisible()
  })

  test('KW1 Pro-Schlüssel (internal/license/generator) aktivieren → Pro-Funktionen schalten frei', async ({ browser }) => {
    const page = await openAs(browser, 'admin')
    await activate(page, readProKey())
    const card = await licenseCard(page)
    await expect(card.getByText('Pro', { exact: true })).toBeVisible()
    await expect(card.getByText('Kernwege GmbH')).toBeVisible()
    await expect(card.getByText(/^Gültig bis /)).toBeVisible()

    // Dieselbe Seite, die eben gesperrt war, arbeitet jetzt.
    await page.goto('/nis2-check/multi')
    await expect(page.getByRole('heading', { name: 'Multi-Framework-Assessment' })).toBeVisible()
    await expect(page.getByRole('heading', { name: 'Pro-Feature' })).toHaveCount(0)
    await expect(page.getByRole('button').first()).toBeVisible()
  })

  test('KW1 Verlängerung: Instanz holt den nächsten Schlüssel selbst (Stub statt api.norvikops.de)', async ({ browser }) => {
    const now = Math.floor(Date.now() / 1000)
    // Ein Schlüssel im letzten Viertel seiner Laufzeit (30 Tage alt, noch 2 Tage) —
    // genau der Zustand, in dem autorefresh.go die Verlängerung anstößt.
    const expiring = signLicense({
      tier: 'pro', features: PRO_FEATURES, org: 'Kernwege GmbH', iat: now - 30 * DAY, exp: now + 2 * DAY, rt: KW.renewalToken,
    })
    // Was der Verkaufsweg nach bezahlter Rechnung ausliefern würde: ein Jahr mehr.
    const renewedExp = now + 365 * DAY
    const renewed = signLicense({
      tier: 'pro', features: PRO_FEATURES, org: 'Kernwege GmbH', iat: now, exp: renewedExp, rt: KW.renewalToken,
    })
    fs.writeFileSync(path.join(KW.dir, 'licstub', 'renewal.json'), JSON.stringify({ key: renewed }))

    const page = await openAs(browser, 'admin')
    await activate(page, expiring)
    let card = await licenseCard(page)
    await expect(card.getByText(`Gültig bis ${deDate(now + 2 * DAY)}`)).toBeVisible()

    // Die Prüfung läuft beim Start und dann alle 24 h — der Neustart ist der
    // „nächste Morgen". Danach muss die Instanz den neuen Schlüssel tragen, ohne
    // dass jemand etwas eingegeben hat.
    compose(['restart', 'api'])
    await waitForHealthy(page)
    await expect(async () => {
      card = await licenseCard(page)
      await expect(card.getByText(`Gültig bis ${deDate(renewedExp)}`)).toBeVisible({ timeout: 2000 })
    }).toPass({ timeout: 60_000 })
    await expect(card.getByText(/automatische Verlängerung greift nicht/)).toHaveCount(0)

    // Der Stub hat die Anfrage nur mit dem Token aus dem Schlüssel beantwortet
    // (licstub.Caddyfile) — die Instanz schickt also das Token, und nur das.
    const log = compose(['logs', '--no-color', 'licstub'])
    expect(log).toMatch(/"uri":"\/api\/v1\/billing\/license"[^\n]*"status":200/)
  })
})
