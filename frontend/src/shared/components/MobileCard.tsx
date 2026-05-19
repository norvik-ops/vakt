interface MobileCardProps {
  title: string
  subtitle?: string
  badge?: { label: string; color: string }
  meta?: Array<{ label: string; value: React.ReactNode }>
  actions?: React.ReactNode
  onClick?: () => void
}

export function MobileCard({ title, subtitle, badge, meta, actions, onClick }: MobileCardProps) {
  return (
    <div
      className={`bg-surface border border-border rounded-lg p-4 ${onClick ? 'cursor-pointer hover:shadow-md transition-shadow' : ''}`}
      onClick={onClick}
    >
      <div className="flex items-start justify-between gap-2">
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <h3 className="font-medium text-sm text-primary truncate">{title}</h3>
            {badge && (
              <span className={`text-xs px-2 py-0.5 rounded-full font-medium flex-shrink-0 ${badge.color}`}>
                {badge.label}
              </span>
            )}
          </div>
          {subtitle && <p className="text-xs text-secondary mt-0.5 truncate">{subtitle}</p>}
        </div>
        {actions && <div className="flex-shrink-0">{actions}</div>}
      </div>
      {meta && meta.length > 0 && (
        <div className="mt-2 grid grid-cols-2 gap-x-4 gap-y-1">
          {meta.map(({ label, value }) => (
            <div key={label}>
              <span className="text-xs text-secondary">{label}</span>
              <div className="text-xs font-medium text-primary">{value}</div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
