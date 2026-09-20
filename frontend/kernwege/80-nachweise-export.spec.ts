import fs from 'node:fs'
import { test, expect, type Page } from '@playwright/test'
import { openAs, openControlFromTable, openIsoDetail, patchState, runToken, closeContexts } from './lib/kw'
import { unzip } from './lib/zip'

/**
 * Kernweg 9 — Nachweise & Audit-Export (docs/launch-gate.md)
 * Erfüllt, wenn: Nachweis hochladen, Kontrolle zuordnen, Audit-Export (PDF/ZIP)
 * enthält die Belege, Zahlen darin widersprechen sich nicht.
 */

const EVIDENCE_TITLE = 'Rollenkonzept Informationssicherheit 2026'
const EVIDENCE_FILE = 'Rollenkonzept-2026.pdf'
// Ein gültiges Mini-PDF mit einer Markierung, die im Export byte-genau wiederkommen muss.
const MARKER = `kernwege-beleg-${runToken('beleg')}`
const PDF = Buffer.from(
  `%PDF-1.4\n1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj\n` +
  `3 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 200 200]>>endobj\n% ${MARKER}\ntrailer<</Root 1 0 R>>\n%%EOF\n`,
)

const ATTACHMENT_FILE = 'Rollenmatrix-Anhang-2026.pdf'
const ATTACHMENT = Buffer.from(`%PDF-1.4\n% ${MARKER}-anhang\ntrailer<<>>\n%%EOF\n`)

