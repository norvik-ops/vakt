import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Spinner } from '../../../components/Spinner'
import { Shield, Award, FileText, Plus, Trash2 } from 'lucide-react'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Button } from '../../../components/ui/button'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Card } from '../../../components/ui/card'
import { apiFetch } from '../../../api/client'
import type { Policy } from '../../../shared/types/policy'
import type { PaginatedResponse } from '../../../shared/types/pagination'

// ─── Types ────────────────────────────────────────────────────────────────────

interface TrustCenterSettings {
  enabled: boolean
  description: string
  contact: string
  logo_url: string
  show_frameworks: boolean
  show_policies: boolean
  show_certs: boolean
  subprocessors_md: string
}

interface Certificate {
  id: string
  name: string
  issuer?: string
  issued_at?: string
  expires_at?: string
}

interface CreateCertInput {
  name: string
  issuer: string
  issued_at: string
  expires_at: string
}

// ─── API hooks ────────────────────────────────────────────────────────────────

function useTrustCenterSettings() {
  return useQuery<{ data: TrustCenterSettings }>({
    queryKey: ['trust-center', 'settings'],
    queryFn: () => apiFetch<{ data: TrustCenterSettings }>('/trust-center/settings'),
    retry: false,
  })
}

function useUpdateTrustCenterSettings() {
  const qc = useQueryClient()
  return useMutation<unknown, Error, TrustCenterSettings>({
    mutationFn: (body) =>
      apiFetch<unknown>('/trust-center/settings', {
        method: 'PATCH',
        body: JSON.stringify(body),
      }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['trust-center', 'settings'] }),
  })
}

function useCertificates() {
  return useQuery<{ data: Certificate[] }>({
    queryKey: ['trust-center', 'certificates'],
    queryFn: () => apiFetch<{ data: Certificate[] }>('/trust-center/certificates'),
    retry: false,
  })
}

function useCreateCertificate() {
  const qc = useQueryClient()
  return useMutation<Certificate, Error, CreateCertInput>({
    mutationFn: (body) =>
      apiFetch<Certificate>('/trust-center/certificates', {
        method: 'POST',
        body: JSON.stringify(body),
      }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['trust-center', 'certificates'] }),
  })
}

function useDeleteCertificate() {
  const qc = useQueryClient()
  return useMutation<unknown, Error, string>({
    mutationFn: (id) =>
      apiFetch<unknown>(`/trust-center/certificates/${id}`, { method: 'DELETE' }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['trust-center', 'certificates'] }),
  })
}

function usePublishedPolicies() {
  return useQuery<{ data: string[] }>({
    queryKey: ['trust-center', 'policies'],
    queryFn: () => apiFetch<{ data: string[] }>('/trust-center/policies'),
    retry: false,
  })
}

function usePublishPolicy() {
  const qc = useQueryClient()
  return useMutation<unknown, Error, string>({
    mutationFn: (policyId) =>
      apiFetch<unknown>(`/trust-center/policies/${policyId}/publish`, { method: 'POST' }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['trust-center', 'policies'] }),
  })
}

function useUnpublishPolicy() {
  const qc = useQueryClient()
  return useMutation<unknown, Error, string>({
    mutationFn: (policyId) =>
      apiFetch<unknown>(`/trust-center/policies/${policyId}/publish`, { method: 'DELETE' }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['trust-center', 'policies'] }),
  })
}

function usePolicies() {
  return useQuery<Policy[]>({
    queryKey: ['vaktcomply', 'policies'],
    queryFn: async () => {
      const resp = await apiFetch<PaginatedResponse<Policy> | Policy[]>(
        '/vaktcomply/policies?page=1&limit=200',
      )
      // ListPolicies returns the paginated envelope {data, pagination}; older
      // callers fell back to a flat array, so we accept either shape.
      if (Array.isArray(resp)) return resp
      return resp.data
    },
    staleTime: 5 * 60 * 1000,
    retry: false,
  })
}

// ─── Toggle component ─────────────────────────────────────────────────────────

function Toggle({
  id,
  checked,
  onChange,
  label,
  description,
}: {
  id: string
  checked: boolean
  onChange: (v: boolean) => void
  label: string
  description?: string
}) {
  return (
    <div className="flex items-start gap-3">
      <input
        id={id}
        type="checkbox"
        checked={checked}
        onChange={(e) => { onChange(e.target.checked); }}
        className="mt-0.5 h-4 w-4 rounded border-gray-300 text-indigo-600 focus:ring-indigo-500 cursor-pointer"
      />
      <div>
        <Label htmlFor={id} className="cursor-pointer font-medium">
          {label}
        </Label>
        {description && <p className="text-xs text-muted-foreground mt-0.5">{description}</p>}
      </div>
    </div>
  )
}

