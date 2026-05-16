import { Component, type ErrorInfo, type ReactNode } from 'react'

interface Props {
  children: ReactNode
}

interface State {
  hasError: boolean
  error: Error | null
}

export class ErrorBoundary extends Component<Props, State> {
  state: State = { hasError: false, error: null }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('[ErrorBoundary]', error, info.componentStack)
  }

  render() {
    if (this.state.hasError) {
      return (
        <div className="min-h-screen flex items-center justify-center bg-background p-8">
          <div className="max-w-md w-full rounded-lg border border-destructive/30 bg-destructive/5 p-8 text-center">
            <h1 className="text-xl font-semibold text-destructive mb-2">Unerwarteter Fehler</h1>
            <p className="text-sm text-muted-foreground mb-4">
              Ein unerwarteter Fehler ist aufgetreten. Bitte Seite neu laden.
            </p>
            {this.state.error && (
              <pre className="text-left text-xs bg-muted rounded p-3 overflow-auto max-h-40 mb-4">
                {this.state.error.message}
              </pre>
            )}
            <button
              onClick={() => window.location.reload()}
              className="px-4 py-2 rounded bg-primary text-primary-foreground text-sm hover:bg-primary/90 transition-colors"
            >
              Seite neu laden
            </button>
          </div>
        </div>
      )
    }
    return this.props.children
  }
}
