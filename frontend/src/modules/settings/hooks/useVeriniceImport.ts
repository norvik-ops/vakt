import { useMutation, useQueryClient } from '@tanstack/react-query'

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

function csrfToken(): string {
  const m = document.cookie.match(/(?:^|;\s*)csrf_token=([^;]+)/)
  return m ? decodeURIComponent(m[1]) : ''
}

async function uploadVNA<T>(path: string, file: File): Promise<T> {
  const fd = new FormData()
  fd.append('file', file)
  const res = await fetch(`/api/v1${path}`, {
    method: 'POST',
    credentials: 'include',
    headers: { 'X-CSRF-Token': csrfToken() },
    body: fd,
  })
  if (!res.ok) {
    const err = (await res.json().catch(() => ({ error: res.statusText }))) as { error?: string }
    throw new Error(err.error ?? res.statusText)
  }
  return res.json() as Promise<T>
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
