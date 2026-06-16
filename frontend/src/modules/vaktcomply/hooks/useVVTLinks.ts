import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'

export interface VVTControlLink {
  id: string
  org_id: string
  vvt_id: string
  vvt_name: string
  control_id: string
  created_at: string
}

export interface LinkVVTInput {
  vvt_id: string
  vvt_name: string
  control_id: string
}

export function useControlVVTLinks(controlId: string) {
  return useQuery<VVTControlLink[]>({
    queryKey: ['vaktcomply', 'controls', controlId, 'vvt-links'],
    queryFn: () => apiFetch<VVTControlLink[]>(`/vaktcomply/controls/${controlId}/vvt-links`),
    staleTime: 60_000,
  })
}

export function useLinkVVT(controlId: string) {
  const qc = useQueryClient()
  return useMutation<VVTControlLink, Error, LinkVVTInput>({
    mutationFn: (input) =>
      apiFetch<VVTControlLink>('/vaktcomply/vvt-links', {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['vaktcomply', 'controls', controlId, 'vvt-links'] })
    },
  })
}

export function useUnlinkVVT(controlId: string) {
  const qc = useQueryClient()
  return useMutation<undefined, Error, string>({
    mutationFn: (id) => apiFetch<undefined>(`/vaktcomply/vvt-links/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['vaktcomply', 'controls', controlId, 'vvt-links'] })
    },
  })
}
