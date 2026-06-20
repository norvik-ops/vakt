import { useEffect, useState, type RefObject } from 'react'
import { X } from 'lucide-react'
import { useFocusTrap } from '../hooks/useFocusTrap'
import { cn } from '../../lib/utils'

interface SlideOverProps {
  open: boolean
  onClose: () => void
  title: string
  description?: string
  /** Width preset for the panel. Use 'lg' for detail-views with rich sidebars. */
  width?: 'sm' | 'md' | 'lg'
  children: React.ReactNode
  /** Optional footer rendered sticky at the bottom of the panel. */
  footer?: React.ReactNode
}

const WIDTHS = {
  sm: 'max-w-md',
  md: 'max-w-2xl',
  lg: 'max-w-4xl',
}

/**
 * Right-side panel that slides over the current view instead of navigating
 * away. Used for detail-views (Control, Risk, Finding) where the user wants
 * to inspect/edit without losing list context — Linear-style pattern.
 *
 * ponytail: CSS-transition based (S98-1) — framer-motion's AnimatePresence was
 * the only reason this always-eager component pulled ~41 KiB into the initial
 * bundle. We keep the enter+exit feel by staying mounted through the close
 * animation (`closing` state) and unmounting on animationEnd.
 *
 * Why a custom component instead of Radix Dialog with side="right":
 * — Radix Dialog renders inside a Portal that traps body scroll; for a
 *   right-aligned panel we want the underlying page to remain visible and
 *   non-scrolling, but interactive (dismiss via dimmed overlay or Esc).
 * — A drawer/sheet primitive isn't installed in this codebase.
 */
export function SlideOver({
  open,
  onClose,
  title,
  description,
  width = 'md',
  children,
  footer,
}: SlideOverProps) {
  const trapRef = useFocusTrap<HTMLDivElement>(open, onClose)
  // `render` stays true during the close animation so the exit transition plays.
  const [render, setRender] = useState(open)

  useEffect(() => {
    if (open) setRender(true)
  }, [open])

  // Prevent body scroll while the panel is visible.
  useEffect(() => {
    if (!render) return
    const previous = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      document.body.style.overflow = previous
    }
  }, [render])

  if (!render) return null

  return (
    <>
      <div
        className={cn('fixed inset-0 z-40 bg-black/40', open ? 'vakt-overlay-in' : 'vakt-overlay-out')}
        onClick={onClose}
        aria-hidden="true"
      />
      <div
        ref={trapRef as RefObject<HTMLDivElement>}
        role="dialog"
        aria-modal="true"
        aria-label={title}
        className={cn(
          'fixed right-0 top-0 bottom-0 z-50 w-full bg-surface border-l border-border shadow-2xl flex flex-col',
          open ? 'vakt-slide-in' : 'vakt-slide-out',
          WIDTHS[width],
        )}
        // When the close animation finishes, unmount.
        onAnimationEnd={() => { if (!open) setRender(false) }}
      >
        <header className="flex items-start justify-between gap-4 px-6 py-4 border-b border-border shrink-0">
          <div className="min-w-0">
            <h2 className="text-base font-semibold text-primary truncate">{title}</h2>
            {description && (
              <p className="text-xs text-secondary mt-1 line-clamp-2">{description}</p>
            )}
          </div>
          <button
            type="button"
            onClick={onClose}
            aria-label="Schließen"
            className="text-secondary hover:text-primary p-1 rounded -m-1 shrink-0"
          >
            <X className="w-4 h-4" aria-hidden="true" />
          </button>
        </header>

        <div className="flex-1 overflow-y-auto px-6 py-5">{children}</div>

        {footer && (
          <footer className="px-6 py-3 border-t border-border shrink-0 bg-surface">
            {footer}
          </footer>
        )}
      </div>
    </>
  )
}
