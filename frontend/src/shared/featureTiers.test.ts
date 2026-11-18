/**
 * Drift-Wächter: UNSOLD_FEATURES im Frontend == features.UnsoldFeatures im Go-Code.
 *
 * Liest die Feature-Konstanten aus backend/internal/license/license.go und die
 * Liste aus backend/internal/shared/platform/features/tiers.go. Prüft NICHT, ob
 * jede Seite, die ein unverkauftes Merkmal gatet, ProGate auch benutzt.
 */
import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { UNSOLD_FEATURES, featureLabel, isUnsoldFeature } from './featureTiers'

// vitest läuft mit dem frontend-Verzeichnis als cwd.
const BACKEND = join(process.cwd(), '..', 'backend')

function featureConstants(): Map<string, string> {
  const src = readFileSync(join(BACKEND, 'internal/license/license.go'), 'utf-8')
  const out = new Map<string, string>()
  for (const m of src.matchAll(/^\s*(Feature\w+)\s*=\s*"([^"]+)"/gm)) out.set(m[1], m[2])
  return out
}

function unsoldFromGo(): string[] {
  const src = readFileSync(join(BACKEND, 'internal/shared/platform/features/tiers.go'), 'utf-8')
  const block = /var UnsoldFeatures = \[\]string\{([\s\S]*?)\}/.exec(src)
  if (!block) throw new Error('UnsoldFeatures nicht in tiers.go gefunden')
  const consts = featureConstants()
  return [...block[1].matchAll(/\b(Feature\w+)\b/g)].map((m) => {
    const v = consts.get(m[1])
    if (!v) throw new Error(`Konstante ${m[1]} nicht in license.go gefunden`)
    return v
  })
}

describe('UNSOLD_FEATURES', () => {
  it('entspricht features.UnsoldFeatures im Go-Code', () => {
    const fromGo = unsoldFromGo()
    expect(fromGo.length).toBeGreaterThan(0)
    expect([...UNSOLD_FEATURES].sort()).toEqual([...fromGo].sort())
  })

  it('erkennt unverkaufte Merkmale, nicht aber Pro-Merkmale', () => {
    expect(isUnsoldFeature('dora')).toBe(true)
    expect(isUnsoldFeature('eu_ai_act')).toBe(false)
    expect(isUnsoldFeature(undefined)).toBe(false)
  })
})

// UX-05: Die Lizenzkarte zeigte Rohschlüssel („bsi_grundschutz", „siem_export").
// Drift-Wächter: Jede Feature-Konstante in license.go braucht einen Anzeigenamen.
// Prüft NICHT, ob der Name fachlich treffend ist.
describe('featureLabel', () => {
  it('kennt für jedes Merkmal aus license.go einen Anzeigenamen', () => {
    const keys = [...featureConstants().values()]
    expect(keys.length).toBeGreaterThan(10)
    for (const key of keys) {
      const label = featureLabel(key)
      expect(label, key).not.toBe(key)
      expect(label, key).not.toMatch(/_/)
      expect(label, key).not.toMatch(/legacy/i)
    }
  })

  it('nimmt für Framework-Merkmale den Namen aus dem Framework-Katalog', () => {
    expect(featureLabel('eu_ai_act')).toBe('EU AI Act')
    expect(featureLabel('iso_42001')).toBe('ISO 42001')
    expect(featureLabel('bsi_grundschutz')).toBe('BSI IT-Grundschutz')
  })

  it('lässt unbekannte Schlüssel stehen, statt sie zu verschlucken', () => {
    expect(featureLabel('kunden_plugin')).toBe('kunden_plugin')
  })
})
