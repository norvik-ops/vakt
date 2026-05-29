import { useEffect, useRef, useState } from 'react'
import { Loader2, CheckCircle2, AlertTriangle } from 'lucide-react'

// Sprint 22 / S22-7: Live-Progress-Indikator für einen laufenden Scan.
// Konsumiert GET /api/v1/vaktscan/scans/:id/progress/stream (SSE).
//
// Pattern angelehnt an useNotificationStream (Sprint 17) und useAIStream
// (Sprint 15). Lokaler Reader-State, kein Hook-Pattern weil die Komponente
// scan-id-spezifisch ist und mit der Detail-Page-Lebensdauer korreliert.

interface ProgressEvent {
  scan_id: string
  phase: 'started' | 'fetching' | 'scanning' | 'parsing' | 'finished' | 'failed'
  percent?: number
  message?: string
  ts: string
}

interface Props {
  scanId: string
  /** Optional callback, wenn der Scan terminal (finished/failed) wird. */
  onTerminal?: (phase: 'finished' | 'failed') => void
}

function readCsrfToken(): string | null {
  const m = document.cookie.match(/(?:^|;\s*)csrf_token=([^;]+)/)
  return m ? decodeURIComponent(m[1]) : null
}

export function ScanProgressIndicator({ scanId, onTerminal }: Props) {
  const [event, setEvent] = useState<ProgressEvent | null>(null)
  const [done, setDone] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const abortRef = useRef<AbortController | null>(null)

  useEffect(() => {
    if (!scanId) return
    let cancelled = false
    const controller = new AbortController()
    abortRef.current = controller

    const stream = async () => {
      try {
        const csrf = readCsrfToken()
        const headers: Record<string, string> = { Accept: 'text/event-stream' }
        if (csrf) headers['X-CSRF-Token'] = csrf
        const res = await fetch(`/api/v1/vaktscan/scans/${scanId}/progress/stream`, {
          credentials: 'include',
          headers,
          signal: controller.signal,
        })
        if (!res.ok) {
          setError(`HTTP ${String(res.status)}`)
          return
        }
        if (!res.body) {
          setError('no stream body')
          return
        }
        const reader = res.body.getReader()
        const decoder = new TextDecoder()
        let buffer = ''
        while (!cancelled) {
          const { value, done: streamDone } = await reader.read()
          if (streamDone) break
          buffer += decoder.decode(value, { stream: true })
          let idx = buffer.indexOf('\n\n')
          while (idx !== -1) {
            const frame = buffer.slice(0, idx).trim()
            buffer = buffer.slice(idx + 2)
            if (frame.startsWith('event: ping')) {
              idx = buffer.indexOf('\n\n')
              continue
            }
            if (frame.startsWith('data: ')) {
              const payload = frame.slice(6)
              if (payload === '[DONE]') {
                setDone(true)
                idx = buffer.indexOf('\n\n')
                continue
              }
              try {
                const evt = JSON.parse(payload) as ProgressEvent
                setEvent(evt)
                if (evt.phase === 'finished' || evt.phase === 'failed') {
                  onTerminal?.(evt.phase)
                }
              } catch {
                // ungültiger Frame
              }
            }
            idx = buffer.indexOf('\n\n')
          }
        }
      } catch (e: unknown) {
        if (e instanceof Error && e.name !== 'AbortError') {
          setError(e.message)
        }
      }
    }
    void stream()

    return () => {
      cancelled = true
      controller.abort()
    }
  }, [scanId, onTerminal])

  if (error && !event) {
    return (
      <div className="flex items-center gap-2 text-xs text-red-600">
        <AlertTriangle className="w-3 h-3" />
        <span>Live-Progress nicht verfügbar: {error}</span>
      </div>
    )
  }

  if (!event) {
    return (
      <div className="flex items-center gap-2 text-xs text-secondary">
        <Loader2 className="w-3 h-3 animate-spin" />
        <span>Verbinde mit Scan-Stream…</span>
      </div>
    )
  }

  const isTerminal = event.phase === 'finished' || event.phase === 'failed' || done
  const phaseLabel: Record<string, string> = {
    started: 'Gestartet',
    fetching: 'Daten holen',
    scanning: 'Scan läuft',
    parsing: 'Ergebnis verarbeiten',
    finished: 'Abgeschlossen',
    failed: 'Fehlgeschlagen',
  }
  const isFail = event.phase === 'failed'
  const Icon = isTerminal ? (isFail ? AlertTriangle : CheckCircle2) : Loader2
  const colorClass = isFail
    ? 'text-severity-critical'
    : isTerminal
      ? 'text-severity-low'
      : 'text-brand'

  return (
    <div className="rounded-lg border border-border bg-surface p-3 space-y-2">
      <div className="flex items-center gap-2 text-sm">
        <Icon className={`w-4 h-4 ${colorClass} ${isTerminal ? '' : 'animate-spin'}`} />
        <span className="font-medium text-primary">{phaseLabel[event.phase] ?? event.phase}</span>
        {event.percent != null && event.percent > 0 && (
          <span className="text-xs text-secondary ml-auto">{event.percent}%</span>
        )}
      </div>
      {event.message && (
        <p className="text-xs text-secondary">{event.message}</p>
      )}
      {event.percent != null && event.percent > 0 && (
        <div className="w-full bg-gray-200 rounded-full h-1">
          <div
            className={`h-1 rounded-full transition-all ${isFail ? 'bg-severity-critical' : 'bg-brand'}`}
            style={{ width: `${String(event.percent)}%` }}
          />
        </div>
      )}
    </div>
  )
}
