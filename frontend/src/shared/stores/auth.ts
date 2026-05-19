import { create } from 'zustand'
import { getUserInfo, setUserInfo, setAuthToken } from '../../api/client'

interface User {
  id: string
  email: string
  display_name: string
  roles: string[]
}

interface AuthState {
  user: User | null
  setAuth: (user: User) => void
  clearAuth: () => void
  isAuthenticated: () => boolean
}

// Hydrate from localStorage on store creation so page refreshes stay logged in.
function hydrateUser(): User | null {
  const info = getUserInfo()
  if (!info) return null
  return {
    id: info.id,
    email: info.email,
    display_name: info.display_name ?? info.email,
    roles: info.roles ?? (info.role ? [info.role] : []),
  }
}

export const useAuthStore = create<AuthState>((set, get) => ({
  user: hydrateUser(),
  setAuth: (user) => {
    setUserInfo({
      id: user.id,
      email: user.email,
      display_name: user.display_name,
      roles: user.roles,
      role: user.roles[0] ?? '',
    })
    set({ user })
  },
  clearAuth: () => {
    setAuthToken(null) // clears vakt_user from localStorage
    set({ user: null })
  },
  isAuthenticated: () => !!get().user || getUserInfo() !== null,
}))
