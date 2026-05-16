interface PageHeaderProps {
  title: string
  description?: string
  actions?: React.ReactNode
}

export function PageHeader({ title, description, actions }: PageHeaderProps) {
  return (
    <div className="flex items-start justify-between px-6 pt-6 pb-0 mb-0">
      <div>
        <h1 className="text-[20px] font-bold text-primary leading-tight">{title}</h1>
        {description && <p className="mt-1 text-[12px] text-secondary">{description}</p>}
      </div>
      {actions && <div className="flex items-center gap-2">{actions}</div>}
    </div>
  )
}
