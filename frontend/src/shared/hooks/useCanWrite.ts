import { useAuthStore } from '../stores/auth'

/**
 * Roles the backend lets write in Vakt Comply: `auth.RequireRole("Admin", "SecurityAnalyst")`
 * (`rw` in backend/internal/modules/vaktcomply/routes.go and assets/routes.go).
 * Viewer and AuditorReadOnly only read.
 *
 * The UI uses this to hide write buttons — comfort, not protection. The backend
 * stays the lock: every write route answers 403 for these roles regardless.
 */
export const WRITE_ROLES: readonly string[] = ['Admin', 'SecurityAnalyst']

export function canWriteWithRoles(roles: readonly string[] | undefined | null): boolean {
  return (roles ?? []).some((r) => WRITE_ROLES.includes(r))
}

/** True when the signed-in user may create, change or delete records in Vakt Comply. */
export function useCanWrite(): boolean {
  const roles = useAuthStore((s) => s.user?.roles)
  return canWriteWithRoles(roles)
}

/** True for the Admin role (routes gated with `auth.RequireRole("Admin")`). */
export function useIsAdmin(): boolean {
  const roles = useAuthStore((s) => s.user?.roles)
  return (roles ?? []).includes('Admin')
}
