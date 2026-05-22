import { cn } from '../lib/utils'

type SpinnerSize = 'xs' | 'sm' | 'md' | 'lg'
type SpinnerColor = 'brand' | 'current' | 'white' | 'primary'

const sizeClasses: Record<SpinnerSize, string> = {
  xs: 'w-3 h-3 border',
  sm: 'w-4 h-4 border-2',
  md: 'w-5 h-5 border-2',
  lg: 'w-6 h-6 border-2',
}

const colorClasses: Record<SpinnerColor, string> = {
  brand: 'border-brand border-t-transparent',
  current: 'border-current border-t-transparent',
  white: 'border-white border-t-transparent',
  primary: 'border-primary border-t-transparent',
}

interface SpinnerProps extends React.HTMLAttributes<HTMLSpanElement> {
  size?: SpinnerSize
  color?: SpinnerColor
}

export function Spinner({ size = 'md', color = 'brand', className, ...props }: SpinnerProps) {
  return (
    <span
      className={cn(
        'inline-block rounded-full animate-spin shrink-0',
        sizeClasses[size],
        colorClasses[color],
        className,
      )}
      aria-hidden="true"
      {...props}
    />
  )
}
