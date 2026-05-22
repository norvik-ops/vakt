import { useState, useEffect, useRef } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Spinner } from '../../../components/Spinner'
import { Link } from 'react-router-dom'
import {
  Building2, Layers, Bell, Trash2, Plus, Check, X,
  Webhook, Globe, Mail, Server, MapPin, Download, ShieldCheck, Shield, FileText, ExternalLink, Sparkles, Rocket, Key, Clock, ArrowUpCircle, RefreshCw, Zap, FileBarChart2, Radio,
} from 'lucide-react'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Button } from '../../../components/ui/button'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Badge } from '../../../components/ui/badge'
import { Switch } from '../../../components/ui/switch'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../../../components/ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../../../components/ui/select'
import { useTranslation } from 'react-i18next'
import { apiFetch, FeatureLockedError } from '../../../api/client'
import { useAuthStore } from '../../../shared/stores/auth'
import { cn } from '../../../lib/utils'
import { VAKT_LS_PORTAL_URL } from '../../../lib/constants'
import { useOrgSector, useUpdateOrgSector } from '../../../modules/secvitals/hooks/useOrgSector'
import { useApprovalSetting, useUpdateApprovalSetting } from '../../../modules/secvitals/hooks/useApprovals'
import { SECTOR_LABELS } from '../../../modules/secvitals/types'
import { useExportData } from '../../../hooks/useDataExport'
import { useAuditReport } from '../../../modules/secvitals/hooks/useAuditReport'
import { ProGate } from '../../../shared/components/ProGate'
import { useUpdateCheck } from '../../../shared/hooks/useUpdateCheck'
import { formatLocale } from '../../../shared/utils/locale'

// ─── Retention / Digest hooks (used by DigestToggleSection) ──────────────────

interface RetentionConfig {
  digest_enabled: boolean
  digest_hour: number
}

function useRetentionConfig() {
  return useQuery<RetentionConfig>({
    queryKey: ['retention', 'config'],
    queryFn: () => apiFetch<RetentionConfig>('/retention/config'),
    staleTime: 60_000,
  })
}

function useUpdateDigestEnabled() {
  const qc = useQueryClient()
  return useMutation<RetentionConfig, Error, boolean>({
    mutationFn: (enabled) =>
      apiFetch<RetentionConfig>('/retention/config', {
        method: 'PUT',
        body: JSON.stringify({ digest_enabled: enabled }),
      }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['retention'] }),
  })
}

// ─── Types ───────────────────────────────────────────────────────────────────

interface OrgSecurity {
  require_mfa: boolean
}

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

function useOrgSecurity() {
  return useQuery<OrgSecurity>({
    queryKey: ['admin', 'org', 'security'],
    queryFn: () => apiFetch<OrgSecurity>('/admin/org/security'),
    retry: false,
  })
}

