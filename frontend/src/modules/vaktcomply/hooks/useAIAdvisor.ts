import { useQuery } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'

export interface AIStatus {
  available: boolean
  model: string
  /** Hostname of the configured AI provider (e.g. "ollama", "api.openai.com").
   *  LocalLLMBadge uses this to decide whether to show the green "Lokal"
   *  badge or the amber "Cloud" badge. Empty string means unknown — the
   *  badge falls back to Cloud on the safe side. */
  provider_host: string
}

/** Checks whether the AI provider is reachable via GET /vaktcomply/ai/status.
 *  Returns available=false when the provider is disabled or unreachable.
 *  Cached for 60 seconds — status rarely changes during a session. */
export function useAIStatus() {
  return useQuery<AIStatus>({
    queryKey: ['ai', 'status'],
    queryFn: async () => {
      try {
        return await apiFetch<AIStatus>('/vaktcomply/ai/status')
      } catch {
        // 404 means AI routes not registered (provider=disabled) — treat as unavailable
        return { available: false, model: '', provider_host: '' }
      }
    },
    staleTime: 60_000,
    // Never throw — the component handles the unavailable state gracefully
    retry: false,
  })
}
