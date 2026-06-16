import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type {
  EmergencyContact,
  CreateEmergencyContactInput,
  UpdateEmergencyContactInput,
} from '../types'

const QK = ['vaktcomply', 'bcm', 'emergency-contacts'] as const

export function useEmergencyContacts() {
  return useQuery<EmergencyContact[]>({
    queryKey: [...QK],
    queryFn: () => apiFetch<EmergencyContact[]>('/vaktcomply/bcm/emergency-contacts'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useCreateEmergencyContact() {
  const queryClient = useQueryClient()
  return useMutation<EmergencyContact, Error, CreateEmergencyContactInput>({
    mutationFn: (input) =>
      apiFetch<EmergencyContact>('/vaktcomply/bcm/emergency-contacts', {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: QK })
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'bcm', 'score'] })
    },
  })
}

export function useUpdateEmergencyContact(id: string) {
  const queryClient = useQueryClient()
  return useMutation<EmergencyContact, Error, UpdateEmergencyContactInput>({
    mutationFn: (input) =>
      apiFetch<EmergencyContact>(`/vaktcomply/bcm/emergency-contacts/${id}`, {
        method: 'PUT',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: QK })
    },
  })
}

export function useDeleteEmergencyContact() {
  const queryClient = useQueryClient()
  return useMutation<undefined, Error, string>({
    mutationFn: (id) =>
      apiFetch<undefined>(`/vaktcomply/bcm/emergency-contacts/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: QK })
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'bcm', 'score'] })
    },
  })
}
