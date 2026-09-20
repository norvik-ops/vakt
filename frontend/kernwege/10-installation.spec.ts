import fs from 'node:fs'
import path from 'node:path'
import { execFileSync } from 'node:child_process'
import { test, expect } from '@playwright/test'
import { KW, compose, closeContexts } from './lib/kw'

/**
 * Kernweg 2 — Installation & Update, Teil 1: Installation (docs/launch-gate.md)
 * Erfüllt, wenn: `docker compose up` nach der öffentlichen Anleitung startet.
 *
 * scripts/kernwege/kernwege.sh hat die .env mit den Befehlen aus README.md
 * („Quick Start") erzeugt und den Stack mit dem Kunden-docker-compose.yml gestartet.
 * Hier wird geprüft, was dabei herausgekommen ist — BEVOR jemand die Instanz
 * anfasst (diese Datei läuft als erste). Das Update steht in 95-update.spec.ts.
 */

const PLACEHOLDER = 'ERSETZEN_SIE_DIESEN_WERT'

function envFile(): Record<string, string> {
  const out: Record<string, string> = {}
  for (const line of fs.readFileSync(KW.envFile, 'utf8').split('\n')) {
    const m = /^([A-Z0-9_]+)=(.*)$/.exec(line)
    if (m) out[m[1]] = m[2].replace(/\s+#.*$/, '').trim()
  }
  return out
}

test.describe('KW2 Installation', () => {
  test.afterEach(closeContexts)

  test('KW2 README-Anleitung erzeugt eigene Geheimnisse statt der veröffentlichten Platzhalter', () => {
    const steps = fs.readFileSync(path.join(KW.dir, 'readme-steps.txt'), 'utf8')
    expect(steps).toMatch(/^cp \.env\.example \.env$/m)
    const env = envFile()
    // Sperre gegen Rückkehr von R1-SA05-D4 (GATE-2, gefixt).
    for (const k of ['VAKT_SECRET_KEY', 'POSTGRES_PASSWORD', 'REDIS_PASSWORD']) {
      expect(env[k], k).toBeTruthy()
      expect(env[k], k).not.toContain(PLACEHOLDER)
      expect(env[k].length, `${k} Länge`).toBeGreaterThanOrEqual(32)
    }
  })

  test('KW2 alle Dienste laufen, Migrationen sind durch, kein Container trägt einen Platzhalter', async () => {
    const services = () => {
      const ps = compose(['ps', '-a', '--format', '{{.Service}}\t{{.State}}\t{{.Status}}'])
      return Object.fromEntries(ps.trim().split('\n').map((l) => { const [svc, st, status] = l.split('\t'); return [svc, `${st} ${status}`] }))
    }
    // Healthchecks brauchen nach dem Start ein paar Intervalle (caddy: 10 s).
    await expect(async () => {
      const state = services()
      for (const svc of ['caddy', 'frontend', 'api', 'worker', 'postgres', 'pgbouncer', 'redis']) {
        expect(state[svc], `${svc}: ${JSON.stringify(state)}`).toMatch(/^running .*\(healthy\)/)
      }
    }).toPass({ timeout: 90_000 })
    const state = services()
    expect(state['migrate'], 'migrate läuft einmal und endet mit 0').toMatch(/^exited Exited \(0\)/)
    const logs = compose(['logs', '--no-color', 'migrate'])
    expect(logs).toMatch(/migrations applied successfully|no change/i)

    for (const svc of ['api', 'worker']) {
      const id = compose(['ps', '-q', svc]).trim()
      const env = execFileSync('docker', ['inspect', id, '--format', '{{range .Config.Env}}{{println .}}{{end}}'], { encoding: 'utf8' })
      expect(env, `${svc}: effektive Umgebung ohne Platzhalter`).not.toContain(PLACEHOLDER)
    }
  })

  test('KW2 frische Instanz antwortet, meldet sich ohne Demo und führt in den Setup-Assistenten', async ({ page }) => {
    const health = await page.request.get('/health')
    expect(health.ok()).toBe(true)
    const body = (await health.json()) as { status: string; demo: boolean; components: Record<string, { status: string }> }
    expect(body.status).toBe('ok')
    expect(body.demo).toBe(false)
    expect(body.components.db.status).toBe('ok')
    expect(body.components.redis.status).toBe('ok')

    await page.goto('/')
    await expect(page).toHaveURL(/\/setup$/)
    await expect(page.getByRole('heading', { name: /Schritt 1 von 3 — Organisation/ })).toBeVisible()
  })
})
