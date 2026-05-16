import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { OrgSectorSettings, UpdateOrgSectorInput, AuthorityInfo } from '../types'

export function useOrgSector() {
  return useQuery<OrgSectorSettings>({
    queryKey: ['secvitals', 'org-sector'],
    queryFn: () => apiFetch<OrgSectorSettings>('/secvitals/org-sector'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useUpdateOrgSector() {
  const queryClient = useQueryClient()
  return useMutation<OrgSectorSettings, Error, UpdateOrgSectorInput>({
    mutationFn: (input) =>
      apiFetch<OrgSectorSettings>('/secvitals/org-sector', { method: 'PATCH', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'org-sector'] })
      void queryClient.invalidateQueries({ queryKey: ['secvitals', 'org-authorities'] })
    },
  })
}

export function useAuthorities() {
  return useQuery<AuthorityInfo[]>({
    queryKey: ['secvitals', 'authorities'],
    queryFn: () => apiFetch<AuthorityInfo[]>('/secvitals/authorities'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useOrgAuthorities() {
  return useQuery<AuthorityInfo[]>({
    queryKey: ['secvitals', 'org-authorities'],
    queryFn: () => apiFetch<AuthorityInfo[]>('/secvitals/org-authorities'),
    staleTime: 5 * 60 * 1000,
  })
}
