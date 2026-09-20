import fs from 'node:fs'
import { test, expect, type Page } from '@playwright/test'
import { openAs, openControlFromTable, openIsoDetail, saveDialogUntilClosed, closeContexts } from './lib/kw'
import { xlsxRows } from './lib/zip'

/**
 * Kernweg 6 — Anwendbarkeit & Kontrollen (docs/launch-gate.md)
 * Erfüllt, wenn: Anwendbarkeitserklärung (SoA) pflegen, Kontrollstatus setzen,
 * Fortschritt stimmt überall mit derselben Zahl.
 *
 * Baut auf Kernweg 5 auf (ISO 27001 ist aktiv).
 */

const UUID = /[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/i


/** Öffnet den Bearbeiten-Dialog einer SoA-Zeile; klappt die Gruppe (A.5 … A.8) auf, wenn nötig. */
async function editSoARow(page: Page, ref: string, group: string) {
  await expect(page.getByRole('row', { name: /^A\.\d+\.\d+ / }).first()).toBeVisible()
  const row = page.getByRole('row', { name: new RegExp(`^${ref.replace(/\./g, '\\.')} `) })
  if (!(await row.isVisible())) await page.getByRole('button', { name: new RegExp(`^${group.replace(/\./g, '\\.')} `) }).click()
  await row.getByRole('button').click()
  const dlg = page.getByRole('dialog', { name: new RegExp(`^${ref.replace(/\./g, '\\.')} —`) })
  await expect(dlg).toBeVisible()
  return dlg
}

/** Die Readiness-Zahl im Kopf der Framework-Detailseite („1% Readiness Score"). */
async function detailReadiness(page: Page): Promise<number> {
  const txt = await page.getByText(/\d+%\s*Readiness Score/).first().innerText()
  return Number(/(\d+)%/.exec(txt)![1])
}

test.describe('KW6 Anwendbarkeit & Kontrollen', () => {
  test.afterEach(closeContexts)

  test('KW6 Kontrollstatus setzen: A.5.1 → Umgesetzt, Kopfzahlen ziehen mit', async ({ browser }) => {
    const page = await openAs(browser, 'admin')
    await openIsoDetail(page)
    await expect(page.getByText(/0\s*Umgesetzt/).first()).toBeVisible()
    const row = page.getByRole('row', { name: /A\.5\.1 Richtlinien zur Informationssicherheit/ })
    await row.getByRole('combobox').click()
    await page.getByRole('option', { name: 'Umgesetzt', exact: true }).click()
    await expect(row.getByRole('combobox')).toHaveText('Umgesetzt')
    // Kopf: 1 von 93 umgesetzt → 1 %.
    await expect(page.getByText(/1\s*Umgesetzt/).first()).toBeVisible()
    await expect(page.getByText(/1%\s*Readiness Score/)).toBeVisible()
    await page.reload()
    await expect(page.getByRole('row', { name: /A\.5\.1 Richtlinien/ }).getByRole('combobox')).toHaveText('Umgesetzt')
  })

  // FrameworkDetailPage lädt nur die erste Seite (useFrameworkControls: limit=25) und
  // hat keine Blätterfunktion. Der Kopf sagt „93 Gesamt", der Reiter „Controls (25)",
  // und die Kontrollen 26–93 (u. a. alle A.8-Technikmaßnahmen) sind dort nicht
  // erreichbar. Der Reiter rechnet seinen Prozentwert auf 25 statt 93 (4 % statt 1 %).
  test('KW6 R1-KW-5 Framework-Detail zeigt alle 93 Kontrollen, Reiter und Kopf rechnen gleich', async ({ browser }) => {
    const page = await openAs(browser, 'admin')
    await openIsoDetail(page)
    await expect(page.getByText(/93\s*Gesamt/)).toBeVisible()
    test.fail(true, 'R1-KW-5: Framework-Detail zeigt nur 25 von 93 Kontrollen (limit=25, kein Blättern), Reiter-Prozent auf 25 gerechnet')
    await expect(page.getByRole('tab', { name: 'Controls (93)' })).toBeVisible({ timeout: 5000 })
  })

  // Nach Kernweg-6-Test 1 ist A.5.1 umgesetzt. Detail und Dashboard zeigen 1 %, die
  // Karte in der Framework-Übersicht bleibt bei 0 %.
  test('KW6 R1-KW-6 Fortschritt: Übersicht, Detail und Dashboard zeigen dieselbe Zahl', async ({ browser }) => {
    const page = await openAs(browser, 'admin')
    await openIsoDetail(page)
    const detail = await detailReadiness(page)
    expect(detail, 'Vorbedingung: A.5.1 ist umgesetzt (1 von 93)').toBe(1)

    await page.goto('/')
    const fw = page.getByText(/ISO27001\s*1 \/ 93/).first()
    await expect(fw, 'Dashboard „Framework-Fortschritt" zeigt 1 / 93').toBeVisible()
    await expect(page.getByText(`1 / 93 · ${detail}%`)).toBeVisible()

    test.fail(true, 'R1-KW-6: Framework-Übersicht zeigt 0 %, Detail und Dashboard 1 %')
    await page.goto('/vaktcomply/frameworks')
    const card = page.getByRole('heading', { name: 'ISO27001', level: 3 }).locator('xpath=ancestor::div[.//button][1]')
    await expect(card).toContainText(`${detail}%`, { timeout: 5000 })
  })

  // Die Kontroll-Detailseite liest einen anderen Status als die Framework-Liste: nach
  // „Umgesetzt" in der Liste steht dort „Offen" — die Änderungshistorie direkt darunter
  // sagt „→ implemented".
  test('KW6 R1-KW-10 Kontroll-Detail zeigt denselben Status wie die Framework-Liste', async ({ browser }) => {
    const page = await openAs(browser, 'admin')
    await openIsoDetail(page)
    await expect(page.getByRole('row', { name: /A\.5\.1 Richtlinien/ }).getByRole('combobox')).toHaveText('Umgesetzt')
    await openControlFromTable(page, 'Richtlinien zur Informationssicherheit')
    await expect(page.getByText('→ implemented')).toBeVisible()
    test.fail(true, 'R1-KW-10: Kontroll-Detail zeigt „Offen", Framework-Liste „Umgesetzt" (manual_status vs. status)')
    await expect(page.getByRole('main').getByRole('combobox').first()).toHaveText('Umgesetzt', { timeout: 5000 })
  })

  test('KW6 SoA initialisieren, Ausschluss begründen, Status pflegen, Version genehmigen', async ({ browser }) => {
    test.setTimeout(240_000)
    const page = await openAs(browser, 'admin')
    await page.goto('/vaktcomply/soa')
    await page.getByRole('button', { name: 'SoA initialisieren' }).click()
    await expect(page.getByText(/Entwurf\s*Version 1/)).toBeVisible()
    await expect(page.getByText(/93\s*Kontrollen gesamt/)).toBeVisible()

    // Ausschluss mit Begründung: A.7.4 (physische Sicherheitsüberwachung) — reines Homeoffice-KMU.
    let dlg = await editSoARow(page, 'A.7.4', 'A.7')
    await dlg.getByRole('combobox').first().click()
    await page.getByRole('option', { name: 'Nein — nicht anwendbar' }).click()
    await dlg.getByPlaceholder(/Warum ist diese Maßnahme nicht anwendbar/).fill('Keine eigenen Geschäftsräume — reines Homeoffice.')
    await saveDialogUntilClosed(dlg)

    // Umsetzungsstatus: A.5.1 umgesetzt, mit Nachweisnotiz.
    dlg = await editSoARow(page, 'A.5.1', 'A.5')
    await dlg.getByPlaceholder('Warum ist diese Maßnahme anwendbar?').fill('Pflicht aus ISO 27001 Kap. 5.2.')
    await dlg.getByRole('combobox').nth(1).click()
    await page.getByRole('option', { name: 'Umgesetzt', exact: true }).click()
    await dlg.getByPlaceholder('Verweis auf Dokumentation, Policy, o.ä.').fill('Informationssicherheitsleitlinie v1.0')
    await saveDialogUntilClosed(dlg)

    await expect(page.getByText(/92\s*Anwendbar/)).toBeVisible()
    await expect(page.getByText(/1\s*Ausgeschlossen/)).toBeVisible()
    await expect(page.getByText(/0\s*Ohne Begründung/)).toBeVisible()
    if (!(await page.getByRole('row', { name: /^A\.5\.1 / }).isVisible())) await page.getByRole('button', { name: /^A\.5 / }).click()
    await expect(page.getByRole('row', { name: /^A\.5\.1 / })).toContainText('Umgesetzt')

    await page.getByRole('button', { name: 'Version genehmigen' }).click()
    const confirm = page.getByRole('alertdialog').or(page.getByRole('dialog')).filter({ hasText: /genehmig/i })
    if (await confirm.isVisible().catch(() => false)) {
      await confirm.getByRole('button', { name: /genehmig/i }).last().click()
    }
    await expect(page.getByText(/Version 1 genehmigt|genehmigt am|Genehmigt/).first()).toBeVisible()
  })

  // Sperre gegen Rückkehr von R1-20-06 / R1-20-07 (GATE-6, gefixt): die SoA-XLSX nennt
  // keine UUID als Verantwortlichen und spricht Deutsch.
  test('KW6 SoA-Export (XLSX): deutsch, keine UUID, Ausschluss und Status wie in der Oberfläche', async ({ browser }) => {
    const page = await openAs(browser, 'admin')
    await page.goto('/vaktcomply/soa')
    await expect(page.getByText(/93\s*Kontrollen gesamt/)).toBeVisible()
    const [dl] = await Promise.all([page.waitForEvent('download'), page.getByRole('button', { name: 'XLSX' }).click()])
    const rows = xlsxRows(fs.readFileSync((await dl.path())!))
    const flat = rows.flat().join(' | ')
    expect(flat).not.toMatch(UUID)
    expect(flat).not.toMatch(/\bimplemented\b|\bnot_started\b|\bApplicable\b/)
    const a51 = rows.find((r) => r.some((c) => c.trim() === 'A.5.1'))
    const a74 = rows.find((r) => r.some((c) => c.trim() === 'A.7.4'))
    // Die Oberfläche sagt „Umgesetzt", der Export „Implementiert" (SoAStatusLabel wie im PDF) —
    // beides deutsch; der Wortunterschied steht als Beobachtung im PR, nicht als Befund.
    expect(a51?.join(' | ')).toMatch(/Umgesetzt|Implementiert/)
    expect(a74?.join(' | ')).toContain('Homeoffice')
    // 93 Datenzeilen + Kopf (ggf. Titelzeilen davor).
    expect(rows.filter((r) => /^A\.\d+\.\d+$/.test(r[0]?.trim() ?? '') || r.some((c) => /^A\.\d+\.\d+$/.test(c.trim()))).length).toBe(93)
  })
})
