import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'

export interface SBOMSummary {
  id: string
  asset_id: string
  format: string
  component_count: number
  created_at: string
}

export interface ComponentSummary {
  id: string
  name: string
  version: string
  purl?: string
  eol_status: 'supported' | 'eol' | 'unknown'
  eol_date?: string
  asset_id: string
}

export interface AssetSBOMResponse {
  sbom: SBOMSummary
  components: ComponentSummary[]
}

export interface EOLDashboardResponse {
  data: ComponentSummary[]
}

export function useAssetSBOM(assetId: string) {
  return useQuery<AssetSBOMResponse>({
    queryKey: ['secpulse', 'sbom', assetId],
    queryFn: () => apiFetch<AssetSBOMResponse>(`/secpulse/assets/${assetId}/sbom`),
    staleTime: 60_000,
    enabled: Boolean(assetId),
  })
}

export function useTriggerSBOM(assetId: string) {
  const queryClient = useQueryClient()
  return useMutation<void, Error, void>({
    mutationFn: () =>
      apiFetch<void>(`/secpulse/assets/${assetId}/sbom`, { method: 'POST' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secpulse', 'sbom', assetId] })
    },
  })
}

export function useEOLDashboard(eolOnly = false) {
  return useQuery<EOLDashboardResponse>({
    queryKey: ['secpulse', 'eol-dashboard', eolOnly],
    queryFn: () =>
      apiFetch<EOLDashboardResponse>(
        `/secpulse/sbom/eol${eolOnly ? '?eol_only=true' : ''}`,
      ),
    staleTime: 60_000,
  })
}