function csv(buf: Buffer | undefined): string[][] {
  if (!buf) return []
  return buf.toString('utf8').trim().split(/\r?\n/).map((l) =>
    [...l.matchAll(/("(?:[^"]|"")*"|[^,]*)(,|$)/g)].map((m) => m[1].replace(/^"|"$/g, '').replace(/""/g, '"')).slice(0, -1),
  )
}

async function openControl(page: Page, title: string): Promise<void> {
  await openIsoDetail(page)
  await openControlFromTable(page, title)
}

async function downloadAuditPackage(page: Page): Promise<Map<string, Buffer>> {
  await page.goto('/vaktcomply/frameworks')
  const [dl] = await Promise.all([
    page.waitForEvent('download'),
    page.getByRole('button', { name: 'Audit-Paket exportieren' }).click(),
  ])
  expect(dl.suggestedFilename()).toMatch(/^audit-paket-\d{4}-\d{2}-\d{2}\.zip$/)
  return unzip(fs.readFileSync((await dl.path())!))
}

test.describe('KW9 Nachweise & Audit-Export', () => {
  test.afterEach(closeContexts)

  test('KW9 Nachweis an A.5.2: Dokument über „Nachweis hinzufügen" und Datei als Anhang hochladen', async ({ browser }) => {
    const page = await openAs(browser, 'admin')
    await openControl(page, 'Rollen und Verantwortlichkeiten für Informationssicherheit')
    await expect(page.getByText('Noch keine Nachweise')).toBeVisible()

    // Weg 1: „Nachweis hinzufügen" → Typ „Dokument (Datei hochladen)".
    await page.getByRole('button', { name: 'Nachweis hinzufügen' }).click()
    const dlg = page.getByRole('dialog', { name: 'Nachweis hinzufügen' })
    await dlg.getByLabel('Titel').fill(EVIDENCE_TITLE)
    await dlg.getByRole('combobox').click()
    await page.getByRole('option', { name: 'Dokument (Datei hochladen)' }).click()
    await dlg.locator('input[type="file"]').setInputFiles({ name: EVIDENCE_FILE, mimeType: 'application/pdf', buffer: PDF })
    await dlg.getByRole('button', { name: 'Nachweis hinzufügen' }).click()
    await expect(dlg).toBeHidden()
    await expect(page.getByRole('row', { name: new RegExp(EVIDENCE_TITLE) })).toBeVisible()

    // Weg 2: „Anhänge" — Datei ablegen/auswählen.
    const [chooser] = await Promise.all([
      page.waitForEvent('filechooser'),
      page.getByRole('button', { name: /Datei hier ablegen oder auswählen/ }).click(),
    ])
    await chooser.setFiles({ name: ATTACHMENT_FILE, mimeType: 'application/pdf', buffer: ATTACHMENT })
    await expect(page.getByRole('listitem').filter({ hasText: ATTACHMENT_FILE })).toBeVisible()

    await page.reload()
    await expect(page.getByRole('row', { name: new RegExp(EVIDENCE_TITLE) })).toBeVisible()
    await expect(page.getByRole('listitem').filter({ hasText: ATTACHMENT_FILE })).toBeVisible()
    patchState({ evidenceFile: ATTACHMENT_FILE })
  })

  test('KW9 Audit-Paket (ZIP): Anhang byte-genau drin, Nachweis an A.5.2 gelistet, Risiko und Richtlinie dabei', async ({ browser }) => {
    const page = await openAs(browser, 'admin')
    const zip = await downloadAuditPackage(page)
    for (const f of ['README.txt', 'controls.csv', 'evidence.csv', 'risks.csv', 'policies.csv']) {
      expect(zip.has(f), `${f} im Audit-Paket`).toBe(true)
    }
    const anhang = zip.get(`evidence/${ATTACHMENT_FILE}`)
    expect(anhang, `evidence/${ATTACHMENT_FILE} im Paket (Inhalt: ${[...zip.keys()].join(', ')})`).toBeTruthy()
    expect(anhang!.equals(ATTACHMENT)).toBe(true)

    const ev = csv(zip.get('evidence.csv'))
    expect(ev.some((r) => r[0] === 'A.5.2' && r[2] === EVIDENCE_TITLE), JSON.stringify(ev)).toBe(true)
    expect(zip.get('risks.csv')!.toString('utf8')).toContain('Ransomware verschlüsselt den Fileserver')
    expect(zip.get('policies.csv')!.toString('utf8')).toContain('Passwort- und Zugangskontrollrichtlinie')
  })

  // „Nachweis hinzufügen → Dokument" schreibt über den veralteten Endpunkt
  // /controls/:id/evidence/upload nach ck_evidence.file_path. Das Audit-Paket liest nur
  // ck_evidence_files („Anhänge"). Die Datei fehlt im Paket — und die Nachweis-Zeile
  // bietet auch keinen Download, nur „Verlauf anzeigen".
  test('KW9 R1-KW-12 als „Nachweis" hochgeladenes Dokument steckt im Audit-Paket', async ({ browser }) => {
    const page = await openAs(browser, 'admin')
    const zip = await downloadAuditPackage(page)
    const ev = csv(zip.get('evidence.csv'))
    expect(ev.some((r) => r[2] === EVIDENCE_TITLE), 'Vorbedingung: Nachweis ist im Register gelistet').toBe(true)
    test.fail(true, 'R1-KW-12: Dokument aus „Nachweis hinzufügen" fehlt im Audit-Paket (veralteter Upload-Weg, Export liest nur Anhänge)')
    // Byte-Vergleich, nicht Teilstring: der Anhang trägt `${MARKER}-anhang` und würde sonst mitzählen.
    const doc = [...zip.entries()].find(([n, b]) => n.startsWith('evidence/') && b.equals(PDF))
    expect(doc, `Beleg mit Markierung im Paket (Inhalt: ${[...zip.keys()].join(', ')})`).toBeTruthy()
  })

  test('KW9 Zahlen im Audit-Paket = Zahlen in der Oberfläche (umgesetzte ISO-Kontrollen)', async ({ browser }) => {
    const page = await openAs(browser, 'admin')
    await openIsoDetail(page)
    const head = await page.getByText(/\d+\s*Umgesetzt/).first().innerText()
    const uiImplemented = Number(/(\d+)/.exec(head)![1])
    expect(uiImplemented, 'A.5.1 ist umgesetzt (Kernweg 6)').toBeGreaterThanOrEqual(1)

    const zip = await downloadAuditPackage(page)
    const rows = csv(zip.get('controls.csv')).filter((r) => r[1] === 'ISO27001')
    expect(rows.length, 'alle 93 ISO-Kontrollen im Paket').toBe(93)
    const zipImplemented = rows.filter((r) => /^(implemented|Umgesetzt)$/i.test(r[5])).length
    expect(zipImplemented).toBe(uiImplemented)
  })
})
