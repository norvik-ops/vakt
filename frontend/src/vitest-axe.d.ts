import 'vitest'
import type { AxeMatchers } from 'vitest-axe/matchers'

declare module '@vitest/expect' {
  interface Assertion<T = unknown> extends AxeMatchers {}
  interface AsymmetricMatchersContaining extends AxeMatchers {}
}