// ─── Sections ─────────────────────────────────────────────────────────────────

function GeneralSection({
  settings,
  onChange,
}: {
  settings: TrustCenterSettings
  onChange: (s: TrustCenterSettings) => void
}) {
  const { t } = useTranslation()
  return (
    <div className="space-y-4">
      <Toggle
        id="tc-enabled"
        checked={settings.enabled}
        onChange={(v) => { onChange({ ...settings, enabled: v }); }}
        label={t('settings.trustCenter.enableLabel')}
        description={t('settings.trustCenter.enableDescription')}
      />
      <div className="space-y-1.5">
        <Label htmlFor="tc-description">{t('settings.trustCenter.descriptionLabel')}</Label>
        <textarea
          id="tc-description"
          rows={3}
          maxLength={300}
          placeholder={t('settings.trustCenter.descriptionPlaceholder')}
          value={settings.description}
          onChange={(e) => { onChange({ ...settings, description: e.target.value }); }}
          className="flex w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 resize-none"
        />
        <p className="text-xs text-muted-foreground text-right">{settings.description.length}/300</p>
      </div>
      <div className="space-y-1.5">
        <Label htmlFor="tc-contact">{t('settings.trustCenter.contactLabel')}</Label>
        <Input
          id="tc-contact"
          type="email"
          placeholder="security@example.com"
          value={settings.contact}
          onChange={(e) => { onChange({ ...settings, contact: e.target.value }); }}
        />
      </div>
      <div className="space-y-1.5">
        <Label htmlFor="tc-logo">{t('settings.trustCenter.logoLabel')}</Label>
        <Input
          id="tc-logo"
          type="url"
          placeholder="https://example.com/logo.png"
          value={settings.logo_url}
          onChange={(e) => { onChange({ ...settings, logo_url: e.target.value }); }}
        />
        <p className="text-xs text-muted-foreground">{t('settings.trustCenter.logoHint')}</p>
      </div>
    </div>
  )
}

function VisibilitySection({
  settings,
  onChange,
}: {
  settings: TrustCenterSettings
  onChange: (s: TrustCenterSettings) => void
}) {
  const { t } = useTranslation()
  return (
    <div className="space-y-4">
      <Toggle
        id="show-frameworks"
        checked={settings.show_frameworks}
        onChange={(v) => { onChange({ ...settings, show_frameworks: v }); }}
        label={t('settings.trustCenter.showFrameworks')}
        description={t('settings.trustCenter.showFrameworksHint')}
      />
      <Toggle
        id="show-certs"
        checked={settings.show_certs}
        onChange={(v) => { onChange({ ...settings, show_certs: v }); }}
        label={t('settings.trustCenter.showCerts')}
        description={t('settings.trustCenter.showCertsHint')}
      />
      <Toggle
        id="show-policies"
        checked={settings.show_policies}
        onChange={(v) => { onChange({ ...settings, show_policies: v }); }}
        label={t('settings.trustCenter.showPolicies')}
        description={t('settings.trustCenter.showPoliciesHint')}
      />
    </div>
  )
}

function SubprocessorsSection({
  settings,
  onChange,
}: {
  settings: TrustCenterSettings
  onChange: (s: TrustCenterSettings) => void
}) {
  const { t } = useTranslation()
  return (
    <div className="space-y-2">
      <Label htmlFor="tc-subprocessors">{t('settings.trustCenter.subprocessorsLabel')}</Label>
      <textarea
        id="tc-subprocessors"
        rows={8}
        placeholder="Liste der Unterauftragnehmer und eingesetzten Dienste..."
        value={settings.subprocessors_md}
        onChange={(e) => { onChange({ ...settings, subprocessors_md: e.target.value }); }}
        className="flex w-full rounded-md border border-input bg-background px-3 py-2 text-sm font-mono ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 resize-y"
      />
      <p className="text-xs text-muted-foreground">
        {t('settings.trustCenter.subprocessorsHint')}
      </p>
    </div>
  )
}

