import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type {
  BCPPlan,
  BCPTest,
  CreateBCPPlanInput,
  UpdateBCPPlanInput,
  CreateBCPTestInput,
} from '../types'

const QK = ['vaktcomply', 'bcp'] as const

export function useBCPPlans() {
  return useQuery<BCPPlan[]>({
    queryKey: [...QK, 'plans'],
    queryFn: () => apiFetch<BCPPlan[]>('/vaktcomply/bcp/plans'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useCreateBCPPlan() {
  const queryClient = useQueryClient()
  return useMutation<BCPPlan, Error, CreateBCPPlanInput>({
    mutationFn: (input) =>
      apiFetch<BCPPlan>('/vaktcomply/bcp/plans', { method: 'POST', body: JSON.stringify(input) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: [...QK, 'plans'] })
    },
  })
}

export function useUpdateBCPPlan(id: string) {
  const queryClient = useQueryClient()
  return useMutation<BCPPlan, Error, UpdateBCPPlanInput>({
    mutationFn: (input) =>
      // S121-C5 (C2): backend registers this as PATCH; PUT returned 404.
      apiFetch<BCPPlan>(`/vaktcomply/bcp/plans/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: [...QK, 'plans'] })
    },
  })
}

export function useDeleteBCPPlan() {
  const queryClient = useQueryClient()
  return useMutation<undefined, Error, string>({
    mutationFn: (planId) =>
      apiFetch<undefined>(`/vaktcomply/bcp/plans/${planId}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: [...QK, 'plans'] })
    },
  })
}

export function useBCPTests(planId: string) {
  return useQuery<BCPTest[]>({
    queryKey: [...QK, 'tests', planId],
    queryFn: () => apiFetch<BCPTest[]>(`/vaktcomply/bcp/plans/${planId}/tests`),
    enabled: !!planId,
    staleTime: 5 * 60 * 1000,
  })
}

export function useAddBCPTest() {
  const queryClient = useQueryClient()
  return useMutation<BCPTest, Error, CreateBCPTestInput>({
    mutationFn: ({ plan_id, ...rest }) =>
      apiFetch<BCPTest>(`/vaktcomply/bcp/plans/${plan_id}/tests`, {
        method: 'POST',
        body: JSON.stringify(rest),
      }),
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({ queryKey: [...QK, 'tests', variables.plan_id] })
      void queryClient.invalidateQueries({ queryKey: [...QK, 'plans'] })
    },
  })
}

// S121-F2 (C3): removed dead hook useLinkBCPPlanAsEvidence — 0 UI references and
// it POSTed to /bcp/plans/:id/link-evidence while the backend registers the
// BCP evidence link as /bcp/plans/:id/evidence (LinkBCPPlanAsEvidence).
