import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Trash2, Plus, CheckCircle2, AlertTriangle, BookOpen } from 'lucide-react'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Card, CardContent } from '../../../components/ui/card'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../../../components/ui/dialog'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Textarea } from '../../../components/ui/textarea'
import { apiFetch } from '../../../api/client'
import { EmptyState } from '../../../shared/components/EmptyState'
import { SkeletonTable } from '../../../shared/components/SkeletonLoaders'

interface DeletionReminder {
  id: string
  org_id: string
  processing_activity_id?: string
  description: string
  data_category: string
  deletion_due_date: string
  reminder_sent_at?: string
  completed_at?: string
  completed_by?: string
  completion_notes: string
  created_at: string
}

interface RetentionTemplate {
  id: string
  data_category: string
  retention_period_months?: number
  retention_type: string
  legal_basis: string
  notes: string
}

interface RetentionSummary {
  total_activities: number
  with_retention_count: number
  missing_retention_count: number
  deletion_reminders_due: number
}

interface CreateReminderInput {
  description: string
  data_category: string
  deletion_due_date: string
  processing_activity_id?: string
}

interface CompleteReminderInput {
  completion_notes: string
}

function useRetentionSummary() {
  return useQuery<RetentionSummary>({
    queryKey: ['vaktprivacy', 'retention-summary'],
    queryFn: () => apiFetch<RetentionSummary>('/vaktprivacy/retention/summary'),
    staleTime: 60 * 1000,
  })
}

function useDeletionReminders() {
  return useQuery<DeletionReminder[]>({
    queryKey: ['vaktprivacy', 'deletion-reminders'],
    queryFn: () => apiFetch<DeletionReminder[]>('/vaktprivacy/deletion-reminders'),
    staleTime: 60 * 1000,
  })
}

function useRetentionTemplates() {
  return useQuery<RetentionTemplate[]>({
    queryKey: ['vaktprivacy', 'retention-templates'],
    queryFn: () => apiFetch<RetentionTemplate[]>('/vaktprivacy/retention-templates'),
    staleTime: 5 * 60 * 1000,
  })
}

function useCreateReminder() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: CreateReminderInput) =>
      apiFetch<DeletionReminder>('/vaktprivacy/deletion-reminders', { method: 'POST', body: JSON.stringify(input) }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['vaktprivacy', 'deletion-reminders'] })
      void qc.invalidateQueries({ queryKey: ['vaktprivacy', 'retention-summary'] })
    },
  })
}

function useCompleteReminder() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: CompleteReminderInput }) =>
      apiFetch<void>(`/vaktprivacy/deletion-reminders/${id}/complete`, { method: 'PATCH', body: JSON.stringify(input) }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['vaktprivacy', 'deletion-reminders'] })
      void qc.invalidateQueries({ queryKey: ['vaktprivacy', 'retention-summary'] })
    },
  })
}

function daysUntil(dateStr: string): number {
  return Math.ceil((new Date(dateStr).getTime() - Date.now()) / (1000 * 60 * 60 * 24))
}

function DueBadge({ date }: { date: string }) {
  const days = daysUntil(date)
  if (days < 0) return <Badge className="bg-red-100 text-red-700 border-red-200 text-xs">Überfällig</Badge>
  if (days <= 7) return <Badge className="bg-orange-100 text-orange-700 border-orange-200 text-xs">in {days}d</Badge>
  if (days <= 14) return <Badge className="bg-yellow-100 text-yellow-700 border-yellow-200 text-xs">in {days}d</Badge>
  return <Badge variant="outline" className="text-xs">in {days}d</Badge>
}

