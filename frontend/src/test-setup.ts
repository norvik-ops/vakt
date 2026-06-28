import '@testing-library/jest-dom'
import 'vitest-axe/extend-expect'
import { expect, vi } from 'vitest'
import * as matchers from 'vitest-axe/matchers'
import './i18n'

expect.extend(matchers)

// JSDOM lacks window.matchMedia, which the theme store consumes at import time.
// Stub it globally for every test so individual files don't need their own shim.
if (typeof window !== 'undefined' && typeof window.matchMedia !== 'function') {
  window.matchMedia = vi.fn().mockImplementation((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }))
}
