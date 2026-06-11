// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type {
  BSITargetObject,
  CreateBSITargetObjectInput,
  BSICheckResult,
  SetCheckResultInput,
  BSICheckSummary,
  BSICockpit,
  BSIGapReport,
  BSIThreat,
  BSIRiskAssessment,
  CreateBSIRiskInput,
  UpdateBSIRiskInput,
  BSIRiskSummary,
  BSIReportExport,
  BSIReportType,
} from '../types'

const QK = ['vaktcomply', 'bsi'] as const

// ── Target Objects ────────────────────────────────────────────────────────────

export function useBSITargetObjects() {
  return useQuery<BSITargetObject[]>({
    queryKey: [...QK, 'target-objects'],
    queryFn: () => apiFetch<BSITargetObject[]>('/vaktcomply/bsi/target-objects'),
    staleTime: 2 * 60 * 1000,
  })
}

export function useBSITargetObject(id: string) {
  return useQuery<BSITargetObject>({
    queryKey: [...QK, 'target-objects', id],
    queryFn: () => apiFetch<BSITargetObject>(`/vaktcomply/bsi/target-objects/${id}`),
    enabled: !!id,
  })
}

export function useCreateBSITargetObject() {
  const qc = useQueryClient()
  return useMutation<BSITargetObject, Error, CreateBSITargetObjectInput>({
    mutationFn: (input) =>
      apiFetch<BSITargetObject>('/vaktcomply/bsi/target-objects', {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: [...QK, 'target-objects'] }),
  })
}

export function useDeleteBSITargetObject() {
  const qc = useQueryClient()
  return useMutation<void, Error, string>({
    mutationFn: (id) =>
      apiFetch<void>(`/vaktcomply/bsi/target-objects/${id}`, { method: 'DELETE' }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: [...QK, 'target-objects'] }),
  })
}

// ── Check Sheet ───────────────────────────────────────────────────────────────

export function useBSICheckSheet(targetObjectId: string) {
  return useQuery<BSICheckResult[]>({
    queryKey: [...QK, 'check', targetObjectId],
    queryFn: () =>
      apiFetch<BSICheckResult[]>(`/vaktcomply/bsi/target-objects/${targetObjectId}/check`),
    enabled: !!targetObjectId,
  })
}

export function useBSICheckSummary(targetObjectId: string) {
  return useQuery<BSICheckSummary>({
    queryKey: [...QK, 'check-summary', targetObjectId],
    queryFn: () =>
      apiFetch<BSICheckSummary>(
        `/vaktcomply/bsi/target-objects/${targetObjectId}/check/summary`,
      ),
    enabled: !!targetObjectId,
  })
}

export function useSetBSICheckResult(targetObjectId: string) {
  const qc = useQueryClient()
  return useMutation<BSICheckResult, Error, { anforderungId: string } & SetCheckResultInput>({
    mutationFn: ({ anforderungId, ...body }) =>
      apiFetch<BSICheckResult>(
        `/vaktcomply/bsi/target-objects/${targetObjectId}/check/${anforderungId}`,
        { method: 'PUT', body: JSON.stringify(body) },
      ),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: [...QK, 'check', targetObjectId] })
      void qc.invalidateQueries({ queryKey: [...QK, 'check-summary', targetObjectId] })
      void qc.invalidateQueries({ queryKey: [...QK, 'cockpit'] })
    },
  })
}

// ── Cockpit + Gap Report ──────────────────────────────────────────────────────

export function useBSICockpit() {
  return useQuery<BSICockpit>({
    queryKey: [...QK, 'cockpit'],
    queryFn: () => apiFetch<BSICockpit>('/vaktcomply/bsi/cockpit'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useBSIGapReport() {
  return useQuery<BSIGapReport>({
    queryKey: [...QK, 'gap-report'],
    queryFn: () => apiFetch<BSIGapReport>('/vaktcomply/bsi/gap-report'),
    staleTime: 5 * 60 * 1000,
  })
}

// ── Threats ───────────────────────────────────────────────────────────────────

export function useBSIThreats() {
  return useQuery<BSIThreat[]>({
    queryKey: [...QK, 'threats'],
    queryFn: () => apiFetch<BSIThreat[]>('/vaktcomply/bsi/threats'),
    staleTime: 60 * 60 * 1000,
  })
}

// ── Risk Assessments ──────────────────────────────────────────────────────────

export function useBSIRisks(targetObjectId: string) {
  return useQuery<BSIRiskAssessment[]>({
    queryKey: [...QK, 'risks', targetObjectId],
    queryFn: () =>
      apiFetch<BSIRiskAssessment[]>(`/vaktcomply/bsi/target-objects/${targetObjectId}/risks`),
    enabled: !!targetObjectId,
  })
}

export function useBSIRiskSummary(targetObjectId: string) {
  return useQuery<BSIRiskSummary>({
    queryKey: [...QK, 'risk-summary', targetObjectId],
    queryFn: () =>
      apiFetch<BSIRiskSummary>(
        `/vaktcomply/bsi/target-objects/${targetObjectId}/risks/summary`,
      ),
    enabled: !!targetObjectId,
  })
}

export function useCreateBSIRisk(targetObjectId: string) {
  const qc = useQueryClient()
  return useMutation<BSIRiskAssessment, Error, CreateBSIRiskInput>({
    mutationFn: (input) =>
      apiFetch<BSIRiskAssessment>(
        `/vaktcomply/bsi/target-objects/${targetObjectId}/risks`,
        { method: 'POST', body: JSON.stringify(input) },
      ),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: [...QK, 'risks', targetObjectId] })
      void qc.invalidateQueries({ queryKey: [...QK, 'risk-summary', targetObjectId] })
    },
  })
}

export function useUpdateBSIRisk(targetObjectId: string, riskId: string) {
  const qc = useQueryClient()
  return useMutation<BSIRiskAssessment, Error, UpdateBSIRiskInput>({
    mutationFn: (input) =>
      apiFetch<BSIRiskAssessment>(
        `/vaktcomply/bsi/target-objects/${targetObjectId}/risks/${riskId}`,
        { method: 'PUT', body: JSON.stringify(input) },
      ),
    onSuccess: () =>
      void qc.invalidateQueries({ queryKey: [...QK, 'risks', targetObjectId] }),
  })
}

export function useDeleteBSIRisk(targetObjectId: string) {
  const qc = useQueryClient()
  return useMutation<void, Error, string>({
    mutationFn: (riskId) =>
      apiFetch<void>(
        `/vaktcomply/bsi/target-objects/${targetObjectId}/risks/${riskId}`,
        { method: 'DELETE' },
      ),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: [...QK, 'risks', targetObjectId] })
      void qc.invalidateQueries({ queryKey: [...QK, 'risk-summary', targetObjectId] })
    },
  })
}

// ── Report Exports ────────────────────────────────────────────────────────────

export function useBSIReportExports() {
  return useQuery<BSIReportExport[]>({
    queryKey: [...QK, 'reports'],
    queryFn: () => apiFetch<BSIReportExport[]>('/vaktcomply/bsi/reports'),
    staleTime: 60 * 1000,
  })
}

export function useBSIReportPreview(type: BSIReportType | null) {
  return useQuery<string>({
    queryKey: [...QK, 'report-preview', type],
    queryFn: () => apiFetch<string>(`/vaktcomply/bsi/reports/${type}/preview`),
    enabled: !!type,
    staleTime: 5 * 60 * 1000,
  })
}
