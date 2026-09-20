import crypto from 'node:crypto'
import fs from 'node:fs'
import path from 'node:path'
import { execFileSync } from 'node:child_process'
import { expect, request, type APIResponse, type Browser, type BrowserContext, type Locator, type Page } from '@playwright/test'

/**
 * Gemeinsame Helfer der Kernweg-Tests. Alles hier bildet ab, was ein Mensch tut
 * (klicken, tippen, eine Mail öffnen, einen Code aus der Authenticator-App abtippen) —
 * kein Seed, kein direkter DB-Zugriff. Wo ein Test die API direkt ruft, steht der
 * Grund daneben (Rollenprüfung: „ohne Rolle kein Schreibzugriff" gilt auch für den,
 * der die Oberfläche umgeht).
 */

function need(name: string): string {
  const v = process.env[name]
  if (!v) throw new Error(`${name} fehlt — Kernwege nur über \`make kernwege\` starten.`)
  return v
}

export const KW = {
  get baseURL() { return need('KW_BASE_URL') },
  get mailURL() { return need('KW_MAIL_URL') },
  get dir() { return need('KW_DIR') },
  get envFile() { return need('KW_ENV_FILE') },
  get renewalToken() { return need('KW_RENEWAL_TOKEN') },
  get compose() { return need('KW_COMPOSE').split(' ') },
}

// ── Zustand zwischen den Kernwegen ───────────────────────────────────────────
// Die Wege bauen aufeinander auf wie bei einem echten Kunden: 20-anmeldung legt den
// Admin an, 40-framework aktiviert NIS2, 80-nachweise lädt einen Beleg hoch usw.
// Was ein späterer Weg wissen muss, steht hier — nicht in einer Seed-Datei.
export interface KwState {
  orgName: string
  admin: { email: string; password: string }
  viewer?: { email: string; password: string; totpSecret?: string }
  frameworkName?: string
  policyTitle?: string
  policyId?: string
  riskTitle?: string
  evidenceFile?: string
}

const statePath = () => path.join(KW.dir, 'state.json')

export function loadState(): KwState {
  const p = statePath()
  if (!fs.existsSync(p)) {
    throw new Error('Kein Kernweg-Zustand — 20-anmeldung-rechte muss zuerst gelaufen sein (Setup legt den Admin an).')
  }
  return JSON.parse(fs.readFileSync(p, 'utf8')) as KwState
}

export function saveState(s: KwState): void {
  fs.writeFileSync(statePath(), JSON.stringify(s, null, 2))
}

/**
 * Zufallswert, der über den ganzen Lauf gleich bleibt. Modul-Konstanten taugen dafür
 * nicht: nach einem fehlgeschlagenen Test startet Playwright einen neuen Worker und
 * wertet die Module neu aus.
 */
export function runToken(name: string, bytes = 12): string {
  const f = path.join(KW.dir, `token-${name}`)
  if (!fs.existsSync(f)) fs.writeFileSync(f, crypto.randomBytes(bytes).toString('hex'), { mode: 0o600 })
  return fs.readFileSync(f, 'utf8').trim()
}

export function patchState(p: Partial<KwState>): KwState {
  const s = { ...loadState(), ...p }
  saveState(s)
  return s
}

// ── Oberfläche ──────────────────────────────────────────────────────────────
/**
 * Zwei Einblendungen beim ersten Login: „Was ist neu in Vakt" (einmal je Version und
 * Browser) und die Produkttour. Ein Mensch klickt sie einmal weg. Die Tests öffnen
 * aber für fast jeden Schritt einen frischen Browser-Kontext — ohne Vorkehrung sähe
 * jeder Schritt die Einblendungen erneut, und die Tour legt sich dabei über offene
 * Dialoge (ein Locator-Handler, der sie wegklicken will, hängt dann am Modal-Overlay).
 *
 * Deshalb:
 *   - Die Tour gilt in jedem Kontext als erledigt (vakt_tour_completed) — sie ist
 *     Teil keines Kernwegs.
 *   - „Was ist neu" sieht der erste Login jeder Rolle echt und klickt es weg
 *     (login()); gespeicherte Sitzungen (openAs) tragen den Merker als wiederkehrender
 *     Nutzer mit. Als Rückfallnetz bleibt ein Locator-Handler.
 */
/**
 * Jede Browser-Sitzung, die ein Test öffnet, wird nach dem Test geschlossen
 * (closeContexts in afterEach). Offen gelassene SPAs pollen weiter die API
 * (Benachrichtigungen, Freigaben …) — nach zwanzig Tests liefen dreißig davon
 * parallel, und Klicks liefen gelegentlich in den Test-Timeout.
 */
const openedContexts = new Set<BrowserContext>()

