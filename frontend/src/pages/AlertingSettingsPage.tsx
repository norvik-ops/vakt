import { useState, useEffect, useRef } from 'react'
import { Trash2, Plus, FlaskConical, ChevronDown, ChevronRight, Bell, GitBranch } from 'lucide-react'
import { PageHeader } from '../shared/components/PageHeader'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { Badge } from '../components/ui/badge'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../components/ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../components/ui/select'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '../components/ui/table'
import { cn } from '../lib/utils'
import {
  ALERT_EVENTS,
  useAlertChannels,
  useCreateAlertChannel,
  useDeleteAlertChannel,
  useToggleAlertChannel,
  useTestAlertChannel,
  useAlertDeliveryLog,
  type AlertChannel,
  type CreateChannelInput,
} from '../modules/settings/hooks/useAlerting'

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

function formatDate(iso: string) {
  return new Date(iso).toLocaleString('de-DE', {
    day: '2-digit', month: '2-digit', year: 'numeric',
    hour: '2-digit', minute: '2-digit',
  })
}

// ─── Add Channel Dialog ───────────────────────────────────────────────────────

function AddChannelDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
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
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Benachrichtigungskanal hinzufügen</DialogTitle>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="space-y-1.5">
            <Label>Name</Label>
            <Input
              placeholder="z.B. Security-Team Slack"
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </div>
          <div className="space-y-1.5">
            <Label>Typ</Label>
            <Select value={type} onValueChange={(v) => setType(v as AlertChannel['type'])}>
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
              onChange={(e) => setUrl(e.target.value)}
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
                    onChange={() => toggleEvent(value)}
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
          <Button variant="outline" onClick={onClose}>Abbrechen</Button>
          <Button
            onClick={handleSave}
            disabled={!name.trim() || !url.trim() || events.length === 0 || create.isPending}
          >
            {create.isPending ? 'Wird gespeichert…' : 'Hinzufügen'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ─── Channels Section ─────────────────────────────────────────────────────────

function ChannelsSection() {
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
    <div className="bg-surface border border-border rounded-xl overflow-hidden">
      <div className="flex items-center justify-between px-5 py-3.5 border-b border-border">
        <div className="flex items-center gap-3">
          <Bell className="w-4 h-4 text-brand" />
          <h2 className="text-sm font-semibold text-primary">Benachrichtigungskanäle</h2>
        </div>
        <Button size="sm" variant="outline" onClick={() => setDialogOpen(true)} className="h-7 text-xs">
          <Plus className="w-3 h-3 mr-1" />
          Kanal hinzufügen
        </Button>
      </div>

      {isLoading && (
        <div className="flex items-center justify-center py-10">
          <div className="w-5 h-5 border-2 border-brand border-t-transparent rounded-full animate-spin" />
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
                    onClick={() => toggleChannel.mutate({ id: ch.id, enabled: !ch.enabled })}
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
                        {testResult.ok ? 'OK' : 'Fehler'}
                      </span>
                    )}
                    <button
                      onClick={() => handleTest(ch.id)}
                      disabled={testingId === ch.id}
                      title="Testbenachrichtigung senden"
                      className="p-1.5 rounded text-secondary hover:text-brand hover:bg-brand/10 transition-colors disabled:opacity-50"
                    >
                      {testingId === ch.id
                        ? <div className="w-3.5 h-3.5 border border-brand border-t-transparent rounded-full animate-spin" />
                        : <FlaskConical className="w-3.5 h-3.5" />
                      }
                    </button>
                    <button
                      onClick={() => deleteChannel.mutate(ch.id)}
                      disabled={deleteChannel.isPending}
                      title="Kanal löschen"
                      className="p-1.5 rounded text-secondary hover:text-red-500 hover:bg-red-500/10 transition-colors disabled:opacity-50"
                    >
                      <Trash2 className="w-3.5 h-3.5" />
                    </button>
                  </div>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}

      <AddChannelDialog open={dialogOpen} onClose={() => setDialogOpen(false)} />
    </div>
  )
}

// ─── Delivery History Section ─────────────────────────────────────────────────

