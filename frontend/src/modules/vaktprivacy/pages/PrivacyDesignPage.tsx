import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { ShieldCheck, AlertCircle, CheckCircle2, PenSquare } from 'lucide-react'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Card, CardContent } from '../../../components/ui/card'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../../../components/ui/dialog'
import { Textarea } from '../../../components/ui/textarea'
import { Label } from '../../../components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../../../components/ui/select'
import { apiFetch } from '../../../api/client'
import { EmptyState } from '../../../shared/components/EmptyState'
import { SkeletonTable } from '../../../shared/components/SkeletonLoaders'

interface PrivacyDesignSummary {
  total_activities: number
  with_assessment: number
  compliant: number
  partially: number
  not_assessed: number
  pending_count: number
  pct_compliant: number
}

interface PrivacyDesignAssessment {
  id: string
  processing_activity_id: string
  design_measures: string
  design_at_conception: boolean
  risk_considered: boolean
  data_minimization: boolean
  purpose_limitation: boolean
  storage_limitation: boolean
  access_limitation: boolean
  default_settings_note: string
  assessment_result: 'compliant' | 'partially' | 'not_assessed'
  reviewed_by?: string
  reviewed_at?: string
}

interface VVTEntry {
  id: string
  name: string
  status: string
}

interface PrivacyDesignInput {
  design_measures: string
  design_at_conception: boolean
  risk_considered: boolean
  data_minimization: boolean
  purpose_limitation: boolean
  storage_limitation: boolean
  access_limitation: boolean
  default_settings_note: string
  assessment_result: string
}

const RESULT_BADGE: Record<string, { label: string; variant: 'success' | 'warning' | 'outline' }> = {
  compliant:    { label: 'Konform', variant: 'success' },
  partially:    { label: 'Teilweise', variant: 'warning' },
  not_assessed: { label: 'Nicht bewertet', variant: 'outline' },
}

const defaultInput: PrivacyDesignInput = {
  design_measures: '',
  design_at_conception: false,
  risk_considered: false,
  data_minimization: false,
  purpose_limitation: false,
  storage_limitation: false,
  access_limitation: false,
  default_settings_note: '',
  assessment_result: 'not_assessed',
}

