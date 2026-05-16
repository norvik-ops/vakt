import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { useQuery, useMutation } from '@tanstack/react-query'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface DSRPortalInfo {
  org_name: string
  slug: string
  intro?: string
  enabled: boolean
}

type DSRType = 'access' | 'deletion' | 'correction' | 'objection'

interface PortalDSRInput {
  type: DSRType
  first_name: string
  last_name: string
  email: string
  description: string
  locale: string
}

// ---------------------------------------------------------------------------
// API helpers
// ---------------------------------------------------------------------------

async function fetchPortalInfo(slug: string): Promise<DSRPortalInfo> {
  const res = await fetch(`/api/v1/secprivacy/dsr-portal/${slug}/info`, {
    headers: { Accept: 'application/json' },
  })
  if (!res.ok) throw new Error('PORTAL_NOT_FOUND')
  return res.json() as Promise<DSRPortalInfo>
}

async function submitDSR(slug: string, input: PortalDSRInput): Promise<{ token: string }> {
  const res = await fetch(`/api/v1/secprivacy/dsr-portal/${slug}/submit`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  })
  if (!res.ok) {
    const err = (await res.json().catch(() => ({}))) as { error?: string }
    throw new Error(err.error ?? 'SUBMIT_FAILED')
  }
  return res.json() as Promise<{ token: string }>
}

// ---------------------------------------------------------------------------
// DSR type options
// ---------------------------------------------------------------------------

const DSR_TYPES: { value: DSRType; label: string; description: string; icon: string }[] = [
  {
    value: 'access',
    label: 'Auskunft (Art. 15)',
    description: 'Ich möchte wissen, welche Daten über mich gespeichert sind.',
    icon: '🔍',
  },
  {
    value: 'deletion',
    label: 'Löschung (Art. 17)',
    description: 'Ich möchte, dass meine Daten gelöscht werden.',
    icon: '🗑️',
  },
  {
    value: 'correction',
    label: 'Berichtigung (Art. 16)',
    description: 'Ich möchte fehlerhafte Daten korrigieren lassen.',
    icon: '✏️',
  },
  {
    value: 'objection',
    label: 'Widerspruch (Art. 21)',
    description: 'Ich widerspreche der Verarbeitung meiner Daten.',
    icon: '🚫',
  },
]

// ---------------------------------------------------------------------------
// DSRPortalPage
// ---------------------------------------------------------------------------

