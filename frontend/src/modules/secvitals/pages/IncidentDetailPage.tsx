import { useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { formatDate, formatDateTime } from '../../../shared/utils/date'
import { ArrowLeft, Save, Clock, CheckCircle2, AlertTriangle, FileDown, ShieldAlert } from 'lucide-react'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Breadcrumbs } from '../../../shared/components/Breadcrumbs'
import { trackPage } from '../../../shared/hooks/useRecentPages'
import { ProGate } from '../../../shared/components/ProGate'
import { FeatureLockedError } from '../../../api/client'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Textarea } from '../../../components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../../../components/ui/select'
import { useIncident, useUpdateIncident, useMarkDeadlineReported, useIncidentReports, useGenerateIncidentReport } from '../hooks/useIncidents'
import { useAICopilot } from '../../../shared/hooks/useAICopilot'
import { toast } from '../../../shared/hooks/useToast'
import { Sparkles } from 'lucide-react'
import { ReportabilityWizard } from '../components/ReportabilityWizard'
import type { Incident, UpdateIncidentInput, DeadlineInfo, IncidentReport } from '../types'

const SEVERITY_CLASS: Record<Incident['severity'], string> = {
  low: 'bg-green-500/20 text-green-400 border-green-500/30',
  medium: 'bg-amber-500/20 text-amber-400 border-amber-500/30',
  high: 'bg-orange-500/20 text-orange-400 border-orange-500/30',
  critical: 'bg-red-500/20 text-red-400 border-red-500/30',
}
const SEVERITY_LABELS: Record<Incident['severity'], string> = {
  low: 'Niedrig', medium: 'Mittel', high: 'Hoch', critical: 'Kritisch',
}
const STATUS_LABELS: Record<Incident['status'], string> = {
  open: 'Offen', investigating: 'In Untersuchung', resolved: 'Gelöst', closed: 'Geschlossen',
}
const INCIDENT_TYPE_LABELS = { general: 'Allgemein', nis2: 'NIS2', dora: 'DORA' }
const OBLIGATION_LABELS = { unknown: 'Unbekannt', required: 'Meldepflichtig', not_required: 'Keine Meldepflicht' }

function deadlineStatusColor(status: DeadlineInfo['status']) {
  switch (status) {
    case 'done': return 'text-green-400'
    case 'green': return 'text-green-400'
    case 'yellow': return 'text-amber-400'
    case 'red': return 'text-red-400'
  }
}

function deadlineBadgeClass(status: DeadlineInfo['status']) {
  switch (status) {
    case 'done': return 'bg-green-500/20 text-green-400 border-green-500/30'
    case 'green': return 'bg-green-500/20 text-green-400 border-green-500/30'
    case 'yellow': return 'bg-amber-500/20 text-amber-400 border-amber-500/30'
    case 'red': return 'bg-red-500/20 text-red-400 border-red-500/30'
  }
}

function deadlineBadgeLabel(status: DeadlineInfo['status']) {
  switch (status) {
    case 'done': return 'Gemeldet'
    case 'green': return 'Offen'
    case 'yellow': return 'Bald fällig'
    case 'red': return 'Überfällig'
  }
}

function DeadlineRow({
  label, info, deadlineKey, incidentId,
}: {
  label: string
  info: DeadlineInfo
  deadlineKey: '4h' | '24h' | '72h' | '30d'
  incidentId: string
}) {
  const mark = useMarkDeadlineReported(incidentId)
  const isDone = info.status === 'done'

  return (
    <div
      className="flex items-center justify-between py-2 border-b border-border last:border-0"
      data-testid={`deadline-row-${deadlineKey}`}
    >
      <div className="flex items-center gap-2">
        {isDone
          ? <CheckCircle2 className="w-4 h-4 text-green-400" />
          : <Clock className={`w-4 h-4 ${deadlineStatusColor(info.status)}`} />
        }
        <div>
          <p className="text-sm font-medium">{label}</p>
          <p className="text-xs text-muted-foreground">
            {new Date(info.deadline).toLocaleString('de-DE')}
            {isDone && info.reported_at && (
              <span className="ml-2 text-green-400">
                ✓ Gemeldet: {new Date(info.reported_at).toLocaleString('de-DE')}
              </span>
            )}
            {!isDone && (
              <span className={`ml-2 ${deadlineStatusColor(info.status)}`}>
                {info.hours_left > 0
                  ? `${Math.round(info.hours_left)}h verbleibend`
                  : `${Math.abs(Math.round(info.hours_left))}h überfällig`}
              </span>
            )}
          </p>
        </div>
        <Badge
          className={`text-xs ml-1 ${deadlineBadgeClass(info.status)}`}
          data-testid={`deadline-badge-${deadlineKey}`}
        >
          {deadlineBadgeLabel(info.status)}
        </Badge>
      </div>
      {!isDone && (
        <Button
          size="sm"
          variant="outline"
          className="text-xs h-7"
          disabled={mark.isPending}
          onClick={() => mark.mutate({ deadline: deadlineKey })}
          data-testid={`deadline-mark-reported-${deadlineKey}`}
        >
          Als gemeldet markieren
        </Button>
      )}
    </div>
  )
}

