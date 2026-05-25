import { DemoTierHint } from './DemoTierHint'

interface PageHeaderProps {
  title: string
  description?: string
  actions?: React.ReactNode
  tier?: 'pro' | 'enterprise'
}

// Uses <header> (implicit role="banner") for proper landmark semantics (WCAG 1.3.1).
export function PageHeader({ title, description, actions, tier }: PageHeaderProps) {
  return (
    <header className="flex items-start justify-between px-6 pt-6 pb-0 mb-0">
      <div>
        <div className="flex items-center gap-2">
          <h1 className="text-[20px] font-bold text-primary leading-tight">{title}</h1>
          {tier && <DemoTierHint tier={tier} />}
        </div>
        {description && <p className="mt-1 text-[12px] text-secondary">{description}</p>}
      </div>
      {actions && <div className="flex items-center gap-2">{actions}</div>}
    </header>
  )
}
