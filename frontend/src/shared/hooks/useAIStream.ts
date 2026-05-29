import { useCallback, useEffect, useRef, useState } from 'react'

/**
 * useAIStream — konsumiert die SSE-Streaming-Antwort vom Backend-Endpoint
 * POST /api/v1/<scope>/ai/chat/stream (Sprint 15 S15-5/6).
 *
 * Backend sendet Frames im Format
 *   data: {"content":"..."}\n\n
 *   ...
 *   data: [DONE]\n\n
 *
 * Hook-API:
 *   - text         — bisher empfangener Gesamttext (laufend wachsend)
 *   - isStreaming  — true zwischen start() und Done/Abort
 *   - error        — null oder Error-Objekt nach Fehlschlag
 *   - start(body)  — startet einen neuen Stream; cancelt einen laufenden
 *   - stop()       — bricht den aktiven Stream kontrolliert ab (Stop-Button)
 *   - durationMs   — Wallclock-Latency, ab "start" bis Done/Abort
 *
 * Erfüllt S15-6 (Streaming-Rendering) und S15-7 (Stop-Button) auf der
 * Frontend-Seite.
 */

export interface AIStreamRequest {
  /** Relative API-Pfad ohne /api/v1 prefix, z.B. "/vaktcomply/ai/chat/stream". */
  endpoint: string
  /** System-Prompt — bleibt deterministisch, niemals von User-Input geprägt. */
  system?: string
  /** User-Prompt. */
  prompt: string
  /** Optional: Max-Tokens, Default ist serverseitig 1200. */
  maxTokens?: number
}

function readCsrfToken(): string | null {
  const m = document.cookie.match(/(?:^|;\s*)csrf_token=([^;]+)/)
  return m ? decodeURIComponent(m[1]) : null
}

export function useAIStream() {
  const [text, setText] = useState<string>('')
  const [isStreaming, setIsStreaming] = useState(false)
  const [error, setError] = useState<Error | null>(null)
  const [durationMs, setDurationMs] = useState<number>(0)

  const abortRef = useRef<AbortController | null>(null)
  const startTimeRef = useRef<number>(0)

  // Wenn die Komponente unmounted wird, brechen wir den Stream ab —
  // verhindert ein State-Update auf einer entladenen Komponente.
  useEffect(() => {
    return () => {
      abortRef.current?.abort()
    }
  }, [])

  const stop = useCallback(() => {
    abortRef.current?.abort()
    setIsStreaming(false)
    setDurationMs(Date.now() - startTimeRef.current)
  }, [])

  const start = useCallback(async (req: AIStreamRequest) => {
    // Bestehenden Stream abbrechen, bevor wir einen neuen starten.
    abortRef.current?.abort()
    const controller = new AbortController()
    abortRef.current = controller

    setText('')
    setError(null)
    setIsStreaming(true)
    startTimeRef.current = Date.now()
    setDurationMs(0)

    const csrf = readCsrfToken()
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      Accept: 'text/event-stream',
    }
    if (csrf) headers['X-CSRF-Token'] = csrf

    try {
      const res = await fetch(`/api/v1${req.endpoint}`, {
        method: 'POST',
        credentials: 'include',
        headers,
        body: JSON.stringify({
          system: req.system ?? '',
          prompt: req.prompt,
          max_tokens: req.maxTokens ?? 0,
        }),
        signal: controller.signal,
      })

      if (!res.ok) {
        // Backend gibt JSON-Error bei 4xx/5xx zurück (Quota, RateLimit, …).
        const errBody = await res.json().catch(() => ({ error: `HTTP ${String(res.status)}` })) as { code?: string; error?: string }
        const code = errBody.code ?? `HTTP_${String(res.status)}`
        const msg = errBody.error ?? `HTTP ${String(res.status)}`
        const e = new Error(msg) as Error & { code?: string }
        e.code = code
        throw e
      }

      const body = res.body
      if (!body) {
        throw new Error('AI: kein Stream-Body — Browser unterstützt readable streams nicht')
      }

      const reader = body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''
      let total = ''

       
      for (;;) {
        const { value, done } = await reader.read()
        if (done) { break }
        buffer += decoder.decode(value, { stream: true })

        // SSE-Frames sind doppelt-newline-separiert.
        let idx = buffer.indexOf('\n\n')
        while (idx !== -1) {
          const frame = buffer.slice(0, idx).trim()
          buffer = buffer.slice(idx + 2)
          if (frame.startsWith('data: ')) {
            const payload = frame.slice(6)
            if (payload === '[DONE]') {
              setIsStreaming(false)
              setDurationMs(Date.now() - startTimeRef.current)
              return
            }
            try {
              const obj = JSON.parse(payload) as { content?: string }
              if (obj.content) {
                total += obj.content
                setText(total)
              }
            } catch {
              // ungültiger Frame, übersprungen
            }
          } else if (frame.startsWith('event: error')) {
            // Backend hat einen error-Frame gesendet.
            const dataLine = frame
              .split('\n')
              .find((l) => l.startsWith('data: '))
            const msg = dataLine ? dataLine.slice(6) : 'AI stream error'
            throw new Error(msg)
          }
          idx = buffer.indexOf('\n\n')
        }
      }
      setIsStreaming(false)
      setDurationMs(Date.now() - startTimeRef.current)
    } catch (e: unknown) {
      if (e instanceof Error && e.name === 'AbortError') {
        // explizit per stop() oder unmount abgebrochen — kein Fehler.
        return
      }
      setError(e instanceof Error ? e : new Error('KI-Stream fehlgeschlagen — bitte erneut versuchen'))
      setIsStreaming(false)
      setDurationMs(Date.now() - startTimeRef.current)
    }
  }, [])

  return { text, isStreaming, error, durationMs, start, stop }
}
