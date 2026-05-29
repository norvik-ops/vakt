import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'

// ─── Types ────────────────────────────────────────────────────────────────────

export interface ApprovalWithDetails {
  id: string
  org_id: string
  control_id: string
  requested_by: string
  requested_status: string
  current_status: string
  comment: string
  status: 'pending' | 'approved' | 'rejected'
  reviewed_by: string
  reviewed_at: string | null
  review_comment: string
  created_at: string
  control_title: string
  control_ref: string
  requester_name: string
  requester_email: string
}

export interface ApprovalCount {
  count: number
}

export interface ApprovalSetting {
  approval_required: boolean
}

// ─── Hooks ────────────────────────────────────────────────────────────────────

export function usePendingApprovals() {
  return useQuery<ApprovalWithDetails[]>({
    queryKey: ['vaktcomply', 'approvals', 'pending'],
    queryFn: () => apiFetch<ApprovalWithDetails[]>('/vaktcomply/approvals'),
    staleTime: 30_000,
  })
}

export function usePendingApprovalCount() {
  return useQuery<ApprovalCount>({
    queryKey: ['vaktcomply', 'approvals', 'count'],
    queryFn: () => apiFetch<ApprovalCount>('/vaktcomply/approvals/count'),
    staleTime: 30_000,
    refetchInterval: 60_000,
  })
}

export function useApprovalSetting() {
  return useQuery<ApprovalSetting>({
    queryKey: ['vaktcomply', 'org', 'approval-setting'],
    queryFn: () => apiFetch<ApprovalSetting>('/vaktcomply/org/approval-setting'),
    staleTime: 60_000,
  })
}

export function useUpdateApprovalSetting() {
  const qc = useQueryClient()
  return useMutation<undefined, Error, boolean>({
    mutationFn: (approval_required) =>
      apiFetch<undefined>('/vaktcomply/org/approval-setting', {
        method: 'PUT',
        body: JSON.stringify({ approval_required }),
      }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['vaktcomply', 'org', 'approval-setting'] }),
  })
}

export function useApproveApproval() {
  const qc = useQueryClient()
  return useMutation<undefined, Error, { id: string; comment: string }>({
    mutationFn: ({ id, comment }) =>
      apiFetch<undefined>(`/vaktcomply/approvals/${id}/approve`, {
        method: 'POST',
        body: JSON.stringify({ comment }),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['vaktcomply', 'approvals'] })
      // Invalidate control cache so the updated status shows immediately.
      void qc.invalidateQueries({ queryKey: ['vaktcomply', 'controls'] })
      void qc.invalidateQueries({ queryKey: ['vaktcomply', 'frameworks'] })
    },
  })
}

export function useRejectApproval() {
  const qc = useQueryClient()
  return useMutation<undefined, Error, { id: string; comment: string }>({
    mutationFn: ({ id, comment }) =>
      apiFetch<undefined>(`/vaktcomply/approvals/${id}/reject`, {
        method: 'POST',
        body: JSON.stringify({ comment }),
      }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['vaktcomply', 'approvals'] }),
  })
}

export function useRequestControlApproval(controlId: string) {
  const qc = useQueryClient()
  return useMutation<void, Error, { requested_status: string; comment: string }>({
    mutationFn: (body) =>
      apiFetch<void>(`/vaktcomply/controls/${controlId}/approval-request`, {
        method: 'POST',
        body: JSON.stringify(body),
      }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['vaktcomply', 'approvals'] }),
  })
}
