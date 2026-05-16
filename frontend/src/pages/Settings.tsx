import { useState, useEffect, useRef } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import {
  Building2, Layers, Bell, Trash2, Plus, Check, X,
  Webhook, Globe, Mail, Server, MapPin, Download, ShieldCheck, FileText, ExternalLink, Sparkles, Rocket, Key, Clock,
} from 'lucide-react'
import { PageHeader } from '../shared/components/PageHeader'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { Badge } from '../components/ui/badge'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../components/ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../components/ui/select'
import { apiFetch, FeatureLockedError } from '../api/client'
import { useAuthStore } from '../shared/stores/auth'
import { cn } from '../lib/utils'
import { VAKT_LS_PORTAL_URL } from '../lib/constants'
import { useOrgSector, useUpdateOrgSector } from '../modules/secvitals/hooks/useOrgSector'
import { SECTOR_LABELS } from '../modules/secvitals/types'
import { useExportData } from '../hooks/useDataExport'
import { useAuditReport } from '../modules/secvitals/hooks/useAuditReport'
import { ProGate } from '../shared/components/ProGate'

// ─── Types ───────────────────────────────────────────────────────────────────

interface ModuleStatus {
  name: string
  enabled: boolean
}

interface NotificationChannel {
  id: string
  type: 'slack' | 'email' | 'webhook'
  name: string
  config: Record<string, string>
  enabled: boolean
  created_at: string
}

interface CreateChannelInput {
  type: 'slack' | 'email' | 'webhook'
  name: string
  config: Record<string, string>
}

// ─── API hooks ───────────────────────────────────────────────────────────────

function useModules() {
  return useQuery<{ data: ModuleStatus[] }>({
    queryKey: ['admin', 'modules'],
    queryFn: () => apiFetch<{ data: ModuleStatus[] }>('/admin/modules'),
    retry: false,
  })
}

function useNotificationChannels() {
  return useQuery<{ data: NotificationChannel[] }>({
    queryKey: ['admin', 'notifications', 'channels'],
    queryFn: () => apiFetch<{ data: NotificationChannel[] }>('/admin/notifications/channels'),
    retry: false,
  })
}

function useCreateChannel() {
  const qc = useQueryClient()
  return useMutation<NotificationChannel, Error, CreateChannelInput>({
    mutationFn: (input) =>
      apiFetch<NotificationChannel>('/admin/notifications/channels', {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['admin', 'notifications', 'channels'] }),
  })
}

function useDeleteChannel() {
  const qc = useQueryClient()
  return useMutation<void, Error, string>({
    mutationFn: (id) => apiFetch<void>(`/admin/notifications/channels/${id}`, { method: 'DELETE' }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['admin', 'notifications', 'channels'] }),
  })
}

// ─── Module labels ────────────────────────────────────────────────────────────

const MODULE_META: Record<string, { label: string; desc: string }> = {
  secpulse:   { label: 'Vakt Scan',     desc: 'Scanner-Orchestrierung & Schwachstellenmanagement' },
  secvitals:  { label: 'Vakt Comply',   desc: 'Compliance Frameworks, Risiken & Governance' },
  secvault:   { label: 'Vakt Vault',    desc: 'Secrets-Verwaltung & Git-Scanning' },
  secreflex:  { label: 'Vakt Aware',    desc: 'Phishing-Simulationen & Awareness-Training' },
  secprivacy: { label: 'Vakt Privacy',  desc: 'DSGVO-Dokumentation (VVT, DSFA, AVV, Datenpannen)' },
}

// ─── License ─────────────────────────────────────────────────────────────────

interface LicenseInfo {
  tier: string
  is_pro: boolean
  features: string[]
  org_name: string
  expires_at: string | null
  demo: boolean
  revoked: boolean
}

const FEATURE_LABELS: Record<string, string> = {
  tisax: 'TISAX',
  dora: 'DORA',
  eu_ai_act: 'EU AI Act',
  cra: 'CRA',
  ai_advisor: 'KI-Berater',
  audit_pdf: 'Audit-PDF Export',
  sso: 'SSO (OIDC/SAML)',
  api_access: 'API-Zugang',
  secreflex_advanced: 'Vakt Aware Pro',
  secpulse_advanced: 'Vakt Scan Pro',
}

