import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { OrgSectorSettings, UpdateOrgSectorInput, AuthorityInfo } from '../types'

export function useOrgSector() {
  return useQuery<OrgSectorSettings>({
    queryKey: ['vaktcomply', 'org-sector'],
    queryFn: () => apiFetch<OrgSectorSettings>('/vaktcomply/org-sector'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useUpdateOrgSector() {
  const queryClient = useQueryClient()
  return useMutation<OrgSectorSettings, Error, UpdateOrgSectorInput>({
    mutationFn: (input) =>
      apiFetch<OrgSectorSettings>('/vaktcomply/org-sector', { method: 'PATCH', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'org-sector'] })
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'org-authorities'] })
    },
  })
}

export function useAuthorities() {
  return useQuery<AuthorityInfo[]>({
    queryKey: ['vaktcomply', 'authorities'],
    queryFn: () => apiFetch<AuthorityInfo[]>('/vaktcomply/authorities'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useOrgAuthorities() {
  return useQuery<AuthorityInfo[]>({
    queryKey: ['vaktcomply', 'org-authorities'],
    queryFn: () => apiFetch<AuthorityInfo[]>('/vaktcomply/org-authorities'),
    staleTime: 5 * 60 * 1000,
  })
}
