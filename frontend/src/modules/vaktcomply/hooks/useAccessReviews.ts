import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type {
  AccessReviewCampaign,
  AccessReviewItem,
  CreateAccessReviewCampaignInput,
  UpdateAccessReviewCampaignInput,
  CreateAccessReviewItemInput,
  UpdateAccessReviewItemInput,
} from '../types'

export function useAccessReviewCampaigns() {
  return useQuery<AccessReviewCampaign[]>({
    queryKey: ['vaktcomply', 'access-reviews'],
    queryFn: () => apiFetch<AccessReviewCampaign[]>('/vaktcomply/access-reviews'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useAccessReviewCampaign(id: string) {
  return useQuery<AccessReviewCampaign>({
    queryKey: ['vaktcomply', 'access-reviews', id],
    queryFn: () => apiFetch<AccessReviewCampaign>(`/vaktcomply/access-reviews/${id}`),
    enabled: !!id,
    staleTime: 5 * 60 * 1000,
  })
}

export function useCreateAccessReviewCampaign() {
  const queryClient = useQueryClient()
  return useMutation<AccessReviewCampaign, Error, CreateAccessReviewCampaignInput>({
    mutationFn: (input) =>
      apiFetch<AccessReviewCampaign>('/vaktcomply/access-reviews', {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'access-reviews'] })
    },
  })
}

export function useUpdateAccessReviewCampaign() {
  const queryClient = useQueryClient()
  return useMutation<AccessReviewCampaign, Error, { id: string; input: UpdateAccessReviewCampaignInput }>({
    mutationFn: ({ id, input }) =>
      apiFetch<AccessReviewCampaign>(`/vaktcomply/access-reviews/${id}`, {
        method: 'PUT',
        body: JSON.stringify(input),
      }),
    onSuccess: (_data, { id }) => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'access-reviews'] })
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'access-reviews', id] })
    },
  })
}

export function useDeleteAccessReviewCampaign() {
  const queryClient = useQueryClient()
  return useMutation<undefined, Error, string>({
    mutationFn: (id) =>
      apiFetch<undefined>(`/vaktcomply/access-reviews/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'access-reviews'] })
    },
  })
}

export function useAccessReviewItems(campaignId: string) {
  return useQuery<AccessReviewItem[]>({
    queryKey: ['vaktcomply', 'access-reviews', campaignId, 'items'],
    queryFn: () => apiFetch<AccessReviewItem[]>(`/vaktcomply/access-reviews/${campaignId}/items`),
    enabled: !!campaignId,
    staleTime: 5 * 60 * 1000,
  })
}

export function useCreateAccessReviewItem() {
  const queryClient = useQueryClient()
  return useMutation<AccessReviewItem, Error, CreateAccessReviewItemInput>({
    mutationFn: (input) =>
      apiFetch<AccessReviewItem>(`/vaktcomply/access-reviews/${input.campaign_id}/items`, {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: (_data, input) => {
      void queryClient.invalidateQueries({
        queryKey: ['vaktcomply', 'access-reviews', input.campaign_id, 'items'],
      })
    },
  })
}

export function useUpdateAccessReviewItem() {
  const queryClient = useQueryClient()
  return useMutation<
    AccessReviewItem,
    Error,
    { campaignId: string; itemId: string; input: UpdateAccessReviewItemInput }
  >({
    mutationFn: ({ campaignId, itemId, input }) =>
      apiFetch<AccessReviewItem>(`/vaktcomply/access-reviews/${campaignId}/items/${itemId}`, {
        method: 'PUT',
        body: JSON.stringify(input),
      }),
    onSuccess: (_data, { campaignId }) => {
      void queryClient.invalidateQueries({
        queryKey: ['vaktcomply', 'access-reviews', campaignId, 'items'],
      })
    },
  })
}
