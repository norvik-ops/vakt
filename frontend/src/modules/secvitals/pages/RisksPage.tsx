import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { ShieldAlert, Plus, List, BarChart2 } from 'lucide-react'
import { Button } from '../../../components/ui/button'
import { Card, CardContent } from '../../../components/ui/card'
import { Badge } from '../../../components/ui/badge'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../../../components/ui/dialog'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Textarea } from '../../../components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../../../components/ui/select'
import { PageHeader } from '../../../shared/components/PageHeader'
import { EmptyState } from '../../../shared/components/EmptyState'
import { Pagination } from '../../../shared/components/Pagination'
import { useRisks, useCreateRisk } from '../hooks/useRisks'
import RiskHeatmap from '../components/RiskHeatmap'
import type { Risk, CreateRiskInput } from '../types'

const SCORE_COLOR = (score: number) => {
  if (score >= 15) return 'bg-red-500/20 text-red-400 border-red-500/30'
  if (score >= 9) return 'bg-amber-500/20 text-amber-400 border-amber-500/30'
  if (score >= 4) return 'bg-yellow-500/20 text-yellow-400 border-yellow-500/30'
  return 'bg-green-500/20 text-green-400 border-green-500/30'
}

const STATUS_LABELS: Record<Risk['status'], string> = {
  open: 'Offen',
  mitigated: 'Gemindert',
  accepted: 'Akzeptiert',
  closed: 'Geschlossen',
}

const TREATMENT_LABELS: Record<Risk['treatment'], string> = {
  avoid: 'Vermeiden',
  mitigate: 'Mindern',
  transfer: 'Übertragen',
  accept: 'Akzeptieren',
}

