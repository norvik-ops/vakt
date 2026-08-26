import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { ClipboardList, Plus, CheckCircle2, AlertTriangle, Download, ChevronDown } from 'lucide-react'
import { Button } from '../../../components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../../../components/ui/dialog'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Textarea } from '../../../components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../../../components/ui/select'
import { useTranslation } from 'react-i18next'
import { apiFetch } from '../../../api/client'
import { SkeletonTable } from '../../../shared/components/SkeletonLoaders'
import { TermTooltip } from '../../../shared/components/TermTooltip'
import { EmptyState } from '../../../shared/components/EmptyState'

// K5-04/05/06: mirrors of audit.AuditProgramAudit / audit.AuditFinding /
// audit.AuditProgramSummary (backend/internal/modules/vaktcomply/audit/models.go).
// The previous shapes were invented end to end — `scheduled_date`,
// `lead_auditor`, `overall_rating`, `total_audits`, `major_nc_count` exist
// nowhere in the backend, which left four of five tiles blank and made every
// create and every complete fail with 422. `python3 scripts/check_fe_be_fields.py`
// (G15) reports the same drift by name when it is run; it is not yet wired into
// any workflow or make target, so today the guard is this file's contract test.
interface AuditProgramAudit {
  id: string
  org_id: string
  audit_plan_id?: string | null
  title: string
  audit_type: string
  scope: string
  methodology: string
  planned_date: string
  actual_date?: string | null
  lead_auditor_id?: string | null
  auditor_ids: string[]
  supplier_id?: string | null
  status: string
  audit_report?: string
  findings_count: number
  created_at: string
  updated_at: string
}

interface AuditFinding {
  id: string
  org_id: string
  audit_id: string
  title: string
  description: string
  severity: string
  affected_control_id?: string | null
  capa_id?: string | null
  created_at: string
}

interface AuditProgramSummary {
  audits_planned_this_year: number
  audits_completed: number
  open_findings: number
  overdue_capas_from_audits: number
}

const STATUS_COLORS: Record<string, string> = {
  planned: 'bg-blue-100 text-blue-800',
  in_progress: 'bg-amber-100 text-amber-800',
  completed: 'bg-green-100 text-green-800',
  cancelled: 'bg-gray-100 text-gray-500',
  open: 'bg-yellow-100 text-yellow-800',
  closed: 'bg-green-100 text-green-800',
  in_review: 'bg-purple-100 text-purple-800',
}

const SEVERITY_COLORS: Record<string, string> = {
  major_nc: 'bg-red-100 text-red-700',
  minor_nc: 'bg-orange-100 text-orange-700',
  observation: 'bg-yellow-100 text-yellow-700',
  ofi: 'bg-blue-100 text-blue-700',
}

function useAuditSummary() {
  return useQuery<AuditProgramSummary>({
    queryKey: ['vaktcomply', 'audit-program-summary'],
    queryFn: () => apiFetch<AuditProgramSummary>('/vaktcomply/audit-program/summary'),
    staleTime: 60 * 1000,
  })
}

function useAuditProgram() {
  return useQuery<AuditProgramAudit[]>({
    queryKey: ['vaktcomply', 'audit-program'],
    queryFn: () => apiFetch<AuditProgramAudit[]>('/vaktcomply/audit-program'),
    staleTime: 2 * 60 * 1000,
  })
}

function useAuditFindings(auditId: string | null) {
  return useQuery<AuditFinding[]>({
    queryKey: ['vaktcomply', 'audit-findings', auditId],
    queryFn: () => apiFetch<AuditFinding[]>(`/vaktcomply/audit-program/${auditId}/findings`),
    enabled: !!auditId,
    staleTime: 60 * 1000,
  })
}

function useCreateAudit() {
  const qc = useQueryClient()
  // The request type is spelled out in the generics on purpose: an apiFetch /
  // useQuery / useMutation type argument is where check_fe_be_fields.py looks for
  // a wire type. Hidden in the mutationFn's parameter annotation it is invisible.
  return useMutation<AuditProgramAudit, Error, CreateAuditProgramAuditInput>({
    mutationFn: (input) =>
      apiFetch<AuditProgramAudit>('/vaktcomply/audit-program', { method: 'POST', body: JSON.stringify(input) }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['vaktcomply', 'audit-program'] })
      void qc.invalidateQueries({ queryKey: ['vaktcomply', 'audit-program-summary'] })
    },
  })
}

