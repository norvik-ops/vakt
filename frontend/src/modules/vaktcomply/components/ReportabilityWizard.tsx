import { useState } from 'react'
import { ShieldAlert, CheckCircle2, HelpCircle } from 'lucide-react'
import { Button } from '../../../components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '../../../components/ui/dialog'
import { useAssessReportability } from '../hooks/useIncidents'
import type { ReportabilityResult } from '../types'

interface Props {
  incidentId: string
  open: boolean
  onClose: () => void
  onResult?: (result: ReportabilityResult) => void
}

const QUESTIONS = [
  {
    key: 'affects_external_data' as const,
    text: 'Betrifft der Vorfall verfügbare oder vertrauliche Daten von Kunden oder Partnern?',
  },
  {
    key: 'affects_essential_service' as const,
    text: 'Ist ein essenzieller Dienst im Sinne des BSIG-neu (§ 28) betroffen?',
  },
  {
    key: 'personal_data_compromised' as const,
    text: 'Wurden personenbezogene Daten kompromittiert (→ ggf. zusätzliche DSGVO-Meldepflicht)?',
  },
]

const OBLIGATION_CONFIG = {
  required: {
    icon: <ShieldAlert className="w-5 h-5 text-red-400" />,
    label: 'Meldepflicht wahrscheinlich',
    css: 'text-red-400',
  },
  unknown: {
    icon: <HelpCircle className="w-5 h-5 text-amber-400" />,
    label: 'Unklar — bitte prüfen',
    css: 'text-amber-400',
  },
  not_required: {
    icon: <CheckCircle2 className="w-5 h-5 text-green-400" />,
    label: 'Keine Meldepflicht',
    css: 'text-green-400',
  },
}

export function ReportabilityWizard({ incidentId, open, onClose, onResult }: Props) {
  const [step, setStep] = useState(0)
  const [answers, setAnswers] = useState({ affects_external_data: false, affects_essential_service: false, personal_data_compromised: false })
  const [result, setResult] = useState<ReportabilityResult | null>(null)
  const assess = useAssessReportability(incidentId)

  function handleAnswer(yes: boolean) {
    const key = QUESTIONS[step].key
    const updated = { ...answers, [key]: yes }
    setAnswers(updated)

    if (step < QUESTIONS.length - 1) {
      setStep(step + 1)
    } else {
      assess.mutate(updated, {
        onSuccess: (res) => {
          setResult(res)
          onResult?.(res)
        },
      })
    }
  }

  function handleClose() {
    setStep(0)
    setAnswers({ affects_external_data: false, affects_essential_service: false, personal_data_compromised: false })
    setResult(null)
    onClose()
  }

  const cfg = result ? OBLIGATION_CONFIG[result.obligation] : null

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) { handleClose(); } }}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Meldepflicht prüfen (NIS2)</DialogTitle>
        </DialogHeader>

        {!result && (
          <div className="space-y-4 pt-2" data-testid="reportability-question">
            <p className="text-xs text-muted-foreground">
              Frage {step + 1} von {QUESTIONS.length}
            </p>
            <p className="text-sm font-medium">{QUESTIONS[step].text}</p>
            <div className="flex gap-3 pt-1">
              <Button
                variant="outline"
                className="flex-1"
                disabled={assess.isPending}
                onClick={() => { handleAnswer(true); }}
                data-testid="reportability-yes-btn"
              >
                Ja
              </Button>
              <Button
                variant="outline"
                className="flex-1"
                disabled={assess.isPending}
                onClick={() => { handleAnswer(false); }}
                data-testid="reportability-no-btn"
              >
                Nein
              </Button>
            </div>
          </div>
        )}

        {result && cfg && (
          <div className="space-y-3 pt-2" data-testid="reportability-result">
            <div className={`flex items-center gap-2 font-semibold ${cfg.css}`}>
              {cfg.icon}
              <span>{cfg.label}</span>
            </div>
            <p className="text-sm text-muted-foreground">{result.explanation}</p>
            {result.gdpr_required && (
              <p className="text-xs text-amber-400 bg-amber-500/10 rounded p-2">
                Personenbezogene Daten betroffen — zusätzliche DSGVO-Meldung an die Datenschutzbehörde prüfen (72h-Frist).
              </p>
            )}
            <p className="text-xs text-muted-foreground">
              Zuständige Behörde: <span className="font-medium text-foreground">{result.notification_authority}</span>
            </p>
            <Button className="w-full" onClick={handleClose} data-testid="reportability-close-btn">
              Schließen
            </Button>
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}
