import { Link } from 'react-router-dom'
import { CheckCircle2, Circle, X, ArrowRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../../api/client'

interface OnboardingStep {
  key: string
  done: boolean
  path: string
}

interface OnboardingProgress {
  steps: OnboardingStep[]
  completed_count: number
  total: number
  percent_done: number
  dismissed: boolean
  all_complete: boolean
}

// Stable order of the 7 "first 30 days" steps (matches the backend).
const STEP_ORDER = ['scope', 'assets', 'protection_need', 'framework', 'risks', 'evidence', 'policy']

function useOnboardingProgress() {
  return useQuery<OnboardingProgress>({
    queryKey: ['onboarding', 'progress'],
    queryFn: () => apiFetch<OnboardingProgress>('/onboarding/progress'),
    staleTime: 30_000,
    retry: false,
  })
}

function useDismissOnboarding() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => apiFetch('/onboarding/dismiss', { method: 'POST' }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['onboarding', 'progress'] })
    },
  })
}

// OnboardingChecklist is the guided "first 30 days" ISB path (S89-5). Each step
// links to the real feature and shows a live "done" status derived from org
// data. Community feature — never Pro-gated. Hidden once dismissed.
export function OnboardingChecklist() {
  const { t } = useTranslation()
  const { data, isLoading, isError } = useOnboardingProgress()
  const dismiss = useDismissOnboarding()

  if (isLoading || isError || !data || data.dismissed) return null

  // Order steps stably; fall back to backend order for unknown keys.
  const steps = [...data.steps].sort(
    (a, b) => STEP_ORDER.indexOf(a.key) - STEP_ORDER.indexOf(b.key),
  )

  return (
    <div
      data-testid="onboarding-checklist"
      className="rounded-xl border border-border bg-surface p-4 mb-4"
    >
      <div className="flex items-start justify-between gap-2 mb-3">
        <div>
          <h3 className="text-sm font-semibold text-primary">{t('onboarding30.title')}</h3>
          <p className="text-xs text-secondary mt-0.5">
            {t('onboarding30.progress', { done: data.completed_count, total: data.total })}
          </p>
        </div>
        <button
          onClick={() => { dismiss.mutate() }}
          className="text-secondary hover:text-primary p-1 rounded"
          aria-label={t('onboarding30.dismiss')}
          data-testid="onboarding-dismiss"
        >
          <X className="w-4 h-4" />
        </button>
      </div>

      {/* Progress bar */}
      <div className="h-1.5 w-full rounded-full bg-muted/40 mb-3 overflow-hidden">
        <div
          className="h-full bg-brand transition-all"
          style={{ width: `${String(data.percent_done)}%` }}
          data-testid="onboarding-progress-bar"
        />
      </div>

      <ol className="space-y-1.5">
        {steps.map((s, i) => (
          <li key={s.key}>
            <Link
              to={s.path}
              className="flex items-center gap-2 rounded-md px-2 py-1.5 hover:bg-bg group"
              data-testid={`onboarding-step-${s.key}`}
            >
              {s.done
                ? <CheckCircle2 className="w-4 h-4 text-green-500 shrink-0" />
                : <Circle className="w-4 h-4 text-secondary shrink-0" />}
              <span className={`text-sm flex-1 ${s.done ? 'text-secondary line-through' : 'text-primary'}`}>
                {i + 1}. {t(`onboarding30.steps.${s.key}`)}
              </span>
              {!s.done && (
                <ArrowRight className="w-3.5 h-3.5 text-secondary opacity-0 group-hover:opacity-100 shrink-0" />
              )}
            </Link>
          </li>
        ))}
      </ol>

      {data.all_complete && (
        <p className="text-xs text-green-500 mt-3 px-2">{t('onboarding30.allDone')}</p>
      )}
    </div>
  )
}
