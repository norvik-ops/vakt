import { Download } from 'lucide-react'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '../../components/ui/dialog'
import { Button } from '../../components/ui/button'
import { useFormatDate } from '../hooks/useFormatDate'
import { useTranslation } from 'react-i18next'

interface RecoveryCodesDialogProps {
  open: boolean
  codes: string[]
  onClose: () => void
}

/**
 * RecoveryCodesDialog shows 8 one-time recovery codes after TOTP setup or
 * regeneration. Provides a "Download as .txt" button and a confirmation close.
 */
export function RecoveryCodesDialog({ open, codes, onClose }: RecoveryCodesDialogProps) {
  const { formatDateTime } = useFormatDate()
  const { t } = useTranslation()
  function downloadCodes() {
    const content = [
      t('recoveryCodes.downloadHeader'),
      '================================',
      t('recoveryCodes.downloadNote'),
      '',
      ...codes,
      '',
      `${t('recoveryCodes.downloadGeneratedAt')}: ${formatDateTime(new Date())}`,
    ].join('\n')

    const blob = new Blob([content], { type: 'text/plain;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = 'vakt-recovery-codes.txt'
    a.click()
    URL.revokeObjectURL(url)
  }

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) onClose() }}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>{t('accountSettingsPage.recoveryCodesTitle')}</DialogTitle>
        </DialogHeader>

        <div className="space-y-4">
          <p className="text-sm text-amber-400 bg-amber-950/30 border border-amber-800/40 rounded p-3">
            {t('recoveryCodes.warning')}
          </p>

          <div className="p-4 rounded bg-muted font-mono text-sm grid grid-cols-2 gap-2">
            {codes.map((code) => (
              <span key={code} className="select-all tracking-wider">
                {code}
              </span>
            ))}
          </div>

          <Button variant="outline" className="w-full" onClick={downloadCodes}>
            <Download className="mr-2 h-4 w-4" />
            {t('recoveryCodes.downloadBtn')}
          </Button>
        </div>

        <DialogFooter>
          <Button className="w-full" onClick={onClose}>
            {t('recoveryCodes.confirmed')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
