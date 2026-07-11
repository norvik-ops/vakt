import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type {
  AccessConcept,
  AccessRole,
  AccessConceptVersionSummary,
  CreateAccessConceptInput,
  UpdateAccessConceptInput,
  CreateAccessRoleInput,
  UpdateAccessRoleInput,
} from '../types'

const QK = ['vakthr', 'access-concepts'] as const

export function useAccessConcepts() {
  return useQuery<AccessConcept[]>({
    queryKey: [...QK],
    queryFn: () => apiFetch<AccessConcept[]>('/vakthr/access-concepts'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useAccessConcept(id: string) {
  return useQuery<AccessConcept>({
    queryKey: [...QK, id],
    queryFn: () => apiFetch<AccessConcept>(`/vakthr/access-concepts/${id}`),
    enabled: !!id,
    staleTime: 5 * 60 * 1000,
  })
}

export function useCreateAccessConcept() {
  const queryClient = useQueryClient()
  return useMutation<AccessConcept, Error, CreateAccessConceptInput>({
    mutationFn: (input) =>
      apiFetch<AccessConcept>('/vakthr/access-concepts', {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: [...QK] })
    },
  })
}

export function useUpdateAccessConcept(id: string) {
  const queryClient = useQueryClient()
  return useMutation<AccessConcept, Error, UpdateAccessConceptInput>({
    mutationFn: (input) =>
      // S121-C5 (C4): backend registers this as PATCH; PUT returned 404.
      apiFetch<AccessConcept>(`/vakthr/access-concepts/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: [...QK] })
      void queryClient.invalidateQueries({ queryKey: [...QK, id] })
    },
  })
}

export function useDeleteAccessConcept() {
  const queryClient = useQueryClient()
  return useMutation<undefined, Error, string>({
    mutationFn: (conceptId) =>
      apiFetch<undefined>(`/vakthr/access-concepts/${conceptId}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: [...QK] })
    },
  })
}

// --- Roles ---

export function useAccessRoles(conceptId: string) {
  return useQuery<AccessRole[]>({
    queryKey: [...QK, conceptId, 'roles'],
    queryFn: () => apiFetch<AccessRole[]>(`/vakthr/access-concepts/${conceptId}/roles`),
    enabled: !!conceptId,
    staleTime: 5 * 60 * 1000,
  })
}

export function useAddAccessRole(conceptId: string) {
  const queryClient = useQueryClient()
  return useMutation<AccessRole, Error, CreateAccessRoleInput>({
    mutationFn: (input) =>
      apiFetch<AccessRole>(`/vakthr/access-concepts/${conceptId}/roles`, {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: [...QK, conceptId, 'roles'] })
    },
  })
}

export function useUpdateAccessRole(conceptId: string, roleId: string) {
  const queryClient = useQueryClient()
  return useMutation<AccessRole, Error, UpdateAccessRoleInput>({
    mutationFn: (input) =>
      // S121-C5 (C5): backend registers this as PATCH; PUT returned 404.
      apiFetch<AccessRole>(`/vakthr/access-concepts/${conceptId}/roles/${roleId}`, {
        method: 'PATCH',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: [...QK, conceptId, 'roles'] })
    },
  })
}

export function useDeleteAccessRole(conceptId: string) {
  const queryClient = useQueryClient()
  return useMutation<undefined, Error, string>({
    mutationFn: (roleId) =>
      apiFetch<undefined>(`/vakthr/access-concepts/${conceptId}/roles/${roleId}`, {
        method: 'DELETE',
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: [...QK, conceptId, 'roles'] })
    },
  })
}

// --- Versions ---

export function useAccessConceptVersions(conceptId: string) {
  return useQuery<AccessConceptVersionSummary[]>({
    queryKey: [...QK, conceptId, 'versions'],
    queryFn: () =>
      apiFetch<AccessConceptVersionSummary[]>(`/vakthr/access-concepts/${conceptId}/versions`),
    enabled: !!conceptId,
    staleTime: 5 * 60 * 1000,
  })
}

export function useSnapshotAccessConcept(conceptId: string) {
  const queryClient = useQueryClient()
  return useMutation<AccessConceptVersionSummary>({
    mutationFn: () =>
      apiFetch<AccessConceptVersionSummary>(`/vakthr/access-concepts/${conceptId}/versions`, {
        method: 'POST',
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: [...QK, conceptId, 'versions'] })
      void queryClient.invalidateQueries({ queryKey: [...QK] })
    },
  })
}
