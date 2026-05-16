import * as React from 'react'
import { cn } from '../../lib/utils'

export type TextareaProps = React.TextareaHTMLAttributes<HTMLTextAreaElement>

const Textarea = React.forwardRef<HTMLTextAreaElement, TextareaProps>(({ className, ...props }, ref) => (
  <textarea
    className={cn(
      'flex w-full rounded-md border border-border bg-surface px-3 py-2 text-[13px] text-primary',
      'placeholder:text-muted transition-colors resize-none',
      'focus-visible:outline-none focus-visible:border-brand focus-visible:ring-0',
      'disabled:cursor-not-allowed disabled:opacity-50',
      className,
    )}
    ref={ref}
    {...props}
  />
))
Textarea.displayName = 'Textarea'

export { Textarea }
