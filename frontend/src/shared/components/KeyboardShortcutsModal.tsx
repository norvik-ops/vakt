import { useTranslation } from 'react-i18next'

import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '../../components/ui/dialog'

interface Props {
  open: boolean
  onClose: () => void
}

const SHORTCUTS = [
  {
    category: 'Aktionen',
    items: [
      { keys: ['⌘', 'K'], label: 'Globale Suche öffnen' },
      { keys: ['?'], label: 'Diese Hilfe anzeigen' },
      { keys: ['Esc'], label: 'Modals schließen' },
    ],
  },
  {
    category: 'Navigation',
    items: [
      { keys: ['g', 'd'], label: 'Dashboard' },
      { keys: ['g', 'f'], label: 'Findings' },
      { keys: ['g', 'r'], label: 'Risiken' },
      { keys: ['g', 'i'], label: 'Incidents' },
    ],
  },
]

export function KeyboardShortcutsModal({ open, onClose }: Props) {
  const { t } = useTranslation()
  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) onClose() }}>
      <DialogContent className="max-w-sm">
        <DialogHeader>
          <DialogTitle>{t('keyboardShortcuts.title')}</DialogTitle>
        </DialogHeader>

        <div className="mt-2 space-y-5">
          {SHORTCUTS.map((group) => (
            <div key={group.category}>
              <p className="text-[10px] font-semibold text-secondary uppercase tracking-wider mb-2">
                {group.category}
              </p>
              <table className="w-full text-sm">
                <tbody>
                  {group.items.map((item) => (
                    <tr key={item.label} className="border-b border-border last:border-0">
                      <td className="py-2 pr-4 text-primary">{item.label}</td>
                      <td className="py-2 text-right">
                        <span className="flex items-center justify-end gap-1">
                          {item.keys.map((k, i) => (
                            <kbd
                              key={i}
                              className="inline-flex items-center px-1.5 py-0.5 rounded border border-border bg-muted text-[11px] font-mono text-secondary"
                            >
                              {k}
                            </kbd>
                          ))}
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ))}
        </div>

        <div className="mt-4 flex justify-end">
          <button
            onClick={onClose}
            className="text-sm text-secondary hover:text-primary transition-colors"
          >
            {t('common.close')}
          </button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
