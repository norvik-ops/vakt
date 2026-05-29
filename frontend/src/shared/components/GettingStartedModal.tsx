import { useState } from 'react'
import { Link } from 'react-router-dom'
import { CheckCircle2, Circle, Rocket } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from '../../components/ui/dialog'
import { Button } from '../../components/ui/button'
import { useFrameworks } from '../../modules/vaktcomply/hooks/useFrameworks'
import { useTeamMembers } from '../../hooks/useTeam'
import { useTOTPStatus, useHasEvidence, useHasVvt } from './GettingStartedChecklist'

const MODAL_SHOWN_KEY = 'vakt_onboarding_modal_shown'

export function GettingStartedModal() {
  const { t } = useTranslation()
  const [open, setOpen] = useState(() => localStorage.getItem(MODAL_SHOWN_KEY) !== '1')

  const { data: frameworks } = useFrameworks()
  const { data: members } = useTeamMembers()
  const { data: totpStatus } = useTOTPStatus()
  const { data: hasEvidence } = useHasEvidence()
  const { data: hasVvt } = useHasVvt()

  const steps = [
    { id: 'framework', done: (frameworks?.length ?? 0) > 0,    to: '/vaktcomply/frameworks', labelKey: 'framework' },
    { id: 'control',   done: hasEvidence ?? false,             to: '/vaktcomply/controls',   labelKey: 'control' },
    { id: 'vvt',       done: hasVvt ?? false,                  to: '/vaktprivacy/vvt',       labelKey: 'vvt' },
    { id: 'org',       done: (members?.length ?? 0) > 1,       to: '/settings',             labelKey: 'org' },
    { id: '2fa',       done: totpStatus?.enabled ?? false,      to: '/account',             labelKey: 'mfa' },
  ] as const

  const completedCount = steps.filter((s) => s.done).length
  const allDone = completedCount === steps.length
  const pct = Math.round((completedCount / steps.length) * 100)

  function handleClose() {
    localStorage.setItem(MODAL_SHOWN_KEY, '1')
    setOpen(false)
  }

  if (allDone) return null

  return (
    <Dialog open={open} onOpenChange={(v) => { if (!v) handleClose() }}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Rocket className="w-5 h-5 text-brand" aria-hidden="true" />
            {t('onboarding.title')}
          </DialogTitle>
          <DialogDescription>
            {t('onboarding.intro')}
          </DialogDescription>
        </DialogHeader>

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
            style={{ width: `${String(pct)}%` }}
          />
        </div>

        <ul className="space-y-1 mt-1">
          {steps.map((step) => (
            <li key={step.id}>
              <Link
                to={step.to}
                onClick={handleClose}
                className="flex items-center gap-2.5 rounded-md px-2 py-1.5 hover:bg-surface2 transition-colors group"
              >
                {step.done ? (
                  <CheckCircle2 className="w-4 h-4 text-severity-low shrink-0" aria-hidden="true" />
                ) : (
                  <Circle className="w-4 h-4 text-secondary shrink-0" aria-hidden="true" />
                )}
                <span className={`text-[13px] ${step.done ? 'line-through text-secondary' : 'text-primary group-hover:text-brand'}`}>
                  {t(`onboarding.steps.${step.labelKey}`)}
                </span>
              </Link>
            </li>
          ))}
        </ul>

        <div className="flex justify-end pt-2">
          <Button variant="outline" size="sm" onClick={handleClose}>
            {t('onboarding.dismiss')}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
