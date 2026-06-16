import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type {
  BackupJob,
  BackupSummary,
  BackupJobInput,
  BackupRestoreTest,
  RestoreTestInput,
} from '../types'

const QK = ['vaktcomply', 'backup'] as const

export function useBackupSummary() {
  return useQuery<BackupSummary>({
    queryKey: [...QK, 'summary'],
    queryFn: () => apiFetch<BackupSummary>('/vaktcomply/backup/summary'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useBackupJobs() {
  return useQuery<BackupJob[]>({
    queryKey: [...QK, 'jobs'],
    queryFn: () => apiFetch<BackupJob[]>('/vaktcomply/backup/jobs'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useCreateBackupJob() {
  const qc = useQueryClient()
  return useMutation<BackupJob, Error, BackupJobInput>({
    mutationFn: (input) =>
      apiFetch<BackupJob>('/vaktcomply/backup/jobs', {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: [...QK, 'jobs'] })
      void qc.invalidateQueries({ queryKey: [...QK, 'summary'] })
    },
  })
}

export function useUpdateBackupJob(id: string) {
  const qc = useQueryClient()
  return useMutation<BackupJob, Error, BackupJobInput>({
    mutationFn: (input) =>
      apiFetch<BackupJob>(`/vaktcomply/backup/jobs/${id}`, {
        method: 'PUT',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: [...QK, 'jobs'] })
      void qc.invalidateQueries({ queryKey: [...QK, 'summary'] })
    },
  })
}

export function useDeleteBackupJob() {
  const qc = useQueryClient()
  return useMutation<undefined, Error, string>({
    mutationFn: (id) =>
      apiFetch<undefined>(`/vaktcomply/backup/jobs/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: [...QK, 'jobs'] })
      void qc.invalidateQueries({ queryKey: [...QK, 'summary'] })
    },
  })
}

export function useRestoreTests(jobId: string | null) {
  return useQuery<BackupRestoreTest[]>({
    queryKey: [...QK, 'restore-tests', jobId],
    queryFn: () => apiFetch<BackupRestoreTest[]>(`/vaktcomply/backup/jobs/${jobId}/restore-tests`),
    enabled: !!jobId,
    staleTime: 5 * 60 * 1000,
  })
}

export function useCreateRestoreTest(jobId: string) {
  const qc = useQueryClient()
  return useMutation<BackupRestoreTest, Error, RestoreTestInput>({
    mutationFn: (input) =>
      apiFetch<BackupRestoreTest>(`/vaktcomply/backup/jobs/${jobId}/restore-tests`, {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: [...QK, 'restore-tests', jobId] })
      void qc.invalidateQueries({ queryKey: [...QK, 'jobs'] })
      void qc.invalidateQueries({ queryKey: [...QK, 'summary'] })
    },
  })
}
