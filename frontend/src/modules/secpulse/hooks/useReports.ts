import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { Report, RiskTrendResponse } from '../types'

export interface CreateReportInput {
  title: string
}

export function useRiskTrend() {
  return useQuery<RiskTrendResponse>({
    queryKey: ['secpulse', 'reports', 'risk-trend'],
    queryFn: () => apiFetch<RiskTrendResponse>('/secpulse/reports/risk-trend'),
    staleTime: 30_000,
  })
}

export function useReports() {
  return useQuery<Report[]>({
    queryKey: ['secpulse', 'reports'],
    queryFn: () => apiFetch<Report[]>('/secpulse/reports'),
    staleTime: 5_000,
    refetchInterval: (query) => {
      const data = query.state.data
      if (Array.isArray(data) && data.some((r) => r.status === 'pending' || r.status === 'processing')) {
        return 3_000
      }
      return false
    },
  })
}

export function useDownloadReport() {
  return (reportId: string, title?: string) => {
    fetch(`/api/v1/secpulse/reports/${reportId}/download`, {
      credentials: 'include',
    })
      .then((r) => r.blob())
      .then((blob) => {
        const url = URL.createObjectURL(blob)
        const a = document.createElement('a')
        a.href = url
        a.download = title ? `${title}.pdf` : `report-${reportId.slice(0, 8)}.pdf`
        document.body.appendChild(a)
        a.click()
        a.remove()
        URL.revokeObjectURL(url)
      })
  }
}

export function useCreateReport() {
  const queryClient = useQueryClient()
  return useMutation<Report, Error, CreateReportInput>({
    mutationFn: (data) =>
      apiFetch<Report>('/secpulse/reports', {
        method: 'POST',
        body: JSON.stringify({ title: data.title }),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['secpulse', 'reports'] })
    },
  })
}
