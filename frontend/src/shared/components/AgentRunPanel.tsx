import { useState } from 'react'
import { useTranslation } from 'react-i18next'
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
  ShieldAlert,
  XCircle,
} from 'lucide-react'
import { useAgentRun, type AgentEvent, type ApprovalRequired } from '../hooks/useAgentRun'

// Sprint 18 / S22-8: AgentRunPanel — Live-Visualisierung des Plan/Execute/
// Reflect-Loops. Konsumiert useAgentRun und rendert pro Event eine Karte:
//
//   plan         → blau, Sparkles-Icon, Plan-Text
//   tool_call    → amber, Wrench-Icon, expandable Argumente
//   tool_result  → grün, CheckCircle, expandable JSON
//   reflect      → grau, Brain-Icon, Reflexions-Text
//   final        → grün, Sparkles, Antwort
//   error        → rot, AlertCircle
//
// S32-2: ApproveCard — wird bei approval_required-Events angezeigt.
//   Zeigt Tool-Name + Args-Preview. Approve/Reject-Buttons rufen das Backend.

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
  const { t } = useTranslation()
  const [expanded, setExpanded] = useState(false)
  const hasDetails =
    evt.arguments !== undefined || evt.result !== undefined

  const unknownTool = t('aiAgent.unknownTool')
  const meta = (() => {
    switch (evt.type) {
      case 'plan':
        return {
          icon: <Sparkles className="w-4 h-4 text-brand" />,
          tint: 'border-brand/30 bg-brand/5',
          label: t('aiAgent.eventPlan', { step: evt.step }),
        }
      case 'tool_call':
        return {
          icon: <Wrench className="w-4 h-4 text-amber-600" />,
          tint: 'border-amber-300/50 bg-amber-50 dark:border-amber-800/50 dark:bg-amber-950/30',
          label: t('aiAgent.eventTool', { tool: evt.tool ?? unknownTool }),
        }
      case 'tool_result':
        return {
          icon: <CheckCircle2 className="w-4 h-4 text-green-600" />,
          tint: 'border-green-300/50 bg-green-50 dark:border-green-800/50 dark:bg-green-950/30',
          label: t('aiAgent.eventResult', { tool: evt.tool ?? unknownTool }),
        }
      case 'reflect':
        return {
          icon: <RotateCw className="w-4 h-4 text-secondary" />,
          tint: 'border-border bg-muted/20',
          label: t('aiAgent.eventReflect'),
        }
      case 'final':
        return {
          icon: <ShieldCheck className="w-4 h-4 text-green-600" />,
          tint: 'border-green-400/50 bg-green-50 dark:border-green-700/50 dark:bg-green-950/30',
          label: t('aiAgent.eventFinal'),
        }
      case 'error':
        return {
          icon: <AlertCircle className="w-4 h-4 text-destructive" />,
          tint: 'border-destructive/40 bg-destructive/5',
          label: t('aiAgent.eventError'),
        }
      case 'approval_required':
        return {
          icon: <ShieldAlert className="w-4 h-4 text-amber-600" />,
          tint: 'border-amber-400/50 bg-amber-50 dark:border-amber-700/50 dark:bg-amber-950/30',
          label: t('aiAgent.eventApproval', { tool: evt.tool ?? unknownTool }),
        }
      case 'run_started':
        return {
          icon: <Sparkles className="w-4 h-4 text-secondary" />,
          tint: 'border-border bg-muted/10',
          label: t('aiAgent.eventRunStarted'),
        }
      default:
        return {
          icon: <AlertCircle className="w-4 h-4 text-secondary" />,
          tint: 'border-border bg-muted/10',
          label: t('aiAgent.eventUnknown'),
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
                onClick={() => { setExpanded((v) => !v); }}
                className="text-[10px] text-secondary hover:text-primary flex items-center gap-0.5"
              >
                {expanded ? <ChevronDown className="w-3 h-3" /> : <ChevronRight className="w-3 h-3" />}
                {expanded ? t('aiAgent.collapse') : 'JSON'}
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
              <span className="text-[10px] text-secondary">{t('aiAgent.argumentsLabel')}</span>
              <JsonBlock data={evt.arguments} />
            </div>
          )}
          {expanded && evt.result !== undefined && (
            <div>
              <span className="text-[10px] text-secondary">{t('aiAgent.resultLabel')}</span>
              <JsonBlock data={evt.result} />
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

interface ApproveCardProps {
  approval: ApprovalRequired
  onApprove: () => Promise<void>
  onReject: () => Promise<void>
}

function ApproveCard({ approval, onApprove, onReject }: ApproveCardProps) {
  const { t } = useTranslation()
  const [loading, setLoading] = useState<'approve' | 'reject' | null>(null)

  const handleApprove = async () => {
    setLoading('approve')
    try {
      await onApprove()
    } finally {
      setLoading(null)
    }
  }

  const handleReject = async () => {
    setLoading('reject')
    try {
      await onReject()
    } finally {
      setLoading(null)
    }
  }

  return (
    <div className="rounded-xl border-2 border-amber-400/60 bg-amber-50 dark:bg-amber-950/30 p-4 space-y-3">
      <div className="flex items-start gap-2.5">
        <ShieldAlert className="w-5 h-5 text-amber-600 shrink-0 mt-0.5" />
        <div className="flex-1 min-w-0">
          <p className="text-sm font-semibold text-amber-900 dark:text-amber-200">
            {t('aiAgent.approvalTitle')}
          </p>
          <p className="text-xs text-amber-700 dark:text-amber-400 mt-0.5">
            {t('aiAgent.approvalBody')}
          </p>
        </div>
      </div>

      <div className="rounded-lg border border-amber-300/50 bg-white/60 dark:bg-amber-950/40 p-3 space-y-1">
        <div className="flex items-center gap-1.5">
          <Wrench className="w-3.5 h-3.5 text-amber-600" />
          <span className="text-xs font-mono font-semibold text-amber-800 dark:text-amber-300">
            {approval.tool}
          </span>
        </div>
        <div>
          <span className="text-[10px] text-amber-700 dark:text-amber-400 uppercase tracking-wide">
            {t('aiAgent.argumentsLabel')}
          </span>
          <pre className="mt-1 text-[11px] font-mono bg-amber-100/60 dark:bg-amber-950/60 p-2 rounded overflow-x-auto whitespace-pre text-amber-900 dark:text-amber-200">
            {JSON.stringify(approval.arguments, null, 2)}
          </pre>
        </div>
      </div>

      <div className="flex items-center gap-2 justify-end">
        <button
          type="button"
          onClick={() => void handleReject()}
          disabled={loading !== null}
          className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg border border-red-300 bg-red-50 text-red-700 text-sm hover:bg-red-100 disabled:opacity-50 disabled:cursor-not-allowed dark:border-red-800 dark:bg-red-950/30 dark:text-red-400 dark:hover:bg-red-950/50"
        >
          <XCircle className="w-3.5 h-3.5" />
          {loading === 'reject' ? t('aiAgent.rejecting') : t('aiAgent.reject')}
        </button>
        <button
          type="button"
          onClick={() => void handleApprove()}
          disabled={loading !== null}
          className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-amber-600 text-white text-sm hover:bg-amber-700 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          <CheckCircle2 className="w-3.5 h-3.5" />
          {loading === 'approve' ? t('aiAgent.approving') : t('aiAgent.approve')}
        </button>
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
  const { t } = useTranslation()
  const [goal, setGoal] = useState(initialGoal)
  const { events, isRunning, error, durationMs, start, stop, runId, pendingApproval, approve, reject, experimental } = useAgentRun()

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
          <h2 className="text-sm font-semibold text-primary">{t('aiAgent.goalTitle')}</h2>
          {experimental && (
            <span className="inline-flex items-center rounded-full bg-amber-500/15 border border-amber-500/30 px-2 py-0.5 text-[10px] font-semibold text-amber-700 dark:text-amber-400">
              {t('aiAgent.betaBadge')}
            </span>
          )}
        </div>
        <textarea
          value={goal}
          onChange={(e) => { setGoal(e.target.value); }}
          placeholder={t('aiAgent.goalPlaceholder')}
          rows={3}
          disabled={isRunning}
          className="w-full rounded-lg border border-border bg-bg p-3 text-sm text-primary placeholder:text-muted focus:outline-none focus:ring-2 focus:ring-brand/40 resize-none disabled:opacity-60"
        />
        <div className="flex items-center justify-between">
          <p className="text-[11px] text-secondary">
            {t('aiAgent.scopesNote')}
          </p>
          {isRunning ? (
            <button
              type="button"
              onClick={stop}
              className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-destructive/10 text-destructive text-sm hover:bg-destructive/20"
            >
              <Square className="w-3.5 h-3.5" />
              {t('aiAgent.stop')}
            </button>
          ) : (
            <button
              type="button"
              onClick={handleStart}
              disabled={!goal.trim()}
              className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-brand text-white text-sm hover:bg-brand/90 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              <Sparkles className="w-3.5 h-3.5" />
              {t('aiAgent.start')}
            </button>
          )}
        </div>
      </div>

      {/* Status */}
      {(isRunning || events.length > 0 || error) && (
        <div className="flex items-center gap-3 text-xs text-secondary">
          <span>
            {isRunning ? t('aiAgent.statusRunning') : t('aiAgent.statusDone')} · {t('aiAgent.eventsCount', { count: events.length })}
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
            <p className="font-medium">{t('aiAgent.errorTitle')}</p>
            <p className="text-xs mt-0.5">{error.message}</p>
          </div>
        </div>
      )}

      {/* Approve Card — oberhalb der Events-Liste wenn eine Freigabe aussteht */}
      {pendingApproval && runId && (
        <ApproveCard
          approval={pendingApproval}
          onApprove={() => approve(runId)}
          onReject={() => reject(runId)}
        />
      )}

      {/* Events */}
      <div className="space-y-2">
        {events.map((evt, i) => (
          <EventCard key={i} evt={evt} index={i} />
        ))}
        {!isRunning && events.length === 0 && !error && (
          <p className="text-xs text-secondary text-center py-6">
            {t('aiAgent.noRunYet')}
          </p>
        )}
      </div>
    </div>
  )
}
