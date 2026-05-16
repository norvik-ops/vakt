import * as React from 'react'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '../../lib/utils'

const badgeVariants = cva(
  'inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-semibold transition-colors',
  {
    variants: {
      variant: {
        default:     'bg-brand/20 text-indigo-400 border border-brand/30',
        secondary:   'bg-surface2 text-secondary border border-border',
        destructive: 'bg-[#7f1d1d] text-[#ef4444] border-transparent',
        success:     'bg-[#14532d] text-[#86efac] border-transparent',
        warning:     'bg-[#78350f] text-[#f59e0b] border-transparent',
        outline:     'border border-border text-secondary',
      },
    },
    defaultVariants: { variant: 'default' },
  },
)

export interface BadgeProps
  extends React.HTMLAttributes<HTMLDivElement>,
    VariantProps<typeof badgeVariants> {}

function Badge({ className, variant, ...props }: BadgeProps) {
  return <div className={cn(badgeVariants({ variant }), className)} {...props} />
}

export { Badge, badgeVariants }
