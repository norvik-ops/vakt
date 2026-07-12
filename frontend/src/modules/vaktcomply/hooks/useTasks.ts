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

// S125 (FE-03): map the singular entity type to the exact backend path segment.
// The old `${entityType}s` silently produced `policys` for `policy` (the backend
// registers `policies`); naive pluralisation was a footgun waiting for the next
// non-regular entity. Keep this in lock-step with the backend loop in
// vaktcomply/routes.go (controls, risks, incidents, policies, audits).
const ENTITY_PATH: Record<string, string> = {
  control: 'controls',
  risk: 'risks',
  incident: 'incidents',
  policy: 'policies',
  audit: 'audits',
}

function entityPath(entityType: string): string {
  return ENTITY_PATH[entityType] ?? `${entityType}s`
}

export function useTasks(entityType: string, entityId: string) {
  return useQuery<CollabTask[]>({
    queryKey: ['vaktcomply', entityType, entityId, 'collab-tasks'],
    queryFn: () =>
      apiFetch<CollabTask[]>(`/vaktcomply/${entityPath(entityType)}/${entityId}/collab-tasks`),
    enabled: !!entityId && !!entityType,
    staleTime: 5 * 60 * 1000,
  })
}

export function useCreateTask(entityType: string, entityId: string) {
  const queryClient = useQueryClient()
  return useMutation<CollabTask, Error, CreateCollabTaskInput>({
    mutationFn: (input) =>
      apiFetch<CollabTask>(`/vaktcomply/${entityPath(entityType)}/${entityId}/collab-tasks`, {
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
      apiFetch<CollabComment[]>(`/vaktcomply/${entityPath(entityType)}/${entityId}/comments`),
    enabled: !!entityId && !!entityType,
    staleTime: 2 * 60 * 1000,
  })
}

export function useCreateComment(entityType: string, entityId: string) {
  const queryClient = useQueryClient()
  return useMutation<CollabComment, Error, CreateCommentInput>({
    mutationFn: (input) =>
      apiFetch<CollabComment>(`/vaktcomply/${entityPath(entityType)}/${entityId}/comments`, {
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
