import { useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'
import { Spinner } from '../../../components/Spinner'
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
import { useAIStatus } from '../hooks/useAIAdvisor'
import { toast } from '../../../shared/hooks/useToast'
import { Sparkles } from 'lucide-react'
import { ReportabilityWizard } from '../components/ReportabilityWizard'
import { ClassifyReportingWizard } from '../components/ClassifyReportingWizard'
import { NIS2StagePanel } from '../components/NIS2StagePanel'
import type { Incident, UpdateIncidentInput, DeadlineInfo, IncidentReport } from '../types'

const SEVERITY_CLASS: Record<Incident['severity'], string> = {
  low: 'bg-green-500/20 text-green-400 border-green-500/30',
  medium: 'bg-amber-500/20 text-amber-400 border-amber-500/30',
  high: 'bg-orange-500/20 text-orange-400 border-orange-500/30',
  critical: 'bg-red-500/20 text-red-400 border-red-500/30',
}

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

function DeadlineRow({
  label, info, deadlineKey, incidentId,
}: {
  label: string
  info: DeadlineInfo
  deadlineKey: '4h' | '24h' | '72h' | '30d'
  incidentId: string
}) {
  const { t } = useTranslation()
  const mark = useMarkDeadlineReported(incidentId)
  const { formatDateTime } = useFormatDate()
  const isDone = info.status === 'done'

  function deadlineBadgeLabel(status: DeadlineInfo['status']) {
    switch (status) {
      case 'done': return t('vaktcomply.incidentDetail.deadlineReported')
      case 'green': return t('vaktcomply.incidentDetail.deadlineOpen')
      case 'yellow': return t('vaktcomply.incidentDetail.deadlineSoon')
      case 'red': return t('vaktcomply.incidentDetail.deadlineOverdue')
    }
  }

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
            {formatDateTime(info.deadline)}
            {isDone && info.reported_at && (
              <span className="ml-2 text-green-400">
                {t('vaktcomply.incidentDetail.deadlineReportedAt')} {formatDateTime(info.reported_at)}
              </span>
            )}
            {!isDone && (
              <span className={`ml-2 ${deadlineStatusColor(info.status)}`}>
                {info.hours_left > 0
                  ? t('vaktcomply.incidentDetail.deadlineHoursLeft', { hours: Math.round(info.hours_left) })
                  : t('vaktcomply.incidentDetail.deadlineHoursOverdue', { hours: Math.abs(Math.round(info.hours_left)) })}
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
          onClick={() => { mark.mutate({ deadline: deadlineKey }); }}
          data-testid={`deadline-mark-reported-${deadlineKey}`}
        >
          {t('vaktcomply.incidentDetail.markReported')}
        </Button>
      )}
    </div>
  )
}