function CertificatesSection() {
  const { t } = useTranslation()
  const { data, isLoading } = useCertificates()
  const createCert = useCreateCertificate()
  const deleteCert = useDeleteCertificate()

  const [name, setName] = useState('')
  const [issuer, setIssuer] = useState('')
  const [issuedAt, setIssuedAt] = useState('')
  const [expiresAt, setExpiresAt] = useState('')
  const [showForm, setShowForm] = useState(false)

  const certs = data?.data ?? []

  function handleCreate() {
    if (!name.trim()) return
    createCert.mutate(
      { name: name.trim(), issuer: issuer.trim(), issued_at: issuedAt, expires_at: expiresAt },
      {
        onSuccess: () => {
          setName('')
          setIssuer('')
          setIssuedAt('')
          setExpiresAt('')
          setShowForm(false)
        },
      },
    )
  }

  return (
    <div className="space-y-4">
      {isLoading && (
        <div className="flex items-center justify-center h-12">
          <Spinner size="sm" />
        </div>
      )}
      {!isLoading && certs.length === 0 && (
        <p className="text-sm text-muted-foreground">{t('settings.trustCenter.noCertificates')}</p>
      )}
      {!isLoading && certs.map((cert) => (
        <div key={cert.id} className="flex items-center justify-between p-3 rounded-lg border bg-surface2 gap-4">
          <div className="flex items-center gap-3 min-w-0">
            <Award className="h-4 w-4 text-indigo-500 shrink-0" />
            <div className="min-w-0">
              <div className="text-sm font-medium text-primary truncate">{cert.name}</div>
              <div className="text-xs text-muted-foreground">
                {cert.issuer && <span>{cert.issuer}</span>}
                {cert.expires_at && <span className="ml-2">· {t('settings.trustCenter.validUntil')} {cert.expires_at}</span>}
              </div>
            </div>
          </div>
          <button
            onClick={() => { deleteCert.mutate(cert.id); }}
            disabled={deleteCert.isPending}
            className="p-1.5 rounded text-muted-foreground hover:text-red-500 hover:bg-red-50 transition-colors shrink-0"
          >
            <Trash2 className="h-3.5 w-3.5" />
          </button>
        </div>
      ))}

      {showForm && (
        <div className="border rounded-lg p-4 space-y-3 bg-gray-50">
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label htmlFor="cert-name">Name *</Label>
              <Input
                id="cert-name"
                placeholder="ISO 27001 Zertifikat"
                value={name}
                onChange={(e) => { setName(e.target.value); }}
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="cert-issuer">{t('settings.trustCenter.certIssuerLabel')}</Label>
              <Input
                id="cert-issuer"
                placeholder="TÜV SÜD"
                value={issuer}
                onChange={(e) => { setIssuer(e.target.value); }}
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="cert-issued">{t('settings.trustCenter.certIssuedLabel')}</Label>
              <Input
                id="cert-issued"
                type="date"
                value={issuedAt}
                onChange={(e) => { setIssuedAt(e.target.value); }}
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="cert-expires">{t('settings.trustCenter.certExpiresLabel')}</Label>
              <Input
                id="cert-expires"
                type="date"
                value={expiresAt}
                onChange={(e) => { setExpiresAt(e.target.value); }}
              />
            </div>
          </div>
          <div className="flex gap-2">
            <Button
              size="sm"
              onClick={handleCreate}
              disabled={!name.trim() || createCert.isPending}
            >
              {createCert.isPending ? t('settings.trustCenter.saving') : t('settings.trustCenter.add')}
            </Button>
            <Button size="sm" variant="outline" onClick={() => { setShowForm(false); }}>
              {t('settings.trustCenter.cancel')}
            </Button>
          </div>
          {createCert.isError && (
            <p className="text-xs text-destructive">{createCert.error.message}</p>
          )}
        </div>
      )}

      {!showForm && (
        <Button size="sm" variant="outline" onClick={() => { setShowForm(true); }} className="mt-2">
          <Plus className="h-3.5 w-3.5 mr-1.5" />
          {t('settings.trustCenter.addCertificate')}
        </Button>
      )}
    </div>
  )
}

