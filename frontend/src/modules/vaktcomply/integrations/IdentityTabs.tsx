import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { RefreshCw } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import {
  useEntraIDConfig,
  useSaveEntraIDConfig,
  useSyncEntraID,
  useEntraIDStatus,
  useEntraIDEvidence,
  useIntuneConfig,
  useSaveIntuneConfig,
  useSyncIntune,
  useIntuneStatus,
  useIntuneEvidence,
  useKeycloakConfig,
  useSaveKeycloakConfig,
  useSyncKeycloak,
  useKeycloakStatus,
  useKeycloakEvidence,
  useLDAPConfig,
  useSaveLDAPConfig,
  useSyncLDAP,
  useLDAPStatus,
  useLDAPEvidence,
} from '../../../hooks/useCloud'
import { toast } from '../../../shared/hooks/useToast'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'
import { SyncLastBadge, RecentEvidenceList } from './shared'

// --- Entra ID tab ---

export function EntraIDTab() {
  const { t } = useTranslation()
  const { data: cfg, isLoading } = useEntraIDConfig()
  const { data: status } = useEntraIDStatus()
  const { data: evidence } = useEntraIDEvidence()
  const saveConfig = useSaveEntraIDConfig()
  const syncEntraID = useSyncEntraID()
  const { formatDateTime } = useFormatDate()

  const [tenantID, setTenantID] = useState('')
  const [clientID, setClientID] = useState('')
  const [clientSecret, setClientSecret] = useState('')
  const [initialized, setInitialized] = useState(false)

  if (cfg && !initialized) {
    setTenantID(cfg.tenant_id)
    setClientID(cfg.client_id)
    setClientSecret(cfg.client_secret)
    setInitialized(true)
  }

  async function handleSave(e: React.FormEvent) {
    e.preventDefault()
    try {
      await saveConfig.mutateAsync({ tenant_id: tenantID, client_id: clientID, client_secret: clientSecret })
      toast(t('integrations.page.saved'), 'success')
    } catch (err) {
      toast(err instanceof Error ? err.message : t('integrations.page.saveFailed'), 'error')
    }
  }

  async function handleSync() {
    try {
      const result = await syncEntraID.mutateAsync()
      if (result.ok) {
        toast(`${t('integrations.page.saved')} — ${result.evidence_created} Evidence-Einträge erstellt`, 'success')
      } else {
        toast(result.error ?? t('common.error'), 'error')
      }
    } catch (err) {
      toast(err instanceof Error ? err.message : t('common.error'), 'error')
    }
  }

  if (isLoading) return <div className="flex items-center justify-center h-32"><Spinner size="md" /></div>

  const lastSyncFormatted = status?.last_sync_at
    ? formatDateTime(status.last_sync_at, { dateStyle: 'short', timeStyle: 'short' })
    : null

  return (
    <div>
      <div className="mb-4">
        <h2 className="text-sm font-semibold text-primary">Microsoft Entra ID (Azure AD) Integration</h2>
        <p className="text-xs text-secondary mt-0.5">
          MFA-Enrollment, Conditional Access, Risky Users und Admin-Rollen täglich als Identity-Evidence erfassen.
          Benötigt App Registration mit Application Permissions (User.Read.All, Policy.Read.All, IdentityRiskyUser.Read.All).
        </p>
      </div>

      {status && (
        <div className="flex items-center gap-3 mb-5 p-3 rounded-lg border border-border bg-surface">
          <div className="flex-1 min-w-0">
            <p className="text-xs text-secondary">
              {lastSyncFormatted ? t('integrations.page.lastSync', { date: lastSyncFormatted }) : t('integrations.page.neverSynced')}
              {status.evidence_count > 0 && ` · ${status.evidence_count} Evidence-Einträge`}
              {status.mfa_enrollment_pct > 0 && ` · MFA ${Math.round(status.mfa_enrollment_pct)}%`}
              {status.risky_user_count > 0 && ` · ${status.risky_user_count} Risky Users`}
            </p>
            {status.last_sync_error && <p className="text-xs text-red-500 truncate mt-0.5">{status.last_sync_error}</p>}
          </div>
          <SyncLastBadge status={status.last_sync_status} lastSyncAt={status.last_sync_at} />
          <button onClick={() => { void handleSync() }} disabled={syncEntraID.isPending || !cfg?.is_configured}
            title="Jetzt synchronisieren"
            className="p-1.5 rounded-md text-secondary hover:text-primary hover:bg-bg transition-colors disabled:opacity-50">
            <RefreshCw className={`w-4 h-4 ${syncEntraID.isPending ? 'animate-spin' : ''}`} />
          </button>
        </div>
      )}

      <form onSubmit={(e) => { void handleSave(e) }} className="space-y-4 max-w-lg">
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Tenant ID</label>
          <input type="text" value={tenantID} onChange={(e) => { setTenantID(e.target.value); }}
            placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30 font-mono"
            required />
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Client ID (Application ID)</label>
          <input type="text" value={clientID} onChange={(e) => { setClientID(e.target.value); }}
            placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30 font-mono"
            required />
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Client Secret</label>
          <input type="password" value={clientSecret} onChange={(e) => { setClientSecret(e.target.value); }}
            placeholder={cfg?.is_configured && cfg.client_secret === '****' ? '****' : 'App-Secret aus Azure Portal'}
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30"
            required />
          <p className="text-[11px] text-secondary mt-1">Secret wird AES-256-GCM verschlüsselt gespeichert.</p>
        </div>
        <div className="flex gap-2 pt-1">
          <button type="submit" disabled={saveConfig.isPending}
            className="px-4 py-1.5 text-xs font-medium bg-brand text-white rounded-md hover:bg-brand/90 transition-colors disabled:opacity-50">
            {saveConfig.isPending ? t('integrations.page.saving') : t('integrations.page.save')}
          </button>
        </div>
      </form>

      {evidence && evidence.length > 0 && (
        <div className="mt-6">
          <p className="text-xs font-medium text-secondary mb-2">{t('integrations.page.recentEvidence')}</p>
          <RecentEvidenceList items={evidence} />
        </div>
      )}
    </div>
  )
}

