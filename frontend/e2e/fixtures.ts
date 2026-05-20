import { test as base } from '@playwright/test'

/**
 * Shared Playwright fixture. Auto-injects baseline mocks that every spec
 * needs.
 *
 *   - `/api/v1/setup/status` returns `{ setup_complete: true }` so the
 *     SetupGuard doesn't redirect everything to `/setup`. Without this
 *     the dev-server proxy on :8080 hangs (no backend in CI), AND any
 *     spec catch-all that returns `{}` triggers the same redirect
 *     because the guard treats `setup_complete: undefined` as missing.
 *
 * Implementation: we monkey-patch `window.fetch` via `addInitScript`
 * instead of `page.route`. Route handlers run in LIFO registration
 * order — a per-spec `mockHttp` catch-all that registers later would
 * match first. The fetch override is unconditional and runs in the
 * browser before any app code, so the response is fixed regardless of
 * what the spec does afterwards.
 *
 * Specs should `import { test, expect } from './fixtures'`.
 */
export const test = base.extend({
  page: async ({ page }, use) => {
    await page.addInitScript(() => {
      const origFetch = window.fetch.bind(window)
      window.fetch = async (input, init) => {
        const url =
          typeof input === 'string'
            ? input
            : input instanceof URL
              ? input.toString()
              : input.url
        if (url.includes('/api/v1/setup/status')) {
          return new Response(JSON.stringify({ setup_complete: true }), {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          })
        }
        return origFetch(input, init)
      }
    })
    await use(page)
  },
})

export { expect } from '@playwright/test'
