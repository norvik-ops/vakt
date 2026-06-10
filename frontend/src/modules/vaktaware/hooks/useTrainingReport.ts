import { useQuery } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { TrainingMatrixReport } from '../types'

const BASE = '/vaktaware'

export function useTrainingMatrixReport(from?: string, to?: string) {
  const params = new URLSearchParams()
  if (from) params.set('from', from)
  if (to) params.set('to', to)
  const qs = params.toString()

  return useQuery<TrainingMatrixReport>({
    queryKey: ['vaktaware', 'training-matrix', from, to],
    queryFn: () =>
      apiFetch<TrainingMatrixReport>(`${BASE}/reports/training-matrix${qs ? `?${qs}` : ''}`),
    staleTime: 5 * 60_000,
  })
}

export function downloadTrainingMatrix(format: 'pdf' | 'csv', from?: string, to?: string) {
  const params = new URLSearchParams()
  if (from) params.set('from', from)
  if (to) params.set('to', to)
  const qs = params.toString()
  const url = `/api/v1/vaktaware/reports/training-matrix/export/${format}${qs ? `?${qs}` : ''}`
  const a = document.createElement('a')
  a.href = url
  a.download = `training-report.${format}`
  a.click()
}