function useLicense() {
  return useQuery<LicenseInfo>({
    queryKey: ['license'],
    queryFn: () => apiFetch<LicenseInfo>('/license'),
    staleTime: 60 * 1000,
  })
}

function useActivateLicense() {
  const qc = useQueryClient()
  return useMutation<LicenseInfo, Error, { key: string }>({
    mutationFn: (input) =>
      apiFetch<LicenseInfo>('/license/activate', {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['license'] }),
  })
}

function daysUntilExpiry(expiresAt: string): number {
  const days = Math.floor((new Date(expiresAt).getTime() - Date.now()) / 86400000)
  return Math.max(0, days)
}

function LicenseSection() {
  const { data: lic, isLoading } = useLicense()
  const activate = useActivateLicense()
  const [licKey, setLicKey] = useState('')
  const [activateSuccess, setActivateSuccess] = useState(false)
  const licTimerRef = useRef<ReturnType<typeof setTimeout>>()

  useEffect(() => () => clearTimeout(licTimerRef.current), [])

  if (isLoading) return (
    <SectionCard title="Lizenz" icon={Sparkles}>
      <div className="h-16 flex items-center justify-center">
        <div className="w-4 h-4 border-2 border-brand border-t-transparent rounded-full animate-spin" />
      </div>
    </SectionCard>
  )

  const isPro = lic?.is_pro ?? false

  function handleActivate() {
    const trimmed = licKey.trim()
    if (!trimmed) return
    activate.mutate({ key: trimmed }, {
      onSuccess: () => {
        setActivateSuccess(true)
        setLicKey('')
        licTimerRef.current = setTimeout(() => setActivateSuccess(false), 5000)
      },
    })
  }

  return (
    <SectionCard title="Lizenz" icon={Sparkles}>
      <div className="space-y-4">
        {lic?.revoked && (
          <div className="text-sm text-amber-700 bg-amber-50 border border-amber-200 rounded p-3">
            Ihr Pro-Abonnement wurde gekündigt. Ihr Zugang läuft mit dem Ende des aktuellen
            Abrechnungszeitraums aus. Bei Fragen wenden Sie sich an{' '}
            <a href="mailto:support@vakt.app" className="underline font-medium">
              support@vakt.app
            </a>
            .
          </div>
        )}
        <div className="flex items-center gap-3">
          <Badge variant={isPro ? 'success' : 'secondary'} className="text-xs px-2.5 py-1">
            {isPro ? (lic?.demo ? 'Pro (Demo)' : 'Pro') : 'Community'}
          </Badge>
          {lic?.org_name && (
            <span className="text-sm text-secondary">{lic.org_name}</span>
          )}
        </div>

        {isPro && lic?.features && lic.features.length > 0 && (
          <div className="flex flex-wrap gap-1.5">
            {lic.features.map((f) => (
              <span key={f} className="text-xs bg-brand/10 text-brand px-2 py-0.5 rounded-md">
                {FEATURE_LABELS[f] ?? f}
              </span>
            ))}
          </div>
        )}

        {lic?.expires_at && (
          <p className="text-xs text-secondary">
            Gültig bis {new Date(lic.expires_at).toLocaleDateString('de-DE')}
          </p>
        )}

        {lic?.expires_at && daysUntilExpiry(lic.expires_at) < 30 && (
          <div className="text-sm text-amber-600 bg-amber-50 border border-amber-200 rounded p-2">
            {daysUntilExpiry(lic.expires_at) === 0
              ? 'Ihr Abonnement ist bereits abgelaufen.'
              : `Ihr Abonnement läuft in ${daysUntilExpiry(lic.expires_at)} Tagen ab.`}
          </div>
        )}

        {isPro && !lic?.demo && (
          <a href={VAKT_LS_PORTAL_URL} target="_blank" rel="noopener noreferrer" className="text-sm text-primary underline">
            Abonnement verwalten →
          </a>
        )}

        {!isPro && (
          <span className="inline-flex items-center gap-1.5 text-xs font-semibold text-brand">
            <Clock className="w-3.5 h-3.5" />
            Vakt Pro — demnächst verfügbar
          </span>
        )}

        {/* Pro-Key activation */}
        <div className="pt-1 border-t border-border space-y-2">
          <Label className="text-xs">Pro-Key aktivieren</Label>
          <div className="flex gap-2">
            <Input
              value={licKey}
              onChange={(e) => { setLicKey(e.target.value); setActivateSuccess(false) }}
              placeholder="Ihr Lizenzschlüssel"
              className="h-8 text-xs font-mono flex-1"
            />
            <Button
              size="sm"
              className="h-8 text-xs gap-1"
              onClick={handleActivate}
              disabled={!licKey.trim() || activate.isPending}
            >
              <Key className="w-3 h-3" />
              {activate.isPending ? 'Aktiviere…' : 'Aktivieren'}
            </Button>
          </div>
          {activateSuccess && (
            <p className="text-[11px] text-green-600 dark:text-green-400">Key aktiviert — Lizenz aktualisiert.</p>
          )}
          {activate.isError && (
            <p className="text-[11px] text-red-500">{activate.error.message}</p>
          )}
        </div>
      </div>
    </SectionCard>
  )
}

