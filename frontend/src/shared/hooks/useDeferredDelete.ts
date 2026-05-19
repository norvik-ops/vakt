import { useRef, useCallback } from 'react'
import { toast, dismissToast } from './useToast'

interface UseDeferredDeleteOptions<T> {
  /** Called when the timer expires — execute the actual DELETE API call here. */
  onDelete: (item: T) => Promise<void>
  /**
   * Called immediately when the user clicks "Rückgängig".
   * Typically: invalidate / refetch the query so the item reappears from server state.
   */
  onUndo?: (item: T) => void
  /** Returns the human-readable label shown in the toast ("${label}" gelöscht). */
  getLabel: (item: T) => string
  /** How long to wait before executing the delete. Defaults to 5000ms. */
  delayMs?: number
}

/**
 * Deferred-delete pattern:
 * 1. Caller removes the item optimistically from local state.
 * 2. A toast with "Rückgängig" appears for `delayMs` ms.
 * 3. If the user clicks "Rückgängig" → timer cancelled, `onUndo` is called.
 * 4. If the timer fires → `onDelete` is called and errors are surfaced via toast.
 */
export function useDeferredDelete<T>({
  onDelete,
  onUndo,
  getLabel,
  delayMs = 5000,
}: UseDeferredDeleteOptions<T>) {
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const cancelledRef = useRef(false)

  const scheduleDelete = useCallback(
    (item: T, onOptimisticRemove: () => void) => {
      // Cancel any in-flight pending delete before scheduling a new one
      if (timerRef.current) {
        clearTimeout(timerRef.current)
        timerRef.current = null
      }

      onOptimisticRemove()
      cancelledRef.current = false

      const label = getLabel(item)

      const toastId = toast(`„${label}" gelöscht`, {
        variant: 'info',
        duration: delayMs,
        action: {
          label: 'Rückgängig',
          onClick: () => {
            cancelledRef.current = true
            if (timerRef.current) {
              clearTimeout(timerRef.current)
              timerRef.current = null
            }
            onUndo?.(item)
          },
        },
      })

      timerRef.current = setTimeout(async () => {
        timerRef.current = null
        if (cancelledRef.current) return
        dismissToast(toastId)
        try {
          await onDelete(item)
        } catch {
          toast('Fehler beim Löschen', 'error')
        }
      }, delayMs)
    },
    [onDelete, onUndo, getLabel, delayMs],
  )

  return { scheduleDelete }
}
