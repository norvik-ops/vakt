import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { ApiRequest, ApiResponse } from '../../../api/contract'
import type { AuditorLink } from '../types'

export function useAuditorLinks() {
  return useQuery<AuditorLink[]>({
    queryKey: ['vaktcomply', 'auditor-links'],
    queryFn: () => apiFetch<AuditorLink[]>('/vaktcomply/auditor-links'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useRevokeAuditorLink() {
  const queryClient = useQueryClient()
  return useMutation<undefined, Error, string>({
    mutationFn: (id: string) =>
      apiFetch<undefined>(`/vaktcomply/auditor-links/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'auditor-links'] })
    },
  })
}

/**
 * L1-01 — what the caller passes in. Deliberately still expressed in days:
 * that is the unit the dialog offers and the unit an auditor link is thought
 * about in. The conversion to the wire format happens once, below.
 */
export interface CreateAuditorLinkPayload {
  expires_in_days: number
  label: string
}

/**
 * The body POST /vaktcomply/frameworks/:id/auditor-link actually accepts.
 * Typed from the OpenAPI contract so this cannot drift again unnoticed —
 * handler_audit.go:386-389 binds `expires_in_hours` with
 * `validate:"required,min=1,max=8760"`.
 */
type CreateAuditorLinkBody = ApiRequest<'/vaktcomply/frameworks/{id}/auditor-link', 'post'>

type CreateAuditorLinkResponse = ApiResponse<'/vaktcomply/frameworks/{id}/auditor-link', 'post'>

/** Backend cap: 8760 hours (one year), enforced by the validate tag above. */
const MAX_LINK_HOURS = 8760

/**
 * L1-01 — creating an auditor link failed every single time.
 *
 * The hook posted `{expires_in_days, label}`. The handler binds
 * `expires_in_hours` (required, 1..8760) and `max_uses`; it has never had a
 * field called `expires_in_days`. Echo's Bind drops the unknown key, so
 * ExpiresInHours stayed 0, the `required` tag rejected it and the request came
 * back 422 VALIDATION_ERROR — with no combination of dialog inputs that could
 * have worked. There was no partial failure to notice: the feature was dead
 * from the first request.
 *
 * `label` is still sent even though nothing reads it today, so the dialog does
 * not have to change again when the backend catches up. What the operator
 * types is currently discarded — see the note in the commit; the whole create
 * path (handler → service → repo → sqlc params) has no label column, which is
 * why the Label column of the list is permanently "—". That fix belongs to
 * backend/ and is reported separately, not worked around here.
 */
export function useCreateAuditorLink(frameworkId: string) {
  const queryClient = useQueryClient()
  return useMutation<CreateAuditorLinkResponse, Error, CreateAuditorLinkPayload>({
    mutationFn: (payload) => {
      const body: CreateAuditorLinkBody & { label: string } = {
        expires_in_hours: Math.min(Math.max(payload.expires_in_days, 1) * 24, MAX_LINK_HOURS),
        label: payload.label,
      }
      return apiFetch<CreateAuditorLinkResponse>(
        `/vaktcomply/frameworks/${frameworkId}/auditor-link`,
        { method: 'POST', body: JSON.stringify(body) },
      )
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktcomply', 'auditor-links'] })
    },
  })
}
