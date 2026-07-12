import { useState, useEffect, useRef } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Spinner } from '../../../components/Spinner'
import { Link } from 'react-router-dom'
import {
  Building2, Layers, Bell, Trash2, Plus, Check, X,
  Webhook, Globe, Mail, Server, MapPin, Download, ShieldCheck, Shield, FileText, ExternalLink, Sparkles, Rocket, Key, Clock, ArrowUpCircle, RefreshCw, Zap, FileBarChart2, Radio,
  Siren, UserCheck, Users, Palette, Sliders, Network, HardDrive,
} from 'lucide-react'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../../../components/ui/tabs'
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
import { VAKT_LICENSE_RENEW_URL } from '../../../lib/constants'
import { useOrgSector, useUpdateOrgSector } from '../../../shared/hooks/useOrgSector'
import { useApprovalSetting, useUpdateApprovalSetting } from '../../../shared/hooks/useApprovals'
import { SECTOR_LABELS } from '../../../shared/types/sectors'
import { useExportData } from '../../../hooks/useDataExport'
import { useAuditReport } from '../../../shared/hooks/useAuditReport'
import { ProGate } from '../../../shared/components/ProGate'
import { useUpdateCheck, useToggleUpdateCheck } from '../../../shared/hooks/useUpdateCheck'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'

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

// ─── Backup config hooks ──────────────────────────────────────────────────────

interface OrgBackupConfig {
  schedule: string
  retention_days: number
  offsite_cmd: string
  notify_cmd: string
  has_passphrase: boolean
  has_notify_webhook: boolean
}

function useOrgBackupConfig() {
  return useQuery<OrgBackupConfig>({
    queryKey: ['admin', 'org', 'backup-config'],
    queryFn: () => apiFetch<OrgBackupConfig>('/admin/org/backup-config'),
    retry: false,
  })
}

function useUpdateOrgBackupConfig() {
  const qc = useQueryClient()
  return useMutation<undefined, Error, Partial<OrgBackupConfig> & { passphrase?: string; notify_webhook?: string }>({
    mutationFn: (input) =>
      apiFetch<undefined>('/admin/org/backup-config', { method: 'PUT', body: JSON.stringify(input) }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['admin', 'org', 'backup-config'] }),
  })
}

// ─── SMTP hooks ──────────────────────────────────────────────────────────────

interface OrgSMTPSettings {
  host: string
  port: string
  user: string
  from: string
  tls: boolean
  has_pass: boolean
}

function useOrgSMTPSettings() {
  return useQuery<OrgSMTPSettings>({
    queryKey: ['admin', 'org', 'smtp'],
    queryFn: () => apiFetch<OrgSMTPSettings>('/admin/org/smtp'),
    retry: false,
  })
}

function useUpdateOrgSMTPSettings() {
  const qc = useQueryClient()
  return useMutation<undefined, Error, Partial<OrgSMTPSettings> & { pass?: string }>({
    mutationFn: (input) =>
      apiFetch<undefined>('/admin/org/smtp', { method: 'PUT', body: JSON.stringify(input) }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['admin', 'org', 'smtp'] }),
  })
}

// ─── LDAP hooks ──────────────────────────────────────────────────────────────

interface OrgLDAPConfig {
  url: string
  bind_dn: string
  base_dn: string
  user_filter: string
  group_filter: string
  tls: boolean
  has_bind_pass: boolean
}

function useOrgLDAPConfig() {
  return useQuery<OrgLDAPConfig>({
    queryKey: ['admin', 'org', 'ldap'],
    queryFn: () => apiFetch<OrgLDAPConfig>('/admin/org/ldap'),
    retry: false,
  })
}

function useUpdateOrgLDAPConfig() {
  const qc = useQueryClient()
  return useMutation<undefined, Error, Partial<OrgLDAPConfig> & { bind_pass?: string }>({
    mutationFn: (input) =>
      apiFetch<undefined>('/admin/org/ldap', { method: 'PUT', body: JSON.stringify(input) }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['admin', 'org', 'ldap'] }),
  })
}

function useTestLDAPConnection() {
  return useMutation<{ ok: boolean; users_found?: number; error?: string }>({
    mutationFn: () => apiFetch('/admin/org/ldap/test', { method: 'POST' }),
  })
}

function useSyncLDAP() {
  return useMutation<{ synced: number }>({
    mutationFn: () => apiFetch('/admin/org/ldap/sync', { method: 'POST' }),
  })
}

// ─── Backup destination hooks ─────────────────────────────────────────────────

interface OrgBackupDest {
  type: string // "none"|"nextcloud"|"s3"|"sftp"|"custom"
  url: string
  user: string
  remote_path: string
  has_pass: boolean
  endpoint: string
  bucket: string
  prefix: string
  access_key: string
  has_secret_key: boolean
  host: string
  port: number
  cmd: string
}

function useOrgBackupDest() {
  return useQuery<OrgBackupDest>({
    queryKey: ['admin', 'org', 'backup-dest'],
    queryFn: () => apiFetch<OrgBackupDest>('/admin/org/backup-dest'),
    retry: false,
  })
}

