import { useEffect, type RefObject } from 'react'
import { X } from 'lucide-react'
import { motion, AnimatePresence } from 'framer-motion'
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
 * Built on framer-motion for the slide animation and useFocusTrap for
 * keyboard accessibility (Tab cycles inside the panel, Esc closes).
 *
 * Why a custom component instead of Radix Dialog with side="right":
 * — Radix Dialog renders inside a Portal that traps body scroll; for a
 *   right-aligned panel we want the underlying page to remain visible and
 *   non-scrolling, but interactive (so the user can dismiss by clicking
 *   the dimmed overlay or hitting Esc).
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

  // Prevent body scroll while the panel is open.
  useEffect(() => {
    if (!open) return
    const previous = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      document.body.style.overflow = previous
    }
  }, [open])

  return (
    <AnimatePresence>
      {open && (
        <>
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.15 }}
            className="fixed inset-0 z-40 bg-black/40"
            onClick={onClose}
            aria-hidden="true"
          />
          <motion.div
            ref={trapRef as RefObject<HTMLDivElement>}
            initial={{ x: '100%' }}
            animate={{ x: 0 }}
            exit={{ x: '100%' }}
            transition={{ type: 'spring', stiffness: 300, damping: 30 }}
            role="dialog"
            aria-modal="true"
            aria-label={title}
            className={cn(
              'fixed right-0 top-0 bottom-0 z-50 w-full bg-surface border-l border-border shadow-2xl flex flex-col',
              WIDTHS[width],
            )}
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
          </motion.div>
        </>
      )}
    </AnimatePresence>
  )
}
