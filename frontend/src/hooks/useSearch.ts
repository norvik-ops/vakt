import { useQuery } from '@tanstack/react-query'
import { apiFetch } from '../api/client'

export interface SearchResult {
  id: string
  entity_type: 'control' | 'risk' | 'policy' | 'incident' | 'capa' | 'asset' | 'finding' | 'dsr' | 'breach'
  title: string
  subtitle: string
  url: string
}

export interface SearchResponse {
  results: SearchResult[]
  total: number
}

/** Fetches cross-module search results. Disabled when query is shorter than 2 chars. */
export function useSearch(query: string) {
  return useQuery<SearchResponse>({
    queryKey: ['search', query],
    queryFn: () => apiFetch<SearchResponse>(`/search?q=${encodeURIComponent(query)}`),
    enabled: query.trim().length >= 2,
    staleTime: 10_000,
  })
}
