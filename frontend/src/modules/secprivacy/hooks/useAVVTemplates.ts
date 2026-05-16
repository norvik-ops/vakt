import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch, FeatureLockedError } from '../../../api/client'
import type {
  AVVTemplate,
  SCCModule,
  AVV,
  CreateAVVFromTemplateInput,
  UpdateAVVSCCInput,
} from '../types'

export function useAVVTemplates() {
  return useQuery<AVVTemplate[]>({
    queryKey: ['secprivacy', 'avv-templates'],
    queryFn: () => apiFetch<AVVTemplate[]>('/secprivacy/avv-templates'),
    staleTime: 60 * 60 * 1000, // templates are static — cache for 1 hour
  })
}

export function useSCCModules() {
  return useQuery<SCCModule[]>({
    queryKey: ['secprivacy', 'scc-modules'],
    queryFn: () => apiFetch<SCCModule[]>('/secprivacy/scc-modules'),
    staleTime: 60 * 60 * 1000,
  })
}

export function useCreateAVVFromTemplate() {
  const queryClient = useQueryClient()
  return useMutation<AVV, Error, CreateAVVFromTemplateInput>({
    mutationFn: (input) =>
      apiFetch<AVV>('/secprivacy/avvs/from-template', {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secprivacy', 'avvs'] })
    },
  })
}

export function useDownloadAVVPDF() {
  return async function downloadAVVPDF(avvId: string, filename?: string): Promise<void> {
    const { getAuthToken } = await import('../../../api/client')
    const token = getAuthToken()
    const res = await fetch(`/api/v1/secprivacy/avvs/${avvId}/pdf`, {
      headers: token ? { Authorization: `Bearer ${token}` } : {},
    })
    if (res.status === 402) {
      const body = (await res.json().catch(() => ({}))) as { feature?: string }
      throw new FeatureLockedError(body.feature ?? 'audit_pdf')
    }
    if (!res.ok) throw new Error(`PDF download failed: ${res.statusText}`)
    const blob = await res.blob()
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = filename ?? `avv-${avvId}.pdf`
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
  }
}

export function useUpdateAVVSCC() {
  const queryClient = useQueryClient()
  return useMutation<void, Error, { id: string; input: UpdateAVVSCCInput }>({
    mutationFn: ({ id, input }) =>
      apiFetch<void>(`/secprivacy/avvs/${id}/scc`, {
        method: 'PATCH',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secprivacy', 'avvs'] })
    },
  })
}

export function useDownloadSCCPDF() {
  return async function downloadSCCPDF(avvId: string, filename?: string): Promise<void> {
    const { getAuthToken } = await import('../../../api/client')
    const token = getAuthToken()
    const res = await fetch(`/api/v1/secprivacy/avvs/${avvId}/scc.pdf`, {
      headers: token ? { Authorization: `Bearer ${token}` } : {},
    })
    if (res.status === 402) {
      const body = (await res.json().catch(() => ({}))) as { feature?: string }
      throw new FeatureLockedError(body.feature ?? 'audit_pdf')
    }
    if (!res.ok) throw new Error(`SCC PDF download failed: ${res.statusText}`)
    const blob = await res.blob()
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = filename ?? `scc-${avvId}.pdf`
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
  }
}
