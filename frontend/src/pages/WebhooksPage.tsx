import { useState } from 'react'
import { Zap, Plus, Pencil, Trash2, Eye, EyeOff } from 'lucide-react'
import { PageHeader } from '../shared/components/PageHeader'
import { CopyButton } from '../shared/components/CopyButton'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { Badge } from '../components/ui/badge'
import { Switch } from '../components/ui/switch'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '../components/ui/dialog'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '../components/ui/alert-dialog'
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from '../components/ui/table'
import { Skeleton } from '../components/ui/skeleton'
import {
  useWebhooks,
  useCreateWebhook,
  useUpdateWebhook,
  useDeleteWebhook,
  useTestWebhook,
  type Webhook,
  type WebhookEvent,
  type CreateWebhookInput,
} from '../hooks/useWebhooks'
import { EmptyState } from '../shared/components/EmptyState'
import { useFormatDate } from '../shared/hooks/useFormatDate'

// ─── Event labels ─────────────────────────────────────────────────────────────

const EVENT_LABELS: Record<WebhookEvent, string> = {
  'finding.created':          'Finding erstellt',
  'finding.severity_changed': 'Finding-Schweregrad geändert',
  'incident.created':         'Vorfall erstellt',
  'incident.status_changed':  'Vorfall-Status geändert',
  'control.status_changed':   'Control-Status geändert',
}

const ALL_EVENTS: WebhookEvent[] = [
  'finding.created',
  'finding.severity_changed',
  'incident.created',
  'incident.status_changed',
  'control.status_changed',
]

// ─── Webhook Form Dialog ──────────────────────────────────────────────────────

interface WebhookDialogProps {
  open: boolean
  onClose: () => void
  initial?: Webhook
}

