import { useQuery } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'

interface AIStatus {
  available: boolean
  model: string
}

/** Checks whether the AI provider is reachable via GET /secvitals/ai/status.
 *  Returns available=false when the provider is disabled or unreachable.
 *  Cached for 60 seconds — status rarely changes during a session. */
export function useAIStatus() {
  return useQuery<AIStatus>({
    queryKey: ['ai', 'status'],
    queryFn: async () => {
      try {
        return await apiFetch<AIStatus>('/secvitals/ai/status')
      } catch {
        // 404 means AI routes not registered (provider=disabled) — treat as unavailable
        return { available: false, model: '' }
      }
    },
    staleTime: 60_000,
    // Never throw — the component handles the unavailable state gracefully
    retry: false,
  })
}
