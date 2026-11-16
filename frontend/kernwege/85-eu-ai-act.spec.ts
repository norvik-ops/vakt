import fs from 'node:fs'
import path from 'node:path'
import { test, expect, type Download, type Locator, type Page } from '@playwright/test'
import { KW, openAs, closeContexts } from './lib/kw'
import { pdfTexts } from './lib/pdf'

/**
 * Kernweg 10 — EU AI Act (docs/launch-gate.md, Entscheid Stefan 2026-11-16)
 * Erfüllt, wenn: Pro aktiv → EU-AI-Act-Framework aktivieren, Kontrollen erscheinen →
 * KI-System anlegen → Assistent bestimmt die Risikoklasse, sie steht in Liste und
 * Dashboard → Technikdoku (Art. 11) speichern → Dossier-PDF und Bericht-PDF enthalten
 * das System mit derselben Klasse.
 *
 * Pro-Schlüssel wie Kernweg 1: 30-kauf-lizenz aktiviert ihn (Merkmal `eu_ai_act`), alle
 * Wege danach laufen auf der lizenzierten Instanz. Ohne Generator (Public Mirror) gibt
 * es keinen Schlüssel — dann wird der Weg übersprungen, nicht still grün.
 *
 * Bewusst NICHT geprüft: konkrete Fristdaten (Dashboard-KPI „Frist Hochrisiko",
 * Kopfzeile des Berichts). Die ändern sich mit dem Digital-Omnibus; ein Kernweg prüft,
 * dass der Weg funktioniert, nicht welches Datum gerade gilt.
 *
 * Die PDFs werden heruntergeladen und ihr Text gelesen (lib/pdf.ts). Lesbarkeit prüft
 * kein Test — das ist der manuelle Schritt in docs/launch-gate.md.
 */

const SYSTEM = 'Bewerber-Vorauswahl KW10'
const USE_CASE = 'Sortiert eingehende Bewerbungen vor und schlägt der Personalabteilung eine Shortlist vor (Personalauswahl).'
const DOC_DESCRIPTION = 'KW10-Dossier: Ranking-Modell auf Basis von Lebenslauf-Merkmalen, betrieben im eigenen Rechenzentrum.'
const DOC_PURPOSE = 'Vorauswahl von Bewerbungen; die Entscheidung trifft immer eine Person aus der Personalabteilung.'
const DOC_OVERSIGHT = 'Jede Shortlist wird von zwei Personen geprüft, bevor eine Absage verschickt wird.'

/**
 * Bildet die Schreibweisen einer Risikoklasse auf den Schlüssel ab. Die Oberfläche sagt
 * „Hohes Risiko", der Bericht nennt im Inventar den Schlüssel „high" — dieselbe Klasse.
 */
function klasse(text: string): string {
  const t = text.toLowerCase()
  if (/inakzeptab|verboten|unacceptable/.test(t)) return 'unacceptable'
  if (/hohes risiko|hochrisiko|\bhoch\b|\bhigh\b/.test(t)) return 'high'
  if (/begrenzt|limited/.test(t)) return 'limited'
  if (/minimal/.test(t)) return 'minimal'
  return `unbekannt: ${text}`
}

async function inventory(page: Page): Promise<void> {
  await page.goto('/vaktcomply/ai-systems')
  await expect(page.getByRole('heading', { name: 'KI-System-Inventar', level: 1 })).toBeVisible()
}

/**
 * Die Karte des Systems im Inventar. Der Name steht in der Kopfzeile der Karte (mit den
 * Knöpfen), die Badges eine Ebene darüber im Karteninhalt — deshalb der zweite Vorfahre,
 * der den Klassifizieren-Knopf trägt.
 */
function systemCard(page: Page): Locator {
  return page.getByText(SYSTEM, { exact: true })
    .locator('xpath=ancestor::div[.//button[@data-testid="classify-ai-system-btn"]][2]')
}

async function readDownload(d: Download): Promise<Buffer> {
  const p = await d.path()
  expect(p, 'Download wurde gespeichert').toBeTruthy()
  return fs.readFileSync(p)
}

