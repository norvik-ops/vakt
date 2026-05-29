import { useMutation } from '@tanstack/react-query'
import { apiFetch } from '../../api/client'

/**
 * Hook into the backend AI-Copilot endpoints (Sprint 12 / P3-34).
 *
 * Two operations:
 *
 *   • draftPolicy({ topic, framework }) → { draft: string }
 *     Generates a Markdown policy draft for a given topic. The admin
 *     reviews and saves it as a regular policy. The draft is *not*
 *     stored server-side — the response is just the suggested text.
 *
 *   • incidentGuide({ summary, type }) → { guide: string }
 *     Produces a numbered response checklist from an incident summary.
 *     Includes legal deadline hints (NIS2 T+24/T+72, DSGVO 72h, DORA T+4).
 *
 * Privacy note: both endpoints call the configured AI provider — local
 * Ollama by default. If the operator points OPENAI_BASE_URL at a cloud
 * endpoint, the user-supplied text leaves the instance. By Vakt's default
 * config it does not.
 */
export function useAICopilot() {
  const draftPolicy = useMutation({
    mutationFn: async (input: { topic: string; framework?: string }) =>
      apiFetch<{ draft: string }>('/vaktcomply/ai/draft-policy', {
        method: 'POST',
        body: JSON.stringify(input),
      }),
  })

  const incidentGuide = useMutation({
    mutationFn: async (input: { summary: string; type?: string }) =>
      apiFetch<{ guide: string }>('/vaktcomply/ai/incident-guide', {
        method: 'POST',
        body: JSON.stringify(input),
      }),
  })

  return { draftPolicy, incidentGuide }
}
