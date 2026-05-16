import { useState } from 'react'
import { AlertTriangle, Plus, Clock, CheckCircle2, Pencil, Trash2, FileDown } from 'lucide-react'
import { Button } from '../../../components/ui/button'
import { Card, CardContent } from '../../../components/ui/card'
import { Badge } from '../../../components/ui/badge'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../../../components/ui/dialog'
import { AlertDialog, AlertDialogContent, AlertDialogHeader, AlertDialogTitle, AlertDialogDescription, AlertDialogFooter, AlertDialogCancel, AlertDialogAction } from '../../../components/ui/alert-dialog'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Textarea } from '../../../components/ui/textarea'
import { PageHeader } from '../../../shared/components/PageHeader'
import { EmptyState } from '../../../shared/components/EmptyState'
import { InfoBanner } from '../../../shared/components/InfoBanner'
import { Pagination } from '../../../shared/components/Pagination'
import { useBreaches, useCreateBreach, useUpdateBreach, useDeleteBreach, useMarkAuthorityNotified, useExportBreachNotification } from '../hooks/useBreaches'
import type { Breach, CreateBreachInput, UpdateBreachInput } from '../types'

const STATUS_LABELS: Record<Breach['status'], string> = {
  open: 'Offen',
  authority_notified: 'Behörde informiert',
  closed: 'Geschlossen',
}

const STATUS_CLASS: Record<Breach['status'], string> = {
  open: 'bg-red-500/20 text-red-400 border-red-500/30',
  authority_notified: 'bg-amber-500/20 text-amber-400 border-amber-500/30',
  closed: 'bg-secondary text-secondary-foreground',
}

function tagsFromRaw(raw: string): string[] {
  return raw.split(',').map((s) => s.trim()).filter(Boolean)
}

function rawFromTags(tags: string[]): string {
  return tags.join(', ')
}

interface BreachFormState {
  title: string
  description: string
  discovered_at: string
  subjects_notification_required: boolean
  rawCount: string
  rawCategories: string
}

function emptyForm(): BreachFormState {
  return {
    title: '',
    description: '',
    discovered_at: new Date().toISOString().slice(0, 16),
    subjects_notification_required: false,
    rawCount: '',
    rawCategories: '',
  }
}

function formFromEntry(b: Breach): BreachFormState {
  return {
    title: b.title,
    description: b.description,
    discovered_at: b.discovered_at.slice(0, 16),
    subjects_notification_required: b.subjects_notification_required,
    rawCount: b.affected_count != null ? String(b.affected_count) : '',
    rawCategories: rawFromTags(b.data_categories),
  }
}

function DeadlineIndicator({ deadline }: { deadline: string }) {
  const now = new Date()
  const dl = new Date(deadline)
  const hoursLeft = (dl.getTime() - now.getTime()) / 1000 / 3600
  const overdue = hoursLeft < 0

  return (
    <div className={`flex items-center gap-1 text-xs ${overdue ? 'text-red-400' : hoursLeft < 24 ? 'text-amber-400' : 'text-muted-foreground'}`}>
      <Clock className="w-3 h-3" />
      {overdue
        ? `Frist überschritten (${dl.toLocaleDateString('de-DE')})`
        : `Meldefrist: ${dl.toLocaleDateString('de-DE', { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' })}`}
    </div>
  )
}

