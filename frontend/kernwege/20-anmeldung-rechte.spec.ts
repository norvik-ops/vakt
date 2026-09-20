import { test, expect, request, type Browser, type Page } from '@playwright/test'
import {
  KW, apiWrite, dismissFirstRunOverlays, login, loadState, markAuthBudgetUsed, noteAuthRequest, openAs, waitForFreshAuthWindow, patchState,
  saveSession, saveState, totp, waitForMail, closeContexts } from './lib/kw'

/**
 * Kernweg 3 — Anmeldung & Rechte (docs/launch-gate.md)
 * Erfüllt, wenn: Setup → erster Admin → Nutzer einladen → Login mit MFA → Rollen greifen
 * (kein Schreibzugriff ohne Rolle).
 *
 * Diese Datei läuft als erste nach 10-installation: sie legt den ersten Admin über
 * /setup an. Alle späteren Kernwege arbeiten mit dieser Anmeldung weiter.
 */

const ORG = 'Kernwege GmbH'
const ADMIN = { email: 'admin@kernwege.test', password: 'Kernwege-Admin-2026!' }
const VIEWER = { email: 'viewer@kernwege.test', password: 'Kernwege-Viewer-2026!' }

async function freshPage(browser: Browser): Promise<Page> {
  const ctx = await browser.newContext()
  const page = await ctx.newPage()
  await dismissFirstRunOverlays(page)
  return page
}

async function openTeam(browser: Browser): Promise<Page> {
  const page = await openAs(browser, 'admin')
  await page.goto('/settings/team')
  await expect(page.getByRole('heading', { name: 'Team', level: 1 })).toBeVisible()
  return page
}

async function invite(page: Page, email: string): Promise<void> {
  await page.getByRole('button', { name: 'Mitglied einladen' }).click()
  const dlg = page.getByRole('dialog', { name: 'Mitglied einladen' })
  await dlg.getByLabel('E-Mail-Adresse').fill(email)
  await dlg.getByRole('combobox').click()
  await page.getByRole('option', { name: /^Viewer/ }).click()
  await dlg.getByRole('button', { name: 'Einladung senden' }).click()
  await expect(page.getByText('Einladung gesendet', { exact: true })).toBeVisible()
}

/** Wartet auf das nächste 30-s-Fenster — ein TOTP-Code gilt nur einmal. */
async function nextTotpWindow(page: Page): Promise<void> {
  const step = Math.floor(Date.now() / 30_000)
  while (Math.floor(Date.now() / 30_000) === step) await page.waitForTimeout(500)
}

