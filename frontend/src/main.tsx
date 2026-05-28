import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider } from 'react-router-dom'
import { router } from './router'
import { useThemeStore } from './shared/stores/theme'
import { useAuthStore } from './shared/stores/auth'
import { ErrorBoundary } from './shared/components/ErrorBoundary'
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
