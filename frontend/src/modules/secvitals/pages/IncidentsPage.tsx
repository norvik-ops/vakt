import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Siren, Plus, AlertTriangle } from 'lucide-react'
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
import { useIncidents, useCreateIncident } from '../hooks/useIncidents'
import { useBreaches } from '../../secprivacy/hooks/useBreaches'
import { useCAPAsForSource } from '../hooks/useCAPAs'
import type { Incident, CreateIncidentInput } from '../types'

const SEVERITY_CLASS: Record<Incident['severity'], string> = {
  low: 'bg-green-500/20 text-green-400 border-green-500/30',
  medium: 'bg-amber-500/20 text-amber-400 border-amber-500/30',
  high: 'bg-orange-500/20 text-orange-400 border-orange-500/30',
  critical: 'bg-red-500/20 text-red-400 border-red-500/30',
}

const SEVERITY_LABELS: Record<Incident['severity'], string> = {
  low: 'Niedrig', medium: 'Mittel', high: 'Hoch', critical: 'Kritisch',
}

const STATUS_CLASS: Record<Incident['status'], string> = {
  open: 'bg-red-500/20 text-red-400 border-red-500/30',
  investigating: 'bg-amber-500/20 text-amber-400 border-amber-500/30',
  resolved: 'bg-blue-500/20 text-blue-400 border-blue-500/30',
  closed: 'bg-secondary text-secondary-foreground',
}

const STATUS_LABELS: Record<Incident['status'], string> = {
  open: 'Offen', investigating: 'In Untersuchung', resolved: 'Gelöst', closed: 'Geschlossen',
}

const DEADLINE_STATUS_COLOR: Record<string, string> = {
  red: 'text-red-400',
  yellow: 'text-amber-400',
  green: 'text-green-400',
  done: 'text-green-400',
}

function getWorstDeadline(incident: Incident) {
  const ds = incident.deadline_status
  if (!ds) return null
  const infos = [ds.d_4h, ds.d_24h, ds.d_72h, ds.d_30d].filter(Boolean)
  const pending = infos.filter((d) => d?.status !== 'done')
  if (pending.length === 0) return 'done'
  if (pending.some((d) => d?.status === 'red')) return 'red'
  if (pending.some((d) => d?.status === 'yellow')) return 'yellow'
  return 'green'
}

function IncidentCard({ incident, onClick }: { incident: Incident; onClick: () => void }) {
  const date = new Date(incident.discovered_at).toLocaleDateString('de-DE', {
    year: 'numeric', month: 'short', day: 'numeric',
  })
  const worstDeadline = getWorstDeadline(incident)
  const { data: capas } = useCAPAsForSource('incident', incident.id)

  return (
    <Card className={`cursor-pointer hover:border-brand/50 transition-colors ${incident.status === 'open' ? 'border-red-500/30' : ''}`} onClick={onClick}>
      <CardContent className="pt-5 space-y-2">
        <div className="flex items-start justify-between gap-2">
          <p className="font-medium text-sm">{incident.title}</p>
          <div className="flex gap-1.5 shrink-0">
            {incident.incident_type !== 'general' && (
              <Badge variant="outline" className="text-xs uppercase">{incident.incident_type}</Badge>
            )}
            {capas && capas.length > 0 && (
              <Badge variant="outline" className="text-xs text-blue-400 border-blue-400/40">
                {capas.length} {capas.length === 1 ? 'CAPA' : 'CAPAs'}
              </Badge>
            )}
            <Badge className={SEVERITY_CLASS[incident.severity]}>{SEVERITY_LABELS[incident.severity]}</Badge>
            <Badge className={STATUS_CLASS[incident.status]}>{STATUS_LABELS[incident.status]}</Badge>
          </div>
        </div>
        {incident.description && (
          <p className="text-xs text-muted-foreground line-clamp-2">{incident.description}</p>
        )}
        {incident.affected_systems.length > 0 && (
          <div className="flex flex-wrap gap-1">
            {incident.affected_systems.map((s) => (
              <span key={s} className="text-xs px-1.5 py-0.5 rounded bg-primary/10 text-primary">{s}</span>
            ))}
          </div>
        )}
        <div className="flex items-center justify-between">
          <p className="text-xs text-muted-foreground">Entdeckt: {date}</p>
          {worstDeadline && worstDeadline !== 'done' && (
            <span className={`flex items-center gap-1 text-xs ${DEADLINE_STATUS_COLOR[worstDeadline]}`}>
              <AlertTriangle className="w-3 h-3" />
              Meldefrist läuft
            </span>
          )}
          {worstDeadline === 'done' && (
            <span className="text-xs text-green-400">Alle Meldungen abgeschlossen</span>
          )}
        </div>
      </CardContent>
    </Card>
  )
}

