// Fokus auf das ISMS (docs/launch-gate.md, Entscheid 2026-10-17): Die Oberfläche zeigt nur Comply.
// Scan, Vault, Aware, Privacy und HR laufen im Backend unverändert weiter (API, Daten, Jobs) — sie
// sind ausgeblendet, nicht abgeschaltet. Ein Modul kommt zurück, indem es in VISIBLE_MODULES steht.
//
// Ausnahme: Das Asset-Inventar liegt technisch in Scan (/vaktscan/assets), gehört fachlich aber zum
// ISMS (ISO 27001 A.5.9). Schutzbedarf, Risiko↔Asset und die Erste-Schritte-Liste hängen daran.

const VISIBLE_MODULES: ReadonlySet<string> = new Set(['vaktcomply'])

const ALWAYS_VISIBLE_PATHS = ['/vaktscan/assets']

const MODULE_PATH = /^\/(vaktscan|vaktvault|vaktaware|vaktprivacy|vakthr)(?=[/?#]|$)/

function underPath(path: string, base: string): boolean {
  return path === base || (path.startsWith(base) && /^[/?#]/.test(path.slice(base.length)))
}

/** isPathVisible sagt, ob ein App-Pfad in der Oberfläche erscheinen darf (Navigation, Links, Routen). */
export function isPathVisible(path: string): boolean {
  if (ALWAYS_VISIBLE_PATHS.some((base) => underPath(path, base))) return true
  const m = MODULE_PATH.exec(path)
  return !m || VISIBLE_MODULES.has(m[1])
}

/** isModuleVisible ist die Variante für Modulschlüssel wie „vaktprivacy". */
export function isModuleVisible(module: string): boolean {
  return isPathVisible(`/${module}`)
}
