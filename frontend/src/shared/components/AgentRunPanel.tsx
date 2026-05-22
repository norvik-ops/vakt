import { useState } from 'react'
import {
  Sparkles,
  Wrench,
  CheckCircle2,
  AlertCircle,
  ChevronDown,
  ChevronRight,
  Square,
  RotateCw,
  ShieldCheck,
} from 'lucide-react'
import { useAgentRun, type AgentEvent } from '../hooks/useAgentRun'

// Sprint 18 / S22-8: AgentRunPanel — Live-Visualisierung des Plan/Execute/
// Reflect-Loops. Konsumiert useAgentRun und rendert pro Event eine Karte:
//
//   plan         → blau, Sparkles-Icon, Plan-Text
//   tool_call    → ambers, Wrench-Icon, expandable Argumente
//   tool_result  → grün, CheckCircle, expandable JSON
//   reflect      → grau, Brain-Icon, Reflexions-Text
//   final        → grün, Sparkles, Antwort
//   error        → rot, AlertCircle
//
// Approve-Cards-Skelett: für tool_call mit mutate-Eigenschaft (Backend-Marker)
// zeigt eine Approve/Reject-UI an. Aktuell nur Skelett, weil das Backend keine
// pause-and-approve-Semantik hat (ADR-0020 markiert das als Pro-Tier-Feature).

interface JsonBlockProps {
  data: unknown
}

function JsonBlock({ data }: JsonBlockProps) {
  if (data === null || data === undefined) return null
  return (
    <pre className="mt-2 text-[11px] font-mono bg-muted/40 p-2 rounded overflow-x-auto whitespace-pre">
      {JSON.stringify(data, null, 2)}
    </pre>
  )
}

interface EventCardProps {
  evt: AgentEvent
  index: number
}

