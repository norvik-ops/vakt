import { useQuery } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { ScannerName } from './useAssets'

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

/**
 * The scanners this instance can actually run, in the order the picker shows
 * them. R1-19-W07: the scan trigger needs a scanner name, and offering one the
 * server has no binary for would only move the failure from a 422 to a scan
 * that fails in the worker.
 */
export function useAvailableScanners(): ScannerName[] {
  const { data } = useScannerStatus()
  if (!data) return []
  const all: ScannerName[] = ['trivy', 'nuclei', 'openvas']
  return all.filter((name) => data[name])
}
