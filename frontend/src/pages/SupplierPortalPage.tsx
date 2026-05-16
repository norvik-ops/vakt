import { useState, useCallback, useRef, useEffect } from 'react'
import { useParams, useSearchParams } from 'react-router-dom'
import { useQuery, useMutation } from '@tanstack/react-query'

// ---------------------------------------------------------------------------
// i18n — simple JS object, no external library
// ---------------------------------------------------------------------------

export const PORTAL_TEXT = {
  de: {
    title: 'Sicherheitsfragebogen',
    loading: 'Fragebogen wird geladen…',
    errorExpired: 'Dieser Link ist abgelaufen oder der Fragebogen wurde bereits eingereicht.',
    errorGeneral: 'Der Fragebogen konnte nicht geladen werden. Bitte versuchen Sie es später erneut.',
    questionOf: 'Frage',
    of: 'von',
    next: 'Weiter',
    back: 'Zurück',
    save: 'Zwischenspeichern',
    saving: 'Wird gespeichert…',
    saved: 'Gespeichert',
    submit: 'Einreichen',
    submitConfirmTitle: 'Fragebogen einreichen?',
    submitConfirmBody: 'Nach dem Einreichen können keine Änderungen mehr vorgenommen werden.',
    submitConfirmYes: 'Ja, einreichen',
    submitConfirmNo: 'Abbrechen',
    successTitle: 'Fragebogen eingereicht',
    successBody: 'Vielen Dank! Ihr Fragebogen wurde erfolgreich eingereicht.',
    required: 'Pflichtfeld',
    yesLabel: 'Ja',
    noLabel: 'Nein',
    uploadLabel: 'Datei hochladen',
    uploadHint: 'PDF, PNG, JPEG oder XLSX (max. 20 MB)',
    uploadSuccess: 'Datei hochgeladen',
    fileSelected: 'Ausgewählt',
  },
  en: {
    title: 'Security Questionnaire',
    loading: 'Loading questionnaire…',
    errorExpired: 'This link has expired or the questionnaire has already been submitted.',
    errorGeneral: 'The questionnaire could not be loaded. Please try again later.',
    questionOf: 'Question',
    of: 'of',
    next: 'Next',
    back: 'Back',
    save: 'Save draft',
    saving: 'Saving…',
    saved: 'Saved',
    submit: 'Submit',
    submitConfirmTitle: 'Submit questionnaire?',
    submitConfirmBody: 'After submitting, no changes can be made.',
    submitConfirmYes: 'Yes, submit',
    submitConfirmNo: 'Cancel',
    successTitle: 'Questionnaire submitted',
    successBody: 'Thank you! Your questionnaire has been submitted successfully.',
    required: 'Required',
    yesLabel: 'Yes',
    noLabel: 'No',
    uploadLabel: 'Upload file',
    uploadHint: 'PDF, PNG, JPEG or XLSX (max. 20 MB)',
    uploadSuccess: 'File uploaded',
    fileSelected: 'Selected',
  },
} as const

export type PortalLang = 'de' | 'en'
type PortalTextKey = keyof (typeof PORTAL_TEXT)['de']

export function getPortalText(lang: PortalLang, key: string): string {
  const dict = PORTAL_TEXT[lang] ?? PORTAL_TEXT.de
  return (dict as Record<string, string>)[key] ?? key
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface Question {
  id: string
  question_text: string
  question_type: 'yes_no' | 'multiple_choice' | 'free_text' | 'file_upload'
  options?: string[]
  required: boolean
  order_idx: number
}

interface Questionnaire {
  id: string
  name: string
  description?: string
  questions: Question[]
}

interface AssessmentResponse {
  id: string
  status: string
  expires_at: string
  questionnaire: Questionnaire
}

interface AnswerInput {
  question_id: string
  answer_text?: string
  answer_bool?: boolean
  answer_options?: string[]
  file_url?: string
}

// ---------------------------------------------------------------------------
// API helpers
// ---------------------------------------------------------------------------

async function fetchAssessment(token: string): Promise<AssessmentResponse> {
  const res = await fetch(`/supplier/${token}`, {
    headers: { Accept: 'application/json' },
  })
  if (res.status === 410) {
    throw new Error('EXPIRED_OR_SUBMITTED')
  }
  if (!res.ok) {
    throw new Error('FETCH_FAILED')
  }
  return res.json() as Promise<AssessmentResponse>
}

async function saveAnswers(token: string, answers: AnswerInput[]): Promise<void> {
  const res = await fetch(`/supplier/${token}/save`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ answers }),
  })
  if (res.status === 410) throw new Error('EXPIRED_OR_SUBMITTED')
  if (!res.ok) throw new Error('SAVE_FAILED')
}

