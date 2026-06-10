import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Trash2, Plus, FlaskConical, ChevronDown, ChevronRight, Bell, History, ExternalLink, Zap } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Button } from '../../../components/ui/button'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Badge } from '../../../components/ui/badge'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../../../components/ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../../../components/ui/select'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '../../../components/ui/table'
import { cn } from '../../../lib/utils'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'
import {
  ALERT_EVENTS,
  useAlertChannels,
  useCreateAlertChannel,
  useDeleteAlertChannel,
  useToggleAlertChannel,
  useTestAlertChannel,
  useAlertDeliveryLog,
  useChannelDeliveries,
  type AlertChannel,
  type CreateChannelInput,
} from '../../../modules/settings/hooks/useAlerting'

// ─── Helpers ─────────────────────────────────────────────────────────────────

const TYPE_BADGE_CLASS: Record<AlertChannel['type'], string> = {
  slack:   'bg-purple-900/40 text-purple-300 border-purple-800/40',
  teams:   'bg-blue-900/40 text-blue-300 border-blue-800/40',
  webhook: 'bg-zinc-800/60 text-zinc-300 border-zinc-700/40',
  email:   'bg-green-900/40 text-green-300 border-green-800/40',
}

const TYPE_LABELS: Record<AlertChannel['type'], string> = {
  slack:   'Slack',
  teams:   'Teams',
  webhook: 'Webhook',
  email:   'E-Mail',
}

// ─── Quick Setup: Slack & Teams ───────────────────────────────────────────────

interface QuickSetupCardProps {
  logo: React.ReactNode
  title: string
  description: string
  placeholder: string
  guideText: string
  guideUrl: string
  channelType: 'slack' | 'teams'
  defaultName: string
}

function QuickSetupCard({
  logo,
  title,
  description,
  placeholder,
  guideText,
  guideUrl,
  channelType,
  defaultName,
}: QuickSetupCardProps) {
  const { t } = useTranslation()
  const [url, setUrl]           = useState('')
  const [saving, setSaving]       = useState(false)
  const [saved, setSaved]         = useState(false)
  const [testing, setTesting]     = useState(false)
  const [testOk, setTestOk]       = useState<boolean | null>(null)
  const [createdId, setCreatedId] = useState<string | null>(null)

  const create    = useCreateAlertChannel()
  const testMut   = useTestAlertChannel()

  function handleSave() {
    if (!url.trim()) return
    setSaving(true)
    setSaved(false)
    setTestOk(null)
    create.mutate(
      {
        name: defaultName,
        type: channelType,
        url: url.trim(),
        events: ALERT_EVENTS.map((e) => e.value),
      },
      {
        onSuccess: (ch) => {
          setSaving(false)
          setSaved(true)
          setCreatedId(ch.id)
        },
        onError: () => { setSaving(false) },
      },
    )
  }

  function handleTest() {
    if (!createdId) return
    setTesting(true)
    setTestOk(null)
    testMut.mutate(createdId, {
      onSuccess: () => { setTesting(false); setTestOk(true)  },
      onError:   () => { setTesting(false); setTestOk(false) },
    })
  }

  return (
    <div className="bg-surface2/50 border border-border rounded-xl p-5 flex flex-col gap-3">
      <div className="flex items-start gap-3">
        <div className="mt-0.5 shrink-0">{logo}</div>
        <div className="min-w-0">
          <h3 className="text-sm font-semibold text-primary">{title}</h3>
          <p className="text-xs text-secondary mt-0.5">{description}</p>
        </div>
      </div>

      <div className="flex gap-2">
        <Input
          className="flex-1 text-xs h-8"
          placeholder={placeholder}
          value={url}
          onChange={(e) => { setUrl(e.target.value); setSaved(false); setCreatedId(null); setTestOk(null) }}
        />
        {saved && createdId ? (
          <Button
            size="sm"
            variant="outline"
            className="h-8 text-xs shrink-0"
            onClick={handleTest}
            disabled={testing}
          >
            {testing ? (
              <Spinner size="xs" className="mr-1.5" />
            ) : (
              <FlaskConical className="w-3.5 h-3.5 mr-1.5" />
            )}
            {t('settings.alerting.test')}
          </Button>
        ) : (
          <Button
            size="sm"
            className="h-8 text-xs shrink-0"
            onClick={handleSave}
            disabled={!url.trim() || saving}
          >
            {saving ? (
              <Spinner size="xs" color="white" className="mr-1.5" />
            ) : null}
            {t('settings.alerting.save')}
          </Button>
        )}
      </div>

      {testOk === true  && <p className="text-[11px] text-green-400">{t('settings.alerting.testSuccess')}</p>}
      {testOk === false && <p className="text-[11px] text-red-400">{t('settings.alerting.testFailed')}</p>}
      {saved && testOk === null && <p className="text-[11px] text-green-400">{t('settings.alerting.saveSuccess')}</p>}

      <a
        href={guideUrl}
        target="_blank"
        rel="noreferrer"
        className="inline-flex items-center gap-1 text-[11px] text-brand hover:underline"
      >
        <ExternalLink className="w-3 h-3" />
        {guideText}
      </a>
    </div>
  )
}

