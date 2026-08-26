import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { Asset, SLAEntry, ClassificationSummary, SLAPolicy, SLASummaryFE } from '../types'
import type { PaginatedResponse } from '../../../shared/types/pagination'

// Mirrors vaktscan.CreateAssetInput. `external_url`, not `target` (K5-16).
export interface CreateAssetInput {
  name: string
  type: Asset['type']
  external_url: string
  criticality: Asset['criticality']
  tags: string[]
  classification?: Asset['classification']
}

export function useAssets(page = 1, limit = 25, tag?: string) {
  const params = new URLSearchParams({ page: String(page), limit: String(limit) })
  if (tag) params.set('tag', tag)
  const query = useQuery<PaginatedResponse<Asset>>({
    queryKey: ['vaktscan', 'assets', page, limit, tag],
    queryFn: () => apiFetch<PaginatedResponse<Asset>>(`/vaktscan/assets?${params.toString()}`),
    staleTime: 30_000,
  })
  return {
    ...query,
    data: query.data?.data,
    pagination: query.data?.pagination,
  }
}

export function useAsset(id: string) {
  return useQuery<Asset>({
    queryKey: ['vaktscan', 'assets', id],
    queryFn: () => apiFetch<Asset>(`/vaktscan/assets/${id}`),
    staleTime: 30_000,
    enabled: Boolean(id),
  })
}

export function useCreateAsset() {
  const queryClient = useQueryClient()
  return useMutation<Asset, Error, CreateAssetInput>({
    mutationFn: (data) =>
      apiFetch<Asset>('/vaktscan/assets', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktscan', 'assets'] })
    },
  })
}

export interface ImportAssetsResult {
  inserted: number
  errored: number
  errors: string[]
}

/**
 * R23-02: AssetsPage no longer calls this — the page's only import path is
 * CSVImportDialog, which posts through apiFetch itself and picks the Community
 * or Pro endpoint by licence. The hook stays because it is the subject of the
 * CSRF regression test for POST /vaktscan/assets/import
 * (api/csrf-write-paths.test.tsx case 6, R1-18-D4); removing it would quietly
 * remove that endpoint's coverage along with it.
 */
export function useImportAssets() {
  const queryClient = useQueryClient()
  return useMutation<ImportAssetsResult, Error, FormData>({
    mutationFn: (formData) =>
      apiFetch<ImportAssetsResult>('/vaktscan/assets/import', {
        method: 'POST',
        body: formData,
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktscan', 'assets'] })
    },
  })
}

export function useSLADashboard() {
  return useQuery<SLAEntry[]>({
    queryKey: ['vaktscan', 'sla-dashboard'],
    queryFn: () => apiFetch<SLAEntry[]>('/vaktscan/sla-dashboard'),
    staleTime: 60_000,
  })
}

export function useClassificationSummary() {
  return useQuery<ClassificationSummary>({
    queryKey: ['vaktscan', 'assets', 'classification-summary'],
    queryFn: () => apiFetch<ClassificationSummary>('/vaktscan/assets/classification-summary'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useDeleteAsset() {
  const queryClient = useQueryClient()
  return useMutation<undefined, Error, string>({
    mutationFn: (id) =>
      apiFetch<undefined>(`/vaktscan/assets/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktscan', 'assets'] })
    },
  })
}

/** The scanners CreateScanInput accepts — `oneof=trivy nuclei openvas`. */
export type ScannerName = 'trivy' | 'nuclei' | 'openvas'

export interface TriggerScanInput {
  scanner: ScannerName
  /**
   * Optional target. Left out, the worker falls back to the ASSET NAME
   * (vaktscan/scanner.go:189-191, :277-282, :499-503) and then rejects any
   * target containing a slash — so an asset called "My Web App" gets scanned
   * as the literal string "My Web App". Passing the asset's external_url is
   * what makes the scan hit the thing the user meant.
   */
  target_url?: string
}

/**
 * R1-19-W07 — "Scan starten" could never work.
 *
 * The mutation posted no body at all. CreateScanInput.Scanner carries
 * `validate:"required,oneof=trivy nuclei openvas"` (vaktscan/models.go:133), so
 * every click came back 422 VALIDATION_ERROR. There was no way to fix it from
 * the UI either: the frontend had no scanner selection anywhere, so no user
 * action could have produced a valid request.
 *
 * check_routes.py cannot see this by construction — it compares method and
 * path, and both were correct.
 */
export function useTriggerScan(assetId: string) {
  const queryClient = useQueryClient()
  return useMutation<undefined, Error, TriggerScanInput>({
    mutationFn: (input) =>
      apiFetch<undefined>(`/vaktscan/assets/${assetId}/scans`, {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktscan', 'assets', assetId] })
      void queryClient.invalidateQueries({ queryKey: ['vaktscan', 'findings'] })
    },
  })
}

// ── S69-3: SLA Policies ───────────────────────────────────────────────────────

export function useSLAPolicies() {
  return useQuery<SLAPolicy[]>({
    queryKey: ['vaktscan', 'sla-policies'],
    queryFn: () => apiFetch<SLAPolicy[]>('/vaktscan/sla-policies'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useSLASummaryNew() {
  return useQuery<SLASummaryFE>({
    queryKey: ['vaktscan', 'sla-summary'],
    queryFn: () => apiFetch<SLASummaryFE>('/vaktscan/sla/summary'),
    staleTime: 60_000,
  })
}

export function useUpsertSLAPolicy() {
  const queryClient = useQueryClient()
  return useMutation<SLAPolicy, Error, { severity: string; remediation_days: number; notification_advance_days: number }>({
    mutationFn: ({ severity, ...body }) =>
      apiFetch<SLAPolicy>(`/vaktscan/sla-policies/${severity}`, { method: 'PUT', body: JSON.stringify(body) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktscan', 'sla-policies'] })
      void queryClient.invalidateQueries({ queryKey: ['vaktscan', 'sla-summary'] })
    },
  })
}

export function useResetSLAPolicies() {
  const queryClient = useQueryClient()
  return useMutation<void>({
    mutationFn: () => apiFetch('/vaktscan/sla-policies/reset', { method: 'POST' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vaktscan', 'sla-policies'] })
    },
  })
}
