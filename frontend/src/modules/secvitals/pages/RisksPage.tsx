import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { ShieldAlert, Plus, List, BarChart2, ChevronsUpDown, ChevronUp, ChevronDown, RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '../../../components/ui/button'
import { Card, CardContent } from '../../../components/ui/card'
import { Badge } from '../../../components/ui/badge'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../../../components/ui/dialog'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Textarea } from '../../../components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../../../components/ui/select'
import { PageHeader } from '../../../shared/components/PageHeader'
import { ExportButton } from '../../../shared/components/ExportButton'
import { EmptyState } from '../../../shared/components/EmptyState'
import { Pagination } from '../../../shared/components/Pagination'
import { BulkActionBar } from '../../../shared/components/BulkActionBar'
import { useSortableTable } from '../../../shared/hooks/useSortableTable'
import { toast } from '../../../shared/hooks/useToast'
import { useRisks, useCreateRisk } from '../hooks/useRisks'
import { useFirstAction } from '../../../shared/hooks/useFirstAction'
import { apiFetch } from '../../../api/client'
import RiskHeatmap from '../components/RiskHeatmap'
import { Skeleton } from '../../../components/ui/skeleton'
import { UserPicker } from '../../../shared/components/UserPicker'
import type { Risk, CreateRiskInput } from '../types'

// ---- Risk Matrix Heatmap (inline, list-view summary) -------------------------

function RiskMatrixHeatmap({ risks }: { risks: Risk[] }) {
  const counts: Record<string, number> = {}
  for (const r of risks) {
    if (r.status !== 'open') continue
    const key = `${r.likelihood}-${r.impact}`
    counts[key] = (counts[key] ?? 0) + 1
  }

  function cellColor(l: number, i: number): string {
    const score = l * i
    if (score >= 15) return '#fecaca'
    if (score >= 10) return '#fed7aa'
    if (score >= 5) return '#fef3c7'
    return '#d1fae5'
  }

  return (
    <div className="bg-white dark:bg-surface border rounded-lg p-4">
      <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-3">Risikomatrix (offene Risiken)</h3>
      <div className="flex gap-2">
        {/* Y-axis label */}
        <div
          className="flex flex-col justify-between text-xs text-gray-400 py-1"
          style={{ writingMode: 'vertical-rl', transform: 'rotate(180deg)', height: 5 * 44 + 'px' }}
        >
          Auswirkung →
        </div>
        <div>
          {/* Grid: impact 5 (top) to 1 (bottom), likelihood 1 (left) to 5 (right) */}
          {[5, 4, 3, 2, 1].map(impact => (
            <div key={impact} className="flex">
              <span className="text-xs text-gray-400 w-4 flex items-center justify-center">{impact}</span>
              {[1, 2, 3, 4, 5].map(likelihood => {
                const count = counts[`${likelihood}-${impact}`] ?? 0
                return (
                  <div
                    key={likelihood}
                    className="w-10 h-10 border border-white flex items-center justify-center text-xs font-semibold rounded-sm m-0.5"
                    style={{ backgroundColor: cellColor(likelihood, impact) }}
                    title={`Wahrscheinlichkeit ${likelihood} × Auswirkung ${impact} = ${likelihood * impact}`}
                  >
                    {count > 0 ? count : ''}
                  </div>
                )
              })}
            </div>
          ))}
          {/* X-axis */}
          <div className="flex mt-1 pl-4">
            {[1, 2, 3, 4, 5].map(l => (
              <span key={l} className="w-10 m-0.5 text-center text-xs text-gray-400">{l}</span>
            ))}
          </div>
          <div className="text-xs text-gray-400 text-center mt-1 pl-4">← Eintrittswahrscheinlichkeit</div>
        </div>
        {/* Legend */}
        <div className="ml-4 flex flex-col gap-1 justify-center text-xs">
          {[
            { color: '#fecaca', label: 'Kritisch (15–25)' },
            { color: '#fed7aa', label: 'Hoch (10–14)' },
            { color: '#fef3c7', label: 'Mittel (5–9)' },
            { color: '#d1fae5', label: 'Niedrig (1–4)' },
          ].map(({ color, label }) => (
            <div key={label} className="flex items-center gap-1">
              <div className="w-3 h-3 rounded-sm" style={{ backgroundColor: color }} />
              <span className="text-gray-600 dark:text-gray-400">{label}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

const SCORE_COLOR = (score: number) => {
  if (score >= 15) return 'bg-red-500/20 text-red-400 border-red-500/30'
  if (score >= 9) return 'bg-amber-500/20 text-amber-400 border-amber-500/30'
  if (score >= 4) return 'bg-yellow-500/20 text-yellow-400 border-yellow-500/30'
  return 'bg-green-500/20 text-green-400 border-green-500/30'
}

function RiskCard({
  risk,
  onClick,
  selected,
  onToggleSelect,
}: {
  risk: Risk
  onClick: () => void
  selected: boolean
  onToggleSelect: (id: string) => void
}) {
  const { t } = useTranslation()

  const STATUS_LABELS: Record<Risk['status'], string> = {
    open: t('secvitals.risksPage.statusOpen'),
    mitigated: t('secvitals.risksPage.statusMitigated'),
    accepted: t('secvitals.risksPage.statusAccepted'),
    closed: t('secvitals.risksPage.statusClosed'),
  }

  const TREATMENT_LABELS: Record<Risk['treatment'], string> = {
    avoid: t('secvitals.risksPage.treatmentAvoid'),
    mitigate: t('secvitals.risksPage.treatmentMitigate'),
    transfer: t('secvitals.risksPage.treatmentTransfer'),
    accept: t('secvitals.risksPage.treatmentAccept'),
  }

  return (
    <Card
      className={`cursor-pointer hover:border-brand/50 transition-colors ${selected ? 'border-brand/60 bg-brand/5' : ''}`}
      onClick={onClick}
    >
      <CardContent className="pt-5 space-y-3">
        <div className="flex items-start justify-between gap-2">
          <div className="flex items-start gap-2">
            <input
              type="checkbox"
              checked={selected}
              onChange={() => onToggleSelect(risk.id)}
              onClick={(e) => e.stopPropagation()}
              className="mt-0.5 rounded shrink-0"
              aria-label={`Risiko "${risk.title}" auswählen`}
            />
            <div>
              <p className="font-medium text-sm">{risk.title}</p>
              {risk.category && <p className="text-xs text-muted-foreground mt-0.5">{risk.category}</p>}
            </div>
          </div>
          <Badge className={SCORE_COLOR(risk.risk_score)}>Score {risk.risk_score}</Badge>
        </div>
        {risk.description && (
          <p className="text-xs text-muted-foreground line-clamp-2">{risk.description}</p>
        )}
        <div className="grid grid-cols-2 gap-2 text-xs text-muted-foreground">
          <span>{t('secvitals.risksPage.likelihood')}: <strong className="text-foreground">{risk.likelihood}/5</strong></span>
          <span>{t('secvitals.risksPage.impact')}: <strong className="text-foreground">{risk.impact}/5</strong></span>
        </div>
        <div className="flex items-center justify-between text-xs">
          <span className="text-muted-foreground">{TREATMENT_LABELS[risk.treatment]}</span>
          <Badge variant="secondary">{STATUS_LABELS[risk.status]}</Badge>
        </div>
      </CardContent>
    </Card>
  )
}

function emptyForm(): CreateRiskInput {
  return {
    title: '',
    description: '',
    category: '',
    likelihood: 3,
    impact: 3,
    owner: '',
    treatment: 'mitigate',
    treatment_notes: '',
  }
}

export default function RisksPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [form, setForm] = useState<CreateRiskInput>(emptyForm())
  const [view, setView] = useState<'list' | 'heatmap'>('list')
  const [page, setPage] = useState(1)
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [bulkStatusDialogOpen, setBulkStatusDialogOpen] = useState(false)
  const [pendingBulkStatus, setPendingBulkStatus] = useState<Risk['status']>('open')
  const [isApplyingBulk, setIsApplyingBulk] = useState(false)
  const { data: risks, isLoading, isError, pagination } = useRisks(page)
  const createRisk = useCreateRisk()
  useFirstAction('risk:first-created', (risks?.length ?? 0) > 0)

  function toggleSelect(id: string) {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  async function handleBulkStatusApply() {
    if (!risks) return
    setIsApplyingBulk(true)
    const selectedRisks = risks.filter((r) => selected.has(r.id))
    const results = await Promise.allSettled(
      selectedRisks.map((r) =>
        apiFetch<Risk>(`/secvitals/risks/${r.id}`, {
          method: 'PATCH',
          body: JSON.stringify({
            title: r.title,
            description: r.description ?? '',
            category: r.category ?? '',
            likelihood: r.likelihood,
            impact: r.impact,
            owner: r.owner ?? '',
            status: pendingBulkStatus,
            treatment: r.treatment,
            treatment_notes: r.treatment_notes ?? '',
          }),
        }),
      ),
    )
    const failed = results.filter((res) => res.status === 'rejected').length
    setIsApplyingBulk(false)
    setSelected(new Set())
    setBulkStatusDialogOpen(false)
    if (failed === 0) {
      toast('Status aktualisiert', 'success')
    } else {
      toast(`${failed} Risiken konnten nicht aktualisiert werden`, 'error')
    }
  }

  const { sorted: sortedRisks, sortKey, sortDir, toggleSort } = useSortableTable<Risk>(
    risks ?? [], { key: 'risk_score', dir: 'desc' },
  )

  const SORT_OPTIONS: { key: keyof Risk; label: string }[] = [
    { key: 'title', label: t('common.name') },
    { key: 'likelihood', label: t('secvitals.risksPage.likelihood') },
    { key: 'impact', label: t('secvitals.risksPage.impact') },
    { key: 'risk_score', label: 'Risiko-Score' },
    { key: 'status', label: t('common.status') },
  ]

  function openDialog() {
    setForm(emptyForm())
    setDialogOpen(true)
  }

  function handleSubmit() {
    createRisk.mutate(form, { onSuccess: () => setDialogOpen(false) })
  }

  const displayRisks = view === 'list' ? sortedRisks : (risks ?? [])
  const high = displayRisks.filter((r) => r.risk_score >= 15)
  const medium = displayRisks.filter((r) => r.risk_score >= 9 && r.risk_score < 15)
  const low = displayRisks.filter((r) => r.risk_score < 9)

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title={t('secvitals.risksPage.title')}
        description={t('secvitals.risksPage.description')}
        actions={
          <div className="flex items-center gap-2">
            {/* View toggle */}
            <div className="flex items-center rounded-md border bg-muted/30 p-0.5 gap-0.5">
              <Button
                size="sm"
                variant={view === 'list' ? 'secondary' : 'ghost'}
                className="h-7 px-2.5 text-xs"
                onClick={() => setView('list')}
              >
                <List className="w-3.5 h-3.5 mr-1" />
                {t('secvitals.risksPage.viewList')}
              </Button>
              <Button
                size="sm"
                variant={view === 'heatmap' ? 'secondary' : 'ghost'}
                className="h-7 px-2.5 text-xs"
                onClick={() => setView('heatmap')}
              >
                <BarChart2 className="w-3.5 h-3.5 mr-1" />
                {t('secvitals.risksPage.viewHeatmap')}
              </Button>
            </div>
            <ExportButton
              endpoint="/api/v1/secvitals/risks/export/xlsx"
              filename={`risiken-${new Date().toISOString().slice(0, 10)}`}
              label="Exportieren"
              format="xlsx"
            />
            <Button onClick={openDialog}>
              <Plus className="w-4 h-4 mr-1" />
              {t('secvitals.risksPage.addRisk')}
            </Button>
          </div>
        }
      />

      <div className="flex-1 p-6 space-y-6">
        {/* Sort toolbar — list view only */}
        {!isLoading && !isError && view === 'list' && risks && risks.length > 0 && (
          <div className="flex flex-wrap items-center gap-2 text-xs text-secondary">
            <span className="font-medium">{t('common.filter')}:</span>
            {SORT_OPTIONS.map((opt) => {
              const isActive = sortKey === opt.key
              return (
                <button
                  key={String(opt.key)}
                  onClick={() => toggleSort(opt.key)}
                  className={`inline-flex items-center gap-1 px-2.5 py-1 rounded-md border transition-colors ${
                    isActive
                      ? 'border-brand/50 bg-brand/10 text-brand'
                      : 'border-border hover:border-brand/30 hover:bg-surface2'
                  }`}
                >
                  {opt.label}
                  {isActive ? (
                    sortDir === 'asc'
                      ? <ChevronUp className="w-3 h-3" />
                      : <ChevronDown className="w-3 h-3" />
                  ) : (
                    <ChevronsUpDown className="w-3 h-3 opacity-50" />
                  )}
                </button>
              )
            })}
          </div>
        )}

        {isLoading && (
          <div className="space-y-2">
            {Array.from({ length: 5 }).map((_, i) => (
              <Skeleton key={i} className="h-12 w-full rounded-lg" />
            ))}
          </div>
        )}
        {isError && (
          <div className="text-sm text-red-400 p-4 bg-red-500/10 rounded-lg">{t('secvitals.risksPage.loadError')}</div>
        )}
        {!isLoading && !isError && risks?.length === 0 && (
          <EmptyState
            icon={ShieldAlert}
            title="Noch keine Risiken erfasst"
            description="Erfasse dein erstes Risiko und verknüpfe es mit Controls — das ist der Kern deines ISMS"
            action={<Button onClick={openDialog}><Plus className="w-4 h-4 mr-1" />Risiko erfassen</Button>}
          />
        )}
        {!isLoading && !isError && risks && risks.length > 0 && view === 'heatmap' && (
          <RiskHeatmap risks={risks} />
        )}
        {!isLoading && !isError && risks && risks.length > 0 && view === 'list' && (
          <>
            <RiskMatrixHeatmap risks={risks} />
            {high.length > 0 && (
              <div className="space-y-3">
                <h2 className="text-sm font-semibold text-red-400">{t('secvitals.risksPage.highRisk')}</h2>
                <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
                  {high.map((r) => (
                    <RiskCard
                      key={r.id}
                      risk={r}
                      onClick={() => navigate(`/secvitals/risks/${r.id}`)}
                      selected={selected.has(r.id)}
                      onToggleSelect={toggleSelect}
                    />
                  ))}
                </div>
              </div>
            )}
            {medium.length > 0 && (
              <div className="space-y-3">
                <h2 className="text-sm font-semibold text-amber-400">{t('secvitals.risksPage.mediumRisk')}</h2>
                <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
                  {medium.map((r) => (
                    <RiskCard
                      key={r.id}
                      risk={r}
                      onClick={() => navigate(`/secvitals/risks/${r.id}`)}
                      selected={selected.has(r.id)}
                      onToggleSelect={toggleSelect}
                    />
                  ))}
                </div>
              </div>
            )}
            {low.length > 0 && (
              <div className="space-y-3">
                <h2 className="text-sm font-semibold text-muted-foreground">{t('secvitals.risksPage.lowRisk')}</h2>
                <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
                  {low.map((r) => (
                    <RiskCard
                      key={r.id}
                      risk={r}
                      onClick={() => navigate(`/secvitals/risks/${r.id}`)}
                      selected={selected.has(r.id)}
                      onToggleSelect={toggleSelect}
                    />
                  ))}
                </div>
              </div>
            )}
          </>
        )}
        <Pagination
          page={page}
          totalPages={pagination?.total_pages ?? 1}
          onPageChange={setPage}
        />
      </div>

      {/* Bulk status dialog */}
      <Dialog open={bulkStatusDialogOpen} onOpenChange={setBulkStatusDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Status setzen</DialogTitle>
          </DialogHeader>
          <div className="py-3 space-y-3">
            <p className="text-sm text-secondary">
              Neuen Status für {selected.size} ausgewählte{selected.size === 1 ? 's Risiko' : ' Risiken'} setzen:
            </p>
            <Select value={pendingBulkStatus} onValueChange={(v) => setPendingBulkStatus(v as Risk['status'])}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="open">{t('secvitals.risksPage.statusOpen')}</SelectItem>
                <SelectItem value="mitigated">{t('secvitals.risksPage.statusMitigated')}</SelectItem>
                <SelectItem value="accepted">{t('secvitals.risksPage.statusAccepted')}</SelectItem>
                <SelectItem value="closed">{t('secvitals.risksPage.statusClosed')}</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setBulkStatusDialogOpen(false)}>Abbrechen</Button>
            <Button onClick={() => { void handleBulkStatusApply() }} disabled={isApplyingBulk}>
              {isApplyingBulk ? 'Wird gespeichert…' : 'Anwenden'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <BulkActionBar
        selectedCount={selected.size}
        onClearSelection={() => setSelected(new Set())}
        actions={[
          {
            label: 'Status setzen',
            icon: RefreshCw,
            onClick: () => setBulkStatusDialogOpen(true),
          },
        ]}
      />

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="max-w-lg max-h-[90vh] overflow-y-auto">
          <DialogHeader><DialogTitle>{t('secvitals.risksPage.dialogTitle')}</DialogTitle></DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-1.5">
              <Label htmlFor="risk-title">{t('secvitals.risksPage.labelTitle')} *</Label>
              <Input id="risk-title" placeholder={t('secvitals.risksPage.placeholderTitle')} value={form.title}
                onChange={(e) => setForm((f) => ({ ...f, title: e.target.value }))} />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="risk-category">{t('secvitals.risksPage.labelCategory')}</Label>
              <Input id="risk-category" placeholder={t('secvitals.risksPage.placeholderCategory')} value={form.category ?? ''}
                onChange={(e) => setForm((f) => ({ ...f, category: e.target.value }))} />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="risk-desc">{t('secvitals.risksPage.labelDescription')}</Label>
              <Textarea id="risk-desc" rows={2} placeholder={t('secvitals.risksPage.placeholderDescription')} value={form.description ?? ''}
                onChange={(e) => setForm((f) => ({ ...f, description: e.target.value }))} />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label htmlFor="risk-likelihood">{t('secvitals.risksPage.labelLikelihood')} *</Label>
                <Input id="risk-likelihood" type="number" min={1} max={5} value={form.likelihood}
                  onChange={(e) => setForm((f) => ({ ...f, likelihood: parseInt(e.target.value, 10) || 1 }))} />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="risk-impact">{t('secvitals.risksPage.labelImpact')} *</Label>
                <Input id="risk-impact" type="number" min={1} max={5} value={form.impact}
                  onChange={(e) => setForm((f) => ({ ...f, impact: parseInt(e.target.value, 10) || 1 }))} />
              </div>
            </div>
            <div className="text-xs text-muted-foreground">
              {t('secvitals.risksPage.previewScore')}: <strong className={`${SCORE_COLOR(form.likelihood * form.impact)} px-1 py-0.5 rounded`}>{form.likelihood * form.impact}</strong>
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="risk-treatment">{t('secvitals.risksPage.labelTreatment')} *</Label>
              <Select value={form.treatment} onValueChange={(v) => setForm((f) => ({ ...f, treatment: v as Risk['treatment'] }))}>
                <SelectTrigger id="risk-treatment"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="avoid">{t('secvitals.risksPage.treatmentAvoid')}</SelectItem>
                  <SelectItem value="mitigate">{t('secvitals.risksPage.treatmentMitigate')}</SelectItem>
                  <SelectItem value="transfer">{t('secvitals.risksPage.treatmentTransfer')}</SelectItem>
                  <SelectItem value="accept">{t('secvitals.risksPage.treatmentAccept')}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="risk-treatment-notes">{t('secvitals.risksPage.labelTreatmentNotes')}</Label>
              <Textarea id="risk-treatment-notes" rows={2} placeholder={t('secvitals.risksPage.placeholderTreatmentNotes')} value={form.treatment_notes ?? ''}
                onChange={(e) => setForm((f) => ({ ...f, treatment_notes: e.target.value }))} />
            </div>
            <div className="space-y-1.5">
              <Label>{t('secvitals.risksPage.labelOwner')}</Label>
              <UserPicker
                value={form.owner ?? undefined}
                onChange={(name) => setForm((f) => ({ ...f, owner: name ?? '' }))}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDialogOpen(false)}>{t('common.cancel')}</Button>
            <Button onClick={handleSubmit} disabled={!form.title || !form.treatment || createRisk.isPending}>
              {createRisk.isPending ? t('common.saving') : t('secvitals.risksPage.addRisk')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
