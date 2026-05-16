import type { LucideIcon } from 'lucide-react'
import { cn } from '../../lib/utils'

interface InfoBannerProps {
  icon?: LucideIcon
  title: string
  children: React.ReactNode
  variant?: 'info' | 'warning'
  className?: string
}

export function InfoBanner({ icon: Icon, title, children, variant = 'info', className }: InfoBannerProps) {
  const colors = variant === 'warning'
    ? 'bg-amber-500/5 border-amber-500/20 [&_p]:text-amber-300/70'
    : 'bg-brand/5 border-brand/20 [&_p]:text-secondary'

  return (
    <div className={cn('mx-6 mt-4 p-4 rounded-lg border text-sm', colors, className)}>
      <div className="flex items-start gap-3">
        {Icon && (
          <Icon className={cn('w-4 h-4 mt-0.5 shrink-0', variant === 'warning' ? 'text-amber-400' : 'text-brand')} />
        )}
        <div className="space-y-1">
          <p className="font-medium text-primary">{title}</p>
          <div className="text-secondary leading-relaxed">{children}</div>
        </div>
      </div>
    </div>
  )
}
