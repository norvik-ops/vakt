import { useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, Save, ClipboardCheck, Plus } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Button } from '../../../components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Textarea } from '../../../components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../../../components/ui/select'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../../../components/ui/dialog'
import { useAuditRecord, useUpdateAuditRecord } from '../hooks/useAudits'
import { useCreateCAPA } from '../hooks/useCAPAs'
import type { AuditRecord, UpdateAuditRecordInput } from '../types'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'

const STATUS_LABELS: Record<AuditRecord['status'], string> = {
  planned: 'Geplant', in_progress: 'Laufend', completed: 'Abgeschlossen',
}

export default function AuditDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { formatDate } = useFormatDate()
  const { data: record, isLoading, isError } = useAuditRecord(id ?? '')
  const update = useUpdateAuditRecord(id ?? '')
  const createCAPA = useCreateCAPA()

  const [form, setForm] = useState<UpdateAuditRecordInput | null>(null)
  const [dirty, setDirty] = useState(false)
  const [capaDialogOpen, setCAPADialogOpen] = useState(false)
  const [capaTitle, setCAPATitle] = useState('')

  useEffect(() => {
    if (record && !form) {
      setForm({
        title: record.title,
        scope: record.scope ?? '',
        auditor: record.auditor ?? '',
        audit_date: record.audit_date.slice(0, 10),
        status: record.status,
        findings: record.findings ?? '',
        recommendations: record.recommendations ?? '',
      })
    }
  }, [record, form])

  function set<K extends keyof UpdateAuditRecordInput>(key: K, value: UpdateAuditRecordInput[K]) {
    setForm((f) => f ? { ...f, [key]: value } : f)
    setDirty(true)
  }

  function handleSave() {
    if (!form) return
    update.mutate(form, { onSuccess: () => { setDirty(false); } })
  }

  function handleCreateCAPA() {
    if (!capaTitle.trim() || !id) return
    createCAPA.mutate(
      { source_type: 'audit', source_id: id, title: capaTitle, priority: 'medium' },
      { onSuccess: () => { setCAPATitle(''); setCAPADialogOpen(false) } },
    )
  }

  if (isLoading) return (
    <div className="flex items-center justify-center h-48">
      <Spinner size="lg" color="primary" />
    </div>
  )
  if (isError || !record) return (
    <div className="p-6 text-sm text-red-400">Audit nicht gefunden.</div>
  )

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title={record.title}
        description={record.scope || 'Auditdetails'}
        actions={
          <div className="flex gap-2">
            <Button variant="outline" onClick={() => { navigate('/secvitals/audits'); }}>
              <ArrowLeft className="w-4 h-4 mr-1" />
              Zurück
            </Button>
            <Button variant="outline" onClick={() => { setCAPADialogOpen(true); }}>
              <ClipboardCheck className="w-4 h-4 mr-1" />
              CAPA erstellen
            </Button>
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
              <CardHeader><CardTitle className="text-sm">Auditdaten</CardTitle></CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-1.5">
                  <Label>Titel</Label>
                  <Input value={form.title} onChange={(e) => { set('title', e.target.value); }} />
                </div>
                <div className="space-y-1.5">
                  <Label>Prüfumfang</Label>
                  <Input value={form.scope ?? ''} placeholder="z.B. A.9 Zugangskontrolle" onChange={(e) => { set('scope', e.target.value); }} />
                </div>
                <div className="grid grid-cols-2 gap-3">
                  <div className="space-y-1.5">
                    <Label>Auditor</Label>
                    <Input value={form.auditor ?? ''} onChange={(e) => { set('auditor', e.target.value); }} />
                  </div>
                  <div className="space-y-1.5">
                    <Label>Prüfdatum</Label>
                    <Input type="date" value={form.audit_date} onChange={(e) => { set('audit_date', e.target.value); }} />
                  </div>
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader><CardTitle className="text-sm">Ergebnisse</CardTitle></CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-1.5">
                  <Label>Feststellungen</Label>
                  <Textarea rows={4} value={form.findings ?? ''} placeholder="Abweichungen und Beobachtungen …" onChange={(e) => { set('findings', e.target.value); }} />
                </div>
                <div className="space-y-1.5">
                  <Label>Empfehlungen</Label>
                  <Textarea rows={3} value={form.recommendations ?? ''} placeholder="Empfohlene Maßnahmen …" onChange={(e) => { set('recommendations', e.target.value); }} />
                </div>
              </CardContent>
            </Card>
          </div>

          <div className="space-y-4">
            <Card>
              <CardHeader><CardTitle className="text-sm">Status</CardTitle></CardHeader>
              <CardContent>
                <Select value={form.status} onValueChange={(v) => { set('status', v as AuditRecord['status']); }}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    {(Object.keys(STATUS_LABELS) as AuditRecord['status'][]).map((k) => (
                      <SelectItem key={k} value={k}>{STATUS_LABELS[k]}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </CardContent>
            </Card>

            <Card>
              <CardContent className="pt-4 space-y-1 text-xs text-muted-foreground">
                <p>Erstellt: {formatDate(record.created_at)}</p>
                <p>Geändert: {formatDate(record.updated_at)}</p>
              </CardContent>
            </Card>
          </div>
        </div>
      )}

      {/* CAPA quick-create dialog */}
      <Dialog open={capaDialogOpen} onOpenChange={(v) => { if (!v) { setCAPADialogOpen(false); } }}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>CAPA aus Audit erstellen</DialogTitle>
          </DialogHeader>
          <div className="space-y-3 py-2">
            <div className="space-y-1.5">
              <Label>Titel der Korrekturmaßnahme *</Label>
              <Input value={capaTitle} onChange={(e) => { setCAPATitle(e.target.value); }} placeholder="Kurzbeschreibung der Maßnahme" />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setCAPADialogOpen(false); }}>Abbrechen</Button>
            <Button onClick={handleCreateCAPA} disabled={!capaTitle.trim() || createCAPA.isPending}>
              <Plus className="w-4 h-4 mr-1" />
              {createCAPA.isPending ? 'Erstellen …' : 'CAPA erstellen'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
