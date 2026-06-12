import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider, MutationCache } from '@tanstack/react-query'
import { RouterProvider } from 'react-router-dom'
import { router } from './router'
import { useThemeStore } from './shared/stores/theme'
import { useAuthStore } from './shared/stores/auth'
import { ErrorBoundary } from './shared/components/ErrorBoundary'
import { FeatureLockedError, MFARequiredError } from './api/client'
import { toast } from './shared/hooks/useToast'
import './i18n'
import './index.css'

// Apply saved theme before first render
useThemeStore.getState().apply()

// Kick off /auth/me to rehydrate the in-memory user from the httpOnly cookie.
// Replaces the previous localStorage snapshot (audit F032 — no PII at rest).
// AuthGuard renders a spinner until this resolves.
void useAuthStore.getState().hydrate()

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { staleTime: 30_000, retry: 1 },
  },
  mutationCache: new MutationCache({
    onError(error) {
      // Pro-gate and auth errors are handled by ProGate / redirect — skip toast.
      if (error instanceof FeatureLockedError) return
      if (error instanceof MFARequiredError) return
      if (error instanceof Error && error.message === 'Unauthorized') return
      const msg = error instanceof Error ? error.message : 'Ein unbekannter Fehler ist aufgetreten.'
      toast(msg, 'error')
    },
  }),
})

const rootElement = document.getElementById('root')
if (!rootElement) throw new Error('Root element not found')

createRoot(rootElement).render(
  <StrictMode>
    <ErrorBoundary>
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </ErrorBoundary>
  </StrictMode>,
)
