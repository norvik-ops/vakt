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
    queryKey: ['vaktcomply', 'suppliers', filters?.criticality ?? '', filters?.assessmentStatus ?? ''],
    queryFn: () => apiFetch<Supplier[]>(`/vaktcomply/suppliers${qs}`),
    staleTime: 5 * 60 * 1000,
  })
}

export function useSupplier(id: string) {
  return useQuery<Supplier>({
    queryKey: ['vaktcomply', 'suppliers', id],
    queryFn: () => apiFetch<Supplier>(`/vaktcomply/suppliers/${id}`),
    enabled: !!id,
    staleTime: 5 * 60 * 1000,
  })
}

export function useCreateSupplier() {
  const queryClient = useQueryClient()
  return useMutation<Supplier, Error, CreateSupplierInput>({
    mutationFn: (input) =>
      apiFetch<Supplier>('/vaktcomply/suppliers', { method: 'POST', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'suppliers'] })
    },
  })
}

export function useUpdateSupplier(id: string) {
  const queryClient = useQueryClient()
  return useMutation<Supplier, Error, UpdateSupplierInput>({
    mutationFn: (input) =>
      apiFetch<Supplier>(`/vaktcomply/suppliers/${id}`, { method: 'PATCH', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'suppliers'] })
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'suppliers', id] })
    },
  })
}

export function useDeleteSupplier() {
  const queryClient = useQueryClient()
  return useMutation<undefined, Error, string>({
    mutationFn: (id) =>
      apiFetch<undefined>(`/vaktcomply/suppliers/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'suppliers'] })
    },
  })
}

export function useImportSuppliersCSV() {
  const queryClient = useQueryClient()
  return useMutation<CSVImportResult, Error, FormData>({
    // apiFetch omits Content-Type for FormData so the browser sets the
    // multipart boundary itself — see client.ts.
    mutationFn: (formData) =>
      apiFetch<CSVImportResult>('/vaktcomply/suppliers/import-csv', {
        method: 'POST',
        body: formData,
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'suppliers'] })
    },
  })
}
