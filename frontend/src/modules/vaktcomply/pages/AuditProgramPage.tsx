import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { ClipboardList, Plus, Pencil, CheckCircle2, AlertTriangle, Download, ChevronDown } from 'lucide-react'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../../../components/ui/dialog'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Textarea } from '../../../components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../../../components/ui/select'
import { apiFetch } from '../../../api/client'
import { SkeletonTable } from '../../../shared/components/SkeletonLoaders'
import { EmptyState } from '../../../shared/components/EmptyState'

interface AuditPlan {
  id: string
  title: string
  year: number
  scope: string
  objectives: string
  status: string
  created_at: string
}

interface AuditProgramAudit {
  id: string
  plan_id: string
  title: string
  audit_type: string
  scheduled_date: string
  completed_date: string
  lead_auditor: string
  status: string
  summary: string
  overall_rating: string
  created_at: string
}

interface AuditFinding {
  id: string
  audit_id: string
  title: string
  description: string
  severity: string
  status: string
  capa_id: string
  created_at: string
}

interface AuditProgramSummary {
  total_plans: number
  total_audits: number
  completed_audits: number
  open_findings: number
  major_nc_count: number
}

const AUDIT_TYPE_LABELS: Record<string, string> = {
  internal: 'Intern',
  external: 'Extern',
  certification: 'Zertifizierung',
  supplier: 'Lieferantenaudit',
  follow_up: 'Nachaudit',
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
  opportunity: 'bg-blue-100 text-blue-700',
}

const SEVERITY_LABELS: Record<string, string> = {
  major_nc: 'Hauptabweichung',
  minor_nc: 'Nebenabweichung',
  observation: 'Beobachtung',
  opportunity: 'Verbesserungsmöglichkeit',
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
  return useMutation({
    mutationFn: (input: Partial<AuditProgramAudit>) =>
      apiFetch<AuditProgramAudit>('/vaktcomply/audit-program', { method: 'POST', body: JSON.stringify(input) }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['vaktcomply', 'audit-program'] })
      void qc.invalidateQueries({ queryKey: ['vaktcomply', 'audit-program-summary'] })
    },
  })
}

function useCompleteAudit() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: { summary?: string; overall_rating?: string; completed_date?: string } }) =>
      apiFetch<AuditProgramAudit>(`/vaktcomply/audit-program/${id}/complete`, { method: 'POST', body: JSON.stringify(input) }),
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

interface AuditForm {
  title: string
  audit_type: string
  scheduled_date: string
  lead_auditor: string
  plan_id: string
}

interface CompleteForm {
  summary: string
  overall_rating: string
  completed_date: string
}

interface FindingForm {
  title: string
  description: string
  severity: string
}

