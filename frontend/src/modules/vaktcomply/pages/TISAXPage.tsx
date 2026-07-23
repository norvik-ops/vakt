import { useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { ArrowLeft, ChevronDown, Download } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { useQuery } from '@tanstack/react-query'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '../../../components/ui/tabs'
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from '../../../components/ui/table'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '../../../components/ui/select'
import { PageHeader } from '../../../shared/components/PageHeader'
import { cn } from '../../../lib/utils'
import { apiFetch } from '../../../api/client'
import { ProGate } from '../../../shared/components/ProGate'
import type { Control } from '../types'
import { maturityLabel, maturityColor } from '../utils/tisax'
import { downloadBlob } from '../../../shared/utils/downloadBlob'

// ── Protection level types ────────────────────────────────────────────────────

type ProtectionLevel = 'normal' | 'high' | 'very_high'
type AssessmentLevel = 'AL1' | 'AL2' | 'AL3'

// ── Domain section ────────────────────────────────────────────────────────────

function DomainSection({
  domain,
  controls,
  frameworkId,
}: {
  domain: string
  controls: Control[]
  frameworkId: string
}) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(true)
  const navigate = useNavigate()

  return (
    <div className="border border-border rounded-lg overflow-x-auto">
      <button
        type="button"
        className="w-full flex items-center justify-between px-4 py-2.5 bg-surface2 hover:bg-surface text-left"
        onClick={() => { setOpen((v) => !v); }}
      >
        <div className="flex items-center gap-3">
          <ChevronDown className={cn('w-4 h-4 text-secondary transition-transform', !open && '-rotate-90')} />
          <span className="text-sm font-medium">{domain}</span>
          <span className="text-xs text-secondary">{t('vaktcomply.tisax.measures', { count: controls.length })}</span>
        </div>
      </button>

      {open && (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-32">{t('vaktcomply.tisax.colId')}</TableHead>
              <TableHead>{t('vaktcomply.tisax.colMeasure')}</TableHead>
              <TableHead className="w-40">{t('vaktcomply.tisax.colMaturity')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {controls.map((ctrl) => (
              <TableRow
                key={ctrl.id}
                className="cursor-pointer hover:bg-surface2"
                onClick={() =>
                  { navigate(`/vaktcomply/controls/${ctrl.id}?frameworkId=${frameworkId}`); }
                }
              >
                <TableCell className="font-mono text-xs">{ctrl.control_id}</TableCell>
                <TableCell className="font-medium">{ctrl.title}</TableCell>
                <TableCell>
                  <span className={cn('text-xs font-medium', maturityColor(ctrl.maturity_score ?? 0))}>
                    {ctrl.maturity_score ?? 0} – {maturityLabel(ctrl.maturity_score ?? 0)}
                  </span>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </div>
  )
}

// ── Page ──────────────────────────────────────────────────────────────────────

export default function TISAXPage() {
  const { t } = useTranslation()
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const frameworkId = id ?? ''

  const [protectionLevel, setProtectionLevel] = useState<ProtectionLevel>('normal')
  const [assessmentLevel, setAssessmentLevel] = useState<AssessmentLevel>('AL2')

  const PROTECTION_LEVEL_LABELS: Record<ProtectionLevel, string> = {
    normal: t('vaktcomply.tisax.protectionNormal'),
    high: t('vaktcomply.tisax.protectionHigh'),
    very_high: t('vaktcomply.tisax.protectionVeryHigh'),
  }

  const ASSESSMENT_LEVEL_LABELS: Record<AssessmentLevel, string> = {
    AL1: t('vaktcomply.tisax.assessmentAL1'),
    AL2: t('vaktcomply.tisax.assessmentAL2'),
    AL3: t('vaktcomply.tisax.assessmentAL3'),
  }

  function handleExportReport() {
    const url = `/api/v1/vaktcomply/frameworks/${frameworkId}/tisax-report-pdf?protection_level=${protectionLevel}&assessment_level=${assessmentLevel}`
    // S131-D1: downloadBlob checks res.ok before saving (no error-body-as-file).
    void downloadBlob(url, `tisax-bereitschaftsbericht-${new Date().toISOString().slice(0, 10)}.pdf`)
  }

  const { data: controls, isLoading, isError, error } = useQuery<Control[]>({
    queryKey: ['vaktcomply', 'tisax-controls', frameworkId, protectionLevel],
    queryFn: () =>
      apiFetch<Control[]>(
        `/vaktcomply/frameworks/${frameworkId}/tisax-controls?protection_level=${protectionLevel}`,
      ),
    enabled: !!frameworkId,
    staleTime: 5 * 60 * 1000,
  })

  // Group controls by domain
  const byDomain = new Map<string, Control[]>()
  for (const ctrl of controls ?? []) {
    const list = byDomain.get(ctrl.domain) ?? []
    list.push(ctrl)
    byDomain.set(ctrl.domain, list)
  }

  const maturityBadgeVariant = (score: number): React.ComponentProps<typeof Badge>['variant'] => {
    if (score === 3) return 'success'
    if (score === 2) return 'warning'
    if (score === 1) return 'secondary'
    return 'destructive'
  }

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title={t('vaktcomply.tisax.title')}
        description={t('vaktcomply.tisax.description')}
        actions={
          <div className="flex items-center gap-2">
            <Select value={assessmentLevel} onValueChange={(v) => { setAssessmentLevel(v as AssessmentLevel); }}>
              <SelectTrigger className="h-8 w-44 text-xs" aria-label="Assessment Level">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {(Object.entries(ASSESSMENT_LEVEL_LABELS) as [AssessmentLevel, string][]).map(([level, label]) => (
                  <SelectItem key={level} value={level}>{label}</SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Button variant="secondary" size="sm" onClick={handleExportReport}>
              <Download className="w-4 h-4 mr-1" />
              {t('vaktcomply.tisax.exportBtn')}
            </Button>
            <Button variant="secondary" size="sm" onClick={() => { navigate('/vaktcomply/tisax-mapping'); }}>
              {t('vaktcomply.tisax.isoMapping')}
            </Button>
            <Button variant="outline" size="sm" onClick={() => { navigate(`/vaktcomply/frameworks/${frameworkId}`); }}>
              <ArrowLeft className="w-4 h-4 mr-1" />
              {t('vaktcomply.tisax.backToFramework')}
            </Button>
          </div>
        }
      />

      <div className="flex-1 p-6 space-y-6">
        {/* Protection level tabs */}
        <Tabs
          value={protectionLevel}
          onValueChange={(v) => { setProtectionLevel(v as ProtectionLevel); }}
        >
          <TabsList>
            {(Object.entries(PROTECTION_LEVEL_LABELS) as [ProtectionLevel, string][]).map(
              ([level, label]) => (
                <TabsTrigger key={level} value={level}>
                  {label}
                </TabsTrigger>
              ),
            )}
          </TabsList>

          {(Object.keys(PROTECTION_LEVEL_LABELS) as ProtectionLevel[]).map((level) => (
            <TabsContent key={level} value={level}>
              {isLoading ? (
                <div className="flex items-center justify-center h-32">
                  <Spinner size="md" />
                </div>
              ) : isError ? (
                <ProGate error={error}><div className="text-sm text-red-400 p-4 bg-red-500/10 rounded-lg">{t('vaktcomply.tisax.loadError')}</div></ProGate>
              ) : !controls || controls.length === 0 ? (
                <p className="text-sm text-secondary py-8 text-center">
                  {t('vaktcomply.tisax.noControls')}
                </p>
              ) : (
                <div className="space-y-4 mt-4">
                  {/* Summary bar */}
                  <div className="flex gap-4 flex-wrap">
                    {([3, 2, 1, 0] as const).map((score) => {
                      const count = controls.filter((c) => (c.maturity_score ?? 0) === score).length
                      return (
                        <div
                          key={score}
                          className="flex items-center gap-2 px-3 py-1.5 bg-surface border border-border rounded-md"
                        >
                          <Badge variant={maturityBadgeVariant(score)} className="text-xs">
                            {score}
                          </Badge>
                          <span className="text-xs text-secondary">
                            {maturityLabel(score)}: <span className="font-medium text-primary">{count}</span>
                          </span>
                        </div>
                      )
                    })}
                  </div>

                  {/* Controls grouped by domain */}
                  {Array.from(byDomain.entries()).map(([domain, domainControls]) => (
                    <DomainSection
                      key={domain}
                      domain={domain}
                      controls={domainControls}
                      frameworkId={frameworkId}
                    />
                  ))}
                </div>
              )}
            </TabsContent>
          ))}
        </Tabs>
      </div>
    </div>
  )
}