function useUpdateOrgBackupDest() {
  const qc = useQueryClient()
  return useMutation<undefined, Error, Partial<OrgBackupDest> & { pass?: string; secret_key?: string }>({
    mutationFn: (input) =>
      apiFetch<undefined>('/admin/org/backup-dest', { method: 'PUT', body: JSON.stringify(input) }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['admin', 'org', 'backup-dest'] }),
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
  return useMutation<undefined, Error, OrgSecurity>({
    mutationFn: (input) =>
      apiFetch<undefined>('/admin/org/security', {
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
  return useMutation<undefined, Error, string>({
    mutationFn: (id) => apiFetch<undefined>(`/admin/notifications/channels/${id}`, { method: 'DELETE' }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['admin', 'notifications', 'channels'] }),
  })
}

// ─── Module labels ────────────────────────────────────────────────────────────

// MODULE_META desc keys map to settingsPage.moduleVakt*Desc in i18n
const MODULE_META: Record<string, { label: string; descKey: string } | undefined> = {
  vaktscan:    { label: 'Vakt Scan',    descKey: 'moduleVaktscanDesc' },
  vaktcomply:  { label: 'Vakt Comply',  descKey: 'moduleVaktcomplyDesc' },
  vaktvault:   { label: 'Vakt Vault',   descKey: 'moduleVaktvaultDesc' },
  vaktaware:   { label: 'Vakt Aware',   descKey: 'moduleVaktawareDesc' },
  vaktprivacy: { label: 'Vakt Privacy', descKey: 'moduleVaktprivacyDesc' },
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
  auto_renewal_enabled: boolean
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
  vaktaware_advanced: 'Vakt Aware Pro',
  vaktscan_advanced: 'Vakt Scan Pro',
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
  const { formatDate } = useFormatDate()
  const { data: lic, isLoading } = useLicense()
  const activate = useActivateLicense()
  const [licKey, setLicKey] = useState('')
  const [activateSuccess, setActivateSuccess] = useState(false)
  const licTimerRef = useRef<ReturnType<typeof setTimeout>>()

  useEffect(() => () => { clearTimeout(licTimerRef.current); }, [])

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
        licTimerRef.current = setTimeout(() => { setActivateSuccess(false); }, 5000)
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
          {lic?.auto_renewal_enabled && (
            <Badge variant="outline" className="text-xs px-2 py-0.5 text-green-700 border-green-300 bg-green-50 dark:text-green-400 dark:border-green-800 dark:bg-green-950/30">
              {t('settingsPage.licenseAutoRenewal')}
            </Badge>
          )}
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
            {t('settingsPage.licenseValidUntil', { date: formatDate(lic.expires_at) })}
          </p>
        )}

        {lic?.expires_at && !lic.auto_renewal_enabled && daysUntilExpiry(lic.expires_at) < 30 && (
          <div className="text-sm text-amber-600 bg-amber-50 border border-amber-200 rounded p-2">
            {daysUntilExpiry(lic.expires_at) === 0
              ? t('settingsPage.licenseExpired')
              : t('settingsPage.licenseExpiringSoon', { days: daysUntilExpiry(lic.expires_at) })}
          </div>
        )}

        {isPro && !lic?.demo && (
          <a href={VAKT_LICENSE_RENEW_URL} target="_blank" rel="noopener noreferrer" className="text-sm text-primary underline">
            {t('settingsPage.renewLicense')}
          </a>
        )}

        {!isPro && (
          <div className="space-y-1.5">
            <p className="text-xs text-secondary">{t('settingsPage.proFeatures')}</p>
            <ul className="text-xs text-secondary space-y-0.5 list-none">
              {([
                'licenseProFeature1', 'licenseProFeature2', 'licenseProFeature3', 'licenseProFeature4',
              ] as const).map((key) => (
                <li key={key} className="flex items-center gap-1.5">
                  <span className="text-brand">›</span>
                  {t(`settingsPage.${key}`)}
                </li>
              ))}
            </ul>
            <span className="inline-flex items-center gap-1.5 text-xs font-semibold text-brand">
              <Clock className="w-3.5 h-3.5" />
              {t('settingsPage.licenseProComingSoon')}
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

  const isAdmin = user?.roles.includes('Admin') ?? false

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
                  {meta?.descKey && <div className="text-[11px] text-secondary">{t(`settingsPage.${meta.descKey}`)}</div>}
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
  const { data, isLoading } = useOrgSMTPSettings()
  const update = useUpdateOrgSMTPSettings()

  const [host, setHost] = useState('')
  const [port, setPort] = useState('587')
  const [user, setUser] = useState('')
  const [pass, setPass] = useState('')
  const [from, setFrom] = useState('')
  const [tls, setTls] = useState(true)
  const [saved, setSaved] = useState(false)

  useEffect(() => {
    if (data) {
      setHost(data.host ?? '')
      setPort(data.port ?? '587')
      setUser(data.user ?? '')
      setFrom(data.from ?? '')
      setTls(data.tls ?? true)
    }
  }, [data])

  function handleSave() {
    update.mutate(
      { host, port, user, pass: pass || undefined, from, tls },
      { onSuccess: () => { setSaved(true); setPass(''); setTimeout(() => { setSaved(false); }, 2000) } },
    )
  }

  if (isLoading) return (
    <SectionCard title={t('settingsPage.smtpTitle')} icon={Mail}>
      <Spinner />
    </SectionCard>
  )

  return (
    <SectionCard title={t('settingsPage.smtpTitle')} icon={Mail}>
      <div className="space-y-3">
        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <Label className="text-xs">{t('settingsPage.smtpHostLabel')}</Label>
            <Input className="h-8 text-sm" placeholder="smtp.example.com" value={host} onChange={(e) => { setHost(e.target.value) }} />
          </div>
          <div className="space-y-1.5">
            <Label className="text-xs">{t('settingsPage.smtpPortLabel')}</Label>
            <Input className="h-8 text-sm" placeholder="587" value={port} onChange={(e) => { setPort(e.target.value) }} />
          </div>
        </div>
        <div className="space-y-1.5">
          <Label className="text-xs">{t('settingsPage.smtpUserLabel')}</Label>
          <Input className="h-8 text-sm" placeholder="user@example.com" value={user} onChange={(e) => { setUser(e.target.value) }} />
        </div>
        <div className="space-y-1.5">
          <Label className="text-xs">{t('settingsPage.smtpPassLabel')}</Label>
          <Input className="h-8 text-sm" type="password" placeholder={data?.has_pass ? '••••••••' : t('settingsPage.smtpPassPlaceholder')} value={pass} onChange={(e) => { setPass(e.target.value) }} />
        </div>
        <div className="space-y-1.5">
          <Label className="text-xs">{t('settingsPage.smtpFromLabel')}</Label>
          <Input className="h-8 text-sm" placeholder="noreply@example.com" value={from} onChange={(e) => { setFrom(e.target.value) }} />
        </div>
        <div className="flex items-center justify-between">
          <Label className="text-xs">{t('settingsPage.smtpTlsLabel')}</Label>
          <Switch checked={tls} onCheckedChange={(v) => { setTls(v) }} />
        </div>
        <div className="flex justify-end">
          <Button size="sm" onClick={handleSave} disabled={update.isPending}>
            {saved ? <><Check className="h-3.5 w-3.5 mr-1" />{t('common.saved')}</> : t('common.save')}
          </Button>
        </div>
        {update.isError && <p className="text-xs text-destructive">{t('settingsPage.smtpSaveError')}</p>}
        <p className="text-[11px] text-secondary">{t('settingsPage.smtpUsage')}</p>
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
                    deleteChannel.mutate(ch.id, { onSettled: () => { setDeletingChannelId(null); } })
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
          <Button size="sm" variant="outline" onClick={() => { setCreateOpen(true); }} className="h-7 text-xs">
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
              <Select value={type} onValueChange={(v) => { setType(v as typeof type); }}>
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
                onChange={(e) => { setName(e.target.value); }}
                onBlur={() => { setFieldTouched((prev) => ({ ...prev, name: true })); }}
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
                onChange={(e) => { setUrl(e.target.value); }}
                onBlur={() => { setFieldTouched((prev) => ({ ...prev, url: true })); }}
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
            <Button variant="outline" onClick={() => { handleDialogClose(false); }}>{t('common.cancel')}</Button>
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
  return useMutation<undefined, Error, Partial<OrgSIEMConfig>>({
    mutationFn: (input) =>
      apiFetch<undefined>('/admin/org/siem', {
        method: 'PUT',
        body: JSON.stringify(input),
      }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['admin', 'org', 'siem'] }),
  })
}

function useTestSIEM() {
  return useMutation<undefined>({
    mutationFn: () => apiFetch<undefined>('/admin/org/siem/test', { method: 'POST' }),
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
          setTimeout(() => { setSaved(false); }, 2500)
        },
      },
    )
  }

  function handleTest() {
    setTestResult('idle')
    setTestError('')
    test.mutate(undefined, {
      onSuccess: () => { setTestResult('ok'); },
      onError: (err) => { setTestResult('err'); setTestError(err.message) },
    })
  }

  const { t } = useTranslation()

  return (
    <SectionCard title={t('settingsPage.siemTitle')} icon={Radio}>
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
              {t('settingsPage.siemProTitle')}
              <span className="ml-2 inline-flex items-center gap-1 text-[10px] font-semibold bg-brand/10 text-brand px-1.5 py-0.5 rounded">Pro</span>
            </p>
            <p className="text-secondary text-sm leading-relaxed mb-2">
              {t('settingsPage.siemProDesc')}
            </p>
            <span className="inline-flex items-center gap-1.5 text-xs font-semibold text-brand">
              <Clock className="w-3.5 h-3.5" />
              {t('settingsPage.siemProRequired')}
            </span>
          </div>
        </div>
      )}

      {!isLoading && !isProLocked && (
        <div className="space-y-4">
          {/* Enable toggle */}
          <div className="flex items-start justify-between gap-4">
            <div className="space-y-1">
              <p className="text-sm font-medium text-primary">{t('settingsPage.siemEnableTitle')}</p>
              <p className="text-[11px] text-secondary leading-relaxed">
                {t('settingsPage.siemEnableDesc')}
              </p>
            </div>
            <Switch
              checked={enabled}
              onCheckedChange={setEnabled}
              aria-label={t('settingsPage.siemAriaEnable')}
            />
          </div>

          {/* Adapter */}
          <div className="space-y-1.5">
            <Label className="text-xs">{t('settingsPage.siemAdapterLabel')}</Label>
            <Select value={adapter} onValueChange={(v) => { setAdapter(v as OrgSIEMConfig['adapter']); }}>
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
            <Label className="text-xs">{t('settingsPage.siemEndpointLabel')}</Label>
            <Input
              value={endpoint}
              onChange={(e) => { setEndpoint(e.target.value); }}
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
              {adapter === 'splunk_hec' ? t('settingsPage.siemTokenLabel') : adapter === 'elastic' ? t('settingsPage.siemApiKeyLabel') : t('settingsPage.siemBearerLabel')}
            </Label>
            <Input
              type="password"
              value={token}
              onChange={(e) => { setToken(e.target.value); }}
              placeholder={data?.token === '***' ? t('settingsPage.siemTokenPlaceholderSet') : t('settingsPage.siemTokenPlaceholder')}
              className="h-8 text-sm"
              autoComplete="new-password"
            />
            <p className="text-[11px] text-secondary">
              {t('settingsPage.siemTokenHint')}
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
                <><Check className="w-3.5 h-3.5 mr-1" />{t('settingsPage.siemSaved')}</>
              ) : update.isPending ? (
                <><Spinner size="sm" />{t('settingsPage.siemSaving')}</>
              ) : (
                t('settingsPage.siemSave')
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
                <><Spinner size="sm" />{t('settingsPage.siemTesting')}</>
              ) : (
                t('settingsPage.siemTestSend')
              )}
            </Button>
          </div>

          {update.isError && (
            <p className="text-[11px] text-red-500">{update.error.message}</p>
          )}
          {testResult === 'ok' && (
            <p className="text-[11px] text-green-600 dark:text-green-400">
              {t('settingsPage.siemTestSuccess')}
            </p>
          )}
          {testResult === 'err' && (
            <p className="text-[11px] text-red-500">{t('settingsPage.siemTestFailed', { error: testError })}</p>
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
  weekly_digest_enabled: boolean
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
    queryFn: () => apiFetch<{ models: string[] }>('/vaktcomply/ai/models'),
    staleTime: 60_000,
  })
}

function useUpdateOrgAISettings() {
  const qc = useQueryClient()
  return useMutation<undefined, Error, OrgAISettings>({
    mutationFn: (data) =>
      apiFetch<undefined>('/admin/org/ai-settings', {
        method: 'PUT',
        body: JSON.stringify(data),
      }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['org-ai-settings'] }),
  })
}

