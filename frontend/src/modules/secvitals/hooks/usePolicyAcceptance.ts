import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface PolicyAcceptanceCampaign {
  id: string
  org_id: string
  policy_id: string
  name: string
  message?: string
  deadline?: string
  created_at: string
}

export interface CreateCampaignInput {
  policy_id: string
  name: string
  message?: string
  deadline?: string
  emails: Array<{ email: string; name?: string }>
}

export interface PolicyAcceptanceRequest {
  id: string
  campaign_id: string
  recipient_email: string
  recipient_name?: string
  accepted_at?: string
  sent_at?: string
  created_at: string
}

export interface CampaignStats {
  total: number
  accepted: number
  pending: number
}

// ---------------------------------------------------------------------------
// Public (no-auth) API helpers
// ---------------------------------------------------------------------------

export interface AcceptancePublicInfo {
  policy_title: string
  org_name: string
  message?: string
  deadline?: string
  accepted_at?: string
}

export async function fetchAcceptanceInfo(token: string): Promise<AcceptancePublicInfo> {
  const res = await fetch(`/api/v1/policy-accept/${token}`, {
    headers: { Accept: 'application/json' },
  })
  if (!res.ok) {
    const err = (await res.json().catch(() => ({}))) as { error?: string }
    throw new Error(err.error ?? 'NOT_FOUND')
  }
  return res.json() as Promise<AcceptancePublicInfo>
}

export async function submitAcceptance(token: string): Promise<{ status: string }> {
  const res = await fetch(`/api/v1/policy-accept/${token}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
  })
  if (!res.ok) {
    const err = (await res.json().catch(() => ({}))) as { error?: string }
    throw new Error(err.error ?? 'ACCEPT_FAILED')
  }
  return res.json() as Promise<{ status: string }>
}

// ---------------------------------------------------------------------------
// Authenticated hooks
// ---------------------------------------------------------------------------

export function useCampaigns(policyId: string) {
  return useQuery<PolicyAcceptanceCampaign[]>({
    queryKey: ['secvitals', 'policies', policyId, 'acceptance-campaigns'],
    queryFn: () => apiFetch<PolicyAcceptanceCampaign[]>(`/secvitals/policies/${policyId}/acceptance-campaigns`),
    enabled: !!policyId,
    staleTime: 60 * 1000,
  })
}

export function useCreateCampaign(policyId: string) {
  const queryClient = useQueryClient()
  return useMutation<PolicyAcceptanceCampaign, Error, CreateCampaignInput>({
    mutationFn: (input) =>
      apiFetch<PolicyAcceptanceCampaign>(`/secvitals/policies/${policyId}/acceptance-campaigns`, {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ['secvitals', 'policies', policyId, 'acceptance-campaigns'],
      })
    },
  })
}

export function useCampaignStats(campaignId: string) {
  return useQuery<CampaignStats>({
    queryKey: ['secvitals', 'acceptance-campaigns', campaignId, 'stats'],
    queryFn: () => apiFetch<CampaignStats>(`/secvitals/policies/acceptance-campaigns/${campaignId}/stats`),
    enabled: !!campaignId,
    staleTime: 30 * 1000,
  })
}

export function useCampaignRequests(campaignId: string) {
  return useQuery<PolicyAcceptanceRequest[]>({
    queryKey: ['secvitals', 'acceptance-campaigns', campaignId, 'requests'],
    queryFn: () =>
      apiFetch<PolicyAcceptanceRequest[]>(`/secvitals/policies/acceptance-campaigns/${campaignId}/requests`),
    enabled: !!campaignId,
    staleTime: 30 * 1000,
  })
}
