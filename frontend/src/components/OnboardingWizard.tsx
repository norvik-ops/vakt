import { useState } from 'react'
import { Link } from 'react-router-dom'
import { CheckCircle, Circle, Settings, Shield, ClipboardList, AlertTriangle, X } from 'lucide-react'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from './ui/dialog'
import { type OnboardingStatus, useOnboardingStatus, useDismissOnboarding } from '../hooks/useOnboarding'
import { useAuthStore } from '../shared/stores/auth'

// ── Step definition ──────────────────────────────────────────────────────────

type Role = 'Admin' | 'SecurityAnalyst' | 'Viewer' | 'AuditorReadOnly'

interface WizardStep {
  key: keyof OnboardingStatus['steps']
  title: string
  description: string
  actionLabel: string
  actionPath: string
  icon: React.ComponentType<{ className?: string }>
  /**
   * Roles for which this step is relevant. Viewers and Auditors never see
   * configuration tasks (they cannot perform them); SecurityAnalyst sees the
   * day-to-day work but not org-level setup.
   */
  roles: Role[]
}

const STEPS: WizardStep[] = [
  {
    key: 'org_configured',
    title: 'Organisation einrichten',
    description: 'Vergeben Sie einen Namen für Ihre Organisation.',
    actionLabel: 'Zu den Einstellungen',
    actionPath: '/settings',
    icon: Settings,
    roles: ['Admin'],
  },
  {
    key: 'framework_selected',
    title: 'Framework auswählen',
    description: 'Wählen Sie, welche Compliance-Standards für Sie gelten.',
    actionLabel: 'Framework hinzufügen',
    actionPath: '/secvitals',
    icon: Shield,
    roles: ['Admin'],
  },
  {
    key: 'first_control_reviewed',
    title: 'Ersten Control überprüfen',
    description: 'Schauen Sie sich einen Control an und markieren Sie ihn als überprüft.',
    actionLabel: 'Controls ansehen',
    actionPath: '/secvitals',
    icon: ClipboardList,
    roles: ['Admin', 'SecurityAnalyst'],
  },
  {
    key: 'first_risk_created',
    title: 'Erstes Risiko erfassen',
    description: 'Dokumentieren Sie Ihr erstes bekanntes Risiko.',
    actionLabel: 'Risiko erstellen',
    actionPath: '/secvitals/risks',
    icon: AlertTriangle,
    roles: ['Admin', 'SecurityAnalyst'],
  },
]

function highestRole(roles: string[] | undefined): Role {
  if (!roles || roles.length === 0) return 'Viewer'
  if (roles.includes('Admin')) return 'Admin'
  if (roles.includes('SecurityAnalyst')) return 'SecurityAnalyst'
  if (roles.includes('AuditorReadOnly')) return 'AuditorReadOnly'
  return 'Viewer'
}

function filterStepsForRole(role: Role): WizardStep[] {
  return STEPS.filter((step) => step.roles.includes(role))
}

// ── Framework cards (step 2 detail) ─────────────────────────────────────────

const FRAMEWORKS = [
  { label: 'ISO 27001', path: '/secvitals?framework=iso27001' },
  { label: 'NIS2',      path: '/secvitals?framework=nis2' },
  { label: 'BSI-Grundschutz', path: '/secvitals?framework=bsi' },
  { label: 'TISAX',     path: '/secvitals?framework=tisax' },
  { label: 'DORA',      path: '/secvitals?framework=dora' },
]

// ── Wizard modal ─────────────────────────────────────────────────────────────

interface OnboardingWizardProps {
  open: boolean
  onClose: () => void
  status: OnboardingStatus
}

