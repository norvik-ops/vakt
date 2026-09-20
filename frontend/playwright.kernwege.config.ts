import { defineConfig, devices } from '@playwright/test'

/**
 * Kernwege — der automatische Prüfer der Ziellinie (docs/launch-gate.md, PROCESS.md P7c).
 *
 * Läuft NICHT gegen den Vite-Dev-Server und NICHT mit gemockten APIs (dafür ist
 * playwright.config.ts / e2e/ da), sondern gegen eine frisch gestartete Instanz aus
 * scripts/kernwege/kernwege.sh: echte Images, echtes Postgres, kein Demo-Seed.
 * Direkt aufrufen nur über `make kernwege` — die Umgebung (KW_*) setzt das Skript.
 *
 * Die Dateien laufen in Namensreihenfolge und seriell: 20-anmeldung legt den ersten
 * Admin an, alle späteren Wege arbeiten auf dieser einen Instanz weiter — wie ein
 * Kunde nach der Installation.
 */
const baseURL = process.env.KW_BASE_URL
if (!baseURL) {
  throw new Error('KW_BASE_URL fehlt — Kernwege nur über `make kernwege` starten (scripts/kernwege/kernwege.sh).')
}

export default defineConfig({
  testDir: './kernwege',
  testMatch: /\d\d-.*\.spec\.ts$/,
  fullyParallel: false,
  workers: 1,
  // Kein Retry: ein Kernweg, der nur beim zweiten Versuch grün wird, ist nicht grün.
  retries: 0,
  forbidOnly: true,
  // 180 s: Tests warten im afterEach ggf. bis zu einer Minute auf das Rate-Limit-Fenster
  // (lib/kw.ts ensureRateBudget) — das zählt zur Testzeit.
  timeout: 180_000,
  expect: { timeout: 15_000 },
  outputDir: './kernwege-results/artifacts',
  reporter: [
    ['./kernwege/reporter.ts'],
    ['html', { outputFolder: './kernwege-results/html', open: 'never' }],
  ],
  use: {
    ...devices['Desktop Chrome'],
    baseURL,
    locale: 'de-DE',
    timezoneId: 'Europe/Berlin',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    acceptDownloads: true,
  },
})
