import { cn } from '../../lib/utils'

function Skeleton({ className, 'aria-label': ariaLabel, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return (
    /* WCAG 4.1.3: role="status" + aria-label announces loading state to screen readers */
    <div
      role="status"
      aria-label={ariaLabel ?? 'Wird geladen'}
      className={cn('animate-pulse rounded-md bg-border/60', className)}
      {...props}
    />
  )
}

export { Skeleton }
