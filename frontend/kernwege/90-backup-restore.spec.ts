import fs from 'node:fs'
import path from 'node:path'
import { execFileSync } from 'node:child_process'
import { test, expect } from '@playwright/test'
import { KW, dismissFirstRunOverlays, loadState, login, openAs, runToken, closeContexts } from './lib/kw'
import { unzip } from './lib/zip'

/**
 * Kernweg 4 — Backup & Restore (docs/launch-gate.md)
 * Erfüllt, wenn: Backup läuft, Status stimmt, Restore auf leerer Instanz stellt die
 * Daten wieder her.
 *
 * Fährt die DOKUMENTIERTEN Befehle (docs/operations.md §1) gegen die Kernweg-Instanz:
 *   Backup:  bash scripts/backup.sh <Zielverzeichnis>        (im Installationsverzeichnis, .env dort)
 *   Restore: neue, leere Instanz mit identischer .env → `docker compose up -d postgres`
 *            → bash scripts/restore.sh <archiv> → `docker compose up -d`
 * Der Unterschied zum Kunden: der Compose-Projektname kommt aus COMPOSE_PROJECT_NAME
 * statt aus dem Verzeichnisnamen, und die leere Instanz läuft neben der ersten.
 */

const ROOT = path.resolve(import.meta.dirname, '..', '..')
const PROJECT = process.env.KW_PROJECT ?? 'vakt-kw'
const RESTORE_PROJECT = `${PROJECT}-restore`
const RESTORE_PORT = String(Number(process.env.KW_HTTP_PORT ?? '18480') + 1)
const passphrase = () => runToken('backup-passphrase', 18)

function sh(cmd: string, args: string[], env: Record<string, string> = {}, input?: string): string {
  return execFileSync(cmd, args, {
    cwd: KW.dir, // Installationsverzeichnis: hier liegt die .env, die die Skripte lesen
    encoding: 'utf8',
    env: { ...process.env, ...env },
    input,
    stdio: ['pipe', 'pipe', 'pipe'],
    timeout: 300_000,
  })
}

function composeFor(project: string, args: string[], env: Record<string, string> = {}): string {
  return sh('docker', [
    'compose', '-p', project,
    '-f', path.join(ROOT, 'docker-compose.yml'), '-f', path.join(ROOT, 'scripts/kernwege/compose.kernwege.yml'),
    '--env-file', KW.envFile, ...args,
  ], env)
}

