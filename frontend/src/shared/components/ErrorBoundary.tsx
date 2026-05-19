import { Component, type ErrorInfo, type ReactNode } from 'react'
import { Link } from 'react-router-dom'

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
    // Forward to backend tracing endpoint when available (picks up X-Trace-ID correlation).
    const traceId = document.cookie.match(/trace_id=([^;]+)/)?.[1] ?? 'unknown'
    fetch('/api/v1/errors', {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        message: error.message,
        stack: error.stack,
        component_stack: info.componentStack,
        url: window.location.href,
        trace_id: traceId,
      }),
    }).catch(() => { /* best-effort — ignore if endpoint unavailable */ })
  }

  render() {
    if (this.state.hasError) {
      return (
        <div className="min-h-screen flex items-center justify-center bg-background p-8">
          <div className="max-w-md w-full rounded-lg border border-destructive/30 bg-destructive/5 p-8 text-center">
            <h1 className="text-xl font-semibold text-destructive mb-2">Etwas ist schiefgelaufen</h1>
            {this.state.error && (
              <p className="text-sm text-muted-foreground mb-4">{this.state.error.message}</p>
            )}
            {import.meta.env.DEV && this.state.error?.stack && (
              <pre className="text-left text-xs bg-muted rounded p-3 overflow-auto max-h-40 mb-4 text-secondary whitespace-pre-wrap">
                {this.state.error.stack}
              </pre>
            )}
            <div className="flex flex-col sm:flex-row gap-2 justify-center">
              <button
                onClick={() => window.location.reload()}
                className="px-4 py-2 rounded bg-primary text-primary-foreground text-sm hover:bg-primary/90 transition-colors"
              >
                Seite neu laden
              </button>
              <Link
                to="/"
                className="px-4 py-2 rounded border border-border text-sm text-secondary hover:text-primary hover:border-brand/60 transition-colors"
              >
                Zurück zur Startseite
              </Link>
            </div>
          </div>
        </div>
      )
    }
    return this.props.children
  }
}