export default function PrivacyDesignPage() {
  const qc = useQueryClient()
  const [selectedActivity, setSelectedActivity] = useState<VVTEntry | null>(null)
  const [form, setForm] = useState<PrivacyDesignInput>(defaultInput)

  const { data: summary, isLoading: summaryLoading } = useQuery<PrivacyDesignSummary>({
    queryKey: ['vaktprivacy', 'privacy-design-summary'],
    queryFn: () => apiFetch('/vaktprivacy/privacy-design/summary'),
  })

  const { data: vvtEntries, isLoading: vvtLoading } = useQuery<VVTEntry[]>({
    queryKey: ['vaktprivacy', 'vvt'],
    queryFn: () => apiFetch('/vaktprivacy/vvt'),
  })

  const { data: currentAssessment } = useQuery<PrivacyDesignAssessment | null>({
    queryKey: ['vaktprivacy', 'privacy-design', selectedActivity?.id],
    queryFn: async () => {
      const res = await apiFetch<PrivacyDesignAssessment | { assessment: null }>(
        `/vaktprivacy/processing-activities/${selectedActivity!.id}/privacy-design`
      )
      if ('assessment' in res && res.assessment === null) return null
      return res as PrivacyDesignAssessment
    },
    enabled: !!selectedActivity,
  })

  const upsertMutation = useMutation({
    mutationFn: (input: PrivacyDesignInput) =>
      apiFetch(`/vaktprivacy/processing-activities/${selectedActivity!.id}/privacy-design`, {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['vaktprivacy', 'privacy-design-summary'] })
      qc.invalidateQueries({ queryKey: ['vaktprivacy', 'privacy-design', selectedActivity?.id] })
      setSelectedActivity(null)
    },
  })

  const openDialog = (activity: VVTEntry) => {
    setSelectedActivity(activity)
    if (currentAssessment) {
      setForm({
        design_measures: currentAssessment.design_measures,
        design_at_conception: currentAssessment.design_at_conception,
        risk_considered: currentAssessment.risk_considered,
        data_minimization: currentAssessment.data_minimization,
        purpose_limitation: currentAssessment.purpose_limitation,
        storage_limitation: currentAssessment.storage_limitation,
        access_limitation: currentAssessment.access_limitation,
        default_settings_note: currentAssessment.default_settings_note,
        assessment_result: currentAssessment.assessment_result,
      })
    } else {
      setForm(defaultInput)
    }
  }

  const isLoading = summaryLoading || vvtLoading

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-semibold">Privacy by Design (Art. 25 DSGVO)</h1>
        <p className="text-sm text-secondary mt-1">
          Dokumentieren Sie technische und organisatorische Maßnahmen nach Art. 25 Abs. 1 (by Design) und Abs. 2 (by Default) für jede Verarbeitungstätigkeit.
        </p>
      </div>

      {summary && (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <Card>
            <CardContent className="pt-4">
              <p className="text-xs text-secondary">Gesamt</p>
              <p className="text-2xl font-bold">{summary.total_activities}</p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-4">
              <p className="text-xs text-secondary">Konform</p>
              <p className="text-2xl font-bold text-green-400">{summary.compliant}</p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-4">
              <p className="text-xs text-secondary">Ausstehend</p>
              <p className="text-2xl font-bold text-warning">{summary.pending_count}</p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-4">
              <p className="text-xs text-secondary">Konformität</p>
              <p className="text-2xl font-bold">{summary.pct_compliant.toFixed(0)}%</p>
            </CardContent>
          </Card>
        </div>
      )}

      {isLoading && <SkeletonTable rows={4} />}

      {!isLoading && !vvtEntries?.length && (
        <EmptyState
          icon={<ShieldCheck className="w-8 h-8" />}
          title="Keine Verarbeitungstätigkeiten"
          description="Legen Sie zuerst Verarbeitungstätigkeiten in der VVT an."
        />
      )}

      {vvtEntries && vvtEntries.length > 0 && (
        <ActivityAssessmentTable
          activities={vvtEntries}
          onEdit={openDialog}
        />
      )}

      <Dialog open={!!selectedActivity} onOpenChange={open => { if (!open) setSelectedActivity(null) }}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>Art. 25 Bewertung – {selectedActivity?.name}</DialogTitle>
          </DialogHeader>
          <form
            className="space-y-4 mt-2"
            onSubmit={e => {
              e.preventDefault()
              upsertMutation.mutate(form)
            }}
          >
            <div>
              <Label>Technische Maßnahmen (Art. 25 Abs. 1)</Label>
              <Textarea
                rows={3}
                value={form.design_measures}
                onChange={e => setForm(f => ({ ...f, design_measures: e.target.value }))}
                placeholder="z.B. Datenverschlüsselung, Pseudonymisierung, Datensparsamkeit bei Systemdesign…"
              />
            </div>

            <div className="space-y-2">
              <p className="text-sm font-medium">Art. 25 Abs. 1 – Privacy by Design</p>
              {([
                ['design_at_conception', 'Datenschutz bereits bei Konzeption berücksichtigt'],
                ['risk_considered', 'Risiken für Betroffenenrechte einbezogen'],
              ] as [keyof PrivacyDesignInput, string][]).map(([field, label]) => (
                <label key={field} className="flex items-center gap-2 text-sm cursor-pointer">
                  <input
                    type="checkbox"
                    checked={form[field] as boolean}
                    onChange={e => setForm(f => ({ ...f, [field]: e.target.checked }))}
                  />
                  {label}
                </label>
              ))}
            </div>

            <div className="space-y-2">
              <p className="text-sm font-medium">Art. 25 Abs. 2 – Privacy by Default</p>
              {([
                ['data_minimization', 'Datensparsamkeit (nur notwendige Daten)'],
                ['purpose_limitation', 'Zweckbindung sichergestellt'],
                ['storage_limitation', 'Speicherbegrenzung implementiert'],
                ['access_limitation', 'Zugriffsbeschränkung auf das Notwendige'],
              ] as [keyof PrivacyDesignInput, string][]).map(([field, label]) => (
                <label key={field} className="flex items-center gap-2 text-sm cursor-pointer">
                  <input
                    type="checkbox"
                    checked={form[field] as boolean}
                    onChange={e => setForm(f => ({ ...f, [field]: e.target.checked }))}
                  />
                  {label}
                </label>
              ))}
            </div>

            <div>
              <Label>Standardeinstellungen (Hinweis)</Label>
              <Textarea
                rows={2}
                value={form.default_settings_note}
                onChange={e => setForm(f => ({ ...f, default_settings_note: e.target.value }))}
                placeholder="z.B. Opt-in statt Opt-out, minimale Profilsichtbarkeit…"
              />
            </div>

            <div>
              <Label>Gesamtbewertung</Label>
              <Select
                value={form.assessment_result}
                onValueChange={v => setForm(f => ({ ...f, assessment_result: v }))}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="compliant">Konform</SelectItem>
                  <SelectItem value="partially">Teilweise konform</SelectItem>
                  <SelectItem value="not_assessed">Nicht bewertet</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setSelectedActivity(null)}>Abbrechen</Button>
              <Button type="submit" disabled={upsertMutation.isPending}>
                {upsertMutation.isPending ? 'Speichern…' : 'Speichern'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  )
}

