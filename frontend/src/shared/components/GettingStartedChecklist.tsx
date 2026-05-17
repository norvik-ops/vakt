import { useState } from 'react'
import { Link } from 'react-router-dom'
import { CheckCircle2, Circle } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useQuery } from '@tanstack/react-query'
import { apiFetch } from '../../api/client'
import { useFrameworks } from '../../modules/secvitals/hooks/useFrameworks'
import { useAssets } from '../../modules/secpulse/hooks/useAssets'
import { useTeamMembers } from '../../hooks/useTeam'

const DISMISS_KEY = 'vakt_onboarding_dismissed'

function useTOTPStatus() {
  return useQuery<{ enabled: boolean }>({
    queryKey: ['auth', '2fa', 'status'],
    queryFn: () => apiFetch<{ enabled: boolean }>('/auth/2fa/status'),
    staleTime: 5 * 60 * 1000,
    retry: false,
  })
}

function useHasEvidence() {
  return useQuery<boolean>({
    queryKey: ['checklist', 'evidence'],
    queryFn: async () => {
      // A small proof-of-evidence query — we just need to know if any exists.
      // We re-use the auto-evidence endpoint since it surfaces pending uploads.
      const data = await apiFetch<{ count?: number; data?: unknown[] }>('/secvitals/evidence/auto?limit=1')
      const count = data?.count ?? (Array.isArray((data as { data?: unknown[] })?.data) ? (data as { data: unknown[] }).data.length : 0)
      return count > 0
    },
    staleTime: 30_000,
    retry: false,
  })
}

interface Step {
  id: string
  labelKey: keyof { org: string; framework: string; asset: string; team: string; evidence: string; mfa: string }
  done: boolean
  to: string
}

export function GettingStartedChecklist() {
  const { t } = useTranslation()
  const [dismissed, setDismissed] = useState(
    () => localStorage.getItem(DISMISS_KEY) === '1',
  )

  const { data: frameworks } = useFrameworks()
  const { pagination: assetPagination } = useAssets(1, 1)
  const { data: members } = useTeamMembers()
  const { data: totpStatus } = useTOTPStatus()
  const { data: hasEvidence } = useHasEvidence()

  const steps: Step[] = [
    {
      id: 'org',
      labelKey: 'org',
      done: true,
      to: '/settings',
    },
    {
      id: 'framework',
      labelKey: 'framework',
      done: (frameworks?.length ?? 0) > 0,
      to: '/secvitals/frameworks',
    },
    {
      id: 'asset',
      labelKey: 'asset',
      done: (assetPagination?.total ?? 0) > 0,
      to: '/secpulse/assets',
    },
    {
      id: 'team',
      labelKey: 'team',
      done: (members?.length ?? 0) > 1,
      to: '/settings/team',
    },
    {
      id: 'evidence',
      labelKey: 'evidence',
      done: hasEvidence ?? false,
      to: '/secvitals/frameworks',
    },
    {
      id: '2fa',
      labelKey: 'mfa',
      done: totpStatus?.enabled ?? false,
      to: '/account',
    },
  ]

  const completedCount = steps.filter((s) => s.done).length
  const allDone = completedCount === steps.length

  if (dismissed || allDone) return null

  function handleDismiss() {
    localStorage.setItem(DISMISS_KEY, '1')
    setDismissed(true)
  }

  const pct = Math.round((completedCount / steps.length) * 100)

  return (
    <section
      aria-label={t('onboarding.title')}
      className="rounded-lg border border-border bg-surface p-4 space-y-3"
    >
      {/* Header */}
      <div className="flex items-center justify-between">
        <h2 className="text-[13px] font-semibold text-primary">{t('onboarding.title')}</h2>
        <span className="text-[11px] text-secondary">
          {t('onboarding.completed', { count: completedCount, total: steps.length })}
        </span>
      </div>

      {/* Progress bar */}
      <div
        role="progressbar"
        aria-valuenow={pct}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-label={t('onboarding.completed', { count: completedCount, total: steps.length })}
        className="h-1.5 rounded-full bg-border overflow-hidden"
      >
        <div
          className="h-full rounded-full bg-brand transition-all duration-500"
          style={{ width: `${pct}%` }}
        />
      </div>

      {/* Steps */}
      <ul className="space-y-1.5">
        {steps.map((step) => (
          <li key={step.id}>
            <Link
              to={step.to}
              className="flex items-center gap-2.5 rounded-md px-2 py-1 hover:bg-surface2 transition-colors group"
            >
              {step.done ? (
                <CheckCircle2
                  className="w-4 h-4 text-[#22c55e] shrink-0"
                  aria-hidden="true"
                />
              ) : (
                <Circle
                  className="w-4 h-4 text-secondary shrink-0"
                  aria-hidden="true"
                />
              )}
              <span
                className={`text-[12px] ${step.done ? 'line-through text-secondary' : 'text-primary group-hover:text-brand'}`}
              >
                {t(`onboarding.steps.${step.labelKey}`)}
              </span>
            </Link>
          </li>
        ))}
      </ul>

      {/* Dismiss */}
      <div className="pt-1">
        <button
          type="button"
          onClick={handleDismiss}
          className="text-[11px] text-secondary hover:text-primary underline transition-colors"
        >
          {t('onboarding.dismiss')}
        </button>
      </div>
    </section>
  )
}
