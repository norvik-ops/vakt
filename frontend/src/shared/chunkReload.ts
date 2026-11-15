// Chunk-Ladefehler nach einem Deploy abfangen.
//
// Warum: Seiten werden per lazy() nachgeladen. Nach einem Update liefert der
// Server die alten Chunk-Hashes nicht mehr aus (404), der neue Service Worker
// hat die alten Precaches abgeräumt. Ein schon offener Tab, der danach eine
// noch nicht geladene Seite öffnet, scheitert am dynamischen Import. Einmal
// neu laden holt die neue index.html und damit die neuen Hashes.
//
// Kein Reload-Loop: Der Zeitpunkt des letzten automatischen Reloads steht in
// sessionStorage. Kommt derselbe Fehler innerhalb von RELOAD_GUARD_MS wieder
// (echter Netzfehler, offline), wird nicht erneut geladen — die Oberfläche
// zeigt dann einen Hinweis mit „Neu laden“-Knopf.

const STORAGE_KEY = 'vakt_chunk_reload_at'
export const RELOAD_GUARD_MS = 60_000

// Meldungen der drei Browser-Engines für einen gescheiterten dynamischen
// Import, dazu Vites eigene Meldung für ein nicht ladbares CSS-Preload.
const CHUNK_ERROR_PATTERNS: readonly RegExp[] = [
  /importing a module script failed/i, // Safari / WebKit
  /failed to fetch dynamically imported module/i, // Chrome / Chromium
  /error loading dynamically imported module/i, // Firefox
  /unable to preload css/i, // Vite-Preload-Helfer
]

export function isChunkLoadError(err: unknown): boolean {
  if (!(err instanceof Error)) return false
  return CHUNK_ERROR_PATTERNS.some((re) => re.test(err.message))
}

export type ChunkReloadResult = 'reloading' | 'gave-up'

export interface ChunkReloadDeps {
  now: () => number
  getStorage: () => Pick<Storage, 'getItem' | 'setItem'>
  reload: () => void
}

const defaultDeps: ChunkReloadDeps = {
  now: () => Date.now(),
  getStorage: () => window.sessionStorage,
  reload: () => { window.location.reload() },
}

// Innerhalb eines Seitenlebens wird höchstens einmal neu geladen. Die Flagge
// sorgt dafür, dass vite:preloadError-Listener und Error-Boundaries, die
// denselben Fehler sehen, dasselbe Ergebnis bekommen.
let reloadPending = false

/**
 * Lädt die Seite genau einmal neu, um nach einem Deploy die neue Version zu
 * holen. Gibt 'gave-up' zurück, wenn kurz vorher schon automatisch neu
 * geladen wurde oder sessionStorage nicht nutzbar ist (ohne Speicher lässt
 * sich ein Loop nicht ausschließen).
 */
export function reloadOnceAfterChunkError(deps: ChunkReloadDeps = defaultDeps): ChunkReloadResult {
  if (reloadPending) return 'reloading'
  const now = deps.now()
  try {
    const storage = deps.getStorage()
    const last = Number(storage.getItem(STORAGE_KEY))
    if (Number.isFinite(last) && last > 0 && now - last < RELOAD_GUARD_MS) return 'gave-up'
    storage.setItem(STORAGE_KEY, String(now))
  } catch {
    return 'gave-up'
  }
  reloadPending = true
  deps.reload()
  return 'reloading'
}

/**
 * Vite meldet jeden gescheiterten dynamischen Import über `vite:preloadError`
 * (auch den Import selbst, nicht nur Preload-Links). Kein preventDefault: der
 * Fehler läuft weiter bis zur Error-Boundary, die bis zum Reload „Neue Version
 * wird geladen“ zeigt bzw. den Hinweis, falls kurz vorher schon neu geladen
 * wurde. Gibt eine Funktion zum Abmelden zurück.
 */
export function installChunkReloadListener(target: Window = window): () => void {
  const onPreloadError = (event: VitePreloadErrorEvent) => {
    if (isChunkLoadError(event.payload)) reloadOnceAfterChunkError()
  }
  target.addEventListener('vite:preloadError', onPreloadError)
  return () => { target.removeEventListener('vite:preloadError', onPreloadError) }
}

/** Nur für Tests: setzt den Zustand des Seitenlebens zurück. */
export function resetChunkReloadStateForTests(): void {
  reloadPending = false
}
