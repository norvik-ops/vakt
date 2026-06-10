// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type {
  ManagementReview,
  CreateManagementReviewInput,
  UpdateManagementReviewInputsInput,
  UpdateManagementReviewOutputsInput,
} from '../types'

const QK = ['vaktcomply', 'management-reviews'] as const

export function useManagementReviews() {
  return useQuery<ManagementReview[]>({
    queryKey: [...QK],
    queryFn: () => apiFetch<ManagementReview[]>('/vaktcomply/management-reviews'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useManagementReview(id: string) {
  return useQuery<ManagementReview>({
    queryKey: [...QK, id],
    queryFn: () => apiFetch<ManagementReview>(`/vaktcomply/management-reviews/${id}`),
    enabled: !!id,
    staleTime: 5 * 60 * 1000,
  })
}

export function useCreateManagementReview() {
  const queryClient = useQueryClient()
  return useMutation<ManagementReview, Error, CreateManagementReviewInput>({
    mutationFn: (input) =>
      apiFetch<ManagementReview>('/vaktcomply/management-reviews', {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: [...QK] })
    },
  })
}

export function useUpdateManagementReviewInputs(id: string) {
  const queryClient = useQueryClient()
  return useMutation<ManagementReview, Error, UpdateManagementReviewInputsInput>({
    mutationFn: (input) =>
      apiFetch<ManagementReview>(`/vaktcomply/management-reviews/${id}/inputs`, {
        method: 'PATCH',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: [...QK, id] })
      void queryClient.invalidateQueries({ queryKey: [...QK] })
    },
  })
}

export function useUpdateManagementReviewOutputs(id: string) {
  const queryClient = useQueryClient()
  return useMutation<ManagementReview, Error, UpdateManagementReviewOutputsInput>({
    mutationFn: (input) =>
      apiFetch<ManagementReview>(`/vaktcomply/management-reviews/${id}/outputs`, {
        method: 'PATCH',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: [...QK, id] })
      void queryClient.invalidateQueries({ queryKey: [...QK] })
    },
  })
}

export function useApproveManagementReview(id: string) {
  const queryClient = useQueryClient()
  return useMutation<ManagementReview>({
    mutationFn: () =>
      apiFetch<ManagementReview>(`/vaktcomply/management-reviews/${id}/approve`, {
        method: 'POST',
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: [...QK, id] })
      void queryClient.invalidateQueries({ queryKey: [...QK] })
    },
  })
}