// ─── Section card ─────────────────────────────────────────────────────────────

function SectionCard({ title, icon: Icon, children }: {
  title: string
  icon: React.ElementType
  children: React.ReactNode
}) {
  return (
    <div className="bg-surface border border-border rounded-xl overflow-hidden h-fit">
      <div className="flex items-center gap-3 px-5 py-3.5 border-b border-border">
        <Icon className="w-4 h-4 text-brand" />
        <h2 className="text-sm font-semibold text-primary">{title}</h2>
      </div>
      <div className="p-5">{children}</div>
    </div>
  )
}

// ─── Organisation ─────────────────────────────────────────────────────────────

function OrgSection() {
  const { user } = useAuthStore()
  return (
    <SectionCard title="Organisation" icon={Building2}>
      <div className="space-y-3">
        <div className="space-y-1.5">
          <Label className="text-xs">Administrator</Label>
          <Input value={user?.email ?? '—'} readOnly className="bg-surface2 h-8 text-sm" />
        </div>
        <div className="space-y-1.5">
          <Label className="text-xs">Anzeigename</Label>
          <Input value={user?.display_name ?? '—'} readOnly className="bg-surface2 h-8 text-sm" />
        </div>
      </div>
    </SectionCard>
  )
}

// ─── Sector / NIS2 Configuration ─────────────────────────────────────────────

const FEDERAL_STATES = [
  'Baden-Württemberg', 'Bayern', 'Berlin', 'Brandenburg', 'Bremen',
  'Hamburg', 'Hessen', 'Mecklenburg-Vorpommern', 'Niedersachsen',
  'Nordrhein-Westfalen', 'Rheinland-Pfalz', 'Saarland', 'Sachsen',
  'Sachsen-Anhalt', 'Schleswig-Holstein', 'Thüringen',
]

