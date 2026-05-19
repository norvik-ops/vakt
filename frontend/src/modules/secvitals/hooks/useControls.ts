import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { Control, Evidence, UpdateControlInput } from '../types'

export interface BulkUpdateControlsInput {
  ids: string[]
  status: 'implemented' | 'in_progress' | 'not_implemented' | 'not_applicable'
}

export function useBulkUpdateControls() {
  const queryClient = useQueryClient()
  return useMutation<void, Error, BulkUpdateControlsInput>({
    mutationFn: (data) =>
      apiFetch<void>('/secvitals/controls/bulk', {
        method: 'PATCH',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'frameworks'] })
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'controls'] })
    },
  })
}

export function useControl(controlId: string) {
  return useQuery<Control>({
    queryKey: ['secvitals', 'controls', controlId],
    queryFn: () => apiFetch<Control>(`/secvitals/controls/${controlId}`),
    enabled: !!controlId,
    staleTime: 5 * 60 * 1000,
  })
}

interface AddEvidencePayload {
  title: string
  type: 'manual' | 'automated' | 'document'
  notes?: string
  expires_at?: string | null
}

export function useAddEvidence(controlId: string) {
  const queryClient = useQueryClient()
  return useMutation<Evidence, Error, AddEvidencePayload>({
    mutationFn: (payload) =>
      apiFetch<Evidence>(`/secvitals/controls/${controlId}/evidence`, {
        method: 'POST',
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ['secvitals', 'controls', controlId, 'evidence'],
      })
    },
  })
}

export function useUploadEvidence(controlId: string) {
  const queryClient = useQueryClient()
  return useMutation<Evidence, Error, FormData>({
    mutationFn: (formData) => {
      return fetch(`/api/v1/secvitals/controls/${controlId}/evidence/upload`, {
        method: 'POST',
        credentials: 'include',
        body: formData,
      }).then(async (res) => {
        if (!res.ok) {
          const err = await res.json().catch(() => ({ error: res.statusText }))
          throw new Error((err as { error?: string }).error ?? res.statusText)
        }
        return res.json() as Promise<Evidence>
      })
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ['secvitals', 'controls', controlId, 'evidence'],
      })
    },
  })
}

export function useCollectEvidence(controlId: string) {
  const queryClient = useQueryClient()
  return useMutation<void, Error>({
    mutationFn: () =>
      apiFetch<void>(`/secvitals/controls/${controlId}/collect`, { method: 'POST' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ['secvitals', 'controls', controlId, 'evidence'],
      })
    },
  })
}

export function useUpdateControl(frameworkId: string) {
  const queryClient = useQueryClient()
  return useMutation<Control, Error, { controlId: string } & UpdateControlInput>({
    mutationFn: ({ controlId, ...input }) =>
      apiFetch<Control>(`/secvitals/controls/${controlId}`, {
        method: 'PATCH',
        body: JSON.stringify(input),
      }),
    onSuccess: (_, variables) => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'controls', variables.controlId] })
      if (frameworkId) {
        void queryClient.invalidateQueries({ queryKey: ['secvitals', 'frameworks', frameworkId, 'controls'] })
        void queryClient.invalidateQueries({ queryKey: ['secvitals', 'frameworks', frameworkId, 'report'] })
      }
    },
  })
}

export function useExportControl(controlId: string) {
  return () => {
    fetch(`/api/v1/secvitals/controls/${controlId}/export`, {
      credentials: 'include',
    })
      .then((r) => r.blob())
      .then((blob) => {
        const url = URL.createObjectURL(blob)
        const a = document.createElement('a')
        a.href = url
        a.download = `control-${controlId}.zip`
        document.body.appendChild(a)
        a.click()
        a.remove()
        URL.revokeObjectURL(url)
      })
  }
}
