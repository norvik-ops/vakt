import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { AuditorLink } from '../types'

export function useAuditorLinks() {
  return useQuery<AuditorLink[]>({
    queryKey: ['secvitals', 'auditor-links'],
    queryFn: () => apiFetch<AuditorLink[]>('/secvitals/auditor-links'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useRevokeAuditorLink() {
  const queryClient = useQueryClient()
  return useMutation<void, Error, string>({
    mutationFn: (id: string) =>
      apiFetch<void>(`/secvitals/auditor-links/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'auditor-links'] })
    },
  })
}

interface CreateAuditorLinkPayload {
  expires_in_days: number
  label: string
}

interface CreateAuditorLinkResponse {
  auditor_url: string
}

export function useCreateAuditorLink(frameworkId: string) {
  const queryClient = useQueryClient()
  return useMutation<CreateAuditorLinkResponse, Error, CreateAuditorLinkPayload>({
    mutationFn: (payload) =>
      apiFetch<CreateAuditorLinkResponse>(`/secvitals/frameworks/${frameworkId}/auditor-link`, {
        method: 'POST',
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'auditor-links'] })
    },
  })
}