export default function DeletionRemindersPage() {
  const { data: summary } = useRetentionSummary()
  const { data: reminders = [], isLoading } = useDeletionReminders()
  const { data: templates = [] } = useRetentionTemplates()
  const createMut = useCreateReminder()
  const completeMut = useCompleteReminder()

  const [createOpen, setCreateOpen] = useState(false)
  const [createForm, setCreateForm] = useState<CreateReminderInput>({
    description: '',
    data_category: '',
    deletion_due_date: '',
  })
  const [completeTarget, setCompleteTarget] = useState<DeletionReminder | null>(null)
  const [completeNotes, setCompleteNotes] = useState('')
  const [showTemplates, setShowTemplates] = useState(false)

  const open = reminders.filter(r => !r.completed_at)
  const done = reminders.filter(r => r.completed_at)
  const overdue = open.filter(r => daysUntil(r.deletion_due_date) < 0)
  const dueSoon = open.filter(r => { const d = daysUntil(r.deletion_due_date); return d >= 0 && d <= 14; })

  function handleCreate() {
    createMut.mutate(createForm, {
      onSuccess: () => {
        setCreateOpen(false)
        setCreateForm({ description: '', data_category: '', deletion_due_date: '' })
      },
    })
  }

  function handleComplete() {
    if (!completeTarget) return
    completeMut.mutate(
      { id: completeTarget.id, input: { completion_notes: completeNotes } },
      { onSuccess: () => { setCompleteTarget(null); setCompleteNotes(''); } },
    )
  }

  if (isLoading) return <div className="p-8"><SkeletonTable rows={5} cols={4} /></div>

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold">Löscherinnerungen</h1>
          <p className="text-gray-500 text-sm mt-1">Art. 5 Abs. 1 lit. e + Art. 30 Abs. 1 lit. f DSGVO — Datenlöschung nach Fristen</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" size="sm" onClick={() => { setShowTemplates(true); }}>
            <BookOpen className="h-4 w-4 mr-1.5" />
            DACH-Templates
          </Button>
          <Button size="sm" onClick={() => { setCreateOpen(true); }}>
            <Plus className="h-4 w-4 mr-1.5" />
            Erinnerung anlegen
          </Button>
        </div>
      </div>

      {/* Summary stats */}
      {summary && (
        <div className="grid grid-cols-4 gap-3">
          <div className="bg-white border rounded-lg p-3 text-center">
            <div className="text-xl font-bold">{summary.total_activities}</div>
            <div className="text-xs text-gray-500">VVT-Einträge</div>
          </div>
          <div className={`border rounded-lg p-3 text-center ${summary.missing_retention_count > 0 ? 'bg-amber-50 border-amber-200' : 'bg-white'}`}>
            <div className={`text-xl font-bold ${summary.missing_retention_count > 0 ? 'text-amber-700' : ''}`}>{summary.missing_retention_count}</div>
            <div className="text-xs text-gray-500">Ohne Löschfrist</div>
          </div>
          <div className={`border rounded-lg p-3 text-center ${overdue.length > 0 ? 'bg-red-50 border-red-200' : 'bg-white'}`}>
            <div className={`text-xl font-bold ${overdue.length > 0 ? 'text-red-600' : ''}`}>{overdue.length + dueSoon.length}</div>
            <div className="text-xs text-gray-500">Fällig (14 Tage)</div>
          </div>
          <div className="bg-white border rounded-lg p-3 text-center">
            <div className="text-xl font-bold text-green-600">{done.length}</div>
            <div className="text-xs text-gray-500">Erledigt</div>
          </div>
        </div>
      )}

      {/* Overdue/due-soon banner */}
      {(overdue.length > 0 || dueSoon.length > 0) && (
        <div className="flex items-start gap-3 p-4 bg-amber-50 border border-amber-200 rounded-lg">
          <AlertTriangle className="w-5 h-5 text-amber-600 shrink-0 mt-0.5" />
          <div>
            <p className="text-sm font-semibold text-amber-800">
              {overdue.length > 0 && `${overdue.length} Löschung${overdue.length > 1 ? 'en' : ''} überfällig`}
              {overdue.length > 0 && dueSoon.length > 0 && ', '}
              {dueSoon.length > 0 && `${dueSoon.length} fällig in den nächsten 14 Tagen`}
            </p>
            <p className="text-xs text-amber-600 mt-0.5">
              Prüfen Sie die Löschpflichten gemäß Art. 5 Abs. 1 lit. e DSGVO.
            </p>
          </div>
        </div>
      )}

      {reminders.length === 0 ? (
        <EmptyState
          icon={Trash2}
          title="Keine Löscherinnerungen"
          description="Legen Sie Erinnerungen für geplante Datenlöschungen an."
          action={<Button onClick={() => { setCreateOpen(true); }}><Plus className="h-4 w-4 mr-1.5" />Erinnerung anlegen</Button>}
        />
      ) : (
        <div className="space-y-6">
          {open.length > 0 && (
            <div className="space-y-3">
              <h2 className="text-sm font-semibold text-gray-700">Offene Erinnerungen ({open.length})</h2>
              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
                {open.map((r) => (
                  <Card key={r.id} className={daysUntil(r.deletion_due_date) < 0 ? 'border-red-300' : daysUntil(r.deletion_due_date) <= 14 ? 'border-amber-300' : ''}>
                    <CardContent className="pt-4 space-y-2">
                      <div className="flex items-start justify-between gap-2">
                        <p className="text-sm font-medium line-clamp-2">{r.description}</p>
                        <DueBadge date={r.deletion_due_date} />
                      </div>
                      {r.data_category && (
                        <Badge variant="outline" className="text-xs">{r.data_category}</Badge>
                      )}
                      <p className="text-xs text-gray-400">Fällig: {r.deletion_due_date}</p>
                      <div className="flex justify-end pt-1">
                        <Button
                          size="sm"
                          variant="outline"
                          className="h-7 text-xs gap-1 text-green-600 border-green-300 hover:bg-green-50"
                          onClick={() => { setCompleteTarget(r); setCompleteNotes(''); }}
                        >
                          <CheckCircle2 className="w-3.5 h-3.5" />
                          Erledigt
                        </Button>
                      </div>
                    </CardContent>
                  </Card>
                ))}
              </div>
            </div>
          )}

          {done.length > 0 && (
            <div className="space-y-3">
              <h2 className="text-sm font-semibold text-gray-400">Erledigte Erinnerungen ({done.length})</h2>
              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
                {done.map((r) => (
                  <Card key={r.id} className="opacity-60">
                    <CardContent className="pt-4 space-y-1.5">
                      <div className="flex items-start justify-between gap-2">
                        <p className="text-sm font-medium line-clamp-1">{r.description}</p>
                        <Badge className="bg-green-100 text-green-700 text-xs shrink-0">Erledigt</Badge>
                      </div>
                      {r.data_category && <p className="text-xs text-gray-400">{r.data_category}</p>}
                      {r.completion_notes && <p className="text-xs text-gray-400 italic line-clamp-1">{r.completion_notes}</p>}
                    </CardContent>
                  </Card>
                ))}
              </div>
            </div>
          )}
        </div>
      )}

      {/* Create Dialog */}
      <Dialog open={createOpen} onOpenChange={(open) => { if (!open) setCreateOpen(false); }}>
        <DialogContent className="max-w-md">
          <DialogHeader><DialogTitle>Löscherinnerung anlegen</DialogTitle></DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-1.5">
              <Label>Beschreibung *</Label>
              <Textarea
                rows={2}
                placeholder="z.B. Kundendaten nach Vertragsende löschen"
                value={createForm.description}
                onChange={(e) => { setCreateForm(f => ({ ...f, description: e.target.value })); }}
              />
            </div>
            <div className="space-y-1.5">
              <Label>Datenkategorie</Label>
              <Input
                placeholder="z.B. Kundenstammdaten, Bewerbungsunterlagen"
                value={createForm.data_category}
                onChange={(e) => { setCreateForm(f => ({ ...f, data_category: e.target.value })); }}
              />
            </div>
            <div className="space-y-1.5">
              <Label>Lösch-Datum *</Label>
              <Input
                type="date"
                value={createForm.deletion_due_date}
                onChange={(e) => { setCreateForm(f => ({ ...f, deletion_due_date: e.target.value })); }}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setCreateOpen(false); }}>Abbrechen</Button>
            <Button
              onClick={handleCreate}
              disabled={!createForm.description.trim() || !createForm.deletion_due_date || createMut.isPending}
            >
              {createMut.isPending ? 'Speichern…' : 'Anlegen'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Complete Dialog */}
      <Dialog open={completeTarget !== null} onOpenChange={(open) => { if (!open) setCompleteTarget(null); }}>
        <DialogContent className="max-w-md">
          <DialogHeader><DialogTitle>Löschung bestätigen</DialogTitle></DialogHeader>
          <p className="text-sm text-gray-500">{completeTarget?.description}</p>
          <div className="space-y-2 py-2">
            <Label>Nachweis / Notiz</Label>
            <Textarea
              rows={3}
              placeholder="z.B. Datensätze aus DB gelöscht, Backups überschrieben nach 30 Tagen."
              value={completeNotes}
              onChange={(e) => { setCompleteNotes(e.target.value); }}
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setCompleteTarget(null); }}>Abbrechen</Button>
            <Button onClick={handleComplete} disabled={completeMut.isPending} className="bg-green-600 hover:bg-green-700 text-white">
              <CheckCircle2 className="w-4 h-4 mr-1.5" />
              {completeMut.isPending ? 'Speichern…' : 'Löschung bestätigen'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Templates Dialog */}
      <Dialog open={showTemplates} onOpenChange={(open) => { if (!open) setShowTemplates(false); }}>
        <DialogContent className="max-w-2xl max-h-[80vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>DACH-Löschfristen-Templates</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-gray-500">Gesetzliche Aufbewahrungsfristen für DACH-Unternehmen. Klicken Sie auf ein Template, um eine Erinnerung anzulegen.</p>
          <div className="space-y-2 py-2">
            {templates.length === 0 && (
              <p className="text-sm text-gray-400 text-center py-4">Keine Templates verfügbar.</p>
            )}
            {templates.map((t) => (
              <div
                key={t.id}
                className="flex items-center justify-between gap-3 p-3 border rounded-lg hover:bg-gray-50 cursor-pointer"
                onClick={() => {
                  const months = t.retention_period_months ?? 12
                  const dueDate = new Date()
                  dueDate.setMonth(dueDate.getMonth() + months)
                  setCreateForm({
                    description: `${t.data_category} — Löschfrist nach ${months} Monaten`,
                    data_category: t.data_category,
                    deletion_due_date: dueDate.toISOString().slice(0, 10),
                  })
                  setShowTemplates(false)
                  setCreateOpen(true)
                }}
              >
                <div className="min-w-0">
                  <p className="text-sm font-medium">{t.data_category}</p>
                  <p className="text-xs text-gray-500 mt-0.5">{t.legal_basis || t.notes}</p>
                </div>
                <div className="text-right shrink-0">
                  {t.retention_period_months && (
                    <Badge variant="outline" className="text-xs">{t.retention_period_months} Monate</Badge>
                  )}
                </div>
              </div>
            ))}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setShowTemplates(false); }}>Schließen</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
