import { useAuthStore } from '../shared/stores/auth'

/**
 * Puts a signed-in user with the given backend role into the auth store.
 * Pages hide write buttons for roles without write rights (useCanWrite), so a
 * test that clicks „anlegen"/„speichern" needs a writing role first.
 */
export function signInAs(role: 'Admin' | 'SecurityAnalyst' | 'Viewer' | 'AuditorReadOnly' | 'InternalAuditor' = 'Admin'): void {
  useAuthStore.setState({
    user: { id: 'u-test', email: `${role.toLowerCase()}@test.local`, display_name: role, roles: [role] },
    hydrating: false,
  })
}
