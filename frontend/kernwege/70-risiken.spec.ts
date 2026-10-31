import { test, expect, type Page } from '@playwright/test'
import { apiWrite, loadState, openAs, patchState, closeContexts } from './lib/kw'

/**
 * Kernweg 8 — Risiken (docs/launch-gate.md)
 * Erfüllt, wenn: Risiko anlegen, bewerten, Maßnahme zuordnen, Restrisiko sichtbar.
 */

const RISK = 'Ransomware verschlüsselt den Fileserver'
const RISK_10 = 'Ausfall der Produktionsdatenbank'

/** Bildet Stufennamen auf eine Grundform ab: „Hohes Risiko" und „Hoch" sind dieselbe Stufe. */
function stufe(text: string): string {
  const t = text.toLowerCase()
  if (/krit/.test(t)) return 'kritisch'
  if (/sehr\s*hoch/.test(t)) return 'sehr hoch'
  if (/\bho(ch|h)/.test(t)) return 'hoch'
  if (/mitt/.test(t)) return 'mittel'
  if (/niedrig|gering/.test(t)) return 'niedrig'
  return `unbekannt: ${text}`
}

async function riskList(page: Page): Promise<void> {
  await page.goto('/vaktcomply/risks')
  await expect(page.getByRole('heading', { name: 'Risikoregister', level: 1 })).toBeVisible()
}

