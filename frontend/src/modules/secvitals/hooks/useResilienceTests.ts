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
    queryKey: ['secvitals', 'resilience-tests'],
    queryFn: () => apiFetch<ResilienceTestsResponse>('/secvitals/resilience-tests'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useResilienceTest(id: string) {
  return useQuery<ResilienceTest>({
    queryKey: ['secvitals', 'resilience-tests', id],
    queryFn: () => apiFetch<ResilienceTest>(`/secvitals/resilience-tests/${id}`),
    enabled: !!id,
    staleTime: 5 * 60 * 1000,
  })
}

export function useCreateResilienceTest() {
  const queryClient = useQueryClient()
  return useMutation<ResilienceTest, Error, CreateResilienceTestInput>({
    mutationFn: (input) =>
      apiFetch<ResilienceTest>('/secvitals/resilience-tests', {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'resilience-tests'] })
    },
  })
}

export function useUpdateResilienceTest(id: string) {
  const queryClient = useQueryClient()
  return useMutation<ResilienceTest, Error, UpdateResilienceTestInput>({
    mutationFn: (input) =>
      apiFetch<ResilienceTest>(`/secvitals/resilience-tests/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'resilience-tests'] })
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'resilience-tests', id] })
    },
  })
}

export function useDeleteResilienceTest() {
  const queryClient = useQueryClient()
  return useMutation<void, Error, string>({
    mutationFn: (id) =>
      apiFetch<void>(`/secvitals/resilience-tests/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'resilience-tests'] })
    },
  })
}

export function useUploadResilienceTestAttachment(id: string) {
  const queryClient = useQueryClient()
  return useMutation<ResilienceTest, Error, FormData>({
    mutationFn: (formData) =>
      apiFetch<ResilienceTest>(`/secvitals/resilience-tests/${id}/attachment`, {
        method: 'POST',
        body: formData,
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'resilience-tests'] })
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'resilience-tests', id] })
    },
  })
}
