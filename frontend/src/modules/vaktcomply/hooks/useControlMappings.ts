import { useQuery } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'

export interface ControlMapping {
  id: string
  source_framework: string
  source_control_code: string
  target_framework: string
  target_control_code: string
  mapping_type: 'equivalent' | 'partial' | 'informative'
  target_control_id: string
  target_control_title: string
  target_framework_name: string
}

interface ControlMappingsResponse {
  mappings: ControlMapping[]
}

/**
 * Fetches all cross-framework mappings for the given control UUID.
 * Returns empty array while controlId is undefined.
 */
export function useControlMappings(controlId: string | undefined) {
  return useQuery<ControlMapping[]>({
    queryKey: ['vaktcomply', 'controls', controlId, 'mappings'],
    queryFn: () =>
      apiFetch<ControlMappingsResponse>(`/vaktcomply/controls/${controlId ?? ''}/mappings`).then(
        (res) => res.mappings,
      ),
    enabled: !!controlId,
    staleTime: 10 * 60 * 1000,
  })
}
