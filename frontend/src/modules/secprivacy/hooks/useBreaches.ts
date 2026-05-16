import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch, getAuthToken } from '../../../api/client'
import type { Breach, CreateBreachInput, UpdateBreachInput } from '../types'
import type { PaginatedResponse } from '../../../shared/types/pagination'

export function useBreaches(page = 1, limit = 25) {
  const query = useQuery<PaginatedResponse<Breach>>({
    queryKey: ['secprivacy', 'breaches', page, limit],
    queryFn: () => apiFetch<PaginatedResponse<Breach>>(`/secprivacy/breaches?page=${page}&limit=${limit}`),
    staleTime: 5 * 60 * 1000,
  })
  return {
    ...query,
    data: query.data?.data,
    pagination: query.data?.pagination,
  }
}

export function useCreateBreach() {
  const queryClient = useQueryClient()
  return useMutation<Breach, Error, CreateBreachInput>({
    mutationFn: (input) =>
      apiFetch<Breach>('/secprivacy/breaches', { method: 'POST', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secprivacy', 'breaches'] })
    },
  })
}

export function useUpdateBreach() {
  const queryClient = useQueryClient()
  return useMutation<Breach, Error, { id: string; input: UpdateBreachInput }>({
    mutationFn: ({ id, input }) =>
      apiFetch<Breach>(`/secprivacy/breaches/${id}`, { method: 'PUT', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secprivacy', 'breaches'] })
    },
  })
}

export function useDeleteBreach() {
  const queryClient = useQueryClient()
  return useMutation<void, Error, string>({
    mutationFn: (id) => apiFetch<void>(`/secprivacy/breaches/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secprivacy', 'breaches'] })
    },
  })
}

export function useMarkAuthorityNotified() {
  const queryClient = useQueryClient()
  return useMutation<void, Error, string>({
    mutationFn: (id) =>
      apiFetch<void>(`/secprivacy/breaches/${id}/notify-authority`, { method: 'POST' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secprivacy', 'breaches'] })
    },
  })
}

export function useExportBreachNotification() {
  return (id: string) => {
    const token = getAuthToken()
    const url = `/api/v1/secprivacy/breaches/${id}/notification-pdf`
    const a = document.createElement('a')
    a.href = url
    if (token) {
      void fetch(url, { headers: { Authorization: `Bearer ${token}` } })
        .then((res) => res.blob())
        .then((blob) => {
          const objectUrl = URL.createObjectURL(blob)
          a.href = objectUrl
          a.download = `breach-notification-${new Date().toISOString().slice(0, 10)}.pdf`
          document.body.appendChild(a)
          a.click()
          a.remove()
          URL.revokeObjectURL(objectUrl)
        })
    } else {
      window.open(url, '_blank')
    }
  }
}
