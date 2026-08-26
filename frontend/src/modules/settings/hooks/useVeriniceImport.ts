import { useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'

export interface VeriniceImportPreview {
  total_objects: number
  assets: number
  controls: number
  risks: number
  unmapped: number
  unmapped_types: string[]
  sample_titles: Record<string, string[]>
}

export interface VeriniceImportResult {
  assets_created: number
  controls_created: number
  risks_created: number
  skipped: number
  framework_id?: string
}

// R1-18-D4: this is the eighth FormData upload, and it used to hand-roll both
// the request and its own csrfToken() reader — a copy that saw only
// document.cookie and so lost the in-memory fallback for proxies that rewrite
// Set-Cookie. apiFetch now omits Content-Type for FormData, so the multipart
// boundary survives and the central path handles CSRF, session and MFA.
async function uploadVNA<T>(path: string, file: File): Promise<T> {
  const fd = new FormData()
  fd.append('file', file)
  return apiFetch<T>(path, { method: 'POST', body: fd })
}

export function useVeriniceImportPreview() {
  return useMutation<VeriniceImportPreview, Error, File>({
    mutationFn: (file) => uploadVNA<VeriniceImportPreview>('/vaktcomply/verinice-import/preview', file),
  })
}

export function useVeriniceImportCommit() {
  const qc = useQueryClient()
  return useMutation<VeriniceImportResult, Error, File>({
    mutationFn: (file) => uploadVNA<VeriniceImportResult>('/vaktcomply/verinice-import/commit', file),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['vaktcomply', 'risks'] })
    },
  })
}
