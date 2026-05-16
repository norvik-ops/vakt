import * as React from 'react'
import { cn } from '../../lib/utils'

export type InputProps = React.InputHTMLAttributes<HTMLInputElement>

const Input = React.forwardRef<HTMLInputElement, InputProps>(({ className, type, ...props }, ref) => (
  <input
    type={type}
    className={cn(
      'flex h-8 w-full rounded-md border border-border bg-surface px-3 py-1 text-[13px] text-primary',
      'placeholder:text-muted transition-colors',
      'focus-visible:outline-none focus-visible:border-brand focus-visible:ring-0',
      'disabled:cursor-not-allowed disabled:opacity-50',
      className,
    )}
    ref={ref}
    {...props}
  />
))
Input.displayName = 'Input'

export { Input }
