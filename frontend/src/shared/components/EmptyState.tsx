import type { LucideIcon } from 'lucide-react'

interface EmptyStateProps {
  icon: LucideIcon
  title: string
  description: string
  action?: React.ReactNode
  size?: 'sm' | 'md'
}

export function EmptyState({ icon: Icon, title, description, action, size = 'md' }: EmptyStateProps) {
  const py = size === 'sm' ? 'py-10' : 'py-16'
  const iconBox = size === 'sm' ? 'w-10 h-10' : 'w-12 h-12'
  const iconSize = size === 'sm' ? 'w-5 h-5' : 'w-6 h-6'

  return (
    // ponytail: CSS fade-in-up replaces framer-motion (S98-1 — keeps vendor-motion out of initial bundle)
    <div className={`vakt-fade-in-up flex flex-col items-center justify-center ${py} text-center`}>
      <div className={`${iconBox} rounded-xl bg-brand/10 border border-brand/20 flex items-center justify-center mb-4`}>
        <Icon className={`${iconSize} text-brand`} />
      </div>
      <h3 className="text-sm font-semibold text-primary">{title}</h3>
      <p className="mt-1 text-sm text-secondary max-w-sm">{description}</p>
      {action && <div className="mt-4">{action}</div>}
    </div>
  )
}