test.describe('KW3 Anmeldung & Rechte', () => {
  test.afterEach(closeContexts)

  test('KW3 frische Instanz leitet auf /setup, Setup legt ersten Admin an, Admin meldet sich an', async ({ page }) => {
    await page.goto('/')
    await expect(page).toHaveURL(/\/setup$/)
    await expect(page.getByRole('heading', { name: /Schritt 1 von 3/ })).toBeVisible()

    await page.getByLabel('Organisationsname').fill(ORG)
    await page.getByRole('button', { name: 'Weiter' }).click()
    await page.getByLabel('E-Mail-Adresse').fill(ADMIN.email)
    await page.getByLabel('Passwort').fill(ADMIN.password)
    await page.getByRole('button', { name: 'Weiter' }).click()
    await expect(page.getByRole('heading', { name: /Schritt 3 von 3/ })).toBeVisible()
    await expect(page.getByRole('checkbox', { name: /^Vakt Comply/ })).toBeChecked()
    await page.getByRole('button', { name: 'Einrichtung abschließen' }).click()

    // Nach dem Setup: Anmeldemaske, und /setup ist zu (Setup läuft genau einmal).
    await expect(page).toHaveURL(/\/login$/)
    saveState({ orgName: ORG, admin: ADMIN })
    await page.goto('/setup')
    await expect(page).toHaveURL(/\/login$/)

    await dismissFirstRunOverlays(page)
    await login(page, ADMIN.email, ADMIN.password)
    await expect(page.getByRole('link', { name: /Benutzerprofil öffnen/ })).toContainText(ADMIN.email)
    await saveSession(page, 'admin')
  })

  // Die Einladung ist der Weg, den launch-gate.md nennt. Sie scheitert an der
  // Anleitung selbst: .env.example schreibt `VAKT_SMTP_USER=   # required …`, und
  // docker compose liest bei leerem Wert den KOMMENTAR als Wert. Die Instanz versucht
  // SMTP-AUTH mit dem Kommentartext — die Mail geht nie raus (API-Log: „smtp: server
  // doesn't support AUTH"), und die Einladungsliste bietet keinen Link zum Weitergeben.
  test('KW3 R1-KW-1 Einladung: Mail kommt an, Link erstellt das Konto, Eingeladener meldet sich an', async ({ browser }) => {
    const invitee = { email: 'eingeladen@kernwege.test', password: 'Kernwege-Invite-2026!' }
    const page = await openTeam(browser)
    await invite(page, invitee.email)
    await expect(page.getByRole('row', { name: /eingeladen@kernwege\.test/ })).toBeVisible()

    // Ab hier der bekannte Befund. test.fail() steht bewusst NACH den Vorbedingungen:
    // scheitert schon das Einladen, ist der Lauf ROT, nicht „erwartet-rot".
    test.fail(true, 'R1-KW-1: .env.example → SMTP_USER/PASS = Kommentartext, Einladungsmail wird nie zugestellt')
    const mail = await waitForMail(page, invitee.email)
    const link = /(https?:\/\/[^\s"<>]+\/invite\/accept\?[^\s"<>]+)/.exec(`${mail.text} ${mail.html}`)?.[1]
    expect(link, 'Einladungsmail enthält einen Annahme-Link').toBeTruthy()

    const p2 = await freshPage(browser)
    await p2.goto(link!.replace(/&amp;/g, '&'))
    await p2.getByLabel('Name').fill('Eingeladene Person')
    await p2.getByLabel('Passwort', { exact: true }).fill(invitee.password)
    await p2.getByLabel('Passwort wiederholen').fill(invitee.password)
    const [acc] = await Promise.all([
      p2.waitForResponse((r) => r.url().endsWith('/api/v1/invite/accept') && r.request().method() === 'POST'),
      p2.getByRole('button', { name: 'Konto erstellen' }).click(),
    ])
    expect(acc.status(), `${acc.request().postData()} → ${await acc.text()}`).toBe(201)
    await expect(p2).toHaveURL(/\/login\?message=account-created$/)
    await login(p2, invitee.email, invitee.password)
  })

  test('KW3 R1-KW-2 Liste der offenen Einladungen zeigt die eingeladene Rolle', async ({ browser }) => {
    const page = await openTeam(browser)
    await invite(page, 'rolle@kernwege.test')
    const row = page.getByRole('row', { name: /rolle@kernwege\.test/ })
    await expect(row).toBeVisible()
    test.fail(true, 'R1-KW-2: API liefert role "Viewer", TeamSettingsPage erwartet "viewer" — Rollenspalte bleibt leer')
    await expect(row.getByRole('cell').nth(1)).toHaveText('Viewer', { timeout: 3000 })
  })

  test('KW3 Admin legt einen Viewer direkt an, Viewer meldet sich an', async ({ browser }) => {
    const page = await openTeam(browser)
    await page.getByRole('button', { name: 'Direkt anlegen' }).click()
    const dlg = page.getByRole('dialog', { name: 'Nutzer direkt anlegen' })
    await dlg.getByLabel('E-Mail-Adresse').fill(VIEWER.email)
    await dlg.getByLabel('Passwort').fill(VIEWER.password)
    await expect(dlg.getByRole('combobox')).toHaveText(/Viewer/)
    await dlg.getByRole('button', { name: 'Nutzer anlegen' }).click()
    await expect(page.getByRole('row', { name: /viewer@kernwege\.test/ })).toContainText('Viewer')
    patchState({ viewer: { ...VIEWER } })

    const v = await freshPage(browser)
    await login(v, VIEWER.email, VIEWER.password)
    await saveSession(v, 'viewer')
  })

  // Der Dialog „2FA aktivieren" holt das Secret in handleOpen() — das hängt an Radix'
  // onOpenChange, das nur feuert, wenn der Dialog SICH SELBST öffnet, nicht wenn die
  // Seite `open` setzt (AccountSettingsPage.tsx). POST /auth/2fa/setup geht nie raus,
  // „Weiter" bleibt gesperrt. Seit dem Monorepo-Start (3f0a6610) kann niemand MFA
  // per Oberfläche einschalten.
  test('KW3 R1-KW-3 Viewer schaltet 2FA in den Konto-Einstellungen ein', async ({ browser }) => {
    const v = await openAs(browser, 'viewer')
    await v.goto('/account')
    await expect(v.getByRole('heading', { name: /Zwei-Faktor-Authentifizierung/ })).toContainText('Inaktiv')
    await v.getByRole('button', { name: '2FA aktivieren' }).click()
    const setup = v.getByRole('dialog', { name: 'Zwei-Faktor-Authentifizierung einrichten' })
    await expect(setup).toBeVisible()

    test.fail(true, 'R1-KW-3: 2FA-Dialog lädt nie ein Secret (handleOpen hängt an onOpenChange) — MFA per Oberfläche nicht einschaltbar')
    const secret = (await setup.locator('code').first().innerText({ timeout: 10_000 })).trim()
    expect(secret).toMatch(/^[A-Z2-7]{16,}=*$/)
    await setup.getByRole('button', { name: 'Weiter' }).click()
    await setup.getByLabel('Authentifizierungscode').fill(totp(secret))
    await setup.getByRole('button', { name: 'Bestätigen' }).click()
    await v.getByRole('button', { name: '2FA ist jetzt aktiv' }).click()
    await expect(v.getByRole('heading', { name: /Zwei-Faktor-Authentifizierung/ })).toContainText('Aktiv')
    patchState({ viewer: { ...VIEWER, totpSecret: secret } })
    await nextTotpWindow(v)
  })

  test('KW3 Login mit MFA: Passwort allein reicht nicht, der TOTP-Code schließt ab', async ({ browser }) => {
    test.setTimeout(240_000) // bis zu 30 s aufs nächste TOTP-Fenster + Login-Abstand + Rate-Budget
    let s = loadState()
    if (!s.viewer?.totpSecret) {
      // Solange R1-KW-3 offen ist, schaltet der Test MFA mit denselben zwei Aufrufen
      // ein, die der Dialog machen müsste (setup → confirm) — damit der LOGIN mit MFA
      // geprüft bleibt. Grenze: das Einschalten per Oberfläche ist R1-KW-3.
      // Bewusst ohne Umweg über /account: jede 2FA-Anfrage zählt gegen ein Budget von
      // 5 in 5 Minuten je IP, auch das Lesen des Status (R1-KW-13).
      const v = await openAs(browser, 'viewer')
      const r = await apiWrite(v.context(), 'POST', '/api/v1/auth/2fa/setup')
      expect(r.status(), await r.text()).toBe(200)
      const { secret } = (await r.json()) as { secret: string }
      const c = await apiWrite(v.context(), 'POST', '/api/v1/auth/2fa/confirm', { code: totp(secret) })
      expect(c.status(), await c.text()).toBe(200)
      s = patchState({ viewer: { ...VIEWER, totpSecret: secret } })
      await nextTotpWindow(v)
    }
    const v2 = await freshPage(browser)
    await login(v2, VIEWER.email, VIEWER.password, s.viewer!.totpSecret)
    await saveSession(v2, 'viewer')
  })

  // Das 2FA-Limit (TOTPRateLimit, 5 Anfragen je 5 Minuten und IP) zählt JEDE Route der
  // /auth/2fa-Gruppe mit — auch GET /2fa/status, das die Konto-Seite beim Öffnen lädt.
  // Wer MFA gerade eingeschaltet hat (Status, Setup, Bestätigen, Login) und sich beim
  // nächsten Login einmal vertippt, ist für 5 Minuten ausgesperrt; die Meldung sagt
  // „bitte 1 Sekunden warten". Hinter einem Büro-NAT teilen sich alle dieses Budget.
  test('KW3 R1-KW-13 nach einem Tippfehler im 2FA-Code kommt der richtige Code durch', async ({ browser }) => {
    const s = loadState()
    const v = await freshPage(browser)
    await v.goto('/login')
    await v.getByLabel('E-Mail').fill(VIEWER.email)
    await v.getByLabel('Passwort').fill(VIEWER.password)
    await v.getByRole('button', { name: 'Anmelden' }).click()
    noteAuthRequest()
    await expect(v.getByLabel('Authentifizierungscode')).toBeVisible()
    await expect(v.getByRole('heading', { name: 'Dashboard', level: 1 })).toHaveCount(0)

    test.fail(true, 'R1-KW-13: 2FA-Limit 5/5 min je IP zählt auch GET /2fa/status — ein Tippfehler nach dem Einrichten sperrt 5 min („bitte 1 Sekunden warten")')
    await v.getByLabel('Authentifizierungscode').fill('000000')
    await v.getByRole('button', { name: 'Bestätigen' }).click()
    await expect(v.getByRole('alert').filter({ hasText: /Ungültig|invalid/i })).toBeVisible({ timeout: 5000 })
    await nextTotpWindow(v)
    await v.getByLabel('Authentifizierungscode').fill(totp(s.viewer!.totpSecret!))
    await v.getByRole('button', { name: 'Bestätigen' }).click()
    await expect(v.getByRole('heading', { name: 'Dashboard', level: 1 })).toBeVisible({ timeout: 10_000 })
  })

  test('KW3 Rollen greifen: Viewer kann kein Risiko anlegen — weder in der Oberfläche noch an der API vorbei', async ({ browser }) => {
    const page = await openAs(browser, 'viewer')
    const title = 'Viewer-Versuch: darf nicht entstehen'
    await page.goto('/vaktcomply/risks')
    await expect(page.getByRole('heading', { name: 'Risikoregister', level: 1 })).toBeVisible()

    // Oberfläche: Der Viewer versucht es wie jeder andere — und bekommt eine Absage.
    const create = page.getByRole('button', { name: 'Risiko erfassen' }).first()
    if (await create.isVisible()) {
      await create.click()
      const dlg = page.getByRole('dialog', { name: 'Risiko erfassen' })
      await dlg.getByLabel('Bezeichnung *').fill(title)
      await dlg.getByRole('button', { name: 'Risiko erfassen' }).click()
      await expect(page.getByText(/Rolle|Berechtigung|nicht erlaubt|keine Rechte|forbidden|insufficient/i).first()).toBeVisible()
    }
    await page.goto('/vaktcomply/risks')
    await expect(page.getByRole('heading', { name: 'Risikoregister', level: 1 })).toBeVisible()
    await expect(page.getByText(title)).toHaveCount(0)

    // API: dieselbe Sitzung, am Formular vorbei.
    const r = await apiWrite(page.context(), 'POST', '/api/v1/vaktcomply/risks', {
      title, likelihood: 3, impact: 3, treatment: 'mitigate',
    })
    expect(r.status(), await r.text()).toBe(403)

    // Sperre gegen Rückkehr von R1-SA13-07 (GATE-3, gefixt): die NIS2-Schreibrouten
    // prüfen die Rolle VOR der Validierung — Viewer bekommt 403, nicht 400.
    for (const url of [
      '/api/v1/vaktcomply/nis2-assessment/migrate-from-anonymous',
      '/api/v1/vaktcomply/nis2-assessment/multi/00000000-0000-0000-0000-000000000000/answer',
    ]) {
      const w = await apiWrite(page.context(), 'POST', url, {})
      expect(w.status(), `${url}: ${await w.text()}`).toBe(403)
    }
  })

  // Die /auth-Gruppe (login, refresh, logout) teilt sich 5 Anfragen pro Minute und IP
  // (cmd/api/routes.go authRateLimiter, seit R1-SA08-01 wirksam). Hinter einem Büro-NAT
  // teilen sich alle Mitarbeiter diese eine IP: die sechste Anmeldung in einer Minute
  // bekommt „Zu viele Anfragen". docs/dev/projekt-referenz.md verspricht das Gegenteil
  // („nur das gezielte Konto, nicht die ganze IP — NAT-sicher").
  test('KW3 R1-KW-4 sechs Anmeldungen hinter einer IP innerhalb einer Minute', async () => {
    test.setTimeout(240_000)
    await waitForFreshAuthWindow()
    const s = loadState()
    const creds = [s.admin, { email: VIEWER.email, password: VIEWER.password }]
    const codes: number[] = []
    try {
      for (let i = 0; i < 6; i++) {
        const ctx = await request.newContext({ baseURL: KW.baseURL })
        const c = creds[i % creds.length]
        const r = await ctx.post('/api/v1/auth/login', { data: { email: c.email, password: c.password } })
        codes.push(r.status())
        await ctx.dispose()
      }
      // Vorbedingung: ein korrekter Login liefert 200.
      expect(codes[0], `Statuscodes: ${codes.join(',')}`).toBe(200)
      test.fail(true, 'R1-KW-4: Login-Limits zählen pro IP, nicht pro Konto — ab der 5. Anmeldung/Minute hinter einem NAT 429')
      expect(codes, `Statuscodes: ${codes.join(',')}`).toEqual([200, 200, 200, 200, 200, 200])
    } finally {
      // Das Fenster ist verbraucht — die nächsten Logins der Suite warten es ab.
      markAuthBudgetUsed(Date.now() + 61_000)
    }
  })
})
