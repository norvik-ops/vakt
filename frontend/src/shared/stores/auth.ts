import { create } from 'zustand'
import { setAuthToken, getAuthToken } from '../../api/client'

interface User {
  id: string
  email: string
  display_name: string
  roles: string[]
}

interface AuthState {
  token: string | null
  user: User | null
  setAuth: (token: string, user: User) => void
  clearAuth: () => void
  isAuthenticated: () => boolean
}

export const useAuthStore = create<AuthState>((set, get) => ({
  token: getAuthToken(),
  user: null,
  setAuth: (token, user) => {
    setAuthToken(token)
    set({ token, user })
  },
  clearAuth: () => {
    setAuthToken(null)
    set({ token: null, user: null })
  },
  isAuthenticated: () => !!get().token,
}))