function BreachCard({
  breach,
  onNotify,
  onEdit,
  onDelete,
  onExportPDF,
}: {
  breach: Breach
  onNotify: (id: string) => void
  onEdit: (b: Breach) => void
  onDelete: (id: string) => void
  onExportPDF: (id: string) => void
}) {
  const discoveredDate = new Date(breach.discovered_at).toLocaleDateString('de-DE', {
    year: 'numeric', month: 'short', day: 'numeric',
  })

  return (
    <Card className={breach.status === 'open' ? 'border-red-500/30' : ''}>
      <CardContent className="pt-5 space-y-2">
        <div className="flex items-start justify-between gap-2">
          <p className="font-medium text-sm">{breach.title}</p>
          <Badge className={STATUS_CLASS[breach.status]}>{STATUS_LABELS[breach.status]}</Badge>
        </div>
        <p className="text-xs text-muted-foreground line-clamp-2">{breach.description}</p>
        <DeadlineIndicator deadline={breach.authority_deadline_at} />
        <div className="flex items-center justify-between text-xs text-muted-foreground">
          <span>Entdeckt: {discoveredDate}</span>
          {breach.affected_count != null && (
            <span>{breach.affected_count.toLocaleString('de-DE')} Betroffene</span>
          )}
        </div>
        {breach.data_categories.length > 0 && (
          <div className="flex flex-wrap gap-1">
            {breach.data_categories.map((c) => (
              <span key={c} className="text-xs px-1.5 py-0.5 rounded bg-red-500/10 text-red-400">{c}</span>
            ))}
          </div>
        )}
        {breach.status === 'open' && (
          <Button
            size="sm"
            variant="outline"
            className="w-full mt-1 text-xs border-amber-500/40 text-amber-400 hover:bg-amber-500/10"
            onClick={() => onNotify(breach.id)}
          >
            <CheckCircle2 className="w-3.5 h-3.5 mr-1" />
            Behörde informiert markieren
          </Button>
        )}
        <div className="flex justify-end gap-1">
          <Button
            size="icon"
            variant="ghost"
            className="h-7 w-7 text-muted-foreground hover:text-primary"
            title="Meldung als PDF exportieren (Art. 33 DSGVO)"
            onClick={() => onExportPDF(breach.id)}
          >
            <FileDown className="w-3.5 h-3.5" />
          </Button>
          <Button size="icon" variant="ghost" className="h-7 w-7" aria-label="Bearbeiten" onClick={() => onEdit(breach)}>
            <Pencil className="w-3.5 h-3.5" />
          </Button>
          <Button
            size="icon"
            variant="ghost"
            className="h-7 w-7 text-destructive hover:text-destructive"
            aria-label="Löschen"
            onClick={() => onDelete(breach.id)}
          >
            <Trash2 className="w-3.5 h-3.5" />
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}

export default function BreachPage() {
  const [dialogMode, setDialogMode] = useState<'create' | 'edit' | null>(null)
  const [editId, setEditId] = useState<string | null>(null)
  const [form, setForm] = useState<BreachFormState>(emptyForm())
  const [page, setPage] = useState(1)
  const [deleteId, setDeleteId] = useState<string | null>(null)

  const { data: breaches, isLoading, isError, pagination } = useBreaches(page)
  const createBreach = useCreateBreach()
  const updateBreach = useUpdateBreach()
  const deleteBreach = useDeleteBreach()
  const markNotified = useMarkAuthorityNotified()
  const exportPDF = useExportBreachNotification()

  function openCreate() {
    setForm(emptyForm())
    setEditId(null)
    setDialogMode('create')
  }

  function openEdit(breach: Breach) {
    setForm(formFromEntry(breach))
    setEditId(breach.id)
    setDialogMode('edit')
  }

  function handleDelete(id: string) {
    setDeleteId(id)
  }

  function confirmDelete() {
    if (deleteId) deleteBreach.mutate(deleteId)
    setDeleteId(null)
  }

  function handleSubmit() {
    const affectedCount = form.rawCount ? parseInt(form.rawCount, 10) : undefined
    const dataCategories = tagsFromRaw(form.rawCategories)

    if (dialogMode === 'create') {
      const payload: CreateBreachInput = {
        title: form.title,
        description: form.description,
        discovered_at: new Date(form.discovered_at).toISOString(),
        subjects_notification_required: form.subjects_notification_required,
        affected_count: affectedCount,
        data_categories: dataCategories,
      }
      createBreach.mutate(payload, { onSuccess: () => setDialogMode(null) })
    } else if (dialogMode === 'edit' && editId) {
      const payload: UpdateBreachInput = {
        title: form.title,
        description: form.description,
        subjects_notification_required: form.subjects_notification_required,
        affected_count: affectedCount,
        data_categories: dataCategories,
      }
      updateBreach.mutate({ id: editId, input: payload }, { onSuccess: () => setDialogMode(null) })
    }
  }

  const isPending = createBreach.isPending || updateBreach.isPending
  const canSubmit = form.title && form.description && (dialogMode === 'edit' || form.discovered_at) && !isPending

  const openBreaches = breaches?.filter((b) => b.status === 'open') ?? []
  const otherBreaches = breaches?.filter((b) => b.status !== 'open') ?? []

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="Datenpannen & Meldepflichten"
        description="Art. 33/34 DSGVO — Datenschutzverletzungen dokumentieren und fristgerecht melden."
        actions={
          <Button onClick={openCreate} variant="destructive">
            <Plus className="w-4 h-4 mr-1" />
            Datenpanne melden
          </Button>
        }
      />

      <InfoBanner icon={AlertTriangle} title="Meldepflicht bei Datenpannen (Art. 33/34 DSGVO)" variant="warning">
        <p>Eine Datenpanne, die voraussichtlich zu einem Risiko für Betroffene führt, <strong>muss binnen 72 Stunden</strong> der zuständigen Datenschutz-Aufsichtsbehörde gemeldet werden — ab dem Zeitpunkt der Entdeckung.</p>
        <p className="mt-1">Bei hohem Risiko ist zusätzlich die direkte Benachrichtigung der betroffenen Personen (Art. 34 DSGVO) erforderlich. Dokumentiere jede Panne — auch wenn keine Meldung nötig ist.</p>
      </InfoBanner>

      <div className="flex-1 p-6 space-y-6">
        {isLoading && (
          <div className="flex items-center justify-center h-48">
            <div className="w-6 h-6 border-2 border-primary border-t-transparent rounded-full animate-spin" />
          </div>
        )}

        {isError && (
          <div className="text-sm text-red-400 p-4 bg-red-500/10 rounded-lg">
            Fehler beim Laden der Datenpannen.
          </div>
        )}

        {!isLoading && !isError && breaches?.length === 0 && (
          <EmptyState
            icon={AlertTriangle}
            title="Keine Datenpannen gemeldet"
            description="Dokumentieren Sie Datenschutzverletzungen und verfolgen Sie die 72-Stunden-Meldefrist."
            action={
              <Button onClick={openCreate} variant="destructive">
                <Plus className="w-4 h-4 mr-1" />
                Datenpanne melden
              </Button>
            }
          />
        )}

        {!isLoading && !isError && breaches && breaches.length > 0 && (
          <>
            {openBreaches.length > 0 && (
              <div className="space-y-3">
                <h2 className="text-sm font-semibold text-red-400">Offene Meldungen ({openBreaches.length})</h2>
                <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
                  {openBreaches.map((b) => (
                    <BreachCard
                      key={b.id}
                      breach={b}
                      onNotify={(id) => markNotified.mutate(id)}
                      onEdit={openEdit}
                      onDelete={handleDelete}
                      onExportPDF={exportPDF}
                    />
                  ))}
                </div>
              </div>
            )}
            {otherBreaches.length > 0 && (
              <div className="space-y-3">
                <h2 className="text-sm font-semibold text-muted-foreground">Abgeschlossene Vorgänge</h2>
                <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
                  {otherBreaches.map((b) => (
                    <BreachCard
                      key={b.id}
                      breach={b}
                      onNotify={(id) => markNotified.mutate(id)}
                      onEdit={openEdit}
                      onDelete={handleDelete}
                      onExportPDF={exportPDF}
                    />
                  ))}
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

      <AlertDialog open={deleteId !== null} onOpenChange={(open) => !open && setDeleteId(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Datenpanne löschen?</AlertDialogTitle>
            <AlertDialogDescription>
              Diese Aktion kann nicht rückgängig gemacht werden.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={() => setDeleteId(null)}>Abbrechen</AlertDialogCancel>
            <AlertDialogAction onClick={confirmDelete} className="bg-destructive text-destructive-foreground hover:bg-destructive/90">Löschen</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <Dialog open={dialogMode !== null} onOpenChange={(open) => !open && setDialogMode(null)}>
        <DialogContent className="max-w-lg max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>
              {dialogMode === 'create' ? 'Datenpanne melden' : 'Datenpanne bearbeiten'}
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            {dialogMode === 'create' && (
              <div className="p-3 rounded-lg bg-amber-500/10 text-amber-400 text-xs">
                Die 72-Stunden-Meldefrist an die Aufsichtsbehörde beginnt ab dem Entdeckungszeitpunkt (Art. 33 DSGVO).
              </div>
            )}
            <div className="space-y-1.5">
              <Label>Bezeichnung *</Label>
              <Input
                placeholder="z.B. Unbefugter Zugriff auf Kundendaten"
                value={form.title}
                onChange={(e) => setForm((f) => ({ ...f, title: e.target.value }))}
              />
            </div>
            <div className="space-y-1.5">
              <Label>Beschreibung *</Label>
              <Textarea
                placeholder="Was ist passiert? Welche Daten sind betroffen?"
                rows={3}
                value={form.description}
                onChange={(e) => setForm((f) => ({ ...f, description: e.target.value }))}
              />
            </div>
            {dialogMode === 'create' && (
              <div className="space-y-1.5">
                <Label>Zeitpunkt der Entdeckung *</Label>
                <Input
                  type="datetime-local"
                  value={form.discovered_at}
                  onChange={(e) => setForm((f) => ({ ...f, discovered_at: e.target.value }))}
                />
              </div>
            )}
            <div className="space-y-1.5">
              <Label>Betroffene Datenkategorien (kommagetrennt)</Label>
              <Input
                placeholder="z.B. E-Mail-Adressen, Passwort-Hashes"
                value={form.rawCategories}
                onChange={(e) => setForm((f) => ({ ...f, rawCategories: e.target.value }))}
              />
            </div>
            <div className="space-y-1.5">
              <Label>Anzahl betroffener Personen (geschätzt)</Label>
              <Input
                type="number"
                min="0"
                placeholder="z.B. 500"
                value={form.rawCount}
                onChange={(e) => setForm((f) => ({ ...f, rawCount: e.target.value }))}
              />
            </div>
            <div className="flex items-center gap-2">
              <input
                type="checkbox"
                id="breach-subjects"
                checked={form.subjects_notification_required}
                onChange={(e) => setForm((f) => ({ ...f, subjects_notification_required: e.target.checked }))}
                className="w-4 h-4"
              />
              <Label htmlFor="breach-subjects">Benachrichtigung der Betroffenen erforderlich (Art. 34 DSGVO)</Label>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDialogMode(null)}>
              Abbrechen
            </Button>
            <Button
              variant={dialogMode === 'create' ? 'destructive' : 'default'}
              onClick={handleSubmit}
              disabled={!canSubmit}
            >
              {isPending ? 'Speichern …' : dialogMode === 'create' ? 'Datenpanne melden' : 'Speichern'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
