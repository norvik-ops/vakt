import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { ThreatCatalogItem, CreateRiskFromCatalogInput, Risk } from '../types'

export interface ThreatCatalogFilter {
  framework?: string
  asset_type?: string
  cia?: string
}

export function useThreatCatalog(filter: ThreatCatalogFilter) {
  const params = new URLSearchParams()
  if (filter.framework) params.set('framework', filter.framework)
  if (filter.asset_type) params.set('asset_type', filter.asset_type)
  if (filter.cia) params.set('cia', filter.cia)
  const qs = params.toString()
  return useQuery<ThreatCatalogItem[]>({
    queryKey: ['vaktcomply', 'threat-catalog', filter],
    queryFn: () => apiFetch<ThreatCatalogItem[]>(`/vaktcomply/threat-catalog${qs ? `?${qs}` : ''}`),
    staleTime: 10 * 60 * 1000,
  })
}

export function useCreateRiskFromCatalog() {
  const qc = useQueryClient()
  return useMutation<Risk, Error, CreateRiskFromCatalogInput>({
    mutationFn: (input) =>
      apiFetch<Risk>('/vaktcomply/threat-catalog/create-risk', {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['vaktcomply', 'risks'] })
    },
  })
}
