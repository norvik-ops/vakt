import { test, expect, type Page } from '@playwright/test'
import { openAs, closeContexts } from './lib/kw'

/**
 * Kernweg 5 — Framework wählen (docs/launch-gate.md)
 * Erfüllt, wenn: NIS2- oder ISO-27001-Framework aktivieren, Kontrollen erscheinen.
 *
 * Beide Frameworks werden aktiviert — ISO 27001 trägt die SoA (Kernweg 6), NIS2 ist
 * der Grund, warum die meisten KMU-Kunden kommen.
 */

async function catalogCard(page: Page, subtitle: string) {
  // Katalogkarte = der kleinste Block, der den Untertitel UND einen Knopf trägt.
  return page.getByText(subtitle, { exact: true })
    .locator('xpath=ancestor::div[.//button][1]')
}

/**
 * Aktivieren im Katalog. Danach öffnet sich einmalig der Einrichtungs-Assistent
 * („Willkommen bei …", FrameworkSetupWizard) — `walkWizard` geht ihn bis zum Ende durch
 * und landet in der Framework-Detailansicht, sonst wird er übersprungen.
 */
async function activateFromCatalog(page: Page, subtitle: string, frameworkName: string, walkWizard: boolean): Promise<void> {
  await page.goto('/vaktcomply/frameworks')
  await expect(page.getByRole('heading', { name: 'Compliance-Frameworks', level: 1 })).toBeVisible()
  const card = await catalogCard(page, subtitle)
  await card.getByRole('button', { name: 'Aktivieren', exact: true }).click()
  const wizard = page.getByRole('dialog', { name: `Willkommen bei ${frameworkName}` })
  await expect(wizard).toBeVisible()
  if (walkWizard) {
    await wizard.getByRole('button', { name: 'Erste Controls anzeigen' }).click()
    await page.getByRole('dialog').getByRole('button', { name: 'Verstanden, weiter' }).click()
    await page.getByRole('dialog').getByRole('button', { name: 'Controls jetzt bewerten' }).click()
    await expect(page).toHaveURL(/\/vaktcomply\/frameworks\/[0-9a-f-]{36}$/)
    await page.goto('/vaktcomply/frameworks')
  } else {
    await wizard.getByRole('button', { name: 'Später einrichten' }).click()
    await expect(wizard).toBeHidden()
  }
  await expect(card.getByText('Aktiviert', { exact: true })).toBeVisible()
  await expect(card.getByRole('button', { name: 'Anzeigen', exact: true })).toBeVisible()
}

test.describe('KW5 Framework wählen', () => {
  test.afterEach(closeContexts)

  test('KW5 ISO 27001 aktivieren → 93 Annex-A-Kontrollen erscheinen', async ({ browser }) => {
    const page = await openAs(browser, 'admin')
    await activateFromCatalog(page, 'ISO/IEC 27001:2022', 'ISO27001', false)
    await expect(page.getByRole('heading', { name: 'ISO27001', level: 3 })).toBeVisible()

    await (await catalogCard(page, 'ISO/IEC 27001:2022')).getByRole('button', { name: 'Anzeigen', exact: true }).click()
    await expect(page.getByRole('heading', { name: 'ISO27001 Details', level: 1 })).toBeVisible()
    await expect(page.getByText(/93\s*Gesamt/)).toBeVisible()
    await expect(page.getByRole('row', { name: /A\.5\.1 Richtlinien zur Informationssicherheit/ })).toBeVisible()
  })

  test('KW5 NIS2 aktivieren → Maßnahmen erscheinen', async ({ browser }) => {
    const page = await openAs(browser, 'admin')
    await activateFromCatalog(page, 'NIS-2-Richtlinie (EU) 2022/2555', 'NIS2', true)
    await expect(page.getByRole('heading', { name: 'NIS2', level: 3 })).toBeVisible()

    await (await catalogCard(page, 'NIS-2-Richtlinie (EU) 2022/2555')).getByRole('button', { name: 'Anzeigen', exact: true }).click()
    await expect(page.getByRole('heading', { name: /NIS2.*Details|NIS2/, level: 1 })).toBeVisible()
    // Mindestens eine Maßnahme mit Status-Auswahl ist sichtbar.
    await expect(page.getByRole('combobox').first()).toBeVisible()
    await expect(page.getByText(/[1-9]\d*\s*Gesamt/)).toBeVisible()
  })

  test('KW5 Dashboard zählt beide aktiven Frameworks', async ({ browser }) => {
    const page = await openAs(browser, 'admin')
    await page.goto('/')
    await expect(page.getByRole('link', { name: /Frameworks aktiv\s*2/ })).toBeVisible()
  })
})