function AISettingsSection() {
  const { t } = useTranslation()
  const { data: settings, isLoading } = useOrgAISettings()
  const { data: modelsData } = useOllamaModels()
  const { data: lic } = useQuery<LicenseInfo>({ queryKey: ['license'], queryFn: () => apiFetch<LicenseInfo>('/license') })
  const update = useUpdateOrgAISettings()

  const [model, setModel] = useState('')
  const [baseURL, setBaseURL] = useState('')
  const [weeklyDigest, setWeeklyDigest] = useState(false)
  const [saved, setSaved] = useState(false)

  useEffect(() => {
    if (settings) {
      setModel(settings.model_override)
      setBaseURL(settings.base_url_override)
      setWeeklyDigest(settings.weekly_digest_enabled)
    }
  }, [settings])

  const handleSave = () => {
    update.mutate(
      { model_override: model, base_url_override: baseURL, weekly_digest_enabled: weeklyDigest },
      { onSuccess: () => { setSaved(true); setTimeout(() => { setSaved(false); }, 2000) } },
    )
  }

  const ollamaModels = modelsData?.models ?? []
  const isPro = lic?.features.includes('ai_advisor') ?? false

  return (
    <SectionCard title={t('settingsPage.aiTitle')} icon={Sparkles}>
      {isLoading ? (
        <Spinner size="sm" />
      ) : (
        <div className="space-y-4">
          <div className="space-y-1.5">
            <Label className="text-xs">{t('settingsPage.aiModelLabel')}</Label>
            {ollamaModels.length > 0 ? (
              <Select
                value={model === '' ? '__default__' : model}
                onValueChange={(v) => { setModel(v === '__default__' ? '' : v); }}
              >
                <SelectTrigger className="h-8 text-sm">
                  <SelectValue placeholder={t('settingsPage.aiModelSelectPlaceholder')} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="__default__">{t('settingsPage.aiModelSelectDefault')}</SelectItem>
                  {ollamaModels.map((m) => (
                    <SelectItem key={m} value={m}>{m}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            ) : (
              <Input
                value={model}
                onChange={(e) => { setModel(e.target.value); }}
                placeholder={t('settingsPage.aiModelInputPlaceholder')}
                className="h-8 text-sm"
              />
            )}
            <p className="text-[11px] text-secondary">
              {t('settingsPage.aiModelHint')}
            </p>
          </div>

          {isPro && (
            <div className="space-y-1.5">
              <Label className="text-xs">{t('settingsPage.aiEndpointLabel')} <Badge variant="secondary" className="text-[10px] ml-1">Pro</Badge></Label>
              <Input
                value={baseURL}
                onChange={(e) => { setBaseURL(e.target.value); }}
                placeholder="https://api.openai.com/v1"
                className="h-8 text-sm"
              />
              <p className="text-[11px] text-secondary">
                {t('settingsPage.aiEndpointHint')}
              </p>
            </div>
          )}

          {/* S52-4: AI Weekly Digest */}
          <div className="border-t border-border pt-4 flex items-start justify-between gap-4">
            <div className="space-y-1">
              <p className="text-sm font-medium text-primary">{t('settingsPage.aiDigestTitle')}</p>
              <p className="text-[11px] text-secondary leading-relaxed">
                {t('settingsPage.aiDigestDesc')}
              </p>
            </div>
            <Switch
              checked={weeklyDigest}
              onCheckedChange={setWeeklyDigest}
              aria-label={t('settingsPage.aiDigestAria')}
            />
          </div>

          <Button
            size="sm"
            onClick={handleSave}
            disabled={update.isPending}
            className="h-8 text-xs"
          >
            {saved ? (
              <><Check className="w-3.5 h-3.5 mr-1" />{t('settingsPage.aiSaved')}</>
            ) : update.isPending ? (
              <><Spinner size="sm" />{t('settingsPage.aiSaving')}</>
            ) : (
              t('settingsPage.aiSave')
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
  jit_provisioning: boolean
}

interface OrgOIDCConfig {
  configured: boolean
  provider_url?: string
  client_id?: string
  enabled?: boolean
  updated_at?: string
}

function useOrgSAMLConfig() {
  return useQuery<OrgSAMLConfig>({
    queryKey: ['org-saml-config'],
    queryFn: () => apiFetch<OrgSAMLConfig>('/admin/org/saml-config'),
  })
}

function useUpdateSAMLConfig() {
  const qc = useQueryClient()
  return useMutation<undefined, Error, Omit<OrgSAMLConfig, 'org_id' | 'cert_pem'>>({
    mutationFn: (data) =>
      apiFetch<undefined>('/admin/org/saml-config', {
        method: 'PUT',
        body: JSON.stringify(data),
      }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['org-saml-config'] }),
  })
}

function useRegenerateSAMLCert() {
  const qc = useQueryClient()
  return useMutation<{ cert_pem: string }>({
    mutationFn: () =>
      apiFetch<{ cert_pem: string }>('/admin/org/saml-config/regenerate-cert', { method: 'POST' }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['org-saml-config'] }),
  })
}

function useFetchSAMLMetadata() {
  return useMutation<{ metadata: string }, Error, { url: string }>({
    mutationFn: (data) =>
      apiFetch<{ metadata: string }>('/admin/org/saml-config/fetch-metadata', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
  })
}

function useOrgOIDCConfig() {
  return useQuery<OrgOIDCConfig>({
    queryKey: ['org-oidc-config'],
    queryFn: () => apiFetch<OrgOIDCConfig>('/admin/org/oidc-config'),
  })
}

function useUpdateOIDCConfig() {
  const qc = useQueryClient()
  return useMutation<undefined, Error, { provider_url: string; client_id: string; client_secret: string; enabled: boolean }>({
    mutationFn: (data) =>
      apiFetch<undefined>('/admin/org/oidc-config', {
        method: 'PUT',
        body: JSON.stringify(data),
      }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['org-oidc-config'] }),
  })
}

function useDisableOIDCConfig() {
  const qc = useQueryClient()
  return useMutation<undefined>({
    mutationFn: () => apiFetch<undefined>('/admin/org/oidc-config', { method: 'DELETE' }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['org-oidc-config'] }),
  })
}

function SAMLSetupSection() {
  const { data, isLoading, error } = useOrgSAMLConfig()
  const update = useUpdateSAMLConfig()
  const regen = useRegenerateSAMLCert()
  const fetchMeta = useFetchSAMLMetadata()

  const [entityID, setEntityID] = useState('')
  const [acsURL, setACSURL] = useState('')
  const [idpMeta, setIdpMeta] = useState('')
  const [idpMetaURL, setIdpMetaURL] = useState('')
  const [enabled, setEnabled] = useState(false)
  const [jitProvisioning, setJitProvisioning] = useState(true)
  const [saved, setSaved] = useState(false)
  const [regenDone, setRegenDone] = useState(false)

  useEffect(() => {
    if (data) {
      setEntityID(data.entity_id ?? '')
      setACSURL(data.acs_url ?? '')
      setIdpMeta(data.idp_metadata ?? '')
      setEnabled(data.enabled ?? false)
      setJitProvisioning(data.jit_provisioning ?? true)
    }
  }, [data])

  const handleSave = () => {
    update.mutate(
      { entity_id: entityID, acs_url: acsURL, idp_metadata: idpMeta, enabled, jit_provisioning: jitProvisioning },
      { onSuccess: () => { setSaved(true); setTimeout(() => { setSaved(false); }, 2000) } },
    )
  }

  const handleFetchMetadata = () => {
    if (!idpMetaURL) return
    fetchMeta.mutate({ url: idpMetaURL }, {
      onSuccess: (res) => { setIdpMeta(res.metadata); setIdpMetaURL('') },
      onError: (err) => { alert(err.message) },
    })
  }

  const handleRegen = () => {
    regen.mutate(undefined, {
      onSuccess: () => { setRegenDone(true); setTimeout(() => { setRegenDone(false); }, 3000) },
    })
  }

  const metadataURL = window.location.origin + '/api/v1/auth/saml/metadata'
  const initiateURL = window.location.origin + '/api/v1/auth/saml/initiate'

  const { t } = useTranslation()

  return (
    <SectionCard title={t('settingsPage.samlTitle')} icon={Shield}>
      {error instanceof FeatureLockedError ? (
        <ProGate error={error}>{''}</ProGate>
      ) : isLoading ? (
        <Spinner size="sm" />
      ) : (
        <div className="space-y-4">
          <div className="flex items-center gap-2">
            <Switch checked={enabled} onCheckedChange={setEnabled} id="saml-enabled" />
            <Label htmlFor="saml-enabled" className="text-xs">{t('settingsPage.samlEnabled')}</Label>
          </div>

          {/* SP Endpoint URLs (read-only) */}
          <div className="rounded-md bg-muted/40 p-3 space-y-2 text-xs">
            <p className="font-medium text-secondary uppercase tracking-wider text-[10px]">{t('settingsPage.samlSpEndpoints')}</p>
            <div className="space-y-1">
              <Label className="text-[10px] text-secondary">{t('settingsPage.samlMetadataUrl')}</Label>
              <code className="block font-mono text-[11px] break-all">{metadataURL}</code>
            </div>
            <div className="space-y-1">
              <Label className="text-[10px] text-secondary">{t('settingsPage.samlAcsUrl')}</Label>
              <code className="block font-mono text-[11px] break-all">{initiateURL.replace('/initiate', '/acs')}</code>
            </div>
          </div>

          <div className="space-y-1.5">
            <Label className="text-xs">{t('settingsPage.samlEntityId')}</Label>
            <Input
              value={entityID}
              onChange={(e) => { setEntityID(e.target.value); }}
              placeholder={`${window.location.origin}/saml`}
              className="h-8 text-sm font-mono"
            />
          </div>

          <div className="space-y-1.5">
            <Label className="text-xs">{t('settingsPage.samlAcsUrlLabel')}</Label>
            <Input
              value={acsURL}
              onChange={(e) => { setACSURL(e.target.value); }}
              placeholder={`${window.location.origin}/api/v1/auth/saml/acs`}
              className="h-8 text-sm font-mono"
            />
          </div>

          <div className="space-y-1.5">
            <Label className="text-xs">{t('settingsPage.samlIdpMetadataLabel')}</Label>
            <div className="flex gap-2">
              <Input
                value={idpMetaURL}
                onChange={(e) => { setIdpMetaURL(e.target.value); }}
                placeholder="https://login.microsoftonline.com/.../federationmetadata.xml"
                className="h-8 text-sm font-mono flex-1"
              />
              <Button
                variant="outline"
                size="sm"
                onClick={handleFetchMetadata}
                disabled={!idpMetaURL || fetchMeta.isPending}
                className="h-8 text-xs shrink-0"
              >
                {fetchMeta.isPending ? <Spinner size="sm" /> : t('settingsPage.samlFetchUrl')}
              </Button>
            </div>
            <textarea
              value={idpMeta}
              onChange={(e) => { setIdpMeta(e.target.value); }}
              placeholder='<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" …'
              className="w-full h-28 rounded-md border border-input bg-transparent px-3 py-2 text-xs font-mono resize-y focus:outline-none focus:ring-1 focus:ring-ring"
            />
            <p className="text-[11px] text-secondary">
              {t('settingsPage.samlIdpMetadataHint')}
            </p>
          </div>

          <div className="flex items-center gap-2">
            <Switch checked={jitProvisioning} onCheckedChange={setJitProvisioning} id="saml-jit" />
            <Label htmlFor="saml-jit" className="text-xs">{t('settingsPage.samlJit')}</Label>
          </div>

          {data?.cert_pem && (
            <div className="space-y-1.5">
              <Label className="text-xs text-secondary">{t('settingsPage.samlCertLabel')}</Label>
              <pre className="text-[10px] font-mono bg-muted/40 rounded p-2 max-h-24 overflow-auto">{data.cert_pem}</pre>
              <Button
                variant="outline"
                size="sm"
                onClick={handleRegen}
                disabled={regen.isPending}
                className="h-7 text-xs"
              >
                {regenDone ? <><Check className="w-3 h-3 mr-1" />{t('settingsPage.samlCertRenewed')}</> : t('settingsPage.samlCertRenew')}
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
              <><Check className="w-3.5 h-3.5 mr-1" />{t('settingsPage.samlSaved')}</>
            ) : update.isPending ? (
              <><Spinner size="sm" />{t('settingsPage.samlSaving')}</>
            ) : (
              t('settingsPage.samlSave')
            )}
          </Button>
          {update.isError && <p className="text-xs text-destructive">{update.error.message}</p>}
        </div>
      )}
    </SectionCard>
  )
}

// ─── S105-2: OIDC/Casdoor Config ──────────────────────────────────────────────

function OIDCConfigSection() {
  const { data, isLoading, error } = useOrgOIDCConfig()
  const update = useUpdateOIDCConfig()
  const disable = useDisableOIDCConfig()

  const [providerURL, setProviderURL] = useState('')
  const [clientID, setClientID] = useState('')
  const [clientSecret, setClientSecret] = useState('')
  const [enabled, setEnabled] = useState(true)
  const [saved, setSaved] = useState(false)

  useEffect(() => {
    if (data?.configured) {
      setProviderURL(data.provider_url ?? '')
      setClientID(data.client_id ?? '')
      setEnabled(data.enabled ?? true)
    }
  }, [data])

  const handleSave = () => {
    update.mutate(
      { provider_url: providerURL, client_id: clientID, client_secret: clientSecret, enabled },
      { onSuccess: () => { setSaved(true); setClientSecret(''); setTimeout(() => { setSaved(false); }, 2000) } },
    )
  }

  const { t } = useTranslation()

  return (
    <SectionCard title={t('settingsPage.oidcTitle')} icon={Shield}>
      {error instanceof FeatureLockedError ? (
        <ProGate error={error}>{''}</ProGate>
      ) : isLoading ? (
        <Spinner size="sm" />
      ) : (
        <div className="space-y-4">
          <p className="text-xs text-secondary">
            {t('settingsPage.oidcHint')}
          </p>

          <div className="flex items-center gap-2">
            <Switch checked={enabled} onCheckedChange={setEnabled} id="oidc-enabled" />
            <Label htmlFor="oidc-enabled" className="text-xs">{t('settingsPage.oidcEnabled')}</Label>
          </div>

          <div className="space-y-1.5">
            <Label className="text-xs">{t('settingsPage.oidcProviderUrl')}</Label>
            <Input
              value={providerURL}
              onChange={(e) => { setProviderURL(e.target.value); }}
              placeholder="https://casdoor.company.com"
              className="h-8 text-sm font-mono"
            />
          </div>

          <div className="space-y-1.5">
            <Label className="text-xs">{t('settingsPage.oidcClientId')}</Label>
            <Input
              value={clientID}
              onChange={(e) => { setClientID(e.target.value); }}
              className="h-8 text-sm font-mono"
            />
          </div>

          <div className="space-y-1.5">
            <Label className="text-xs">{t('settingsPage.oidcClientSecret')}</Label>
            <Input
              type="password"
              value={clientSecret}
              onChange={(e) => { setClientSecret(e.target.value); }}
              placeholder={data?.configured ? t('settingsPage.oidcClientSecretPlaceholder') : ''}
              className="h-8 text-sm"
            />
          </div>

          <div className="flex gap-2">
            <Button
              size="sm"
              onClick={handleSave}
              disabled={update.isPending || !providerURL || !clientID || (!data?.configured && !clientSecret)}
              className="h-8 text-xs"
            >
              {saved ? <><Check className="w-3.5 h-3.5 mr-1" />{t('settingsPage.oidcSaved')}</> : update.isPending ? <><Spinner size="sm" />{t('settingsPage.oidcSaving')}</> : t('settingsPage.oidcSave')}
            </Button>
            {data?.configured && (
              <Button
                variant="outline"
                size="sm"
                onClick={() => { disable.mutate() }}
                disabled={disable.isPending}
                className="h-8 text-xs text-destructive hover:text-destructive"
              >
                {t('settingsPage.oidcDisable')}
              </Button>
            )}
          </div>
          {update.isError && <p className="text-xs text-destructive">{update.error.message}</p>}
        </div>
      )}
    </SectionCard>
  )
}

// ─── LDAP / AD Section ───────────────────────────────────────────────────────

function LDAPSection() {
  const { t } = useTranslation()
  const { data, isLoading } = useOrgLDAPConfig()
  const update = useUpdateOrgLDAPConfig()
  const testConn = useTestLDAPConnection()
  const sync = useSyncLDAP()

  const [url, setUrl] = useState('')
  const [bindDn, setBindDn] = useState('')
  const [bindPass, setBindPass] = useState('')
  const [baseDn, setBaseDn] = useState('')
  const [userFilter, setUserFilter] = useState('')
  const [groupFilter, setGroupFilter] = useState('')
  const [tls, setTls] = useState(false)
  const [saved, setSaved] = useState(false)
  const [testMsg, setTestMsg] = useState<string | null>(null)
  const [syncMsg, setSyncMsg] = useState<string | null>(null)

  useEffect(() => {
    if (data) {
      setUrl(data.url ?? '')
      setBindDn(data.bind_dn ?? '')
      setBaseDn(data.base_dn ?? '')
      setUserFilter(data.user_filter ?? '')
      setGroupFilter(data.group_filter ?? '')
      setTls(data.tls ?? false)
    }
  }, [data])

  const handleSave = () => {
    update.mutate(
      { url, bind_dn: bindDn, bind_pass: bindPass || undefined, base_dn: baseDn, user_filter: userFilter, group_filter: groupFilter, tls },
      {
        onSuccess: () => { setSaved(true); setBindPass(''); setTimeout(() => { setSaved(false) }, 2000) },
        onError: () => { alert(t('settingsPage.ldapSaveError')) },
      },
    )
  }

  const handleTest = () => {
    setTestMsg(null)
    testConn.mutate(undefined, {
      onSuccess: (res) => {
        setTestMsg(res.ok
          ? t('settingsPage.ldapTestOk', { count: res.users_found ?? 0 })
          : t('settingsPage.ldapTestFail') + (res.error ? `: ${res.error}` : ''))
      },
      onError: (err) => { setTestMsg(t('settingsPage.ldapTestFail') + `: ${err.message}`) },
    })
  }

  const handleSync = () => {
    setSyncMsg(null)
    sync.mutate(undefined, {
      onSuccess: (res) => { setSyncMsg(t('settingsPage.ldapSyncOk', { count: res.synced })) },
      onError: (err) => { setSyncMsg(t('settingsPage.ldapSyncFail') + `: ${err.message}`) },
    })
  }

  if (isLoading) return null

  return (
    <SectionCard title={t('settingsPage.ldapTitle')} icon={Network}>
      <div className="space-y-3">
        <div className="grid grid-cols-1 gap-3">
          <div>
            <label className="block text-xs font-medium text-muted-foreground mb-1">{t('settingsPage.ldapUrlLabel')}</label>
            <Input value={url} onChange={(e) => { setUrl(e.target.value) }} placeholder={t('settingsPage.ldapUrlPlaceholder')} />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-xs font-medium text-muted-foreground mb-1">{t('settingsPage.ldapBindDnLabel')}</label>
              <Input value={bindDn} onChange={(e) => { setBindDn(e.target.value) }} placeholder={t('settingsPage.ldapBindDnPlaceholder')} />
            </div>
            <div>
              <label className="block text-xs font-medium text-muted-foreground mb-1">
                {t('settingsPage.ldapBindPassLabel')}
                {data?.has_bind_pass && <span className="ml-1 text-green-600 text-xs">✓</span>}
              </label>
              <Input type="password" value={bindPass} onChange={(e) => { setBindPass(e.target.value) }} placeholder={t('settingsPage.ldapBindPassPlaceholder')} />
            </div>
          </div>
          <div>
            <label className="block text-xs font-medium text-muted-foreground mb-1">{t('settingsPage.ldapBaseDnLabel')}</label>
            <Input value={baseDn} onChange={(e) => { setBaseDn(e.target.value) }} placeholder={t('settingsPage.ldapBaseDnPlaceholder')} />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-xs font-medium text-muted-foreground mb-1">{t('settingsPage.ldapUserFilterLabel')}</label>
              <Input value={userFilter} onChange={(e) => { setUserFilter(e.target.value) }} placeholder={t('settingsPage.ldapUserFilterPlaceholder')} />
            </div>
            <div>
              <label className="block text-xs font-medium text-muted-foreground mb-1">{t('settingsPage.ldapGroupFilterLabel')}</label>
              <Input value={groupFilter} onChange={(e) => { setGroupFilter(e.target.value) }} placeholder={t('settingsPage.ldapGroupFilterPlaceholder')} />
            </div>
          </div>
          <div className="flex items-center gap-2">
            <Switch checked={tls} onCheckedChange={setTls} id="ldap-tls" />
            <label htmlFor="ldap-tls" className="text-sm cursor-pointer">{t('settingsPage.ldapTlsLabel')}</label>
          </div>
        </div>
        <div className="flex flex-wrap gap-2 pt-1">
          <Button size="sm" onClick={handleSave} disabled={update.isPending}>
            {saved ? t('common.saved') : t('common.save')}
          </Button>
          <Button size="sm" variant="outline" onClick={handleTest} disabled={testConn.isPending}>
            {t('settingsPage.ldapTestBtn')}
          </Button>
          <Button size="sm" variant="outline" onClick={handleSync} disabled={sync.isPending}>
            {t('settingsPage.ldapSyncBtn')}
          </Button>
        </div>
        {testMsg && <p className="text-xs text-muted-foreground">{testMsg}</p>}
        {syncMsg && <p className="text-xs text-muted-foreground">{syncMsg}</p>}
      </div>
    </SectionCard>
  )
}

// ─── Server Info ──────────────────────────────────────────────────────────────

function UpdateSection() {
  const { t } = useTranslation()
  const { data, isLoading } = useUpdateCheck()
  const toggle = useToggleUpdateCheck()

  return (
    <SectionCard title={t('settingsPage.updatesTitle')} icon={RefreshCw}>
      <div className="space-y-2 text-xs">
        <div className="flex items-center justify-between">
          <label htmlFor="update-check-toggle" className="text-secondary cursor-pointer">
            {t('settingsPage.updatesCheckEnabled')}
          </label>
          <Switch
            id="update-check-toggle"
            checked={data?.check_enabled ?? false}
            disabled={isLoading || toggle.isPending}
            onCheckedChange={(v) => { toggle.mutate(v) }}
          />
        </div>

        {isLoading && <p className="text-secondary">{t('settingsPage.updatesChecking')}</p>}

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
        {([
          ['serverApiPort', 'serverApiPortVal'],
          ['serverDatabase', 'serverDatabaseVal'],
          ['serverQueue', 'serverQueueVal'],
          ['serverEncryption', 'serverEncryptionVal'],
          ['serverAuthToken', 'serverAuthTokenVal'],
        ] as const).map(([kKey, vKey]) => (
          <div key={kKey} className="flex justify-between py-1.5 px-3 rounded-lg bg-surface2">
            <span className="text-secondary">{t(`settingsPage.${kKey}`)}</span>
            <span className="text-primary font-medium">{t(`settingsPage.${vKey}`)}</span>
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
          onClick={() => { void exportData(); }}
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
          onClick={() => { void generate(); }}
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

  const promote = useMutation({
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
                  ? t('settingsPage.stagingError2', { error: promote.error.message })
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
            <Button variant="outline" onClick={() => { setConfirming(false); }}>{t('common.cancel')}</Button>
            <Button
              onClick={() => { promote.mutate(); }}
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

// ─── Guided Backup Destination (System-Tab) ──────────────────────────────────

const BACKUP_DEST_TYPES = ['none', 'nextcloud', 's3', 'sftp', 'custom'] as const

function BackupDestSection() {
  const { t } = useTranslation()
  const { data, isLoading } = useOrgBackupDest()
  const update = useUpdateOrgBackupDest()

  const [type, setType] = useState('none')
  // nextcloud / sftp shared
  const [url, setUrl] = useState('')
  const [user, setUser] = useState('')
  const [pass, setPass] = useState('')
  const [remotePath, setRemotePath] = useState('')
  // s3
  const [endpoint, setEndpoint] = useState('')
  const [bucket, setBucket] = useState('')
  const [prefix, setPrefix] = useState('')
  const [accessKey, setAccessKey] = useState('')
  const [secretKey, setSecretKey] = useState('')
  // sftp
  const [host, setHost] = useState('')
  const [port, setPort] = useState('22')
  // custom
  const [cmd, setCmd] = useState('')
  const [saved, setSaved] = useState(false)

  useEffect(() => {
    if (!data) return
    setType(data.type || 'none')
    setUrl(data.url || '')
    setUser(data.user || '')
    setRemotePath(data.remote_path || '')
    setEndpoint(data.endpoint || '')
    setBucket(data.bucket || '')
    setPrefix(data.prefix || '')
    setAccessKey(data.access_key || '')
    setHost(data.host || '')
    setPort(data.port ? String(data.port) : '22')
    setCmd(data.cmd || '')
  }, [data])

  function handleSave() {
    update.mutate(
      {
        type,
        url,
        user,
        pass: pass || undefined,
        remote_path: remotePath,
        endpoint,
        bucket,
        prefix,
        access_key: accessKey,
        secret_key: secretKey || undefined,
        host,
        port: parseInt(port, 10) || 22,
        cmd,
      },
      {
        onSuccess: () => {
          setSaved(true)
          setPass('')
          setSecretKey('')
          setTimeout(() => { setSaved(false) }, 2000)
        },
      },
    )
  }

  if (isLoading) return (
    <SectionCard title={t('settingsPage.backupDestTitle')} icon={HardDrive}>
      <Spinner />
    </SectionCard>
  )

  return (
    <SectionCard title={t('settingsPage.backupDestTitle')} icon={HardDrive}>
      <div className="space-y-3">
        <div className="space-y-1.5">
          <Label className="text-xs">{t('settingsPage.backupDestTypeLabel')}</Label>
          <select
            className="w-full h-8 rounded-md border border-input bg-background px-3 text-sm"
            value={type}
            onChange={(e) => { setType(e.target.value) }}
          >
            {BACKUP_DEST_TYPES.map((dt) => (
              <option key={dt} value={dt}>{t(`settingsPage.backupDestType_${dt}`)}</option>
            ))}
          </select>
        </div>

        {type === 'nextcloud' && (
          <>
            <div className="space-y-1.5">
              <Label className="text-xs">{t('settingsPage.backupDestUrlLabel')}</Label>
              <Input className="h-8 text-sm" placeholder="https://cloud.example.com/remote.php/dav/files/user/" value={url} onChange={(e) => { setUrl(e.target.value) }} />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label className="text-xs">{t('settingsPage.backupDestUserLabel')}</Label>
                <Input className="h-8 text-sm" placeholder="user" value={user} onChange={(e) => { setUser(e.target.value) }} />
              </div>
              <div className="space-y-1.5">
                <Label className="text-xs">{t('settingsPage.backupDestPassLabel')}</Label>
                <Input className="h-8 text-sm" type="password" placeholder={data?.has_pass ? '••••••••' : t('settingsPage.backupDestPassPlaceholder')} value={pass} onChange={(e) => { setPass(e.target.value) }} />
              </div>
            </div>
            <div className="space-y-1.5">
              <Label className="text-xs">{t('settingsPage.backupDestRemotePathLabel')}</Label>
              <Input className="h-8 text-sm font-mono text-xs" placeholder="/vakt-backups/" value={remotePath} onChange={(e) => { setRemotePath(e.target.value) }} />
            </div>
          </>
        )}

        {type === 's3' && (
          <>
            <div className="space-y-1.5">
              <Label className="text-xs">{t('settingsPage.backupDestEndpointLabel')}</Label>
              <Input className="h-8 text-sm" placeholder="https://s3.eu-central-1.amazonaws.com" value={endpoint} onChange={(e) => { setEndpoint(e.target.value) }} />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label className="text-xs">{t('settingsPage.backupDestBucketLabel')}</Label>
                <Input className="h-8 text-sm" placeholder="my-backup-bucket" value={bucket} onChange={(e) => { setBucket(e.target.value) }} />
              </div>
              <div className="space-y-1.5">
                <Label className="text-xs">{t('settingsPage.backupDestPrefixLabel')}</Label>
                <Input className="h-8 text-sm font-mono text-xs" placeholder="vakt/" value={prefix} onChange={(e) => { setPrefix(e.target.value) }} />
              </div>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label className="text-xs">{t('settingsPage.backupDestAccessKeyLabel')}</Label>
                <Input className="h-8 text-sm font-mono text-xs" placeholder="AKIAIOSFODNN7EXAMPLE" value={accessKey} onChange={(e) => { setAccessKey(e.target.value) }} />
              </div>
              <div className="space-y-1.5">
                <Label className="text-xs">{t('settingsPage.backupDestSecretKeyLabel')}</Label>
                <Input className="h-8 text-sm" type="password" placeholder={data?.has_secret_key ? '••••••••' : t('settingsPage.backupDestSecretKeyPlaceholder')} value={secretKey} onChange={(e) => { setSecretKey(e.target.value) }} />
              </div>
            </div>
          </>
        )}

        {type === 'sftp' && (
          <>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label className="text-xs">{t('settingsPage.backupDestHostLabel')}</Label>
                <Input className="h-8 text-sm" placeholder="backup.example.com" value={host} onChange={(e) => { setHost(e.target.value) }} />
              </div>
              <div className="space-y-1.5">
                <Label className="text-xs">{t('settingsPage.backupDestPortLabel')}</Label>
                <Input className="h-8 text-sm" type="number" placeholder="22" value={port} onChange={(e) => { setPort(e.target.value) }} />
              </div>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label className="text-xs">{t('settingsPage.backupDestUserLabel')}</Label>
                <Input className="h-8 text-sm" placeholder="backup-user" value={user} onChange={(e) => { setUser(e.target.value) }} />
              </div>
              <div className="space-y-1.5">
                <Label className="text-xs">{t('settingsPage.backupDestPassLabel')}</Label>
                <Input className="h-8 text-sm" type="password" placeholder={data?.has_pass ? '••••••••' : t('settingsPage.backupDestPassPlaceholder')} value={pass} onChange={(e) => { setPass(e.target.value) }} />
              </div>
            </div>
            <div className="space-y-1.5">
              <Label className="text-xs">{t('settingsPage.backupDestRemotePathLabel')}</Label>
              <Input className="h-8 text-sm font-mono text-xs" placeholder="/home/backup-user/vakt-backups/" value={remotePath} onChange={(e) => { setRemotePath(e.target.value) }} />
            </div>
          </>
        )}

        {type === 'custom' && (
          <div className="space-y-1.5">
            <Label className="text-xs">{t('settingsPage.backupDestCmdLabel')}</Label>
            <textarea
              className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm font-mono resize-none"
              rows={3}
              placeholder='rclone copy "$ARCHIVE" myremote:bucket/vakt/'
              value={cmd}
              onChange={(e) => { setCmd(e.target.value) }}
            />
            <p className="text-[11px] text-secondary">{t('settingsPage.backupDestCmdHint')}</p>
          </div>
        )}

        <div className="flex justify-end">
          <Button size="sm" onClick={handleSave} disabled={update.isPending}>
            {saved ? t('settingsPage.saved') : update.isPending ? t('settingsPage.saving') : t('settingsPage.save')}
          </Button>
        </div>
      </div>
    </SectionCard>
  )
}

// ─── Backup-Konfiguration (System-Tab) ───────────────────────────────────────

function BackupSection() {
  const { t } = useTranslation()
  const { data, isLoading } = useOrgBackupConfig()
  const update = useUpdateOrgBackupConfig()

  const [schedule, setSchedule] = useState('0 2 * * *')
  const [retentionDays, setRetentionDays] = useState('30')
  const [passphrase, setPassphrase] = useState('')
  const [notifyWebhook, setNotifyWebhook] = useState('')
  const [offsiteCmd, setOffsiteCmd] = useState('')
  const [notifyCmd, setNotifyCmd] = useState('')
  const [saved, setSaved] = useState(false)

  useEffect(() => {
    if (data) {
      setSchedule(data.schedule || '0 2 * * *')
      setRetentionDays(String(data.retention_days || 30))
      setOffsiteCmd(data.offsite_cmd || '')
      setNotifyCmd(data.notify_cmd || '')
    }
  }, [data])

  function handleSave() {
    update.mutate(
      {
        schedule,
        retention_days: parseInt(retentionDays, 10) || 30,
        passphrase: passphrase || undefined,
        notify_webhook: notifyWebhook || undefined,
        offsite_cmd: offsiteCmd,
        notify_cmd: notifyCmd,
      },
      {
        onSuccess: () => {
          setSaved(true)
          setPassphrase('')
          setNotifyWebhook('')
          setTimeout(() => { setSaved(false) }, 2000)
        },
      },
    )
  }

  if (isLoading) return (
    <SectionCard title={t('settingsPage.backupTitle')} icon={Server}>
      <Spinner />
    </SectionCard>
  )

  return (
    <SectionCard title={t('settingsPage.backupTitle')} icon={Server}>
      <div className="space-y-3">
        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <Label className="text-xs">{t('settingsPage.backupScheduleLabel')}</Label>
            <Input className="h-8 text-sm font-mono" placeholder="0 2 * * *" value={schedule} onChange={(e) => { setSchedule(e.target.value) }} />
          </div>
          <div className="space-y-1.5">
            <Label className="text-xs">{t('settingsPage.backupRetentionLabel')}</Label>
            <Input className="h-8 text-sm" type="number" min="1" placeholder="30" value={retentionDays} onChange={(e) => { setRetentionDays(e.target.value) }} />
          </div>
        </div>
        <div className="space-y-1.5">
          <Label className="text-xs">{t('settingsPage.backupPassphraseLabel')}</Label>
          <Input className="h-8 text-sm" type="password" placeholder={data?.has_passphrase ? '••••••••' : t('settingsPage.backupPassphrasePlaceholder')} value={passphrase} onChange={(e) => { setPassphrase(e.target.value) }} />
        </div>
        <div className="space-y-1.5">
          <Label className="text-xs">{t('settingsPage.backupNotifyWebhookLabel')}</Label>
          <Input className="h-8 text-sm" placeholder={data?.has_notify_webhook ? '••••••••' : 'https://hooks.example.com/...'} value={notifyWebhook} onChange={(e) => { setNotifyWebhook(e.target.value) }} />
        </div>
        <div className="space-y-1.5">
          <Label className="text-xs">{t('settingsPage.backupOffsiteCmdLabel')}</Label>
          <Input className="h-8 text-sm font-mono text-xs" placeholder='aws s3 cp "$ARCHIVE" s3://...' value={offsiteCmd} onChange={(e) => { setOffsiteCmd(e.target.value) }} />
        </div>
        <div className="space-y-1.5">
          <Label className="text-xs">{t('settingsPage.backupNotifyCmdLabel')}</Label>
          <Input className="h-8 text-sm font-mono text-xs" placeholder='logger -t vakt "$MESSAGE"' value={notifyCmd} onChange={(e) => { setNotifyCmd(e.target.value) }} />
        </div>
        <p className="text-[11px] text-secondary">{t('settingsPage.backupScheduleHint')}</p>
        <div className="flex justify-end">
          <Button size="sm" onClick={handleSave} disabled={update.isPending}>
            {saved ? <><Check className="h-3.5 w-3.5 mr-1" />{t('common.saved')}</> : t('common.save')}
          </Button>
        </div>
        {update.isError && <p className="text-xs text-destructive">{t('settingsPage.backupSaveError')}</p>}
      </div>
    </SectionCard>
  )
}

// ─── Link-only cards for sub-pages reached via Settings hub ─────────────────

function LinkCard({
  title, icon: Icon, to, description, linkLabel,
}: {
  title: string
  icon: React.ElementType
  to: string
  description: string
  linkLabel: string
}) {
  return (
    <SectionCard title={title} icon={Icon}>
      <div className="space-y-3">
        <p className="text-xs text-secondary leading-relaxed">{description}</p>
        <Link to={to} className="inline-flex items-center gap-1.5 text-sm font-medium text-primary hover:underline">
          {linkLabel} <ExternalLink className="h-3.5 w-3.5" />
        </Link>
      </div>
    </SectionCard>
  )
}

// ─── Tabs ─────────────────────────────────────────────────────────────────────

// Labels/descriptions for SETTINGS_TABS are resolved inside the component via t()
const SETTINGS_TAB_IDS = [
  'platform', 'access', 'notifications', 'integrations', 'privacy', 'ai', 'public', 'system',
] as const

type TabId = typeof SETTINGS_TAB_IDS[number]

function isTabId(s: string): s is TabId {
  return SETTINGS_TAB_IDS.some((id) => id === s)
}

// ─── Main Page ────────────────────────────────────────────────────────────────

export default function Settings() {
  const { t } = useTranslation()

  // Deep-linking via URL hash (#access, #integrations, …)
  const initialHash = typeof window !== 'undefined' ? window.location.hash.replace('#', '') : ''
  const [tab, setTab] = useState<TabId>(isTabId(initialHash) ? initialHash : 'platform')

  useEffect(() => {
    function syncFromHash() {
      const h = window.location.hash.replace('#', '')
      if (isTabId(h)) setTab(h)
    }
    window.addEventListener('hashchange', syncFromHash)
    return () => { window.removeEventListener('hashchange', syncFromHash); }
  }, [])

  function changeTab(next: string) {
    if (!isTabId(next)) return
    setTab(next)
    if (window.location.hash.replace('#', '') !== next) {
      window.history.replaceState(null, '', `#${next}`)
    }
  }

  return (
    <div className="flex flex-col h-full">
      <PageHeader title={t('settingsPage.title')} description={t('settingsPage.description')} />
      <div className="flex-1 p-6 overflow-auto">
        <div className="max-w-5xl">
          <Tabs value={tab} onValueChange={changeTab}>
            <TabsList className="flex flex-wrap mb-6 w-full justify-start">
              {SETTINGS_TAB_IDS.map((id) => (
                <TabsTrigger key={id} value={id} title={t(`settingsPage.tab${id.charAt(0).toUpperCase()}${id.slice(1)}Desc`)}>{t(`settingsPage.tab${id.charAt(0).toUpperCase()}${id.slice(1)}`)}</TabsTrigger>
              ))}
            </TabsList>

            <TabsContent value="platform">
              <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
                <OrgSection />
                <ModulesSection />
                <SectorSection />
                <LicenseSection />
                <LinkCard
                  title={t('settingsPage.brandingTitle')}
                  icon={Palette}
                  to="/settings/branding"
                  description={t('settingsPage.brandingDesc')}
                  linkLabel={t('settingsPage.brandingLink')}
                />
                <LinkCard
                  title={t('settingsPage.scoreConfigTitle')}
                  icon={Sliders}
                  to="/settings/score-config"
                  description={t('settingsPage.scoreConfigDesc')}
                  linkLabel={t('settingsPage.scoreConfigLink')}
                />
              </div>
            </TabsContent>

            <TabsContent value="access">
              <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
                <LinkCard
                  title={t('settingsPage.teamLinkTitle')}
                  icon={Users}
                  to="/settings/team"
                  description={t('settingsPage.teamLinkDesc')}
                  linkLabel={t('settingsPage.teamLinkLabel')}
                />
                <LinkCard
                  title={t('settingsPage.auditorsTitle')}
                  icon={UserCheck}
                  to="/settings/auditors"
                  description={t('settingsPage.auditorsDesc')}
                  linkLabel={t('settingsPage.auditorsLink')}
                />
                <SAMLSetupSection />
                <OIDCConfigSection />
                <LDAPSection />
              </div>
            </TabsContent>

            <TabsContent value="notifications">
              <div className="grid grid-cols-1 lg:grid-cols-3 gap-5">
                <SmtpSection />
                <NotificationsSection />
                <DigestToggleSection />
                <LinkCard
                  title={t('settingsPage.alertRulesTitle')}
                  icon={Siren}
                  to="/settings/alerting"
                  description={t('settingsPage.alertRulesDesc')}
                  linkLabel={t('settingsPage.alertRulesLink')}
                />
                <LinkCard
                  title={t('settingsPage.personalNotifTitle')}
                  icon={Bell}
                  to="/settings/notifications"
                  description={t('settingsPage.personalNotifDesc')}
                  linkLabel={t('settingsPage.personalNotifLink')}
                />
                <LinkCard
                  title={t('settingsPage.scheduledReportsPlan')}
                  icon={FileBarChart2}
                  to="/settings/reports"
                  description={t('settingsPage.scheduledReportsDesc')}
                  linkLabel={t('settingsPage.scheduledReportsPlan')}
                />
              </div>
            </TabsContent>

            <TabsContent value="integrations">
              <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
                <LinkCard
                  title="Webhooks"
                  icon={Zap}
                  to="/settings/webhooks"
                  description={t('settingsPage.webhooksDesc')}
                  linkLabel={t('settingsPage.webhooksManage')}
                />
                <LinkCard
                  title={t('settingsPage.apiKeysTitle')}
                  icon={Key}
                  to="/settings/api-keys"
                  description={t('settingsPage.apiKeysDesc')}
                  linkLabel={t('settingsPage.apiKeysManage')}
                />
                <SIEMSection />
              </div>
            </TabsContent>

            <TabsContent value="privacy">
              <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
                <DataExportSection />
                <AuditReportSection />
                <LinkCard
                  title={t('settingsPage.retentionTitle')}
                  icon={Trash2}
                  to="/settings/retention"
                  description={t('settingsPage.retentionDesc')}
                  linkLabel={t('settingsPage.retentionLink')}
                />
              </div>
            </TabsContent>

            <TabsContent value="ai">
              <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
                <AISettingsSection />
              </div>
            </TabsContent>

            <TabsContent value="public">
              <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
                <LinkCard
                  title={t('settingsPage.trustCenterTitle')}
                  icon={Globe}
                  to="/settings/trust-center"
                  description={t('settingsPage.trustCenterDesc2')}
                  linkLabel={t('settingsPage.trustCenterConfigure2')}
                />
              </div>
            </TabsContent>

            <TabsContent value="system">
              <div className="grid grid-cols-1 lg:grid-cols-3 gap-5">
                <UpdateSection />
                <ServerSection />
                <BackupSection />
              </div>
              <div className="grid grid-cols-1 lg:grid-cols-2 gap-5 mt-5">
                <BackupDestSection />
              </div>
              <div className="mt-5">
                <StagingSection />
              </div>
            </TabsContent>
          </Tabs>
        </div>
      </div>
    </div>
  )
}