function useUpdateOrgSecurity() {
  const qc = useQueryClient()
  return useMutation<void, Error, OrgSecurity>({
    mutationFn: (input) =>
      apiFetch<void>('/admin/org/security', {
        method: 'PUT',
        body: JSON.stringify(input),
      }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['admin', 'org', 'security'] }),
  })
}

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
  ai_advisor: 'KI-Berater (legacy — seit v0.6.x Community)',
  audit_pdf: 'Audit-PDF Export',
  sso: 'SSO (OIDC/SAML)',
  api_access: 'API-Zugang',
  secreflex_advanced: 'Vakt Aware Pro',
  secpulse_advanced: 'Vakt Scan Pro',
  granular_permissions: 'Granulare Modul-Berechtigungen pro Benutzer',
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
  const { t } = useTranslation()
  const { data: lic, isLoading } = useLicense()
  const activate = useActivateLicense()
  const [licKey, setLicKey] = useState('')
  const [activateSuccess, setActivateSuccess] = useState(false)
  const licTimerRef = useRef<ReturnType<typeof setTimeout>>()

  useEffect(() => () => clearTimeout(licTimerRef.current), [])

  if (isLoading) return (
    <SectionCard title={t('settingsPage.licenseTitle')} icon={Sparkles}>
      <div className="h-16 flex items-center justify-center">
        <Spinner size="sm" />
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
    <SectionCard title={t('settingsPage.licenseTitle')} icon={Sparkles}>
      <div className="space-y-4">
        {lic?.revoked && (
          <div className="text-sm text-amber-700 bg-amber-50 border border-amber-200 rounded p-3">
            {t('settingsPage.licenseRevoked')}
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
            Gültig bis {new Date(lic.expires_at).toLocaleDateString(formatLocale())}
          </p>
        )}

        {lic?.expires_at && daysUntilExpiry(lic.expires_at) < 30 && (
          <div className="text-sm text-amber-600 bg-amber-50 border border-amber-200 rounded p-2">
            {daysUntilExpiry(lic.expires_at) === 0
              ? t('settingsPage.licenseExpired')
              : t('settingsPage.licenseExpiringSoon', { days: daysUntilExpiry(lic.expires_at) })}
          </div>
        )}

        {isPro && !lic?.demo && (
          <a href={VAKT_LS_PORTAL_URL} target="_blank" rel="noopener noreferrer" className="text-sm text-primary underline">
            {t('settingsPage.manageSubscription')}
          </a>
        )}

        {!isPro && (
          <div className="space-y-1.5">
            <p className="text-xs text-secondary">{t('settingsPage.proFeatures')}</p>
            <ul className="text-xs text-secondary space-y-0.5 list-none">
              {[
                'Rollen: Admin, Analyst, Viewer, Auditor',
                'Granulare Modul-Berechtigungen pro Benutzer',
                'TISAX, DORA, EU AI Act, CRA',
                'KI-Berater, Audit-PDF Export, SSO, API-Zugang',
              ].map((f) => (
                <li key={f} className="flex items-center gap-1.5">
                  <span className="text-brand">›</span>
                  {f}
                </li>
              ))}
            </ul>
            <span className="inline-flex items-center gap-1.5 text-xs font-semibold text-brand">
              <Clock className="w-3.5 h-3.5" />
              Vakt Pro — demnächst verfügbar
            </span>
          </div>
        )}

        {/* Pro-Key activation */}
        <div className="pt-1 border-t border-border space-y-2">
          <Label className="text-xs">{t('settingsPage.proKeyActivate')}</Label>
          <div className="flex gap-2">
            <Input
              value={licKey}
              onChange={(e) => { setLicKey(e.target.value); setActivateSuccess(false) }}
              placeholder={t('settingsPage.proKeyPlaceholder')}
              className="h-8 text-xs font-mono flex-1"
            />
            <Button
              size="sm"
              className="h-8 text-xs gap-1"
              onClick={handleActivate}
              disabled={!licKey.trim() || activate.isPending}
            >
              <Key className="w-3 h-3" />
              {activate.isPending ? t('settingsPage.activating') : t('settingsPage.activate')}
            </Button>
          </div>
          {activateSuccess && (
            <p className="text-[11px] text-green-600 dark:text-green-400">{t('settingsPage.keyActivated')}</p>
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
  const { t } = useTranslation()
  const { user } = useAuthStore()
  const { data: security, isLoading: secLoading } = useOrgSecurity()
  const updateSecurity = useUpdateOrgSecurity()
  const [mfaChecked, setMfaChecked] = useState(false)

  const { data: approvalSetting, isLoading: approvalLoading } = useApprovalSetting()
  const updateApprovalSetting = useUpdateApprovalSetting()
  const [approvalChecked, setApprovalChecked] = useState(false)

  useEffect(() => {
    if (security) setMfaChecked(security.require_mfa)
  }, [security])

  useEffect(() => {
    if (approvalSetting) setApprovalChecked(approvalSetting.approval_required)
  }, [approvalSetting])

  const isAdmin = user?.roles?.includes('Admin') ?? false

  function handleMfaToggle(value: boolean) {
    setMfaChecked(value)
    updateSecurity.mutate({ require_mfa: value })
  }

  function handleApprovalToggle(value: boolean) {
    setApprovalChecked(value)
    updateApprovalSetting.mutate(value)
  }

  return (
    <SectionCard title={t('settingsPage.orgSectionTitle')} icon={Building2}>
      <div className="space-y-3">
        <div className="space-y-1.5">
          <Label className="text-xs">{t('settingsPage.labelAdmin')}</Label>
          <Input value={user?.email ?? '—'} readOnly className="bg-surface2 h-8 text-sm" />
        </div>
        <div className="space-y-1.5">
          <Label className="text-xs">{t('settingsPage.labelDisplayName')}</Label>
          <Input value={user?.display_name ?? '—'} readOnly className="bg-surface2 h-8 text-sm" />
        </div>

        {isAdmin && (
          <div className="pt-2 border-t border-border space-y-4">
            {/* MFA toggle */}
            <div>
              {secLoading ? (
                <div className="flex items-center justify-center h-8">
                  <Spinner size="sm" />
                </div>
              ) : (
                <div className="flex items-start justify-between gap-4">
                  <div className="space-y-1">
                    <p className="text-sm font-medium text-primary">{t('settingsPage.mfaTitle')}</p>
                    <p className="text-[11px] text-secondary leading-relaxed">
                      {t('settingsPage.mfaDesc')}
                    </p>
                  </div>
                  <Switch
                    checked={mfaChecked}
                    onCheckedChange={handleMfaToggle}
                    disabled={updateSecurity.isPending}
                    aria-label={t('settingsPage.mfaTitle')}
                  />
                </div>
              )}
              {updateSecurity.isError && (
                <p className="text-[11px] text-red-500 mt-1">{t('settingsPage.saveError')}</p>
              )}
              {updateSecurity.isSuccess && (
                <p className="text-[11px] text-green-600 dark:text-green-400 mt-1">
                  {mfaChecked ? t('settingsPage.mfaEnabled') : t('settingsPage.mfaDisabled')}
                </p>
              )}
            </div>

            {/* 4-Augen approval toggle */}
            <div className="border-t border-border pt-4">
              {approvalLoading ? (
                <div className="flex items-center justify-center h-8">
                  <Spinner size="sm" />
                </div>
              ) : (
                <div className="flex items-start justify-between gap-4">
                  <div className="space-y-1">
                    <p className="text-sm font-medium text-primary">{t('settingsPage.approvalTitle')}</p>
                    <p className="text-[11px] text-secondary leading-relaxed">
                      {t('settingsPage.approvalDesc')}
                    </p>
                  </div>
                  <Switch
                    checked={approvalChecked}
                    onCheckedChange={handleApprovalToggle}
                    disabled={updateApprovalSetting.isPending}
                    aria-label={t('settingsPage.approvalTitle')}
                  />
                </div>
              )}
              {updateApprovalSetting.isError && (
                <p className="text-[11px] text-red-500 mt-1">{t('settingsPage.saveError')}</p>
              )}
              {updateApprovalSetting.isSuccess && (
                <p className="text-[11px] text-green-600 dark:text-green-400 mt-1">
                  {approvalChecked ? t('settingsPage.approvalEnabled') : t('settingsPage.approvalDisabled')}
                </p>
              )}
            </div>
          </div>
        )}
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
  const { t } = useTranslation()
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
    <SectionCard title={t('settingsPage.sectorTitle')} icon={MapPin}>
      {lic !== undefined && !isPro ? (
        <div className="flex items-start gap-4">
          <div className="mt-0.5 p-2 rounded-lg bg-brand/10 shrink-0">
            <Sparkles className="w-4 h-4 text-brand" />
          </div>
          <div>
            <p className="font-semibold text-primary text-sm mb-1">{t('settingsPage.sectorProFeature')}</p>
            <p className="text-secondary text-sm leading-relaxed mb-2">
              {t('settingsPage.sectorProDesc')}
            </p>
            <span className="inline-flex items-center gap-1.5 text-xs font-semibold text-brand">
              <Clock className="w-3.5 h-3.5" />
              {t('settingsPage.comingSoon')}
            </span>
          </div>
        </div>
      ) : (
        <div className="space-y-3">
          <div className="space-y-1.5">
            <Label className="text-xs">{t('settingsPage.labelSector')}</Label>
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
            <p className="text-[11px] text-secondary">{t('settingsPage.sectorHint')}</p>
          </div>
          <div className="space-y-1.5">
            <Label className="text-xs">{t('settingsPage.labelFederalState')}</Label>
            <Select value={federalState} onValueChange={setFederalState}>
              <SelectTrigger className="h-8 text-sm" data-testid="federal-state-select">
                <SelectValue placeholder={t('settingsPage.federalStatePlaceholder')} />
              </SelectTrigger>
              <SelectContent>
                {FEDERAL_STATES.map((s) => (
                  <SelectItem key={s} value={s}>{s}</SelectItem>
                ))}
              </SelectContent>
            </Select>
            <p className="text-[11px] text-secondary">{t('settingsPage.federalStateHint')}</p>
          </div>
          <Button
            size="sm"
            className="h-7 text-xs"
            onClick={handleSave}
            disabled={!isDirty || update.isPending}
            data-testid="sector-save-btn"
          >
            {update.isPending ? t('common.saving') : t('common.save')}
          </Button>
          {update.isSuccess && (
            <p className="text-[11px] text-green-600 dark:text-green-400">{t('settingsPage.saved')}</p>
          )}
          {update.isError && (
            <p className="text-[11px] text-red-500">{t('settingsPage.saveError')}</p>
          )}
        </div>
      )}
    </SectionCard>
  )
}

// ─── Module Status ────────────────────────────────────────────────────────────

function ModulesSection() {
  const { t } = useTranslation()
  const { data, isLoading, isError } = useModules()
  const modules = data?.data ?? []

  return (
    <SectionCard title={t('settingsPage.modulesTitle')} icon={Layers}>
      {isLoading && (
        <div className="flex items-center justify-center h-16">
          <Spinner size="sm" />
        </div>
      )}
      {isError && (
        <p className="text-xs text-secondary">{t('settingsPage.modulesNotLoadable')}</p>
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
                  ? <Badge variant="success" className="text-[10px] shrink-0"><Check className="w-2.5 h-2.5 mr-1" />{t('settingsPage.moduleEnabled')}</Badge>
                  : <Badge variant="secondary" className="text-[10px] shrink-0"><X className="w-2.5 h-2.5 mr-1" />{t('settingsPage.moduleDisabled')}</Badge>
                }
              </div>
            )
          })}
          <p className="text-[11px] text-secondary pt-1">
            {t('settingsPage.modulesEnvHint')}
          </p>
        </div>
      )}
    </SectionCard>
  )
}