export function OnboardingWizard({ open, onClose, status }: OnboardingWizardProps) {
  const dismiss = useDismissOnboarding()
  const { refetch } = useOnboardingStatus()
  const [activeStep, setActiveStep] = useState<number | null>(null)

  const user = useAuthStore((s) => s.user)
  const role = highestRole(user?.roles)
  const visibleSteps = filterStepsForRole(role)

  // Viewer / AuditorReadOnly have nothing to set up — don't show the wizard.
  if (visibleSteps.length === 0) {
    return null
  }

  const completedCount = visibleSteps.filter((s) => status.steps[s.key]).length

  function handleDismiss() {
    dismiss.mutate(undefined, { onSuccess: onClose })
  }

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) onClose() }}>
      <DialogContent className="max-w-[600px] p-0 overflow-hidden">
        {/* Header */}
        <DialogHeader className="px-6 pt-6 pb-4 border-b border-border">
          <DialogTitle className="text-[17px] font-bold text-primary">
            Willkommen bei Vakt
          </DialogTitle>
          <p className="text-[13px] text-secondary mt-1">
            In wenigen Schritten zur vollständigen Compliance-Dokumentation.
          </p>
        </DialogHeader>

        {/* Steps */}
        <div className="px-6 py-4 space-y-3">
          {visibleSteps.map((step, idx) => {
            const done = status.steps[step.key]
            const Icon = step.icon
            const isExpanded = activeStep === idx
            return (
              <div
                key={step.key}
                className={`rounded-lg border transition-colors ${done ? 'border-[#22c55e]/40 bg-[#22c55e]/5' : 'border-border bg-surface2'}`}
              >
                {/* Step row */}
                <button
                  type="button"
                  className="w-full flex items-center gap-3 px-4 py-3 text-left"
                  onClick={() => setActiveStep(isExpanded ? null : idx)}
                >
                  {/* Circle indicator */}
                  <span className="shrink-0">
                    {done ? (
                      <CheckCircle className="w-5 h-5 text-[#22c55e]" />
                    ) : (
                      <Circle className="w-5 h-5 text-secondary/50" />
                    )}
                  </span>
                  <Icon className={`w-4 h-4 shrink-0 ${done ? 'text-[#22c55e]' : 'text-secondary'}`} />
                  <div className="flex-1 min-w-0">
                    <p className={`text-[13px] font-medium ${done ? 'text-[#22c55e]' : 'text-primary'}`}>
                      {idx + 1}. {step.title}
                    </p>
                    <p className="text-[11px] text-secondary mt-0.5 truncate">{step.description}</p>
                  </div>
                  <span className={`text-[10px] font-semibold uppercase tracking-wide px-2 py-0.5 rounded-full ${done ? 'bg-[#22c55e]/20 text-[#22c55e]' : 'bg-surface text-secondary'}`}>
                    {done ? 'Fertig' : 'Offen'}
                  </span>
                </button>

                {/* Expanded content */}
                {isExpanded && !done && (
                  <div className="px-4 pb-4 border-t border-border pt-3">
                    {step.key === 'framework_selected' ? (
                      <div>
                        <p className="text-[12px] text-secondary mb-2">
                          Welche Compliance-Standards gelten für Sie?
                        </p>
                        <div className="flex flex-wrap gap-2">
                          {FRAMEWORKS.map((fw) => (
                            <Link
                              key={fw.label}
                              to={fw.path}
                              onClick={onClose}
                              className="px-3 py-1.5 rounded-md border border-border bg-surface text-[12px] font-medium text-primary hover:border-brand hover:bg-brand/5 transition-colors"
                            >
                              {fw.label}
                            </Link>
                          ))}
                        </div>
                      </div>
                    ) : (
                      <Link
                        to={step.actionPath}
                        onClick={() => { void refetch(); onClose() }}
                        className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md bg-brand text-white text-[12px] font-medium hover:bg-brand/90 transition-colors"
                      >
                        {step.actionLabel}
                      </Link>
                    )}
                  </div>
                )}
              </div>
            )
          })}
        </div>

        {/* Footer */}
        <div className="px-6 py-4 border-t border-border flex items-center justify-between gap-3 bg-surface2">
          <p className="text-[12px] text-secondary">
            <span className="font-semibold text-primary">{completedCount}</span> von{' '}
            <span className="font-semibold text-primary">{visibleSteps.length}</span> Schritten abgeschlossen
          </p>
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={onClose}
              className="px-3 py-1.5 text-[12px] text-secondary border border-border rounded-md hover:bg-surface transition-colors"
            >
              Schließen
            </button>
            <button
              type="button"
              onClick={handleDismiss}
              disabled={dismiss.isPending}
              className="px-3 py-1.5 text-[12px] text-secondary hover:text-primary transition-colors disabled:opacity-50"
            >
              Nicht mehr anzeigen
            </button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}

// ── Banner (inline, shown on Dashboard) ─────────────────────────────────────

interface OnboardingBannerProps {
  status: OnboardingStatus
  onOpen: () => void
}

export function OnboardingBanner({ status, onOpen }: OnboardingBannerProps) {
  const dismiss = useDismissOnboarding()
  const user = useAuthStore((s) => s.user)
  const role = highestRole(user?.roles)
  const visibleSteps = filterStepsForRole(role)
  const completedCount = visibleSteps.filter((s) => status.steps[s.key]).length

  // No setup work for this role — don't show the banner.
  if (visibleSteps.length === 0) {
    return null
  }

  return (
    <div className="flex items-center justify-between gap-4 px-4 py-2.5 rounded-lg border border-[#f59e0b]/40 bg-[#f59e0b]/8 mb-4">
      <div className="flex items-center gap-2.5 min-w-0">
        <span className="text-[15px] shrink-0">🎯</span>
        <p className="text-[12px] text-primary font-medium truncate">
          <span className="font-bold">{completedCount} von {visibleSteps.length}</span> Einrichtungsschritten abgeschlossen
        </p>
      </div>
      <div className="flex items-center gap-2 shrink-0">
        <button
          type="button"
          onClick={onOpen}
          className="text-[12px] text-brand font-semibold hover:underline whitespace-nowrap"
        >
          Setup fortsetzen →
        </button>
        <button
          type="button"
          onClick={() => dismiss.mutate()}
          disabled={dismiss.isPending}
          aria-label="Onboarding schließen"
          className="text-secondary hover:text-primary transition-colors disabled:opacity-50"
        >
          <X className="w-3.5 h-3.5" />
        </button>
      </div>
    </div>
  )
}
