import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { Supplier, CreateSupplierInput, UpdateSupplierInput, CSVImportResult } from '../types'

export interface SupplierFilters {
  criticality?: string
  assessmentStatus?: string
}

export function useSuppliers(filters?: SupplierFilters) {
  const params = new URLSearchParams()
  if (filters?.criticality) params.set('criticality', filters.criticality)
  if (filters?.assessmentStatus) params.set('assessment_status', filters.assessmentStatus)
  const qs = params.toString() ? `?${params.toString()}` : ''

  return useQuery<Supplier[]>({
    queryKey: ['secvitals', 'suppliers', filters?.criticality ?? '', filters?.assessmentStatus ?? ''],
    queryFn: () => apiFetch<Supplier[]>(`/secvitals/suppliers${qs}`),
    staleTime: 5 * 60 * 1000,
  })
}

export function useSupplier(id: string) {
  return useQuery<Supplier>({
    queryKey: ['secvitals', 'suppliers', id],
    queryFn: () => apiFetch<Supplier>(`/secvitals/suppliers/${id}`),
    enabled: !!id,
    staleTime: 5 * 60 * 1000,
  })
}

export function useCreateSupplier() {
  const queryClient = useQueryClient()
  return useMutation<Supplier, Error, CreateSupplierInput>({
    mutationFn: (input) =>
      apiFetch<Supplier>('/secvitals/suppliers', { method: 'POST', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'suppliers'] })
    },
  })
}

export function useUpdateSupplier(id: string) {
  const queryClient = useQueryClient()
  return useMutation<Supplier, Error, UpdateSupplierInput>({
    mutationFn: (input) =>
      apiFetch<Supplier>(`/secvitals/suppliers/${id}`, { method: 'PATCH', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'suppliers'] })
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'suppliers', id] })
    },
  })
}

export function useDeleteSupplier() {
  const queryClient = useQueryClient()
  return useMutation<void, Error, string>({
    mutationFn: (id) =>
      apiFetch<void>(`/secvitals/suppliers/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'suppliers'] })
    },
  })
}

export function useImportSuppliersCSV() {
  const queryClient = useQueryClient()
  return useMutation<CSVImportResult, Error, FormData>({
    mutationFn: async (formData) => {
      const res = await fetch('/api/v1/secvitals/suppliers/import-csv', {
        method: 'POST',
        credentials: 'include',
        body: formData,
        // Do NOT set Content-Type — browser sets multipart boundary automatically.
      })
      if (!res.ok) {
        const body = (await res.json().catch(() => ({}))) as { error?: string }
        throw new Error(body.error ?? `HTTP ${res.status.toString()}`)
      }
      return res.json() as Promise<CSVImportResult>
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'suppliers'] })
    },
  })
}
