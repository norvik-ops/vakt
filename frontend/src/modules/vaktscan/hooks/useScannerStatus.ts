import { useQuery } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'

export interface ScannerStatus {
  trivy: boolean
  nuclei: boolean
  openvas: boolean
}

export function useScannerStatus() {
  return useQuery<ScannerStatus>({
    queryKey: ['vaktscan', 'scanner-status'],
    queryFn: () => apiFetch<ScannerStatus>('/vaktscan/scanner-status'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useNoScannersAvailable(): boolean {
  const { data } = useScannerStatus()
  if (!data) return false
  return !data.trivy && !data.nuclei && !data.openvas
}
