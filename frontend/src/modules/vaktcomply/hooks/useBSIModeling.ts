// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type {
  BSIModelingEntry,
  BSIModelingStats,
  CreateBSIModelingInput,
  UpdateBSIModelingInput,
} from '../types'

const QK = ['vaktcomply', 'bsi-modeling'] as const

export function useBSIModelingMatrix() {
  return useQuery<BSIModelingEntry[]>({
    queryKey: [...QK, 'matrix'],
    queryFn: () => apiFetch<BSIModelingEntry[]>('/vaktcomply/bsi-modeling'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useBSIModelingStats() {
  return useQuery<BSIModelingStats>({
    queryKey: [...QK, 'stats'],
    queryFn: () => apiFetch<BSIModelingStats>('/vaktcomply/bsi-modeling/stats'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useCreateBSIModeling() {
  const queryClient = useQueryClient()
  return useMutation<BSIModelingEntry, Error, CreateBSIModelingInput>({
    mutationFn: (input) =>
      apiFetch<BSIModelingEntry>('/vaktcomply/bsi-modeling', {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: [...QK] })
    },
  })
}

export function useUpdateBSIModeling(id: string) {
  const queryClient = useQueryClient()
  return useMutation<BSIModelingEntry, Error, UpdateBSIModelingInput>({
    mutationFn: (input) =>
      apiFetch<BSIModelingEntry>(`/vaktcomply/bsi-modeling/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: [...QK] })
    },
  })
}

export function useDeleteBSIModeling() {
  const queryClient = useQueryClient()
  return useMutation<undefined, Error, string>({
    mutationFn: (id) =>
      apiFetch<undefined>(`/vaktcomply/bsi-modeling/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: [...QK] })
    },
  })
}

export function useBSIBausteinSuggestions(assetType: string) {
  return useQuery<{ suggestions: string[] }>({
    queryKey: [...QK, 'suggestions', assetType],
    queryFn: () =>
      apiFetch<{ suggestions: string[] }>(
        `/vaktcomply/bsi-modeling/suggestions?asset_type=${encodeURIComponent(assetType)}`,
      ),
    staleTime: 60 * 60 * 1000, // suggestions rarely change
    enabled: assetType.length > 0,
  })
}
