import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useFirstAction } from './useFirstAction'

// Mock the toast module — we want to assert *that* it gets called, not
// instantiate the real Toaster which would need a Provider in the test tree.
const toastMock = vi.fn()
vi.mock('./useToast', () => ({
  toast: (...args: unknown[]) => toastMock(...args),
}))

beforeEach(() => {
  localStorage.clear()
  toastMock.mockClear()
})

describe('useFirstAction', () => {
  it('does NOT fire when condition is already true on initial mount', () => {
    renderHook(() => useFirstAction('control:first-created', true))
    expect(toastMock).not.toHaveBeenCalled()
  })

  it('fires once on transition false → true', () => {
    const { rerender } = renderHook(({ cond }: { cond: boolean }) => useFirstAction('control:first-created', cond), {
      initialProps: { cond: false },
    })
    expect(toastMock).not.toHaveBeenCalled()

    act(() => rerender({ cond: true }))
    expect(toastMock).toHaveBeenCalledTimes(1)
  })

  it('persists in localStorage and does not fire again after remount', () => {
    const { rerender, unmount } = renderHook(({ cond }: { cond: boolean }) => useFirstAction('control:first-created', cond), {
      initialProps: { cond: false },
    })
    act(() => rerender({ cond: true }))
    expect(toastMock).toHaveBeenCalledTimes(1)
    unmount()

    // Remount the hook — should NOT fire again even though condition is true,
    // because the key is already in localStorage.
    renderHook(() => useFirstAction('control:first-created', true))
    expect(toastMock).toHaveBeenCalledTimes(1)
  })

  it('silently no-ops for unknown keys', () => {
    const { rerender } = renderHook(({ cond }: { cond: boolean }) => useFirstAction('unknown:key', cond), {
      initialProps: { cond: false },
    })
    act(() => rerender({ cond: true }))
    // Unknown keys are skipped — no toast.
    expect(toastMock).not.toHaveBeenCalled()
  })
})
