import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'

export interface AlertChannel {
  id: string
  name: string
  type: 'slack' | 'teams' | 'webhook' | 'email'
  events: string[]
  enabled: boolean
  created_at: string
}

export interface CreateChannelInput {
  name: string
  type: AlertChannel['type']
  url: string
  events: string[]
}

export const ALERT_EVENTS = [
  { value: 'finding.sla_overdue', label: 'Finding SLA überschritten' },
  { value: 'breach.created', label: 'Neue Datenpanne' },
  { value: 'dsr.overdue', label: 'DSR-Frist überschritten' },
  { value: 'avv.expired', label: 'AVV abgelaufen' },
  { value: 'scan.failed', label: 'Scan fehlgeschlagen' },
]

export function useAlertChannels() {
  return useQuery<AlertChannel[]>({
    queryKey: ['alerting', 'channels'],
    queryFn: () => apiFetch<AlertChannel[]>('/alerting/channels'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useCreateAlertChannel() {
  const qc = useQueryClient()
  return useMutation<AlertChannel, Error, CreateChannelInput>({
    mutationFn: (data) =>
      apiFetch<AlertChannel>('/alerting/channels', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['alerting', 'channels'] }),
  })
}

export function useDeleteAlertChannel() {
  const qc = useQueryClient()
  return useMutation<void, Error, string>({
    mutationFn: (id) => apiFetch<void>(`/alerting/channels/${id}`, { method: 'DELETE' }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['alerting', 'channels'] }),
  })
}

export function useToggleAlertChannel() {
  const qc = useQueryClient()
  return useMutation<void, Error, { id: string; enabled: boolean }>({
    mutationFn: ({ id, enabled }) =>
      apiFetch<void>(`/alerting/channels/${id}/toggle`, {
        method: 'PUT',
        body: JSON.stringify({ enabled }),
      }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['alerting', 'channels'] }),
  })
}

export function useTestAlertChannel() {
  return useMutation<void, Error, string>({
    mutationFn: (id) => apiFetch<void>(`/alerting/channels/${id}/test`, { method: 'POST' }),
  })
}

export interface DeliveryLogEntry {
  id: string
  channel_id?: string
  event: string
  status: 'sent' | 'failed'
  response_code?: number
  sent_at: string
}

export function useAlertDeliveryLog() {
  return useQuery<DeliveryLogEntry[]>({
    queryKey: ['alerting', 'history'],
    queryFn: () => apiFetch<DeliveryLogEntry[]>('/alerting/history'),
    staleTime: 30_000,
  })
}
