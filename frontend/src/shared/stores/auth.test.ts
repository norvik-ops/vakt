import { describe, it, expect, beforeEach } from 'vitest'
import { useAuthStore } from './auth'

const testUser = { id: '1', email: 'test@test.com', display_name: 'Test', roles: ['Viewer'] }

describe('auth store', () => {
  beforeEach(() => {
    useAuthStore.setState({ user: null })
    localStorage.clear()
  })

  it('initializes with null user', () => {
    expect(useAuthStore.getState().user).toBeNull()
  })

  it('setAuth stores the user', () => {
    useAuthStore.getState().setAuth(testUser)
    expect(useAuthStore.getState().user).toEqual(testUser)
  })

  it('isAuthenticated returns false when no user', () => {
    expect(useAuthStore.getState().isAuthenticated()).toBe(false)
  })

  it('isAuthenticated returns true after setAuth', () => {
    useAuthStore.getState().setAuth(testUser)
    expect(useAuthStore.getState().isAuthenticated()).toBe(true)
  })

  it('clearAuth resets user to null', () => {
    useAuthStore.getState().setAuth(testUser)
    useAuthStore.getState().clearAuth()
    expect(useAuthStore.getState().user).toBeNull()
  })
})
