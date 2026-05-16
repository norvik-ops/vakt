import { useState } from 'react'
import { FileText, ChevronRight } from 'lucide-react'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '../../../components/ui/dialog'
import { Button } from '../../../components/ui/button'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { useAVVTemplates, useCreateAVVFromTemplate } from '../hooks/useAVVTemplates'
import type { AVVTemplate } from '../types'

interface AVVTemplatePickerDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onCreated?: () => void
}

type Step = 'pick' | 'fill'

interface VarFormState {
  auftraggeber: string
  auftragnehmer: string
  datum: string
  zweck: string
  [key: string]: string
}

function emptyVars(tpl: AVVTemplate): VarFormState {
  const state: VarFormState = {
    auftraggeber: '',
    auftragnehmer: '',
    datum: new Date().toLocaleDateString('de-DE'),
    zweck: '',
  }
  for (const v of tpl.variables) {
    if (!(v in state)) state[v] = ''
  }
  return state
}

const VAR_LABELS: Record<string, string> = {
  auftraggeber: 'Auftraggeber (Verantwortlicher)',
  auftragnehmer: 'Auftragnehmer (Auftragsverarbeiter)',
  datum: 'Datum',
  zweck: 'Zweck der Verarbeitung',
}

export function AVVTemplatePickerDialog({
  open,
  onOpenChange,
  onCreated,
}: AVVTemplatePickerDialogProps) {
  const [step, setStep] = useState<Step>('pick')
  const [selected, setSelected] = useState<AVVTemplate | null>(null)
  const [vars, setVars] = useState<VarFormState>({ auftraggeber: '', auftragnehmer: '', datum: '', zweck: '' })

  const { data: templates, isLoading } = useAVVTemplates()
  const createFromTemplate = useCreateAVVFromTemplate()

  function handleSelectTemplate(tpl: AVVTemplate) {
    setSelected(tpl)
    setVars(emptyVars(tpl))
    setStep('fill')
  }

  function handleBack() {
    setStep('pick')
    setSelected(null)
  }

  function handleClose(open: boolean) {
    if (!open) {
      setStep('pick')
      setSelected(null)
    }
    onOpenChange(open)
  }

  function handleConfirm() {
    if (!selected) return
    createFromTemplate.mutate(
      { template_id: selected.id, vars },
      {
        onSuccess: () => {
          handleClose(false)
          onCreated?.()
        },
      },
    )
  }

  const allVarsFilled = selected
    ? selected.variables.every((v) => vars[v]?.trim())
    : false

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className="max-w-xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>
            {step === 'pick' ? 'Vorlage auswählen' : `Vorlage ausfüllen — ${selected?.title}`}
          </DialogTitle>
        </DialogHeader>

        {step === 'pick' && (
          <div className="space-y-2 py-2">
            {isLoading && (
              <div className="flex items-center justify-center h-24">
                <div className="w-5 h-5 border-2 border-primary border-t-transparent rounded-full animate-spin" />
              </div>
            )}
            {templates?.map((tpl) => (
              <button
                key={tpl.id}
                type="button"
                className="w-full text-left flex items-start gap-3 p-3 rounded-lg border border-border hover:bg-muted/50 transition-colors group"
                onClick={() => handleSelectTemplate(tpl)}
              >
                <FileText className="w-5 h-5 mt-0.5 text-primary shrink-0" />
                <div className="flex-1 min-w-0">
                  <p className="font-medium text-sm">{tpl.title}</p>
                  <p className="text-xs text-muted-foreground mt-0.5">{tpl.description}</p>
                  <p className="text-xs text-muted-foreground mt-1">
                    Platzhalter: {tpl.variables.join(', ')}
                  </p>
                </div>
                <ChevronRight className="w-4 h-4 text-muted-foreground shrink-0 mt-0.5 group-hover:text-foreground transition-colors" />
              </button>
            ))}
          </div>
        )}

        {step === 'fill' && selected && (
          <div className="space-y-4 py-2">
            {selected.variables.map((v) => (
              <div key={v} className="space-y-1.5">
                <Label>{VAR_LABELS[v] ?? v}</Label>
                <Input
                  placeholder={VAR_LABELS[v] ?? v}
                  value={vars[v] ?? ''}
                  onChange={(e) => setVars((prev) => ({ ...prev, [v]: e.target.value }))}
                />
              </div>
            ))}
          </div>
        )}

        <DialogFooter className="gap-2">
          {step === 'fill' && (
            <Button variant="outline" onClick={handleBack}>
              Zurück
            </Button>
          )}
          <Button variant="outline" onClick={() => handleClose(false)}>
            Abbrechen
          </Button>
          {step === 'fill' && (
            <Button
              onClick={handleConfirm}
              disabled={!allVarsFilled || createFromTemplate.isPending}
            >
              {createFromTemplate.isPending ? 'Erstellen …' : 'AVV erstellen'}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