function ActivityAssessmentTable({
  activities,
  onEdit,
}: {
  activities: VVTEntry[]
  onEdit: (a: VVTEntry) => void
}) {
  return (
    <div className="space-y-2">
      {activities.map(activity => (
        <ActivityRow key={activity.id} activity={activity} onEdit={onEdit} />
      ))}
    </div>
  )
}

function ActivityRow({ activity, onEdit }: { activity: VVTEntry; onEdit: (a: VVTEntry) => void }) {
  const { data: assessment } = useQuery<PrivacyDesignAssessment | null>({
    queryKey: ['vaktprivacy', 'privacy-design', activity.id],
    queryFn: async () => {
      const res = await apiFetch<PrivacyDesignAssessment | { assessment: null }>(
        `/vaktprivacy/processing-activities/${activity.id}/privacy-design`
      )
      if ('assessment' in res && res.assessment === null) return null
      return res as PrivacyDesignAssessment
    },
  })

  const resultInfo = assessment
    ? RESULT_BADGE[assessment.assessment_result] ?? RESULT_BADGE.not_assessed
    : null

  return (
    <div className="flex items-center justify-between px-4 py-3 bg-surface border border-border rounded-lg gap-3">
      <div className="flex items-center gap-3 min-w-0">
        {!assessment ? (
          <AlertCircle className="w-4 h-4 text-warning shrink-0" />
        ) : assessment.assessment_result === 'compliant' ? (
          <CheckCircle2 className="w-4 h-4 text-green-400 shrink-0" />
        ) : (
          <AlertCircle className="w-4 h-4 text-warning shrink-0" />
        )}
        <p className="text-sm font-medium truncate">{activity.name}</p>
      </div>
      <div className="flex items-center gap-2 shrink-0">
        {resultInfo ? (
          <Badge variant={resultInfo.variant} className="text-xs">{resultInfo.label}</Badge>
        ) : (
          <Badge variant="outline" className="text-xs">Keine Bewertung</Badge>
        )}
        <Button size="sm" variant="ghost" onClick={() => onEdit(activity)}>
          <PenSquare className="w-3 h-3" />
        </Button>
      </div>
    </div>
  )
}