function EventCard({ evt, index }: EventCardProps) {
  const [expanded, setExpanded] = useState(false)
  const hasDetails =
    evt.arguments !== undefined || evt.result !== undefined

  const meta = (() => {
    switch (evt.type) {
      case 'plan':
        return {
          icon: <Sparkles className="w-4 h-4 text-brand" />,
          tint: 'border-brand/30 bg-brand/5',
          label: `Plan #${evt.step.toString()}`,
        }
      case 'tool_call':
        return {
          icon: <Wrench className="w-4 h-4 text-amber-600" />,
          tint: 'border-amber-300/50 bg-amber-50 dark:border-amber-800/50 dark:bg-amber-950/30',
          label: `Tool: ${evt.tool ?? 'unbekannt'}`,
        }
      case 'tool_result':
        return {
          icon: <CheckCircle2 className="w-4 h-4 text-green-600" />,
          tint: 'border-green-300/50 bg-green-50 dark:border-green-800/50 dark:bg-green-950/30',
          label: `Ergebnis: ${evt.tool ?? 'unbekannt'}`,
        }
      case 'reflect':
        return {
          icon: <RotateCw className="w-4 h-4 text-secondary" />,
          tint: 'border-border bg-muted/20',
          label: 'Reflexion',
        }
      case 'final':
        return {
          icon: <ShieldCheck className="w-4 h-4 text-green-600" />,
          tint: 'border-green-400/50 bg-green-50 dark:border-green-700/50 dark:bg-green-950/30',
          label: 'Antwort',
        }
      case 'error':
        return {
          icon: <AlertCircle className="w-4 h-4 text-destructive" />,
          tint: 'border-destructive/40 bg-destructive/5',
          label: 'Fehler',
        }
    }
  })()

  return (
    <div className={`rounded-lg border p-3 ${meta.tint}`} data-event-index={index}>
      <div className="flex items-start gap-2.5">
        {meta.icon}
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <span className="text-xs font-semibold uppercase tracking-wide text-secondary">
              {meta.label}
            </span>
            {hasDetails && (
              <button
                type="button"
                onClick={() => setExpanded((v) => !v)}
                className="text-[10px] text-secondary hover:text-primary flex items-center gap-0.5"
              >
                {expanded ? <ChevronDown className="w-3 h-3" /> : <ChevronRight className="w-3 h-3" />}
                {expanded ? 'einklappen' : 'JSON'}
              </button>
            )}
          </div>
          {evt.message && (
            <p className="text-sm text-primary mt-1 whitespace-pre-wrap break-words">
              {evt.message}
            </p>
          )}
          {expanded && evt.arguments !== undefined && (
            <div>
              <span className="text-[10px] text-secondary">Arguments:</span>
              <JsonBlock data={evt.arguments} />
            </div>
          )}
          {expanded && evt.result !== undefined && (
            <div>
              <span className="text-[10px] text-secondary">Result:</span>
              <JsonBlock data={evt.result} />
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

interface AgentRunPanelProps {
  // Optional vorausgefüllter Goal — z.B. wenn die Komponente von einer
  // Control-Detail-Page mit Kontext aufgerufen wird.
  initialGoal?: string
  contextHints?: string[]
}

export function AgentRunPanel({ initialGoal = '', contextHints }: AgentRunPanelProps) {
  const [goal, setGoal] = useState(initialGoal)
  const { events, isRunning, error, durationMs, start, stop } = useAgentRun()

  const handleStart = () => {
    if (!goal.trim()) return
    void start({ goal: goal.trim(), contextHints })
  }

  return (
    <div className="space-y-4">
      {/* Goal-Input */}
      <div className="rounded-xl border border-border bg-surface p-4 space-y-3">
        <div className="flex items-center gap-2">
          <Sparkles className="w-4 h-4 text-brand shrink-0" />
          <h2 className="text-sm font-semibold text-primary">Agent-Auftrag</h2>
        </div>
        <textarea
          value={goal}
          onChange={(e) => setGoal(e.target.value)}
          placeholder="Z.B.: Erstelle eine Übersicht aller offenen Controls für NIS2 und schlage Prioritäten vor."
          rows={3}
          disabled={isRunning}
          className="w-full rounded-lg border border-border bg-bg p-3 text-sm text-primary placeholder:text-muted focus:outline-none focus:ring-2 focus:ring-brand/40 resize-none disabled:opacity-60"
        />
        <div className="flex items-center justify-between">
          <p className="text-[11px] text-secondary">
            Der Agent darf nur Tools nutzen, für die du die nötigen Scopes hast (ADR-0020).
          </p>
          {isRunning ? (
            <button
              type="button"
              onClick={stop}
              className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-destructive/10 text-destructive text-sm hover:bg-destructive/20"
            >
              <Square className="w-3.5 h-3.5" />
              Stoppen
            </button>
          ) : (
            <button
              type="button"
              onClick={handleStart}
              disabled={!goal.trim()}
              className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-brand text-white text-sm hover:bg-brand/90 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              <Sparkles className="w-3.5 h-3.5" />
              Starten
            </button>
          )}
        </div>
      </div>

      {/* Status */}
      {(isRunning || events.length > 0 || error) && (
        <div className="flex items-center gap-3 text-xs text-secondary">
          <span>
            {isRunning ? 'läuft…' : 'fertig'} · {events.length.toString()} Events
          </span>
          {durationMs > 0 && (
            <span>
              · {(durationMs / 1000).toFixed(1)}s
            </span>
          )}
        </div>
      )}

      {/* Error */}
      {error && (
        <div className="rounded-lg border border-destructive/40 bg-destructive/5 p-3 text-sm text-destructive flex items-start gap-2">
          <AlertCircle className="w-4 h-4 mt-0.5 shrink-0" />
          <div>
            <p className="font-medium">Agent-Lauf fehlgeschlagen</p>
            <p className="text-xs mt-0.5">{error.message}</p>
          </div>
        </div>
      )}

      {/* Events */}
      <div className="space-y-2">
        {events.map((evt, i) => (
          <EventCard key={i} evt={evt} index={i} />
        ))}
        {!isRunning && events.length === 0 && !error && (
          <p className="text-xs text-secondary text-center py-6">
            Noch kein Lauf gestartet.
          </p>
        )}
      </div>
    </div>
  )
}
