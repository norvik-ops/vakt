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
    queryKey: ['secvitals', entityType, entityId, 'collab-tasks'],
    queryFn: () =>
      apiFetch<CollabTask[]>(`/secvitals/${entityType}s/${entityId}/collab-tasks`),
    enabled: !!entityId && !!entityType,
    staleTime: 5 * 60 * 1000,
  })
}

export function useCreateTask(entityType: string, entityId: string) {
  const queryClient = useQueryClient()
  return useMutation<CollabTask, Error, CreateCollabTaskInput>({
    mutationFn: (input) =>
      apiFetch<CollabTask>(`/secvitals/${entityType}s/${entityId}/collab-tasks`, {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ['secvitals', entityType, entityId, 'collab-tasks'],
      })
    },
  })
}

export function useUpdateTask() {
  const queryClient = useQueryClient()
  return useMutation<CollabTask, Error, { taskId: string; entityType: string; entityId: string } & UpdateCollabTaskInput>({
    mutationFn: ({ taskId, entityType: _et, entityId: _eid, ...input }) =>
      apiFetch<CollabTask>(`/secvitals/collab-tasks/${taskId}`, {
        method: 'PATCH',
        body: JSON.stringify(input),
      }),
    onSuccess: (_data, vars) => {
      void queryClient.invalidateQueries({
        queryKey: ['secvitals', vars.entityType, vars.entityId, 'collab-tasks'],
      })
    },
  })
}

export function useDeleteTask(entityType: string, entityId: string) {
  const queryClient = useQueryClient()
  return useMutation<void, Error, string>({
    mutationFn: (taskId) =>
      apiFetch<void>(`/secvitals/collab-tasks/${taskId}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ['secvitals', entityType, entityId, 'collab-tasks'],
      })
    },
  })
}

// ── Comments ───────────────────────────────────────────────────────────────────

export function useComments(entityType: string, entityId: string) {
  return useQuery<CollabComment[]>({
    queryKey: ['secvitals', entityType, entityId, 'comments'],
    queryFn: () =>
      apiFetch<CollabComment[]>(`/secvitals/${entityType}s/${entityId}/comments`),
    enabled: !!entityId && !!entityType,
    staleTime: 2 * 60 * 1000,
  })
}

export function useCreateComment(entityType: string, entityId: string) {
  const queryClient = useQueryClient()
  return useMutation<CollabComment, Error, CreateCommentInput>({
    mutationFn: (input) =>
      apiFetch<CollabComment>(`/secvitals/${entityType}s/${entityId}/comments`, {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ['secvitals', entityType, entityId, 'comments'],
      })
    },
  })
}

export function useDeleteComment(entityType: string, entityId: string) {
  const queryClient = useQueryClient()
  return useMutation<void, Error, string>({
    mutationFn: (commentId) =>
      apiFetch<void>(`/secvitals/comments/${commentId}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ['secvitals', entityType, entityId, 'comments'],
      })
    },
  })
}