test.describe('KW8 Risiken', () => {
  test.afterEach(closeContexts)

  // UserPicker (Verantwortlicher) rendert je Teammitglied <SelectItem value={member.name}>.
  // Ein per „Direkt anlegen" erzeugter Nutzer hat keinen Anzeigenamen → value="" →
  // Radix wirft, die Seite stürzt ab. Nach Kernweg 3 gibt es genau so einen Nutzer —
  // wie bei jedem Kunden, der sein Team direkt anlegt.
  test('KW8 R1-KW-9 Risiko über „Risiko erfassen" anlegen und bewerten', async ({ browser }) => {
    const page = await openAs(browser, 'admin')
    await page.goto('/settings/team')
    await expect(page.getByRole('row', { name: /viewer@kernwege\.test/ }), 'Vorbedingung: Teammitglied ohne Anzeigenamen (aus Kernweg 3)').toBeVisible()
    await riskList(page)
    await page.getByRole('button', { name: 'Risiko erfassen' }).first().click()

    const dlg = page.getByRole('dialog', { name: 'Risiko erfassen' })
    await expect(page.getByRole('heading', { name: 'Etwas ist schiefgelaufen' })).toHaveCount(0, { timeout: 3000 })
    await dlg.getByLabel('Bezeichnung *').fill(RISK)
    await dlg.getByLabel('Kategorie').fill('Cyber')
    await dlg.getByLabel('Wahrscheinlichkeit (1–5) *').fill('4')
    await dlg.getByLabel('Auswirkung (1–5) *').fill('5')
    await expect(dlg.getByText(/Voraussichtlicher Risiko-Score:\s*20/)).toBeVisible()
    await dlg.getByRole('button', { name: 'Risiko erfassen' }).click()
    await expect(page.getByText(RISK)).toBeVisible({ timeout: 5000 })
    patchState({ riskTitle: RISK })
  })

  test('KW8 Risiko bewerten, Maßnahme (Control) zuordnen, Restrisiko ist sichtbar', async ({ browser }) => {
    const page = await openAs(browser, 'admin')
    await riskList(page)
    if (!loadState().riskTitle) {
      // Solange R1-KW-9 offen ist, legt der Test das Risiko mit demselben Aufruf an,
      // den das Formular machen würde — damit Bewertung, Maßnahme und Restrisiko
      // geprüft bleiben. Grenze: das Anlegen per Formular ist R1-KW-9.
      const r = await apiWrite(page.context(), 'POST', '/api/v1/vaktcomply/risks', {
        title: RISK, category: 'Cyber', likelihood: 4, impact: 5, treatment: 'mitigate',
      })
      expect(r.status(), await r.text()).toBe(201)
      patchState({ riskTitle: RISK })
      await riskList(page)
    }
    // Bewertung sichtbar: Score 20 in der Gruppe „Hohes Risiko".
    await expect(page.getByText(RISK)).toBeVisible()
    await expect(page.getByRole('heading', { name: /Hohes Risiko/ })).toBeVisible()
    await page.getByText(RISK).click()
    await expect(page.getByRole('heading', { name: RISK, level: 1 })).toBeVisible()
    await expect(page.getByText(/Risiko-Score\s*20\s*\/\s*25/)).toBeVisible()

    // Maßnahme: Behandlungsplan + Control A.5.15 (Zugangskontrolle) verknüpfen.
    // (Die fachlich passendere A.8.13 „Datensicherung" bietet der Picker nicht an — R1-KW-5.)
    await page.getByPlaceholder(/Konkrete Maßnahmen, Verantwortlichkeiten und Zeitplan/).fill('Offline-Backups täglich, Restore-Test quartalsweise, EDR auf allen Clients.')
    await page.getByRole('button', { name: 'Control verknüpfen' }).click()
    const link = page.getByRole('dialog', { name: 'Control verknüpfen' })
    await link.getByRole('combobox').selectOption({ label: 'ISO27001' })
    await link.getByPlaceholder('Controls durchsuchen …').fill('A.5.15')
    await link.getByRole('button', { name: /^A\.5\.15 / }).click()
    await expect(page.getByText(/A\.5\.15/).first()).toBeVisible()

    // Restrisiko nach Behandlung: 2 × 2 (native Schieberegler — Tastatur wie ein Mensch).
    for (const i of [0, 1]) {
      const slider = page.getByRole('slider').nth(i)
      await slider.focus()
      await page.keyboard.press('Home')
      await page.keyboard.press('ArrowRight')
      await expect(slider).toHaveValue('2')
    }
    // Kein Erfolgshinweis nach dem Speichern — der Test wartet auf die Antwort des Servers
    // und prüft danach das Ergebnis nach einem Neuladen.
    const [saved] = await Promise.all([
      page.waitForResponse((r) => /\/risks\/[0-9a-f-]{36}\/treatment$/.test(r.url()) && r.request().method() === 'PATCH'),
      page.getByRole('button', { name: 'Behandlung speichern' }).click(),
    ])
    expect(saved.status(), await saved.text()).toBeLessThan(300)

    await page.reload()
    await expect(page.getByRole('slider').nth(0)).toHaveValue('2')
    await expect(page.getByRole('slider').nth(1)).toHaveValue('2')
    await expect(page.getByText(/A\.5\.15/).first()).toBeVisible()
    await expect(page.getByPlaceholder(/Konkrete Maßnahmen/)).toHaveValue(/Offline-Backups/)
  })

  // Derselbe Ladefehler wie im Framework-Detail (R1-KW-5): der Control-Picker am Risiko
  // bekommt nur die ersten 25 von 93 ISO-Kontrollen. Alle A.6–A.8-Maßnahmen — auch
  // A.8.13 „Datensicherung", die zu einem Ransomware-Risiko gehört — sind nicht wählbar.
  test('KW8 R1-KW-5 Control-Picker am Risiko bietet alle 93 ISO-Kontrollen an (A.8.13 Datensicherung)', async ({ browser }) => {
    const page = await openAs(browser, 'admin')
    await riskList(page)
    await page.getByText(RISK).click()
    await page.getByRole('button', { name: 'Control verknüpfen' }).click()
    const link = page.getByRole('dialog', { name: 'Control verknüpfen' })
    await link.getByRole('combobox').selectOption({ label: 'ISO27001' })
    await expect(link.getByRole('button', { name: /^A\.5\.1 / })).toBeVisible()
    await link.getByPlaceholder('Controls durchsuchen …').fill('A.8.13')
    await expect(link.getByRole('button', { name: /^A\.8\.13 / })).toBeVisible({ timeout: 5000 })
  })

  // Die Risikostufen sind in jeder Ansicht anders geschnitten: Legende und Zellfarbe der
  // Matrix sagen „Hoch (10–14)", die Liste gruppiert ab 9 als „Mittleres Risiko", die
  // Score-Badges schneiden bei 15/9/4, das Restrisiko bei 12/6, die SecVitals-Übersicht bei 12.
  // Dasselbe Risiko (Score 10) heißt damit je nach Ansicht „hoch" oder „mittel".
  // Die Prüfung verlangt kein bestimmtes Schema: Sie vergleicht nur die Stufe, die die
  // Legende für Score 10 nennt, mit der Stufe der Listengruppe, in der die Karte steht.
  test('KW8 R1-KW-17 Risiko mit Score 10 steht in der Listengruppe, deren Stufe die Legende nennt', async ({ browser }) => {
    const page = await openAs(browser, 'admin')
    await riskList(page)
    const r = await apiWrite(page.context(), 'POST', '/api/v1/vaktcomply/risks', {
      title: RISK_10, category: 'Betrieb', likelihood: 2, impact: 5, treatment: 'mitigate',
    })
    expect(r.status(), await r.text()).toBe(201)
    await riskList(page)
    const card = page.getByText(RISK_10)
    await expect(card, 'Vorbedingung: das neue Risiko steht in der Liste').toBeVisible()

    // Legende der Matrix lesen: Einträge der Form „Stufe (von–bis)".
    const matrix = page.getByRole('heading', { name: /Risikomatrix/ }).locator('xpath=..')
    const legendText = await matrix.innerText()
    const ranges = [...legendText.matchAll(/(\p{L}[\p{L} ]*?)\s*\((\d+)\s*[–-]\s*(\d+)\)/gu)]
      .map((m) => ({ label: m[1]!.trim(), from: Number(m[2]), to: Number(m[3]) }))
    const legendFor10 = ranges.find((x) => x.from <= 10 && 10 <= x.to)
    expect(legendFor10, `Vorbedingung: Legende nennt eine Stufe für Score 10 (gelesen: ${JSON.stringify(ranges)})`).toBeTruthy()

    // Überschrift der Gruppe, in der die Karte steht.
    const groupHeading = card.locator('xpath=ancestor::*[h2][1]/h2')
    await expect(groupHeading, 'Vorbedingung: die Karte steht unter einer Gruppenüberschrift').toHaveCount(1)
    const groupText = await groupHeading.innerText()

    // Ab hier der bekannte Befund. test.fail() steht bewusst NACH den Vorbedingungen.
    test.fail(true, 'R1-KW-17: Legende nennt Score 10 „Hoch (10–14)", die Liste gruppiert ihn unter „Mittleres Risiko (Score 9–14)"')
    expect(stufe(groupText), `Gruppe „${groupText}" vs. Legende „${legendFor10!.label}"`).toBe(stufe(legendFor10!.label))
  })
})