export default function IncidentDetailPage() {
  const { t } = useTranslation()
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { formatDate, formatDateTime } = useFormatDate()
  const { data: incident, isLoading, isError } = useIncident(id ?? '')
  const update = useUpdateIncident(id ?? '')
  const { data: incidentReports } = useIncidentReports(id ?? '')
  const generateReport = useGenerateIncidentReport(id ?? '')

  const [form, setForm] = useState<UpdateIncidentInput | null>(null)
  const [rawSystems, setRawSystems] = useState('')
  const [dirty, setDirty] = useState(false)
  const [wizardOpen, setWizardOpen] = useState(false)
  const [classifyWizardOpen, setClassifyWizardOpen] = useState(false)
  const [pdfError, setPdfError] = useState<Error | null>(null)

  const severityLabels: Record<Incident['severity'], string> = {
    low: t('vaktcomply.incidentsPage.severityLow'),
    medium: t('vaktcomply.incidentsPage.severityMedium'),
    high: t('vaktcomply.incidentsPage.severityHigh'),
    critical: t('vaktcomply.incidentsPage.severityCritical'),
  }
  const statusLabels: Record<Incident['status'], string> = {
    open: t('vaktcomply.incidentsPage.statusOpen'),
    investigating: t('vaktcomply.incidentsPage.statusInvestigating'),
    resolved: t('vaktcomply.incidentsPage.statusResolved'),
    closed: t('vaktcomply.incidentsPage.statusClosed'),
  }
  const incidentTypeLabels = {
    general: t('vaktcomply.incidentDetail.typeGeneral'),
    nis2: t('vaktcomply.incidentDetail.typeNIS2'),
    dora: t('vaktcomply.incidentDetail.typeDORA'),
  }
  const obligationLabels = {
    unknown: t('vaktcomply.incidentDetail.obligationUnknown'),
    required: t('vaktcomply.incidentDetail.obligationRequired'),
    not_required: t('vaktcomply.incidentDetail.obligationNotRequired'),
  }

  async function handleDownloadPDF() {
    if (!id) return
    const res = await fetch(`/api/v1/vaktcomply/incidents/${id}/report-pdf`, {
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
    if (incident) trackPage(`/vaktcomply/incidents/${id}`, incident.title, '🚨')
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
    update.mutate(payload, { onSuccess: () => { setDirty(false); } })
  }

  const ds = incident?.deadline_status

  if (isLoading) return (
    <div className="flex items-center justify-center h-48">
      <Spinner size="lg" color="primary" />
    </div>
  )
  if (isError || !incident) return (
    <div className="p-6 text-sm text-red-400">{t('vaktcomply.incidentDetail.notFound')}</div>
  )

  return (
    <div className="flex flex-col h-full">
      <Breadcrumbs items={[
        { label: 'Vakt Comply', href: '/vaktcomply' },
        { label: t('vaktcomply.incidentDetail.breadcrumbIncidents'), href: '/vaktcomply/incidents' },
        { label: incident.title },
      ]} />
      <PageHeader
        title={incident.title}
        description={`${t('vaktcomply.incidentDetail.discoveredAt')} ${formatDate(incident.discovered_at)}`}
        actions={
          <div className="flex gap-2">
            <Button variant="outline" onClick={() => { navigate('/vaktcomply/incidents'); }}>
              <ArrowLeft className="w-4 h-4 mr-1" />
              {t('common.back')}
            </Button>
            {incident?.incident_type === 'dora' && (
              <Button
                variant="outline"
                onClick={() => { void handleDownloadPDF(); }}
                data-testid="download-pdf-button"
              >
                <FileDown className="w-4 h-4 mr-1" />
                {t('vaktcomply.incidentDetail.bafinReportPdf')}
              </Button>
            )}
            {(incident?.incident_type === 'nis2' || incident?.incident_type === 'general') && (
              <>
                <Button
                  variant="outline"
                  onClick={() => { setWizardOpen(true); }}
                  data-testid="assess-reportability-btn"
                >
                  <ShieldAlert className="w-4 h-4 mr-1" />
                  {t('vaktcomply.incidentDetail.checkReportability')}
                </Button>
                <Button
                  variant="outline"
                  onClick={() => { setClassifyWizardOpen(true); }}
                  data-testid="classify-reporting-btn"
                >
                  <ShieldAlert className="w-4 h-4 mr-1" />
                  {t('vaktcomply.incidentDetail.bsiClassification')}
                </Button>
              </>
            )}
            <Button onClick={handleSave} disabled={!dirty || update.isPending}>
              <Save className="w-4 h-4 mr-1" />
              {update.isPending ? t('common.savePending') : t('common.save')}
            </Button>
          </div>
        }
      />

      {form && (
        <div className="flex-1 p-6 grid grid-cols-1 lg:grid-cols-3 gap-6">
          <div className="lg:col-span-2 space-y-4">
            <Card>
              <CardHeader><CardTitle className="text-sm">{t('vaktcomply.incidentDetail.cardIncidentDetails')}</CardTitle></CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-1.5">
                  <Label>{t('vaktcomply.incidentDetail.labelTitle')}</Label>
                  <Input value={form.title} onChange={(e) => { set('title', e.target.value); }} />
                </div>
                <div className="space-y-1.5">
                  <div className="flex items-center justify-between">
                    <Label>{t('vaktcomply.incidentDetail.labelDescription')}</Label>
                    <AISuggestActionsButton
                      summary={form.description}
                      type={incident.incident_type}
                      onAppend={(guide) => { set('description', `${form.description}\n\n--- KI-Sofortmaßnahmen ---\n${guide}`); }}
                    />
                  </div>
                  <Textarea rows={4} value={form.description} onChange={(e) => { set('description', e.target.value); }} />
                </div>
                <div className="space-y-1.5">
                  <Label>{t('vaktcomply.incidentDetail.labelAffectedSystems')}</Label>
                  <Input value={rawSystems} onChange={(e) => { setRawSystems(e.target.value); setDirty(true) }} />
                </div>
              </CardContent>
            </Card>

            {ds && (
              <Card className="border-primary/20">
                <CardHeader>
                  <CardTitle className="text-sm flex items-center gap-2">
                    <AlertTriangle className="w-4 h-4 text-amber-400" />
                    {t('vaktcomply.incidentDetail.cardDeadlines')}
                    <Badge variant="outline" className="text-xs ml-auto">
                      {incident.incident_type === 'dora' ? 'DORA' : 'NIS2'}
                    </Badge>
                  </CardTitle>
                </CardHeader>
                <CardContent className="divide-y divide-border">
                  {ds.has_4h && ds.d_4h && id && (
                    <DeadlineRow label={t('vaktcomply.incidentDetail.deadlineLabel4h')} info={ds.d_4h} deadlineKey="4h" incidentId={id} />
                  )}
                  {ds.has_24h && ds.d_24h && id && (
                    <DeadlineRow label={t('vaktcomply.incidentDetail.deadlineLabel24h')} info={ds.d_24h} deadlineKey="24h" incidentId={id} />
                  )}
                  {ds.has_72h && ds.d_72h && id && (
                    <DeadlineRow label={t('vaktcomply.incidentDetail.deadlineLabel72h')} info={ds.d_72h} deadlineKey="72h" incidentId={id} />
                  )}
                  {ds.has_30d && ds.d_30d && id && (
                    <DeadlineRow label={t('vaktcomply.incidentDetail.deadlineLabel30d')} info={ds.d_30d} deadlineKey="30d" incidentId={id} />
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
                    {t('vaktcomply.incidentDetail.cardReportForms')}
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
                        onClick={() => { generateReport.mutate({ report_type: rt }); }}
                        data-testid={`generate-report-btn-${rt}`}
                      >
                        {t('vaktcomply.incidentDetail.createReport', { type: rt })}
                      </Button>
                    ))}
                  </div>

                  {incidentReports && incidentReports.length > 0 && (
                    <div data-testid="report-history">
                      <p className="text-xs text-muted-foreground mb-2">{t('vaktcomply.incidentDetail.reportHistory')}</p>
                      <div className="space-y-1">
                        {incidentReports.map((r: IncidentReport) => (
                          <div key={r.id} className="flex items-center justify-between text-xs bg-muted/30 rounded px-2 py-1.5">
                            <span className="font-medium">{r.report_type} — {r.authority}</span>
                            <span className="text-muted-foreground">
                              {formatDateTime(r.generated_at)}
                            </span>
                            <a
                              href={`/api/v1/vaktcomply/incident-reports/${r.id}/pdf`}
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

            {(incident.incident_type === 'nis2' || incident.incident_type === 'general') && id && (
              <NIS2StagePanel incidentId={id} />
            )}

            {form.incident_type === 'dora' && (
              <Card className="border-blue-500/30" data-testid="dora-fields-card">
                <CardHeader>
                  <CardTitle className="text-sm flex items-center gap-2">
                    {t('vaktcomply.incidentDetail.doraFields')}
                    {incident.is_major_incident && (
                      <Badge className="text-xs bg-red-500/20 text-red-400 border-red-500/30 ml-auto" data-testid="major-incident-badge">
                        {t('vaktcomply.incidentDetail.doraMajorBadge')}
                      </Badge>
                    )}
                  </CardTitle>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="space-y-1.5">
                    <Label htmlFor="affected-customers">{t('vaktcomply.incidentDetail.labelAffectedCustomers')}</Label>
                    <Input
                      id="affected-customers"
                      type="number"
                      min={0}
                      placeholder={t('vaktcomply.incidentDetail.placeholderAffectedCustomers')}
                      value={form.affected_customers ?? ''}
                      onChange={(e) => { set('affected_customers', e.target.value ? Number(e.target.value) : undefined); }}
                      data-testid="affected-customers-input"
                    />
                  </div>
                  <div className="space-y-1.5">
                    <Label htmlFor="financial-impact">{t('vaktcomply.incidentDetail.labelFinancialImpact')}</Label>
                    <Textarea
                      id="financial-impact"
                      rows={2}
                      placeholder={t('vaktcomply.incidentDetail.placeholderFinancialImpact')}
                      value={form.financial_impact_estimate ?? ''}
                      onChange={(e) => { set('financial_impact_estimate', e.target.value); }}
                      data-testid="financial-impact-textarea"
                    />
                  </div>
                  <div className="flex items-center gap-2">
                    <input
                      id="is-major-incident"
                      type="checkbox"
                      className="w-4 h-4 accent-primary cursor-pointer"
                      checked={form.is_major_incident ?? false}
                      onChange={(e) => { set('is_major_incident', e.target.checked); }}
                      data-testid="is-major-incident-checkbox"
                    />
                    <Label htmlFor="is-major-incident" className="cursor-pointer">
                      {t('vaktcomply.incidentDetail.labelMajorIncident')}
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
                    {t('vaktcomply.incidentDetail.linkedBreach')}
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <a
                    href="/vaktprivacy/breach"
                    className="text-sm text-amber-400 hover:underline"
                  >
                    {t('vaktcomply.incidentDetail.openBreach')}
                  </a>
                </CardContent>
              </Card>
            )}
          </div>

          <div className="space-y-4">
            <Card>
              <CardHeader><CardTitle className="text-sm">{t('vaktcomply.incidentDetail.cardClassification')}</CardTitle></CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-1.5">
                  <Label>{t('vaktcomply.incidentDetail.labelSeverity')}</Label>
                  <Select value={form.severity} onValueChange={(v) => { set('severity', v as Incident['severity']); }}>
                    <SelectTrigger><SelectValue /></SelectTrigger>
                    <SelectContent>
                      {(Object.keys(severityLabels) as Incident['severity'][]).map((k) => (
                        <SelectItem key={k} value={k}>
                          <span className="flex items-center gap-2">
                            <span className={`inline-block w-2 h-2 rounded-full ${SEVERITY_CLASS[k].split(' ')[0]}`} />
                            {severityLabels[k]}
                          </span>
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-1.5">
                  <Label>{t('vaktcomply.incidentDetail.labelStatus')}</Label>
                  <Select value={form.status} onValueChange={(v) => { set('status', v as Incident['status']); }}>
                    <SelectTrigger><SelectValue /></SelectTrigger>
                    <SelectContent>
                      {(Object.keys(statusLabels) as Incident['status'][]).map((k) => (
                        <SelectItem key={k} value={k}>{statusLabels[k]}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="flex gap-2 flex-wrap text-xs">
                  <Badge className={SEVERITY_CLASS[form.severity]}>{severityLabels[form.severity]}</Badge>
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader><CardTitle className="text-sm">{t('vaktcomply.incidentDetail.cardReportingObligation')}</CardTitle></CardHeader>
              <CardContent className="space-y-3">
                <div className="space-y-1.5">
                  <Label>{t('vaktcomply.incidentDetail.labelIncidentType')}</Label>
                  <Select value={form.incident_type ?? 'general'} onValueChange={(v) => { set('incident_type', v as Incident['incident_type']); }}>
                    <SelectTrigger><SelectValue /></SelectTrigger>
                    <SelectContent>
                      {Object.entries(incidentTypeLabels).map(([k, label]) => (
                        <SelectItem key={k} value={k}>{label}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-1.5">
                  <Label>{t('vaktcomply.incidentDetail.labelObligation')}</Label>
                  <Select value={form.reporting_obligation ?? 'unknown'} onValueChange={(v) => { set('reporting_obligation', v as Incident['reporting_obligation']); }}>
                    <SelectTrigger><SelectValue /></SelectTrigger>
                    <SelectContent>
                      {Object.entries(obligationLabels).map(([k, label]) => (
                        <SelectItem key={k} value={k}>{label}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-1.5">
                  <Label>{t('vaktcomply.incidentDetail.labelAuthority')}</Label>
                  <Input
                    placeholder={t('vaktcomply.incidentDetail.placeholderAuthority')}
                    value={form.notification_authority ?? ''}
                    onChange={(e) => { set('notification_authority', e.target.value); }}
                  />
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardContent className="pt-4 space-y-1 text-xs text-muted-foreground">
                <p>{t('vaktcomply.incidentDetail.metaDiscovered')} {formatDateTime(incident.discovered_at)}</p>
                {incident.resolved_at && <p>{t('vaktcomply.incidentDetail.metaResolved')} {formatDateTime(incident.resolved_at)}</p>}
                <p>{t('vaktcomply.incidentDetail.metaCreated')} {formatDate(incident.created_at)}</p>
              </CardContent>
            </Card>
          </div>
        </div>
      )}

      {id && (
        <ReportabilityWizard
          incidentId={id}
          open={wizardOpen}
          onClose={() => { setWizardOpen(false); }}
        />
      )}
      {id && (
        <ClassifyReportingWizard
          incidentId={id}
          open={classifyWizardOpen}
          onClose={() => { setClassifyWizardOpen(false); }}
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

// AISuggestActionsButton calls the AI copilot (POST /vaktcomply/ai/incident-guide)
// and appends the returned numbered checklist to the incident description.
// Disabled while the description is empty — the LLM needs context to work with.
function AISuggestActionsButton({ summary, type, onAppend }: AISuggestActionsProps) {
  const { t } = useTranslation()
  const { data: aiStatus } = useAIStatus()
  const { incidentGuide } = useAICopilot()

  // S121-F3 (P5): hide the AI action entirely when the provider is disabled —
  // the /ai/incident-guide route is not registered and would 404 on click.
  if (!aiStatus?.available) {
    return null
  }

  const handleClick = () => {
    if (!summary.trim()) return
    incidentGuide.mutate(
      { summary, type: type ?? '' },
      {
        onSuccess: (resp) => {
          onAppend(resp.guide)
          toast(t('vaktcomply.incidentDetail.aiActionsAppended'), { variant: 'success' })
        },
        onError: () => {
          toast(t('vaktcomply.incidentDetail.aiUnavailable'), { variant: 'error' })
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
      title={t('vaktcomply.incidentDetail.aiButtonTitle')}
    >
      <Sparkles className="w-3 h-3" aria-hidden="true" />
      {incidentGuide.isPending ? t('vaktcomply.incidentDetail.aiThinking') : t('vaktcomply.incidentDetail.aiActions')}
    </button>
  )
}
