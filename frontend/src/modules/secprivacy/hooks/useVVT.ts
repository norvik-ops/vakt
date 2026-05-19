import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { VVTEntry, CreateVVTInput, UpdateVVTInput } from '../types'
import type { PaginatedResponse } from '../../../shared/types/pagination'

export function useVVT(page = 1, limit = 25) {
  const query = useQuery<PaginatedResponse<VVTEntry>>({
    queryKey: ['secprivacy', 'vvt', page, limit],
    queryFn: () => apiFetch<PaginatedResponse<VVTEntry>>(`/secprivacy/vvt?page=${page}&limit=${limit}`),
    staleTime: 5 * 60 * 1000,
  })
  return {
    ...query,
    data: query.data?.data,
    pagination: query.data?.pagination,
  }
}

export function useCreateVVT() {
  const queryClient = useQueryClient()
  return useMutation<VVTEntry, Error, CreateVVTInput>({
    mutationFn: (input) =>
      apiFetch<VVTEntry>('/secprivacy/vvt', { method: 'POST', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secprivacy', 'vvt'] })
    },
  })
}

export function useUpdateVVT() {
  const queryClient = useQueryClient()
  return useMutation<VVTEntry, Error, { id: string; input: UpdateVVTInput }>({
    mutationFn: ({ id, input }) =>
      apiFetch<VVTEntry>(`/secprivacy/vvt/${id}`, { method: 'PUT', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secprivacy', 'vvt'] })
    },
  })
}

export function useDeleteVVT() {
  const queryClient = useQueryClient()
  return useMutation<void, Error, string>({
    mutationFn: (id) => apiFetch<void>(`/secprivacy/vvt/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secprivacy', 'vvt'] })
    },
  })
}

export function useExportVVT() {
  return () => {
    const url = '/api/v1/secprivacy/vvt/export'
    const a = document.createElement('a')
    void fetch(url, { credentials: 'include' })
      .then((res) => res.blob())
      .then((blob) => {
        const objectUrl = URL.createObjectURL(blob)
        a.href = objectUrl
        a.download = `vvt-export-${new Date().toISOString().slice(0, 10)}.pdf`
        document.body.appendChild(a)
        a.click()
        a.remove()
        URL.revokeObjectURL(objectUrl)
      })
  }
}