function useCompleteAudit() {
  const qc = useQueryClient()
  return useMutation<AuditProgramAudit, Error, { id: string; input: CompleteAuditInput }>({
    mutationFn: ({ id, input }) =>
      // S121-C5 (C1): backend registers this as PATCH; POST returned 404.
      apiFetch<AuditProgramAudit>(`/vaktcomply/audit-program/${id}/complete`, { method: 'PATCH', body: JSON.stringify(input) }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['vaktcomply', 'audit-program'] })
      void qc.invalidateQueries({ queryKey: ['vaktcomply', 'audit-program-summary'] })
    },
  })
}

function useCreateFinding() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ auditId, input }: { auditId: string; input: Partial<AuditFinding> }) =>
      apiFetch<AuditFinding>(`/vaktcomply/audit-program/${auditId}/findings`, { method: 'POST', body: JSON.stringify(input) }),
    onSuccess: (_data, vars) => {
      void qc.invalidateQueries({ queryKey: ['vaktcomply', 'audit-findings', vars.auditId] })
      void qc.invalidateQueries({ queryKey: ['vaktcomply', 'audit-program-summary'] })
    },
  })
}

// Named after the Go struct it mirrors, not after the dialog it feeds: that is
// what lets scripts/check_fe_be_fields.py pair the two and check the fields.
// audit.CreateAuditProgramAuditInput. `scope` and `planned_date` are
// `validate:"required"` — the form used to send neither, so POST was a
// guaranteed 422. `methodology` is deliberately omitted: it is optional and the
// repository substitutes 'combined' for the empty string.
interface CreateAuditProgramAuditInput {
  title: string
  audit_type: string
  scope: string
  planned_date: string
}

// audit.CompleteAuditInput: audit_report (min 10) + actual_date, both
// required. There is no overall_rating column in ck_audit_program_audits, so the
// rating select that used to sit in this dialog was discarded on every submit.
interface CompleteAuditInput {
  audit_report: string
  actual_date: string
}

interface FindingForm {
  title: string
  description: string
  severity: string
}

