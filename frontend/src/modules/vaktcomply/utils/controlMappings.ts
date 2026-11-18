import type { ControlMapping } from '../hooks/useControlMappings'
import { frameworkDisplayName } from './frameworkCatalogue'

export type MappingType = ControlMapping['mapping_type']

/** Reihenfolge = Stärke: gleichwertig vor teilweise vor informativ. */
export const MAPPING_TYPES: readonly MappingType[] = ['equivalent', 'partial', 'informative']

/**
 * UX-18: Dieselbe Zielkontrolle kam mehrfach zurück und stand als zwei gleiche
 * Chips „NIS2 NIS2-A.1" da. Eine Zielkontrolle = ein Eintrag; bei verschiedenen
 * Arten gewinnt die stärkere.
 */
export function dedupeMappings(mappings: ControlMapping[]): ControlMapping[] {
  const byTarget = new Map<string, ControlMapping>()
  for (const m of mappings) {
    const key = `${m.target_framework}\u0000${m.target_control_code}`
    const prev = byTarget.get(key)
    if (!prev || MAPPING_TYPES.indexOf(m.mapping_type) < MAPPING_TYPES.indexOf(prev.mapping_type)) {
      byTarget.set(key, m)
    }
  }
  return Array.from(byTarget.values())
}

/**
 * Chip-Text. „NIS2 NIS2-A.1" wiederholte das Framework: Beginnt der Kontrollcode
 * schon mit dem Framework-Namen, steht nur der Code da.
 */
export function mappingChipLabel(m: Pick<ControlMapping, 'target_framework' | 'target_control_code'>): string {
  const fw = frameworkDisplayName(m.target_framework)
  const code = m.target_control_code
  const norm = (s: string) => s.toLowerCase().replace(/[\s_-]/g, '')
  if (norm(code).startsWith(norm(fw)) || norm(code).startsWith(norm(m.target_framework))) return code
  return `${fw} ${code}`
}
