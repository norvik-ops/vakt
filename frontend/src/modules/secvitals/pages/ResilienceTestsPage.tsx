import { useState } from 'react'
import { ShieldAlert, Plus, Pencil, Trash2, Paperclip } from 'lucide-react'
import { PageHeader } from '../../../shared/components/PageHeader'
import { EmptyState } from '../../../shared/components/EmptyState'
import { ProGate } from '../../../shared/components/ProGate'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Card, CardContent } from '../../../components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '../../../components/ui/dialog'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Textarea } from '../../../components/ui/textarea'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '../../../components/ui/select'
import {
  useResilienceTests,
  useCreateResilienceTest,
  useUpdateResilienceTest,
  useDeleteResilienceTest,
  useUploadResilienceTestAttachment,
} from '../hooks/useResilienceTests'
import type { ResilienceTest, CreateResilienceTestInput } from '../types'

// ─── Type helpers ─────────────────────────────────────────────────────────────

const TYPE_LABELS: Record<ResilienceTest['type'], string> = {
  tlpt: 'TLPT',
  pentest: 'Pentest',
  scenario_based: 'Szenariobasiert',
  vulnerability_assessment: 'Vulnerability Assessment',
}

const TYPE_CLASS: Record<ResilienceTest['type'], string> = {
  tlpt: 'bg-red-500/20 text-red-400 border-red-500/30',
  pentest: 'bg-amber-500/20 text-amber-400 border-amber-500/30',
  scenario_based: 'bg-secondary text-secondary-foreground',
  vulnerability_assessment: 'bg-secondary text-secondary-foreground',
}

const REMEDIATION_LABELS: Record<ResilienceTest['remediation_status'], string> = {
  open: 'Offen',
  in_progress: 'In Bearbeitung',
  completed: 'Abgeschlossen',
  accepted: 'Akzeptiert',
}

const REMEDIATION_CLASS: Record<ResilienceTest['remediation_status'], string> = {
  open: 'bg-red-500/20 text-red-400 border-red-500/30',
  in_progress: 'bg-amber-500/20 text-amber-400 border-amber-500/30',
  completed: 'bg-green-500/20 text-green-400 border-green-500/30',
  accepted: 'bg-secondary text-secondary-foreground',
}

// ─── Empty form ───────────────────────────────────────────────────────────────

function emptyForm(): CreateResilienceTestInput {
  return {
    type: '',
    scope: '',
    provider: '',
    test_date: '',
    summary: '',
    remediation_status: 'open',
  }
}

function testToForm(t: ResilienceTest): CreateResilienceTestInput {
  return {
    type: t.type,
    scope: t.scope ?? '',
    provider: t.provider ?? '',
    test_date: t.test_date ? t.test_date.slice(0, 10) : '',
    summary: t.summary ?? '',
    remediation_status: t.remediation_status,
  }
}

// ─── Row component ────────────────────────────────────────────────────────────

