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
    queryKey: ['vaktcomply', 'evidence', evidenceId, 'files'],
    queryFn: () => apiFetch<EvidenceFile[]>(`/vaktcomply/evidence/${evidenceId}/files`),
    enabled: !!evidenceId,
    staleTime: 5 * 60 * 1000,
  })
}

export function useEvidenceFilesByControl(controlId: string | undefined) {
  return useQuery<EvidenceFile[]>({
    queryKey: ['vaktcomply', 'controls', controlId, 'evidence-files'],
    queryFn: () => apiFetch<EvidenceFile[]>(`/vaktcomply/controls/${controlId}/evidence-files`),
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
      return apiFetch<EvidenceFile>(`/vaktcomply/controls/${controlId}/evidence-files`, {
        method: 'POST',
        body: formData,
      })
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ['vaktcomply', 'controls', controlId, 'evidence-files'],
      })
    },
  })
}

export function useDeleteEvidenceFile(controlId: string) {
  const queryClient = useQueryClient()
  return useMutation<undefined, Error, string>({
    mutationFn: (fileId: string) =>
      apiFetch<undefined>(`/vaktcomply/evidence-files/${fileId}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ['vaktcomply', 'controls', controlId, 'evidence-files'],
      })
    },
  })
}
