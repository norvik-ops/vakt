import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../../api/client'
import type { Environment, Secret, AccessLogPage } from '../types'

const BASE = '/vaktvault'

// ---- Environments ----

/**
 * Fetches all environments belonging to the given project.
 * The query is disabled while `projectId` is empty to avoid spurious API calls
 * during initial render before route params resolve.
 */
export function useEnvironments(projectId: string) {
  return useQuery<Environment[]>({
    queryKey: ['vaktvault', 'projects', projectId, 'envs'],
    queryFn: () => apiFetch<Environment[]>(`${BASE}/projects/${projectId}/envs`),
    staleTime: 30_000,
    enabled: Boolean(projectId),
  })
}

/**
 * Creates a new environment inside the given project and invalidates the
 * environment list cache so the UI reflects the addition immediately.
 */
export function useCreateEnvironment(projectId: string) {
  const queryClient = useQueryClient()
  return useMutation<Environment, Error, { name: string }>({
    mutationFn: (data) =>
      apiFetch<Environment>(`${BASE}/projects/${projectId}/envs`, {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ['vaktvault', 'projects', projectId, 'envs'],
      })
    },
  })
}

// ---- Secrets ----

/**
 * Fetches the list of secret key names for an environment.
 * Values are never returned here — use `useSecretValue` to decrypt a specific key
 * on demand, minimising the blast radius of a compromised session.
 */
export function useSecretKeys(projectId: string, envId: string) {
  return useQuery<string[]>({
    queryKey: ['vaktvault', 'projects', projectId, 'envs', envId, 'secrets'],
    queryFn: () =>
      apiFetch<string[]>(`${BASE}/projects/${projectId}/envs/${envId}/secrets`),
    staleTime: 30_000,
    enabled: Boolean(projectId) && Boolean(envId),
  })
}

/**
 * Fetches and decrypts a single secret value when `enabled` is true.
 * `staleTime` and `gcTime` are both 0 so the plaintext value is never held in
 * the React Query cache longer than the current render cycle, reducing exposure
 * in memory and devtools.
 */
export function useSecretValue(projectId: string, envId: string, key: string, enabled: boolean) {
  return useQuery<Secret>({
    queryKey: ['vaktvault', 'projects', projectId, 'envs', envId, 'secrets', key],
    queryFn: () =>
      apiFetch<Secret>(`${BASE}/projects/${projectId}/envs/${envId}/secrets/${encodeURIComponent(key)}`),
    staleTime: 0,
    enabled: enabled && Boolean(projectId) && Boolean(envId) && Boolean(key),
    gcTime: 0,
  })
}

/**
 * Creates or updates (upserts) a secret key/value pair within an environment.
 * On success, invalidates both the key-list and the individual value cache entry
 * so stale plaintext is not served from a prior `useSecretValue` call.
 */
export function useUpsertSecret(projectId: string, envId: string) {
  const queryClient = useQueryClient()
  return useMutation<undefined, Error, { key: string; value: string }>({
    mutationFn: ({ key, value }) =>
      apiFetch<undefined>(
        `${BASE}/projects/${projectId}/envs/${envId}/secrets/${encodeURIComponent(key)}`,
        {
          method: 'PUT',
          body: JSON.stringify({ value }),
        },
      ),
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({
        queryKey: ['vaktvault', 'projects', projectId, 'envs', envId, 'secrets'],
      })
      // Invalidate the specific secret value cache
      void queryClient.invalidateQueries({
        queryKey: [
          'vaktvault',
          'projects',
          projectId,
          'envs',
          envId,
          'secrets',
          variables.key,
        ],
      })
    },
  })
}

/**
 * Fetches a paginated slice of the project's secret-access audit log.
 *
 * Both `page` and `limit` are included in the query key so that navigating
 * between pages produces isolated cache entries — changing `limit` on the same
 * `page` will not return stale data from a different page size.
 *
 * @param projectId - UUID of the SecVault project.
 * @param page      - 1-based page number (default 1).
 * @param limit     - Entries per page (default 25).
 */
export function useProjectAccessLog(projectId: string, page = 1, limit = 25) {
  return useQuery<AccessLogPage>({
    queryKey: ['vaktvault', 'projects', projectId, 'access-log', page, limit],
    queryFn: () =>
      apiFetch<AccessLogPage>(`${BASE}/projects/${projectId}/access-log?page=${String(page)}&limit=${String(limit)}`),
    staleTime: 30_000,
    enabled: Boolean(projectId),
  })
}

/**
 * Permanently deletes a secret by key from the given environment.
 * Invalidates the key-list cache so deleted keys disappear from the UI without
 * requiring a manual refresh.
 */
export function useDeleteSecret(projectId: string, envId: string) {
  const queryClient = useQueryClient()
  return useMutation<undefined, Error, string>({
    mutationFn: (key) =>
      apiFetch<undefined>(
        `${BASE}/projects/${projectId}/envs/${envId}/secrets/${encodeURIComponent(key)}`,
        { method: 'DELETE' },
      ),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ['vaktvault', 'projects', projectId, 'envs', envId, 'secrets'],
      })
    },
  })
}