function RiskCard({ risk, onClick }: { risk: Risk; onClick: () => void }) {
  return (
    <Card className="cursor-pointer hover:border-brand/50 transition-colors" onClick={onClick}>
      <CardContent className="pt-5 space-y-3">
        <div className="flex items-start justify-between gap-2">
          <div>
            <p className="font-medium text-sm">{risk.title}</p>
            {risk.category && <p className="text-xs text-muted-foreground mt-0.5">{risk.category}</p>}
          </div>
          <Badge className={SCORE_COLOR(risk.risk_score)}>Score {risk.risk_score}</Badge>
        </div>
        {risk.description && (
          <p className="text-xs text-muted-foreground line-clamp-2">{risk.description}</p>
        )}
        <div className="grid grid-cols-2 gap-2 text-xs text-muted-foreground">
          <span>Wahrscheinlichkeit: <strong className="text-foreground">{risk.likelihood}/5</strong></span>
          <span>Auswirkung: <strong className="text-foreground">{risk.impact}/5</strong></span>
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
  const navigate = useNavigate()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [form, setForm] = useState<CreateRiskInput>(emptyForm())
  const [view, setView] = useState<'list' | 'heatmap'>('list')
  const [page, setPage] = useState(1)
  const { data: risks, isLoading, isError, pagination } = useRisks(page)
  const createRisk = useCreateRisk()

  function openDialog() {
    setForm(emptyForm())
    setDialogOpen(true)
  }

  function handleSubmit() {
    createRisk.mutate(form, { onSuccess: () => setDialogOpen(false) })
  }

  const high = risks?.filter((r) => r.risk_score >= 15) ?? []
  const medium = risks?.filter((r) => r.risk_score >= 9 && r.risk_score < 15) ?? []
  const low = risks?.filter((r) => r.risk_score < 9) ?? []

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="Risikoregister"
        description="Identifizieren, bewerten und behandeln Sie Risiken für Ihre Organisation."
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
                Liste
              </Button>
              <Button
                size="sm"
                variant={view === 'heatmap' ? 'secondary' : 'ghost'}
                className="h-7 px-2.5 text-xs"
                onClick={() => setView('heatmap')}
              >
                <BarChart2 className="w-3.5 h-3.5 mr-1" />
                Heatmap
              </Button>
            </div>
            <Button onClick={openDialog}>
              <Plus className="w-4 h-4 mr-1" />
              Risiko erfassen
            </Button>
          </div>
        }
      />

      <div className="flex-1 p-6 space-y-6">
        {isLoading && (
          <div className="flex items-center justify-center h-48">
            <div className="w-6 h-6 border-2 border-primary border-t-transparent rounded-full animate-spin" />
          </div>
        )}
        {isError && (
          <div className="text-sm text-red-400 p-4 bg-red-500/10 rounded-lg">Fehler beim Laden des Risikoregisters.</div>
        )}
        {!isLoading && !isError && risks?.length === 0 && (
          <EmptyState
            icon={ShieldAlert}
            title="Kein Risiko erfasst"
            description="Dokumentieren Sie Risiken und deren Behandlungsmaßnahmen."
            action={<Button onClick={openDialog}><Plus className="w-4 h-4 mr-1" />Risiko erfassen</Button>}
          />
        )}
        {!isLoading && !isError && risks && risks.length > 0 && view === 'heatmap' && (
          <RiskHeatmap risks={risks} />
        )}
        {!isLoading && !isError && risks && risks.length > 0 && view === 'list' && (
          <>
            {high.length > 0 && (
              <div className="space-y-3">
                <h2 className="text-sm font-semibold text-red-400">Hohes Risiko (Score ≥ 15)</h2>
                <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
                  {high.map((r) => <RiskCard key={r.id} risk={r} onClick={() => navigate(`/secvitals/risks/${r.id}`)} />)}
                </div>
              </div>
            )}
            {medium.length > 0 && (
              <div className="space-y-3">
                <h2 className="text-sm font-semibold text-amber-400">Mittleres Risiko (Score 9–14)</h2>
                <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
                  {medium.map((r) => <RiskCard key={r.id} risk={r} onClick={() => navigate(`/secvitals/risks/${r.id}`)} />)}
                </div>
              </div>
            )}
            {low.length > 0 && (
              <div className="space-y-3">
                <h2 className="text-sm font-semibold text-muted-foreground">Niedriges Risiko</h2>
                <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
                  {low.map((r) => <RiskCard key={r.id} risk={r} onClick={() => navigate(`/secvitals/risks/${r.id}`)} />)}
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

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="max-w-lg max-h-[90vh] overflow-y-auto">
          <DialogHeader><DialogTitle>Risiko erfassen</DialogTitle></DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-1.5">
              <Label htmlFor="risk-title">Bezeichnung *</Label>
              <Input id="risk-title" placeholder="z.B. Datenverlust durch Ransomware" value={form.title}
                onChange={(e) => setForm((f) => ({ ...f, title: e.target.value }))} />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="risk-category">Kategorie</Label>
              <Input id="risk-category" placeholder="z.B. Cyber, Compliance, Betrieb" value={form.category ?? ''}
                onChange={(e) => setForm((f) => ({ ...f, category: e.target.value }))} />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="risk-desc">Beschreibung</Label>
              <Textarea id="risk-desc" rows={2} placeholder="Was könnte passieren und warum?" value={form.description ?? ''}
                onChange={(e) => setForm((f) => ({ ...f, description: e.target.value }))} />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label htmlFor="risk-likelihood">Wahrscheinlichkeit (1–5) *</Label>
                <Input id="risk-likelihood" type="number" min={1} max={5} value={form.likelihood}
                  onChange={(e) => setForm((f) => ({ ...f, likelihood: parseInt(e.target.value, 10) || 1 }))} />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="risk-impact">Auswirkung (1–5) *</Label>
                <Input id="risk-impact" type="number" min={1} max={5} value={form.impact}
                  onChange={(e) => setForm((f) => ({ ...f, impact: parseInt(e.target.value, 10) || 1 }))} />
              </div>
            </div>
            <div className="text-xs text-muted-foreground">
              Voraussichtlicher Risiko-Score: <strong className={`${SCORE_COLOR(form.likelihood * form.impact)} px-1 py-0.5 rounded`}>{form.likelihood * form.impact}</strong>
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="risk-treatment">Behandlungsstrategie *</Label>
              <Select value={form.treatment} onValueChange={(v) => setForm((f) => ({ ...f, treatment: v as Risk['treatment'] }))}>
                <SelectTrigger id="risk-treatment"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="avoid">Vermeiden</SelectItem>
                  <SelectItem value="mitigate">Mindern</SelectItem>
                  <SelectItem value="transfer">Übertragen (z.B. Versicherung)</SelectItem>
                  <SelectItem value="accept">Akzeptieren</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="risk-treatment-notes">Maßnahmen</Label>
              <Textarea id="risk-treatment-notes" rows={2} placeholder="Konkrete Maßnahmen zur Risikobehandlung …" value={form.treatment_notes ?? ''}
                onChange={(e) => setForm((f) => ({ ...f, treatment_notes: e.target.value }))} />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="risk-owner">Verantwortlicher</Label>
              <Input id="risk-owner" placeholder="z.B. Max Mustermann" value={form.owner ?? ''}
                onChange={(e) => setForm((f) => ({ ...f, owner: e.target.value }))} />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDialogOpen(false)}>Abbrechen</Button>
            <Button onClick={handleSubmit} disabled={!form.title || !form.treatment || createRisk.isPending}>
              {createRisk.isPending ? 'Speichern …' : 'Risiko erfassen'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