function DeliveryHistorySection() {
  const [expanded, setExpanded] = useState(false)
  const { data: entries = [], isLoading } = useAlertDeliveryLog()

  return (
    <div className="bg-surface border border-border rounded-xl overflow-hidden">
      <button
        onClick={() => setExpanded((v) => !v)}
        className="w-full flex items-center justify-between px-5 py-3.5 border-b border-border hover:bg-surface2/50 transition-colors"
      >
        <div className="flex items-center gap-3">
          {expanded
            ? <ChevronDown className="w-4 h-4 text-secondary" />
            : <ChevronRight className="w-4 h-4 text-secondary" />
          }
          <h2 className="text-sm font-semibold text-primary">Zustellungsprotokoll</h2>
          {!expanded && entries.length > 0 && (
            <span className="text-[11px] text-secondary">({entries.length} Einträge)</span>
          )}
        </div>
      </button>

      {expanded && (
        <>
          {isLoading && (
            <div className="flex items-center justify-center py-6">
              <div className="w-4 h-4 border-2 border-brand border-t-transparent rounded-full animate-spin" />
            </div>
          )}
          {!isLoading && entries.length === 0 && (
            <p className="px-5 py-4 text-xs text-secondary">Noch keine Zustellungen protokolliert.</p>
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
                        {entry.status === 'sent' ? 'Gesendet' : 'Fehler'}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <span className="text-xs text-secondary">
                        {entry.response_code ?? '—'}
                      </span>
                    </TableCell>
                    <TableCell>
                      <span className="text-xs text-secondary">{formatDate(entry.sent_at)}</span>
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

// ─── Escalation Chain Section ─────────────────────────────────────────────────

function EscalationChainSection() {
  const [tier1Days, setTier1Days] = useState(3)
  const [tier2Days, setTier2Days] = useState(7)
  const [tier2Email, setTier2Email] = useState('')
  const [tier3Days, setTier3Days] = useState(14)
  const [tier3Email, setTier3Email] = useState('')
  const [saved, setSaved] = useState<string | null>(null)
  const timerRef = useRef<ReturnType<typeof setTimeout>>()

  useEffect(() => () => clearTimeout(timerRef.current), [])

  function handleSave() {
    setSaved('Konfiguration gespeichert (Feature in Vorbereitung)')
    timerRef.current = setTimeout(() => setSaved(null), 3000)
  }

  return (
    <div className="bg-surface border border-border rounded-xl overflow-hidden">
      <div className="flex items-center gap-3 px-5 py-3.5 border-b border-border">
        <GitBranch className="w-4 h-4 text-brand" />
        <h2 className="text-sm font-semibold text-primary">Eskalationskette</h2>
        <span className="ml-auto inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-semibold border bg-amber-900/30 text-amber-300 border-amber-800/40">
          In Entwicklung
        </span>
      </div>

      <div className="px-5 py-4 space-y-5">
        <p className="text-xs text-secondary leading-relaxed">
          Wenn ein Alerting-Channel nicht reagiert oder ein Finding die SLA überschreitet,
          können Eskalationsstufen definiert werden.
        </p>

        {saved && (
          <div className="px-4 py-2 bg-green-50 border border-green-200 rounded-lg text-sm text-green-800">
            {saved}
          </div>
        )}

        <div className="space-y-4">
          {/* Tier 1 */}
          <div className="bg-surface border border-border rounded-lg p-4 space-y-3">
            <div className="flex items-center gap-2">
              <span className="text-[10px] font-bold uppercase tracking-wider text-secondary border border-border rounded px-1.5 py-0.5">
                Stufe 1
              </span>
              <span className="text-sm font-medium text-primary">Analyst benachrichtigen</span>
            </div>
            <div className="flex items-center gap-3">
              <Label className="text-xs text-secondary whitespace-nowrap">Nach</Label>
              <Input
                type="number"
                min={1}
                max={365}
                value={tier1Days}
                onChange={(e) => setTier1Days(Number(e.target.value))}
                className="h-8 text-sm w-20"
              />
              <Label className="text-xs text-secondary">Tagen</Label>
            </div>
          </div>

          {/* Tier 2 */}
          <div className="bg-surface border border-border rounded-lg p-4 space-y-3">
            <div className="flex items-center gap-2">
              <span className="text-[10px] font-bold uppercase tracking-wider text-secondary border border-border rounded px-1.5 py-0.5">
                Stufe 2
              </span>
              <span className="text-sm font-medium text-primary">CISO benachrichtigen</span>
            </div>
            <div className="flex items-center gap-3">
              <Label className="text-xs text-secondary whitespace-nowrap">Nach</Label>
              <Input
                type="number"
                min={1}
                max={365}
                value={tier2Days}
                onChange={(e) => setTier2Days(Number(e.target.value))}
                className="h-8 text-sm w-20"
              />
              <Label className="text-xs text-secondary">Tagen</Label>
            </div>
            <div className="space-y-1.5">
              <Label className="text-xs text-secondary">E-Mail-Adresse CISO</Label>
              <Input
                type="email"
                placeholder="ciso@example.com"
                value={tier2Email}
                onChange={(e) => setTier2Email(e.target.value)}
                className="h-8 text-sm"
              />
            </div>
          </div>

          {/* Tier 3 */}
          <div className="bg-surface border border-border rounded-lg p-4 space-y-3">
            <div className="flex items-center gap-2">
              <span className="text-[10px] font-bold uppercase tracking-wider text-secondary border border-border rounded px-1.5 py-0.5">
                Stufe 3
              </span>
              <span className="text-sm font-medium text-primary">Geschäftsführung per E-Mail</span>
            </div>
            <div className="flex items-center gap-3">
              <Label className="text-xs text-secondary whitespace-nowrap">Nach</Label>
              <Input
                type="number"
                min={1}
                max={365}
                value={tier3Days}
                onChange={(e) => setTier3Days(Number(e.target.value))}
                className="h-8 text-sm w-20"
              />
              <Label className="text-xs text-secondary">Tagen</Label>
            </div>
            <div className="space-y-1.5">
              <Label className="text-xs text-secondary">E-Mail-Adresse Geschäftsführung</Label>
              <Input
                type="email"
                placeholder="ceo@example.com"
                value={tier3Email}
                onChange={(e) => setTier3Email(e.target.value)}
                className="h-8 text-sm"
              />
            </div>
          </div>
        </div>

        <div className="flex justify-end pt-1">
          <Button size="sm" onClick={handleSave} className="h-8 text-sm">
            Speichern
          </Button>
        </div>
      </div>
    </div>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function AlertingSettingsPage() {
  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="Benachrichtigungen"
        description="Externe Webhooks und Alert-Kanäle für sicherheitsrelevante Ereignisse."
      />
      <div className="flex-1 p-6 overflow-auto">
        <div className="max-w-5xl space-y-5">
          <ChannelsSection />
          <DeliveryHistorySection />
          {/* TODO: implement when backend exists */}
          {false && <EscalationChainSection />}
        </div>
      </div>
    </div>
  )
}
