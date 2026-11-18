import React, { useState } from 'react'
import { useCanWrite } from '../../../shared/hooks/useCanWrite'
import { useTranslation } from 'react-i18next'
import { ShieldCheck, ShieldOff, ArrowRightLeft, CheckCircle2, Link2, Unlink, Plus, Save } from 'lucide-react'
import { Button } from '../../../components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Textarea } from '../../../components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../../../components/ui/select'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../../../components/ui/dialog'
import {
  useRiskControls,
  useLinkRiskControl,
  useUnlinkRiskControl,
} from '../hooks/useRisks'
import { useFrameworks, useFrameworkControls } from '../hooks/useFrameworks'
import type { TreatmentOption, TreatmentStatus, Control } from '../types'
import { residualScoreOf, type RiskEditForm } from '../utils/riskEditForm'
import { riskLevel, type RiskLevel } from '../../../shared/utils/riskLevel'
import { frameworkDisplayName } from '../utils/frameworkCatalogue'

// ---------------------------------------------------------------------------
// Treatment option button configuration
// ---------------------------------------------------------------------------

interface TreatmentOptionConfig {
  value: TreatmentOption
  label: string
  description: string
  icon: React.ReactNode
  color: string
}

const TREATMENT_OPTIONS: TreatmentOptionConfig[] = [
  {
    value: 'mitigate',
    label: 'Mindern',
    description: 'Maßnahmen zur Reduzierung des Risikos',
    icon: <ShieldCheck className="w-4 h-4" />,
    color: 'border-blue-500/50 bg-blue-500/10 text-blue-400',
  },
  {
    value: 'accept',
    label: 'Akzeptieren',
    description: 'Risiko bewusst in Kauf nehmen',
    icon: <CheckCircle2 className="w-4 h-4" />,
    color: 'border-green-500/50 bg-green-500/10 text-green-400',
  },
  {
    value: 'transfer',
    label: 'Übertragen',
    description: 'z.B. Versicherung oder Auslagerung',
    icon: <ArrowRightLeft className="w-4 h-4" />,
    color: 'border-amber-500/50 bg-amber-500/10 text-amber-400',
  },
  {
    value: 'avoid',
    label: 'Vermeiden',
    description: 'Risikoquelle beseitigen',
    icon: <ShieldOff className="w-4 h-4" />,
    color: 'border-red-500/50 bg-red-500/10 text-red-400',
  },
]

const TREATMENT_STATUS_LABELS: Record<TreatmentStatus, string> = {
  pending: 'Ausstehend',
  in_progress: 'In Bearbeitung',
  implemented: 'Umgesetzt',
  verified: 'Verifiziert',
}

// Residual score color preview
// Same levels as gross risk (R1-KW-17).
const RESIDUAL_COLOR: Record<RiskLevel, string> = {
  critical: 'bg-red-500',
  high: 'bg-orange-500',
  medium: 'bg-yellow-500',
  low: 'bg-green-500',
}

function residualColor(score: number): string {
  return RESIDUAL_COLOR[riskLevel(score)]
}

// ---------------------------------------------------------------------------
// Link Controls Dialog (reused from RiskDetailPage pattern)
// ---------------------------------------------------------------------------