function WebhookDialog({ open, onClose, initial }: WebhookDialogProps) {
  const [name, setName] = useState(initial?.name ?? '')
  const [url, setUrl] = useState(initial?.url ?? '')
  const [secret, setSecret] = useState('')
  const [showSecret, setShowSecret] = useState(false)
  const [events, setEvents] = useState<WebhookEvent[]>(initial?.events ?? [])
  const [active, setActive] = useState(initial?.active ?? true)

  const createWebhook = useCreateWebhook()
  const updateWebhook = useUpdateWebhook(initial?.id ?? '')

  const isEdit = !!initial
  const isPending = createWebhook.isPending || updateWebhook.isPending

  function toggleEvent(ev: WebhookEvent) {
    setEvents((prev) =>
      prev.includes(ev) ? prev.filter((e) => e !== ev) : [...prev, ev]
    )
  }

  async function handleSave() {
    if (!url.trim()) return
    const input: CreateWebhookInput = {
      name: name.trim(),
      url: url.trim(),
      events,
      active,
      ...(secret.trim() ? { secret: secret.trim() } : {}),
    }
    try {
      if (isEdit) {
        await updateWebhook.mutateAsync(input)
      } else {
        await createWebhook.mutateAsync(input)
      }
      onClose()
    } catch {
      // Error stays visible in form
    }
  }

  const error = createWebhook.error ?? updateWebhook.error

  function handleOpenChange(v: boolean) {
    if (!v) onClose()
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{isEdit ? 'Webhook bearbeiten' : 'Webhook hinzufügen'}</DialogTitle>
        </DialogHeader>

        <div className="space-y-4 py-2">
          {/* Name */}
          <div className="space-y-1.5">
            <Label htmlFor="wh-name">Name</Label>
            <Input
              id="wh-name"
              value={name}
              onChange={(e) => { setName(e.target.value); }}
              placeholder="z.B. Security-Slack"
            />
          </div>

          {/* URL */}
          <div className="space-y-1.5">
            <Label htmlFor="wh-url">
              URL <span className="text-red-500">*</span>
            </Label>
            <Input
              id="wh-url"
              value={url}
              onChange={(e) => { setUrl(e.target.value); }}
              placeholder="https://hooks.example.com/…"
              required
            />
          </div>

          {/* Secret */}
          <div className="space-y-1.5">
            <Label htmlFor="wh-secret">Secret (optional)</Label>
            <div className="flex items-center gap-2">
              <div className="relative flex-1">
                <Input
                  id="wh-secret"
                  type={showSecret ? 'text' : 'password'}
                  value={secret}
                  onChange={(e) => { setSecret(e.target.value); }}
                  placeholder={isEdit ? '(unverändert)' : 'HMAC-Signatur-Secret'}
                  className="pr-9"
                />
                <button
                  type="button"
                  className="absolute right-2 top-1/2 -translate-y-1/2 text-secondary hover:text-primary"
                  onClick={() => { setShowSecret((s) => !s); }}
                  aria-label={showSecret ? 'Secret verbergen' : 'Secret anzeigen'}
                >
                  {showSecret ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                </button>
              </div>
              {showSecret && secret && (
                <CopyButton value={secret} className="shrink-0" />
              )}
            </div>
          </div>

          {/* Events */}
          <div className="space-y-2">
            <Label>Events</Label>
            <div className="space-y-2">
              {ALL_EVENTS.map((ev) => (
                <label key={ev} className="flex items-center gap-2.5 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={events.includes(ev)}
                    onChange={() => { toggleEvent(ev); }}
                    className="rounded border-border w-4 h-4 accent-brand"
                  />
                  <span className="text-sm text-primary">{EVENT_LABELS[ev]}</span>
                  <span className="text-xs text-secondary font-mono">{ev}</span>
                </label>
              ))}
            </div>
          </div>

          {/* Active */}
          <div className="flex items-center justify-between">
            <Label htmlFor="wh-active">Aktiv</Label>
            <Switch
              id="wh-active"
              checked={active}
              onCheckedChange={setActive}
            />
          </div>

          {error && (
            <p className="text-xs text-red-500">{error.message}</p>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={onClose} disabled={isPending}>
            Abbrechen
          </Button>
          <Button onClick={() => { void handleSave() }} disabled={isPending || !url.trim()}>
            {isPending ? 'Wird gespeichert…' : 'Speichern'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ─── Main Page ────────────────────────────────────────────────────────────────

export default function WebhooksPage() {
  const { formatDateTime } = useFormatDate()
  const { data, isLoading } = useWebhooks()
  const webhooks = data?.data ?? []

  const deleteWebhook = useDeleteWebhook()
  const testWebhook = useTestWebhook()

  const [dialogOpen, setDialogOpen] = useState(false)
  const [editTarget, setEditTarget] = useState<Webhook | undefined>()
  const [deleteTarget, setDeleteTarget] = useState<Webhook | null>(null)

  function openCreate() {
    setEditTarget(undefined)
    setDialogOpen(true)
  }

  function openEdit(wh: Webhook) {
    setEditTarget(wh)
    setDialogOpen(true)
  }

  function handleDelete() {
    if (!deleteTarget) return
    deleteWebhook.mutate(deleteTarget.id, {
      onSettled: () => { setDeleteTarget(null); },
    })
  }

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="Webhooks"
        description="Automatische HTTP-Benachrichtigungen bei Plattform-Ereignissen."
        actions={
          <Button onClick={openCreate}>
            <Plus className="w-4 h-4 mr-1.5" />
            Webhook hinzufügen
          </Button>
        }
      />

      <div className="flex-1 p-6 overflow-auto">
        {isLoading && (
          <div className="space-y-2">
            {Array.from({ length: 3 }).map((_, i) => (
              <Skeleton key={i} className="h-12 w-full rounded-lg" />
            ))}
          </div>
        )}

        {!isLoading && webhooks.length === 0 && (
          <EmptyState
            icon={Zap}
            title="Noch keine Webhooks"
            description="Erstellen Sie einen Webhook, um externe Systeme bei Ereignissen zu benachrichtigen."
            action={
              <Button onClick={openCreate}>
                <Plus className="w-4 h-4 mr-1.5" />
                Webhook hinzufügen
              </Button>
            }
          />
        )}

        {!isLoading && webhooks.length > 0 && (
          <div className="rounded-md border border-border bg-surface overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>URL</TableHead>
                  <TableHead>Events</TableHead>
                  <TableHead>Aktiv</TableHead>
                  <TableHead>Zuletzt ausgelöst</TableHead>
                  <TableHead className="text-right">Aktionen</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {webhooks.map((wh) => (
                  <TableRow key={wh.id}>
                    <TableCell className="font-medium text-primary">{wh.name}</TableCell>
                    <TableCell className="font-mono text-xs text-secondary max-w-[200px] truncate">
                      {wh.url.length > 40 ? `${wh.url.slice(0, 40)}…` : wh.url}
                    </TableCell>
                    <TableCell>
                      <div className="flex flex-wrap gap-1">
                        {wh.events.length === 0 && (
                          <span className="text-xs text-secondary">—</span>
                        )}
                        {wh.events.map((ev) => (
                          <Badge key={ev} variant="secondary" className="text-[10px] px-1.5 py-0">
                            {EVENT_LABELS[ev] ?? ev}
                          </Badge>
                        ))}
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge variant={wh.active ? 'success' : 'secondary'} className="text-[10px]">
                        {wh.active ? 'Aktiv' : 'Inaktiv'}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-xs text-secondary">
                      {wh.last_triggered_at
                        ? formatDateTime(wh.last_triggered_at)
                        : '—'}
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center justify-end gap-1">
                        <Button
                          size="sm"
                          variant="ghost"
                          className="h-7 w-7 p-0"
                          title="Test-Ping senden"
                          onClick={() => { testWebhook.mutate(wh.id); }}
                          disabled={testWebhook.isPending}
                        >
                          <Zap className="w-3.5 h-3.5" aria-hidden="true" />
                          <span className="sr-only">Test senden</span>
                        </Button>
                        <Button
                          size="sm"
                          variant="ghost"
                          className="h-7 w-7 p-0"
                          title="Bearbeiten"
                          onClick={() => { openEdit(wh); }}
                        >
                          <Pencil className="w-3.5 h-3.5" aria-hidden="true" />
                          <span className="sr-only">Bearbeiten</span>
                        </Button>
                        <Button
                          size="sm"
                          variant="ghost"
                          className="h-7 w-7 p-0 text-secondary hover:text-red-500 hover:bg-red-500/10"
                          title="Löschen"
                          onClick={() => { setDeleteTarget(wh); }}
                        >
                          <Trash2 className="w-3.5 h-3.5" aria-hidden="true" />
                          <span className="sr-only">Löschen</span>
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </div>

      {/* Create / Edit Dialog */}
      {dialogOpen && (
        <WebhookDialog
          open={dialogOpen}
          onClose={() => { setDialogOpen(false); setEditTarget(undefined) }}
          initial={editTarget}
        />
      )}

      {/* Delete Confirmation */}
      <AlertDialog open={!!deleteTarget} onOpenChange={(v) => { if (!v) setDeleteTarget(null) }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Webhook löschen?</AlertDialogTitle>
            <AlertDialogDescription>
              Der Webhook <strong>{deleteTarget?.name}</strong> wird dauerhaft gelöscht.
              Diese Aktion kann nicht rückgängig gemacht werden.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Abbrechen</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleDelete}
              className="bg-red-600 hover:bg-red-700 focus:ring-red-600"
            >
              Löschen
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
