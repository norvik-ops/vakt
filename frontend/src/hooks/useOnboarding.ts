import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../api/client'

export interface OnboardingStatus {
  completed: boolean
  dismissed: boolean
  steps: {
    org_configured: boolean
    framework_selected: boolean
    first_control_reviewed: boolean
    first_risk_created: boolean
  }
}

export function useOnboardingStatus() {
  return useQuery<OnboardingStatus>({
    queryKey: ['onboarding', 'status'],
    queryFn: () => apiFetch<OnboardingStatus>('/onboarding/status'),
    staleTime: 30_000,
  })
}

export function useDismissOnboarding() {
  const qc = useQueryClient()
  return useMutation<void, Error, void>({
    mutationFn: () => apiFetch<void>('/onboarding/dismiss', { method: 'POST' }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['onboarding', 'status'] })
    },
  })
}