function LinkControlDialog({ riskId, open, onClose }: { riskId: string; open: boolean; onClose: () => void }) {
  const { t } = useTranslation()
  const [selectedFramework, setSelectedFramework] = useState('')
  const [search, setSearch] = useState('')
  const [linkError, setLinkError] = useState<string | null>(null)
  const { data: frameworks } = useFrameworks()
  const { data: controls } = useFrameworkControls(selectedFramework)
  const link = useLinkRiskControl(riskId)

  const filtered = (controls ?? []).filter(
    (c: Control) =>
      c.title.toLowerCase().includes(search.toLowerCase()) ||
      c.control_id.toLowerCase().includes(search.toLowerCase()),
  )

  function handleClose() {
    setLinkError(null)
    onClose()
  }

  function handleLink(control: Control) {
    setLinkError(null)
    link.mutate(control.id, {
      onSuccess: handleClose,
      onError: (err: unknown) => {
        const msg = err instanceof Error ? err.message : String(err)
        if (/duplicate|already/i.test(msg)) {
          setLinkError('Control bereits verknüpft')
        } else {
          setLinkError(msg)
        }
      },
    })
  }

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) { handleClose(); } }}>
      <DialogContent className="max-w-lg max-h-[80vh] flex flex-col">
        <DialogHeader>
          <DialogTitle>{t('vaktcomply.riskTreatmentPanel.linkControlTitle')}</DialogTitle>
        </DialogHeader>
        <div className="space-y-3 flex-1 overflow-hidden flex flex-col">
          <div className="space-y-1.5">
            <Label>{t('vaktcomply.riskTreatmentPanel.framework')}</Label>
            <select
              className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
              value={selectedFramework}
              onChange={(e) => { setSelectedFramework(e.target.value); setSearch(''); setLinkError(null) }}
            >
              <option value="">— Framework wählen —</option>
              {(frameworks ?? []).map((f) => (
                <option key={f.id} value={f.id}>{frameworkDisplayName(f.name)}</option>
              ))}
            </select>
          </div>
          {selectedFramework && (
            <>
              <Input
                placeholder="Controls durchsuchen …"
                value={search}
                onChange={(e) => { setSearch(e.target.value); }}
              />
              <div className="overflow-y-auto flex-1 space-y-1 pr-1">
                {filtered.length === 0 && (
                  <p className="text-sm text-muted-foreground text-center py-4">Keine Controls gefunden.</p>
                )}
                {filtered.map((c: Control) => (
                  <button
                    key={c.id}
                    className="w-full text-left px-3 py-2 rounded-md hover:bg-accent text-sm flex items-start gap-2 group"
                    onClick={() => { handleLink(c); }}
                    disabled={link.isPending}
                  >
                    <span className="font-mono text-xs text-muted-foreground shrink-0 mt-0.5">{c.control_id}</span>
                    <span className="flex-1 line-clamp-2">{c.title}</span>
                    <Link2 className="w-3.5 h-3.5 shrink-0 opacity-0 group-hover:opacity-100 text-primary mt-0.5" />
                  </button>
                ))}
              </div>
            </>
          )}
        </div>
        <DialogFooter className="flex-col items-stretch gap-2">
          {linkError && (
            <p className="text-sm text-destructive text-center">{linkError}</p>
          )}
          <Button variant="outline" onClick={handleClose}>{t('common.close')}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ---------------------------------------------------------------------------
// Main RiskTreatmentPanel
// ---------------------------------------------------------------------------

/** The treatment part of the risk edit form; owner, title etc. live in the page. */
export type TreatmentFields = Pick<
  RiskEditForm,
  'strategy' | 'plan' | 'dueDate' | 'treatmentStatus' | 'residualLikelihood' | 'residualImpact'
>

interface Props {
  riskId: string
  /** Gross risk score (likelihood × impact), shown next to the residual score. */
  grossScore: number
  value: TreatmentFields
  onChange: (patch: Partial<TreatmentFields>) => void
  onSave: () => void
  saving: boolean
  dirty: boolean
}

/**
 * Treatment section of the risk detail (ISO 27001 clause 6.1.3). Controlled by the
 * page: one form, one save (UX-26). The owner is asked once, in the page's basic data.
 */
