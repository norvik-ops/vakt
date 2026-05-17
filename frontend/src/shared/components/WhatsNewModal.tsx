import { useState, useEffect } from 'react'
import { Sparkles, CheckCircle2 } from 'lucide-react'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '../../components/ui/dialog'
import { useWhatsNew } from '../hooks/useWhatsNew'

const HIGHLIGHTS = [
  'Granulare Modul-Berechtigungen pro Benutzer (Pro)',
  'In-App Update-Benachrichtigungen',
  'Pro-Tier — demnächst verfügbar: erweiterte Rollen',
  'Verbesserter Audit-Export und Compliance-Fortschritt',
]

export function WhatsNewModal() {
  const { isNew, currentVersion, dismiss } = useWhatsNew()
  const [open, setOpen] = useState(false)

  // Open the modal once we know there's a new version
  useEffect(() => {
    if (isNew) {
      setOpen(true)
    }
  }, [isNew])

  function handleDismiss() {
    dismiss()
    setOpen(false)
  }

  if (!isNew && !open) return null

  return (
    <Dialog open={open} onOpenChange={(v) => { if (!v) handleDismiss() }}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <div className="flex items-center gap-2 mb-1">
            <Sparkles className="w-5 h-5 text-brand shrink-0" />
            <DialogTitle>
              Was ist neu in Vakt{currentVersion ? ` ${currentVersion}` : ''}
            </DialogTitle>
          </div>
        </DialogHeader>

        <ul className="mt-3 space-y-2.5">
          {HIGHLIGHTS.map((item) => (
            <li key={item} className="flex items-start gap-2.5">
              <CheckCircle2 className="w-4 h-4 text-brand mt-0.5 shrink-0" />
              <span className="text-[13px] text-primary leading-snug">{item}</span>
            </li>
          ))}
        </ul>

        <DialogFooter className="mt-5">
          <button
            onClick={handleDismiss}
            className="px-4 py-2 rounded-md bg-brand text-white text-[13px] font-medium hover:bg-brand/90 transition-colors"
          >
            Verstanden
          </button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
