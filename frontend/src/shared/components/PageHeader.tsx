import { DemoTierHint } from './DemoTierHint'

interface PageHeaderProps {
  title: string
  description?: string
  actions?: React.ReactNode
  /** Siehe DemoTierHint: es gibt genau ein bezahltes Tier. */
  tier?: 'pro'
}

// Uses <header> (implicit role="banner") for proper landmark semantics (WCAG 1.3.1).
//
// UX-39 (Handy, 390 px): Titel und Aktionen standen nebeneinander ohne Umbruch —
// Knöpfe wie „Zurück" liefen aus dem Bild, der Titel wurde auf Wortbreite gequetscht.
// Jetzt bricht die Kopfzeile um: Reicht der Platz nicht, rutschen die Aktionen unter
// den Titel und brechen ihrerseits um. Viele Seiten reichen ihre Knöpfe in einem
// eigenen <div className="flex gap-2"> herein; `[&>div]:flex-wrap` lässt auch diese
// Hülle umbrechen, ohne jede Seite anzufassen.
export function PageHeader({ title, description, actions, tier }: PageHeaderProps) {
  return (
    <header
      className="flex flex-wrap items-start justify-between gap-x-4 gap-y-3 px-6 pt-6 pb-0 mb-0"
      data-testid="page-header"
    >
      <div className="min-w-0 flex-1 basis-64">
        <div className="flex items-center gap-2">
          <h1 className="text-[20px] font-bold text-primary leading-tight break-words min-w-0">{title}</h1>
          {tier && <DemoTierHint />}
        </div>
        {description && <p className="mt-1 text-[12px] text-secondary">{description}</p>}
      </div>
      {actions && (
        <div
          className="flex flex-wrap items-center gap-2 max-w-full [&>div]:flex-wrap"
          data-testid="page-header-actions"
        >
          {actions}
        </div>
      )}
    </header>
  )
}
