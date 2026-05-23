import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { Campaign, CreateCampaignInput, CampaignStats } from '../types'

const BASE = '/secreflex'

export function useCampaigns() {
  return useQuery<Campaign[]>({
    queryKey: ['secreflex', 'campaigns'],
    queryFn: async () => {
      const resp = await apiFetch<{ data: Campaign[] } | Campaign[]>(`${BASE}/campaigns`)
      return Array.isArray(resp) ? resp : (resp.data ?? [])
    },
    staleTime: 30_000,
  })
}

export function useCampaign(id: string) {
  return useQuery<Campaign>({
    queryKey: ['secreflex', 'campaigns', id],
    queryFn: () => apiFetch<Campaign>(`${BASE}/campaigns/${id}`),
    staleTime: 30_000,
    enabled: Boolean(id),
  })
}

export function useCampaignStats(id: string) {
  return useQuery<CampaignStats>({
    queryKey: ['secreflex', 'campaigns', id, 'stats'],
    queryFn: () => apiFetch<CampaignStats>(`${BASE}/campaigns/${id}/stats`),
    staleTime: 30_000,
    enabled: Boolean(id),
  })
}

export function useCreateCampaign() {
  const queryClient = useQueryClient()
  return useMutation<Campaign, Error, CreateCampaignInput>({
    mutationFn: (data) =>
      apiFetch<Campaign>(`${BASE}/campaigns`, {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secreflex', 'campaigns'] })
    },
  })
}

export function useLaunchCampaign(id: string) {
  const queryClient = useQueryClient()
  return useMutation<{ status: string }>({
    mutationFn: () =>
      apiFetch<{ status: string }>(`${BASE}/campaigns/${id}/launch`, { method: 'POST' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secreflex', 'campaigns', id] })
      void queryClient.invalidateQueries({ queryKey: ['secreflex', 'campaigns'] })
    },
  })
}

export function useAbortCampaign(id: string) {
  const queryClient = useQueryClient()
  return useMutation<{ status: string }>({
    mutationFn: () =>
      apiFetch<{ status: string }>(`${BASE}/campaigns/${id}/abort`, { method: 'POST' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secreflex', 'campaigns', id] })
      void queryClient.invalidateQueries({ queryKey: ['secreflex', 'campaigns'] })
    },
  })
}

export function useDownloadCampaignReport() {
  return (campaignId: string, campaignName?: string) => {
    void fetch(`/api/v1/secreflex/campaigns/${campaignId}/report`, {
      credentials: 'include',
    })
      .then((r) => r.blob())
      .then((blob) => {
        const url = URL.createObjectURL(blob)
        const a = document.createElement('a')
        a.href = url
        a.download = campaignName ? `${campaignName}.pdf` : `campaign-${campaignId.slice(0, 8)}.pdf`
        document.body.appendChild(a)
        a.click()
        a.remove()
        URL.revokeObjectURL(url)
      })
  }
}