test.describe('KW10 EU AI Act', () => {
  test.afterEach(closeContexts)

  test.skip(!fs.existsSync(path.join(KW.dir, 'license-pro.key')),
    'kein Test-Pro-Schlüssel — internal/license/generator fehlt im Baum (Public Mirror)')

  test('KW10 Pro aktiv → EU AI Act aktivieren → Kontrollen erscheinen', async ({ browser }) => {
    const page = await openAs(browser, 'admin')
    // Vorbedingung aus Kernweg 1: die Instanz trägt den Pro-Schlüssel.
    await page.goto('/settings')
    const lic = page.getByRole('tabpanel', { name: 'Plattform' })
    await expect(lic.getByRole('heading', { name: 'Lizenz', level: 2 })).toBeVisible()
    await expect(lic.getByText('Pro', { exact: true }), 'Vorbedingung: Pro-Lizenz aus Kernweg 1').toBeVisible()

    await page.goto('/vaktcomply/frameworks')
    await expect(page.getByRole('heading', { name: 'Compliance-Frameworks', level: 1 })).toBeVisible()
    const card = page.getByText('EU AI Act — Verordnung (EU) 2024/1689', { exact: true })
      .locator('xpath=ancestor::div[.//button][1]')
    await card.getByRole('button', { name: 'Aktivieren', exact: true }).click()
    const wizard = page.getByRole('dialog', { name: 'Willkommen bei EUAIACT' })
    await expect(wizard).toBeVisible()
    await wizard.getByRole('button', { name: 'Später einrichten' }).click()
    await expect(wizard).toBeHidden()
    await expect(card.getByText('Aktiviert', { exact: true })).toBeVisible()

    await card.getByRole('button', { name: 'Anzeigen', exact: true }).click()
    await expect(page.getByRole('heading', { name: 'EUAIACT Details', level: 1 })).toBeVisible()
    await expect(page.getByText(/[1-9]\d*\s*Gesamt/)).toBeVisible()
    await expect(page.getByRole('row', { name: /AIACT-1\.1 KI-Risikomanagementsystem/ })).toBeVisible()
  })

  test('KW10 KI-System anlegen → Assistent ergibt „Hohes Risiko" (Anhang III, Personalauswahl)', async ({ browser }) => {
    const page = await openAs(browser, 'admin')
    await inventory(page)
    await page.getByRole('button', { name: 'KI-System erfassen' }).first().click()
    const dlg = page.getByRole('dialog', { name: 'KI-System erfassen' })
    await expect(dlg).toBeVisible()
    await dlg.getByPlaceholder('z.B. ChatGPT-Integration').fill(SYSTEM)
    await dlg.getByPlaceholder('z.B. OpenAI, Google, intern').fill('intern')
    await dlg.getByPlaceholder('Wofür wird das KI-System eingesetzt?').fill(USE_CASE)
    await dlg.getByPlaceholder('z.B. Mitarbeitende, Kunden, Bewerber').fill('Bewerberinnen und Bewerber')
    await dlg.getByRole('button', { name: 'Erfassen', exact: true }).click()
    await expect(dlg).toBeHidden()
    const card = systemCard(page)
    await expect(card).toBeVisible()

    // Assistent: kein verbotenes System (Art. 5) → Nein; Beschäftigung/Personalauswahl
    // ist Anhang III Nr. 4 → Ja. Ergebnis muss „Hohes Risiko" sein.
    await card.getByRole('button', { name: 'Risikoklasse bestimmen' }).click()
    const wiz = page.getByRole('dialog', { name: `Risikoklassifizierung — ${SYSTEM}` })
    await expect(wiz).toBeVisible()
    await expect(wiz.getByText('Schritt 1 von 3')).toBeVisible()
    await expect(wiz.getByText(/verbotenen Anwendungen nach Art\. 5/)).toBeVisible()
    await wiz.getByRole('button', { name: 'Nein', exact: true }).click()
    await expect(wiz.getByText('Schritt 2 von 3')).toBeVisible()
    await expect(wiz.getByText(/Hochrisikobereiche nach Annex III/)).toBeVisible()
    await wiz.getByRole('button', { name: /^Ja/ }).click()
    const result = wiz.getByTestId('wizard-result')
    await expect(result).toBeVisible()
    await expect(result.getByText('Hohes Risiko', { exact: true })).toBeVisible()
    await expect(result.getByText(/Annex III/).first()).toBeVisible()
    await wiz.getByPlaceholder('Name der verantwortlichen Person').fill('KI-Beauftragte Kernwege')
    await wiz.getByRole('button', { name: 'Klassifizierung speichern' }).click()
    await expect(wiz).toBeHidden()
  })

  test('KW10 Risikoklasse steht in Liste und Dashboard gleich', async ({ browser }) => {
    const page = await openAs(browser, 'admin')
    await inventory(page)
    const card = systemCard(page)
    await expect(card).toBeVisible()
    await expect(card.getByText('Hohes Risiko', { exact: true })).toBeVisible()
    const listLabel = (await card.getByText('Hohes Risiko', { exact: true }).innerText()).trim()

    await page.goto('/vaktcomply/eu-ai-act/dashboard')
    await expect(page.getByRole('heading', { name: 'EU AI Act Dashboard', level: 1 })).toBeVisible()
    await expect(page.getByTestId('kpi-total-systems')).toContainText(/gesamt\s*1$/)
    await expect(page.getByTestId('kpi-high-risk-count')).toContainText(/Hochrisiko-Systeme\s*1\b/)
    const breakdown = page.getByTestId('risk-class-breakdown')
    // Genau eine Klasse hat Systeme — dieselbe, die die Liste zeigt, mit Anzahl 1.
    const rows = breakdown.locator(':scope > div')
    await expect(rows).toHaveCount(1)
    await expect(rows.first()).toContainText(/1$/)
    const dashLabel = (await rows.first().locator('div,span').first().innerText()).trim()
    expect(klasse(dashLabel), `Dashboard „${dashLabel}" vs. Liste „${listLabel}"`).toBe(klasse(listLabel))
    expect(klasse(listLabel)).toBe('high')
  })

  test('KW10 Technikdoku (Art. 11) speichern → nach dem Neuladen noch da', async ({ browser }) => {
    const page = await openAs(browser, 'admin')
    await inventory(page)
    await systemCard(page).getByRole('link', { name: 'Technisches Dossier' }).click()
    await expect(page.getByRole('heading', { name: `Technisches Dossier — ${SYSTEM}`, level: 1 })).toBeVisible()
    await expect(page).toHaveURL(/\/vaktcomply\/ai-systems\/[0-9a-f-]{36}\/documentation$/)

    await page.getByPlaceholder('Systembeschreibung ausfüllen …').fill(DOC_DESCRIPTION)
    await page.getByPlaceholder('Bestimmungsgemäßer Verwendungszweck ausfüllen …').fill(DOC_PURPOSE)
    await page.getByPlaceholder('Maßnahmen für menschliche Aufsicht ausfüllen …').fill(DOC_OVERSIGHT)
    await page.getByPlaceholder('Name der verantwortlichen Person').fill('KI-Beauftragte Kernwege')
    await page.getByRole('button', { name: 'Als Entwurf speichern' }).click()
    await expect(page.getByText('Gespeichert', { exact: true })).toBeVisible()

    await page.reload()
    await expect(page.getByRole('heading', { name: `Technisches Dossier — ${SYSTEM}`, level: 1 })).toBeVisible()
    await expect(page.getByPlaceholder('Systembeschreibung ausfüllen …')).toHaveValue(DOC_DESCRIPTION)
    await expect(page.getByPlaceholder('Bestimmungsgemäßer Verwendungszweck ausfüllen …')).toHaveValue(DOC_PURPOSE)
    await expect(page.getByPlaceholder('Maßnahmen für menschliche Aufsicht ausfüllen …')).toHaveValue(DOC_OVERSIGHT)
    await expect(page.getByRole('button', { name: /^1 Version$/ })).toBeVisible()
  })

  // Das Dossier (Art. 11 / Annex IV) nennt im Kopf Name, Version und Datum, aber nicht
  // die Risikoklasse (GenerateAIDocumentationPDF, backend/…/vaktcomply/pdf.go). Wer das
  // PDF an eine Behörde oder einen Auditor gibt, sieht nicht, warum das Dossier überhaupt
  // nötig ist — und kann nicht abgleichen, ob es zur Klasse im Inventar passt.
  test('KW10 R1-KW-23 Dossier-PDF enthält das System mit seiner Risikoklasse', async ({ browser }) => {
    const page = await openAs(browser, 'admin')
    await inventory(page)
    await systemCard(page).getByRole('link', { name: 'Technisches Dossier' }).click()
    await expect(page.getByRole('heading', { name: `Technisches Dossier — ${SYSTEM}`, level: 1 })).toBeVisible()
    await expect(page.getByPlaceholder('Systembeschreibung ausfüllen …')).toHaveValue(DOC_DESCRIPTION)
    const [dl] = await Promise.all([
      page.waitForEvent('download'),
      page.getByRole('button', { name: 'PDF-Export' }).click(),
    ])
    const lines = pdfTexts(await readDownload(dl))
    // Fließtext bricht MultiCell auf mehrere Zeilen um (je Zeile ein Tj) — für den
    // Vergleich wieder zu einem Text mit einfachen Leerzeichen zusammensetzen.
    const flow = lines.join(' ').replace(/\s+/g, ' ')
    expect(flow, 'Vorbedingung: das Dossier nennt das System').toContain(SYSTEM)
    expect(flow, `Vorbedingung: das Dossier trägt die gespeicherte Technikdoku (gelesen: ${JSON.stringify(lines.slice(0, 30))})`).toContain(DOC_DESCRIPTION)
    expect(flow).toContain(DOC_OVERSIGHT)

    // Ab hier der bekannte Befund. test.fail() steht bewusst NACH den Vorbedingungen:
    // fehlt schon das System oder die Doku im PDF, ist der Lauf ROT, nicht „erwartet-rot".
    test.fail(true, 'R1-KW-23: Dossier-PDF nennt die Risikoklasse des Systems nicht')
    const classLine = lines.find((l) => /risikoklasse|risiko(stufe)?:/i.test(l) || klasse(l) === 'high')
    expect(classLine, `Dossier nennt eine Risikoklasse (gelesen: ${JSON.stringify(lines.slice(0, 12))} …)`).toBeTruthy()
    expect(klasse(classLine!)).toBe('high')
  })

  test('KW10 Bericht-PDF enthält das System mit derselben Klasse', async ({ browser }) => {
    const page = await openAs(browser, 'admin')
    await page.goto('/vaktcomply/eu-ai-act/dashboard')
    await expect(page.getByRole('heading', { name: 'EU AI Act Dashboard', level: 1 })).toBeVisible()
    await expect(page.getByTestId('kpi-total-systems')).toBeVisible()
    const [dl] = await Promise.all([
      page.waitForEvent('download'),
      page.getByRole('button', { name: 'Compliance-Report PDF' }).click(),
    ])
    const lines = pdfTexts(await readDownload(dl))
    const text = lines.join('\n')
    expect(text).toContain('EU AI Act Compliance Report')

    // Inventar-Tabelle: je Zelle ein Text, die Zelle nach dem Namen ist die Risikoklasse.
    const i = lines.indexOf(SYSTEM)
    expect(i, `Bericht führt das System im Inventar (gelesen: ${JSON.stringify(lines.slice(0, 40))} …)`).toBeGreaterThanOrEqual(0)
    expect(klasse(lines[i + 1] ?? ''), `Klasse in der Inventarzeile: „${lines[i + 1]}"`).toBe('high')

    // Abschnitt „Systeme nach Risikoklasse": genau eine Klasse mit Systemen, dieselbe, Anzahl 1.
    const from = lines.indexOf('Systeme nach Risikoklasse')
    const to = lines.indexOf('KI-System-Inventar')
    expect(from, 'Bericht hat den Abschnitt „Systeme nach Risikoklasse"').toBeGreaterThanOrEqual(0)
    const byClass = lines.slice(from + 1, to > from ? to : undefined).filter((l) => /:\s*\d+\s*$/.test(l))
    expect(byClass, `Abschnitt „Systeme nach Risikoklasse" (gelesen: ${JSON.stringify(byClass)})`).toHaveLength(1)
    expect(klasse(byClass[0]!.split(':')[0]!)).toBe('high')
    expect(byClass[0]).toMatch(/:\s*1\s*$/)
  })
})
