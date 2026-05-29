import { useState } from 'react'
import { ShieldAlert, CheckCircle2, HelpCircle } from 'lucide-react'
import { Button } from '../../../components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '../../../components/ui/dialog'
import { useClassifyReportingObligation } from '../hooks/useIncidents'
import type { ClassificationResult } from '../types'

interface Props {
  incidentId: string
  open: boolean
  onClose: () => void
  onResult?: (result: ClassificationResult) => void
}

const QUESTIONS = [
  {
    key: 'essential_service' as const,
    text: 'Ist ein essenzieller Dienst im Sinne des BSIG-neu (§ 28) betroffen?',
  },
  {
    key: 'customer_data' as const,
    text: 'Sind externe Kundendaten oder vertrauliche Partnerdaten betroffen?',
  },
  {
    key: 'personal_data' as const,
    text: 'Wurden personenbezogene Daten kompromittiert (→ ggf. DSGVO-Meldepflicht Art. 33)?',
  },
]

const OBLIGATION_CONFIG: Record<ClassificationResult['obligation'], {
  icon: React.ReactNode
  label: string
  css: string
}> = {
  probably: {
    icon: <ShieldAlert className="w-5 h-5 text-red-400" />,
    label: 'Meldepflicht wahrscheinlich (§ 32 BSIG-neu)',
    css: 'text-red-400',
  },
  unclear: {
    icon: <HelpCircle className="w-5 h-5 text-amber-400" />,
    label: 'Unklar — rechtliche Prüfung empfohlen',
    css: 'text-amber-400',
  },
  none: {
    icon: <CheckCircle2 className="w-5 h-5 text-green-400" />,
    label: 'Keine Hinweise auf NIS2-Meldepflicht',
    css: 'text-green-400',
  },
}

export function ClassifyReportingWizard({ incidentId, open, onClose, onResult }: Props) {
  const [step, setStep] = useState(0)
  const [answers, setAnswers] = useState({
    essential_service: false,
    customer_data: false,
    personal_data: false,
  })
  const [result, setResult] = useState<ClassificationResult | null>(null)
  const classify = useClassifyReportingObligation(incidentId)

  function handleAnswer(yes: boolean) {
    const key = QUESTIONS[step].key
    const updated = { ...answers, [key]: yes }
    setAnswers(updated)

    if (step < QUESTIONS.length - 1) {
      setStep(step + 1)
    } else {
      classify.mutate(updated, {
        onSuccess: (res) => {
          setResult(res)
          onResult?.(res)
        },
      })
    }
  }

  function handleClose() {
    setStep(0)
    setAnswers({ essential_service: false, customer_data: false, personal_data: false })
    setResult(null)
    onClose()
  }

  const cfg = result ? OBLIGATION_CONFIG[result.obligation] : null

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) { handleClose(); } }}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Meldepflicht klassifizieren (NIS2 § 32 BSIG-neu)</DialogTitle>
        </DialogHeader>

        {!result && (
          <div className="space-y-4 pt-2" data-testid="classify-question">
            <p className="text-xs text-muted-foreground">
              Frage {step + 1} von {QUESTIONS.length}
            </p>
            <p className="text-sm font-medium">{QUESTIONS[step].text}</p>
            <div className="flex gap-3 pt-1">
              <Button
                variant="outline"
                className="flex-1"
                disabled={classify.isPending}
                onClick={() => { handleAnswer(true); }}
                data-testid="classify-yes-btn"
              >
                Ja
              </Button>
              <Button
                variant="outline"
                className="flex-1"
                disabled={classify.isPending}
                onClick={() => { handleAnswer(false); }}
                data-testid="classify-no-btn"
              >
                Nein
              </Button>
            </div>
          </div>
        )}

        {result && cfg && (
          <div className="space-y-3 pt-2" data-testid="classify-result">
            <div className={`flex items-center gap-2 font-semibold ${cfg.css}`}>
              {cfg.icon}
              <span>{cfg.label}</span>
            </div>
            <p className="text-sm text-muted-foreground">{result.reason}</p>
            {result.authority && (
              <p className="text-xs text-muted-foreground">
                Zuständige Behörde:{' '}
                <span className="font-medium text-foreground" data-testid="classify-authority">
                  {result.authority}
                </span>
              </p>
            )}
            {result.obligation === 'probably' && (
              <div className="text-xs text-red-400 bg-red-500/10 rounded p-2">
                Meldefristen: 24h (Frühwarnung), 72h (Vollständige Meldung), 30 Tage (Abschlussbericht).
                Meldung über das BSI MELDUNG Portal einreichen.
              </div>
            )}
            <Button
              className="w-full"
              onClick={handleClose}
              data-testid="classify-close-btn"
            >
              Schließen
            </Button>
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}
