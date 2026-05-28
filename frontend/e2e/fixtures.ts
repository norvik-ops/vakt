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
        if (url.endsWith('/health')) {
          return new Response(
            JSON.stringify({ demo: false, version: 'e2e-test', sso_enabled: false }),
            { status: 200, headers: { 'Content-Type': 'application/json' } },
          )
        }
        // F032: hydrate replaced the vakt_user localStorage snapshot with a
        // server-side fetch on app boot. Without this mock the SPA waits on
        // ECONNREFUSED, AuthGuard renders the spinner forever and every test
        // that doesn't explicitly visit /login times out. Mirror the same
        // identity that specs used to inject via localStorage.
        if (url.includes('/api/v1/auth/me')) {
          return new Response(
            JSON.stringify({ id: 'user-1', email: 'admin@example.com', display_name: 'Test Admin', roles: ['Admin'] }),
            { status: 200, headers: { 'Content-Type': 'application/json' } },
          )
        }
        // OnboardingWizard reads status.steps — catch-all {} causes "steps is undefined" crash
        if (url.includes('/api/v1/onboarding/status')) {
          return new Response(
            JSON.stringify({ completed: true, dismissed: true, steps: { org_configured: true, framework_selected: true, first_control_reviewed: true, first_risk_created: true } }),
            { status: 200, headers: { 'Content-Type': 'application/json' } },
          )
        }
        // NotificationBell calls .filter() on this — must be array, not paginated object
        if (url.includes('/api/v1/dashboard/notifications')) {
          return new Response(JSON.stringify([]), {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          })
        }
        // Dashboard scoreTrend guard uses .length which is undefined on objects → crash
        if (url.includes('/api/v1/secvitals/score-history')) {
          return new Response(JSON.stringify([]), {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          })
        }
        // QuickWinsCard calls .filter() on controls — must be array
        if (url.includes('/api/v1/secvitals/controls')) {
          return new Response(JSON.stringify([]), {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          })
        }
        // MyTasksWidget uses tasks.slice() — must be array
        if (url.includes('/api/v1/secvitals/my-tasks')) {
          return new Response(JSON.stringify([]), {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          })
        }
        // SLADashboardPage calls all.filter() — must be array
        if (url.includes('/api/v1/secpulse/sla-dashboard')) {
          return new Response(JSON.stringify([]), {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          })
        }
        // SecPrivacyOverviewPage calls dpias?.filter() — must be array
        if (url.includes('/api/v1/secprivacy/dpias')) {
          return new Response(JSON.stringify([]), {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          })
        }
        // SecPrivacyOverviewPage calls avvs?.filter() — must be array
        if (url.includes('/api/v1/secprivacy/avvs')) {
          return new Response(JSON.stringify([]), {
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