// --- Intune tab (S88-7) ---

export function IntuneTab() {
  const { t } = useTranslation()
  const { data: cfg, isLoading } = useIntuneConfig()
  const { data: status } = useIntuneStatus()
  const { data: evidence } = useIntuneEvidence()
  const saveConfig = useSaveIntuneConfig()
  const syncIntune = useSyncIntune()
  const { formatDateTime } = useFormatDate()

  const [tenantID, setTenantID] = useState('')
  const [clientID, setClientID] = useState('')
  const [clientSecret, setClientSecret] = useState('')
  const [initialized, setInitialized] = useState(false)

  if (cfg && !initialized) {
    setTenantID(cfg.tenant_id)
    setClientID(cfg.client_id)
    setClientSecret(cfg.client_secret)
    setInitialized(true)
  }

  async function handleSave(e: React.FormEvent) {
    e.preventDefault()
    try {
      await saveConfig.mutateAsync({ tenant_id: tenantID, client_id: clientID, client_secret: clientSecret })
      toast(t('integrations.page.saved'), 'success')
    } catch (err) {
      toast(err instanceof Error ? err.message : t('integrations.page.saveFailed'), 'error')
    }
  }

  async function handleSync() {
    try {
      const result = await syncIntune.mutateAsync()
      if (result.ok) {
        toast(`${t('integrations.page.saved')} — ${result.evidence_created} Evidence-Einträge erstellt`, 'success')
      } else {
        toast(result.error ?? t('common.error'), 'error')
      }
    } catch (err) {
      toast(err instanceof Error ? err.message : t('common.error'), 'error')
    }
  }

  if (isLoading) return <div className="flex items-center justify-center h-32"><Spinner size="md" /></div>

  const lastSyncFormatted = status?.last_sync_at
    ? formatDateTime(status.last_sync_at, { dateStyle: 'short', timeStyle: 'short' })
    : null

  return (
    <div>
      <div className="mb-4">
        <h2 className="text-sm font-semibold text-primary">{t('integrations.intune.title')}</h2>
        <p className="text-xs text-secondary mt-0.5">{t('integrations.intune.description')}</p>
      </div>

      {status && (
        <div className="flex items-center gap-3 mb-5 p-3 rounded-lg border border-border bg-surface">
          <div className="flex-1 min-w-0">
            <p className="text-xs text-secondary">
              {lastSyncFormatted ? t('integrations.page.lastSync', { date: lastSyncFormatted }) : t('integrations.page.neverSynced')}
              {status.evidence_count > 0 && ` · ${status.evidence_count} Evidence-Einträge`}
              {status.device_compliance_pct > 0 && ` · ${t('integrations.intune.compliance')} ${Math.round(status.device_compliance_pct)}%`}
            </p>
            {status.last_sync_error && <p className="text-xs text-red-500 truncate mt-0.5">{status.last_sync_error}</p>}
          </div>
          <SyncLastBadge status={status.last_sync_status} lastSyncAt={status.last_sync_at} />
          <button onClick={() => { void handleSync() }} disabled={syncIntune.isPending || !cfg?.is_configured}
            title={t('integrations.page.syncNow')}
            className="p-1.5 rounded-md text-secondary hover:text-primary hover:bg-bg transition-colors disabled:opacity-50">
            <RefreshCw className={`w-4 h-4 ${syncIntune.isPending ? 'animate-spin' : ''}`} />
          </button>
        </div>
      )}

      <form onSubmit={(e) => { void handleSave(e) }} className="space-y-4 max-w-lg">
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Tenant ID</label>
          <input type="text" value={tenantID} onChange={(e) => { setTenantID(e.target.value); }}
            placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30 font-mono"
            required />
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Client ID (Application ID)</label>
          <input type="text" value={clientID} onChange={(e) => { setClientID(e.target.value); }}
            placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30 font-mono"
            required />
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Client Secret</label>
          <input type="password" value={clientSecret} onChange={(e) => { setClientSecret(e.target.value); }}
            placeholder={cfg?.is_configured && cfg.client_secret === '****' ? '****' : 'App-Secret aus Azure Portal'}
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30"
            required />
          <p className="text-[11px] text-secondary mt-1">{t('integrations.intune.permissionsHint')}</p>
        </div>
        <div className="flex gap-2 pt-1">
          <button type="submit" disabled={saveConfig.isPending}
            className="px-4 py-1.5 text-xs font-medium bg-brand text-white rounded-md hover:bg-brand/90 transition-colors disabled:opacity-50">
            {saveConfig.isPending ? t('integrations.page.saving') : t('integrations.page.save')}
          </button>
        </div>
      </form>

      {evidence && evidence.length > 0 && (
        <div className="mt-6">
          <p className="text-xs font-medium text-secondary mb-2">{t('integrations.page.recentEvidence')}</p>
          <RecentEvidenceList items={evidence} />
        </div>
      )}
    </div>
  )
}

