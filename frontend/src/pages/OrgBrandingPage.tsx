import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { useMutation, useQuery } from '@tanstack/react-query'
import { FileImage, Info, Shield, ExternalLink } from 'lucide-react'
import { PageHeader } from '../shared/components/PageHeader'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { Card } from '../components/ui/card'
import { apiFetch } from '../api/client'

// ─── Types ────────────────────────────────────────────────────────────────────

interface OrgPayload {
  logo_url: string
}

interface CurrentOrg {
  id: string
  name: string
  slug: string
  trust_center_enabled: boolean
  trust_center_description: string
  trust_center_contact: string
}

interface TrustCenterPayload {
  enabled: boolean
  description: string
  contact: string
}

// ─── Hooks ────────────────────────────────────────────────────────────────────

function useUpdateOrg() {
  return useMutation<unknown, Error, OrgPayload>({
    mutationFn: (body) =>
      apiFetch<unknown>('/admin/org', {
        method: 'PUT',
        body: JSON.stringify(body),
      }),
  })
}

function useCurrentOrg() {
  return useQuery<{ data: CurrentOrg }>({
    queryKey: ['admin', 'org'],
    queryFn: () => apiFetch<{ data: CurrentOrg }>('/admin/org'),
    retry: false,
  })
}

function useUpdateTrustCenter() {
  return useMutation<unknown, Error, TrustCenterPayload>({
    mutationFn: (body) =>
      apiFetch<unknown>('/admin/trust-center', {
        method: 'PUT',
        body: JSON.stringify(body),
      }),
  })
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function OrgBrandingPage() {
  const { t } = useTranslation()
  const [logoUrl, setLogoUrl] = useState('')
  const [saved, setSaved] = useState(false)
  const update = useUpdateOrg()

  // Trust Center state
  const { data: orgData } = useCurrentOrg()
  const org = orgData?.data
  const [tcEnabled, setTcEnabled] = useState(false)
  const [tcDescription, setTcDescription] = useState('')
  const [tcContact, setTcContact] = useState('')
  const [tcSaved, setTcSaved] = useState(false)
  const updateTrustCenter = useUpdateTrustCenter()

  // Sync trust center fields from fetched org data
  useEffect(() => {
    if (org) {
      setTcEnabled(org.trust_center_enabled)
      setTcDescription(org.trust_center_description)
      setTcContact(org.trust_center_contact)
    }
  }, [org])

  function handleSave() {
    setSaved(false)
    update.mutate(
      { logo_url: logoUrl },
      {
        onSuccess: () => { setSaved(true); },
        onError: () => { setSaved(false); },
      },
    )
  }

  function handleTrustCenterSave() {
    setTcSaved(false)
    updateTrustCenter.mutate(
      { enabled: tcEnabled, description: tcDescription, contact: tcContact },
      {
        onSuccess: () => { setTcSaved(true); },
        onError: () => { setTcSaved(false); },
      },
    )
  }

  const isValidUrl = logoUrl === '' || logoUrl.startsWith('http://') || logoUrl.startsWith('https://')
  const trustCenterUrl = org ? `/trust/${org.slug}` : null

  return (
    <div className="space-y-6 p-6">
      <PageHeader
        title={t('orgBranding.title')}
        description={t('orgBranding.description')}
      />

      {/* Logo URL card */}
      <Card className="p-6 space-y-4">
        <div className="flex items-center gap-2">
          <FileImage className="w-5 h-5 text-secondary" />
          <h2 className="text-base font-semibold">{t('orgBranding.logoTitle')}</h2>
        </div>

        <div className="space-y-2">
          <Label htmlFor="logo-url">{t('orgBranding.logoLabel')}</Label>
          <Input
            id="logo-url"
            type="url"
            placeholder="https://example.com/logo.png"
            value={logoUrl}
            onChange={(e) => {
              setLogoUrl(e.target.value)
              setSaved(false)
            }}
          />
          {!isValidUrl && (
            <p className="text-xs text-destructive">{t('orgBranding.invalidUrl')}</p>
          )}
          <p className="text-xs text-muted-foreground">
            {t('orgBranding.logoHint')}
          </p>
        </div>

        {update.isError && (
          <p className="text-sm text-destructive">
            {t('orgBranding.saveError', { message: update.error.message })}
          </p>
        )}
        {saved && (
          <p className="text-sm text-green-500">{t('orgBranding.settingsSaved')}</p>
        )}

        <div className="flex justify-end">
          <Button
            onClick={handleSave}
            disabled={update.isPending || !isValidUrl}
          >
            {update.isPending ? t('orgBranding.saving') : t('orgBranding.save')}
          </Button>
        </div>
      </Card>

      {/* Trust Center card */}
      <Card className="p-6 space-y-4">
        <div className="flex items-center gap-2">
          <Shield className="w-5 h-5 text-secondary" />
          <h2 className="text-base font-semibold">{t('orgBranding.trustCenterTitle')}</h2>
        </div>

        <p className="text-sm text-muted-foreground">
          {t('orgBranding.trustCenterDescription')}
        </p>

        {/* Enable toggle */}
        <div className="flex items-center gap-3">
          <input
            id="tc-enabled"
            type="checkbox"
            checked={tcEnabled}
            onChange={(e) => { setTcEnabled(e.target.checked); setTcSaved(false) }}
            className="h-4 w-4 rounded border-gray-300 text-indigo-600 focus:ring-indigo-500 cursor-pointer"
          />
          <Label htmlFor="tc-enabled" className="cursor-pointer">
            {t('orgBranding.enableTrustCenter')}
          </Label>
        </div>

        {/* Description */}
        <div className="space-y-2">
          <Label htmlFor="tc-description">{t('orgBranding.descriptionLabel')}</Label>
          <textarea
            id="tc-description"
            maxLength={300}
            rows={3}
            placeholder="Kurze Beschreibung deines Sicherheitsprogramms für Kunden und Partner..."
            value={tcDescription}
            onChange={(e) => { setTcDescription(e.target.value); setTcSaved(false) }}
            className="flex w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50 resize-none"
          />
          <p className="text-xs text-muted-foreground text-right">{tcDescription.length}/300</p>
        </div>

        {/* Contact */}
        <div className="space-y-2">
          <Label htmlFor="tc-contact">{t('orgBranding.contactLabel')}</Label>
          <Input
            id="tc-contact"
            type="email"
            placeholder="security@example.com"
            value={tcContact}
            onChange={(e) => { setTcContact(e.target.value); setTcSaved(false) }}
          />
        </div>

        {/* Link preview when enabled */}
        {tcEnabled && trustCenterUrl && (
          <div className="flex items-center gap-2 rounded-lg bg-indigo-50 border border-indigo-200 px-3 py-2">
            <ExternalLink className="w-4 h-4 text-indigo-500 shrink-0" />
            <span className="text-sm text-indigo-700">
              {t('orgBranding.trustCenterLink')}{' '}
              <a
                href={trustCenterUrl}
                target="_blank"
                rel="noreferrer"
                className="font-mono underline hover:text-indigo-900"
              >
                {trustCenterUrl}
              </a>
            </span>
          </div>
        )}

        {updateTrustCenter.isError && (
          <p className="text-sm text-destructive">
            {t('orgBranding.saveError', { message: updateTrustCenter.error.message })}
          </p>
        )}
        {tcSaved && (
          <p className="text-sm text-green-500">{t('orgBranding.trustCenterSaved')}</p>
        )}

        <div className="flex justify-end">
          <Button
            onClick={handleTrustCenterSave}
            disabled={updateTrustCenter.isPending}
          >
            {updateTrustCenter.isPending ? t('orgBranding.saving') : t('orgBranding.save')}
          </Button>
        </div>
      </Card>

      {/* Info card */}
      <Card className="p-6 space-y-3">
        <div className="flex items-center gap-2">
          <Info className="w-5 h-5 text-secondary" />
          <h2 className="text-base font-semibold">{t('orgBranding.reportsTitle')}</h2>
        </div>
        <p className="text-sm text-muted-foreground leading-relaxed">
          {t('orgBranding.reportsHint')}
        </p>
        <ul className="text-sm text-muted-foreground space-y-1 list-disc list-inside">
          <li>Audit-Ready Compliance Reports (Vakt Comply)</li>
          <li>Vulnerability-Berichte (Vakt Scan)</li>
          <li>Datenschutz-Dokumente (Vakt Privacy)</li>
          <li>Awareness-Kampagnen-Berichte (Vakt Aware)</li>
        </ul>
      </Card>
    </div>
  )
}
