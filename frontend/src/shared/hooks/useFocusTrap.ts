import { useEffect, useRef } from 'react'

/**
 * Constrains keyboard focus to a container element while it is mounted.
 *
 * - Tab cycles forward through focusable descendants, looping back to the first.
 * - Shift+Tab cycles backward, looping to the last.
 * - Escape calls onEscape (if provided).
 * - On mount, the previously-focused element is captured and restored on unmount.
 *
 * Use for custom dialogs / overlays that are NOT built on Radix UI's Dialog
 * primitive (Radix has focus-trap built in — don't double up).
 *
 * Usage:
 *   const ref = useFocusTrap<HTMLDivElement>(open, () => setOpen(false))
 *   return <div ref={ref} role="dialog" aria-modal="true">…</div>
 */
export function useFocusTrap<T extends HTMLElement>(
  active: boolean,
  onEscape?: () => void,
): React.RefObject<T | null> {
  const ref = useRef<T>(null)

  useEffect(() => {
    if (!active) return
    const container = ref.current
    if (!container) return

    const previouslyFocused = document.activeElement as HTMLElement | null

    // Focus the first focusable element inside the container on activation.
    const getFocusable = (): HTMLElement[] => {
      const selector = [
        'a[href]',
        'button:not([disabled])',
        'textarea:not([disabled])',
        'input:not([disabled]):not([type="hidden"])',
        'select:not([disabled])',
        '[tabindex]:not([tabindex="-1"])',
      ].join(',')
      return Array.from(container.querySelectorAll<HTMLElement>(selector)).filter(
        el => el.offsetParent !== null,
      )
    }

    // Initial focus: first focusable, or the container itself.
    const focusable = getFocusable()
    if (focusable.length > 0) {
      focusable[0].focus()
    } else if (container.tabIndex >= 0) {
      container.focus()
    }

    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === 'Escape' && onEscape) {
        e.preventDefault()
        onEscape()
        return
      }
      if (e.key !== 'Tab') return
      const items = getFocusable()
      if (items.length === 0) {
        e.preventDefault()
        return
      }
      const first = items[0]
      const last = items[items.length - 1]
      const activeEl = document.activeElement as HTMLElement | null
      if (e.shiftKey && activeEl === first) {
        e.preventDefault()
        last.focus()
      } else if (!e.shiftKey && activeEl === last) {
        e.preventDefault()
        first.focus()
      }
    }

    container.addEventListener('keydown', handleKeyDown)
    return () => {
      container.removeEventListener('keydown', handleKeyDown)
      // Restore focus to where it was before the trap activated, if that
      // element is still in the DOM and focusable.
      if (previouslyFocused && document.body.contains(previouslyFocused)) {
        previouslyFocused.focus()
      }
    }
  }, [active, onEscape])

  return ref
}