// --- Keycloak tab ---

export function KeycloakTab() {
  const { t } = useTranslation()
  const { data: cfg, isLoading } = useKeycloakConfig()
  const { data: status } = useKeycloakStatus()
  const { data: evidence } = useKeycloakEvidence()
  const saveConfig = useSaveKeycloakConfig()
  const syncKeycloak = useSyncKeycloak()
  const { formatDateTime } = useFormatDate()

  const [keycloakURL, setKeycloakURL] = useState('')
  const [realm, setRealm] = useState('')
  const [clientID, setClientID] = useState('')
  const [clientSecret, setClientSecret] = useState('')
  const [initialized, setInitialized] = useState(false)

  if (cfg && !initialized) {
    setKeycloakURL(cfg.keycloak_url)
    setRealm(cfg.realm)
    setClientID(cfg.client_id)
    setClientSecret(cfg.client_secret)
    setInitialized(true)
  }

  async function handleSave(e: React.FormEvent) {
    e.preventDefault()
    try {
      await saveConfig.mutateAsync({
        keycloak_url: keycloakURL,
        realm,
        client_id: clientID,
        client_secret: clientSecret,
      })
      toast(t('integrations.page.saved'), 'success')
    } catch (err) {
      toast(err instanceof Error ? err.message : t('integrations.page.saveFailed'), 'error')
    }
  }

  async function handleSync() {
    try {
      const result = await syncKeycloak.mutateAsync()
      if (result.ok) {
        toast(`${t('integrations.page.saved')} — ${result.evidence_created} Evidence-Einträge erstellt`, 'success')
      } else {
        toast(result.error ?? t('common.error'), 'error')
      }
    } catch (err) {
      toast(err instanceof Error ? err.message : t('common.error'), 'error')
    }
  }

  if (isLoading) return <div className="flex items-center justify-center h-32"><Spinner size="md" /></div>

  const lastSyncFormatted = status?.last_sync_at
    ? formatDateTime(status.last_sync_at, { dateStyle: 'short', timeStyle: 'short' })
    : null

  return (
    <div>
      <div className="mb-4">
        <h2 className="text-sm font-semibold text-primary">Keycloak REST Collector</h2>
        <p className="text-xs text-secondary mt-0.5">
          MFA-Status, Password-Policy, inaktive Accounts und Admin-Rollen aus Keycloak als Compliance-Evidence erfassen.
          Service Account benötigt &apos;view-users&apos; und &apos;view-realm&apos; Rollen.
        </p>
      </div>

      {status && (
        <div className="flex items-center gap-3 mb-5 p-3 rounded-lg border border-border bg-surface">
          <div className="flex-1 min-w-0">
            <p className="text-xs text-secondary">
              {lastSyncFormatted ? t('integrations.page.lastSync', { date: lastSyncFormatted }) : t('integrations.page.neverSynced')}
              {status.evidence_count > 0 && ` · ${status.evidence_count} Evidence-Einträge`}
              {status.user_count > 0 && ` · ${status.user_count} User`}
              {status.mfa_enrollment_pct > 0 && ` · MFA ${Math.round(status.mfa_enrollment_pct)}%`}
            </p>
            {status.last_sync_error && <p className="text-xs text-red-500 truncate mt-0.5">{status.last_sync_error}</p>}
          </div>
          <SyncLastBadge status={status.last_sync_status} lastSyncAt={status.last_sync_at} />
          <button onClick={() => { void handleSync() }} disabled={syncKeycloak.isPending || !cfg?.is_configured}
            title="Jetzt synchronisieren"
            className="p-1.5 rounded-md text-secondary hover:text-primary hover:bg-bg transition-colors disabled:opacity-50">
            <RefreshCw className={`w-4 h-4 ${syncKeycloak.isPending ? 'animate-spin' : ''}`} />
          </button>
        </div>
      )}

      <form onSubmit={(e) => { void handleSave(e) }} className="space-y-4 max-w-lg">
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Keycloak URL</label>
          <input type="url" value={keycloakURL} onChange={(e) => { setKeycloakURL(e.target.value); }}
            placeholder="https://keycloak.example.com"
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30 font-mono"
            required />
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Realm</label>
          <input type="text" value={realm} onChange={(e) => { setRealm(e.target.value); }}
            placeholder="master"
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30"
            required />
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Client ID</label>
          <input type="text" value={clientID} onChange={(e) => { setClientID(e.target.value); }}
            placeholder="vakt-collector"
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30"
            required />
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Client Secret</label>
          <input type="password" value={clientSecret} onChange={(e) => { setClientSecret(e.target.value); }}
            placeholder={cfg?.is_configured && cfg.client_secret === '****' ? '****' : 'Service-Account-Secret'}
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30"
            required />
        </div>
        <div className="flex gap-2 pt-1">
          <button type="submit" disabled={saveConfig.isPending}
            className="px-4 py-1.5 text-xs font-medium bg-brand text-white rounded-md hover:bg-brand/90 transition-colors disabled:opacity-50">
            {saveConfig.isPending ? t('integrations.page.saving') : t('integrations.page.save')}
          </button>
        </div>
      </form>

      {evidence && evidence.length > 0 && (
        <div className="mt-6">
          <p className="text-xs font-medium text-secondary mb-2">{t('integrations.page.recentEvidence')}</p>
          <RecentEvidenceList items={evidence} />
        </div>
      )}
    </div>
  )
}

