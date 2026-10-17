import { afterEach, describe, expect, it, vi } from 'vitest'
import { isModuleVisible, isPathVisible, useEnabledModules } from './moduleVisibility'
import * as client from '../api/client'

function enable(...modules: string[]) {
  useEnabledModules.setState({ modules: new Set(modules) })
}

afterEach(() => {
  useEnabledModules.setState({ modules: null })
  vi.restoreAllMocks()
})

describe('isPathVisible', () => {
  it('zeigt Comply und Plattform-Seiten', () => {
    enable('vaktcomply')
    expect(isPathVisible('/')).toBe(true)
    expect(isPathVisible('/vaktcomply/risks')).toBe(true)
    expect(isPathVisible('/settings')).toBe(true)
  })

  it('blendet abgeschaltete Module aus, auch mit Query und Unterpfad', () => {
    enable('vaktcomply')
    expect(isPathVisible('/vaktscan')).toBe(false)
    expect(isPathVisible('/vaktscan/findings?severity=critical')).toBe(false)
    expect(isPathVisible('/vaktvault')).toBe(false)
    expect(isPathVisible('/vaktaware/training')).toBe(false)
    expect(isPathVisible('/vaktprivacy?filter=breach')).toBe(false)
    expect(isPathVisible('/vakthr/employees')).toBe(false)
  })

  it('zeigt Module, die der Server meldet', () => {
    enable('vaktcomply', 'vaktprivacy', 'vakthr')
    expect(isPathVisible('/vaktprivacy/vvt')).toBe(true)
    expect(isPathVisible('/vakthr/employees')).toBe(true)
    expect(isPathVisible('/vaktscan/findings')).toBe(false)
  })

  it('lässt das Asset-Inventar sichtbar, weil das ISMS es braucht', () => {
    enable('vaktcomply')
    expect(isPathVisible('/vaktscan/assets')).toBe(true)
    expect(isPathVisible('/vaktscan/assets/123')).toBe(true)
    expect(isPathVisible('/vaktscan/assetsXYZ')).toBe(false)
  })

  it('verwechselt keine Präfixe', () => {
    enable('vaktcomply')
    expect(isPathVisible('/vaktscanner')).toBe(true)
  })

  it('zeigt vor dem Laden nur den ISMS-Kern', () => {
    expect(isPathVisible('/vaktcomply')).toBe(true)
    expect(isPathVisible('/vaktscan/findings')).toBe(false)
  })
})

describe('isModuleVisible', () => {
  it('nimmt Modulschlüssel', () => {
    enable('vaktcomply')
    expect(isModuleVisible('vaktcomply')).toBe(true)
    expect(isModuleVisible('vaktprivacy')).toBe(false)
  })
})

describe('useEnabledModules.load', () => {
  it('übernimmt die Liste aus GET /modules', async () => {
    const spy = vi.spyOn(client, 'apiFetch').mockResolvedValue({ modules: ['vaktcomply', 'vaktscan'] })
    await useEnabledModules.getState().load()
    expect(spy).toHaveBeenCalledWith('/modules')
    expect(isPathVisible('/vaktscan/findings')).toBe(true)
  })

  it('fällt bei einem Fehler auf den ISMS-Kern zurück statt leer zu bleiben', async () => {
    vi.spyOn(client, 'apiFetch').mockRejectedValue(new Error('offline'))
    await useEnabledModules.getState().load()
    expect(useEnabledModules.getState().modules).toEqual(new Set(['vaktcomply']))
    expect(isPathVisible('/vaktscan/findings')).toBe(false)
  })
})