// Inline SVG logos (no external CDN dependency)
function SlackLogo() {
  return (
    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
      <path d="M5.042 15.165a2.528 2.528 0 0 1-2.52 2.523A2.528 2.528 0 0 1 0 15.165a2.527 2.527 0 0 1 2.522-2.52h2.52v2.52zm1.27 0a2.527 2.527 0 0 1 2.521-2.52 2.527 2.527 0 0 1 2.521 2.52v6.313A2.528 2.528 0 0 1 8.833 24a2.528 2.528 0 0 1-2.521-2.522v-6.313zM8.833 5.042a2.528 2.528 0 0 1-2.521-2.52A2.528 2.528 0 0 1 8.833 0a2.528 2.528 0 0 1 2.521 2.522v2.52H8.833zm0 1.27a2.528 2.528 0 0 1 2.521 2.521 2.528 2.528 0 0 1-2.521 2.521H2.522A2.528 2.528 0 0 1 0 8.833a2.528 2.528 0 0 1 2.522-2.521h6.311zm10.124 2.521a2.528 2.528 0 0 1 2.521-2.521A2.528 2.528 0 0 1 24 8.833a2.528 2.528 0 0 1-2.522 2.521h-2.521V8.833zm-1.268 0a2.528 2.528 0 0 1-2.523 2.521 2.527 2.527 0 0 1-2.52-2.521V2.522A2.527 2.527 0 0 1 15.166 0a2.528 2.528 0 0 1 2.523 2.522v6.311zm-2.523 10.124a2.528 2.528 0 0 1 2.523 2.522A2.528 2.528 0 0 1 15.166 24a2.527 2.527 0 0 1-2.52-2.522v-2.522h2.52zm0-1.268a2.527 2.527 0 0 1-2.52-2.523 2.526 2.526 0 0 1 2.52-2.52h6.313A2.527 2.527 0 0 1 24 15.165a2.528 2.528 0 0 1-2.522 2.523h-6.313z" fill="#E01E5A"/>
    </svg>
  )
}

function TeamsLogo() {
  return (
    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
      <path d="M20.625 6.375a2.625 2.625 0 1 1-5.25 0 2.625 2.625 0 0 1 5.25 0z" fill="#5059C9"/>
      <path d="M22.5 10.5h-7.875a.375.375 0 0 0-.375.375V16.5a5.625 5.625 0 0 0 5.25 5.625V20.25h.375A1.875 1.875 0 0 0 21.75 18.375v-7.5A.375.375 0 0 0 22.5 10.5z" fill="#5059C9"/>
      <path d="M12.375 4.5a3 3 0 1 1-6 0 3 3 0 0 1 6 0z" fill="#7B83EB"/>
      <path d="M15.375 10.5H3.375A.375.375 0 0 0 3 10.875v8.25A5.25 5.25 0 0 0 8.25 24h2.25a5.25 5.25 0 0 0 5.25-5.25v-7.875a.375.375 0 0 0-.375-.375z" fill="#7B83EB"/>
      <path d="M9.375 10.5H3.375A.375.375 0 0 0 3 10.875v8.25A5.25 5.25 0 0 0 8.25 24h1.125V10.5z" fill="#7B83EB" opacity=".1"/>
      <path d="M14.25 10.5H9.375V24h1.125a5.25 5.25 0 0 0 5.25-5.25v-7.875a.375.375 0 0 0-.5-.375z" fill="#7B83EB" opacity=".2"/>
    </svg>
  )
}

