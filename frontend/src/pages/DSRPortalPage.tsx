import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { useQuery, useMutation } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

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
  const res = await fetch(`/api/v1/vaktprivacy/dsr-portal/${slug}/info`, {
    headers: { Accept: 'application/json' },
  })
  if (!res.ok) throw new Error('PORTAL_NOT_FOUND')
  return res.json() as Promise<DSRPortalInfo>
}

async function submitDSR(slug: string, input: PortalDSRInput): Promise<{ token: string }> {
  const res = await fetch(`/api/v1/vaktprivacy/dsr-portal/${slug}/submit`, {
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
// Hook for DSR type options
// ---------------------------------------------------------------------------

function useDsrTypes(): { value: DSRType; label: string; description: string; icon: string }[] {
  const { t } = useTranslation()
  return [
    {
      value: 'access',
      label: t('dsr.portal.typeAccess'),
      description: t('dsr.portal.typeAccessDesc'),
      icon: '🔍',
    },
    {
      value: 'deletion',
      label: t('dsr.portal.typeDeletion'),
      description: t('dsr.portal.typeDeletionDesc'),
      icon: '🗑️',
    },
    {
      value: 'correction',
      label: t('dsr.portal.typeCorrection'),
      description: t('dsr.portal.typeCorrectionDesc'),
      icon: '✏️',
    },
    {
      value: 'objection',
      label: t('dsr.portal.typeObjection'),
      description: t('dsr.portal.typeObjectionDesc'),
      icon: '🚫',
    },
  ]
}

// ---------------------------------------------------------------------------
// DSRPortalPage
// ---------------------------------------------------------------------------

export default function DSRPortalPage() {
  const { t } = useTranslation()
  const { slug } = useParams<{ slug: string }>()
  const dsrTypes = useDsrTypes()

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
        <p className="text-gray-500">{t('dsr.portal.loading')}</p>
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
            {t('dsr.portal.notAvailableTitle')}
          </h1>
          <p className="text-gray-600">
            {t('dsr.portal.notAvailableHint')}
          </p>
        </div>
      </div>
    )
  }

  const steps = [
    { n: 1, label: t('dsr.portal.step1') },
    { n: 2, label: t('dsr.portal.step2') },
    { n: 3, label: t('dsr.portal.step3') },
  ]

  return (
    <div className="min-h-screen bg-gray-50 flex flex-col">
      {/* Header */}
      <header className="bg-white border-b px-6 py-4 shadow-sm">
        <div className="max-w-2xl mx-auto">
          <h1 className="text-lg font-semibold text-gray-800">
            {t('dsr.portal.headerTitle', { orgName: portalInfo.org_name })}
          </h1>
          <p className="text-sm text-gray-500 mt-0.5">
            {t('dsr.portal.headerSubtitle')}
          </p>
        </div>
      </header>

      {/* Progress indicator */}
      <div className="bg-white border-b">
        <div className="max-w-2xl mx-auto px-6 py-3 flex gap-6">
          {steps.map(({ n, label }) => (
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
                className={`text-sm ${step >= n ? 'text-gray-800 font-medium' : 'text-gray-500'}`}
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
                {t('dsr.portal.typeQuestion')}
              </h2>

              {portalInfo.intro && (
                <p className="text-sm text-gray-600 bg-blue-50 rounded-lg p-3">
                  {portalInfo.intro}
                </p>
              )}

              <div className="grid gap-3 sm:grid-cols-2">
                {dsrTypes.map((dt) => (
                  <button
                    key={dt.value}
                    onClick={() => { setSelectedType(dt.value); }}
                    className={`text-left p-4 rounded-lg border-2 transition-colors ${
                      selectedType === dt.value
                        ? 'border-blue-600 bg-blue-50'
                        : 'border-gray-200 hover:border-gray-300 hover:bg-gray-50'
                    }`}
                  >
                    <div className="text-2xl mb-2">{dt.icon}</div>
                    <div className="font-medium text-gray-800 text-sm">{dt.label}</div>
                    <div className="text-xs text-gray-500 mt-1">{dt.description}</div>
                  </button>
                ))}
              </div>

              <div className="flex justify-end pt-2">
                <button
                  onClick={() => { setStep(2); }}
                  disabled={!selectedType}
                  className="px-6 py-2 bg-blue-600 text-white rounded-lg text-sm font-medium hover:bg-blue-700 disabled:opacity-40 disabled:cursor-not-allowed"
                >
                  {t('dsr.portal.next')}
                </button>
              </div>
            </div>
          )}

          {/* Step 2 — Personal data */}
          {step === 2 && (
            <div className="bg-white rounded-xl shadow p-6 space-y-4">
              <h2 className="text-lg font-semibold text-gray-800">
                {t('dsr.portal.contactTitle')}
              </h2>
              <p className="text-sm text-gray-500">
                {t('dsr.portal.contactHint')}
              </p>

              <div className="grid gap-4 sm:grid-cols-2">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">
                    {t('dsr.portal.firstName')} <span className="text-red-500">*</span>
                  </label>
                  <input
                    type="text"
                    value={firstName}
                    onChange={(e) => { setFirstName(e.target.value); }}
                    className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                    placeholder="Max"
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">
                    {t('dsr.portal.lastName')} <span className="text-red-500">*</span>
                  </label>
                  <input
                    type="text"
                    value={lastName}
                    onChange={(e) => { setLastName(e.target.value); }}
                    className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                    placeholder="Mustermann"
                  />
                </div>
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  {t('dsr.portal.emailLabel')} <span className="text-red-500">*</span>
                </label>
                <input
                  type="email"
                  value={email}
                  onChange={(e) => { setEmail(e.target.value); }}
                  className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                  placeholder="max.mustermann@beispiel.de"
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  {t('dsr.portal.descriptionLabel')}{' '}
                  <span className="text-gray-500 font-normal">{t('dsr.portal.descriptionOptional')}</span>
                </label>
                <textarea
                  value={description}
                  onChange={(e) => { setDescription(e.target.value); }}
                  rows={4}
                  className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 resize-none"
                  placeholder={t('dsr.portal.descriptionPlaceholder')}
                />
              </div>

              {submitError && (
                <p className="text-sm text-red-600 bg-red-50 rounded-lg p-3">
                  {t('dsr.portal.submitError', { message: submitError })}
                </p>
              )}

              <div className="flex justify-between pt-2">
                <button
                  onClick={() => { setStep(1); }}
                  className="px-6 py-2 border border-gray-300 rounded-lg text-sm hover:bg-gray-50"
                >
                  {t('dsr.portal.back')}
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
                  {submitMutation.isPending ? t('dsr.portal.submitting') : t('dsr.portal.submit')}
                </button>
              </div>
            </div>
          )}

          {/* Step 3 — Confirmation */}
          {step === 3 && statusToken && (
            <div className="bg-white rounded-xl shadow p-6 space-y-4 text-center">
              <div className="text-5xl mb-2">✅</div>
              <h2 className="text-xl font-semibold text-gray-800">
                {t('dsr.portal.successTitle')}
              </h2>
              <p className="text-gray-600 text-sm">
                {t('dsr.portal.successHint', { orgName: portalInfo.org_name })}
              </p>

              <div className="bg-gray-50 rounded-lg p-4 text-left">
                <p className="text-xs text-gray-500 mb-1 font-medium">
                  {t('dsr.portal.tokenLabel')}
                </p>
                <p className="font-mono text-sm text-gray-800 break-all">{statusToken}</p>
              </div>

              <p className="text-sm text-gray-500">
                {t('dsr.portal.tokenHint')}{' '}
                <a
                  href={`/dsr/status/${statusToken}`}
                  className="text-blue-600 underline hover:text-blue-700"
                >
                  /dsr/status/{statusToken.slice(0, 8)}…
                </a>
              </p>
            </div>
          )}
        </div>
      </main>

      {/* Footer */}
      <footer className="py-4 text-center text-xs text-gray-500">
        {t('dsr.portal.footer')}
      </footer>
    </div>
  )
}