export default function AuditProgramPage() {
  const { t } = useTranslation()
  const { data: summary } = useAuditSummary()
  // S131-D3 (D18-02): `?? []` (not the `= []` destructuring default) so a null
  // response can never reach `audits.length`. The backend now returns [] at the
  // root; this is defense-in-depth against the react-query null-vs-undefined trap.
  const { data: auditsData, isLoading } = useAuditProgram()
  const audits = auditsData ?? []
  const createMut = useCreateAudit()
  const completeMut = useCompleteAudit()
  const createFindingMut = useCreateFinding()

  const [createOpen, setCreateOpen] = useState(false)
  const [auditForm, setAuditForm] = useState<CreateAuditProgramAuditInput>({ title: '', audit_type: 'isms_internal', scope: '', planned_date: '' })
  const [completeTarget, setCompleteTarget] = useState<AuditProgramAudit | null>(null)
  const [completeForm, setCompleteForm] = useState<CompleteAuditInput>({ audit_report: '', actual_date: new Date().toISOString().slice(0, 10) })
  const [findingTarget, setFindingTarget] = useState<AuditProgramAudit | null>(null)
  const [findingForm, setFindingForm] = useState<FindingForm>({ title: '', description: '', severity: 'observation' })
  const [expandedAudit, setExpandedAudit] = useState<string | null>(null)

  const { data: findings = [] } = useAuditFindings(expandedAudit)

  // Exactly the four values ck_audit_program_audits.audit_type allows (and that
  // CreateAuditProgramAuditInput's `oneof` accepts). The five labels that used to
  // be offered here — internal/external/certification/supplier/follow_up — were
  // all outside the oneof, so no selection could ever be created.
  const AUDIT_TYPE_LABELS: Record<string, string> = {
    isms_internal: t('vaktcomply.auditProgram.typeInternal'),
    compliance_check: t('vaktcomply.auditProgram.typeComplianceCheck'),
    supplier_audit: t('vaktcomply.auditProgram.typeSupplier'),
    process_audit: t('vaktcomply.auditProgram.typeProcessAudit'),
  }

  // `ofi` (opportunity for improvement), not `opportunity` —
  // CreateAuditFindingInput validates `oneof=major_nc minor_nc observation ofi`
  // and the DDL CHECK matches.
  const SEVERITY_LABELS: Record<string, string> = {
    major_nc: t('vaktcomply.auditProgram.severityMajorNC'),
    minor_nc: t('vaktcomply.auditProgram.severityMinorNC'),
    observation: t('vaktcomply.auditProgram.severityObservation'),
    ofi: t('vaktcomply.auditProgram.severityOpportunity'),
  }

  function handleCreateAudit() {
    createMut.mutate(auditForm, { onSuccess: () => { setCreateOpen(false); } })
  }

  function handleComplete() {
    if (!completeTarget) return
    completeMut.mutate({ id: completeTarget.id, input: completeForm }, { onSuccess: () => { setCompleteTarget(null); } })
  }

  function handleCreateFinding() {
    if (!findingTarget) return
    createFindingMut.mutate({ auditId: findingTarget.id, input: findingForm }, { onSuccess: () => { setFindingTarget(null); } })
  }

  function handleExportReport(auditId: string) {
    const a = document.createElement('a')
    a.href = `/api/v1/vaktcomply/audit-program/${auditId}/export`
    a.download = `audit-report-${auditId.slice(0, 8)}.pdf`
    document.body.appendChild(a)
    a.click()
    a.remove()
  }

  if (isLoading) return <div className="p-8"><SkeletonTable rows={5} cols={5} /></div>

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold">{t('vaktcomply.auditProgram.title')}</h1>
          <p className="text-gray-500 text-sm mt-1"><TermTooltip term="ISO 27001" glossaryKey="ISO27001">ISO 27001</TermTooltip> {t('vaktcomply.auditProgram.description')}</p>
        </div>
        <Button size="sm" onClick={() => { setCreateOpen(true); }}>
          <Plus className="h-4 w-4 mr-1.5" />
          {t('vaktcomply.auditProgram.createBtn')}
        </Button>
      </div>

      {/* Summary — four tiles, one per field audit.AuditProgramSummary actually
          emits. There is no major-NC and no plan-period count in that struct; the
          two tiles that used to claim them rendered `undefined`, and the red
          major-NC warning could never fire. */}
      {summary && (
        <div className="grid grid-cols-4 gap-3">
          {[
            { label: t('vaktcomply.auditProgram.summaryPlannedThisYear'), value: summary.audits_planned_this_year },
            { label: t('vaktcomply.auditProgram.summaryCompleted'), value: summary.audits_completed, cls: 'text-green-700' },
            { label: t('vaktcomply.auditProgram.summaryOpenFindings'), value: summary.open_findings, cls: summary.open_findings > 0 ? 'text-amber-700' : '' },
            { label: t('vaktcomply.auditProgram.summaryOverdueCapas'), value: summary.overdue_capas_from_audits, cls: summary.overdue_capas_from_audits > 0 ? 'text-red-600' : '' },
          ].map(({ label, value, cls = '' }) => (
            <div key={label} className="bg-white border rounded-lg p-3 text-center">
              <div className={`text-xl font-bold ${cls}`}>{value}</div>
              <div className="text-xs text-gray-500 mt-0.5">{label}</div>
            </div>
          ))}
        </div>
      )}

      {audits.length === 0 ? (
        <EmptyState
          icon={ClipboardList}
          title={t('vaktcomply.auditProgram.emptyTitle')}
          description={t('vaktcomply.auditProgram.emptyDesc')}
          action={<Button onClick={() => { setCreateOpen(true); }}><Plus className="h-4 w-4 mr-1.5" />{t('vaktcomply.auditProgram.createBtn')}</Button>}
        />
      ) : (
        <div className="space-y-3">
          {audits.map((audit) => (
            <Card key={audit.id}>
              <CardHeader className="py-3 px-4">
                <div className="flex items-center justify-between gap-3">
                  <div className="flex items-center gap-3 min-w-0">
                    <button
                      onClick={() => { setExpandedAudit(expandedAudit === audit.id ? null : audit.id); }}
                      className="p-1 hover:bg-gray-100 rounded"
                    >
                      <ChevronDown className={`h-4 w-4 transition-transform ${expandedAudit === audit.id ? 'rotate-180' : ''}`} />
                    </button>
                    <div className="min-w-0">
                      <CardTitle className="text-sm font-semibold truncate">{audit.title}</CardTitle>
                      <p className="text-xs text-gray-500 mt-0.5">
                        {AUDIT_TYPE_LABELS[audit.audit_type] ?? audit.audit_type}
                        {audit.planned_date && ` · ${audit.planned_date}`}
                      </p>
                    </div>
                  </div>
                  <div className="flex items-center gap-2 shrink-0">
                    <span className={`text-xs px-2 py-0.5 rounded-full font-medium ${STATUS_COLORS[audit.status] ?? 'bg-gray-100 text-gray-700'}`}>
                      {audit.status}
                    </span>
                    {audit.status !== 'completed' && audit.status !== 'cancelled' && (
                      <>
                        <Button size="sm" variant="outline" className="h-7 text-xs" onClick={() => { setFindingTarget(audit); }}>
                          <AlertTriangle className="h-3.5 w-3.5 mr-1" />
                          {t('vaktcomply.auditProgram.findingBtn')}
                        </Button>
                        <Button size="sm" variant="outline" className="h-7 text-xs" onClick={() => {
                          setCompleteTarget(audit)
                          setCompleteForm({ audit_report: '', actual_date: new Date().toISOString().slice(0, 10) })
                        }}>
                          <CheckCircle2 className="h-3.5 w-3.5 mr-1" />
                          {t('vaktcomply.auditProgram.completeBtn')}
                        </Button>
                      </>
                    )}
                    {audit.status === 'completed' && (
                      <Button size="sm" variant="ghost" className="h-7 text-xs" onClick={() => { handleExportReport(audit.id); }}>
                        <Download className="h-3.5 w-3.5 mr-1" />
                        {t('vaktcomply.auditProgram.reportBtn')}
                      </Button>
                    )}
                  </div>
                </div>
              </CardHeader>
              {expandedAudit === audit.id && (
                <CardContent className="pt-0 px-4 pb-4">
                  {audit.audit_report && (
                    <p className="text-sm text-gray-600 mb-3">{audit.audit_report}</p>
                  )}
                  {findings.length === 0 ? (
                    <p className="text-xs text-gray-400 italic">{t('vaktcomply.auditProgram.noFindings')}</p>
                  ) : (
                    <div className="space-y-2">
                      <p className="text-xs font-medium text-gray-500">{t('vaktcomply.auditProgram.findingsCount')} ({findings.length})</p>
                      {findings.map((f) => (
                        <div key={f.id} className="flex items-start gap-2 p-2 bg-gray-50 rounded text-sm">
                          <span className={`text-xs px-1.5 py-0.5 rounded shrink-0 ${SEVERITY_COLORS[f.severity] ?? 'bg-gray-100'}`}>
                            {SEVERITY_LABELS[f.severity] ?? f.severity}
                          </span>
                          <div>
                            <p className="font-medium text-xs">{f.title}</p>
                            {f.capa_id && <p className="text-xs text-blue-500 mt-0.5">{t('vaktcomply.auditProgram.capaCreated')}</p>}
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </CardContent>
              )}
            </Card>
          ))}
        </div>
      )}

      {/* Create Audit Dialog */}
      <Dialog open={createOpen} onOpenChange={(open) => { if (!open) setCreateOpen(false); }}>
        <DialogContent className="max-w-md">
          <DialogHeader><DialogTitle>{t('vaktcomply.auditProgram.createDialogTitle')}</DialogTitle></DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-1.5">
              <Label>{t('vaktcomply.auditProgram.labelTitle')}</Label>
              <Input placeholder={t('vaktcomply.auditProgram.placeholderTitle')} value={auditForm.title} onChange={(e) => { setAuditForm(f => ({ ...f, title: e.target.value })); }} />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label>{t('vaktcomply.auditProgram.labelAuditType')}</Label>
                <Select value={auditForm.audit_type} onValueChange={(v) => { setAuditForm(f => ({ ...f, audit_type: v })); }}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    {Object.entries(AUDIT_TYPE_LABELS).map(([v, l]) => <SelectItem key={v} value={v}>{l}</SelectItem>)}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <Label>{t('vaktcomply.auditProgram.labelScheduledDate')}</Label>
                <Input type="date" value={auditForm.planned_date} onChange={(e) => { setAuditForm(f => ({ ...f, planned_date: e.target.value })); }} />
              </div>
            </div>
            {/* `scope` is validate:"required,max=5000" — the dialog never had this
                field, which is why every create returned 422. */}
            <div className="space-y-1.5">
              <Label>{t('vaktcomply.auditProgram.labelScope')}</Label>
              <Textarea rows={3} placeholder={t('vaktcomply.auditProgram.placeholderScope')} value={auditForm.scope} onChange={(e) => { setAuditForm(f => ({ ...f, scope: e.target.value })); }} />
            </div>
            {/* The former free-text "lead auditor" input had no backend home:
                lead_auditor_id is a UUID FK to users(id), not a name. Dropped
                rather than posting a name into a uuid column. */}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setCreateOpen(false); }}>{t('common.cancel')}</Button>
            <Button
              onClick={handleCreateAudit}
              disabled={!auditForm.title.trim() || !auditForm.scope.trim() || !auditForm.planned_date || createMut.isPending}
            >
              {createMut.isPending ? t('vaktcomply.auditProgram.savingBtn') : t('vaktcomply.auditProgram.createSubmitBtn')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Complete Audit Dialog */}
      <Dialog open={completeTarget !== null} onOpenChange={(open) => { if (!open) setCompleteTarget(null); }}>
        <DialogContent className="max-w-md">
          <DialogHeader><DialogTitle>{t('vaktcomply.auditProgram.completeDialogTitle')}</DialogTitle></DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-1.5">
              <Label>{t('vaktcomply.auditProgram.labelCompletedDate')}</Label>
              <Input type="date" value={completeForm.actual_date} onChange={(e) => { setCompleteForm(f => ({ ...f, actual_date: e.target.value })); }} />
            </div>
            {/* CompleteAuditInput has no rating field — the select that used to be
                here was silently dropped. audit_report is validate:"min=10". */}
            <div className="space-y-1.5">
              <Label>{t('vaktcomply.auditProgram.labelAuditReport')}</Label>
              <Textarea rows={4} placeholder={t('vaktcomply.auditProgram.placeholderAuditReport')} value={completeForm.audit_report} onChange={(e) => { setCompleteForm(f => ({ ...f, audit_report: e.target.value })); }} />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setCompleteTarget(null); }}>{t('common.cancel')}</Button>
            <Button
              onClick={handleComplete}
              disabled={completeForm.audit_report.trim().length < 10 || !completeForm.actual_date || completeMut.isPending}
            >
              {completeMut.isPending ? t('vaktcomply.auditProgram.savingBtn') : t('vaktcomply.auditProgram.completeSubmitBtn')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Create Finding Dialog */}
      <Dialog open={findingTarget !== null} onOpenChange={(open) => { if (!open) setFindingTarget(null); }}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{t('vaktcomply.auditProgram.findingDialogTitle')}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="p-3 rounded-lg bg-amber-50 text-amber-700 text-xs">
              {t('vaktcomply.auditProgram.findingHint')}
            </div>
            <div className="space-y-1.5">
              <Label>{t('vaktcomply.auditProgram.labelFindingTitle')}</Label>
              <Input placeholder={t('vaktcomply.auditProgram.placeholderFindingTitle')} value={findingForm.title} onChange={(e) => { setFindingForm(f => ({ ...f, title: e.target.value })); }} />
            </div>
            <div className="space-y-1.5">
              <Label>{t('vaktcomply.auditProgram.labelSeverity')}</Label>
              <Select value={findingForm.severity} onValueChange={(v) => { setFindingForm(f => ({ ...f, severity: v })); }}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  {Object.entries(SEVERITY_LABELS).map(([v, l]) => <SelectItem key={v} value={v}>{l}</SelectItem>)}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1.5">
              <Label>{t('vaktcomply.auditProgram.labelFindingDesc')}</Label>
              <Textarea rows={3} placeholder={t('vaktcomply.auditProgram.placeholderFindingDesc')} value={findingForm.description} onChange={(e) => { setFindingForm(f => ({ ...f, description: e.target.value })); }} />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setFindingTarget(null); }}>{t('common.cancel')}</Button>
            <Button onClick={handleCreateFinding} disabled={!findingForm.title.trim() || createFindingMut.isPending}>
              {createFindingMut.isPending ? t('vaktcomply.auditProgram.savingBtn') : t('vaktcomply.auditProgram.findingSubmitBtn')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
