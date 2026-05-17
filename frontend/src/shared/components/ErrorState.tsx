import { AlertCircle } from 'lucide-react'
import { Button } from '../../components/ui/button'

interface ErrorStateProps {
  title?: string
  message?: string
  onRetry?: () => void
}

export function ErrorState({
  title = 'Fehler beim Laden',
  message,
  onRetry,
}: ErrorStateProps) {
  return (
    <div className="flex flex-col items-center justify-center gap-4 py-16 text-center">
      <AlertCircle className="w-10 h-10 text-destructive" aria-hidden="true" />
      <div className="space-y-1">
        <p className="text-sm font-semibold text-primary">{title}</p>
        {message && (
          <p className="text-xs text-secondary max-w-sm">{message}</p>
        )}
      </div>
      {onRetry && (
        <Button variant="outline" size="sm" onClick={onRetry}>
          Erneut versuchen
        </Button>
      )}
    </div>
  )
}