function emptyForm(): CreateIncidentInput {
  return {
    title: '',
    description: '',
    severity: 'medium',
    discovered_at: new Date().toISOString().slice(0, 16),
    affected_systems: [],
    incident_type: 'general',
    reporting_obligation: 'unknown',
    notification_authority: '',
  }
}

export default function IncidentsPage() {
  const navigate = useNavigate()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [form, setForm] = useState<CreateIncidentInput>(emptyForm())
  const [rawSystems, setRawSystems] = useState('')
  const [page, setPage] = useState(1)

  const { data: incidents, isLoading, isError, pagination } = useIncidents(page)
  const { data: breaches } = useBreaches()
  const createIncident = useCreateIncident()

  function openDialog() {
    setForm(emptyForm())
    setRawSystems('')
    setDialogOpen(true)
  }

  function handleSubmit() {
    const payload: CreateIncidentInput = {
      ...form,
      discovered_at: new Date(form.discovered_at).toISOString(),
      affected_systems: rawSystems.split(',').map((s) => s.trim()).filter(Boolean),
    }
    createIncident.mutate(payload, { onSuccess: () => setDialogOpen(false) })
  }

  const open = incidents?.filter((i) => i.status === 'open' || i.status === 'investigating') ?? []
  const closed = incidents?.filter((i) => i.status === 'resolved' || i.status === 'closed') ?? []

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="Vorfallregister"
        description="Sicherheitsvorfälle dokumentieren und verfolgen."
        actions={
          <Button onClick={openDialog} variant="destructive">
            <Plus className="w-4 h-4 mr-1" />
            Vorfall melden
          </Button>
        }
      />

      <div className="flex-1 p-6 space-y-6">
        {isLoading && (
          <div className="flex items-center justify-center h-48">
            <div className="w-6 h-6 border-2 border-primary border-t-transparent rounded-full animate-spin" />
          </div>
        )}
        {isError && (
          <div className="text-sm text-red-400 p-4 bg-red-500/10 rounded-lg">Fehler beim Laden des Vorfallregisters.</div>
        )}
        {!isLoading && !isError && incidents?.length === 0 && (
          <EmptyState
            icon={Siren}
            title="Keine Vorfälle gemeldet"
            description="Dokumentieren Sie Sicherheitsvorfälle für Audit-Nachweise und Lerneffekte."
            action={<Button onClick={openDialog} variant="destructive"><Plus className="w-4 h-4 mr-1" />Vorfall melden</Button>}
          />
        )}
        {!isLoading && !isError && incidents && incidents.length > 0 && (
          <>
            {open.length > 0 && (
              <div className="space-y-3">
                <h2 className="text-sm font-semibold text-red-400">Aktive Vorfälle ({open.length})</h2>
                <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
                  {open.map((i) => <IncidentCard key={i.id} incident={i} onClick={() => navigate(`/secvitals/incidents/${i.id}`)} />)}
                </div>
              </div>
            )}
            {closed.length > 0 && (
              <div className="space-y-3">
                <h2 className="text-sm font-semibold text-muted-foreground">Abgeschlossen</h2>
                <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
                  {closed.map((i) => <IncidentCard key={i.id} incident={i} onClick={() => navigate(`/secvitals/incidents/${i.id}`)} />)}
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
          <DialogHeader><DialogTitle>Vorfall melden</DialogTitle></DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-1.5">
              <Label htmlFor="inc-title">Bezeichnung *</Label>
              <Input id="inc-title" placeholder="z.B. Phishing-Angriff auf Buchhaltung" value={form.title}
                onChange={(e) => setForm((f) => ({ ...f, title: e.target.value }))} />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="inc-severity">Schweregrad *</Label>
              <Select value={form.severity} onValueChange={(v) => setForm((f) => ({ ...f, severity: v as Incident['severity'] }))}>
                <SelectTrigger id="inc-severity"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="low">Niedrig</SelectItem>
                  <SelectItem value="medium">Mittel</SelectItem>
                  <SelectItem value="high">Hoch</SelectItem>
                  <SelectItem value="critical">Kritisch</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="inc-discovered">Entdeckungszeitpunkt</Label>
              <Input id="inc-discovered" type="datetime-local" value={form.discovered_at}
                onChange={(e) => setForm((f) => ({ ...f, discovered_at: e.target.value }))} />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label>Vorfalltyp</Label>
                <Select value={form.incident_type ?? 'general'} onValueChange={(v) => setForm((f) => ({ ...f, incident_type: v as Incident['incident_type'] }))}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="general">Allgemein</SelectItem>
                    <SelectItem value="nis2">NIS2</SelectItem>
                    <SelectItem value="dora">DORA</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <Label>Meldepflicht</Label>
                <Select value={form.reporting_obligation ?? 'unknown'} onValueChange={(v) => setForm((f) => ({ ...f, reporting_obligation: v as Incident['reporting_obligation'] }))}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="unknown">Unbekannt</SelectItem>
                    <SelectItem value="required">Meldepflichtig</SelectItem>
                    <SelectItem value="not_required">Keine Meldepflicht</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
            {(form.incident_type === 'nis2' || form.incident_type === 'dora') && (
              <div className="space-y-1.5">
                <Label htmlFor="inc-authority">Meldebehörde</Label>
                <Input id="inc-authority" placeholder="z.B. BSI, BaFin, BNetzA" value={form.notification_authority ?? ''}
                  onChange={(e) => setForm((f) => ({ ...f, notification_authority: e.target.value }))} />
              </div>
            )}
            {form.incident_type === 'dora' && (
              <>
                <div className="space-y-1.5">
                  <Label htmlFor="inc-customers">Betroffene Kunden (DORA)</Label>
                  <Input
                    id="inc-customers"
                    type="number"
                    min={0}
                    placeholder="Anzahl betroffener Kunden"
                    value={form.affected_customers ?? ''}
                    onChange={(e) => setForm((f) => ({ ...f, affected_customers: e.target.value ? Number(e.target.value) : undefined }))}
                    data-testid="create-affected-customers-input"
                  />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="inc-financial">Geschätzter finanzieller Schaden (DORA)</Label>
                  <Textarea
                    id="inc-financial"
                    rows={2}
                    placeholder="z.B. ca. 500.000 EUR"
                    value={form.financial_impact_estimate ?? ''}
                    onChange={(e) => setForm((f) => ({ ...f, financial_impact_estimate: e.target.value }))}
                    data-testid="create-financial-impact-textarea"
                  />
                </div>
                <div className="flex items-center gap-2">
                  <input
                    id="inc-major"
                    type="checkbox"
                    className="w-4 h-4 accent-primary cursor-pointer"
                    checked={form.is_major_incident ?? false}
                    onChange={(e) => setForm((f) => ({ ...f, is_major_incident: e.target.checked }))}
                    data-testid="create-is-major-incident-checkbox"
                  />
                  <Label htmlFor="inc-major" className="cursor-pointer">
                    Schwerwiegender IKT-Vorfall (Art. 18 DORA)
                  </Label>
                </div>
              </>
            )}
            <div className="space-y-1.5">
              <Label htmlFor="inc-desc">Beschreibung *</Label>
              <Textarea id="inc-desc" rows={3} placeholder="Was ist passiert?" value={form.description}
                onChange={(e) => setForm((f) => ({ ...f, description: e.target.value }))} />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="inc-systems">Betroffene Systeme (kommagetrennt)</Label>
              <Input id="inc-systems" placeholder="z.B. E-Mail-Server, CRM, VPN" value={rawSystems}
                onChange={(e) => setRawSystems(e.target.value)} />
            </div>
            {breaches && breaches.length > 0 && (
              <div className="space-y-1.5">
                <Label htmlFor="inc-breach">Verknüpfte Datenpanne (optional)</Label>
                <select
                  id="inc-breach"
                  className="flex w-full rounded-md border border-border bg-surface px-3 py-2 text-[13px] text-primary focus:outline-none focus:border-brand"
                  value={form.breach_id ?? ''}
                  onChange={(e) => setForm((f) => ({ ...f, breach_id: e.target.value || undefined }))}
                >
                  <option value="">— Keine —</option>
                  {breaches.map((b) => (
                    <option key={b.id} value={b.id}>{b.title}</option>
                  ))}
                </select>
              </div>
            )}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDialogOpen(false)}>Abbrechen</Button>
            <Button variant="destructive" onClick={handleSubmit}
              disabled={!form.title || !form.description || createIncident.isPending}>
              {createIncident.isPending ? 'Melden …' : 'Vorfall melden'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
