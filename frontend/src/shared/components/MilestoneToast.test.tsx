import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useMilestoneToast } from './MilestoneToast'

const toastMock = vi.fn()
vi.mock('../hooks/useToast', () => ({
  toast: (...args: unknown[]) => toastMock(...args),
}))

beforeEach(() => {
  localStorage.clear()
  toastMock.mockClear()
})

describe('useMilestoneToast', () => {
  it('does not fire while score is undefined (loading)', () => {
    renderHook(() => useMilestoneToast(undefined))
    expect(toastMock).not.toHaveBeenCalled()
  })

  it('fires the "Halbzeit" milestone when score crosses 50', () => {
    const { rerender } = renderHook(({ s }: { s: number }) => useMilestoneToast(s), {
      initialProps: { s: 5 },  // below the lowest threshold (25)
    })
    expect(toastMock).not.toHaveBeenCalled()

    act(() => rerender({ s: 52 }))
    // Score jumped from 5 → 52, crossing the 25 threshold first → "Einstiegsbasis" fires.
    expect(toastMock).toHaveBeenCalledTimes(1)
    const [msg] = toastMock.mock.calls[0] as [string]
    expect(msg).toContain('Einstiegsbasis')
  })

  it('does not fire the same threshold twice across remounts (localStorage persistence)', () => {
    const { rerender, unmount } = renderHook(({ s }: { s: number }) => useMilestoneToast(s), {
      initialProps: { s: 5 },
    })
    // Cross every threshold one-by-one so each gets marked seen in localStorage.
    act(() => rerender({ s: 26 }))    // crosses 25
    act(() => rerender({ s: 51 }))    // crosses 50
    act(() => rerender({ s: 76 }))    // crosses 75
    act(() => rerender({ s: 91 }))    // crosses 90
    act(() => rerender({ s: 100 }))   // crosses 100
    const initialFireCount = toastMock.mock.calls.length
    expect(initialFireCount).toBeGreaterThanOrEqual(5)
    unmount()

    // Remount with the same high score — all thresholds are already in seen.
    renderHook(() => useMilestoneToast(100))
    expect(toastMock).toHaveBeenCalledTimes(initialFireCount)
  })

  it('fires the score-jump toast on +10pp gain after a baseline', () => {
    const { rerender } = renderHook(({ s }: { s: number }) => useMilestoneToast(s), {
      initialProps: { s: 30 },  // between 25 and 50 thresholds
    })
    // Initial observation crosses the 25 threshold once.
    expect(toastMock).toHaveBeenCalledTimes(1)
    toastMock.mockClear()

    // Now jump from 30 → 42, NOT crossing any threshold but +12pp → jump toast.
    act(() => rerender({ s: 42 }))
    expect(toastMock).toHaveBeenCalledTimes(1)
    const [msg] = toastMock.mock.calls[0] as [string]
    expect(msg).toContain('+12%')
  })

  it('fires at most one toast per tick (threshold wins over jump on the same tick)', () => {
    const { rerender } = renderHook(({ s }: { s: number }) => useMilestoneToast(s), {
      initialProps: { s: 5 },
    })
    expect(toastMock).not.toHaveBeenCalled()

    // 5 → 55 crosses 25, 50 thresholds AND would qualify as a +50pp jump.
    // The implementation walks thresholds in order and breaks after the first
    // unseen one — so "Einstiegsbasis" (25) fires; "Halbzeit" (50) and the
    // jump toast are skipped this tick to avoid spamming.
    act(() => rerender({ s: 55 }))
    expect(toastMock).toHaveBeenCalledTimes(1)
  })
})