// --- LDAP tab ---

export function LDAPTab() {
  const { t } = useTranslation()
  const { data: cfg, isLoading } = useLDAPConfig()
  const { data: status } = useLDAPStatus()
  const { data: evidence } = useLDAPEvidence()
  const saveConfig = useSaveLDAPConfig()
  const syncLDAP = useSyncLDAP()
  const { formatDateTime } = useFormatDate()

  const [host, setHost] = useState('')
  const [port, setPort] = useState('389')
  const [bindDN, setBindDN] = useState('')
  const [bindPassword, setBindPassword] = useState('')
  const [baseDN, setBaseDN] = useState('')
  const [useTLS, setUseTLS] = useState(false)
  const [isAD, setIsAD] = useState(true)
  const [privilegedGroups, setPrivilegedGroups] = useState('Domain Admins,Administrators')
  const [initialized, setInitialized] = useState(false)

  if (cfg && !initialized) {
    setHost(cfg.host)
    setPort(String(cfg.port || 389))
    setBindDN(cfg.bind_dn)
    setBindPassword(cfg.bind_password)
    setBaseDN(cfg.base_dn)
    setUseTLS(cfg.use_tls)
    setIsAD(cfg.is_active_directory)
    setPrivilegedGroups((cfg.privileged_groups ?? []).join(',') || 'Domain Admins,Administrators')
    setInitialized(true)
  }

  async function handleSave(e: React.FormEvent) {
    e.preventDefault()
    try {
      await saveConfig.mutateAsync({
        host,
        port: parseInt(port, 10),
        bind_dn: bindDN,
        bind_password: bindPassword,
        base_dn: baseDN,
        use_tls: useTLS,
        is_active_directory: isAD,
        privileged_groups: privilegedGroups.split(',').map((g) => g.trim()).filter(Boolean),
      })
      toast(t('integrations.page.saved'), 'success')
    } catch (err) {
      toast(err instanceof Error ? err.message : t('integrations.page.saveFailed'), 'error')
    }
  }

  async function handleSync() {
    try {
      const result = await syncLDAP.mutateAsync()
      if (result.ok) {
        toast(`${t('integrations.page.saved')} — ${result.evidence_created} Evidence-Einträge erstellt`, 'success')
      } else {
        toast(result.error ?? t('common.error'), 'error')
      }
    } catch (err) {
      toast(err instanceof Error ? err.message : t('common.error'), 'error')
    }
  }

  if (isLoading) return <div className="flex items-center justify-center h-32"><Spinner size="md" /></div>

  const lastSyncFormatted = status?.last_sync_at
    ? formatDateTime(status.last_sync_at, { dateStyle: 'short', timeStyle: 'short' })
    : null

  return (
    <div>
      <div className="mb-4">
        <h2 className="text-sm font-semibold text-primary">LDAP / Active Directory Collector</h2>
        <p className="text-xs text-secondary mt-0.5">
          Inaktive Accounts, Password-Hygiene und privilegierte Gruppen aus Active Directory oder OpenLDAP als Evidence erfassen.
          Service Account benötigt Read-Zugriff auf das Verzeichnis.
        </p>
      </div>

      {status && (
        <div className="flex items-center gap-3 mb-5 p-3 rounded-lg border border-border bg-surface">
          <div className="flex-1 min-w-0">
            <p className="text-xs text-secondary">
              {lastSyncFormatted ? t('integrations.page.lastSync', { date: lastSyncFormatted }) : t('integrations.page.neverSynced')}
              {status.evidence_count > 0 && ` · ${status.evidence_count} Evidence-Einträge`}
              {status.user_count > 0 && ` · ${status.user_count} aktive User`}
              {status.inactive_count > 0 && ` · ${status.inactive_count} inaktiv`}
              {status.privileged_count > 0 && ` · ${status.privileged_count} privilegiert`}
            </p>
            {status.last_sync_error && <p className="text-xs text-red-500 truncate mt-0.5">{status.last_sync_error}</p>}
          </div>
          <SyncLastBadge status={status.last_sync_status} lastSyncAt={status.last_sync_at} />
          <button onClick={() => { void handleSync() }} disabled={syncLDAP.isPending || !cfg?.is_configured}
            title="Jetzt synchronisieren"
            className="p-1.5 rounded-md text-secondary hover:text-primary hover:bg-bg transition-colors disabled:opacity-50">
            <RefreshCw className={`w-4 h-4 ${syncLDAP.isPending ? 'animate-spin' : ''}`} />
          </button>
        </div>
      )}

      <form onSubmit={(e) => { void handleSave(e) }} className="space-y-4 max-w-lg">
        <div className="grid grid-cols-3 gap-3">
          <div className="col-span-2">
            <label className="block text-xs font-medium text-secondary mb-1">LDAP-Host</label>
            <input type="text" value={host} onChange={(e) => { setHost(e.target.value); }}
              placeholder="dc.example.com"
              className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30 font-mono"
              required />
          </div>
          <div>
            <label className="block text-xs font-medium text-secondary mb-1">Port</label>
            <input type="number" value={port} onChange={(e) => { setPort(e.target.value); }}
              placeholder="389"
              className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary focus:outline-none focus:ring-2 focus:ring-brand/30"
              required />
          </div>
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Bind DN (Service Account)</label>
          <input type="text" value={bindDN} onChange={(e) => { setBindDN(e.target.value); }}
            placeholder="CN=vakt-svc,OU=ServiceAccounts,DC=example,DC=com"
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30 font-mono"
            required />
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Bind Password</label>
          <input type="password" value={bindPassword} onChange={(e) => { setBindPassword(e.target.value); }}
            placeholder={cfg?.is_configured && cfg.bind_password === '****' ? '****' : 'Service-Account-Passwort'}
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30"
            required />
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Base DN</label>
          <input type="text" value={baseDN} onChange={(e) => { setBaseDN(e.target.value); }}
            placeholder="DC=example,DC=com"
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30 font-mono"
            required />
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Privilegierte Gruppen (kommagetrennt)</label>
          <input type="text" value={privilegedGroups} onChange={(e) => { setPrivilegedGroups(e.target.value); }}
            placeholder="Domain Admins,Administrators"
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30" />
        </div>
        <div className="flex flex-col gap-2">
          <div className="flex items-center gap-2">
            <input type="checkbox" id="ldap-tls" checked={useTLS} onChange={(e) => { setUseTLS(e.target.checked); }}
              className="rounded border-border" />
            <label htmlFor="ldap-tls" className="text-xs text-secondary">LDAPS / STARTTLS verwenden (Port 636)</label>
          </div>
          <div className="flex items-center gap-2">
            <input type="checkbox" id="ldap-ad" checked={isAD} onChange={(e) => { setIsAD(e.target.checked); }}
              className="rounded border-border" />
            <label htmlFor="ldap-ad" className="text-xs text-secondary">Active Directory Modus (Windows FILETIME für lastLogon)</label>
          </div>
        </div>
        <div className="flex gap-2 pt-1">
          <button type="submit" disabled={saveConfig.isPending}
            className="px-4 py-1.5 text-xs font-medium bg-brand text-white rounded-md hover:bg-brand/90 transition-colors disabled:opacity-50">
            {saveConfig.isPending ? t('integrations.page.saving') : t('integrations.page.save')}
          </button>
        </div>
      </form>

      {evidence && evidence.length > 0 && (
        <div className="mt-6">
          <p className="text-xs font-medium text-secondary mb-2">{t('integrations.page.recentEvidence')}</p>
          <RecentEvidenceList items={evidence} />
        </div>
      )}
    </div>
  )
}
