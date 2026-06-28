import { useState } from 'react'
import { useTranslation } from 'react-i18next'
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

function useEventLabels(): Record<WebhookEvent, string> {
  const { t } = useTranslation()
  return {
    'finding.created':          t('webhooks.events.findingCreated'),
    'finding.severity_changed': t('webhooks.events.findingSeverityChanged'),
    'incident.created':         t('webhooks.events.incidentCreated'),
    'incident.status_changed':  t('webhooks.events.incidentStatusChanged'),
    'control.status_changed':   t('webhooks.events.controlStatusChanged'),
  }
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
  const { t } = useTranslation()
  const eventLabels = useEventLabels()
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
          <DialogTitle>{isEdit ? t('webhooks.dialog.titleEdit') : t('webhooks.dialog.titleCreate')}</DialogTitle>
        </DialogHeader>

        <div className="space-y-4 py-2">
          {/* Name */}
          <div className="space-y-1.5">
            <Label htmlFor="wh-name">{t('webhooks.dialog.labelName')}</Label>
            <Input
              id="wh-name"
              value={name}
              onChange={(e) => { setName(e.target.value); }}
              placeholder={t('webhooks.namePlaceholder')}
            />
          </div>

          {/* URL */}
          <div className="space-y-1.5">
            <Label htmlFor="wh-url">
              {t('webhooks.dialog.labelUrl')} <span className="text-red-500">*</span>
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
            <Label htmlFor="wh-secret">{t('webhooks.dialog.labelSecret')}</Label>
            <div className="flex items-center gap-2">
              <div className="relative flex-1">
                <Input
                  id="wh-secret"
                  type={showSecret ? 'text' : 'password'}
                  value={secret}
                  onChange={(e) => { setSecret(e.target.value); }}
                  placeholder={isEdit ? t('webhooks.secretUnchanged') : t('webhooks.secretPlaceholder')}
                  className="pr-9"
                />
                <button
                  type="button"
                  className="absolute right-2 top-1/2 -translate-y-1/2 text-secondary hover:text-primary"
                  onClick={() => { setShowSecret((s) => !s); }}
                  aria-label={showSecret ? t('webhooks.dialog.secretHideLabel') : t('webhooks.dialog.secretShowLabel')}
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
            <Label>{t('webhooks.dialog.labelEvents')}</Label>
            <div className="space-y-2">
              {ALL_EVENTS.map((ev) => (
                <label key={ev} className="flex items-center gap-2.5 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={events.includes(ev)}
                    onChange={() => { toggleEvent(ev); }}
                    className="rounded border-border w-4 h-4 accent-brand"
                  />
                  <span className="text-sm text-primary">{eventLabels[ev]}</span>
                  <span className="text-xs text-secondary font-mono">{ev}</span>
                </label>
              ))}
            </div>
          </div>

          {/* Active */}
          <div className="flex items-center justify-between">
            <Label htmlFor="wh-active">{t('webhooks.dialog.labelActive')}</Label>
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
            {t('webhooks.dialog.cancel')}
          </Button>
          <Button onClick={() => { void handleSave() }} disabled={isPending || !url.trim()}>
            {isPending ? t('webhooks.dialog.saving') : t('webhooks.dialog.save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ─── Main Page ────────────────────────────────────────────────────────────────

export default function WebhooksPage() {
  const { t } = useTranslation()
  const eventLabels = useEventLabels()
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
        title={t('webhooks.title')}
        description={t('webhooks.description')}
        actions={
          <Button onClick={openCreate}>
            <Plus className="w-4 h-4 mr-1.5" />
            {t('webhooks.addButton')}
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
            title={t('webhooks.noWebhooks')}
            description={t('webhooks.noWebhooksHint')}
            action={
              <Button onClick={openCreate}>
                <Plus className="w-4 h-4 mr-1.5" />
                {t('webhooks.addButton')}
              </Button>
            }
          />
        )}

        {!isLoading && webhooks.length > 0 && (
          <div className="rounded-md border border-border bg-surface overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('webhooks.colName')}</TableHead>
                  <TableHead>{t('webhooks.colUrl')}</TableHead>
                  <TableHead>{t('webhooks.colEvents')}</TableHead>
                  <TableHead>{t('webhooks.colActive')}</TableHead>
                  <TableHead>{t('webhooks.colLastTriggered')}</TableHead>
                  <TableHead className="text-right">{t('webhooks.colActions')}</TableHead>
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
                            {eventLabels[ev] ?? ev}
                          </Badge>
                        ))}
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge variant={wh.active ? 'success' : 'secondary'} className="text-[10px]">
                        {wh.active ? t('webhooks.statusActive') : t('webhooks.statusInactive')}
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
                          title={t('webhooks.testPing')}
                          onClick={() => { testWebhook.mutate(wh.id); }}
                          disabled={testWebhook.isPending}
                        >
                          <Zap className="w-3.5 h-3.5" aria-hidden="true" />
                          <span className="sr-only">{t('webhooks.testPing')}</span>
                        </Button>
                        <Button
                          size="sm"
                          variant="ghost"
                          className="h-7 w-7 p-0"
                          title={t('webhooks.edit')}
                          onClick={() => { openEdit(wh); }}
                        >
                          <Pencil className="w-3.5 h-3.5" aria-hidden="true" />
                          <span className="sr-only">{t('webhooks.edit')}</span>
                        </Button>
                        <Button
                          size="sm"
                          variant="ghost"
                          className="h-7 w-7 p-0 text-secondary hover:text-red-500 hover:bg-red-500/10"
                          title={t('webhooks.delete')}
                          onClick={() => { setDeleteTarget(wh); }}
                        >
                          <Trash2 className="w-3.5 h-3.5" aria-hidden="true" />
                          <span className="sr-only">{t('webhooks.delete')}</span>
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
            <AlertDialogTitle>{t('webhooks.deleteDialog.title')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('webhooks.deleteDialog.description', { name: deleteTarget?.name ?? '' })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('webhooks.deleteDialog.cancel')}</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleDelete}
              className="bg-red-600 hover:bg-red-700 focus:ring-red-600"
            >
              {t('webhooks.deleteDialog.confirm')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
