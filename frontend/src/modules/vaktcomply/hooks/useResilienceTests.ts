import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type {
  ResilienceTest,
  ResilienceTestsResponse,
  CreateResilienceTestInput,
  UpdateResilienceTestInput,
} from '../types'

export function useResilienceTests() {
  return useQuery<ResilienceTestsResponse>({
    queryKey: ['vaktcomply', 'resilience-tests'],
    queryFn: () => apiFetch<ResilienceTestsResponse>('/vaktcomply/resilience-tests'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useResilienceTest(id: string) {
  return useQuery<ResilienceTest>({
    queryKey: ['vaktcomply', 'resilience-tests', id],
    queryFn: () => apiFetch<ResilienceTest>(`/vaktcomply/resilience-tests/${id}`),
    enabled: !!id,
    staleTime: 5 * 60 * 1000,
  })
}

export function useCreateResilienceTest() {
  const queryClient = useQueryClient()
  return useMutation<ResilienceTest, Error, CreateResilienceTestInput>({
    mutationFn: (input) =>
      apiFetch<ResilienceTest>('/vaktcomply/resilience-tests', {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'resilience-tests'] })
    },
  })
}

export function useUpdateResilienceTest(id: string) {
  const queryClient = useQueryClient()
  return useMutation<ResilienceTest, Error, UpdateResilienceTestInput>({
    mutationFn: (input) =>
      apiFetch<ResilienceTest>(`/vaktcomply/resilience-tests/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'resilience-tests'] })
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'resilience-tests', id] })
    },
  })
}

export function useDeleteResilienceTest() {
  const queryClient = useQueryClient()
  return useMutation<undefined, Error, string>({
    mutationFn: (id) =>
      apiFetch<undefined>(`/vaktcomply/resilience-tests/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'resilience-tests'] })
    },
  })
}

export function useLinkResilienceTestAsEvidence(id: string) {
  const queryClient = useQueryClient()
  return useMutation<{ id: string }, Error, { control_id: string }>({
    mutationFn: (body) =>
      apiFetch<{ id: string }>(`/vaktcomply/resilience-tests/${id}/link-evidence`, {
        method: 'POST',
        body: JSON.stringify(body),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'resilience-tests'] })
    },
  })
}

export function useUploadResilienceTestAttachment(id: string) {
  const queryClient = useQueryClient()
  return useMutation<ResilienceTest, Error, FormData>({
    mutationFn: (formData) =>
      apiFetch<ResilienceTest>(`/vaktcomply/resilience-tests/${id}/attachment`, {
        method: 'POST',
        body: formData,
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'resilience-tests'] })
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'resilience-tests', id] })
    },
  })
}