export default function AuditProgramPage() {
  const { data: summary } = useAuditSummary()
  const { data: audits = [], isLoading } = useAuditProgram()
  const createMut = useCreateAudit()
  const completeMut = useCompleteAudit()
  const createFindingMut = useCreateFinding()

  const [createOpen, setCreateOpen] = useState(false)
  const [auditForm, setAuditForm] = useState<AuditForm>({ title: '', audit_type: 'internal', scheduled_date: '', lead_auditor: '', plan_id: '' })
  const [completeTarget, setCompleteTarget] = useState<AuditProgramAudit | null>(null)
  const [completeForm, setCompleteForm] = useState<CompleteForm>({ summary: '', overall_rating: 'satisfactory', completed_date: new Date().toISOString().slice(0, 10) })
  const [findingTarget, setFindingTarget] = useState<AuditProgramAudit | null>(null)
  const [findingForm, setFindingForm] = useState<FindingForm>({ title: '', description: '', severity: 'observation' })
  const [expandedAudit, setExpandedAudit] = useState<string | null>(null)

  const { data: findings = [] } = useAuditFindings(expandedAudit)

  function handleCreateAudit() {
    createMut.mutate(auditForm as Partial<AuditProgramAudit>, { onSuccess: () => { setCreateOpen(false); } })
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
          <h1 className="text-2xl font-bold">Audit-Programm</h1>
          <p className="text-gray-500 text-sm mt-1">ISO 27001 Klausel 9.2 — Interne Audits planen und dokumentieren</p>
        </div>
        <Button size="sm" onClick={() => { setCreateOpen(true); }}>
          <Plus className="h-4 w-4 mr-1.5" />
          Audit anlegen
        </Button>
      </div>

      {/* Summary */}
      {summary && (
        <div className="grid grid-cols-5 gap-3">
          {[
            { label: 'Audits gesamt', value: summary.total_audits },
            { label: 'Abgeschlossen', value: summary.completed_audits, cls: 'text-green-700' },
            { label: 'Offene Findings', value: summary.open_findings, cls: summary.open_findings > 0 ? 'text-amber-700' : '' },
            { label: 'Hauptabweichungen', value: summary.major_nc_count, cls: summary.major_nc_count > 0 ? 'text-red-600' : '' },
            { label: 'Planperioden', value: summary.total_plans },
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
          title="Keine Audits"
          description="Planen Sie interne Audits gemäß ISO 27001 Klausel 9.2."
          action={<Button onClick={() => { setCreateOpen(true); }}><Plus className="h-4 w-4 mr-1.5" />Audit anlegen</Button>}
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
                        {audit.lead_auditor && ` · ${audit.lead_auditor}`}
                        {audit.scheduled_date && ` · ${audit.scheduled_date}`}
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
                          Finding
                        </Button>
                        <Button size="sm" variant="outline" className="h-7 text-xs" onClick={() => {
                          setCompleteTarget(audit)
                          setCompleteForm({ summary: '', overall_rating: 'satisfactory', completed_date: new Date().toISOString().slice(0, 10) })
                        }}>
                          <CheckCircle2 className="h-3.5 w-3.5 mr-1" />
                          Abschließen
                        </Button>
                      </>
                    )}
                    {audit.status === 'completed' && (
                      <Button size="sm" variant="ghost" className="h-7 text-xs" onClick={() => { handleExportReport(audit.id); }}>
                        <Download className="h-3.5 w-3.5 mr-1" />
                        Bericht
                      </Button>
                    )}
                  </div>
                </div>
              </CardHeader>
              {expandedAudit === audit.id && (
                <CardContent className="pt-0 px-4 pb-4">
                  {audit.summary && (
                    <p className="text-sm text-gray-600 mb-3">{audit.summary}</p>
                  )}
                  {findings.length === 0 ? (
                    <p className="text-xs text-gray-400 italic">Keine Findings</p>
                  ) : (
                    <div className="space-y-2">
                      <p className="text-xs font-medium text-gray-500">Findings ({findings.length})</p>
                      {findings.map((f) => (
                        <div key={f.id} className="flex items-start gap-2 p-2 bg-gray-50 rounded text-sm">
                          <span className={`text-xs px-1.5 py-0.5 rounded shrink-0 ${SEVERITY_COLORS[f.severity] ?? 'bg-gray-100'}`}>
                            {SEVERITY_LABELS[f.severity] ?? f.severity}
                          </span>
                          <div>
                            <p className="font-medium text-xs">{f.title}</p>
                            {f.capa_id && <p className="text-xs text-blue-500 mt-0.5">CAPA angelegt</p>}
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
          <DialogHeader><DialogTitle>Audit anlegen</DialogTitle></DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-1.5">
              <Label>Titel *</Label>
              <Input placeholder="z.B. Internes ISO 27001 Audit 2026" value={auditForm.title} onChange={(e) => { setAuditForm(f => ({ ...f, title: e.target.value })); }} />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label>Audit-Typ</Label>
                <Select value={auditForm.audit_type} onValueChange={(v) => { setAuditForm(f => ({ ...f, audit_type: v })); }}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    {Object.entries(AUDIT_TYPE_LABELS).map(([v, l]) => <SelectItem key={v} value={v}>{l}</SelectItem>)}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <Label>Geplantes Datum</Label>
                <Input type="date" value={auditForm.scheduled_date} onChange={(e) => { setAuditForm(f => ({ ...f, scheduled_date: e.target.value })); }} />
              </div>
            </div>
            <div className="space-y-1.5">
              <Label>Leitender Auditor</Label>
              <Input placeholder="Name" value={auditForm.lead_auditor} onChange={(e) => { setAuditForm(f => ({ ...f, lead_auditor: e.target.value })); }} />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setCreateOpen(false); }}>Abbrechen</Button>
            <Button onClick={handleCreateAudit} disabled={!auditForm.title.trim() || createMut.isPending}>
              {createMut.isPending ? 'Speichern…' : 'Anlegen'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Complete Audit Dialog */}
      <Dialog open={completeTarget !== null} onOpenChange={(open) => { if (!open) setCompleteTarget(null); }}>
        <DialogContent className="max-w-md">
          <DialogHeader><DialogTitle>Audit abschließen</DialogTitle></DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-1.5">
              <Label>Abschlussdatum</Label>
              <Input type="date" value={completeForm.completed_date} onChange={(e) => { setCompleteForm(f => ({ ...f, completed_date: e.target.value })); }} />
            </div>
            <div className="space-y-1.5">
              <Label>Gesamtbewertung</Label>
              <Select value={completeForm.overall_rating} onValueChange={(v) => { setCompleteForm(f => ({ ...f, overall_rating: v })); }}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="satisfactory">Zufriedenstellend</SelectItem>
                  <SelectItem value="minor_issues">Kleinere Abweichungen</SelectItem>
                  <SelectItem value="major_issues">Wesentliche Abweichungen</SelectItem>
                  <SelectItem value="critical">Kritisch</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1.5">
              <Label>Zusammenfassung</Label>
              <Textarea rows={3} placeholder="Kurze Zusammenfassung der Audit-Ergebnisse …" value={completeForm.summary} onChange={(e) => { setCompleteForm(f => ({ ...f, summary: e.target.value })); }} />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setCompleteTarget(null); }}>Abbrechen</Button>
            <Button onClick={handleComplete} disabled={completeMut.isPending}>
              {completeMut.isPending ? 'Speichern…' : 'Abschließen'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Create Finding Dialog */}
      <Dialog open={findingTarget !== null} onOpenChange={(open) => { if (!open) setFindingTarget(null); }}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>Finding hinzufügen</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="p-3 rounded-lg bg-amber-50 text-amber-700 text-xs">
              Hauptabweichungen (Major NC) und Nebenabweichungen (Minor NC) lösen automatisch eine CAPA aus.
            </div>
            <div className="space-y-1.5">
              <Label>Titel *</Label>
              <Input placeholder="Kurze Beschreibung des Findings" value={findingForm.title} onChange={(e) => { setFindingForm(f => ({ ...f, title: e.target.value })); }} />
            </div>
            <div className="space-y-1.5">
              <Label>Schweregrad</Label>
              <Select value={findingForm.severity} onValueChange={(v) => { setFindingForm(f => ({ ...f, severity: v })); }}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  {Object.entries(SEVERITY_LABELS).map(([v, l]) => <SelectItem key={v} value={v}>{l}</SelectItem>)}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1.5">
              <Label>Beschreibung</Label>
              <Textarea rows={3} placeholder="Details zum Finding …" value={findingForm.description} onChange={(e) => { setFindingForm(f => ({ ...f, description: e.target.value })); }} />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setFindingTarget(null); }}>Abbrechen</Button>
            <Button onClick={handleCreateFinding} disabled={!findingForm.title.trim() || createFindingMut.isPending}>
              {createFindingMut.isPending ? 'Speichern…' : 'Finding anlegen'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