function PoliciesSection() {
  const { t } = useTranslation()
  const { data: allPoliciesData, isLoading: policiesLoading } = usePolicies()
  const { data: publishedData, isLoading: publishedLoading } = usePublishedPolicies()
  const publishPolicy = usePublishPolicy()
  const unpublishPolicy = useUnpublishPolicy()

  const allPolicies = allPoliciesData ?? []
  const publishedIds = new Set(publishedData?.data ?? [])

  const isLoading = policiesLoading || publishedLoading

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-12">
        <Spinner size="sm" />
      </div>
    )
  }

  if (allPolicies.length === 0) {
    return (
      <p className="text-sm text-muted-foreground">
        {t('settings.trustCenter.noPolicies')}
      </p>
    )
  }

  function togglePublish(policyId: string, isPublished: boolean) {
    if (isPublished) {
      unpublishPolicy.mutate(policyId)
    } else {
      publishPolicy.mutate(policyId)
    }
  }

  return (
    <div className="space-y-2">
      {allPolicies.map((policy) => {
        const isPublished = publishedIds.has(policy.id)
        return (
          <div
            key={policy.id}
            className="flex items-center justify-between p-3 rounded-lg border bg-surface2 gap-4"
          >
            <div className="flex items-center gap-3 min-w-0">
              <FileText className="h-4 w-4 text-indigo-500 shrink-0" />
              <div className="min-w-0">
                <div className="text-sm font-medium text-primary truncate">{policy.title}</div>
                <div className="text-xs text-muted-foreground">{policy.status}</div>
              </div>
            </div>
            <div className="flex items-center gap-2 shrink-0">
              {isPublished && (
                <span className="text-xs text-green-600 font-medium">{t('settings.trustCenter.published')}</span>
              )}
              <input
                type="checkbox"
                checked={isPublished}
                onChange={() => { togglePublish(policy.id, isPublished); }}
                disabled={publishPolicy.isPending || unpublishPolicy.isPending}
                className="h-4 w-4 rounded border-gray-300 text-indigo-600 focus:ring-indigo-500 cursor-pointer"
              />
            </div>
          </div>
        )
      })}
      <p className="text-xs text-muted-foreground pt-1">
        {t('settings.trustCenter.policyHint')}
      </p>
    </div>
  )
}

// ─── Main Page ────────────────────────────────────────────────────────────────

export default function TrustCenterSettingsPage() {
  const { t } = useTranslation()
  const { data, isLoading, isError } = useTrustCenterSettings()
  const updateSettings = useUpdateTrustCenterSettings()
  const [settings, setSettings] = useState<TrustCenterSettings>({
    enabled: false,
    description: '',
    contact: '',
    logo_url: '',
    show_frameworks: true,
    show_policies: false,
    show_certs: true,
    subprocessors_md: '',
  })
  const [saved, setSaved] = useState(false)

  useEffect(() => {
    if (data?.data) {
      setSettings(data.data)
    }
  }, [data])

  function handleSave() {
    setSaved(false)
    updateSettings.mutate(settings, {
      onSuccess: () => { setSaved(true); },
      onError: () => { setSaved(false); },
    })
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Spinner size="lg" />
      </div>
    )
  }

  if (isError) {
    return (
      <div className="p-6">
        <p className="text-sm text-destructive">{t('settings.trustCenter.loadError')}</p>
      </div>
    )
  }

  return (
    <div className="space-y-6 p-6">
      <PageHeader
        title={t('settings.trustCenter.title')}
        description={t('settings.trustCenter.description')}
      />

      {/* Allgemein */}
      <Card className="p-6 space-y-4">
        <div className="flex items-center gap-2">
          <Shield className="w-5 h-5 text-secondary" />
          <h2 className="text-base font-semibold">{t('settings.trustCenter.general')}</h2>
        </div>
        <GeneralSection settings={settings} onChange={setSettings} />
      </Card>

      {/* Sichtbarkeit */}
      <Card className="p-6 space-y-4">
        <h2 className="text-base font-semibold">{t('settings.trustCenter.visibility')}</h2>
        <VisibilitySection settings={settings} onChange={setSettings} />
      </Card>

      {/* Unterauftragnehmer */}
      <Card className="p-6 space-y-4">
        <h2 className="text-base font-semibold">{t('settings.trustCenter.subprocessors')}</h2>
        <SubprocessorsSection settings={settings} onChange={setSettings} />
      </Card>

      {/* Save button for settings */}
      <div className="flex items-center gap-3">
        <Button onClick={handleSave} disabled={updateSettings.isPending}>
          {updateSettings.isPending ? t('settings.trustCenter.saving') : t('settings.trustCenter.saveSettings')}
        </Button>
        {saved && <span className="text-sm text-green-600">{t('settings.trustCenter.saved')}</span>}
        {updateSettings.isError && (
          <span className="text-sm text-destructive">{updateSettings.error.message}</span>
        )}
      </div>

      {/* Zertifikate */}
      <Card className="p-6 space-y-4">
        <div className="flex items-center gap-2">
          <Award className="w-5 h-5 text-secondary" />
          <h2 className="text-base font-semibold">{t('settings.trustCenter.certificates')}</h2>
        </div>
        <CertificatesSection />
      </Card>

      {/* Policies veröffentlichen */}
      <Card className="p-6 space-y-4">
        <div className="flex items-center gap-2">
          <FileText className="w-5 h-5 text-secondary" />
          <h2 className="text-base font-semibold">{t('settings.trustCenter.publishPolicies')}</h2>
        </div>
        <PoliciesSection />
      </Card>
    </div>
  )
}