function SectorSection() {
  const { data: settings } = useOrgSector()
  const { data: lic } = useLicense()
  const update = useUpdateOrgSector()
  const [sector, setSector] = useState('other')
  const [federalState, setFederalState] = useState('')

  useEffect(() => {
    if (settings) {
      setSector(settings.sector)
      setFederalState(settings.federal_state ?? '')
    }
  }, [settings])

  function handleSave() {
    update.mutate({ sector, federal_state: federalState || undefined })
  }

  const isDirty = settings
    ? sector !== settings.sector || federalState !== (settings.federal_state ?? '')
    : false

  // Community users see an upgrade prompt instead of the sector form
  const isPro = lic?.is_pro ?? true // default to true while loading to avoid flicker

  return (
    <SectionCard title="Sektor & NIS2-Konfiguration" icon={MapPin}>
      {lic !== undefined && !isPro ? (
        <div className="flex items-start gap-4">
          <div className="mt-0.5 p-2 rounded-lg bg-brand/10 shrink-0">
            <Sparkles className="w-4 h-4 text-brand" />
          </div>
          <div>
            <p className="font-semibold text-primary text-sm mb-1">Pro-Feature</p>
            <p className="text-secondary text-sm leading-relaxed mb-2">
              Die NIS2-Sektor-Konfiguration ist in der Community Edition nicht verfügbar.
              Vakt Pro mit KRITIS-Sektor-Mapping ist in Planung.
            </p>
            <span className="inline-flex items-center gap-1.5 text-xs font-semibold text-brand">
              <Clock className="w-3.5 h-3.5" />
              Demnächst verfügbar
            </span>
          </div>
        </div>
      ) : (
        <div className="space-y-3">
          <div className="space-y-1.5">
            <Label className="text-xs">KRITIS-Sektor</Label>
            <Select value={sector} onValueChange={setSector}>
              <SelectTrigger className="h-8 text-sm" data-testid="sector-select">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {Object.entries(SECTOR_LABELS).map(([value, label]) => (
                  <SelectItem key={value} value={value}>{label}</SelectItem>
                ))}
              </SelectContent>
            </Select>
            <p className="text-[11px] text-secondary">Bestimmt die zuständige NIS2-Meldebehörde.</p>
          </div>
          <div className="space-y-1.5">
            <Label className="text-xs">Bundesland</Label>
            <Select value={federalState} onValueChange={setFederalState}>
              <SelectTrigger className="h-8 text-sm" data-testid="federal-state-select">
                <SelectValue placeholder="— auswählen —" />
              </SelectTrigger>
              <SelectContent>
                {FEDERAL_STATES.map((s) => (
                  <SelectItem key={s} value={s}>{s}</SelectItem>
                ))}
              </SelectContent>
            </Select>
            <p className="text-[11px] text-secondary">Für DSGVO-Meldungen an Landesdatenschutzbehörde.</p>
          </div>
          <Button
            size="sm"
            className="h-7 text-xs"
            onClick={handleSave}
            disabled={!isDirty || update.isPending}
            data-testid="sector-save-btn"
          >
            {update.isPending ? 'Wird gespeichert…' : 'Speichern'}
          </Button>
          {update.isSuccess && (
            <p className="text-[11px] text-green-600 dark:text-green-400">Gespeichert.</p>
          )}
          {update.isError && (
            <p className="text-[11px] text-red-500">Fehler beim Speichern. Bitte erneut versuchen.</p>
          )}
        </div>
      )}
    </SectionCard>
  )
}

// ─── Module Status ────────────────────────────────────────────────────────────

function ModulesSection() {
  const { data, isLoading, isError } = useModules()
  const modules = data?.data ?? []

  return (
    <SectionCard title="Module" icon={Layers}>
      {isLoading && (
        <div className="flex items-center justify-center h-16">
          <div className="w-4 h-4 border-2 border-brand border-t-transparent rounded-full animate-spin" />
        </div>
      )}
      {isError && (
        <p className="text-xs text-secondary">Nicht ladbar — Admin-Rechte erforderlich.</p>
      )}
      {!isLoading && !isError && (
        <div className="space-y-1.5">
          {modules.map((m) => {
            const meta = MODULE_META[m.name]
            return (
              <div key={m.name} className="flex items-center justify-between py-2 px-3 rounded-lg bg-surface2">
                <div>
                  <div className="text-xs font-medium text-primary">{meta?.label ?? m.name}</div>
                  {meta?.desc && <div className="text-[11px] text-secondary">{meta.desc}</div>}
                </div>
                {m.enabled
                  ? <Badge variant="success" className="text-[10px] shrink-0"><Check className="w-2.5 h-2.5 mr-1" />Aktiv</Badge>
                  : <Badge variant="secondary" className="text-[10px] shrink-0"><X className="w-2.5 h-2.5 mr-1" />Deaktiviert</Badge>
                }
              </div>
            )
          })}
          <p className="text-[11px] text-secondary pt-1">
            Aktivierung über <code className="font-mono bg-surface2 px-1 rounded">.env</code> — Neustart erforderlich.
          </p>
        </div>
      )}
    </SectionCard>
  )
}

// ─── E-Mail / SMTP ────────────────────────────────────────────────────────────

