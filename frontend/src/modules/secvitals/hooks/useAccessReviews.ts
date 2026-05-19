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
    queryKey: ['secvitals', 'access-reviews'],
    queryFn: () => apiFetch<AccessReviewCampaign[]>('/secvitals/access-reviews'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useAccessReviewCampaign(id: string) {
  return useQuery<AccessReviewCampaign>({
    queryKey: ['secvitals', 'access-reviews', id],
    queryFn: () => apiFetch<AccessReviewCampaign>(`/secvitals/access-reviews/${id}`),
    enabled: !!id,
    staleTime: 5 * 60 * 1000,
  })
}

export function useCreateAccessReviewCampaign() {
  const queryClient = useQueryClient()
  return useMutation<AccessReviewCampaign, Error, CreateAccessReviewCampaignInput>({
    mutationFn: (input) =>
      apiFetch<AccessReviewCampaign>('/secvitals/access-reviews', {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'access-reviews'] })
    },
  })
}

export function useUpdateAccessReviewCampaign() {
  const queryClient = useQueryClient()
  return useMutation<AccessReviewCampaign, Error, { id: string; input: UpdateAccessReviewCampaignInput }>({
    mutationFn: ({ id, input }) =>
      apiFetch<AccessReviewCampaign>(`/secvitals/access-reviews/${id}`, {
        method: 'PUT',
        body: JSON.stringify(input),
      }),
    onSuccess: (_data, { id }) => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'access-reviews'] })
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'access-reviews', id] })
    },
  })
}

export function useDeleteAccessReviewCampaign() {
  const queryClient = useQueryClient()
  return useMutation<void, Error, string>({
    mutationFn: (id) =>
      apiFetch<void>(`/secvitals/access-reviews/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'access-reviews'] })
    },
  })
}

export function useAccessReviewItems(campaignId: string) {
  return useQuery<AccessReviewItem[]>({
    queryKey: ['secvitals', 'access-reviews', campaignId, 'items'],
    queryFn: () => apiFetch<AccessReviewItem[]>(`/secvitals/access-reviews/${campaignId}/items`),
    enabled: !!campaignId,
    staleTime: 5 * 60 * 1000,
  })
}

export function useCreateAccessReviewItem() {
  const queryClient = useQueryClient()
  return useMutation<AccessReviewItem, Error, CreateAccessReviewItemInput>({
    mutationFn: (input) =>
      apiFetch<AccessReviewItem>(`/secvitals/access-reviews/${input.campaign_id}/items`, {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: (_data, input) => {
      void queryClient.invalidateQueries({
        queryKey: ['secvitals', 'access-reviews', input.campaign_id, 'items'],
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
      apiFetch<AccessReviewItem>(`/secvitals/access-reviews/${campaignId}/items/${itemId}`, {
        method: 'PUT',
        body: JSON.stringify(input),
      }),
    onSuccess: (_data, { campaignId }) => {
      void queryClient.invalidateQueries({
        queryKey: ['secvitals', 'access-reviews', campaignId, 'items'],
      })
    },
  })
}