export default function DSRPortalPage() {
  const { slug } = useParams<{ slug: string }>()

  const [step, setStep] = useState<1 | 2 | 3>(1)
  const [selectedType, setSelectedType] = useState<DSRType | null>(null)
  const [firstName, setFirstName] = useState('')
  const [lastName, setLastName] = useState('')
  const [email, setEmail] = useState('')
  const [description, setDescription] = useState('')
  const [statusToken, setStatusToken] = useState<string | null>(null)
  const [submitError, setSubmitError] = useState<string | null>(null)

  const { data: portalInfo, isLoading, isError } = useQuery({
    queryKey: ['dsr-portal-info', slug],
    queryFn: () => fetchPortalInfo(slug!),
    enabled: !!slug,
    retry: false,
  })

  const submitMutation = useMutation({
    mutationFn: (input: PortalDSRInput) => submitDSR(slug!, input),
    onSuccess: (data) => {
      setStatusToken(data.token)
      setStep(3)
    },
    onError: (err: Error) => {
      setSubmitError(err.message)
    },
  })

  function handleSubmit() {
    if (!selectedType) return
    setSubmitError(null)
    submitMutation.mutate({
      type: selectedType,
      first_name: firstName,
      last_name: lastName,
      email,
      description,
      locale: 'de',
    })
  }

  // Loading state
  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50">
        <p className="text-gray-500">Portal wird geladen…</p>
      </div>
    )
  }

  // Error or portal not found/disabled
  if (isError || !portalInfo || !portalInfo.enabled) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50 p-4">
        <div className="max-w-md w-full bg-white rounded-xl shadow p-8 text-center">
          <div className="text-4xl mb-4">⚠️</div>
          <h1 className="text-xl font-semibold text-gray-800 mb-3">
            Portal nicht verfügbar
          </h1>
          <p className="text-gray-600">
            Dieses Datenschutz-Portal ist nicht verfügbar oder wurde deaktiviert.
            Bitte wenden Sie sich direkt an die verantwortliche Stelle.
          </p>
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-gray-50 flex flex-col">
      {/* Header */}
      <header className="bg-white border-b px-6 py-4 shadow-sm">
        <div className="max-w-2xl mx-auto">
          <h1 className="text-lg font-semibold text-gray-800">
            Datenschutz-Anfrage — {portalInfo.org_name}
          </h1>
          <p className="text-sm text-gray-500 mt-0.5">
            Betroffenenanfrage nach Art. 15–21 DSGVO
          </p>
        </div>
      </header>

      {/* Progress indicator */}
      <div className="bg-white border-b">
        <div className="max-w-2xl mx-auto px-6 py-3 flex gap-6">
          {[
            { n: 1, label: 'Anfragetyp' },
            { n: 2, label: 'Ihre Daten' },
            { n: 3, label: 'Bestätigung' },
          ].map(({ n, label }) => (
            <div key={n} className="flex items-center gap-2">
              <div
                className={`w-6 h-6 rounded-full flex items-center justify-center text-xs font-bold ${
                  step >= n
                    ? 'bg-blue-600 text-white'
                    : 'bg-gray-200 text-gray-500'
                }`}
              >
                {n}
              </div>
              <span
                className={`text-sm ${step >= n ? 'text-gray-800 font-medium' : 'text-gray-400'}`}
              >
                {label}
              </span>
            </div>
          ))}
        </div>
      </div>

      {/* Main content */}
      <main className="flex-1 flex items-start justify-center p-4 sm:p-8">
        <div className="w-full max-w-2xl">

          {/* Step 1 — Choose request type */}
          {step === 1 && (
            <div className="bg-white rounded-xl shadow p-6 space-y-4">
              <h2 className="text-lg font-semibold text-gray-800">
                Welche Art von Anfrage möchten Sie stellen?
              </h2>

              {portalInfo.intro && (
                <p className="text-sm text-gray-600 bg-blue-50 rounded-lg p-3">
                  {portalInfo.intro}
                </p>
              )}

              <div className="grid gap-3 sm:grid-cols-2">
                {DSR_TYPES.map((t) => (
                  <button
                    key={t.value}
                    onClick={() => setSelectedType(t.value)}
                    className={`text-left p-4 rounded-lg border-2 transition-colors ${
                      selectedType === t.value
                        ? 'border-blue-600 bg-blue-50'
                        : 'border-gray-200 hover:border-gray-300 hover:bg-gray-50'
                    }`}
                  >
                    <div className="text-2xl mb-2">{t.icon}</div>
                    <div className="font-medium text-gray-800 text-sm">{t.label}</div>
                    <div className="text-xs text-gray-500 mt-1">{t.description}</div>
                  </button>
                ))}
              </div>

              <div className="flex justify-end pt-2">
                <button
                  onClick={() => setStep(2)}
                  disabled={!selectedType}
                  className="px-6 py-2 bg-blue-600 text-white rounded-lg text-sm font-medium hover:bg-blue-700 disabled:opacity-40 disabled:cursor-not-allowed"
                >
                  Weiter
                </button>
              </div>
            </div>
          )}

          {/* Step 2 — Personal data */}
          {step === 2 && (
            <div className="bg-white rounded-xl shadow p-6 space-y-4">
              <h2 className="text-lg font-semibold text-gray-800">
                Ihre Kontaktdaten
              </h2>
              <p className="text-sm text-gray-500">
                Bitte geben Sie Ihre Daten an, damit wir Ihre Anfrage bearbeiten können.
                Wir benötigen diese Angaben zur Identitätsprüfung gemäß Art. 12 DSGVO.
              </p>

              <div className="grid gap-4 sm:grid-cols-2">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">
                    Vorname <span className="text-red-500">*</span>
                  </label>
                  <input
                    type="text"
                    value={firstName}
                    onChange={(e) => setFirstName(e.target.value)}
                    className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                    placeholder="Max"
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">
                    Nachname <span className="text-red-500">*</span>
                  </label>
                  <input
                    type="text"
                    value={lastName}
                    onChange={(e) => setLastName(e.target.value)}
                    className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                    placeholder="Mustermann"
                  />
                </div>
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  E-Mail-Adresse <span className="text-red-500">*</span>
                </label>
                <input
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                  placeholder="max.mustermann@beispiel.de"
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Beschreibung{' '}
                  <span className="text-gray-400 font-normal">(optional)</span>
                </label>
                <textarea
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  rows={4}
                  className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 resize-none"
                  placeholder="Bitte beschreiben Sie Ihre Anfrage genauer…"
                />
              </div>

              {submitError && (
                <p className="text-sm text-red-600 bg-red-50 rounded-lg p-3">
                  Fehler beim Einreichen der Anfrage: {submitError}
                </p>
              )}

              <div className="flex justify-between pt-2">
                <button
                  onClick={() => setStep(1)}
                  className="px-6 py-2 border border-gray-300 rounded-lg text-sm hover:bg-gray-50"
                >
                  Zurück
                </button>
                <button
                  onClick={handleSubmit}
                  disabled={
                    submitMutation.isPending ||
                    !firstName.trim() ||
                    !lastName.trim() ||
                    !email.trim()
                  }
                  className="px-6 py-2 bg-blue-600 text-white rounded-lg text-sm font-medium hover:bg-blue-700 disabled:opacity-40 disabled:cursor-not-allowed"
                >
                  {submitMutation.isPending ? 'Wird eingereicht…' : 'Anfrage einreichen'}
                </button>
              </div>
            </div>
          )}

          {/* Step 3 — Confirmation */}
          {step === 3 && statusToken && (
            <div className="bg-white rounded-xl shadow p-6 space-y-4 text-center">
              <div className="text-5xl mb-2">✅</div>
              <h2 className="text-xl font-semibold text-gray-800">
                Anfrage erfolgreich eingereicht
              </h2>
              <p className="text-gray-600 text-sm">
                Vielen Dank! Ihre Datenschutzanfrage wurde erfolgreich an{' '}
                <strong>{portalInfo.org_name}</strong> übermittelt. Sie erhalten
                innerhalb von 30 Tagen eine Antwort (Art. 12 Abs. 3 DSGVO).
              </p>

              <div className="bg-gray-50 rounded-lg p-4 text-left">
                <p className="text-xs text-gray-500 mb-1 font-medium">
                  Ihr Status-Token (bitte sichern):
                </p>
                <p className="font-mono text-sm text-gray-800 break-all">{statusToken}</p>
              </div>

              <p className="text-sm text-gray-500">
                Mit diesem Token können Sie den Bearbeitungsstatus Ihrer Anfrage jederzeit
                unter{' '}
                <a
                  href={`/dsr/status/${statusToken}`}
                  className="text-blue-600 underline hover:text-blue-700"
                >
                  /dsr/status/{statusToken.slice(0, 8)}…
                </a>{' '}
                einsehen.
              </p>
            </div>
          )}
        </div>
      </main>

      {/* Footer */}
      <footer className="py-4 text-center text-xs text-gray-400">
        Datenschutz-Self-Service · Powered by Vakt
      </footer>
    </div>
  )
}