export async function closeContexts(): Promise<void> {
  const all = [...openedContexts]
  openedContexts.clear()
  await Promise.all(all.map((c) => c.close().catch(() => undefined)))
  await ensureRateBudget()
}

/**
 * Die API erlaubt je Organisation 300 Anfragen pro Minute (OrgRateLimitRedis, festes
 * Minutenfenster), jeder Seitenaufruf der SPA kostet 20–30. Die Suite klickt schneller
 * als ein Mensch; ohne Pause liefe sie mitten in einem Test in 429. Vor jedem Test
 * liest dieser Helfer den Rest des Fensters aus X-RateLimit-Remaining und wartet, wenn
 * weniger als 150 übrig sind, bis zum Fensterwechsel (X-RateLimit-Reset).
 */
async function ensureRateBudget(): Promise<void> {
  if (!fs.existsSync(sessionFile('admin'))) return
  const api = await request.newContext({ baseURL: KW.baseURL, storageState: sessionFile('admin') })
  try {
    const r = await api.get('/api/v1/license')
    const remaining = Number(r.headers()['x-ratelimit-remaining'] ?? '300')
    const reset = Number(r.headers()['x-ratelimit-reset'] ?? '0')
    if (remaining < 150 && reset > 0) {
      const wait = reset * 1000 - Date.now() + 500
      if (wait > 0 && wait < 65_000) await new Promise((res) => setTimeout(res, wait))
    }
  } finally {
    await api.dispose()
  }
}

export async function prepareContext(ctx: BrowserContext, returningUser: boolean): Promise<void> {
  openedContexts.add(ctx)
  const version = process.env.KW_APP_VERSION ?? 'kernwege'
  await ctx.addInitScript(({ returning, v }) => {
    localStorage.setItem('vakt_tour_completed', '1')
    if (returning) localStorage.setItem('vakt_last_seen_version', v)
  }, { returning: returningUser, v: version })
}

export async function dismissFirstRunOverlays(page: Page): Promise<void> {
  await prepareContext(page.context(), false)
  await page.addLocatorHandler(page.getByRole('dialog', { name: /^Was ist neu in Vakt/ }), async (d) => {
    await d.getByRole('button', { name: 'Verstanden' }).click()
  }, { noWaitAfter: true })
}

/**
 * Die ganze /auth-Gruppe (login, refresh, logout) teilt sich 5 Anfragen pro Minute und
 * IP (cmd/api/routes.go, authRateLimiter). Alle Tests kommen von einer IP — ohne
 * Abstand liefe die Suite in 429 „Zu viele Anfragen". Ein Mensch meldet sich nicht
 * sechsmal pro Minute an; die Tests halten deshalb ≥ 13 s Abstand zwischen Logins
 * (und nutzen sonst gespeicherte Sitzungen, siehe sessionFile). Dass mehrere Menschen
 * hinter EINER IP sich gegenseitig aussperren, prüft R1-KW-4 in 20-anmeldung-rechte.
 */
async function throttleAuth(page: Page): Promise<void> {
  const f = path.join(KW.dir, 'last-auth-request')
  const last = fs.existsSync(f) ? Number(fs.readFileSync(f, 'utf8')) : 0
  const wait = last + 13_000 - Date.now()
  if (wait > 0) await page.waitForTimeout(wait)
  fs.writeFileSync(f, String(Date.now()))
}

/** Wartet, bis das 1-Minuten-Fenster des Limiters seit der letzten Auth-Anfrage sicher abgelaufen ist. */
export async function waitForFreshAuthWindow(): Promise<void> {
  const f = path.join(KW.dir, 'last-auth-request')
  const last = fs.existsSync(f) ? Number(fs.readFileSync(f, 'utf8')) : 0
  const wait = last + 61_000 - Date.now()
  if (wait > 0) await new Promise((r) => setTimeout(r, wait))
}

/** Für Logins, die ein Test von Hand durchklickt (statt über login()). */
export function noteAuthRequest(): void {
  fs.writeFileSync(path.join(KW.dir, 'last-auth-request'), String(Date.now()))
}

export function markAuthBudgetUsed(until: number): void {
  fs.writeFileSync(path.join(KW.dir, 'last-auth-request'), String(until - 13_000))
}

/** Gespeicherte Sitzung je Rolle — spart Logins (Rate-Limit, s. throttleAuth). */
export function sessionFile(role: 'admin' | 'viewer'): string {
  return path.join(KW.dir, `session-${role}.json`)
}

export async function saveSession(page: Page, role: 'admin' | 'viewer'): Promise<void> {
  await page.context().storageState({ path: sessionFile(role) })
}

