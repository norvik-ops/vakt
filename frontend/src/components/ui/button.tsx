import * as React from 'react'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '../../lib/utils'

const buttonVariants = cva(
  'inline-flex items-center justify-center gap-1.5 rounded-md text-[13px] font-semibold transition-all duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand disabled:pointer-events-none disabled:opacity-50',
  {
    variants: {
      variant: {
        default:     'bg-brand text-white hover:bg-brand-hover',
        destructive: 'bg-red-600 text-white hover:bg-red-700',
        outline:     'border border-border bg-transparent text-primary hover:bg-[#f1f5f9] dark:hover:bg-[#1E2235]',
        secondary:   'bg-surface2 border border-border text-primary hover:bg-[#e2e8f0] dark:hover:bg-[#1E2235]',
        ghost:       'text-secondary hover:bg-[#f1f5f9] dark:hover:bg-[#1E2235] hover:text-primary',
        link:        'text-brand underline-offset-4 hover:underline',
      },
      size: {
        default: 'h-8 px-4 py-2',
        sm:      'h-7 px-3 text-xs',
        lg:      'h-10 px-6',
        icon:    'h-8 w-8',
      },
    },
    defaultVariants: { variant: 'default', size: 'default' },
  },
)

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {}

const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, ...props }, ref) => (
    <button ref={ref} className={cn(buttonVariants({ variant, size, className }))} {...props} />
  ),
)
Button.displayName = 'Button'

export { Button, buttonVariants }