export default function IncidentDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { data: incident, isLoading, isError } = useIncident(id ?? '')
  const update = useUpdateIncident(id ?? '')
  const { data: incidentReports } = useIncidentReports(id ?? '')
  const generateReport = useGenerateIncidentReport(id ?? '')

  const [form, setForm] = useState<UpdateIncidentInput | null>(null)
  const [rawSystems, setRawSystems] = useState('')
  const [dirty, setDirty] = useState(false)
  const [wizardOpen, setWizardOpen] = useState(false)
  const [pdfError, setPdfError] = useState<Error | null>(null)

  async function handleDownloadPDF() {
    if (!id) return
    const res = await fetch(`/api/v1/secvitals/incidents/${id}/report-pdf`, {
      credentials: 'include',
    })
    if (!res.ok) {
      setPdfError(new FeatureLockedError('report-pdf'))
      return
    }
    const blob = await res.blob()
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `incident-${id}-bafin.pdf`
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
  }

  useEffect(() => {
    if (incident) trackPage(`/secvitals/incidents/${id}`, incident.title, '🚨')
  }, [incident?.id])

  useEffect(() => {
    if (incident && !form) {
      setForm({
        title: incident.title,
        description: incident.description ?? '',
        severity: incident.severity,
        status: incident.status,
        affected_systems: incident.affected_systems,
        incident_type: incident.incident_type,
        reporting_obligation: incident.reporting_obligation,
        notification_authority: incident.notification_authority ?? '',
        affected_customers: incident.affected_customers,
        financial_impact_estimate: incident.financial_impact_estimate ?? '',
        is_major_incident: incident.is_major_incident,
      })
      setRawSystems(incident.affected_systems.join(', '))
    }
  }, [incident, form])

  function set<K extends keyof UpdateIncidentInput>(key: K, value: UpdateIncidentInput[K]) {
    setForm((f) => f ? { ...f, [key]: value } : f)
    setDirty(true)
  }

  function handleSave() {
    if (!form) return
    const payload: UpdateIncidentInput = {
      ...form,
      affected_systems: rawSystems.split(',').map((s) => s.trim()).filter(Boolean),
    }
    update.mutate(payload, { onSuccess: () => setDirty(false) })
  }

  const ds = incident?.deadline_status

  if (isLoading) return (
    <div className="flex items-center justify-center h-48">
      <div className="w-6 h-6 border-2 border-primary border-t-transparent rounded-full animate-spin" />
    </div>
  )
  if (isError || !incident) return (
    <div className="p-6 text-sm text-red-400">Vorfall nicht gefunden.</div>
  )

  return (
    <div className="flex flex-col h-full">
      <Breadcrumbs items={[
        { label: 'SecVitals', href: '/secvitals' },
        { label: 'Incidents', href: '/secvitals/incidents' },
        { label: incident.title },
      ]} />
      <PageHeader
        title={incident.title}
        description={`Entdeckt: ${formatDate(incident.discovered_at)}`}
        actions={
          <div className="flex gap-2">
            <Button variant="outline" onClick={() => navigate('/secvitals/incidents')}>
              <ArrowLeft className="w-4 h-4 mr-1" />
              Zurück
            </Button>
            {incident?.incident_type === 'dora' && (
              <Button
                variant="outline"
                onClick={handleDownloadPDF}
                data-testid="download-pdf-button"
              >
                <FileDown className="w-4 h-4 mr-1" />
                BaFin-Bericht PDF
              </Button>
            )}
            {(incident?.incident_type === 'nis2' || incident?.incident_type === 'general') && (
              <Button
                variant="outline"
                onClick={() => setWizardOpen(true)}
                data-testid="assess-reportability-btn"
              >
                <ShieldAlert className="w-4 h-4 mr-1" />
                Meldepflicht prüfen
              </Button>
            )}
            <Button onClick={handleSave} disabled={!dirty || update.isPending}>
              <Save className="w-4 h-4 mr-1" />
              {update.isPending ? 'Speichern …' : 'Speichern'}
            </Button>
          </div>
        }
      />

      {form && (
        <div className="flex-1 p-6 grid grid-cols-1 lg:grid-cols-3 gap-6">
          <div className="lg:col-span-2 space-y-4">
            <Card>
              <CardHeader><CardTitle className="text-sm">Vorfalldetails</CardTitle></CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-1.5">
                  <Label>Bezeichnung</Label>
                  <Input value={form.title} onChange={(e) => set('title', e.target.value)} />
                </div>
                <div className="space-y-1.5">
                  <div className="flex items-center justify-between">
                    <Label>Beschreibung</Label>
                    <AISuggestActionsButton
                      summary={form.description}
                      type={incident.incident_type}
                      onAppend={(guide) => set('description', `${form.description}\n\n--- KI-Sofortmaßnahmen ---\n${guide}`)}
                    />
                  </div>
                  <Textarea rows={4} value={form.description} onChange={(e) => set('description', e.target.value)} />
                </div>
                <div className="space-y-1.5">
                  <Label>Betroffene Systeme (kommagetrennt)</Label>
                  <Input value={rawSystems} onChange={(e) => { setRawSystems(e.target.value); setDirty(true) }} />
                </div>
              </CardContent>
            </Card>

            {ds && (
              <Card className="border-primary/20">
                <CardHeader>
                  <CardTitle className="text-sm flex items-center gap-2">
                    <AlertTriangle className="w-4 h-4 text-amber-400" />
                    Meldefristen
                    <Badge variant="outline" className="text-xs ml-auto">
                      {incident.incident_type === 'dora' ? 'DORA' : 'NIS2'}
                    </Badge>
                  </CardTitle>
                </CardHeader>
                <CardContent className="divide-y divide-border">
                  {ds.has_4h && ds.d_4h && id && (
                    <DeadlineRow label="Erstmeldung (4h)" info={ds.d_4h} deadlineKey="4h" incidentId={id} />
                  )}
                  {ds.has_24h && ds.d_24h && id && (
                    <DeadlineRow label="Frühmeldung (24h)" info={ds.d_24h} deadlineKey="24h" incidentId={id} />
                  )}
                  {ds.has_72h && ds.d_72h && id && (
                    <DeadlineRow label="Vollständige Meldung (72h)" info={ds.d_72h} deadlineKey="72h" incidentId={id} />
                  )}
                  {ds.has_30d && ds.d_30d && id && (
                    <DeadlineRow label="Abschlussbericht (30 Tage)" info={ds.d_30d} deadlineKey="30d" incidentId={id} />
                  )}
                </CardContent>
              </Card>
            )}

            {/* Meldungsformular buttons (NIS2 and DORA incidents) */}
            {(incident.incident_type === 'nis2' || incident.incident_type === 'dora') && id && (
              <Card data-testid="report-form-card">
                <CardHeader>
                  <CardTitle className="text-sm flex items-center gap-2">
                    <FileDown className="w-4 h-4" />
                    Meldungsformulare
                  </CardTitle>
                </CardHeader>
                <CardContent className="space-y-3">
                  <div className="flex flex-wrap gap-2">
                    {(['24h', '72h', '30d'] as const).map((rt) => (
                      <Button
                        key={rt}
                        size="sm"
                        variant="outline"
                        disabled={generateReport.isPending}
                        onClick={() => generateReport.mutate({ report_type: rt })}
                        data-testid={`generate-report-btn-${rt}`}
                      >
                        Meldung {rt} erstellen
                      </Button>
                    ))}
                  </div>

                  {incidentReports && incidentReports.length > 0 && (
                    <div data-testid="report-history">
                      <p className="text-xs text-muted-foreground mb-2">Meldungshistorie:</p>
                      <div className="space-y-1">
                        {incidentReports.map((r: IncidentReport) => (
                          <div key={r.id} className="flex items-center justify-between text-xs bg-muted/30 rounded px-2 py-1.5">
                            <span className="font-medium">{r.report_type} — {r.authority}</span>
                            <span className="text-muted-foreground">
                              {new Date(r.generated_at).toLocaleString('de-DE')}
                            </span>
                            <a
                              href={`/api/v1/secvitals/incident-reports/${r.id}/pdf`}
                              target="_blank"
                              rel="noopener noreferrer"
                              className="text-primary hover:underline ml-2"
                              data-testid={`download-report-${r.id}`}
                            >
                              PDF
                            </a>
                          </div>
                        ))}
                      </div>
                    </div>
                  )}
                </CardContent>
              </Card>
            )}

            {form.incident_type === 'dora' && (
              <Card className="border-blue-500/30" data-testid="dora-fields-card">
                <CardHeader>
                  <CardTitle className="text-sm flex items-center gap-2">
                    DORA-spezifische Angaben
                    {incident.is_major_incident && (
                      <Badge className="text-xs bg-red-500/20 text-red-400 border-red-500/30 ml-auto" data-testid="major-incident-badge">
                        Art. 18 DORA — Schwerwiegend
                      </Badge>
                    )}
                  </CardTitle>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="space-y-1.5">
                    <Label htmlFor="affected-customers">Betroffene Kunden</Label>
                    <Input
                      id="affected-customers"
                      type="number"
                      min={0}
                      placeholder="Anzahl betroffener Kunden"
                      value={form.affected_customers ?? ''}
                      onChange={(e) => set('affected_customers', e.target.value ? Number(e.target.value) : undefined)}
                      data-testid="affected-customers-input"
                    />
                  </div>
                  <div className="space-y-1.5">
                    <Label htmlFor="financial-impact">Geschätzter finanzieller Schaden</Label>
                    <Textarea
                      id="financial-impact"
                      rows={2}
                      placeholder="z.B. ca. 500.000 EUR"
                      value={form.financial_impact_estimate ?? ''}
                      onChange={(e) => set('financial_impact_estimate', e.target.value)}
                      data-testid="financial-impact-textarea"
                    />
                  </div>
                  <div className="flex items-center gap-2">
                    <input
                      id="is-major-incident"
                      type="checkbox"
                      className="w-4 h-4 accent-primary cursor-pointer"
                      checked={form.is_major_incident ?? false}
                      onChange={(e) => set('is_major_incident', e.target.checked)}
                      data-testid="is-major-incident-checkbox"
                    />
                    <Label htmlFor="is-major-incident" className="cursor-pointer">
                      Schwerwiegender IKT-Vorfall (Art. 18 DORA)
                    </Label>
                  </div>
                </CardContent>
              </Card>
            )}

            {incident?.breach_id && (
              <Card className="border-amber-500/30 bg-amber-500/5">
                <CardHeader className="pb-2">
                  <CardTitle className="text-sm flex items-center gap-2">
                    <ShieldAlert className="w-4 h-4 text-amber-400" />
                    Verknüpfte Datenpanne
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <a
                    href={`/secprivacy/breaches/${incident.breach_id}`}
                    className="text-sm text-amber-400 hover:underline"
                  >
                    Datenpanne öffnen →
                  </a>
                </CardContent>
              </Card>
            )}
          </div>

          <div className="space-y-4">
            <Card>
              <CardHeader><CardTitle className="text-sm">Klassifizierung</CardTitle></CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-1.5">
                  <Label>Schweregrad</Label>
                  <Select value={form.severity} onValueChange={(v) => set('severity', v as Incident['severity'])}>
                    <SelectTrigger><SelectValue /></SelectTrigger>
                    <SelectContent>
                      {(Object.keys(SEVERITY_LABELS) as Incident['severity'][]).map((k) => (
                        <SelectItem key={k} value={k}>
                          <span className="flex items-center gap-2">
                            <span className={`inline-block w-2 h-2 rounded-full ${SEVERITY_CLASS[k].split(' ')[0]}`} />
                            {SEVERITY_LABELS[k]}
                          </span>
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-1.5">
                  <Label>Status</Label>
                  <Select value={form.status} onValueChange={(v) => set('status', v as Incident['status'])}>
                    <SelectTrigger><SelectValue /></SelectTrigger>
                    <SelectContent>
                      {(Object.keys(STATUS_LABELS) as Incident['status'][]).map((k) => (
                        <SelectItem key={k} value={k}>{STATUS_LABELS[k]}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="flex gap-2 flex-wrap text-xs">
                  <Badge className={SEVERITY_CLASS[form.severity]}>{SEVERITY_LABELS[form.severity]}</Badge>
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader><CardTitle className="text-sm">Meldepflicht</CardTitle></CardHeader>
              <CardContent className="space-y-3">
                <div className="space-y-1.5">
                  <Label>Vorfalltyp</Label>
                  <Select value={form.incident_type ?? 'general'} onValueChange={(v) => set('incident_type', v as Incident['incident_type'])}>
                    <SelectTrigger><SelectValue /></SelectTrigger>
                    <SelectContent>
                      {Object.entries(INCIDENT_TYPE_LABELS).map(([k, label]) => (
                        <SelectItem key={k} value={k}>{label}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-1.5">
                  <Label>Meldepflicht</Label>
                  <Select value={form.reporting_obligation ?? 'unknown'} onValueChange={(v) => set('reporting_obligation', v as Incident['reporting_obligation'])}>
                    <SelectTrigger><SelectValue /></SelectTrigger>
                    <SelectContent>
                      {Object.entries(OBLIGATION_LABELS).map(([k, label]) => (
                        <SelectItem key={k} value={k}>{label}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-1.5">
                  <Label>Behörde / Meldestelle</Label>
                  <Input
                    placeholder="z.B. BSI, BaFin, BNetzA"
                    value={form.notification_authority ?? ''}
                    onChange={(e) => set('notification_authority', e.target.value)}
                  />
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardContent className="pt-4 space-y-1 text-xs text-muted-foreground">
                <p>Entdeckt: {formatDateTime(incident.discovered_at)}</p>
                {incident.resolved_at && <p>Gelöst: {formatDateTime(incident.resolved_at)}</p>}
                <p>Erstellt: {formatDate(incident.created_at)}</p>
              </CardContent>
            </Card>
          </div>
        </div>
      )}

      {id && (
        <ReportabilityWizard
          incidentId={id}
          open={wizardOpen}
          onClose={() => setWizardOpen(false)}
        />
      )}
      <ProGate error={pdfError}>{null}</ProGate>
    </div>
  )
}

interface AISuggestActionsProps {
  summary: string
  type: string | undefined
  onAppend: (guide: string) => void
}

// AISuggestActionsButton calls the AI copilot (POST /secvitals/ai/incident-guide)
// and appends the returned numbered checklist to the incident description.
// Disabled while the description is empty — the LLM needs context to work with.
function AISuggestActionsButton({ summary, type, onAppend }: AISuggestActionsProps) {
  const { incidentGuide } = useAICopilot()
  const handleClick = () => {
    if (!summary.trim()) return
    incidentGuide.mutate(
      { summary, type: type ?? '' },
      {
        onSuccess: (resp) => {
          onAppend(resp.guide)
          toast('KI-Sofortmaßnahmen angehängt', { variant: 'success' })
        },
        onError: () => {
          toast('KI temporär nicht verfügbar', { variant: 'error' })
        },
      },
    )
  }
  return (
    <button
      type="button"
      onClick={handleClick}
      disabled={!summary.trim() || incidentGuide.isPending}
      className="text-[11px] text-primary hover:underline disabled:opacity-50 disabled:cursor-not-allowed inline-flex items-center gap-1"
      title="KI-Vorschlag für Sofortmaßnahmen anhängen (lokales LLM)"
    >
      <Sparkles className="w-3 h-3" aria-hidden="true" />
      {incidentGuide.isPending ? 'KI denkt…' : 'KI-Sofortmaßnahmen'}
    </button>
  )
}