// ─── Weekly Digest Toggle ────────────────────────────────────────────────────

function DigestToggleSection() {
  const { t } = useTranslation()
  const { data, isLoading } = useRetentionConfig()
  const update = useUpdateDigestEnabled()
  const [checked, setChecked] = useState(false)

  useEffect(() => {
    if (data) setChecked(data.digest_enabled)
  }, [data])

  function handleToggle(value: boolean) {
    setChecked(value)
    update.mutate(value)
  }

  return (
    <SectionCard title={t('settingsPage.digestTitle')} icon={Mail}>
      <div className="space-y-3">
        {isLoading ? (
          <div className="flex items-center justify-center h-10">
            <Spinner size="sm" />
          </div>
        ) : (
          <div className="flex items-start justify-between gap-4">
            <div className="space-y-1">
              <p className="text-sm font-medium text-primary">{t('settingsPage.digestToggleTitle')}</p>
              <p className="text-[11px] text-secondary leading-relaxed">
                {t('settingsPage.digestToggleDesc')}
              </p>
            </div>
            <Switch
              checked={checked}
              onCheckedChange={handleToggle}
              disabled={update.isPending}
              aria-label={t('settingsPage.digestToggleTitle')}
            />
          </div>
        )}
        {update.isError && (
          <p className="text-[11px] text-red-500">{t('settingsPage.saveError')}</p>
        )}
        {update.isSuccess && (
          <p className="text-[11px] text-green-600 dark:text-green-400">
            {checked ? t('settingsPage.digestEnabled') : t('settingsPage.digestDisabled')}
          </p>
        )}
        <p className="text-[11px] text-secondary">
          {t('settingsPage.digestSmtpHint')}
        </p>
      </div>
    </SectionCard>
  )
}

// ─── E-Mail / SMTP ────────────────────────────────────────────────────────────

