import fs from 'node:fs'
import path from 'node:path'
import { execFileSync } from 'node:child_process'
import { test, expect, request } from '@playwright/test'
import { KW, dismissFirstRunOverlays, login, closeContexts } from './lib/kw'

/**
 * Kernweg 2 — Installation & Update, Teil 2: Update (docs/launch-gate.md)
 * Erfüllt, wenn: ein Update auf die nächste Version migriert ohne Handarbeit.
 *
 * Eigene Instanz neben der Kernweg-Instanz:
 *   1. Installation der LETZTEN VERÖFFENTLICHTEN Version (KW_FROM_TAG, z. B. v0.44.0):
 *      docker-compose.yml + Caddyfile aus diesem Git-Tag, Images von GHCR.
 *   2. Daten anlegen (Setup + ein Risiko).
 *   3. Update wie in docs/operations.md §2 „Von Hand": neue Auslieferungs-Dateien
 *      (= dieser Arbeitsbaum), neue Images (= lokal gebaut statt `docker compose
 *      pull`), `docker compose up -d` — die Migration läuft als Abhängigkeit mit.
 *   4. Prüfen: neue Version meldet sich, Anmeldung und Daten sind da.
 * Grenze: `git pull` und `docker compose pull` gegen die ECHTE nächste Version gibt
 * es erst nach dem Release — dann ist dieser Test der Beleg, dass der Weg trägt.
 */

const ROOT = path.resolve(import.meta.dirname, '..', '..')
const FROM = process.env.KW_FROM_TAG ?? ''
const PROJECT = `${process.env.KW_PROJECT ?? 'vakt-kw'}-update`
const PORT = String(Number(process.env.KW_HTTP_PORT ?? '18480') + 2)
const ADMIN = { email: 'update@kernwege.test', password: 'Kernwege-Update-2026!' }
const RISK = 'Update-Test: Daten überleben das Update'

function compose(composeFile: string, args: string[], env: Record<string, string>): string {
  return execFileSync('docker', [
    'compose', '-p', PROJECT, '-f', composeFile,
    '-f', path.join(ROOT, 'scripts/kernwege/compose.kernwege.yml'), '--env-file', KW.envFile, ...args,
  ], { encoding: 'utf8', env: { ...process.env, KW_HTTP_PORT: PORT, ...env }, stdio: ['ignore', 'pipe', 'pipe'], timeout: 600_000 })
}

async function waitVersion(version: string): Promise<void> {
  const api = await request.newContext({ baseURL: `http://localhost:${PORT}` })
  try {
    await expect(async () => {
      const r = await api.get('/health', { timeout: 3000 })
      expect(r.ok()).toBe(true)
      expect(((await r.json()) as { version: string }).version).toBe(version)
    }).toPass({ timeout: 180_000 })
  } finally {
    await api.dispose()
  }
}

test.describe('KW2 Update', () => {
  test.afterEach(closeContexts)

  test.describe.configure({ timeout: 900_000 })

  test('KW2 Update von der letzten veröffentlichten Version migriert ohne Handarbeit, Daten bleiben', async ({ browser }) => {
    test.skip(!FROM, 'Update-Phase aus (KW_UPDATE=0) oder kein v*-Tag vor HEAD — zählt als übersprungen, nicht als grün')
    const oldDir = path.join(KW.dir, 'update-from')
    fs.mkdirSync(oldDir, { recursive: true })
    for (const f of ['docker-compose.yml', 'Caddyfile']) {
      fs.writeFileSync(path.join(oldDir, f), execFileSync('git', ['-C', ROOT, 'show', `${FROM}:${f}`]))
    }
    const oldCompose = path.join(oldDir, 'docker-compose.yml')
    const oldEnv = { VAKT_TAG: FROM, KW_API_IMAGE: `ghcr.io/norvik-ops/vakt-api:${FROM}` }
    const newEnv = { VAKT_TAG: 'kernwege', KW_API_IMAGE: process.env.KW_API_IMAGE ?? 'vakt-kw-api:testkey' }
    try {
      // 1. Alte Version installieren.
      compose(oldCompose, ['pull', '--quiet', 'migrate', 'api', 'worker', 'frontend', 'vakt-scanners'], oldEnv)
      compose(oldCompose, ['up', '-d', '--no-build', 'caddy', 'worker'], oldEnv)
      await waitVersion(FROM)

      // 2. Daten anlegen — Setup und Risiko über die API (die Oberfläche prüft Kernweg 3/8).
      const api = await request.newContext({ baseURL: `http://localhost:${PORT}` })
      const setup = await api.post('/api/v1/setup', { data: { org_name: 'Update GmbH', admin_email: ADMIN.email, admin_password: ADMIN.password } })
      expect(setup.status(), await setup.text()).toBeLessThan(300)
      const li = await api.post('/api/v1/auth/login', { data: ADMIN })
      expect(li.status(), await li.text()).toBe(200)
      const csrf = (await api.storageState()).cookies.find((c) => c.name === 'csrf_token')?.value ?? ''
      const risk = await api.post('/api/v1/vaktcomply/risks', {
        data: { title: RISK, likelihood: 3, impact: 4, treatment: 'mitigate' }, headers: { 'X-CSRF-Token': csrf },
      })
      expect(risk.status(), await risk.text()).toBe(201)
      await api.dispose()

      // 3. Update: neue Auslieferungs-Dateien + neue Images, dann `docker compose up -d`.
      compose(path.join(ROOT, 'docker-compose.yml'), ['up', '-d', '--no-build', 'caddy', 'worker'], newEnv)
      await waitVersion('kernwege')
      const migrate = compose(path.join(ROOT, 'docker-compose.yml'), ['logs', '--no-color', 'migrate'], newEnv)
      expect(migrate).toMatch(/migrations applied successfully|no change/i)

      // 4. Anmeldung und Daten nach dem Update.
      const ctx = await browser.newContext({ baseURL: `http://localhost:${PORT}` })
      const page = await ctx.newPage()
      await dismissFirstRunOverlays(page)
      await login(page, ADMIN.email, ADMIN.password)
      await page.goto('/vaktcomply/risks')
      await expect(page.getByText(RISK)).toBeVisible()
      await ctx.close()
    } finally {
      try { compose(path.join(ROOT, 'docker-compose.yml'), ['down', '-v', '--remove-orphans'], newEnv) } catch { /* Abräumen best effort */ }
    }
  })
})