test.describe('KW4 Backup & Restore', () => {
  test.afterEach(closeContexts)

  test.describe.configure({ timeout: 360_000 })

  let archive = ''

  test('KW4 Backup nach Anleitung läuft, Archiv ist signiert, der Backup-Status im Produkt springt um', async ({ browser }) => {
    const page = await openAs(browser, 'admin')
    await page.goto('/')
    await expect(page.getByText(/Kein Backup in den letzten 7 Tagen/)).toBeVisible()

    const dir = path.join(KW.dir, 'backups')
    fs.mkdirSync(dir, { recursive: true })
    const out = sh('bash', [path.join(ROOT, 'scripts/backup.sh'), dir], {
      COMPOSE_PROJECT_NAME: PROJECT,
      VAKT_BACKUP_PASSPHRASE: passphrase(),
    })
    expect(out).toMatch(/backup_log aktualisiert/)
    const files = fs.readdirSync(dir)
    const tar = files.find((f) => /^vakt-backup-.*\.tar\.gz$/.test(f))
    expect(tar, `Archiv in ${dir}: ${files.join(', ')}`).toBeTruthy()
    expect(files).toContain(`${tar}.sig`)
    archive = path.join(dir, tar!)
    fs.writeFileSync(path.join(KW.dir, 'backup-archive'), archive)

    // Dry-Run aus der Anleitung: Signatur + Schlüssel prüfbar, ohne etwas anzufassen.
    const dry = sh('bash', [path.join(ROOT, 'scripts/restore.sh'), archive, '--dry-run'], { VAKT_BACKUP_PASSPHRASE: passphrase() })
    expect(dry).toMatch(/Signature valid/)
    expect(dry).toMatch(/Recovered key matches/)

    await page.reload()
    await expect(page.getByRole('heading', { name: 'Dashboard', level: 1 })).toBeVisible()
    await expect(page.getByText(/Kein Backup in den letzten 7 Tagen/)).toHaveCount(0)
  })

  test('KW4 Restore auf leerer Instanz stellt Admin, Risiko, Richtlinie und Beleg wieder her', async ({ browser }) => {
    const f = path.join(KW.dir, 'backup-archive')
    expect(fs.existsSync(f), 'Archiv aus dem vorigen Test').toBe(true)
    archive = fs.readFileSync(f, 'utf8').trim()
    const s = loadState()
    const env = { KW_HTTP_PORT: RESTORE_PORT }
    try {
      // Neue, leere Instanz: nur die Datenbank starten (operations.md Schritt 4) — und
      // warten, bis sie läuft („damit die Datenbank läuft"). Ohne Warten findet
      // restore.sh den Socket noch nicht (pg_restore: „No such file or directory");
      // restore.sh selbst wartet nicht (Beobachtung im PR).
      composeFor(RESTORE_PROJECT, ['up', '-d', '--wait', '--no-build', 'postgres'], env)
      // restore.sh fragt „Continue? [y/N]" — der Betreiber antwortet mit y.
      const out = sh('bash', [path.join(ROOT, 'scripts/restore.sh'), archive], {
        COMPOSE_PROJECT_NAME: RESTORE_PROJECT,
        VAKT_BACKUP_PASSPHRASE: passphrase(),
      }, 'y\n')
      expect(out).toMatch(/Datenbank wiederhergestellt/)
      // Restlichen Stack starten (Schritt 6).
      composeFor(RESTORE_PROJECT, ['up', '-d', '--no-build', 'caddy', 'worker'], env)

      const ctx = await browser.newContext({ baseURL: `http://localhost:${RESTORE_PORT}` })
      const page = await ctx.newPage()
      await expect(async () => {
        const r = await page.request.get('/health', { timeout: 3000 })
        expect(r.ok()).toBe(true)
      }).toPass({ timeout: 120_000 })
      await dismissFirstRunOverlays(page)
      // Wiederhergestellte Instanz: kein Setup-Assistent, der Admin von vorher meldet sich an.
      await page.goto('/')
      await expect(page).toHaveURL(/\/login$/)
      await login(page, s.admin.email, s.admin.password)

      await page.goto('/vaktcomply/risks')
      await expect(page.getByText('Ransomware verschlüsselt den Fileserver')).toBeVisible()
      await page.goto('/vaktcomply/policies')
      await expect(page.getByText('Passwort- und Zugangskontrollrichtlinie').first()).toBeVisible()
      await page.goto('/vaktcomply/frameworks')
      await expect(page.getByRole('heading', { name: 'ISO27001', level: 3 })).toBeVisible()
      await expect(page.getByRole('heading', { name: 'NIS2', level: 3 })).toBeVisible()

      // Der hochgeladene Anhang (Uploads-Volume) ist mitgekommen: das Audit-Paket der
      // wiederhergestellten Instanz enthält ihn byte-genau.
      const [dl] = await Promise.all([
        page.waitForEvent('download'),
        page.getByRole('button', { name: 'Audit-Paket exportieren' }).click(),
      ])
      const zip = unzip(fs.readFileSync((await dl.path())!))
      const anhang = [...zip.entries()].find(([n]) => n === `evidence/${s.evidenceFile}`)
      expect(anhang, `evidence/${s.evidenceFile} im Paket der wiederhergestellten Instanz (${[...zip.keys()].join(', ')})`).toBeTruthy()
      expect(anhang![1].toString('latin1')).toContain(`kernwege-beleg-${runToken('beleg')}-anhang`)
      await ctx.close()
    } finally {
      composeFor(RESTORE_PROJECT, ['down', '-v', '--remove-orphans'], env)
    }
  })
})
