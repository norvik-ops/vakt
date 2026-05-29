import { useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, ChevronDown } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { PageHeader } from '../../../shared/components/PageHeader'
import { ProGate } from '../../../shared/components/ProGate'
import { cn } from '../../../lib/utils'
import { useFrameworkControls } from '../hooks/useFrameworks'
import type { Control } from '../types'

// ── Domain → Article group mapping ───────────────────────────────────────────

const DOMAIN_TO_ARTICLE: Record<string, string> = {
  'ICT-Risikomanagement': 'Art. 5–16',
  'Vorfallmanagement': 'Art. 17–23',
  'Resilienztests': 'Art. 24–27',
  'Drittparteienrisiken': 'Art. 28–44',
}

const ARTICLE_ORDER = ['Art. 5–16', 'Art. 17–23', 'Art. 24–27', 'Art. 28–44']

const ARTICLE_DESCRIPTIONS: Record<string, string> = {
  'Art. 5–16': 'ICT-Risikomanagement — Anforderungen an Risikomanagement-Framework, Asset-Inventar, Schutzmaßnahmen, BCM und Patch-Management.',
  'Art. 17–23': 'ICT-bezogenes Vorfallmanagement — Klassifizierung, Meldepflichten gegenüber BaFin/EBA sowie Incident-Response-Prozesse.',
  'Art. 24–27': 'Digital Operational Resilience Testing — Jährliche Resilienz-Tests, TLPT und szenarienbasierte Übungen.',
  'Art. 28–44': 'IKT-Drittparteienrisiken — Management-Framework, Vertragsanforderungen und Ausstiegsstrategien für kritische IKT-Anbieter.',
}

/**
 * groupDoraControlsByArticle groups DORA controls by their DORA article section
 * based on the domain name.
 */
export function groupDoraControlsByArticle(controls: Control[]): Record<string, Control[]> {
  const result: Record<string, Control[]> = {}

  for (const ctrl of controls) {
    const article = DOMAIN_TO_ARTICLE[ctrl.domain]
    if (article) {
      if (!result[article]) result[article] = []
      result[article].push(ctrl)
    }
  }

  return result
}

// ── Status badge helper ───────────────────────────────────────────────────────

function statusVariant(status: Control['status']): React.ComponentProps<typeof Badge>['variant'] {
  if (status === 'covered' || status === 'implemented') return 'success'
  if (status === 'partial' || status === 'in_progress') return 'warning'
  if (status === 'not_applicable') return 'secondary'
  return 'destructive'
}

function statusLabel(status: Control['status']): string {
  if (status === 'covered' || status === 'implemented') return 'Umgesetzt'
  if (status === 'partial' || status === 'in_progress') return 'In Bearbeitung'
  if (status === 'not_applicable') return 'N/A'
  return 'Offen'
}

// ── Article Section ───────────────────────────────────────────────────────────

function ArticleSection({
  article,
  controls,
  frameworkId,
}: {
  article: string
  controls: Control[]
  frameworkId: string
}) {
  const [open, setOpen] = useState(true)
  const navigate = useNavigate()
  const done = controls.filter((c) => c.status === 'covered' || c.status === 'implemented').length

  return (
    <div className="border border-border rounded-lg overflow-hidden">
      <button
        type="button"
        className="w-full flex items-center justify-between px-4 py-3 bg-surface2 hover:bg-surface text-left"
        onClick={() => { setOpen((v) => !v); }}
      >
        <div className="flex items-center gap-3">
          <ChevronDown className={cn('w-4 h-4 text-secondary transition-transform', !open && '-rotate-90')} />
          <span className="font-semibold text-sm">{article}</span>
          <span className="text-xs text-secondary hidden sm:inline">
            {ARTICLE_DESCRIPTIONS[article]?.split('—')[0].trim()}
          </span>
        </div>
        <span className="text-xs text-secondary shrink-0">
          {done}/{controls.length} umgesetzt
        </span>
      </button>

      {open && (
        <div className="divide-y divide-border">
          {controls.map((ctrl) => (
            <div
              key={ctrl.id}
              className="flex items-center justify-between px-4 py-3 hover:bg-surface2 cursor-pointer"
              onClick={() =>
                { navigate(`/vaktcomply/controls/${ctrl.id}?frameworkId=${frameworkId}`); }
              }
            >
              <div className="flex items-center gap-3 min-w-0">
                <span className="font-mono text-xs text-secondary shrink-0">{ctrl.control_id}</span>
                <span className="text-sm truncate">{ctrl.title}</span>
              </div>
              <Badge variant={statusVariant(ctrl.status)} className="shrink-0 ml-2">
                {statusLabel(ctrl.status)}
              </Badge>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

// ── Page ──────────────────────────────────────────────────────────────────────

export default function DORAPage() {
  const { frameworkId = '' } = useParams<{ frameworkId: string }>()
  const navigate = useNavigate()
  const { data: controls, isLoading, isError, error } = useFrameworkControls(frameworkId)

  const grouped = controls ? groupDoraControlsByArticle(controls) : {}

  return (
    <ProGate error={isError ? error : null}>
      <div className="flex flex-col h-full">
        <PageHeader
          title="DORA — Artikel-Übersicht"
          description="Controls gegliedert nach den DORA-Artikelgruppen (Art. 5–16, 17–23, 24–27, 28–44)."
          actions={
            <Button
              variant="outline"
              size="sm"
              onClick={() => { navigate(`/vaktcomply/frameworks/${frameworkId}`); }}
            >
              <ArrowLeft className="w-4 h-4 mr-1" />
              Zurück
            </Button>
          }
        />

        <div className="flex-1 p-6 space-y-4">
          {isLoading ? (
            <div className="flex items-center justify-center h-32">
              <Spinner size="md" />
            </div>
          ) : (
            ARTICLE_ORDER.map((article) => {
              const articleControls = grouped[article] ?? []
              return (
                <ArticleSection
                  key={article}
                  article={article}
                  controls={articleControls}
                  frameworkId={frameworkId}
                />
              )
            })
          )}
        </div>
      </div>
    </ProGate>
  )
}