function SmtpSection() {
  return (
    <SectionCard title="E-Mail (SMTP)" icon={Mail}>
      <div className="space-y-3">
        <div className="space-y-1.5">
          <Label className="text-xs">SMTP-Host</Label>
          <Input
            placeholder="smtp.example.com"
            readOnly
            className="bg-surface2 h-8 text-sm text-secondary"
            value="Wird über .env konfiguriert"
          />
        </div>
        <div className="space-y-1.5">
          <Label className="text-xs">SMTP-Port</Label>
          <Input
            placeholder="587"
            readOnly
            className="bg-surface2 h-8 text-sm text-secondary"
            value="Wird über .env konfiguriert"
          />
        </div>
        <div className="rounded-lg bg-surface2 p-3 text-[11px] text-secondary space-y-1 leading-relaxed">
          <p className="font-medium text-primary">Konfiguration in der .env-Datei:</p>
          <code className="block font-mono">VAKT_SMTP_HOST=smtp.example.com</code>
          <code className="block font-mono">VAKT_SMTP_PORT=587</code>
          <code className="block font-mono">VAKT_SMTP_USER=user@example.com</code>
          <code className="block font-mono">VAKT_SMTP_PASS=geheimespasswort</code>
          <p className="pt-1">Wird für Vakt Aware-Kampagnen und Einladungs-E-Mails verwendet.</p>
        </div>
      </div>
    </SectionCard>
  )
}

// ─── Notification Channels ────────────────────────────────────────────────────

const CHANNEL_ICONS: Record<string, React.ElementType> = {
  slack:   Webhook,
  email:   Mail,
  webhook: Globe,
}

const CHANNEL_LABELS: Record<string, string> = {
  slack:   'Slack',
  email:   'E-Mail',
  webhook: 'Webhook',
}

