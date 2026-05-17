import type { LucideIcon } from 'lucide-react'
import { X } from 'lucide-react'
import { Button } from '../../components/ui/button'

interface BulkAction {
  label: string
  icon?: LucideIcon
  variant?: 'default' | 'destructive'
  onClick: () => void
  disabled?: boolean
}

interface BulkActionBarProps {
  selectedCount: number
  onClearSelection: () => void
  actions: BulkAction[]
}

/**
 * Floating bottom bar shown when one or more items are selected.
 * Renders nothing when selectedCount === 0.
 */
export function BulkActionBar({ selectedCount, onClearSelection, actions }: BulkActionBarProps) {
  if (selectedCount === 0) return null

  return (
    <div className="fixed bottom-6 left-1/2 -translate-x-1/2 z-50">
      <div className="flex items-center gap-3 px-4 py-2.5 rounded-xl border border-white/10 bg-[#1a1f2e] shadow-2xl">
        {/* Selection count + clear */}
        <span className="text-sm font-medium text-white whitespace-nowrap">
          {selectedCount} ausgewählt
        </span>

        <Button
          variant="ghost"
          size="icon"
          className="h-6 w-6 text-white/60 hover:text-white hover:bg-white/10"
          onClick={onClearSelection}
          aria-label="Auswahl aufheben"
          title="Auswahl aufheben"
        >
          <X className="w-3.5 h-3.5" />
        </Button>

        <div className="w-px h-5 bg-white/20" />

        {/* Action buttons */}
        <div className="flex items-center gap-2">
          {actions.map((action) => {
            const Icon = action.icon
            return (
              <Button
                key={action.label}
                size="sm"
                variant={action.variant === 'destructive' ? 'destructive' : 'outline'}
                className={
                  action.variant === 'destructive'
                    ? undefined
                    : 'border-white/20 bg-white/10 text-white hover:bg-white/20 hover:text-white'
                }
                onClick={action.onClick}
                disabled={action.disabled}
              >
                {Icon && <Icon className="w-3.5 h-3.5" />}
                {action.label}
              </Button>
            )
          })}
        </div>
      </div>
    </div>
  )
}
