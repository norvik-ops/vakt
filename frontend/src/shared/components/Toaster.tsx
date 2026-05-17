import * as RadixToast from '@radix-ui/react-toast'
import { X, CheckCircle2, AlertCircle, Info } from 'lucide-react'
import { cn } from '../../lib/utils'
import { useToastStore, type ToastVariant } from '../hooks/useToast'

const VARIANT_CONFIG: Record<ToastVariant, { icon: React.ElementType; className: string; iconClass: string }> = {
  success: {
    icon: CheckCircle2,
    className: 'border-green-500/30 bg-green-500/10',
    iconClass: 'text-green-500',
  },
  error: {
    icon: AlertCircle,
    className: 'border-red-500/30 bg-red-500/10',
    iconClass: 'text-red-500',
  },
  info: {
    icon: Info,
    className: 'border-brand/30 bg-brand/10',
    iconClass: 'text-brand',
  },
}

export function Toaster() {
  const { toasts, dismiss } = useToastStore()

  return (
    <RadixToast.Provider swipeDirection="right">
      {toasts.map(({ id, message, variant }) => {
        const cfg = VARIANT_CONFIG[variant]
        const Icon = cfg.icon
        return (
          <RadixToast.Root
            key={id}
            open
            onOpenChange={(open) => { if (!open) dismiss(id) }}
            /* WCAG 4.1.3: role="alert" for errors (assertive); role="status" for others (polite) */
            role={variant === 'error' ? 'alert' : 'status'}
            aria-live={variant === 'error' ? 'assertive' : 'polite'}
            aria-atomic="true"
            className={cn(
              'flex items-start gap-3 rounded-lg border px-4 py-3 shadow-lg',
              'data-[state=open]:animate-in data-[state=closed]:animate-out',
              'data-[state=closed]:fade-out-80 data-[state=open]:fade-in-0',
              'data-[state=closed]:slide-out-to-right-full data-[state=open]:slide-in-from-top-full',
              'bg-surface text-primary text-sm',
              cfg.className,
            )}
          >
            {/* WCAG 1.1.1: icon is decorative — the text message conveys meaning */}
            <Icon className={cn('w-4 h-4 shrink-0 mt-0.5', cfg.iconClass)} aria-hidden="true" />
            <RadixToast.Title className="flex-1 leading-snug">{message}</RadixToast.Title>
            <RadixToast.Close
              className="text-secondary hover:text-primary transition-colors shrink-0 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand rounded"
              aria-label="Schließen"
              onClick={() => dismiss(id)}
            >
              <X className="w-3.5 h-3.5" aria-hidden="true" />
            </RadixToast.Close>
          </RadixToast.Root>
        )
      })}
      {/* WCAG 4.1.3: Viewport is the live region anchor for screen readers */}
      <RadixToast.Viewport
        className="fixed top-4 right-4 z-50 flex flex-col gap-2 w-[360px] max-w-[calc(100vw-2rem)]"
        aria-label="Benachrichtigungen"
      />
    </RadixToast.Provider>
  )
}
