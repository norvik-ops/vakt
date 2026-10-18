import { create } from 'zustand'
import { apiFetch } from '../api/client'

// Welche Module die Oberfläche zeigt, entscheidet allein der Server: GET /modules liefert die
// Module aus VAKT_MODULES_ENABLED. Ein Modul, das dort fehlt, hat auch keine API-Routen — es
// erscheint nirgends (Navigation, Routen, Dashboard, Suche, Verweise). Eine zweite Liste im
// Frontend gibt es nicht.

const CORE_MODULE = 'vaktcomply'

const MODULE_PATH = /^\/(vaktscan|vaktvault|vaktaware|vaktprivacy|vakthr)(?=[/?#]|$)/

interface EnabledModulesState {
  // null, bis GET /modules geantwortet hat. AuthGuard rendert die App erst danach.
  modules: ReadonlySet<string> | null
  load: () => Promise<void>
}

export const useEnabledModules = create<EnabledModulesState>((set) => ({
  modules: null,
  load: async () => {
    try {
      const res = await apiFetch<{ modules: string[] }>('/modules')
      set({ modules: new Set(res.modules) })
    } catch {
      // Ohne Antwort zeigt die App nur den ISMS-Kern statt Seiten, deren API womöglich fehlt.
      // Ein Neuladen holt die Liste erneut.
      set({ modules: new Set([CORE_MODULE]) })
    }
  },
}))

function enabledModules(): ReadonlySet<string> {
  // Vor dem Laden (nur in Unit-Tests erreichbar, weil AuthGuard wartet) gilt der ISMS-Kern.
  return useEnabledModules.getState().modules ?? new Set([CORE_MODULE])
}

/** isPathVisible sagt, ob ein App-Pfad in der Oberfläche erscheinen darf (Navigation, Links, Routen). */
export function isPathVisible(path: string): boolean {
  const m = MODULE_PATH.exec(path)
  return !m || enabledModules().has(m[1])
}

/** isModuleVisible ist die Variante für Modulschlüssel wie „vaktprivacy". */
export function isModuleVisible(module: string): boolean {
  return isPathVisible(`/${module}`)
}