function SmtpSection() {
  const { t } = useTranslation()
  return (
    <SectionCard title={t('settingsPage.smtpTitle')} icon={Mail}>
      <div className="space-y-3">
        <div className="space-y-1.5">
          <Label className="text-xs">{t('settingsPage.smtpHostLabel')}</Label>
          <Input
            placeholder="smtp.example.com"
            readOnly
            className="bg-surface2 h-8 text-sm text-secondary"
            value={t('settingsPage.smtpConfiguredViaEnv')}
          />
        </div>
        <div className="space-y-1.5">
          <Label className="text-xs">{t('settingsPage.smtpPortLabel')}</Label>
          <Input
            placeholder="587"
            readOnly
            className="bg-surface2 h-8 text-sm text-secondary"
            value={t('settingsPage.smtpConfiguredViaEnv')}
          />
        </div>
        <div className="rounded-lg bg-surface2 p-3 text-[11px] text-secondary space-y-1 leading-relaxed">
          <p className="font-medium text-primary">{t('settingsPage.smtpEnvHint')}</p>
          <code className="block font-mono">VAKT_SMTP_HOST=smtp.example.com</code>
          <code className="block font-mono">VAKT_SMTP_PORT=587</code>
          <code className="block font-mono">VAKT_SMTP_USER=user@example.com</code>
          <code className="block font-mono">VAKT_SMTP_PASS=geheimespasswort</code>
          <p className="pt-1">{t('settingsPage.smtpUsage')}</p>
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
  const { t } = useTranslation()
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
    <SectionCard title={t('settingsPage.notificationsTitle')} icon={Bell}>
      <div className="space-y-2">
        {isLoading && (
          <div className="flex items-center justify-center h-12">
            <Spinner size="sm" />
          </div>
        )}
        {isError && <p className="text-xs text-secondary">{t('settingsPage.notificationsNotLoadable')}</p>}
        {!isLoading && !isError && channels.length === 0 && (
          <p className="text-xs text-secondary">{t('settingsPage.noChannels')}</p>
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
                  {ch.enabled ? t('settingsPage.channelActive') : t('settingsPage.channelInactive')}
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
            {t('settingsPage.addChannel')}
          </Button>
        </div>
      </div>

      <Dialog open={createOpen} onOpenChange={handleDialogClose}>
        <DialogContent>
          <DialogHeader><DialogTitle>{t('settingsPage.addChannelTitle')}</DialogTitle></DialogHeader>
          <div className="space-y-3 py-2">
            <div className="space-y-1.5">
              <Label>{t('settingsPage.channelType')}</Label>
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
              <Label>{t('settingsPage.channelName')}</Label>
              <Input
                placeholder={t('settingsPage.channelNamePlaceholder')}
                value={name}
                onChange={(e) => setName(e.target.value)}
                onBlur={() => setFieldTouched((prev) => ({ ...prev, name: true }))}
                aria-invalid={fieldTouched.name && !name.trim()}
              />
              {fieldTouched.name && !name.trim() && (
                <p className="text-xs text-destructive mt-1">{t('settingsPage.channelNameRequired')}</p>
              )}
            </div>
            <div className="space-y-1.5">
              <Label>{type === 'email' ? t('settingsPage.channelEmail') : t('settingsPage.channelUrl')}</Label>
              <Input
                placeholder={type === 'slack' ? 'https://hooks.slack.com/…' : type === 'email' ? 'team@example.com' : 'https://webhook.example.com'}
                value={url}
                onChange={(e) => setUrl(e.target.value)}
                onBlur={() => setFieldTouched((prev) => ({ ...prev, url: true }))}
                aria-invalid={fieldTouched.url && !url.trim()}
              />
              {fieldTouched.url && !url.trim() && (
                <p className="text-xs text-destructive mt-1">{type === 'email' ? t('settingsPage.channelEmailRequired') : t('settingsPage.channelUrlRequired')}</p>
              )}
            </div>
          </div>
          {createChannel.isError && (
            <p className="text-xs text-red-500 px-1">{t('settingsPage.channelError')}</p>
          )}
          <DialogFooter>
            <Button variant="outline" onClick={() => handleDialogClose(false)}>{t('common.cancel')}</Button>
            <Button onClick={handleCreate} disabled={createChannel.isPending}>
              {createChannel.isPending ? t('settingsPage.channelSaving') : t('settingsPage.channelAdd')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </SectionCard>
  )
}

// ─── SIEM Integration (S21-7, S21-8) ─────────────────────────────────────────

interface OrgSIEMConfig {
  enabled: boolean
  adapter: 'splunk_hec' | 'elastic' | 'webhook'
  endpoint: string
  token: string // write-only: comes back as "***" or ""
}

function useOrgSIEMConfig() {
  return useQuery<OrgSIEMConfig>({
    queryKey: ['admin', 'org', 'siem'],
    queryFn: () => apiFetch<OrgSIEMConfig>('/admin/org/siem'),
    retry: false,
  })
}

function useUpdateSIEMConfig() {
  const qc = useQueryClient()
  return useMutation<void, Error, Partial<OrgSIEMConfig>>({
    mutationFn: (input) =>
      apiFetch<void>('/admin/org/siem', {
        method: 'PUT',
        body: JSON.stringify(input),
      }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['admin', 'org', 'siem'] }),
  })
}

function useTestSIEM() {
  return useMutation<void, Error>({
    mutationFn: () => apiFetch<void>('/admin/org/siem/test', { method: 'POST' }),
  })
}

function SIEMSection() {
  const { data, isLoading, error: queryError } = useOrgSIEMConfig()
  const update = useUpdateSIEMConfig()
  const test = useTestSIEM()

  const [enabled, setEnabled] = useState(false)
  const [adapter, setAdapter] = useState<OrgSIEMConfig['adapter']>('webhook')
  const [endpoint, setEndpoint] = useState('')
  const [token, setToken] = useState('')
  const [saved, setSaved] = useState(false)
  const [testResult, setTestResult] = useState<'idle' | 'ok' | 'err'>('idle')
  const [testError, setTestError] = useState('')

  useEffect(() => {
    if (data) {
      setEnabled(data.enabled)
      setAdapter(data.adapter)
      setEndpoint(data.endpoint)
      // Don't pre-fill the token input — it's write-only
    }
  }, [data])

  const isProLocked = queryError instanceof FeatureLockedError

  function handleSave() {
    update.mutate(
      { enabled, adapter, endpoint, token: token || '' },
      {
        onSuccess: () => {
          setSaved(true)
          setToken('')
          setTimeout(() => setSaved(false), 2500)
        },
      },
    )
  }

  function handleTest() {
    setTestResult('idle')
    setTestError('')
    test.mutate(undefined, {
      onSuccess: () => setTestResult('ok'),
      onError: (err) => { setTestResult('err'); setTestError(err.message) },
    })
  }

  return (
    <SectionCard title="SIEM-Integration" icon={Radio}>
      {isLoading && (
        <div className="flex items-center justify-center h-16">
          <Spinner size="sm" />
        </div>
      )}

      {isProLocked && (
        <div className="flex items-start gap-4">
          <div className="mt-0.5 p-2 rounded-lg bg-brand/10 shrink-0">
            <Sparkles className="w-4 h-4 text-brand" />
          </div>
          <div>
            <p className="font-semibold text-primary text-sm mb-1">
              Pro-Feature
              <span className="ml-2 inline-flex items-center gap-1 text-[10px] font-semibold bg-brand/10 text-brand px-1.5 py-0.5 rounded">Pro</span>
            </p>
            <p className="text-secondary text-sm leading-relaxed mb-2">
              Leite Audit-Log-Einträge automatisch an Splunk, Elasticsearch oder einen generischen Webhook weiter.
              Verfügbar mit Vakt Pro.
            </p>
            <span className="inline-flex items-center gap-1.5 text-xs font-semibold text-brand">
              <Clock className="w-3.5 h-3.5" />
              Vakt Pro erforderlich
            </span>
          </div>
        </div>
      )}

      {!isLoading && !isProLocked && (
        <div className="space-y-4">
          {/* Enable toggle */}
          <div className="flex items-start justify-between gap-4">
            <div className="space-y-1">
              <p className="text-sm font-medium text-primary">SIEM-Weiterleitung aktivieren</p>
              <p className="text-[11px] text-secondary leading-relaxed">
                Audit-Einträge werden alle 5 Minuten an das konfigurierte SIEM weitergeleitet.
              </p>
            </div>
            <Switch
              checked={enabled}
              onCheckedChange={setEnabled}
              aria-label="SIEM aktivieren"
            />
          </div>

          {/* Adapter */}
          <div className="space-y-1.5">
            <Label className="text-xs">Adapter</Label>
            <Select value={adapter} onValueChange={(v) => setAdapter(v as OrgSIEMConfig['adapter'])}>
              <SelectTrigger className="h-8 text-sm">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="splunk_hec">Splunk HEC</SelectItem>
                <SelectItem value="elastic">Elasticsearch</SelectItem>
                <SelectItem value="webhook">Webhook</SelectItem>
              </SelectContent>
            </Select>
          </div>

          {/* Endpoint */}
          <div className="space-y-1.5">
            <Label className="text-xs">Endpunkt-URL</Label>
            <Input
              value={endpoint}
              onChange={(e) => setEndpoint(e.target.value)}
              placeholder={
                adapter === 'splunk_hec'
                  ? 'https://splunk.example.com:8088'
                  : adapter === 'elastic'
                  ? 'https://elastic.example.com:9200'
                  : 'https://webhook.example.com/siem'
              }
              className="h-8 text-sm"
            />
          </div>

          {/* Token */}
          <div className="space-y-1.5">
            <Label className="text-xs">
              {adapter === 'splunk_hec' ? 'HEC Token' : adapter === 'elastic' ? 'API Key' : 'Bearer Token (optional)'}
            </Label>
            <Input
              type="password"
              value={token}
              onChange={(e) => setToken(e.target.value)}
              placeholder={data?.token === '***' ? '••••••••  (gesetzt — leer lassen zum Beibehalten)' : '••••••••'}
              className="h-8 text-sm"
              autoComplete="new-password"
            />
            <p className="text-[11px] text-secondary">
              Leer lassen, um den gespeicherten Token beizubehalten.
            </p>
          </div>

          {/* Actions */}
          <div className="flex items-center gap-2 pt-1">
            <Button
              size="sm"
              onClick={handleSave}
              disabled={update.isPending}
              className="h-8 text-xs"
            >
              {saved ? (
                <><Check className="w-3.5 h-3.5 mr-1" />Gespeichert</>
              ) : update.isPending ? (
                <><Spinner size="sm" />Speichern…</>
              ) : (
                'Speichern'
              )}
            </Button>
            <Button
              size="sm"
              variant="outline"
              onClick={handleTest}
              disabled={test.isPending || !endpoint}
              className="h-8 text-xs"
            >
              {test.isPending ? (
                <><Spinner size="sm" />Testen…</>
              ) : (
                'Test-Event senden'
              )}
            </Button>
          </div>

          {update.isError && (
            <p className="text-[11px] text-red-500">{update.error.message}</p>
          )}
          {testResult === 'ok' && (
            <p className="text-[11px] text-green-600 dark:text-green-400">
              Test-Event erfolgreich gesendet.
            </p>
          )}
          {testResult === 'err' && (
            <p className="text-[11px] text-red-500">Test fehlgeschlagen: {testError}</p>
          )}
        </div>
      )}
    </SectionCard>
  )
}

// ─── AI Model Settings (S32-3 ADR-0024) ──────────────────────────────────────

interface OrgAISettings {
  model_override: string
  base_url_override: string
}

function useOrgAISettings() {
  return useQuery<OrgAISettings>({
    queryKey: ['org-ai-settings'],
    queryFn: () => apiFetch<OrgAISettings>('/admin/org/ai-settings'),
  })
}

function useOllamaModels() {
  return useQuery<{ models: string[] }>({
    queryKey: ['ollama-models'],
    queryFn: () => apiFetch<{ models: string[] }>('/secvitals/ai/models'),
    staleTime: 60_000,
  })
}

function useUpdateOrgAISettings() {
  const qc = useQueryClient()
  return useMutation<void, Error, OrgAISettings>({
    mutationFn: (data) =>
      apiFetch<void>('/admin/org/ai-settings', {
        method: 'PUT',
        body: JSON.stringify(data),
      }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['org-ai-settings'] }),
  })
}

function AISettingsSection() {
  const { data: settings, isLoading } = useOrgAISettings()
  const { data: modelsData } = useOllamaModels()
  const { data: lic } = useQuery<LicenseInfo>({ queryKey: ['license'], queryFn: () => apiFetch<LicenseInfo>('/license') })
  const update = useUpdateOrgAISettings()

  const [model, setModel] = useState('')
  const [baseURL, setBaseURL] = useState('')
  const [saved, setSaved] = useState(false)

  useEffect(() => {
    if (settings) {
      setModel(settings.model_override)
      setBaseURL(settings.base_url_override)
    }
  }, [settings])

  const handleSave = () => {
    update.mutate(
      { model_override: model, base_url_override: baseURL },
      { onSuccess: () => { setSaved(true); setTimeout(() => setSaved(false), 2000) } },
    )
  }

  const ollamaModels = modelsData?.models ?? []
  const isPro = lic?.features?.includes('ai_advisor') ?? false

  return (
    <SectionCard title="KI-Modell" icon={Sparkles}>
      {isLoading ? (
        <Spinner size="sm" />
      ) : (
        <div className="space-y-4">
          <div className="space-y-1.5">
            <Label className="text-xs">Modell</Label>
            {ollamaModels.length > 0 ? (
              <Select value={model} onValueChange={setModel}>
                <SelectTrigger className="h-8 text-sm">
                  <SelectValue placeholder="System-Standard verwenden" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="">System-Standard</SelectItem>
                  {ollamaModels.map((m) => (
                    <SelectItem key={m} value={m}>{m}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            ) : (
              <Input
                value={model}
                onChange={(e) => setModel(e.target.value)}
                placeholder="System-Standard (aus VAKT_AI_MODEL)"
                className="h-8 text-sm"
              />
            )}
            <p className="text-[11px] text-secondary">
              Leer = ENV-Wert <code className="font-mono">VAKT_AI_MODEL</code> wird verwendet.
            </p>
          </div>

          {isPro && (
            <div className="space-y-1.5">
              <Label className="text-xs">Custom Endpunkt (BYOK) <Badge variant="secondary" className="text-[10px] ml-1">Pro</Badge></Label>
              <Input
                value={baseURL}
                onChange={(e) => setBaseURL(e.target.value)}
                placeholder="https://api.openai.com/v1"
                className="h-8 text-sm"
              />
              <p className="text-[11px] text-secondary">
                Bei Extern-Endpunkten trägt der Betreiber die DSGVO-Verantwortung
                (Art. 28 AVV empfohlen).
              </p>
            </div>
          )}

          <Button
            size="sm"
            onClick={handleSave}
            disabled={update.isPending}
            className="h-8 text-xs"
          >
            {saved ? (
              <><Check className="w-3.5 h-3.5 mr-1" />Gespeichert</>
            ) : update.isPending ? (
              <><Spinner size="sm" />Speichern…</>
            ) : (
              'Speichern'
            )}
          </Button>
          {update.isError && (
            <p className="text-xs text-destructive">{update.error.message}</p>
          )}
        </div>
      )}
    </SectionCard>
  )
}

// ─── SAML Direct SP Setup (S21-1/2) ──────────────────────────────────────────

interface OrgSAMLConfig {
  org_id: string
  entity_id: string
  acs_url: string
  idp_metadata: string
  cert_pem: string
  enabled: boolean
}

function useOrgSAMLConfig() {
  return useQuery<OrgSAMLConfig>({
    queryKey: ['org-saml-config'],
    queryFn: () => apiFetch<OrgSAMLConfig>('/admin/org/saml-config'),
  })
}

function useUpdateSAMLConfig() {
  const qc = useQueryClient()
  return useMutation<void, Error, Omit<OrgSAMLConfig, 'org_id' | 'cert_pem'>>({
    mutationFn: (data) =>
      apiFetch<void>('/admin/org/saml-config', {
        method: 'PUT',
        body: JSON.stringify(data),
      }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['org-saml-config'] }),
  })
}

function useRegenerateSAMLCert() {
  const qc = useQueryClient()
  return useMutation<{ cert_pem: string }, Error>({
    mutationFn: () =>
      apiFetch<{ cert_pem: string }>('/admin/org/saml-config/regenerate-cert', { method: 'POST' }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['org-saml-config'] }),
  })
}

function SAMLSetupSection() {
  const { data, isLoading } = useOrgSAMLConfig()
  const update = useUpdateSAMLConfig()
  const regen = useRegenerateSAMLCert()

  const [entityID, setEntityID] = useState('')
  const [acsURL, setACSURL] = useState('')
  const [idpMeta, setIdpMeta] = useState('')
  const [enabled, setEnabled] = useState(false)
  const [saved, setSaved] = useState(false)
  const [regenDone, setRegenDone] = useState(false)

  useEffect(() => {
    if (data) {
      setEntityID(data.entity_id ?? '')
      setACSURL(data.acs_url ?? '')
      setIdpMeta(data.idp_metadata ?? '')
      setEnabled(data.enabled ?? false)
    }
  }, [data])

  const handleSave = () => {
    update.mutate(
      { entity_id: entityID, acs_url: acsURL, idp_metadata: idpMeta, enabled },
      { onSuccess: () => { setSaved(true); setTimeout(() => setSaved(false), 2000) } },
    )
  }

  const handleRegen = () => {
    regen.mutate(undefined, {
      onSuccess: () => { setRegenDone(true); setTimeout(() => setRegenDone(false), 3000) },
    })
  }

  const metadataURL = window.location.origin + '/api/v1/auth/saml/metadata'
  const initiateURL = window.location.origin + '/api/v1/auth/saml/initiate'

  return (
    <SectionCard title="SAML 2.0 (Single Sign-On)" icon={Shield}>
      {isLoading ? (
        <Spinner size="sm" />
      ) : (
        <div className="space-y-4">
          <div className="flex items-center gap-2">
            <Switch checked={enabled} onCheckedChange={setEnabled} id="saml-enabled" />
            <Label htmlFor="saml-enabled" className="text-xs">SAML aktiviert</Label>
          </div>

          {/* SP Endpoint URLs (read-only) */}
          <div className="rounded-md bg-muted/40 p-3 space-y-2 text-xs">
            <p className="font-medium text-secondary uppercase tracking-wider text-[10px]">SP-Endpunkte (in IdP eintragen)</p>
            <div className="space-y-1">
              <Label className="text-[10px] text-secondary">Metadata URL</Label>
              <code className="block font-mono text-[11px] break-all">{metadataURL}</code>
            </div>
            <div className="space-y-1">
              <Label className="text-[10px] text-secondary">ACS URL / Single Sign-On URL</Label>
              <code className="block font-mono text-[11px] break-all">{initiateURL.replace('/initiate', '/acs')}</code>
            </div>
          </div>

          <div className="space-y-1.5">
            <Label className="text-xs">SP Entity ID</Label>
            <Input
              value={entityID}
              onChange={(e) => setEntityID(e.target.value)}
              placeholder={`${window.location.origin}/saml`}
              className="h-8 text-sm font-mono"
            />
          </div>

          <div className="space-y-1.5">
            <Label className="text-xs">ACS URL</Label>
            <Input
              value={acsURL}
              onChange={(e) => setACSURL(e.target.value)}
              placeholder={`${window.location.origin}/api/v1/auth/saml/acs`}
              className="h-8 text-sm font-mono"
            />
          </div>

          <div className="space-y-1.5">
            <Label className="text-xs">IdP Metadata XML</Label>
            <textarea
              value={idpMeta}
              onChange={(e) => setIdpMeta(e.target.value)}
              placeholder='<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" …'
              className="w-full h-28 rounded-md border border-input bg-transparent px-3 py-2 text-xs font-mono resize-y focus:outline-none focus:ring-1 focus:ring-ring"
            />
            <p className="text-[11px] text-secondary">
              Aus dem IdP herunterladen (AzureAD: Enterprise-App → SAML → Federation Metadata XML).
            </p>
          </div>

          {data?.cert_pem && (
            <div className="space-y-1.5">
              <Label className="text-xs text-secondary">SP Zertifikat (öffentlich)</Label>
              <pre className="text-[10px] font-mono bg-muted/40 rounded p-2 max-h-24 overflow-auto">{data.cert_pem}</pre>
              <Button
                variant="outline"
                size="sm"
                onClick={handleRegen}
                disabled={regen.isPending}
                className="h-7 text-xs"
              >
                {regenDone ? <><Check className="w-3 h-3 mr-1" />Erneuert</> : 'Zertifikat erneuern'}
              </Button>
            </div>
          )}

          <Button
            size="sm"
            onClick={handleSave}
            disabled={update.isPending}
            className="h-8 text-xs"
          >
            {saved ? (
              <><Check className="w-3.5 h-3.5 mr-1" />Gespeichert</>
            ) : update.isPending ? (
              <><Spinner size="sm" />Speichern…</>
            ) : (
              'Speichern'
            )}
          </Button>
          {update.isError && <p className="text-xs text-destructive">{update.error.message}</p>}
        </div>
      )}
    </SectionCard>
  )
}

// ─── Server Info ──────────────────────────────────────────────────────────────

function UpdateSection() {
  const { t } = useTranslation()
  const { data, isLoading } = useUpdateCheck()

  return (
    <SectionCard title={t('settingsPage.updatesTitle')} icon={RefreshCw}>
      <div className="space-y-2 text-xs">
        {isLoading && <p className="text-secondary">{t('settingsPage.updatesChecking')}</p>}

        {!isLoading && !data?.check_enabled && (
          <p className="text-secondary">
            {t('settingsPage.updatesDisabled')}
          </p>
        )}

        {!isLoading && data?.check_enabled && (
          <div className="space-y-1.5">
            <div className="flex justify-between py-1.5 px-3 rounded-lg bg-surface2">
              <span className="text-secondary">{t('settingsPage.installedVersion')}</span>
              <span className="font-mono font-medium text-primary">{data.current_version || '—'}</span>
            </div>
            <div className="flex justify-between py-1.5 px-3 rounded-lg bg-surface2">
              <span className="text-secondary">{t('settingsPage.latestVersion')}</span>
              <span className="font-mono font-medium text-primary">{data.latest_version || '—'}</span>
            </div>

            {data.update_available ? (
              <div className="flex items-center gap-2 py-2 px-3 rounded-lg bg-amber-50 dark:bg-amber-950/30 border border-amber-200 dark:border-amber-800">
                <ArrowUpCircle className="w-3.5 h-3.5 text-amber-600 shrink-0" />
                <span className="text-amber-700 dark:text-amber-400 flex-1">{t('settingsPage.updateAvailable')}</span>
                {data.release_url && (
                  <a
                    href={data.release_url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="font-medium text-amber-700 dark:text-amber-400 hover:underline flex items-center gap-1"
                  >
                    {t('settingsPage.releaseNotes')} <ExternalLink className="w-3 h-3" />
                  </a>
                )}
              </div>
            ) : (
              <div className="flex items-center gap-2 py-1.5 px-3 rounded-lg bg-green-50 dark:bg-green-950/30 border border-green-200 dark:border-green-800">
                <Check className="w-3.5 h-3.5 text-green-600 shrink-0" />
                <span className="text-green-700 dark:text-green-400">{t('settingsPage.upToDate')}</span>
              </div>
            )}
          </div>
        )}
      </div>
    </SectionCard>
  )
}

function ServerSection() {
  const { t } = useTranslation()
  return (
    <SectionCard title={t('settingsPage.serverTitle')} icon={Server}>
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
      </div>
    </SectionCard>
  )
}

// ─── Data Export ─────────────────────────────────────────────────────────────

function DataExportSection() {
  const { t } = useTranslation()
  const { exportData, isLoading, error } = useExportData()

  return (
    <SectionCard title={t('settingsPage.dataExportTitle')} icon={ShieldCheck}>
      <div className="space-y-3">
        <p className="text-xs text-secondary leading-relaxed">
          {t('settingsPage.dataExportDesc')}
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
              <Spinner size="xs" color="current" className="mr-1.5" />
              {t('settingsPage.exporting')}
            </>
          ) : (
            <>
              <Download className="w-3 h-3 mr-1.5" />
              {t('settingsPage.exportData')}
            </>
          )}
        </Button>
        {error && (
          <p className="text-[11px] text-red-500">{error}</p>
        )}
        <p className="text-[11px] text-secondary">
          {t('settingsPage.dataExportHint')}
        </p>
      </div>
    </SectionCard>
  )
}

// ─── Audit Report ─────────────────────────────────────────────────────────────

function AuditReportSection() {
  const { t } = useTranslation()
  const { generate, isGenerating, error } = useAuditReport()

  return (
    <SectionCard title={t('settingsPage.auditReportTitle')} icon={FileText}>
      <div className="space-y-3">
        <p className="text-xs text-secondary leading-relaxed">
          {t('settingsPage.auditReportDesc')}
        </p>
        <Button
          size="sm"
          onClick={generate}
          disabled={isGenerating}
          className="h-7 text-xs gap-1.5"
        >
          {isGenerating ? (
            <>
              <Spinner size="xs" color="current" />
              {t('settingsPage.generatingReport')}
            </>
          ) : (
            <>
              <FileText className="w-3 h-3" />
              {t('settingsPage.generateAuditReport')}
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
          {t('settingsPage.auditReportHint')}
        </p>
      </div>
    </SectionCard>
  )
}

// ─── Staging Release ─────────────────────────────────────────────────────────

function StagingSection() {
  const { t } = useTranslation()
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
      <h3 className="text-xs font-semibold text-secondary uppercase tracking-wider mb-3">{t('settingsPage.sectionStaging')}</h3>
      <div className="max-w-sm">
        <SectionCard title={t('settingsPage.stagingPromoteTitle')} icon={Rocket}>
          <div className="space-y-3">
            <p className="text-xs text-secondary leading-relaxed">
              {t('settingsPage.stagingPromoteDesc')}
            </p>
            <Button
              size="sm"
              className="h-7 text-xs gap-1.5"
              onClick={() => { setResult('idle'); setConfirming(true) }}
            >
              <Rocket className="w-3 h-3" />
              {t('settingsPage.stagingPromote')}
            </Button>
            {result === 'ok' && (
              <p className="text-[11px] text-green-600">{t('settingsPage.stagingSuccess')}</p>
            )}
            {result === 'err' && (
              <p className="text-[11px] text-red-500">
                {promote.error?.message
                  ? `Fehler: ${promote.error.message}`
                  : t('settingsPage.stagingError')}
              </p>
            )}
          </div>
        </SectionCard>
      </div>

      <Dialog open={confirming} onOpenChange={setConfirming}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('settingsPage.stagingConfirmTitle')}</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">
            {t('settingsPage.stagingConfirmDesc')}
          </p>
          <DialogFooter>
            <Button variant="outline" onClick={() => setConfirming(false)}>{t('common.cancel')}</Button>
            <Button
              onClick={() => promote.mutate()}
              disabled={promote.isPending}
            >
              {promote.isPending ? (
                <><Spinner size="xs" color="current" className="mr-1.5" />{t('settingsPage.stagingStarting')}</>
              ) : t('settingsPage.stagingConfirm')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

// ─── Main Page ────────────────────────────────────────────────────────────────

export default function Settings() {
  const { t } = useTranslation()
  return (
    <div className="flex flex-col h-full">
      <PageHeader title={t('settingsPage.title')} description={t('settingsPage.description')} />
      <div className="flex-1 p-6 overflow-auto">
        <div className="max-w-5xl space-y-6">
          {/* Row 1: Organisation + Module + Sector + Lizenz */}
          <div>
            <h3 className="text-xs font-semibold text-secondary uppercase tracking-wider mb-3">{t('settingsPage.sectionPlatform')}</h3>
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
              <OrgSection />
              <ModulesSection />
              <SectorSection />
              <LicenseSection />
            </div>
          </div>

          {/* Row 2: Integrations — interactive, needs more visual weight */}
          <div>
            <h3 className="text-xs font-semibold text-secondary uppercase tracking-wider mb-3">{t('settingsPage.sectionIntegrations')}</h3>
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
              <SmtpSection />
              <NotificationsSection />
              <DigestToggleSection />
              <SectionCard title="Webhooks" icon={Zap}>
                <div className="space-y-3">
                  <p className="text-xs text-secondary leading-relaxed">
                    {t('settingsPage.webhooksDesc')}
                  </p>
                  <Link to="/settings/webhooks" className="inline-flex items-center gap-1.5 text-sm font-medium text-primary hover:underline">
                    {t('settingsPage.webhooksManage')} <ExternalLink className="h-3.5 w-3.5" />
                  </Link>
                </div>
              </SectionCard>
              <SectionCard title="Benachrichtigungseinstellungen" icon={Bell}>
                <div className="space-y-3">
                  <p className="text-xs text-secondary leading-relaxed">
                    Steuere welche Ereignisse du per E-Mail oder In-App erhältst — Digest, Findings, Vorfälle, Genehmigungen.
                  </p>
                  <Link to="/settings/notifications" className="inline-flex items-center gap-1.5 text-sm font-medium text-primary hover:underline">
                    Benachrichtigungen konfigurieren <ExternalLink className="h-3.5 w-3.5" />
                  </Link>
                </div>
              </SectionCard>
              <SectionCard title={t('settingsPage.scheduledReportsPlan')} icon={FileBarChart2}>
                <div className="space-y-3">
                  <p className="text-xs text-secondary leading-relaxed">
                    {t('settingsPage.scheduledReportsDesc')}
                  </p>
                  <Link to="/settings/reports" className="inline-flex items-center gap-1.5 text-sm font-medium text-primary hover:underline">
                    {t('settingsPage.scheduledReportsPlan')} <ExternalLink className="h-3.5 w-3.5" />
                  </Link>
                </div>
              </SectionCard>
            </div>
          </div>

          {/* Row 3: Data & Privacy export + Audit Report + API Keys */}
          <div>
            <h3 className="text-xs font-semibold text-secondary uppercase tracking-wider mb-3">{t('settingsPage.sectionPrivacy')}</h3>
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-5 max-w-2xl">
              <DataExportSection />
              <AuditReportSection />
              <SectionCard title={t('settingsPage.apiKeysTitle')} icon={Key}>
                <div className="space-y-3">
                  <p className="text-xs text-secondary leading-relaxed">
                    {t('settingsPage.apiKeysDesc')}
                  </p>
                  <Link to="/settings/api-keys" className="inline-flex items-center gap-1.5 text-sm font-medium text-primary hover:underline">
                    {t('settingsPage.apiKeysManage')} <ExternalLink className="h-3.5 w-3.5" />
                  </Link>
                </div>
              </SectionCard>
            </div>
          </div>

          {/* Row 4: Trust Center */}
          <div>
            <h3 className="text-xs font-semibold text-secondary uppercase tracking-wider mb-3">{t('settingsPage.sectionPublicPages')}</h3>
            <div className="max-w-sm">
              <SectionCard title={t('settingsPage.trustCenterTitle')} icon={Globe}>
                <p className="text-sm text-muted-foreground mb-3">
                  {t('settingsPage.trustCenterDesc2')}
                </p>
                <Link to="/settings/trust-center" className="inline-flex items-center gap-1.5 text-sm font-medium text-primary hover:underline">
                  {t('settingsPage.trustCenterConfigure2')} <ExternalLink className="h-3.5 w-3.5" />
                </Link>
              </SectionCard>
            </div>
          </div>

          {/* Staging-only: promote to demo — StagingSection renders null on non-staging instances */}
          <StagingSection />

          {/* AI Model Settings (S32-3) */}
          <div>
            <h3 className="text-xs font-semibold text-secondary uppercase tracking-wider mb-3">KI-Einstellungen</h3>
            <div className="max-w-sm">
              <AISettingsSection />
            </div>
          </div>

          {/* SAML 2.0 SSO Setup (S21-1, S21-2) */}
          <div>
            <h3 className="text-xs font-semibold text-secondary uppercase tracking-wider mb-3">Single Sign-On (SAML)</h3>
            <div className="max-w-sm">
              <SAMLSetupSection />
            </div>
          </div>

          {/* SIEM Integration (S21-7, S21-8) */}
          <div>
            <h3 className="text-xs font-semibold text-secondary uppercase tracking-wider mb-3">SIEM-Integration</h3>
            <div className="max-w-sm">
              <SIEMSection />
            </div>
          </div>

          {/* Row 4: System info — read-only reference, visually de-emphasized */}
          <div>
            <h3 className="text-xs font-semibold text-secondary uppercase tracking-wider mb-3">{t('settingsPage.sectionSystem')}</h3>
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-5 max-w-2xl">
              <UpdateSection />
              <ServerSection />
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
