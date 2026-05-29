import { useState } from 'react'
import { Bot, CheckCircle, AlertTriangle, ChevronRight } from 'lucide-react'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../../../components/ui/dialog'
import { Button } from '../../../components/ui/button'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Badge } from '../../../components/ui/badge'
import { useClassifyAISystem } from '../hooks/useAISystems'
import { WIZARD_STEPS, getStep } from './aiWizardTree'
import type { WizardResult } from './aiWizardTree'
import { RISK_CLASS_LABELS, RISK_CLASS_CSS } from './aiRiskClassConfig'

type WizardState =
  | { phase: 'question'; stepId: string }
  | { phase: 'result'; result: WizardResult; answers: Record<string, boolean> }

interface Props {
  systemId: string
  systemName: string
  open: boolean
  onClose: () => void
}

export function AIClassificationWizard({ systemId, systemName, open, onClose }: Props) {
  const [state, setState] = useState<WizardState>({ phase: 'question', stepId: 'step_prohibited' })
  const [classifiedBy, setClassifiedBy] = useState('')
  const [answers, setAnswers] = useState<Record<string, boolean>>({})
  const classify = useClassifyAISystem(systemId)

  function handleAnswer(yes: boolean) {
    if (state.phase !== 'question') return
    const step = getStep(state.stepId)
    if (!step) return
    const newAnswers = { ...answers, [step.id]: yes }
    setAnswers(newAnswers)
    const result = yes ? step.yesResult : step.noResult
    const next = yes ? step.yesLeadsTo : step.noLeadsTo
    if (result) {
      setState({ phase: 'result', result, answers: newAnswers })
    } else if (next) {
      setState({ phase: 'question', stepId: next })
    }
  }

  function handleSave() {
    if (state.phase !== 'result') return
    classify.mutate(
      {
        risk_class: state.result.riskClass,
        rationale: state.result.rationale,
        classified_by: classifiedBy,
        wizard_answers: state.answers,
      },
      {
        onSuccess: () => {
          onClose()
          resetWizard()
        },
      },
    )
  }

  function resetWizard() {
    setState({ phase: 'question', stepId: 'step_prohibited' })
    setAnswers({})
    setClassifiedBy('')
  }

  function handleClose() {
    onClose()
    resetWizard()
  }

  const currentStep = state.phase === 'question' ? getStep(state.stepId) : null
  const stepIndex = state.phase === 'question' ? WIZARD_STEPS.findIndex((s) => s.id === state.stepId) : WIZARD_STEPS.length
  const progress = Math.round((stepIndex / WIZARD_STEPS.length) * 100)

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) handleClose() }}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Bot className="w-5 h-5 text-primary" />
            Risikoklassifizierung — {systemName}
          </DialogTitle>
        </DialogHeader>

        {/* Progress bar */}
        <div className="w-full bg-muted rounded-full h-1.5 -mt-1">
          <div
            className="bg-primary h-1.5 rounded-full transition-all duration-300"
            style={{ width: `${state.phase === 'result' ? 100 : progress}%` }}
          />
        </div>

        {state.phase === 'question' && currentStep && (
          <div className="space-y-4 py-2">
            <div className="text-xs text-muted-foreground font-medium uppercase tracking-wide">
              Schritt {stepIndex + 1} von {WIZARD_STEPS.length}
            </div>
            <p className="font-medium text-sm leading-relaxed">{currentStep.question}</p>
            <div className="bg-muted/50 rounded-lg p-3 text-xs text-muted-foreground leading-relaxed">
              {currentStep.explanation}
            </div>
            <p className="text-xs text-primary/70">{currentStep.article}</p>
            <div className="flex gap-3 pt-1">
              <Button
                className="flex-1"
                variant="outline"
                onClick={() => { handleAnswer(false); }}
                data-testid="wizard-no-btn"
              >
                Nein
              </Button>
              <Button
                className="flex-1"
                onClick={() => { handleAnswer(true); }}
                data-testid="wizard-yes-btn"
              >
                Ja
                <ChevronRight className="w-4 h-4 ml-1" />
              </Button>
            </div>
          </div>
        )}

        {state.phase === 'result' && (
          <div className="space-y-4 py-2" data-testid="wizard-result">
            <div className="flex items-center gap-2">
              {state.result.riskClass === 'unacceptable' ? (
                <AlertTriangle className="w-5 h-5 text-red-400" />
              ) : (
                <CheckCircle className="w-5 h-5 text-green-400" />
              )}
              <p className="font-semibold">Klassifizierungsergebnis</p>
            </div>
            <div className="flex items-center gap-2">
              <span className="text-sm text-muted-foreground">Risikoklasse:</span>
              <Badge className={RISK_CLASS_CSS[state.result.riskClass] ?? ''}>
                {RISK_CLASS_LABELS[state.result.riskClass] ?? state.result.riskClass}
              </Badge>
            </div>
            <div className="bg-muted/50 rounded-lg p-3 text-xs leading-relaxed">
              <p className="font-medium mb-1">Begründung</p>
              <p className="text-muted-foreground">{state.result.rationale}</p>
            </div>
            <p className="text-xs text-primary/70">Rechtsgrundlage: {state.result.article}</p>
            <div className="space-y-1.5">
              <Label className="text-xs">Klassifiziert durch (optional)</Label>
              <Input
                placeholder="Name der verantwortlichen Person"
                value={classifiedBy}
                onChange={(e) => { setClassifiedBy(e.target.value); }}
                data-testid="wizard-classified-by"
              />
            </div>
          </div>
        )}

        <DialogFooter>
          <Button variant="outline" onClick={handleClose}>
            Abbrechen
          </Button>
          {state.phase === 'result' && (
            <Button onClick={handleSave} disabled={classify.isPending} data-testid="wizard-save-btn">
              {classify.isPending ? 'Speichern …' : 'Klassifizierung speichern'}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
