import { describe, it, expect, beforeEach } from 'vitest'
import { useAuthStore } from './auth'

describe('auth store', () => {
  beforeEach(() => {
    useAuthStore.setState({ token: null, user: null })
  })

  it('initializes with null token', () => {
    expect(useAuthStore.getState().token).toBeNull()
  })

  it('setAuth stores the token', () => {
    useAuthStore.getState().setAuth('test-token-123', {
      id: '1', email: 'test@test.com', display_name: 'Test', roles: ['Viewer'],
    })
    expect(useAuthStore.getState().token).toBe('test-token-123')
  })

  it('clearAuth resets token to null', () => {
    useAuthStore.getState().setAuth('test-token-123', {
      id: '1', email: 'test@test.com', display_name: 'Test', roles: ['Viewer'],
    })
    useAuthStore.getState().clearAuth()
    expect(useAuthStore.getState().token).toBeNull()
  })
})
