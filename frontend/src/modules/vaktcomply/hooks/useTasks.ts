import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type {
  CollabTask,
  CollabComment,
  CreateCollabTaskInput,
  UpdateCollabTaskInput,
  CreateCommentInput,
} from '../types'

// ── Tasks ──────────────────────────────────────────────────────────────────────

export function useTasks(entityType: string, entityId: string) {
  return useQuery<CollabTask[]>({
    queryKey: ['vaktcomply', entityType, entityId, 'collab-tasks'],
    queryFn: () =>
      apiFetch<CollabTask[]>(`/vaktcomply/${entityType}s/${entityId}/collab-tasks`),
    enabled: !!entityId && !!entityType,
    staleTime: 5 * 60 * 1000,
  })
}

export function useCreateTask(entityType: string, entityId: string) {
  const queryClient = useQueryClient()
  return useMutation<CollabTask, Error, CreateCollabTaskInput>({
    mutationFn: (input) =>
      apiFetch<CollabTask>(`/vaktcomply/${entityType}s/${entityId}/collab-tasks`, {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ['vaktcomply', entityType, entityId, 'collab-tasks'],
      })
    },
  })
}

export function useUpdateTask() {
  const queryClient = useQueryClient()
  return useMutation<CollabTask, Error, { taskId: string; entityType: string; entityId: string } & UpdateCollabTaskInput>({
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    mutationFn: ({ taskId, entityType: _entityType, entityId: _entityId, ...input }) =>
      apiFetch<CollabTask>(`/vaktcomply/collab-tasks/${taskId}`, {
        method: 'PATCH',
        body: JSON.stringify(input),
      }),
    onSuccess: (_data, vars) => {
      void queryClient.invalidateQueries({
        queryKey: ['vaktcomply', vars.entityType, vars.entityId, 'collab-tasks'],
      })
    },
  })
}

export function useDeleteTask(entityType: string, entityId: string) {
  const queryClient = useQueryClient()
  return useMutation<undefined, Error, string>({
    mutationFn: (taskId) =>
      apiFetch<undefined>(`/vaktcomply/collab-tasks/${taskId}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ['vaktcomply', entityType, entityId, 'collab-tasks'],
      })
    },
  })
}

// ── Comments ───────────────────────────────────────────────────────────────────

export function useComments(entityType: string, entityId: string) {
  return useQuery<CollabComment[]>({
    queryKey: ['vaktcomply', entityType, entityId, 'comments'],
    queryFn: () =>
      apiFetch<CollabComment[]>(`/vaktcomply/${entityType}s/${entityId}/comments`),
    enabled: !!entityId && !!entityType,
    staleTime: 2 * 60 * 1000,
  })
}

export function useCreateComment(entityType: string, entityId: string) {
  const queryClient = useQueryClient()
  return useMutation<CollabComment, Error, CreateCommentInput>({
    mutationFn: (input) =>
      apiFetch<CollabComment>(`/vaktcomply/${entityType}s/${entityId}/comments`, {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ['vaktcomply', entityType, entityId, 'comments'],
      })
    },
  })
}

export function useDeleteComment(entityType: string, entityId: string) {
  const queryClient = useQueryClient()
  return useMutation<void, Error, string>({
    mutationFn: (commentId) =>
      apiFetch<void>(`/vaktcomply/comments/${commentId}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ['vaktcomply', entityType, entityId, 'comments'],
      })
    },
  })
}