function QuickSetupSection() {
  const { t } = useTranslation()
  const { data: channels = [] } = useAlertChannels()
  const hasSlack = channels.some((ch) => ch.type === 'slack')
  const hasTeams = channels.some((ch) => ch.type === 'teams')

  if (hasSlack && hasTeams) return null

  return (
    <div className="bg-surface border border-border rounded-xl overflow-hidden">
      <div className="flex items-center gap-3 px-5 py-3.5 border-b border-border">
        <Zap className="w-4 h-4 text-brand" />
        <h2 className="text-sm font-semibold text-primary">{t('settings.alerting.quickSetup')}</h2>
        <span className="text-xs text-secondary">Slack und Teams in einem Schritt verbinden</span>
      </div>
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4 p-5">
        {!hasSlack && (
          <QuickSetupCard
            logo={<SlackLogo />}
            title="Slack"
            description="Erhalten Sie Alerts direkt in Ihrem Slack-Kanal."
            placeholder="https://hooks.slack.com/services/…"
            guideText="Anleitung: Incoming Webhook in Slack erstellen"
            guideUrl="https://api.slack.com/messaging/webhooks"
            channelType="slack"
            defaultName="Slack"
          />
        )}
        {!hasTeams && (
          <QuickSetupCard
            logo={<TeamsLogo />}
            title="Microsoft Teams"
            description="Erhalten Sie Alerts direkt in Ihrem Teams-Kanal."
            placeholder="https://outlook.office.com/webhook/…"
            guideText="Anleitung: Teams → Kanal → Connectors → Incoming Webhook"
            guideUrl="https://learn.microsoft.com/de-de/microsoftteams/platform/webhooks-and-connectors/how-to/add-incoming-webhook"
            channelType="teams"
            defaultName="Microsoft Teams"
          />
        )}
      </div>
    </div>
  )
}

// ─── Add Channel Dialog ───────────────────────────────────────────────────────

function AddChannelDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { t } = useTranslation()
  const [name, setName]   = useState('')
  const [type, setType]   = useState<AlertChannel['type']>('slack')
  const [url, setUrl]     = useState('')
  const [events, setEvents] = useState<string[]>([])

  const create = useCreateAlertChannel()

  function toggleEvent(value: string) {
    setEvents((prev) =>
      prev.includes(value) ? prev.filter((e) => e !== value) : [...prev, value],
    )
  }

  function handleSave() {
    if (!name.trim() || !url.trim() || events.length === 0) return
    const input: CreateChannelInput = { name: name.trim(), type, url: url.trim(), events }
    create.mutate(input, {
      onSuccess: () => {
        onClose()
        setName('')
        setUrl('')
        setEvents([])
        setType('slack')
      },
    })
  }

  return (
    <Dialog open={open} onOpenChange={(v) => { if (!v) { onClose(); } }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('settings.alerting.addChannel')}</DialogTitle>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="space-y-1.5">
            <Label>Name</Label>
            <Input
              placeholder="z.B. Security-Team Slack"
              value={name}
              onChange={(e) => { setName(e.target.value); }}
            />
          </div>
          <div className="space-y-1.5">
            <Label>Typ</Label>
            <Select value={type} onValueChange={(v) => { setType(v as AlertChannel['type']); }}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="slack">Slack Webhook</SelectItem>
                <SelectItem value="teams">Microsoft Teams</SelectItem>
                <SelectItem value="webhook">Webhook (HTTP POST)</SelectItem>
                <SelectItem value="email">E-Mail</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-1.5">
            <Label>{type === 'email' ? 'E-Mail-Adresse' : 'Webhook-URL'}</Label>
            <Input
              placeholder={
                type === 'slack'
                  ? 'https://hooks.slack.com/…'
                  : type === 'teams'
                  ? 'https://outlook.office.com/webhook/…'
                  : type === 'email'
                  ? 'team@example.com'
                  : 'https://webhook.example.com'
              }
              value={url}
              onChange={(e) => { setUrl(e.target.value); }}
            />
          </div>
          <div className="space-y-2">
            <Label>Ereignisse</Label>
            <div className="space-y-1.5">
              {ALERT_EVENTS.map(({ value, label }) => (
                <label
                  key={value}
                  className="flex items-center gap-2.5 cursor-pointer group"
                >
                  <input
                    type="checkbox"
                    checked={events.includes(value)}
                    onChange={() => { toggleEvent(value); }}
                    className="w-4 h-4 rounded border-border accent-indigo-500"
                  />
                  <span className="text-sm text-primary">{label}</span>
                  <code className="text-[10px] text-secondary font-mono ml-auto">{value}</code>
                </label>
              ))}
            </div>
            {events.length === 0 && (
              <p className="text-[11px] text-secondary">Mindestens ein Ereignis auswählen.</p>
            )}
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>{t('settings.alerting.cancel')}</Button>
          <Button
            onClick={handleSave}
            disabled={!name.trim() || !url.trim() || events.length === 0 || create.isPending}
          >
            {create.isPending ? t('settings.alerting.saving') : t('settings.alerting.add')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ─── Per-Channel Delivery History ────────────────────────────────────────────

function ChannelDeliveryHistory({ channelId }: { channelId: string }) {
  const { t } = useTranslation()
  const [expanded, setExpanded] = useState(false)
  const { data: entries = [], isLoading } = useChannelDeliveries(channelId, expanded)
  const { formatDateTime } = useFormatDate()

  return (
    <div className="border-t border-border">
      <button
        onClick={() => { setExpanded((v) => !v); }}
        className="w-full flex items-center gap-2 px-4 py-2 text-left hover:bg-surface2/50 transition-colors"
      >
        {expanded
          ? <ChevronDown className="w-3.5 h-3.5 text-secondary shrink-0" />
          : <ChevronRight className="w-3.5 h-3.5 text-secondary shrink-0" />
        }
        <History className="w-3.5 h-3.5 text-secondary shrink-0" />
        <span className="text-xs text-secondary">{t('settings.alerting.deliveryHistory')}</span>
        {!expanded && entries.length > 0 && (
          <span className="text-[10px] text-secondary ml-1">({entries.length})</span>
        )}
      </button>
      {expanded && (
        <div className="px-4 pb-3">
          {isLoading && (
            <div className="flex items-center gap-2 py-2">
              <Spinner size="xs" />
              <span className="text-[11px] text-secondary">{t('settings.alerting.loading')}</span>
            </div>
          )}
          {!isLoading && entries.length === 0 && (
            <p className="text-[11px] text-secondary py-2">{t('settings.alerting.noDeliveries')}</p>
          )}
          {!isLoading && entries.length > 0 && (
            <table className="w-full text-[11px]">
              <thead>
                <tr className="text-secondary border-b border-border">
                  <th className="text-left font-medium py-1 pr-3">Datum</th>
                  <th className="text-left font-medium py-1 pr-3">Event-Typ</th>
                  <th className="text-left font-medium py-1 pr-3">Status</th>
                  <th className="text-left font-medium py-1">HTTP</th>
                </tr>
              </thead>
              <tbody>
                {entries.slice(0, 50).map((e) => (
                  <tr key={e.id} className="border-b border-border/50 last:border-0">
                    <td className="py-1 pr-3 text-secondary whitespace-nowrap">{formatDateTime(e.sent_at, { day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' })}</td>
                    <td className="py-1 pr-3 font-mono text-secondary">{e.event}</td>
                    <td className="py-1 pr-3">
                      {e.status === 'sent'
                        ? <span className="text-green-400 font-semibold">{t('settings.alerting.sent')}</span>
                        : <span className="text-red-400 font-semibold">{t('settings.alerting.error')}</span>
                      }
                    </td>
                    <td className="py-1 text-secondary">{e.response_code ?? '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}
    </div>
  )
}

// ─── Channels Section ─────────────────────────────────────────────────────────

function ChannelsSection() {
  const { t } = useTranslation()
  const [dialogOpen, setDialogOpen] = useState(false)
  const { data: channels = [], isLoading, isError } = useAlertChannels()
  const deleteChannel  = useDeleteAlertChannel()
  const toggleChannel  = useToggleAlertChannel()
  const testChannel    = useTestAlertChannel()
  const [testingId, setTestingId] = useState<string | null>(null)
  const [testResult,  setTestResult]  = useState<{ id: string; ok: boolean } | null>(null)

  function handleTest(id: string) {
    setTestingId(id)
    setTestResult(null)
    testChannel.mutate(id, {
      onSuccess: () => { setTestResult({ id, ok: true  }); setTestingId(null) },
      onError:   () => { setTestResult({ id, ok: false }); setTestingId(null) },
    })
  }

  return (
    <div className="bg-surface border border-border rounded-xl overflow-x-auto">
      <div className="flex items-center justify-between px-5 py-3.5 border-b border-border">
        <div className="flex items-center gap-3">
          <Bell className="w-4 h-4 text-brand" />
          <h2 className="text-sm font-semibold text-primary">{t('settings.alerting.title')}</h2>
        </div>
        <Button size="sm" variant="outline" onClick={() => { setDialogOpen(true); }} className="h-7 text-xs">
          <Plus className="w-3 h-3 mr-1" />
          {t('settings.alerting.addChannel')}
        </Button>
      </div>

      {isLoading && (
        <div className="flex items-center justify-center py-10">
          <Spinner size="md" />
        </div>
      )}
      {isError && (
        <p className="px-5 py-4 text-xs text-secondary">Kanäle konnten nicht geladen werden.</p>
      )}
      {!isLoading && !isError && channels.length === 0 && (
        <p className="px-5 py-4 text-xs text-secondary">Noch keine Kanäle eingerichtet.</p>
      )}
      {!isLoading && !isError && channels.length > 0 && (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Typ</TableHead>
              <TableHead>Ereignisse</TableHead>
              <TableHead>Status</TableHead>
              <TableHead className="w-[120px]" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {channels.map((ch) => (
              <>
                <TableRow key={ch.id}>
                  <TableCell className="font-medium text-sm">{ch.name}</TableCell>
                  <TableCell>
                    <span
                      className={cn(
                        'inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-semibold border',
                        TYPE_BADGE_CLASS[ch.type],
                      )}
                    >
                      {TYPE_LABELS[ch.type]}
                    </span>
                  </TableCell>
                  <TableCell>
                    <span className="text-xs text-secondary">{ch.events.join(', ')}</span>
                  </TableCell>
                  <TableCell>
                    <button
                      onClick={() => { toggleChannel.mutate({ id: ch.id, enabled: !ch.enabled }); }}
                      disabled={toggleChannel.isPending}
                      className={cn(
                        'relative inline-flex h-5 w-9 items-center rounded-full transition-colors focus:outline-none',
                        ch.enabled ? 'bg-indigo-600' : 'bg-zinc-600',
                        toggleChannel.isPending && 'opacity-50 cursor-not-allowed',
                      )}
                      title={ch.enabled ? 'Deaktivieren' : 'Aktivieren'}
                    >
                      <span
                        className={cn(
                          'inline-block h-3.5 w-3.5 transform rounded-full bg-white shadow transition-transform',
                          ch.enabled ? 'translate-x-[18px]' : 'translate-x-[3px]',
                        )}
                      />
                    </button>
                  </TableCell>
                  <TableCell>
                    <div className="flex items-center gap-1.5 justify-end">
                      {testResult?.id === ch.id && (
                        <span className={cn('text-[10px] font-medium', testResult.ok ? 'text-green-400' : 'text-red-400')}>
                          {testResult.ok ? 'OK' : t('settings.alerting.error')}
                        </span>
                      )}
                      <button
                        onClick={() => { handleTest(ch.id); }}
                        disabled={testingId === ch.id}
                        title="Testbenachrichtigung senden"
                        className="p-1.5 rounded text-secondary hover:text-brand hover:bg-brand/10 transition-colors disabled:opacity-50"
                      >
                        {testingId === ch.id
                          ? <Spinner size="sm" className="w-3.5 h-3.5 border" />
                          : <FlaskConical className="w-3.5 h-3.5" />
                        }
                      </button>
                      <button
                        onClick={() => { deleteChannel.mutate(ch.id); }}
                        disabled={deleteChannel.isPending}
                        title="Kanal löschen"
                        className="p-1.5 rounded text-secondary hover:text-red-500 hover:bg-red-500/10 transition-colors disabled:opacity-50"
                      >
                        <Trash2 className="w-3.5 h-3.5" />
                      </button>
                    </div>
                  </TableCell>
                </TableRow>
                <TableRow key={`${ch.id}-deliveries`} className="hover:bg-transparent">
                  <TableCell colSpan={5} className="p-0">
                    <ChannelDeliveryHistory channelId={ch.id} />
                  </TableCell>
                </TableRow>
              </>
            ))}
          </TableBody>
        </Table>
      )}

      <AddChannelDialog open={dialogOpen} onClose={() => { setDialogOpen(false); }} />
    </div>
  )
}

// ─── Delivery History Section ─────────────────────────────────────────────────

function DeliveryHistorySection() {
  const { t } = useTranslation()
  const [expanded, setExpanded] = useState(false)
  const { data: entries = [], isLoading } = useAlertDeliveryLog()
  const { formatDateTime } = useFormatDate()

  return (
    <div className="bg-surface border border-border rounded-xl overflow-x-auto">
      <button
        onClick={() => { setExpanded((v) => !v); }}
        className="w-full flex items-center justify-between px-5 py-3.5 border-b border-border hover:bg-surface2/50 transition-colors"
      >
        <div className="flex items-center gap-3">
          {expanded
            ? <ChevronDown className="w-4 h-4 text-secondary" />
            : <ChevronRight className="w-4 h-4 text-secondary" />
          }
          <h2 className="text-sm font-semibold text-primary">{t('settings.alerting.deliveryLog')}</h2>
          {!expanded && entries.length > 0 && (
            <span className="text-[11px] text-secondary">({entries.length} Einträge)</span>
          )}
        </div>
      </button>

      {expanded && (
        <>
          {isLoading && (
            <div className="flex items-center justify-center py-6">
              <Spinner size="sm" />
            </div>
          )}
          {!isLoading && entries.length === 0 && (
            <p className="px-5 py-4 text-xs text-secondary">{t('settings.alerting.noDeliveries')}</p>
          )}
          {!isLoading && entries.length > 0 && (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Ereignis</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>HTTP</TableHead>
                  <TableHead>Zeitpunkt</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {entries.slice(0, 50).map((entry) => (
                  <TableRow key={entry.id}>
                    <TableCell>
                      <code className="text-xs font-mono text-secondary">{entry.event}</code>
                    </TableCell>
                    <TableCell>
                      <Badge
                        variant={entry.status === 'sent' ? 'success' : 'destructive'}
                        className="text-[10px]"
                      >
                        {entry.status === 'sent' ? t('settings.alerting.sent') : t('settings.alerting.error')}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <span className="text-xs text-secondary">
                        {entry.response_code ?? '—'}
                      </span>
                    </TableCell>
                    <TableCell>
                      <span className="text-xs text-secondary">{formatDateTime(entry.sent_at, { day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' })}</span>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </>
      )}
    </div>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function AlertingSettingsPage() {
  const { t } = useTranslation()
  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title={t('settings.alerting.title')}
        description={t('settings.alerting.description')}
      />
      <div className="flex-1 p-6 overflow-auto">
        <div className="max-w-5xl space-y-5">
          <QuickSetupSection />
          <ChannelsSection />
          <DeliveryHistorySection />
        </div>
      </div>
    </div>
  )
}
