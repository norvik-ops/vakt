import { useState, useCallback } from 'react'

export type ToastVariant = 'success' | 'error' | 'info'

export interface ToastAction {
  label: string
  onClick: () => void
}

export interface ToastMessage {
  id: number
  message: string
  variant: ToastVariant
  action?: ToastAction
}

interface AddToastOptions {
  variant?: ToastVariant
  action?: ToastAction
  duration?: number
}

type AddToastFn = (msg: string, variantOrOptions?: ToastVariant | AddToastOptions) => number

let _addToast: AddToastFn | null = null
let _dismissToast: ((id: number) => void) | null = null
let _counter = 0

/**
 * Global toast store — used by the Toaster component and the toast() helper.
 */
export function useToastStore() {
  const [toasts, setToasts] = useState<ToastMessage[]>([])

  const dismiss = useCallback((id: number) => {
    setToasts((prev) => prev.filter((t) => t.id !== id))
  }, [])

  const addToast = useCallback<AddToastFn>((message, variantOrOptions) => {
    const id = ++_counter
    let variant: ToastVariant = 'info'
    let action: ToastAction | undefined
    let duration = 4000

    if (typeof variantOrOptions === 'string') {
      variant = variantOrOptions
    } else if (variantOrOptions) {
      variant = variantOrOptions.variant ?? 'info'
      action = variantOrOptions.action
      duration = variantOrOptions.duration ?? 4000
    }

    setToasts((prev) => [...prev, { id, message, variant, action }])
    setTimeout(() => {
      setToasts((prev) => prev.filter((t) => t.id !== id))
    }, duration)

    return id
  }, [])

  // Register the global handlers when this store mounts inside <Toaster />
  _addToast = addToast
  _dismissToast = dismiss

  return { toasts, addToast, dismiss }
}

/**
 * Imperative toast() helper — can be called from anywhere.
 * The Toaster component must be mounted for this to work.
 * Returns the toast id so callers can dismiss it early.
 */
export function toast(message: string, variantOrOptions?: ToastVariant | AddToastOptions): number {
  if (_addToast) {
    return _addToast(message, variantOrOptions)
  }
  return -1
}

/**
 * Imperative dismiss helper — dismisses a toast by id.
 */
export function dismissToast(id: number) {
  if (_dismissToast) _dismissToast(id)
}

/**
 * shadcn-compatible hook shim for components that use { toast } = useToast().
 */
export function useToast() {
  const toastFn = useCallback(
    ({ title, description, variant }: { title: string; description?: string; variant?: string }) => {
      const msg = description ? `${title}: ${description}` : title
      const v: ToastVariant = variant === 'destructive' ? 'error' : variant === 'default' ? 'success' : 'info'
      if (_addToast) _addToast(msg, v)
    },
    []
  )
  return { toast: toastFn }
}