export async function login(page: Page, email: string, password: string, totpSecret?: string): Promise<void> {
  await throttleAuth(page)
  await page.goto('/login')
  await page.getByLabel('E-Mail').fill(email)
  await page.getByLabel('Passwort').fill(password)
  await page.getByRole('button', { name: 'Anmelden' }).click()
  if (totpSecret) {
    const code = page.getByLabel('Authentifizierungscode')
    await expect(code).toBeVisible()
    await code.fill(totp(totpSecret))
    await page.getByRole('button', { name: 'Bestätigen' }).click()
  }
  // Nach dem ersten Login einer Sitzung legt sich „Was ist neu" über das Dashboard
  // (aria-modal — der Rest der Seite ist dann für Screenreader und getByRole
  // unsichtbar). Ein Mensch klickt „Verstanden"; genau das hier, bis das Dashboard da ist.
  const dashboard = page.getByRole('heading', { name: 'Dashboard', level: 1 })
  const whatsNew = page.getByRole('dialog', { name: /^Was ist neu in Vakt/ })
  // Wegklicken mit kurzem Timeout: der Locator-Handler aus dismissFirstRunOverlays kann
  // dem Klick zuvorkommen — dann ist der Knopf schon weg, und das ist in Ordnung.
  const ack = () => whatsNew.getByRole('button', { name: 'Verstanden' }).click({ timeout: 2000 }).catch(() => undefined)
  await expect(async () => {
    if (await whatsNew.isVisible()) await ack()
    await expect(dashboard).toBeVisible({ timeout: 1000 })
  }).toPass({ timeout: 20_000 })
  // „Was ist neu" kommt erst, wenn /update-check geantwortet hat — oft NACH dem
  // Dashboard. Kurz darauf warten und wegklicken, damit die gespeicherte Sitzung den
  // Merker trägt.
  if (await whatsNew.waitFor({ state: 'visible', timeout: 4000 }).then(() => true, () => false)) {
    await ack()
    await expect(whatsNew).toBeHidden()
  }
}

/** Neue Browser-Sitzung mit der gespeicherten Anmeldung einer Rolle (kein neuer Login). */
export async function openAs(browser: Browser, role: 'admin' | 'viewer'): Promise<Page> {
  const ctx = await browser.newContext({ storageState: sessionFile(role) })
  await prepareContext(ctx, true)
  const page = await ctx.newPage()
  await page.addLocatorHandler(page.getByRole('dialog', { name: /^Was ist neu in Vakt/ }), async (d) => {
    await d.getByRole('button', { name: 'Verstanden' }).click()
  }, { noWaitAfter: true })
  return page
}

/**
 * „Speichern" in einem Dialog, bis er sich schließt. Die API begrenzt jede Organisation
 * auf 300 Anfragen pro Minute (OrgRateLimitRedis); jeder Seitenaufruf der SPA kostet
 * 20–30 davon. Ein Test klickt schneller als ein Mensch und läuft gelegentlich in 429 —
 * die Oberfläche zeigt dann einen Fehlerhinweis, der Dialog bleibt offen. Ein Mensch
 * klickt nach einem Moment noch einmal; genau das, höchstens bis das Minutenfenster
 * vorbei ist.
 */
export async function saveDialogUntilClosed(dialog: Locator, buttonName = 'Speichern'): Promise<void> {
  await expect(async () => {
    if (await dialog.isVisible()) await dialog.getByRole('button', { name: buttonName, exact: true }).click({ timeout: 3000 })
    await expect(dialog).toBeHidden({ timeout: 3000 })
  }).toPass({ timeout: 75_000, intervals: [2_000, 5_000, 10_000] })
}

/** Framework-Übersicht → Katalogkarte „ISO/IEC 27001:2022" → „Anzeigen" → Detailseite. */
export async function openIsoDetail(page: Page): Promise<void> {
  await page.goto('/vaktcomply/frameworks')
  const heading = page.getByRole('heading', { name: 'ISO27001 Details', level: 1 })
  await expect(async () => {
    if (!(await heading.isVisible())) {
      await page.getByText('ISO/IEC 27001:2022', { exact: true }).locator('xpath=ancestor::div[.//button][1]')
        .getByRole('button', { name: 'Anzeigen', exact: true }).click({ timeout: 5000 })
    }
    await expect(heading).toBeVisible({ timeout: 5000 })
  }).toPass({ timeout: 45_000 })
}

/**
 * Öffnet eine Kontrolle aus der Framework-Detailansicht (Klick auf den Titel in der
 * Tabelle). Der Klick trifft gelegentlich die Tabelle, während sie nach dem Laden noch
 * einmal rendert — dann passiert nichts, und ein Mensch klickt ein zweites Mal. Genau das.
 */
