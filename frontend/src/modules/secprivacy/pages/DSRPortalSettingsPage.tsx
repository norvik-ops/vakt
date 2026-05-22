import { useState, useEffect, useRef } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Spinner } from '../../../components/Spinner'
import { apiFetch } from '../../../api/client'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface DSRPortalSettings {
  enabled: boolean
  slug: string
  dpo_email: string
  intro: string
}

// ---------------------------------------------------------------------------
// API helpers
// ---------------------------------------------------------------------------

async function getSettings(): Promise<DSRPortalSettings> {
  return apiFetch<DSRPortalSettings>('/secprivacy/dsr-portal-settings')
}

async function updateSettings(input: DSRPortalSettings): Promise<void> {
  await apiFetch('/secprivacy/dsr-portal-settings', {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  })
}

// ---------------------------------------------------------------------------
// DSRPortalSettingsPage
// ---------------------------------------------------------------------------

export default function DSRPortalSettingsPage() {
  const queryClient = useQueryClient()

  const { data, isLoading, isError } = useQuery({
    queryKey: ['dsr-portal-settings'],
    queryFn: getSettings,
  })

  const [enabled, setEnabled] = useState(false)
  const [slug, setSlug] = useState('')
  const [dpoEmail, setDpoEmail] = useState('')
  const [intro, setIntro] = useState('')
  const [saved, setSaved] = useState(false)
  const savedTimerRef = useRef<ReturnType<typeof setTimeout>>()

  useEffect(() => () => clearTimeout(savedTimerRef.current), [])

  useEffect(() => {
    if (data) {
      setEnabled(data.enabled)
      setSlug(data.slug ?? '')
      setDpoEmail(data.dpo_email ?? '')
      setIntro(data.intro ?? '')
    }
  }, [data])

  const mutation = useMutation({
    mutationFn: () =>
      updateSettings({ enabled, slug, dpo_email: dpoEmail, intro }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['dsr-portal-settings'] })
      setSaved(true)
      savedTimerRef.current = setTimeout(() => setSaved(false), 3000)
    },
  })

  const portalUrl = slug
    ? `${window.location.origin}/dsr/${slug}`
    : null

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
        <p className="text-red-600 text-sm">
          Portal-Einstellungen konnten nicht geladen werden.
        </p>
      </div>
    )
  }

  return (
    <div className="max-w-2xl mx-auto p-6 space-y-6">
      <div>
        <h1 className="text-xl font-semibold text-gray-900">
          DSR Self-Service Portal
        </h1>
        <p className="text-sm text-gray-500 mt-1">
          Ermöglichen Sie Betroffenen, Datenschutzanfragen (Art. 15–21 DSGVO) direkt
          über einen öffentlichen Link einzureichen — ohne Login.
        </p>
      </div>

      {/* Enable toggle */}
      <div className="bg-white rounded-xl shadow p-5 space-y-4">
        <h2 className="text-base font-medium text-gray-800">Portal-Status</h2>

        <label className="flex items-center justify-between cursor-pointer">
          <div>
            <span className="text-sm font-medium text-gray-700">Portal aktivieren</span>
            <p className="text-xs text-gray-500 mt-0.5">
              Wenn aktiviert, können Betroffene über den öffentlichen Link Anfragen stellen.
            </p>
          </div>
          <button
            type="button"
            role="switch"
            aria-checked={enabled}
            onClick={() => setEnabled((v) => !v)}
            className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
              enabled ? 'bg-blue-600' : 'bg-gray-200'
            }`}
          >
            <span
              className={`inline-block h-4 w-4 transform rounded-full bg-white shadow transition-transform ${
                enabled ? 'translate-x-6' : 'translate-x-1'
              }`}
            />
          </button>
        </label>
      </div>

      {/* Portal configuration */}
      <div className="bg-white rounded-xl shadow p-5 space-y-4">
        <h2 className="text-base font-medium text-gray-800">Portal-Konfiguration</h2>

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            Portal-Slug
          </label>
          <div className="flex items-center gap-2">
            <span className="text-sm text-gray-400 shrink-0">/dsr/</span>
            <input
              type="text"
              value={slug}
              onChange={(e) => setSlug(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, ''))}
              className="flex-1 border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              placeholder="mein-unternehmen"
            />
          </div>
          <p className="text-xs text-gray-400 mt-1">
            Nur Kleinbuchstaben, Ziffern und Bindestriche. Muss eindeutig sein.
          </p>
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            DSB / DPO E-Mail
          </label>
          <input
            type="email"
            value={dpoEmail}
            onChange={(e) => setDpoEmail(e.target.value)}
            className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
            placeholder="datenschutz@beispiel.de"
          />
          <p className="text-xs text-gray-400 mt-1">
            Diese Adresse wird für interne Benachrichtigungen bei neuen DSR-Einreichungen genutzt.
          </p>
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            Einleitungstext
          </label>
          <textarea
            value={intro}
            onChange={(e) => setIntro(e.target.value)}
            rows={4}
            className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 resize-none"
            placeholder="Willkommen auf unserem Datenschutz-Portal. Hier können Sie Ihre Rechte nach DSGVO wahrnehmen…"
          />
          <p className="text-xs text-gray-400 mt-1">
            Wird oben auf dem öffentlichen Portal angezeigt.
          </p>
        </div>
      </div>

      {/* Portal URL preview */}
      {portalUrl && (
        <div className="bg-blue-50 rounded-xl p-4 space-y-2">
          <h3 className="text-sm font-medium text-blue-800">Öffentliche Portal-URL</h3>
          <div className="flex items-center gap-2">
            <code className="flex-1 text-xs text-blue-900 bg-blue-100 rounded px-2 py-1 break-all">
              {portalUrl}
            </code>
            <button
              onClick={() => void navigator.clipboard.writeText(portalUrl)}
              className="shrink-0 text-xs text-blue-700 hover:text-blue-900 px-2 py-1 border border-blue-300 rounded"
            >
              Kopieren
            </button>
          </div>
          {!enabled && (
            <p className="text-xs text-amber-700 bg-amber-50 rounded px-2 py-1">
              Das Portal ist derzeit deaktiviert. Aktivieren Sie es oben, damit der Link öffentlich zugänglich ist.
            </p>
          )}
        </div>
      )}

      {/* Save button */}
      <div className="flex items-center justify-end gap-3">
        {saved && (
          <span className="text-sm text-green-600 font-medium">Einstellungen gespeichert</span>
        )}
        <button
          onClick={() => mutation.mutate()}
          disabled={mutation.isPending}
          className="px-5 py-2 bg-blue-600 text-white rounded-lg text-sm font-medium hover:bg-blue-700 disabled:opacity-40"
        >
          {mutation.isPending ? 'Wird gespeichert…' : 'Einstellungen speichern'}
        </button>
      </div>
    </div>
  )
}