function ResilienceTestRow({
  test,
  onEdit,
  onDelete,
}: {
  test: ResilienceTest
  onEdit: () => void
  onDelete: () => void
}) {
  return (
    <Card>
      <CardContent className="pt-5 space-y-2">
        <div className="flex items-start justify-between gap-2">
          <div className="space-y-1 flex-1">
            <div className="flex items-center gap-2 flex-wrap">
              <Badge className={TYPE_CLASS[test.type]}>{TYPE_LABELS[test.type]}</Badge>
              {test.overdue_warning && (
                <Badge className="bg-red-500/20 text-red-400 border-red-500/30 text-xs">
                  Überfällig
                </Badge>
              )}
              <Badge className={REMEDIATION_CLASS[test.remediation_status]}>
                {REMEDIATION_LABELS[test.remediation_status]}
              </Badge>
            </div>
            <p className="text-sm font-medium">
              {new Date(test.test_date).toLocaleDateString('de-DE')}
              {test.provider ? ` · ${test.provider}` : ''}
            </p>
            {test.scope && (
              <p className="text-xs text-muted-foreground">Scope: {test.scope}</p>
            )}
            {test.summary && (
              <p className="text-xs text-muted-foreground line-clamp-2">{test.summary}</p>
            )}
            {test.attachment_url && (
              <p className="text-xs text-muted-foreground flex items-center gap-1">
                <Paperclip className="w-3 h-3" />
                Anhang vorhanden
              </p>
            )}
          </div>
          <div className="flex items-center gap-1 shrink-0">
            <Button variant="ghost" size="icon" className="h-7 w-7" onClick={onEdit}>
              <Pencil className="w-3.5 h-3.5" />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7 text-red-400 hover:text-red-300"
              onClick={onDelete}
            >
              <Trash2 className="w-3.5 h-3.5" />
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function ResilienceTestsPage() {
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editId, setEditId] = useState<string | null>(null)
  const [form, setForm] = useState<CreateResilienceTestInput>(emptyForm())

  const { data, isLoading, isError, error } = useResilienceTests()
  const createTest = useCreateResilienceTest()
  const updateTest = useUpdateResilienceTest(editId ?? '')
  const deleteTest = useDeleteResilienceTest()
  const uploadAttachment = useUploadResilienceTestAttachment(editId ?? '')

  const tests = data?.tests ?? []
  const tlptOverdueWarning = data?.tlpt_overdue_warning ?? false

  function openCreate() {
    setEditId(null)
    setForm(emptyForm())
    setDialogOpen(true)
  }

  function openEdit(t: ResilienceTest) {
    setEditId(t.id)
    setForm(testToForm(t))
    setDialogOpen(true)
  }

  function handleDelete(id: string) {
    if (confirm('Resilience-Test wirklich löschen?')) {
      deleteTest.mutate(id)
    }
  }

  function handleSubmit() {
    const payload = {
      ...form,
      test_date: form.test_date ? new Date(form.test_date).toISOString() : '',
    }
    if (editId) {
      updateTest.mutate(
        { ...payload, remediation_status: payload.remediation_status ?? 'open' },
        { onSuccess: () => setDialogOpen(false) },
      )
    } else {
      createTest.mutate(payload, { onSuccess: () => setDialogOpen(false) })
    }
  }

  function handleFileUpload(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file || !editId) return
    const fd = new FormData()
    fd.append('file', file)
    uploadAttachment.mutate(fd)
  }

  const isPending =
    createTest.isPending || updateTest.isPending || uploadAttachment.isPending

  return (
    <div className="flex flex-col h-full">
      <ProGate error={error}>{null}</ProGate>
      {tlptOverdueWarning && (
        <div
          data-testid="tlpt-overdue-warning"
          className="mx-6 mt-4 p-4 rounded-lg bg-red-500/10 border border-red-500/30 text-red-400 text-sm flex items-start gap-2"
        >
          <ShieldAlert className="w-5 h-5 shrink-0 mt-0.5" />
          <span>
            Kein TLPT in den letzten 3 Jahren durchgeführt. DORA Art. 26 verlangt alle 3
            Jahre einen TLPT.
          </span>
        </div>
      )}

      <PageHeader
        title="Resilience-Tests"
        description="DORA Art. 24-27 — TLPT, Pentests und szenariobasierte Tests dokumentieren."
        actions={
          <Button onClick={openCreate}>
            <Plus className="w-4 h-4 mr-1" />
            Neuer Test
          </Button>
        }
      />

      <div className="flex-1 p-6">
        {isLoading && (
          <div className="flex items-center justify-center h-48">
            <div className="w-6 h-6 border-2 border-primary border-t-transparent rounded-full animate-spin" />
          </div>
        )}
        {isError && (
          <div className="text-sm text-red-400 p-4 bg-red-500/10 rounded-lg">
            Fehler beim Laden der Resilience-Tests.
          </div>
        )}
        {!isLoading && !isError && tests.length === 0 && (
          <EmptyState
            icon={ShieldAlert}
            title="Keine Resilience-Tests erfasst"
            description="Dokumentieren Sie TLPT, Pentests und szenariobasierte Tests für DORA Art. 24-27."
            action={
              <Button onClick={openCreate}>
                <Plus className="w-4 h-4 mr-1" />
                Neuer Test
              </Button>
            }
          />
        )}
        {!isLoading && !isError && tests.length > 0 && (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {tests.map((t) => (
              <ResilienceTestRow
                key={t.id}
                test={t}
                onEdit={() => openEdit(t)}
                onDelete={() => handleDelete(t.id)}
              />
            ))}
          </div>
        )}
      </div>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="max-w-lg max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>{editId ? 'Test bearbeiten' : 'Neuer Test'}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-1.5">
              <Label>Typ *</Label>
              <Select
                value={form.type}
                onValueChange={(v) => setForm((f) => ({ ...f, type: v }))}
              >
                <SelectTrigger>
                  <SelectValue placeholder="Testtyp auswählen …" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="tlpt">TLPT</SelectItem>
                  <SelectItem value="pentest">Pentest</SelectItem>
                  <SelectItem value="scenario_based">Szenariobasiert</SelectItem>
                  <SelectItem value="vulnerability_assessment">
                    Vulnerability Assessment
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1.5">
              <Label>Testdatum *</Label>
              <Input
                type="date"
                value={form.test_date ?? ''}
                onChange={(e) => setForm((f) => ({ ...f, test_date: e.target.value }))}
              />
            </div>

            <div className="space-y-1.5">
              <Label>Scope / Geltungsbereich</Label>
              <Input
                placeholder="z.B. Core-Banking-Systeme"
                value={form.scope ?? ''}
                onChange={(e) => setForm((f) => ({ ...f, scope: e.target.value }))}
              />
            </div>

            <div className="space-y-1.5">
              <Label>Dienstleister / Provider</Label>
              <Input
                placeholder="z.B. CyberProof GmbH"
                value={form.provider ?? ''}
                onChange={(e) => setForm((f) => ({ ...f, provider: e.target.value }))}
              />
            </div>

            <div className="space-y-1.5">
              <Label>Remediationsstatus</Label>
              <Select
                value={form.remediation_status ?? 'open'}
                onValueChange={(v) => setForm((f) => ({ ...f, remediation_status: v }))}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="open">Offen</SelectItem>
                  <SelectItem value="in_progress">In Bearbeitung</SelectItem>
                  <SelectItem value="completed">Abgeschlossen</SelectItem>
                  <SelectItem value="accepted">Akzeptiert</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1.5">
              <Label>Zusammenfassung / Ergebnis</Label>
              <Textarea
                rows={4}
                placeholder="Ergebnisse, Befunde, Empfehlungen …"
                value={form.summary ?? ''}
                onChange={(e) => setForm((f) => ({ ...f, summary: e.target.value }))}
              />
            </div>

            {editId && (
              <div className="space-y-1.5">
                <Label>Anhang hochladen (max. 20 MB)</Label>
                <Input type="file" onChange={handleFileUpload} />
              </div>
            )}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDialogOpen(false)}>
              Abbrechen
            </Button>
            <Button
              onClick={handleSubmit}
              disabled={!form.type || !form.test_date || isPending}
            >
              {isPending ? 'Speichern …' : editId ? 'Speichern' : 'Hinzufügen'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
