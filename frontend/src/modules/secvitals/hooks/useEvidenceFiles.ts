import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'

export interface EvidenceFile {
  id: string
  evidence_id: string
  control_id: string
  original_name: string
  mime_type: string
  size_bytes: number
  uploaded_by: string
  created_at: string
  download_url: string
}

export function useEvidenceFiles(evidenceId: string | undefined) {
  return useQuery<EvidenceFile[]>({
    queryKey: ['secvitals', 'evidence', evidenceId, 'files'],
    queryFn: () => apiFetch<EvidenceFile[]>(`/secvitals/evidence/${evidenceId}/files`),
    enabled: !!evidenceId,
    staleTime: 5 * 60 * 1000,
  })
}

export function useEvidenceFilesByControl(controlId: string | undefined) {
  return useQuery<EvidenceFile[]>({
    queryKey: ['secvitals', 'controls', controlId, 'evidence-files'],
    queryFn: () => apiFetch<EvidenceFile[]>(`/secvitals/controls/${controlId}/evidence-files`),
    enabled: !!controlId,
    staleTime: 5 * 60 * 1000,
  })
}

export function useUploadEvidenceFile(controlId: string) {
  const queryClient = useQueryClient()
  return useMutation<EvidenceFile, Error, File>({
    mutationFn: (file: File) => {
      const formData = new FormData()
      formData.append('file', file)
      return fetch(`/api/v1/secvitals/controls/${controlId}/evidence-files`, {
        method: 'POST',
        credentials: 'include',
        body: formData,
      }).then(async (res) => {
        if (!res.ok) {
          const err = await res.json().catch(() => ({ error: res.statusText }))
          throw new Error((err as { error?: string }).error ?? res.statusText)
        }
        return res.json() as Promise<EvidenceFile>
      })
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ['secvitals', 'controls', controlId, 'evidence-files'],
      })
    },
  })
}

export function useDeleteEvidenceFile(controlId: string) {
  const queryClient = useQueryClient()
  return useMutation<void, Error, string>({
    mutationFn: (fileId: string) =>
      apiFetch<void>(`/secvitals/evidence-files/${fileId}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ['secvitals', 'controls', controlId, 'evidence-files'],
      })
    },
  })
}
