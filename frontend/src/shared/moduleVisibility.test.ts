import { describe, expect, it } from 'vitest'
import { isModuleVisible, isPathVisible } from './moduleVisibility'

describe('isPathVisible', () => {
  it('zeigt Comply und Plattform-Seiten', () => {
    expect(isPathVisible('/')).toBe(true)
    expect(isPathVisible('/vaktcomply/risks')).toBe(true)
    expect(isPathVisible('/settings')).toBe(true)
  })

  it('blendet die übrigen Module aus, auch mit Query und Unterpfad', () => {
    expect(isPathVisible('/vaktscan')).toBe(false)
    expect(isPathVisible('/vaktscan/findings?severity=critical')).toBe(false)
    expect(isPathVisible('/vaktvault')).toBe(false)
    expect(isPathVisible('/vaktaware/training')).toBe(false)
    expect(isPathVisible('/vaktprivacy?filter=breach')).toBe(false)
    expect(isPathVisible('/vakthr/employees')).toBe(false)
  })

  it('lässt das Asset-Inventar sichtbar, weil das ISMS es braucht', () => {
    expect(isPathVisible('/vaktscan/assets')).toBe(true)
    expect(isPathVisible('/vaktscan/assets/123')).toBe(true)
    expect(isPathVisible('/vaktscan/assetsXYZ')).toBe(false)
  })

  it('verwechselt keine Präfixe', () => {
    expect(isPathVisible('/vaktscanner')).toBe(true)
  })
})

describe('isModuleVisible', () => {
  it('nimmt Modulschlüssel', () => {
    expect(isModuleVisible('vaktcomply')).toBe(true)
    expect(isModuleVisible('vaktprivacy')).toBe(false)
  })
})