export async function openControlFromTable(page: Page, title: string): Promise<void> {
  const heading = page.getByRole('heading', { name: title, level: 1 })
  await expect(async () => {
    if (!(await heading.isVisible())) await page.getByRole('cell', { name: title, exact: true }).click({ timeout: 3000 })
    await expect(heading).toBeVisible({ timeout: 3000 })
  }).toPass({ timeout: 30_000 })
}

/** Schreib-Request mit der Sitzung des Browsers (Cookie + CSRF-Double-Submit wie apiFetch). */
export async function apiWrite(
  context: BrowserContext,
  method: 'POST' | 'PUT' | 'PATCH' | 'DELETE',
  url: string,
  data?: unknown,
): Promise<APIResponse> {
  const csrf = (await context.cookies()).find((c) => c.name === 'csrf_token')?.value ?? ''
  return context.request.fetch(url, { method, data, headers: { 'X-CSRF-Token': csrf } })
}

// ── TOTP (RFC 6238) — was die Authenticator-App rechnet ─────────────────────
function base32Decode(s: string): Buffer {
  const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567'
  const clean = s.replace(/=+$/, '').replace(/\s+/g, '').toUpperCase()
  let bits = 0
  let value = 0
  const out: number[] = []
  for (const ch of clean) {
    const idx = alphabet.indexOf(ch)
    if (idx < 0) throw new Error(`base32: ungültiges Zeichen ${ch}`)
    value = (value << 5) | idx
    bits += 5
    if (bits >= 8) {
      out.push((value >>> (bits - 8)) & 0xff)
      bits -= 8
    }
  }
  return Buffer.from(out)
}

export function totp(secret: string, at = Date.now()): string {
  const counter = Buffer.alloc(8)
  counter.writeBigUInt64BE(BigInt(Math.floor(at / 1000 / 30)))
  const h = crypto.createHmac('sha1', base32Decode(secret)).update(counter).digest()
  const off = h[h.length - 1] & 0x0f
  const code = (h.readUInt32BE(off) & 0x7fffffff) % 1_000_000
  return code.toString().padStart(6, '0')
}

// ── Mailpit (lokale SMTP-Senke statt echtem Versand) ────────────────────────
interface MailSummary { ID: string; Subject: string; To: { Address: string }[] }

export async function waitForMail(page: Page, to: string, timeoutMs = 20_000): Promise<{ subject: string; text: string; html: string }> {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    const r = await page.request.get(`${KW.mailURL}/api/v1/search?query=${encodeURIComponent(`to:${to}`)}`)
    if (r.ok()) {
      const body = (await r.json()) as { messages: MailSummary[] }
      const first = body.messages[0]
      if (first) {
        const m = await page.request.get(`${KW.mailURL}/api/v1/message/${first.ID}`)
        const msg = (await m.json()) as { Subject: string; Text: string; HTML: string }
        return { subject: msg.Subject, text: msg.Text, html: msg.HTML }
      }
    }
    await page.waitForTimeout(1000)
  }
  throw new Error(`Keine Mail an ${to} in Mailpit nach ${timeoutMs / 1000} s`)
}

// ── Lizenzschlüssel (Format wie internal/license/sign.go) ───────────────────
export interface LicensePayload {
  tier: 'pro' | 'community'
  features: string[]
  org: string
  iat: number
  exp?: number
  rt?: string
}

export function signLicense(p: LicensePayload): string {
  const priv = fs.readFileSync(path.join(KW.dir, 'license-priv.pem'), 'utf8')
  const payloadB64 = Buffer.from(JSON.stringify(p)).toString('base64url')
  // license.go: ECDSA über SHA-256(payloadB64), Signatur als rohes r||s (64 Byte).
  const sig = crypto.sign('sha256', Buffer.from(payloadB64), { key: priv, dsaEncoding: 'ieee-p1363' })
  return `${payloadB64}.${sig.toString('base64url')}`
}

export function readProKey(): string {
  return fs.readFileSync(path.join(KW.dir, 'license-pro.key'), 'utf8').trim()
}

// ── Stack-Steuerung (Neustart der API für Lizenz-Verlängerung, Backup/Restore) ─
export function compose(args: string[], opts: { input?: string } = {}): string {
  const [cmd, ...base] = KW.compose
  return execFileSync(cmd, [...base, ...args], { encoding: 'utf8', input: opts.input, stdio: ['pipe', 'pipe', 'pipe'] })
}

export async function waitForHealthy(page: Page, timeoutMs = 90_000): Promise<void> {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    try {
      const r = await page.request.get('/health', { timeout: 3000 })
      if (r.ok()) return
    } catch {
      // API startet noch
    }
    await page.waitForTimeout(1000)
  }
  throw new Error('API wurde nach dem Neustart nicht wieder gesund')
}