const RiskTreatmentPanel: React.FC<Props> = ({ riskId, grossScore, value: form, onChange: set, onSave, saving, dirty }) => {
  const { t } = useTranslation()
  const { data: linkedControls } = useRiskControls(riskId)
  const unlink = useUnlinkRiskControl(riskId)
  const [linkDialogOpen, setLinkDialogOpen] = useState(false)
  const canWrite = useCanWrite()

  const residualScore = residualScoreOf(form.residualLikelihood, form.residualImpact)

  return (
    <div className="space-y-4">
      {/* Treatment Option selector */}
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-sm">{t('vaktcomply.riskTreatmentPanel.strategy')}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-2" role="radiogroup" aria-label={t('vaktcomply.riskTreatmentPanel.strategy')}>
            {TREATMENT_OPTIONS.map((opt) => (
              <button
                key={opt.value}
                type="button"
                role="radio"
                aria-checked={form.strategy === opt.value}
                onClick={() => { set({ strategy: opt.value }); }}
                className={`flex items-start gap-2 rounded-lg border p-3 text-left transition-colors text-sm ${
                  form.strategy === opt.value
                    ? opt.color + ' ring-1 ring-current'
                    : 'border-border hover:border-muted-foreground/40'
                }`}
              >
                <span className="mt-0.5 shrink-0">{opt.icon}</span>
                <span>
                  <span className="font-medium block">{opt.label}</span>
                  <span className="text-xs text-muted-foreground">{opt.description}</span>
                </span>
              </button>
            ))}
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="risk-treatment-plan">{t('vaktcomply.riskTreatmentPanel.treatmentPlan')}</Label>
            <Textarea
              id="risk-treatment-plan"
              rows={4}
              placeholder="Konkrete Maßnahmen, Verantwortlichkeiten und Zeitplan …"
              value={form.plan}
              onChange={(e) => { set({ plan: e.target.value }); }}
            />
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label htmlFor="risk-treatment-due">{t('vaktcomply.riskTreatmentPanel.dueDate')}</Label>
              <Input
                id="risk-treatment-due"
                type="date"
                value={form.dueDate}
                onChange={(e) => { set({ dueDate: e.target.value }); }}
              />
            </div>
            <div className="space-y-1.5">
              <Label>{t('vaktcomply.riskTreatmentPanel.treatmentStatus')}</Label>
              <Select
                value={form.treatmentStatus}
                onValueChange={(v) => { set({ treatmentStatus: v as TreatmentStatus }); }}
              >
                <SelectTrigger aria-label={t('vaktcomply.riskTreatmentPanel.treatmentStatus')}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {(Object.keys(TREATMENT_STATUS_LABELS) as TreatmentStatus[]).map((k) => (
                    <SelectItem key={k} value={k}>{TREATMENT_STATUS_LABELS[k]}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Residual Risk */}
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-sm">Restrisiko nach Behandlung</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label>
                Restrisiko-Wahrscheinlichkeit
                {form.residualLikelihood != null && (
                  <span className="ml-1 font-mono text-muted-foreground">{form.residualLikelihood}/5</span>
                )}
              </Label>
              <input
                type="range"
                min={1}
                max={5}
                step={1}
                className="w-full accent-primary"
                aria-label="Restrisiko-Wahrscheinlichkeit"
                value={form.residualLikelihood ?? 3}
                onChange={(e) => { set({ residualLikelihood: parseInt(e.target.value, 10) }); }}
              />
              <div className="flex justify-between text-[10px] text-muted-foreground">
                <span>1</span><span>2</span><span>3</span><span>4</span><span>5</span>
              </div>
            </div>
            <div className="space-y-2">
              <Label>
                Restrisiko-Auswirkung
                {form.residualImpact != null && (
                  <span className="ml-1 font-mono text-muted-foreground">{form.residualImpact}/5</span>
                )}
              </Label>
              <input
                type="range"
                min={1}
                max={5}
                step={1}
                className="w-full accent-primary"
                aria-label="Restrisiko-Auswirkung"
                value={form.residualImpact ?? 3}
                onChange={(e) => { set({ residualImpact: parseInt(e.target.value, 10) }); }}
              />
              <div className="flex justify-between text-[10px] text-muted-foreground">
                <span>1</span><span>2</span><span>3</span><span>4</span><span>5</span>
              </div>
            </div>
          </div>

          {residualScore != null && (
            <div className="flex items-center gap-2 text-sm">
              <span className="text-muted-foreground">Restrisiko-Score:</span>
              <span className={`px-2 py-0.5 rounded-md text-white text-xs font-semibold ${residualColor(residualScore)}`}>
                {residualScore} / 25
              </span>
              <span className="text-muted-foreground text-xs">
                (vorher: {grossScore})
              </span>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Linked Controls */}
      <Card>
        <CardHeader className="pb-2">
          <div className="flex items-center justify-between">
            <CardTitle className="text-sm">Verknüpfte Controls</CardTitle>
            {canWrite && (
              <Button size="sm" variant="outline" className="h-7 text-xs" onClick={() => { setLinkDialogOpen(true); }}>
                <Plus className="w-3.5 h-3.5 mr-1" />
                {t('vaktcomply.riskTreatmentPanel.linkControlTitle')}
              </Button>
            )}
          </div>
        </CardHeader>
        <CardContent className="space-y-1">
          {(!linkedControls || linkedControls.length === 0) && (
            <p className="text-xs text-muted-foreground">Noch keine Controls verknüpft.</p>
          )}
          {(linkedControls ?? []).map((c) => (
            <div key={c.id} className="flex items-start gap-2 group py-1">
              <div className="flex-1 min-w-0">
                <span className="font-mono text-xs text-muted-foreground">{c.control_id}</span>
                <p className="text-xs line-clamp-2">{c.title}</p>
              </div>
              {canWrite && <Button
                size="icon"
                variant="ghost"
                className="h-6 w-6 shrink-0 text-destructive opacity-0 group-hover:opacity-100"
                onClick={() => { unlink.mutate(c.id); }}
                title="Verknüpfung aufheben"
              >
                <Unlink className="w-3 h-3" />
              </Button>}
            </div>
          ))}
        </CardContent>
      </Card>

      {/* Same save as the page header: one form, one save (UX-26). */}
      {canWrite && <div className="flex justify-end">
        <Button onClick={onSave} disabled={!dirty || saving}>
          <Save className="w-4 h-4 mr-1" />
          {saving ? 'Speichern …' : 'Behandlung speichern'}
        </Button>
      </div>}

      <LinkControlDialog
        riskId={riskId}
        open={linkDialogOpen}
        onClose={() => { setLinkDialogOpen(false); }}
      />
    </div>
  )
}

export default RiskTreatmentPanel
