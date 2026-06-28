import { useTranslation } from 'react-i18next'
import { Info } from 'lucide-react'
import { PageHeader } from '../../../shared/components/PageHeader'
import { TermTooltip } from '../../../shared/components/TermTooltip'
import { Spinner } from '../../../components/Spinner'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import { Badge } from '../../../components/ui/badge'
import { useFrameworks } from '../hooks/useFrameworks'
import { useDSGVOTOMCoverage } from '../hooks/useDSGVOMapping'
import type { MappingResult } from '../types'

function CoveredBadge({ covered }: { covered: boolean }) {
  if (covered) {
    return (
      <Badge variant="success" className="text-xs">
        Abgedeckt
      </Badge>
    )
  }
  return (
    <Badge variant="destructive" className="text-xs">
      Offen
    </Badge>
  )
}

function TOMRow({ result }: { result: MappingResult }) {
  return (
    <div
      className="flex flex-col sm:flex-row sm:items-center gap-2 p-4 bg-surface border border-border rounded-lg"
      data-testid={`tom-row-${result.tisax_control_id}`}
    >
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 flex-wrap">
          <span className="font-mono text-xs text-secondary">{result.tisax_control_id}</span>
          <span className="text-sm font-medium text-primary">{result.tisax_control_title}</span>
        </div>
        {result.iso_control_id && (
          <div className="mt-1 flex items-center gap-1.5 flex-wrap">
            <span className="font-mono text-xs text-secondary">{result.iso_control_id}</span>
            <span className="text-xs text-secondary">—</span>
            <span className="text-xs text-secondary">{result.iso_control_title}</span>
          </div>
        )}
      </div>
      <div className="shrink-0">
        <CoveredBadge covered={result.covered} />
      </div>
    </div>
  )
}

export default function DSGVOTOMPage() {
  const { t } = useTranslation()
  const { data: frameworks } = useFrameworks()
  const framework = frameworks?.find((f) => f.name === 'DSGVO-TOM')

  const { data: toms, isLoading } = useDSGVOTOMCoverage(framework?.id)

  const totalCount = (toms ?? []).length
  const coveredCount = (toms ?? []).filter((t) => t.covered).length
  const openCount = totalCount - coveredCount

  if (!framework) {
    return (
      <div className="flex flex-col h-full">
        <PageHeader
          title="DSGVO Art. 32 — TOM-Übersicht"
          description="Abdeckung der technischen und organisatorischen Maßnahmen basierend auf ISO 27001."
        />
        <div className="flex-1 p-6">
          <div className="flex items-start gap-3 p-4 rounded-lg bg-surface border border-border text-sm text-secondary">
            <Info className="w-5 h-5 text-secondary mt-0.5 shrink-0" />
            <span>
              DSGVO-TOM Framework ist noch nicht aktiviert. Aktivieren Sie es unter Frameworks.
            </span>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="DSGVO Art. 32 — TOM-Übersicht"
        description="Abdeckung der technischen und organisatorischen Maßnahmen basierend auf ISO 27001."
      />

      <div className="flex-1 p-6 space-y-6">
        {/* KPI Cards */}
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
          <Card data-testid="kpi-total">
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium text-secondary">{t('vaktcomply.dsgvoTom.total')} <TermTooltip term="TOMs" explanation="Technische und organisatorische Maßnahmen (Art. 32 DSGVO) — Sicherheitsvorkehrungen zum Schutz personenbezogener Daten: Verschlüsselung, Zugangskontrolle, Backups, Pseudonymisierung u.a.">TOMs</TermTooltip></CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-3xl font-bold text-primary">{totalCount}</div>
            </CardContent>
          </Card>

          <Card data-testid="kpi-covered">
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium text-secondary">{t('vaktcomply.dsgvoTom.covered')}</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-3xl font-bold text-brand">{coveredCount}</div>
            </CardContent>
          </Card>

          <Card data-testid="kpi-open">
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium text-secondary">{t('vaktcomply.dsgvoTom.open')}</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-3xl font-bold text-primary">{openCount}</div>
            </CardContent>
          </Card>
        </div>

        {/* TOM List */}
        {isLoading ? (
          <div className="flex items-center justify-center h-32">
            <Spinner size="md" />
          </div>
        ) : (
          <div className="flex flex-col gap-2" data-testid="tom-list">
            {(toms ?? []).map((tom) => (
              <TOMRow key={tom.tisax_control_id} result={tom} />
            ))}
          </div>
        )}

        {/* Note Card */}
        <div className="flex items-start gap-3 p-4 bg-surface border border-border rounded-lg text-sm text-secondary">
          <Info className="w-4 h-4 mt-0.5 shrink-0 text-secondary" />
          <span>
            Basis dieser Auswertung ist Ihr aktiviertes ISO 27001-Framework. Abgedeckt = ISO-Control
            implementiert oder mit Nachweis versehen.
          </span>
        </div>
      </div>
    </div>
  )
}
