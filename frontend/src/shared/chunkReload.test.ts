import { describe, it, expect, beforeEach, vi } from 'vitest'
import {
  isChunkLoadError,
  reloadOnceAfterChunkError,
  resetChunkReloadStateForTests,
  RELOAD_GUARD_MS,
  type ChunkReloadDeps,
} from './chunkReload'

function memoryStorage(): Pick<Storage, 'getItem' | 'setItem'> {
  const data = new Map<string, string>()
  return {
    getItem: (k) => data.get(k) ?? null,
    setItem: (k, v) => { data.set(k, v) },
  }
}

function makeDeps(storage = memoryStorage(), start = 1_000_000) {
  let now = start
  const reload = vi.fn()
  const deps: ChunkReloadDeps = { now: () => now, getStorage: () => storage, reload }
  return { deps, reload, storage, advance: (ms: number) => { now += ms } }
}

describe('isChunkLoadError', () => {
  it.each([
    ['Safari', 'Importing a module script failed.'],
    ['Chrome', 'Failed to fetch dynamically imported module: https://x/assets/Settings-abc.js'],
    ['Firefox', 'error loading dynamically imported module: https://x/assets/Settings-abc.js'],
    ['Vite CSS-Preload', 'Unable to preload CSS for /assets/Settings-abc.css'],
  ])('erkennt die %s-Meldung', (_engine, message) => {
    expect(isChunkLoadError(new TypeError(message))).toBe(true)
  })

  it('lässt andere Fehler durch', () => {
    expect(isChunkLoadError(new Error('Cannot read properties of undefined'))).toBe(false)
    expect(isChunkLoadError(new TypeError('Failed to fetch'))).toBe(false)
  })

  it('erkennt Nicht-Error-Werte nicht als Chunk-Fehler', () => {
    expect(isChunkLoadError('Importing a module script failed.')).toBe(false)
    expect(isChunkLoadError(undefined)).toBe(false)
    expect(isChunkLoadError({ message: 'Importing a module script failed.' })).toBe(false)
  })
})

describe('reloadOnceAfterChunkError', () => {
  beforeEach(() => { resetChunkReloadStateForTests() })

  it('lädt beim ersten Chunk-Fehler genau einmal neu', () => {
    const { deps, reload } = makeDeps()
    expect(reloadOnceAfterChunkError(deps)).toBe('reloading')
    expect(reload).toHaveBeenCalledTimes(1)
  })

  it('lädt im selben Seitenleben nicht ein zweites Mal', () => {
    const { deps, reload } = makeDeps()
    reloadOnceAfterChunkError(deps)
    expect(reloadOnceAfterChunkError(deps)).toBe('reloading')
    expect(reload).toHaveBeenCalledTimes(1)
  })

  it('gibt nach dem Reload auf, wenn der Fehler sofort wiederkommt (kein Loop)', () => {
    const { deps, reload, storage, advance } = makeDeps()
    reloadOnceAfterChunkError(deps)
    // Neues Seitenleben nach dem Reload: Modulzustand weg, sessionStorage bleibt.
    resetChunkReloadStateForTests()
    advance(2_000)
    const second = makeDeps(storage, 1_002_000)
    expect(reloadOnceAfterChunkError(second.deps)).toBe('gave-up')
    expect(second.reload).not.toHaveBeenCalled()
    expect(reload).toHaveBeenCalledTimes(1)
  })

  it('lädt nach Ablauf der Sperrfrist beim nächsten Update wieder neu', () => {
    const { deps, storage } = makeDeps()
    reloadOnceAfterChunkError(deps)
    resetChunkReloadStateForTests()
    const later = makeDeps(storage, 1_000_000 + RELOAD_GUARD_MS + 1)
    expect(reloadOnceAfterChunkError(later.deps)).toBe('reloading')
    expect(later.reload).toHaveBeenCalledTimes(1)
  })

  it('lädt nicht neu, wenn sessionStorage nicht nutzbar ist', () => {
    const reload = vi.fn()
    const deps: ChunkReloadDeps = {
      now: () => 1,
      getStorage: () => { throw new DOMException('blocked', 'SecurityError') },
      reload,
    }
    expect(reloadOnceAfterChunkError(deps)).toBe('gave-up')
    expect(reload).not.toHaveBeenCalled()
  })
})