async function submitAnswers(token: string, answers: AnswerInput[]): Promise<void> {
  const res = await fetch(`/supplier/${token}/submit`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ answers }),
  })
  if (res.status === 410) throw new Error('EXPIRED_OR_SUBMITTED')
  if (!res.ok) throw new Error('SUBMIT_FAILED')
}

async function uploadFile(token: string, file: File): Promise<string> {
  const form = new FormData()
  form.append('file', file)
  const res = await fetch(`/supplier/${token}/upload`, {
    method: 'POST',
    body: form,
  })
  if (res.status === 410) throw new Error('EXPIRED_OR_SUBMITTED')
  if (!res.ok) throw new Error('UPLOAD_FAILED')
  const data = (await res.json()) as { file_url: string }
  return data.file_url
}

// ---------------------------------------------------------------------------
// SupplierPortalPage
// ---------------------------------------------------------------------------

export default function SupplierPortalPage() {
  const { token } = useParams<{ token: string }>()
  const [searchParams] = useSearchParams()
  const rawLang = searchParams.get('lang') ?? 'de'
  const lang: PortalLang = rawLang === 'en' ? 'en' : 'de'
  const t = useCallback((key: PortalTextKey) => getPortalText(lang, key), [lang])

  const [currentStep, setCurrentStep] = useState(0)
  const [answers, setAnswers] = useState<Record<string, AnswerInput>>({})
  const [saveStatus, setSaveStatus] = useState<'idle' | 'saving' | 'saved'>('idle')
  const [showConfirm, setShowConfirm] = useState(false)
  const [submitted, setSubmitted] = useState(false)
  const [expiredError, setExpiredError] = useState(false)
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const saveStatusTimerRef = useRef<ReturnType<typeof setTimeout>>()

  const { data, isLoading, isError, error } = useQuery({
    queryKey: ['supplier-portal', token],
    queryFn: () => fetchAssessment(token!),
    enabled: !!token,
    retry: false,
  })

  const isExpiredError =
    expiredError ||
    (isError && error instanceof Error && error.message === 'EXPIRED_OR_SUBMITTED')

  useEffect(() => () => { clearTimeout(saveStatusTimerRef.current); if (debounceRef.current) clearTimeout(debounceRef.current) }, [])

  const saveMutation = useMutation({
    mutationFn: (ans: AnswerInput[]) => saveAnswers(token!, ans),
    onMutate: () => setSaveStatus('saving'),
    onSuccess: () => {
      setSaveStatus('saved')
      saveStatusTimerRef.current = setTimeout(() => setSaveStatus('idle'), 2000)
    },
    onError: (err: Error) => {
      if (err.message === 'EXPIRED_OR_SUBMITTED') setExpiredError(true)
      setSaveStatus('idle')
    },
  })

  const submitMutation = useMutation({
    mutationFn: (ans: AnswerInput[]) => submitAnswers(token!, ans),
    onSuccess: () => setSubmitted(true),
    onError: (err: Error) => {
      if (err.message === 'EXPIRED_OR_SUBMITTED') setExpiredError(true)
    },
  })

  const uploadMutation = useMutation({
    mutationFn: ({ qid, file }: { qid: string; file: File }) =>
      uploadFile(token!, file).then((url) => ({ qid, url })),
    onSuccess: ({ qid, url }: { qid: string; url: string }) => {
      setAnswers((prev) => ({
        ...prev,
        [qid]: { ...(prev[qid] ?? { question_id: qid }), file_url: url },
      }))
    },
    onError: (err: Error) => {
      if (err.message === 'EXPIRED_OR_SUBMITTED') setExpiredError(true)
    },
  })

  // Debounced auto-save (2 s).
  const triggerAutoSave = useCallback(
    (current: Record<string, AnswerInput>) => {
      if (debounceRef.current) clearTimeout(debounceRef.current)
      debounceRef.current = setTimeout(() => {
        saveMutation.mutate(Object.values(current))
      }, 2000)
    },
    [saveMutation],
  )

  function updateAnswer(qid: string, patch: Partial<AnswerInput>) {
    setAnswers((prev) => {
      const next = { ...prev, [qid]: { ...(prev[qid] ?? { question_id: qid }), ...patch } }
      triggerAutoSave(next)
      return next
    })
  }

  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50">
        <p className="text-gray-500">{t('loading')}</p>
      </div>
    )
  }

  if (isExpiredError) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50 p-4">
        <div className="max-w-md w-full bg-white rounded-xl shadow p-8 text-center">
          <h1 className="text-xl font-semibold text-red-600 mb-3">{t('title')}</h1>
          <p className="text-gray-600">{t('errorExpired')}</p>
        </div>
      </div>
    )
  }

  if (isError || !data) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50 p-4">
        <div className="max-w-md w-full bg-white rounded-xl shadow p-8 text-center">
          <h1 className="text-xl font-semibold text-red-600 mb-3">{t('title')}</h1>
          <p className="text-gray-600">{t('errorGeneral')}</p>
        </div>
      </div>
    )
  }

  if (submitted) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50 p-4">
        <div className="max-w-md w-full bg-white rounded-xl shadow p-8 text-center">
          <h1 className="text-2xl font-bold text-green-600 mb-3">{t('successTitle')}</h1>
          <p className="text-gray-600">{t('successBody')}</p>
        </div>
      </div>
    )
  }

  const questions = (data.questionnaire?.questions ?? []).sort(
    (a: Question, b: Question) => a.order_idx - b.order_idx,
  )
  const total = questions.length
  const current = questions[currentStep]

  if (!current) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50 p-4">
        <p className="text-gray-500">{t('loading')}</p>
      </div>
    )
  }

  const ans = answers[current.id] ?? { question_id: current.id }

  function handleManualSave() {
    if (debounceRef.current) clearTimeout(debounceRef.current)
    saveMutation.mutate(Object.values(answers))
  }

  function handleSubmit() {
    submitMutation.mutate(Object.values(answers))
    setShowConfirm(false)
  }

  return (
    <div className="min-h-screen bg-gray-50 flex flex-col">
      {/* Header */}
      <header className="bg-white border-b px-6 py-4 flex items-center justify-between shadow-sm">
        <h1 className="text-lg font-semibold text-gray-800">{t('title')}</h1>
        <span className="text-sm text-gray-500">
          {data.questionnaire?.name}
        </span>
      </header>

      {/* Progress bar */}
      <div className="w-full bg-gray-200 h-1">
        <div
          className="bg-blue-600 h-1 transition-all"
          style={{ width: total > 0 ? `${((currentStep + 1) / total) * 100}%` : '0%' }}
        />
      </div>

      {/* Step indicator */}
      <div className="text-center text-sm text-gray-500 mt-4">
        {t('questionOf')} {currentStep + 1} {t('of')} {total}
      </div>

      {/* Question card */}
      <main className="flex-1 flex items-start justify-center p-4 sm:p-8">
        <div className="w-full max-w-xl bg-white rounded-xl shadow p-6 space-y-4">
          <div className="flex items-start gap-2">
            <p className="text-gray-800 font-medium flex-1">{current.question_text}</p>
            {current.required && (
              <span className="text-xs text-red-500 mt-0.5 shrink-0">*{t('required')}</span>
            )}
          </div>

          {/* yes_no */}
          {current.question_type === 'yes_no' && (
            <div className="flex gap-3">
              {[true, false].map((val) => (
                <label key={String(val)} className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="radio"
                    name={`yesno-${current.id}`}
                    checked={ans.answer_bool === val}
                    onChange={() => updateAnswer(current.id, { question_id: current.id, answer_bool: val })}
                    className="accent-blue-600"
                  />
                  <span className="text-sm">{val ? t('yesLabel') : t('noLabel')}</span>
                </label>
              ))}
            </div>
          )}

          {/* multiple_choice */}
          {current.question_type === 'multiple_choice' && (
            <div className="space-y-2">
              {(current.options ?? []).map((opt) => {
                const selected = ans.answer_options?.includes(opt) ?? false
                return (
                  <label key={opt} className="flex items-center gap-2 cursor-pointer">
                    <input
                      type="checkbox"
                      checked={selected}
                      onChange={() => {
                        const prev = ans.answer_options ?? []
                        const next = selected ? prev.filter((o) => o !== opt) : [...prev, opt]
                        updateAnswer(current.id, { question_id: current.id, answer_options: next })
                      }}
                      className="accent-blue-600"
                    />
                    <span className="text-sm">{opt}</span>
                  </label>
                )
              })}
            </div>
          )}

          {/* free_text */}
          {current.question_type === 'free_text' && (
            <textarea
              className="w-full border border-gray-300 rounded-lg p-3 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 resize-none"
              rows={5}
              value={ans.answer_text ?? ''}
              onChange={(e) => updateAnswer(current.id, { question_id: current.id, answer_text: e.target.value })}
            />
          )}

          {/* file_upload */}
          {current.question_type === 'file_upload' && (
            <div className="space-y-2">
              <p className="text-xs text-gray-500">{t('uploadHint')}</p>
              <input
                type="file"
                accept=".pdf,.png,.jpg,.jpeg,.xlsx"
                className="block text-sm"
                onChange={(e) => {
                  const file = e.target.files?.[0]
                  if (file) {
                    uploadMutation.mutate({ qid: current.id, file })
                  }
                }}
              />
              {ans.file_url && (
                <p className="text-xs text-green-600">
                  {t('uploadSuccess')}: {ans.file_url.split('/').pop()}
                </p>
              )}
            </div>
          )}
        </div>
      </main>

      {/* Navigation footer */}
      <footer className="bg-white border-t px-6 py-4 flex items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <button
            disabled={currentStep === 0}
            onClick={() => setCurrentStep((s) => s - 1)}
            className="px-4 py-2 text-sm rounded-lg border border-gray-300 disabled:opacity-40 hover:bg-gray-50"
          >
            {t('back')}
          </button>
          {currentStep < total - 1 && (
            <button
              onClick={() => setCurrentStep((s) => s + 1)}
              className="px-4 py-2 text-sm rounded-lg bg-blue-600 text-white hover:bg-blue-700"
            >
              {t('next')}
            </button>
          )}
        </div>

        <div className="flex items-center gap-2">
          {/* Save status indicator */}
          {saveStatus !== 'idle' && (
            <span className="text-xs text-gray-500">
              {saveStatus === 'saving' ? t('saving') : t('saved')}
            </span>
          )}

          {/* Manual save */}
          <button
            onClick={handleManualSave}
            disabled={saveMutation.isPending}
            className="px-4 py-2 text-sm rounded-lg border border-gray-300 hover:bg-gray-50 disabled:opacity-40"
          >
            {t('save')}
          </button>

          {/* Submit — only on last step */}
          {currentStep === total - 1 && (
            <button
              onClick={() => setShowConfirm(true)}
              disabled={submitMutation.isPending}
              className="px-4 py-2 text-sm rounded-lg bg-green-600 text-white hover:bg-green-700 disabled:opacity-40"
            >
              {t('submit')}
            </button>
          )}
        </div>
      </footer>

      {/* Confirm dialog */}
      {showConfirm && (
        <div className="fixed inset-0 bg-black/40 flex items-center justify-center z-50 p-4">
          <div className="bg-white rounded-xl shadow-lg p-6 max-w-sm w-full space-y-4">
            <h2 className="text-lg font-semibold">{t('submitConfirmTitle')}</h2>
            <p className="text-sm text-gray-600">{t('submitConfirmBody')}</p>
            <div className="flex gap-3 justify-end">
              <button
                onClick={() => setShowConfirm(false)}
                className="px-4 py-2 text-sm rounded-lg border border-gray-300 hover:bg-gray-50"
              >
                {t('submitConfirmNo')}
              </button>
              <button
                onClick={handleSubmit}
                disabled={submitMutation.isPending}
                className="px-4 py-2 text-sm rounded-lg bg-green-600 text-white hover:bg-green-700 disabled:opacity-40"
              >
                {t('submitConfirmYes')}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