function NotificationsSection() {
  const [createOpen, setCreateOpen] = useState(false)
  const [type, setType] = useState<'slack' | 'email' | 'webhook'>('slack')
  const [name, setName] = useState('')
  const [url, setUrl] = useState('')
  const [fieldTouched, setFieldTouched] = useState({ name: false, url: false })
  const [deletingChannelId, setDeletingChannelId] = useState<string | null>(null)

  const { data, isLoading, isError } = useNotificationChannels()
  const channels = data?.data ?? []
  const createChannel = useCreateChannel()
  const deleteChannel = useDeleteChannel()

  function handleCreate() {
    setFieldTouched({ name: true, url: true })
    if (!name.trim() || !url.trim()) return
    const config: Record<string, string> = {}
    if (type === 'slack') config.webhook_url = url
    if (type === 'email') config.address = url
    if (type === 'webhook') config.url = url

    createChannel.mutate({ type, name: name.trim(), config }, {
      onSuccess: () => { setCreateOpen(false); setName(''); setUrl(''); setFieldTouched({ name: false, url: false }) },
      // On error: keep dialog open so user can retry
    })
  }

  function handleDialogClose(open: boolean) {
    if (!open) { setFieldTouched({ name: false, url: false }) }
    setCreateOpen(open)
  }

  return (
    <SectionCard title="Benachrichtigungskanäle" icon={Bell}>
      <div className="space-y-2">
        {isLoading && (
          <div className="flex items-center justify-center h-12">
            <div className="w-4 h-4 border-2 border-brand border-t-transparent rounded-full animate-spin" />
          </div>
        )}
        {isError && <p className="text-xs text-secondary">Nicht ladbar — Admin-Rechte erforderlich.</p>}
        {!isLoading && !isError && channels.length === 0 && (
          <p className="text-xs text-secondary">Noch keine Kanäle eingerichtet.</p>
        )}
        {!isLoading && !isError && channels.map((ch) => {
          const Icon = CHANNEL_ICONS[ch.type] ?? Globe
          return (
            <div key={ch.id} className="flex items-center justify-between py-2 px-3 rounded-lg bg-surface2">
              <div className="flex items-center gap-2">
                <Icon className="w-3.5 h-3.5 text-secondary" />
                <div>
                  <div className="text-xs font-medium text-primary">{ch.name}</div>
                  <div className="text-[11px] text-secondary">{CHANNEL_LABELS[ch.type]}</div>
                </div>
              </div>
              <div className="flex items-center gap-1.5">
                <Badge variant={ch.enabled ? 'success' : 'secondary'} className="text-[10px]">
                  {ch.enabled ? 'Aktiv' : 'Inaktiv'}
                </Badge>
                <button
                  onClick={() => {
                    setDeletingChannelId(ch.id)
                    deleteChannel.mutate(ch.id, { onSettled: () => setDeletingChannelId(null) })
                  }}
                  disabled={deletingChannelId === ch.id}
                  className={cn('p-1 rounded text-secondary hover:text-red-500 hover:bg-red-500/10 transition-colors', deletingChannelId === ch.id && 'opacity-50')}
                >
                  <Trash2 className="w-3 h-3" />
                </button>
              </div>
            </div>
          )
        })}
        <div className="pt-1">
          <Button size="sm" variant="outline" onClick={() => setCreateOpen(true)} className="h-7 text-xs">
            <Plus className="w-3 h-3 mr-1" />
            Kanal hinzufügen
          </Button>
        </div>
      </div>

      <Dialog open={createOpen} onOpenChange={handleDialogClose}>
        <DialogContent>
          <DialogHeader><DialogTitle>Benachrichtigungskanal hinzufügen</DialogTitle></DialogHeader>
          <div className="space-y-3 py-2">
            <div className="space-y-1.5">
              <Label>Typ</Label>
              <Select value={type} onValueChange={(v) => setType(v as typeof type)}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="slack">Slack Webhook</SelectItem>
                  <SelectItem value="email">E-Mail</SelectItem>
                  <SelectItem value="webhook">Webhook (HTTP POST)</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1.5">
              <Label>Name</Label>
              <Input
                placeholder="z.B. Security-Team Slack"
                value={name}
                onChange={(e) => setName(e.target.value)}
                onBlur={() => setFieldTouched((t) => ({ ...t, name: true }))}
                aria-invalid={fieldTouched.name && !name.trim()}
              />
              {fieldTouched.name && !name.trim() && (
                <p className="text-xs text-destructive mt-1">Name ist erforderlich.</p>
              )}
            </div>
            <div className="space-y-1.5">
              <Label>{type === 'email' ? 'E-Mail-Adresse' : 'URL'}</Label>
              <Input
                placeholder={type === 'slack' ? 'https://hooks.slack.com/…' : type === 'email' ? 'team@example.com' : 'https://webhook.example.com'}
                value={url}
                onChange={(e) => setUrl(e.target.value)}
                onBlur={() => setFieldTouched((t) => ({ ...t, url: true }))}
                aria-invalid={fieldTouched.url && !url.trim()}
              />
              {fieldTouched.url && !url.trim() && (
                <p className="text-xs text-destructive mt-1">{type === 'email' ? 'E-Mail-Adresse ist erforderlich.' : 'URL ist erforderlich.'}</p>
              )}
            </div>
          </div>
          {createChannel.isError && (
            <p className="text-xs text-red-500 px-1">Fehler beim Erstellen des Kanals. Bitte erneut versuchen.</p>
          )}
          <DialogFooter>
            <Button variant="outline" onClick={() => handleDialogClose(false)}>Abbrechen</Button>
            <Button onClick={handleCreate} disabled={createChannel.isPending}>
              {createChannel.isPending ? 'Wird gespeichert…' : 'Hinzufügen'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </SectionCard>
  )
}

// ─── Server Info ──────────────────────────────────────────────────────────────

function ServerSection() {
  return (
    <SectionCard title="Server" icon={Server}>
      <div className="space-y-1.5 text-xs text-secondary">
        {[
          ['API-Port', '8080 (Standard)'],
          ['Datenbank', 'PostgreSQL 16'],
          ['Queue', 'Redis / Valkey 7'],
          ['Verschlüsselung', 'AES-256-GCM'],
          ['Auth-Token', 'Paseto v4 (local)'],
        ].map(([k, v]) => (
          <div key={k} className="flex justify-between py-1.5 px-3 rounded-lg bg-surface2">
            <span className="text-secondary">{k}</span>
            <span className="text-primary font-medium">{v}</span>
          </div>
        ))}
        <p className="pt-1">Alle Konfigurationswerte werden über Umgebungsvariablen gesetzt.</p>
      </div>
    </SectionCard>
  )
}

// ─── Data Export ─────────────────────────────────────────────────────────────

function DataExportSection() {
  const { exportData, isLoading, error } = useExportData()

  return (
    <SectionCard title="Datenschutz &amp; Export" icon={ShieldCheck}>
      <div className="space-y-3">
        <p className="text-xs text-secondary leading-relaxed">
          Exportieren Sie alle Ihre Compliance-Daten als ZIP-Archiv (JSON-Format).
          Enthält alle Controls, Risiken, Richtlinien, Nachweise, DSGVO-Dokumente und mehr.
        </p>
        <Button
          size="sm"
          variant="outline"
          className="h-7 text-xs"
          onClick={exportData}
          disabled={isLoading}
        >
          {isLoading ? (
            <>
              <div className="w-3 h-3 border-2 border-current border-t-transparent rounded-full animate-spin mr-1.5" />
              Wird exportiert…
            </>
          ) : (
            <>
              <Download className="w-3 h-3 mr-1.5" />
              Daten exportieren
            </>
          )}
        </Button>
        {error && (
          <p className="text-[11px] text-red-500">{error}</p>
        )}
        <p className="text-[11px] text-secondary">
          Alle Daten verbleiben lokal — kein Cloud-Upload. DSGVO Art. 20 (Datenportabilität).
        </p>
      </div>
    </SectionCard>
  )
}

// ─── Audit Report ─────────────────────────────────────────────────────────────

function AuditReportSection() {
  const { generate, isGenerating, error } = useAuditReport()

  return (
    <SectionCard title="Audit-Bericht" icon={FileText}>
      <div className="space-y-3">
        <p className="text-xs text-secondary leading-relaxed">
          Generiert einen vollständigen Compliance-Auditbericht als PDF — mit allen
          Framework-Scores, Controls, Risiken, Vorfällen, Richtlinien und CAPAs.
          Bereit für externe Prüfer ohne manuelle Aufbereitung.
        </p>
        <Button
          size="sm"
          onClick={generate}
          disabled={isGenerating}
          className="h-7 text-xs gap-1.5"
        >
          {isGenerating ? (
            <>
              <div className="w-3 h-3 border-2 border-current border-t-transparent rounded-full animate-spin" />
              Wird erstellt…
            </>
          ) : (
            <>
              <FileText className="w-3 h-3" />
              Audit-Bericht generieren
            </>
          )}
        </Button>
        {/* Show ProGate upgrade prompt for Community users */}
        <ProGate error={error instanceof FeatureLockedError ? error : null}>{''}</ProGate>

        {/* Show generic error for other failures */}
        {error instanceof Error && !(error instanceof FeatureLockedError) && (
          <p className="text-[11px] text-red-500">{error.message}</p>
        )}
        <p className="text-[11px] text-secondary">
          Der Bericht enthält alle Compliance-Daten der Organisation — keine Cloud-Übertragung.
        </p>
      </div>
    </SectionCard>
  )
}

// ─── Staging Release ─────────────────────────────────────────────────────────

function StagingSection() {
  const [confirming, setConfirming] = useState(false)
  const [result, setResult] = useState<'idle' | 'ok' | 'err'>('idle')

  const { data: stagingInfo } = useQuery({
    queryKey: ['admin', 'staging', 'info'],
    queryFn: () => apiFetch<{ staging: boolean }>('/admin/staging/info'),
    retry: false,
    staleTime: Infinity,
  })

  const promote = useMutation<unknown, Error>({
    mutationFn: () => apiFetch('/admin/staging/promote', { method: 'POST' }),
    onSuccess: () => { setResult('ok'); setConfirming(false) },
    onError: () => { setResult('err'); setConfirming(false) },
  })

  if (!stagingInfo?.staging) return null

  return (
    <div>
      <h3 className="text-xs font-semibold text-secondary uppercase tracking-wider mb-3">Staging</h3>
      <div className="max-w-sm">
        <SectionCard title="Zur Demo promoten" icon={Rocket}>
          <div className="space-y-3">
            <p className="text-xs text-secondary leading-relaxed">
              Überträgt den aktuellen Staging-Stand auf die Demo-Umgebung.
              Das Image <code className="font-mono bg-surface2 px-1 rounded">:staging</code> wird
              als <code className="font-mono bg-surface2 px-1 rounded">:latest</code> veröffentlicht
              und die Demo automatisch neu gestartet.
            </p>
            <Button
              size="sm"
              className="h-7 text-xs gap-1.5"
              onClick={() => { setResult('idle'); setConfirming(true) }}
            >
              <Rocket className="w-3 h-3" />
              Zur Demo promoten
            </Button>
            {result === 'ok' && (
              <p className="text-[11px] text-green-600">Workflow gestartet — Demo wird in ca. 2 Minuten aktualisiert.</p>
            )}
            {result === 'err' && (
              <p className="text-[11px] text-red-500">
                {promote.error?.message
                  ? `Fehler: ${promote.error.message}`
                  : 'Fehler beim Auslösen des Workflows. GitHub-Token prüfen.'}
              </p>
            )}
          </div>
        </SectionCard>
      </div>

      <Dialog open={confirming} onOpenChange={setConfirming}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Demo wirklich aktualisieren?</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">
            Der aktuelle Staging-Stand wird auf die öffentliche Demo-Umgebung übertragen.
            Dieser Vorgang kann nicht rückgängig gemacht werden.
          </p>
          <DialogFooter>
            <Button variant="outline" onClick={() => setConfirming(false)}>Abbrechen</Button>
            <Button
              onClick={() => promote.mutate()}
              disabled={promote.isPending}
            >
              {promote.isPending ? (
                <><div className="w-3 h-3 border-2 border-current border-t-transparent rounded-full animate-spin mr-1.5" />Wird gestartet…</>
              ) : 'Ja, promoten'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

// ─── Main Page ────────────────────────────────────────────────────────────────

export default function Settings() {
  return (
    <div className="flex flex-col h-full">
      <PageHeader title="Einstellungen" description="Plattform-Konfiguration und Systemverwaltung." />
      <div className="flex-1 p-6 overflow-auto">
        <div className="max-w-5xl space-y-6">
          {/* Row 1: Organisation + Module + Sector + Lizenz */}
          <div>
            <h3 className="text-xs font-semibold text-secondary uppercase tracking-wider mb-3">Plattform</h3>
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
              <OrgSection />
              <ModulesSection />
              <SectorSection />
              <LicenseSection />
            </div>
          </div>

          {/* Row 2: Integrations — interactive, needs more visual weight */}
          <div>
            <h3 className="text-xs font-semibold text-secondary uppercase tracking-wider mb-3">Integrationen</h3>
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
              <SmtpSection />
              <NotificationsSection />
            </div>
          </div>

          {/* Row 3: Data & Privacy export + Audit Report */}
          <div>
            <h3 className="text-xs font-semibold text-secondary uppercase tracking-wider mb-3">Datenschutz &amp; Dokumentation</h3>
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-5 max-w-2xl">
              <DataExportSection />
              <AuditReportSection />
            </div>
          </div>

          {/* Row 4: Trust Center */}
          <div>
            <h3 className="text-xs font-semibold text-secondary uppercase tracking-wider mb-3">Öffentliche Seiten</h3>
            <div className="max-w-sm">
              <SectionCard title="Trust Center" icon={Globe}>
                <p className="text-sm text-muted-foreground mb-3">
                  Öffentliche Compliance-Seite für Kunden, Partner und Auditoren.
                </p>
                <Link to="/settings/trust-center" className="inline-flex items-center gap-1.5 text-sm font-medium text-primary hover:underline">
                  Trust Center konfigurieren <ExternalLink className="h-3.5 w-3.5" />
                </Link>
              </SectionCard>
            </div>
          </div>

          {/* Staging-only: promote to demo — StagingSection renders null on non-staging instances */}
          <StagingSection />

          {/* Row 4: System info — read-only reference, visually de-emphasized */}
          <div>
            <h3 className="text-xs font-semibold text-secondary uppercase tracking-wider mb-3">System</h3>
            <div className="max-w-sm">
              <ServerSection />
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
