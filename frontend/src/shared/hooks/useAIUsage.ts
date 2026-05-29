import { useQuery } from '@tanstack/react-query'
import { apiFetch } from '../../api/client'

interface AIUsage {
  used: number
  limit: number
  is_pro: boolean
}

/** Returns current CE monthly AI request count (25/month limit). Pro orgs get is_pro=true. */
export function useAIUsage() {
  return useQuery<AIUsage>({
    queryKey: ['ai-usage'],
    queryFn: () => apiFetch<AIUsage>('/vaktcomply/ai/usage'),
    staleTime: 60 * 1000, // refresh every minute
    retry: false,
  })
}
