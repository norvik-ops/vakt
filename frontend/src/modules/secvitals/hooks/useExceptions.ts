import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'

export interface ControlException {
  id: string
  org_id: string
  control_id: string
  title: string
  reason: string
  risk_accepted: string
  approved_by?: string
  expires_at?: string | null
  status: 'active' | 'expired' | 'revoked'
  created_by?: string
  created_at: string
  updated_at: string
}

export interface CreateControlExceptionInput {
  title: string
  reason: string
  risk_accepted: string
  approved_by?: string
  expires_at?: string | null
}

export interface UpdateControlExceptionInput {
  title?: string
  reason?: string
  risk_accepted?: string
  approved_by?: string
  expires_at?: string | null
  status?: 'active' | 'expired' | 'revoked'
}

export function useControlExceptions(controlId: string) {
  return useQuery<ControlException[]>({
    queryKey: ['secvitals', 'exceptions', controlId],
    queryFn: () =>
      apiFetch<ControlException[]>(`/secvitals/exceptions?control_id=${controlId}`),
    enabled: !!controlId,
    staleTime: 5 * 60 * 1000,
  })
}

export function useCreateControlException(controlId: string) {
  const queryClient = useQueryClient()
  return useMutation<ControlException, Error, CreateControlExceptionInput>({
    mutationFn: (input) =>
      apiFetch<ControlException>(`/secvitals/controls/${controlId}/exceptions`, {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ['secvitals', 'exceptions', controlId],
      })
    },
  })
}

export function useUpdateControlException() {
  const queryClient = useQueryClient()
  return useMutation<ControlException, Error, { id: string; input: UpdateControlExceptionInput }>({
    mutationFn: ({ id, input }) =>
      apiFetch<ControlException>(`/secvitals/exceptions/${id}`, {
        method: 'PUT',
        body: JSON.stringify(input),
      }),
    onSuccess: (data) => {
      void queryClient.invalidateQueries({
        queryKey: ['secvitals', 'exceptions', data.control_id],
      })
    },
  })
}

export function useDeleteControlException() {
  const queryClient = useQueryClient()
  return useMutation<void, Error, { id: string; controlId: string }>({
    mutationFn: ({ id }) =>
      apiFetch<void>(`/secvitals/exceptions/${id}`, { method: 'DELETE' }),
    onSuccess: (_data, { controlId }) => {
      void queryClient.invalidateQueries({
        queryKey: ['secvitals', 'exceptions', controlId],
      })
    },
  })
}
