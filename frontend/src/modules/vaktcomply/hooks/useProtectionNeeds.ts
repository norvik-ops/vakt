import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type {
  ProtectionNeedAssessment,
  CreateProtectionNeedInput,
  UpdateProtectionNeedInput,
} from '../types'

const QK = ['vaktcomply', 'protection-needs'] as const

export function useProtectionNeeds() {
  return useQuery<ProtectionNeedAssessment[]>({
    queryKey: [...QK],
    queryFn: () => apiFetch<ProtectionNeedAssessment[]>('/vaktcomply/protection-needs'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useCreateProtectionNeed() {
  const queryClient = useQueryClient()
  return useMutation<ProtectionNeedAssessment, Error, CreateProtectionNeedInput>({
    mutationFn: (input) =>
      apiFetch<ProtectionNeedAssessment>('/vaktcomply/protection-needs', {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: [...QK] })
    },
  })
}

export function useUpdateProtectionNeed(id: string) {
  const queryClient = useQueryClient()
  return useMutation<ProtectionNeedAssessment, Error, UpdateProtectionNeedInput>({
    mutationFn: (input) =>
      apiFetch<ProtectionNeedAssessment>(`/vaktcomply/protection-needs/${id}`, {
        method: 'PUT',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: [...QK] })
    },
  })
}

export function useFinalizeProtectionNeed() {
  const queryClient = useQueryClient()
  return useMutation<ProtectionNeedAssessment, Error, string>({
    mutationFn: (id) =>
      apiFetch<ProtectionNeedAssessment>(`/vaktcomply/protection-needs/${id}/finalize`, {
        method: 'POST',
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: [...QK] })
    },
  })
}

export function useDeleteProtectionNeed() {
  const queryClient = useQueryClient()
  return useMutation<undefined, Error, string>({
    mutationFn: (id) =>
      apiFetch<undefined>(`/vaktcomply/protection-needs/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: [...QK] })
    },
  })
}

export function useLinkAssetToPNA() {
  const queryClient = useQueryClient()
  return useMutation<{ pna_id: string; vb_asset_id: string | null }, Error, { pnaId: string; assetId: string | null }>({
    mutationFn: ({ pnaId, assetId }) =>
      apiFetch<{ pna_id: string; vb_asset_id: string | null }>(
        `/vaktcomply/protection-needs/assessments/${pnaId}/asset-link`,
        { method: 'PATCH', body: JSON.stringify({ vb_asset_id: assetId }) },
      ),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: [...QK] })
    },
  })
}
