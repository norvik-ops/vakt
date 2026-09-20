import { test, expect } from '@playwright/test'
import { dismissFirstRunOverlays, loadState, openAs, openControlFromTable, openIsoDetail, patchState, waitForMail, closeContexts } from './lib/kw'

/**
 * Kernweg 7 — Richtlinien (docs/launch-gate.md)
 * Erfüllt, wenn: Richtlinie anlegen → freigeben → Mitarbeiter bestätigen
 * (PolicyAcceptPage) → Nachweis in Comply.
 *
 * Drei Wege, eine Richtlinie anzulegen, stehen im Produkt. Zwei davon sind kaputt
 * (R1-KW-7, R1-KW-8); der dritte (Vorlagenbibliothek) trägt den Hauptweg.
 */

const POLICY = 'Passwort- und Zugangskontrollrichtlinie'
const EMPLOYEE = 'mitarbeiterin@kernwege.test'

test.describe('KW7 Richtlinien', () => {
  test.afterEach(closeContexts)

  // PoliciesPage.tsx rendert <SelectItem value=""> („kein Framework") — Radix wirft,
  // die ErrorBoundary zeigt „Etwas ist schiefgelaufen".
  test('KW7 R1-KW-7 „Richtlinie anlegen" öffnet das Formular', async ({ browser }) => {
    const page = await openAs(browser, 'admin')
    await page.goto('/vaktcomply/policies')
    await expect(page.getByRole('heading', { name: 'Richtlinienmanagement', level: 1 })).toBeVisible()
    await page.getByRole('button', { name: 'Richtlinie anlegen' }).first().click()
    test.fail(true, 'R1-KW-7: „Richtlinie anlegen" → Absturz „A <Select.Item /> must have a value prop that is not an empty string"')
    await expect(page.getByRole('heading', { name: 'Etwas ist schiefgelaufen' })).toHaveCount(0, { timeout: 3000 })
    await expect(page.getByRole('dialog')).toBeVisible({ timeout: 3000 })
  })

  // POST /policy-templates/:id/apply trägt einen Slug („password-policy"), der
  // UUID-Wächter (ValidateUUIDParams, Denylist ohne "id"-Ausnahme) lehnt ihn mit 400 ab.
  test('KW7 R1-KW-8 „Aus Vorlage" auf der Richtlinienseite legt die Richtlinie an', async ({ browser }) => {
    const page = await openAs(browser, 'admin')
    await page.goto('/vaktcomply/policies')
    await page.getByRole('button', { name: 'Aus Vorlage' }).click()
    const dlg = page.getByRole('dialog', { name: 'Richtlinie aus Vorlage erstellen' })
    await expect(dlg).toBeVisible()
    const [resp] = await Promise.all([
      page.waitForResponse((r) => r.url().includes('/policy-templates/') && r.request().method() === 'POST'),
      dlg.getByRole('button', { name: /^Richtlinie zur akzeptablen Nutzung/ }).click(),
    ])
    test.fail(true, 'R1-KW-8: „Aus Vorlage" → 400 „invalid id: must be a UUID" (Slug-Parameter vom UUID-Wächter abgelehnt)')
    expect(resp.status(), await resp.text()).toBeLessThan(300)
  })

  test('KW7 Richtlinie aus der Vorlagenbibliothek anlegen und freigeben', async ({ browser }) => {
    const page = await openAs(browser, 'admin')
    await page.goto('/vaktcomply/policy-templates')
    await expect(page.getByRole('heading', { name: 'Vorlagenbibliothek', level: 1 })).toBeVisible()
    await page.getByText(POLICY, { exact: true }).click()
    await page.getByRole('dialog').getByRole('button', { name: 'Diese Vorlage verwenden' }).click()

    // Sperre gegen Rückkehr von R1-SA28-02 (GATE-7, gefixt): Erfolg nur, wenn die
    // Richtlinie wirklich existiert — Detailseite mit ID, kein 400.
    await expect(page).toHaveURL(/\/vaktcomply\/policies\/[0-9a-f-]{36}$/)
    await expect(page.getByRole('heading', { name: POLICY, level: 1 })).toBeVisible()
    const policyId = /policies\/([0-9a-f-]{36})/.exec(page.url())![1]

    // Freigeben: Status Entwurf → Aktiv, mit Änderungsnotiz.
    await page.getByRole('combobox').filter({ hasText: 'Entwurf' }).click()
    await page.getByRole('option', { name: 'Aktiv', exact: true }).click()
    await page.getByLabel(/Änderungsnotiz/).fill('Freigabe durch die Geschäftsführung (Kernwege-Test)')
    await page.getByRole('button', { name: 'Speichern' }).click()
    await expect(page.getByText(/Gespeichert|gespeichert|aktualisiert/).first()).toBeVisible()
    await page.reload()
    await expect(page.getByRole('combobox').filter({ hasText: 'Aktiv' })).toBeVisible()
    await expect(page.getByText(/Version|Revision/).first()).toBeVisible()

    await page.goto('/vaktcomply/policies')
    await expect(page.getByText(POLICY).first()).toBeVisible()
    patchState({ policyTitle: POLICY, policyId })
  })

  // Die Akzeptanz-Kampagnen (PolicyAcceptancePage, Route policies/:id/acceptance)
  // sind von keiner Seite aus verlinkt — nur per URL erreichbar.
  test('KW7 R1-KW-11 Weg zur Mitarbeiter-Bestätigung ist aus der Richtlinie heraus erreichbar', async ({ browser }) => {
    const s = loadState()
    expect(s.policyId, 'Richtlinie aus dem vorigen Test').toBeTruthy()
    const page = await openAs(browser, 'admin')
    await page.goto(`/vaktcomply/policies/${s.policyId}`)
    await expect(page.getByRole('heading', { name: POLICY, level: 1 })).toBeVisible()
    test.fail(true, 'R1-KW-11: Akzeptanz-Kampagnen (policies/:id/acceptance) sind nirgends verlinkt')
    await expect(page.getByRole('link', { name: /Akzeptanz|bestätigen/i })
      .or(page.getByRole('button', { name: /Akzeptanz|bestätigen/i })).first()).toBeVisible({ timeout: 3000 })
  })

  test('KW7 Mitarbeiterin bestätigt die Richtlinie per Link → Nachweis an A.5.1 in Comply', async ({ browser }) => {
    const s = loadState()
    expect(s.policyId, 'Richtlinie aus dem vorigen Test').toBeTruthy()
    const page = await openAs(browser, 'admin')
    // Bis R1-KW-11 behoben ist, führt nur die URL hierher.
    await page.goto(`/vaktcomply/policies/${s.policyId}/acceptance`)
    await expect(page.getByRole('heading', { name: 'Richtlinien-Akzeptanz', level: 1 })).toBeVisible()
    await page.getByRole('button', { name: 'Neue Kampagne' }).first().click()
    const dlg = page.getByRole('dialog')
    await dlg.getByLabel('Kampagnenname *').fill('Passwortrichtlinie 2026')
    await dlg.getByLabel('E-Mail-Adressen *').fill(`${EMPLOYEE},Mia Mitarbeiterin`)
    await dlg.getByRole('button', { name: 'Kampagne erstellen & E-Mails senden' }).click()
    await expect(page.getByText('Passwortrichtlinie 2026')).toBeVisible()
    await expect(page.getByText(/0 von 1 bestätigt/)).toBeVisible()

    // Ab hier braucht es die Mail an die Mitarbeiterin — R1-KW-1 verhindert sie.
    test.fail(true, 'R1-KW-1: Akzeptanz-Mail wird nie zugestellt (SMTP-AUTH mit Kommentartext aus .env.example)')
    const mail = await waitForMail(page, EMPLOYEE)
    const link = /(https?:\/\/[^\s"<>]+\/policy\/accept\/[A-Za-z0-9_-]+)/.exec(`${mail.text} ${mail.html}`)?.[1]
    expect(link, 'Mail enthält den Bestätigungslink').toBeTruthy()

    const ctx = await browser.newContext()
    const emp = await ctx.newPage()
    await dismissFirstRunOverlays(emp)
    await emp.goto(link!)
    await expect(emp.getByText(POLICY).first()).toBeVisible()
    await emp.getByRole('button', { name: 'Ich habe die Richtlinie gelesen und akzeptiere sie' }).click()
    await expect(emp.getByText(/bestätigt|Vielen Dank/i).first()).toBeVisible()

    await page.reload()
    await expect(page.getByText(/1 von 1 bestätigt/)).toBeVisible()
    // Nachweis landet an ISO 27001 A.5.1.
    await openIsoDetail(page)
    await openControlFromTable(page, 'Richtlinien zur Informationssicherheit')
    await expect(page.getByText(`Richtlinien-Akzeptanz: ${POLICY}`)).toBeVisible()
  })
})
