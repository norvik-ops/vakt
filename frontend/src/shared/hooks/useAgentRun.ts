import { useCallback, useEffect, useRef, useState } from 'react'

// Sprint 18 / S22-8: Hook zur Konsumierung des Agent-Run-SSE-Endpoints
// POST /api/v1/vaktcomply/ai/agent/run. Backend streamt strukturierte
// AgentEvent-Frames (plan, tool_call, tool_result, reflect, final, error)
// und schließt mit "data: [DONE]\n\n".
//
// S32-2: Approval-Flow — run_started + approval_required Events,
// approve() und reject() Calls zu den neuen Backend-Endpoints.

export type AgentEventType =
  | 'plan'
  | 'tool_call'
  | 'tool_result'
  | 'reflect'
  | 'final'
  | 'error'
  | 'approval_required'
  | 'run_started'

export interface AgentEvent {
  type: AgentEventType
  step: number
  message?: string
  tool?: string
  arguments?: unknown
  result?: unknown
}

export interface AgentRunRequest {
  goal: string
  contextHints?: string[]
  maxIterations?: number
}

export interface ApprovalRequired {
  runId: string
  tool: string
  arguments: unknown
}

function readCsrfToken(): string | null {
  const m = document.cookie.match(/(?:^|;\s*)csrf_token=([^;]+)/)
  return m ? decodeURIComponent(m[1]) : null
}

export function useAgentRun() {
  const [events, setEvents] = useState<AgentEvent[]>([])
  const [isRunning, setIsRunning] = useState(false)
  const [error, setError] = useState<Error | null>(null)
  const [durationMs, setDurationMs] = useState(0)
  const [runId, setRunId] = useState<string | null>(null)
  const [pendingApproval, setPendingApproval] = useState<ApprovalRequired | null>(null)

  const abortRef = useRef<AbortController | null>(null)
  const startTimeRef = useRef(0)
  const runIdRef = useRef<string | null>(null)

  useEffect(() => () => abortRef.current?.abort(), [])

  const stop = useCallback(() => {
    abortRef.current?.abort()
    setIsRunning(false)
    setDurationMs(Date.now() - startTimeRef.current)
  }, [])

  const start = useCallback(async (req: AgentRunRequest) => {
    abortRef.current?.abort()
    const controller = new AbortController()
    abortRef.current = controller

    setEvents([])
    setError(null)
    setIsRunning(true)
    setRunId(null)
    runIdRef.current = null
    setPendingApproval(null)
    startTimeRef.current = Date.now()
    setDurationMs(0)

    const csrf = readCsrfToken()
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      Accept: 'text/event-stream',
    }
    if (csrf) headers['X-CSRF-Token'] = csrf

    try {
      const res = await fetch('/api/v1/vaktcomply/ai/agent/run', {
        method: 'POST',
        credentials: 'include',
        headers,
        body: JSON.stringify({
          goal: req.goal,
          context_hints: req.contextHints ?? [],
          max_iterations: req.maxIterations ?? 0,
        }),
        signal: controller.signal,
      })

      if (!res.ok) {
        const body = (await res.json().catch(() => ({}))) as { error?: string; code?: string }
        const e = new Error(body.error ?? `HTTP ${res.status.toString()}`) as Error & { code?: string }
        e.code = body.code
        throw e
      }

      const body = res.body
      if (!body) throw new Error('Browser hat keinen Stream-Body — Update notwendig')

      const reader = body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''

       
      for (;;) {
        const { value, done } = await reader.read()
        if (done) { break }
        buffer += decoder.decode(value, { stream: true })

        let idx = buffer.indexOf('\n\n')
        while (idx !== -1) {
          const frame = buffer.slice(0, idx).trim()
          buffer = buffer.slice(idx + 2)
          if (frame.startsWith('data: ')) {
            const payload = frame.slice(6)
            if (payload === '[DONE]') {
              setIsRunning(false)
              setDurationMs(Date.now() - startTimeRef.current)
              return
            }
            try {
              const evt = JSON.parse(payload) as AgentEvent

              // run_started: RunID aus message extrahieren.
              if (evt.type === 'run_started' && evt.message) {
                runIdRef.current = evt.message
                setRunId(evt.message)
              }
              // approval_required: ApproveCard anzeigen.
              else if (evt.type === 'approval_required' && evt.tool) {
                setPendingApproval({
                  runId: runIdRef.current ?? '',
                  tool: evt.tool ?? '',
                  arguments: evt.arguments ?? {},
                })
              }
              // tool_result nach Approval: ApproveCard ausblenden.
              else if (evt.type === 'tool_result') {
                setPendingApproval(null)
              }

              setEvents((prev) => [...prev, evt])
            } catch {
              // Frame ungültig — überspringen.
            }
          }
          idx = buffer.indexOf('\n\n')
        }
      }
      setIsRunning(false)
      setDurationMs(Date.now() - startTimeRef.current)
    } catch (e: unknown) {
      if (e instanceof Error && e.name === 'AbortError') return
      setError(e instanceof Error ? e : new Error('Agent-Ausführung fehlgeschlagen — bitte erneut versuchen'))
      setIsRunning(false)
      setDurationMs(Date.now() - startTimeRef.current)
    }
  }, [])

  const approve = useCallback(async (rid: string) => {
    const csrf = readCsrfToken()
    const headers: Record<string, string> = { 'Content-Type': 'application/json' }
    if (csrf) headers['X-CSRF-Token'] = csrf
    await fetch(`/api/v1/vaktcomply/ai/agent/runs/${rid}/approve`, {
      method: 'POST',
      credentials: 'include',
      headers,
    })
  }, [])

  const reject = useCallback(async (rid: string) => {
    const csrf = readCsrfToken()
    const headers: Record<string, string> = { 'Content-Type': 'application/json' }
    if (csrf) headers['X-CSRF-Token'] = csrf
    await fetch(`/api/v1/vaktcomply/ai/agent/runs/${rid}/reject`, {
      method: 'POST',
      credentials: 'include',
      headers,
    })
    setPendingApproval(null)
  }, [])

  return { events, isRunning, error, durationMs, start, stop, runId, pendingApproval, approve, reject }
}
