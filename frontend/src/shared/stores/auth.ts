import { create } from 'zustand'
import { fetchMe, registerUnauthorizedHandler, setSessionId } from '../../api/client'

interface User {
  id: string
  email: string
  display_name: string
  roles: string[]
}

interface AuthState {
  user: User | null
  // hydrating is true on app start until /auth/me has been queried at least
  // once. Route guards use it to render a loading state instead of
  // bouncing the user to /login while we're still finding out who they are.
  hydrating: boolean
  setAuth: (user: User) => void
  clearAuth: () => void
  hydrate: () => Promise<void>
  isAuthenticated: () => boolean
}

export const useAuthStore = create<AuthState>((set, get) => ({
  user: null,
  hydrating: true,
  setAuth: (user) => { set({ user, hydrating: false }) },
  clearAuth: () => {
    setSessionId(null)
    set({ user: null, hydrating: false })
  },
  hydrate: async () => {
    // One-time cleanup of the deprecated localStorage snapshot (audit F032).
    // Safe to remove unconditionally — never read any more.
    try { localStorage.removeItem('vakt_user') } catch { /* SSR / private mode */ }
    const me = await fetchMe()
    if (me) {
      set({
        user: {
          id: me.id,
          email: me.email,
          display_name: me.display_name,
          roles: me.roles,
        },
        hydrating: false,
      })
    } else {
      set({ user: null, hydrating: false })
    }
  },
  isAuthenticated: () => !!get().user,
}))

// Wire apiFetch → store so a 401 anywhere clears the in-memory user before
// the redirect to /login. Done at module import time; no static cycle since
// client.ts does not import the store.
registerUnauthorizedHandler(() => {
  useAuthStore.setState({ user: null, hydrating: false })
})
