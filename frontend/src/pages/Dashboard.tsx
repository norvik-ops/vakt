import { useState, useRef, useEffect, useCallback } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import {
  Bug, FileCheck, Key, Fish, Eye,
  AlertTriangle, CheckCircle, ShieldAlert, Activity, Flame, ChevronRight, Settings,
  ClipboardList, Clock, TriangleAlert, ListTodo,
  Shield, FileText, User, Database,
  TrendingUp, TrendingDown, Minus, CalendarDays,
  Settings2, GripVertical, Zap,
} from 'lucide-react'
import {
  AreaChart,
  Area,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
} from 'recharts'
import { useQuery } from '@tanstack/react-query'
import { apiFetch } from '../api/client'
import { useFindings } from '../modules/secpulse/hooks/useFindings'
import { useFrameworks } from '../modules/secvitals/hooks/useFrameworks'
import { useRisks } from '../modules/secvitals/hooks/useRisks'
import { useIncidents } from '../modules/secvitals/hooks/useIncidents'
import { useProjects } from '../modules/secvault/hooks/useProjects'
import { useCampaigns } from '../modules/secreflex/hooks/useCampaigns'
import { useBreaches } from '../modules/secprivacy/hooks/useBreaches'
import { useDashboardScore, useDashboardAggregate } from '../hooks/useDashboard'
import type { RiskSummary, ActivityEntry } from '../hooks/useDashboard'
import { useOnboardingStatus } from '../hooks/useOnboarding'
import { OnboardingBanner, OnboardingWizard } from '../components/OnboardingWizard'
import { GettingStartedChecklist } from '../shared/components/GettingStartedChecklist'
import { Skeleton } from '../components/ui/skeleton'
import type { FrameworkScore } from '../hooks/useDashboard'
import { useScoreHistory } from '../modules/secvitals/hooks/useScoreHistory'
import type { ScoreHistoryEntry } from '../modules/secvitals/hooks/useScoreHistory'
import { useNextMilestone } from '../modules/secvitals/hooks/useMilestones'
import { useRecentPages } from '../shared/hooks/useRecentPages'
import { Switch } from '../components/ui/switch'
import { Label } from '../components/ui/label'
import { Button } from '../components/ui/button'
import type { Control, Risk, Incident } from '../modules/secvitals/types'

function fmt(n: number | null | undefined): string {
  return n == null ? '—' : n.toString()
}

function scoreColor(score: number | undefined): string {
  if (score == null) return 'text-secondary'
  if (score >= 70) return 'text-[#22c55e]'
  if (score >= 40) return 'text-[#f59e0b]'
  return 'text-[#ef4444]'
}

/** Color the progress bar based on compliance percentage. */
function barColor(pct: number): string {
  if (pct >= 80) return 'bg-[#22c55e]'
  if (pct >= 50) return 'bg-[#f59e0b]'
  return 'bg-[#ef4444]'
}

/** Color the risk score badge. */
function riskBadgeColor(score: number): string {
  if (score >= 15) return 'bg-[#ef4444]/15 text-[#ef4444]'
  if (score >= 9) return 'bg-[#f59e0b]/15 text-[#f59e0b]'
  return 'bg-[#22c55e]/15 text-[#22c55e]'
}

/** Pick a lucide icon for audit-log resource_type. */
function entityIcon(entityType: string) {
  switch (entityType) {
    case 'control': return Shield
    case 'policy': return FileText
    case 'risk': return TriangleAlert
    case 'incident': return Flame
    case 'vvt':
    case 'dpia':
    case 'avv': return Eye
    case 'breach': return Flame
    case 'dsr': return User
    case 'audit': return ClipboardList
    default: return Database
  }
}

/** Translate entity_type to German label for display. */
function entityLabel(entityType: string): string {
  const map: Record<string, string> = {
    control: 'Control',
    policy: 'Richtlinie',
    risk: 'Risiko',
    incident: 'Vorfall',
    vvt: 'VVT',
    dpia: 'DPIA',
    avv: 'AVV',
    breach: 'Datenpanne',
    dsr: 'Betroffenenanfrage',
    audit: 'Audit',
  }
  return map[entityType] ?? entityType
}

/** Returns a German relative-time string without external deps. */
function relativeTime(isoString: string): string {
  const diff = Math.floor((Date.now() - new Date(isoString).getTime()) / 1000)
  if (diff < 60) return 'gerade eben'
  if (diff < 3600) return `vor ${Math.floor(diff / 60)} Min.`
  if (diff < 86400) return `vor ${Math.floor(diff / 3600)} Std.`
  if (diff < 2592000) return `vor ${Math.floor(diff / 86400)} Tagen`
  return `vor ${Math.floor(diff / 2592000)} Monaten`
}

/** Translate action to German. */
function actionLabel(action: string): string {
  const map: Record<string, string> = {
    create: 'erstellt',
    update: 'aktualisiert',
    delete: 'gelöscht',
    approve: 'genehmigt',
    export: 'exportiert',
    review: 'überprüft',
  }
  return map[action] ?? action
}

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------

function KPICard({
  label,
  value,
  icon: Icon,
  to,
  critical,
  isLoading,
}: {
  label: string
  value: number | undefined
  icon: React.ElementType
  to: string
  critical?: boolean
  isLoading?: boolean
}) {
  const isAlert = critical && (value ?? 0) > 0
  return (
    <Link
      to={to}
      className="flex flex-col gap-1 rounded-lg border border-border bg-surface p-4 hover:border-brand/60 transition-colors"
      aria-label={`${label}: ${value ?? 'wird geladen'}`}
    >
      <div className="flex items-center gap-2 mb-1">
        {/* WCAG 1.1.1: icon is decorative, label text conveys meaning */}
        <Icon className={`w-4 h-4 ${isAlert ? 'text-[#ef4444]' : 'text-secondary'}`} aria-hidden="true" />
        <span className="text-[11px] text-secondary uppercase tracking-wider font-semibold">{label}</span>
      </div>
      {isLoading ? (
        <Skeleton className="h-8 w-16 mt-1" aria-label={`${label} wird geladen`} />
      ) : (
        <p className={`text-[32px] font-black leading-none ${isAlert ? 'text-[#ef4444]' : 'text-primary'}`} aria-hidden="true">
          {value ?? '—'}
        </p>
      )}
    </Link>
  )
}

function FrameworkProgress({
  scores,
}: {
  scores: Array<{
    framework_id: string
    framework_name: string
    total_controls: number
    implemented_controls: number
    score_pct: number
  }>
}) {
  if (scores.length === 0) {
    return <p className="text-[12px] text-secondary">Keine Frameworks konfiguriert.</p>
  }
  return (
    <div className="space-y-3">
      {scores.map((fw) => {
        const pct = Math.round(fw.score_pct)
        const color = barColor(pct)
        const progressId = `fw-progress-${fw.framework_id}`
        return (
          <div key={fw.framework_id}>
            <div className="flex items-center justify-between mb-1">
              <span className="text-[12px] font-medium text-primary truncate max-w-[60%]" id={progressId}>
                {fw.framework_name}
              </span>
              <span className="text-[12px] text-secondary shrink-0 ml-2">
                {fw.implemented_controls} / {fw.total_controls} · {pct}%
              </span>
            </div>
            {/* WCAG 1.3.1: progressbar role with aria-valuenow communicates progress to screen readers */}
            <div
              className="h-1.5 rounded-full bg-border overflow-hidden"
              role="progressbar"
              aria-valuenow={pct}
              aria-valuemin={0}
              aria-valuemax={100}
              aria-labelledby={progressId}
              aria-label={`${fw.framework_name}: ${pct}% umgesetzt`}
            >
              <div
                className={`h-full rounded-full transition-all ${color}`}
                style={{ width: `${pct}%` }}
              />
            </div>
          </div>
        )
      })}
    </div>
  )
}

const RISK_STATUS_LABELS: Record<string, string> = {
  open: 'Offen',
  in_review: 'In Prüfung',
  accepted: 'Akzeptiert',
  closed: 'Geschlossen',
  mitigated: 'Gemindert',
}

function TopRisksList({ risks }: { risks: RiskSummary[] }) {
  if (risks.length === 0) {
    return <p className="text-[12px] text-secondary">Keine Risiken erfasst.</p>
  }
  return (
    <ol className="space-y-2">
      {risks.map((r, i) => (
        <li key={r.id} className="flex items-center gap-2">
          <span className="text-[11px] font-bold text-secondary w-4 shrink-0">#{i + 1}</span>
          <span className="text-[12px] text-primary flex-1 truncate">{r.title}</span>
          <span className={`text-[11px] font-bold px-1.5 py-0.5 rounded ${riskBadgeColor(r.score)}`}>
            {r.score}
          </span>
          <span className="text-[10px] text-secondary shrink-0">{RISK_STATUS_LABELS[r.status] ?? r.status}</span>
        </li>
      ))}
    </ol>
  )
}

function ActivityTimeline({ entries }: { entries: ActivityEntry[] }) {
  if (entries.length === 0) {
    return <p className="text-[12px] text-secondary">Keine Aktivitäten vorhanden.</p>
  }
  return (
    <ol className="space-y-2">
      {entries.map((e) => {
        const Icon = entityIcon(e.entity_type)
        const relTime = relativeTime(e.created_at)
        return (
          <li key={e.id} className="flex items-start gap-2.5">
            <span className="mt-0.5 p-1 rounded bg-border/60 shrink-0" aria-hidden="true">
              <Icon className="w-3 h-3 text-secondary" />
            </span>
            <div className="flex-1 min-w-0">
              <p className="text-[12px] text-primary leading-snug">
                <span className="font-medium">{entityLabel(e.entity_type)}</span>
                {' '}
                <span className="text-secondary">{actionLabel(e.action)}</span>
              </p>
              <p className="text-[10px] text-secondary truncate">
                {e.user_email || 'System'} · {relTime}
              </p>
            </div>
          </li>
        )
      })}
    </ol>
  )
}

// ---------------------------------------------------------------------------
// Compliance progress card — aggregates all framework controls
// ---------------------------------------------------------------------------

function ComplianceProgressCard({
  scores,
  isLoading,
}: {
  scores: FrameworkScore[]
  isLoading?: boolean
}) {
  const totals = scores.reduce(
    (acc, fw) => {
      acc.total += fw.total_controls
      acc.implemented += fw.implemented_controls
      return acc
    },
    { total: 0, implemented: 0 },
  )
  const pct = totals.total > 0 ? Math.round((totals.implemented / totals.total) * 100) : 0
  const color = barColor(pct)

  return (
    <section className="rounded-lg border border-border bg-surface p-4">
      <div className="flex items-center justify-between mb-3">
        <h2 className="text-[13px] font-semibold text-primary">Compliance-Fortschritt</h2>
        {!isLoading && totals.total > 0 && (
          <span className="text-[11px] text-secondary">{pct}%</span>
        )}
      </div>

      {isLoading ? (
        <div className="space-y-2">
          <Skeleton className="h-4 w-40" />
          <Skeleton className="h-2 w-full" />
        </div>
      ) : totals.total === 0 ? (
        <p className="text-[12px] text-secondary">Keine Frameworks konfiguriert.</p>
      ) : (
        <>
          <div className="flex items-end justify-between mb-1.5">
            <span className="text-[12px] text-primary">
              <span className="font-semibold">{totals.implemented}</span>
              <span className="text-secondary"> von {totals.total} Controls umgesetzt</span>
            </span>
            <span className={`text-[11px] font-medium ${pct >= 80 ? 'text-[#22c55e]' : pct >= 50 ? 'text-[#f59e0b]' : 'text-[#ef4444]'}`}>
              {totals.total - totals.implemented} offen
            </span>
          </div>
          {/* WCAG 1.3.1: progressbar communicates overall compliance progress */}
          <div
            className="h-2 rounded-full bg-border overflow-hidden"
            role="progressbar"
            aria-valuenow={pct}
            aria-valuemin={0}
            aria-valuemax={100}
            aria-label={`Gesamt-Compliance: ${pct}%`}
          >
            <div
              className={`h-full rounded-full transition-all duration-500 ${color}`}
              style={{ width: `${pct}%` }}
            />
          </div>
        </>
      )}
    </section>
  )
}

// ---------------------------------------------------------------------------
// Linear forecast helper
// ---------------------------------------------------------------------------

function linearForecast(points: { x: number; y: number }[], futureX: number): number {
  const n = points.length
  const sumX = points.reduce((a, p) => a + p.x, 0)
  const sumY = points.reduce((a, p) => a + p.y, 0)
  const sumXY = points.reduce((a, p) => a + p.x * p.y, 0)
  const sumXX = points.reduce((a, p) => a + p.x * p.x, 0)
  const slope = (n * sumXY - sumX * sumY) / (n * sumXX - sumX * sumX)
  const intercept = (sumY - slope * sumX) / n
  return slope * futureX + intercept
}

/** Build a short forecast hint line from the last up-to-4 score history entries. */
function ScoreForecastHint({ entries }: { entries: ScoreHistoryEntry[] }) {
  // Use last 4 data points
  const sample = entries.slice(-4)
  if (sample.length < 2) return null

  const points = sample.map((e, i) => ({ x: i, y: e.score }))
  const lastX = points[points.length - 1].x

  // Project 6 weeks: each data point is roughly 1 week apart in typical usage
  const futureX = lastX + 6
  const forecast = linearForecast(points, futureX)
  const slope = points[points.length - 1].y - points[0].y

  const currentScore = sample[sample.length - 1].score
  const forecastClamped = Math.min(100, Math.max(0, Math.round(forecast)))

  if (Math.abs(slope) < 0.5) {
    return (
      <p className="text-[11px] text-secondary mt-2">
        Trend stagniert — keine signifikante Veränderung der letzten Messpunkte.
      </p>
    )
  }

  if (slope > 0) {
    return (
      <p className="text-[11px] text-secondary mt-2">
        Bei aktuellem Tempo erreichst du voraussichtlich{' '}
        <span className="font-semibold text-[#22c55e]">{forecastClamped}%</span> in ~6 Wochen
        {forecastClamped <= currentScore ? ' (Score bereits stabil)' : ''}.
      </p>
    )
  }

  return (
    <p className="text-[11px] text-secondary mt-2">
      Abwärtstrend — ohne Maßnahmen könnte der Score auf{' '}
      <span className="font-semibold text-[#ef4444]">{forecastClamped}%</span> in ~6 Wochen fallen.
    </p>
  )
}

// ---------------------------------------------------------------------------
// Compliance trend chart
// ---------------------------------------------------------------------------

/** Format "YYYY-MM-DD" → "DD.MM." for the X-axis label. */
function fmtAxisDate(iso: string): string {
  const parts = iso.split('-')
  if (parts.length !== 3) return iso
  return `${parts[2]}.${parts[1]}.`
}

interface ChartTooltipProps {
  active?: boolean
  payload?: Array<{ value: number; payload: ScoreHistoryEntry }>
  label?: string
}

function ScoreChartTooltip({ active, payload }: ChartTooltipProps) {
  if (!active || !payload?.length) return null
  const d = payload[0].payload
  return (
    <div className="rounded-md border border-border bg-surface px-3 py-2 shadow-lg text-[12px]">
      <p className="font-semibold text-primary mb-1">{fmtAxisDate(d.date)}</p>
      <p className="text-secondary">Score: <span className="font-bold text-primary">{d.score.toFixed(1)}%</span></p>
      <p className="text-secondary">Controls: <span className="font-bold text-primary">{d.controls_implemented} / {d.controls_total}</span></p>
    </div>
  )
}

function ScoreHistoryCard() {
  const [days, setDays] = useState<30 | 90>(30)
  const { data: entries, isLoading } = useScoreHistory(days)

  // Determine trend: compare latest score to oldest score in the window.
  let trendDelta: number | null = null
  if (entries && entries.length >= 2) {
    trendDelta = entries[entries.length - 1].score - entries[0].score
  }

  const chartData = entries?.map((e) => ({
    ...e,
    label: fmtAxisDate(e.date),
  })) ?? []

  return (
    <section className="rounded-lg border border-border bg-surface p-4">
      <div className="flex items-center justify-between mb-3">
        <h2 className="text-[13px] font-semibold text-primary">Compliance-Verlauf</h2>
        <div className="flex items-center gap-2">
          {trendDelta !== null && (
            <span
              className={`flex items-center gap-0.5 text-[11px] font-semibold ${trendDelta > 0.5 ? 'text-[#22c55e]' : trendDelta < -0.5 ? 'text-[#ef4444]' : 'text-secondary'}`}
              aria-label={`Trend: ${trendDelta > 0 ? '+' : ''}${trendDelta.toFixed(1)}%`}
            >
              {trendDelta > 0.5 ? (
                <TrendingUp className="w-3 h-3" aria-hidden="true" />
              ) : trendDelta < -0.5 ? (
                <TrendingDown className="w-3 h-3" aria-hidden="true" />
              ) : (
                <Minus className="w-3 h-3" aria-hidden="true" />
              )}
              {trendDelta > 0 ? '+' : ''}{trendDelta.toFixed(1)}%
            </span>
          )}
          <div className="flex rounded-md border border-border overflow-hidden text-[11px]">
            <button
              className={`px-2 py-1 transition-colors ${days === 30 ? 'bg-brand text-white' : 'bg-surface text-secondary hover:bg-border/50'}`}
              onClick={() => setDays(30)}
              aria-label="Verlauf 30 Tage anzeigen"
              aria-pressed={days === 30}
            >
              30 Tage
            </button>
            <button
              className={`px-2 py-1 transition-colors ${days === 90 ? 'bg-brand text-white' : 'bg-surface text-secondary hover:bg-border/50'}`}
              onClick={() => setDays(90)}
              aria-label="Verlauf 90 Tage anzeigen"
              aria-pressed={days === 90}
            >
              90 Tage
            </button>
          </div>
        </div>
      </div>

      {isLoading ? (
        <div className="space-y-2">
          <Skeleton className="h-[160px] w-full" />
        </div>
      ) : chartData.length === 0 ? (
        <div className="flex items-center justify-center h-[160px]">
          <p className="text-[12px] text-secondary">Verlaufsdaten werden ab morgen gesammelt</p>
        </div>
      ) : (
        <ResponsiveContainer width="100%" height={160}>
          <AreaChart data={chartData} margin={{ top: 4, right: 4, bottom: 0, left: -24 }}>
            <defs>
              <linearGradient id="scoreGrad" x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%" stopColor="#22c55e" stopOpacity={0.25} />
                <stop offset="95%" stopColor="#22c55e" stopOpacity={0} />
              </linearGradient>
            </defs>
            <XAxis
              dataKey="label"
              tick={{ fontSize: 10, fill: 'var(--color-secondary, #94a3b8)' }}
              axisLine={false}
              tickLine={false}
              interval="preserveStartEnd"
            />
            <YAxis
              domain={[0, 100]}
              tick={{ fontSize: 10, fill: 'var(--color-secondary, #94a3b8)' }}
              axisLine={false}
              tickLine={false}
              tickFormatter={(v: number) => `${v}%`}
            />
            <Tooltip content={<ScoreChartTooltip />} />
            <Area
              type="monotone"
              dataKey="score"
              stroke="#22c55e"
              strokeWidth={2}
              fill="url(#scoreGrad)"
              dot={false}
              activeDot={{ r: 4, fill: '#22c55e', strokeWidth: 0 }}
            />
          </AreaChart>
        </ResponsiveContainer>
      )}
    </section>
  )
}

// ---------------------------------------------------------------------------
// Today widget
// ---------------------------------------------------------------------------

function TodayWidget() {
  const navigate = useNavigate()
  const today = new Date().toISOString().slice(0, 10)

  const { data: risks } = useRisks(1, 100)
  const { data: incidents } = useIncidents(1, 100)

  const todayRisks = (risks ?? []).filter((r: Risk) => {
    if (r.status !== 'open') return false
    const due = r.treatment_due_date
    if (!due) return false
    return due.slice(0, 10) <= today
  })

  const todayIncidents = (incidents ?? []).filter((i: Incident) => {
    if (i.status === 'resolved' || i.status === 'closed') return false
    const created = i.created_at.slice(0, 10)
    return created === today || i.status === 'open'
  })

  const total = todayRisks.length + todayIncidents.length

  if (total === 0) {
    return (
      <section className="rounded-lg border border-border bg-surface p-4">
        <div className="flex items-center gap-2 mb-3">
          <ListTodo className="w-4 h-4 text-secondary" aria-hidden="true" />
          <h2 className="text-[13px] font-semibold text-primary">Heute zu tun</h2>
        </div>
        <p className="text-[12px] text-secondary">Nichts Dringendes heute 🎉</p>
      </section>
    )
  }

  return (
    <section className="rounded-lg border border-border bg-surface p-4">
      <div className="flex items-center gap-2 mb-3">
        <ListTodo className="w-4 h-4 text-brand" aria-hidden="true" />
        <h2 className="text-[13px] font-semibold text-primary">Heute zu tun</h2>
        <span className="ml-auto text-[11px] font-bold text-brand">{total}</span>
      </div>
      <div className="space-y-3">
        {todayRisks.length > 0 && (
          <div>
            <p className="text-[10px] font-semibold text-secondary uppercase tracking-wider mb-1.5">
              Risiken fällig
            </p>
            <ol className="space-y-1.5">
              {todayRisks.slice(0, 5).map((r) => {
                const isOverdue = (r.treatment_due_date ?? '') < today
                return (
                  <li key={r.id}>
                    <button
                      className="w-full flex items-center gap-2 text-left rounded-md px-2 py-1.5 hover:bg-border/50 transition-colors group"
                      onClick={() => { navigate(`/secvitals/risks/${r.id}`) }}
                    >
                      <TriangleAlert className={`w-3.5 h-3.5 shrink-0 ${isOverdue ? 'text-[#ef4444]' : 'text-[#f59e0b]'}`} aria-hidden="true" />
                      <span className="text-[12px] text-primary flex-1 truncate group-hover:text-brand">{r.title}</span>
                      <span className={`text-[10px] shrink-0 ${isOverdue ? 'text-[#ef4444]' : 'text-secondary'}`}>
                        {isOverdue ? 'Überfällig' : 'Heute fällig'}
                      </span>
                    </button>
                  </li>
                )
              })}
            </ol>
          </div>
        )}
        {todayIncidents.length > 0 && (
          <div>
            <p className="text-[10px] font-semibold text-secondary uppercase tracking-wider mb-1.5">
              Vorfälle offen
            </p>
            <ol className="space-y-1.5">
              {todayIncidents.slice(0, 5).map((i) => (
                <li key={i.id}>
                  <button
                    className="w-full flex items-center gap-2 text-left rounded-md px-2 py-1.5 hover:bg-border/50 transition-colors group"
                    onClick={() => { navigate(`/secvitals/incidents/${i.id}`) }}
                  >
                    <Flame className="w-3.5 h-3.5 shrink-0 text-[#ef4444]" aria-hidden="true" />
                    <span className="text-[12px] text-primary flex-1 truncate group-hover:text-brand">{i.title}</span>
                    <span className="text-[10px] text-secondary shrink-0">{i.severity}</span>
                  </button>
                </li>
              ))}
            </ol>
          </div>
        )}
      </div>
    </section>
  )
}

// ---------------------------------------------------------------------------
// Dashboard widget order — drag & drop persistence
// ---------------------------------------------------------------------------

const DEFAULT_WIDGET_ORDER = ['today', 'score_history', 'quick_wins', 'compliance_progress', 'frameworks', 'risks', 'activity', 'modules']

function useDashboardOrder(defaultOrder: string[]) {
  const [order, setOrder] = useState<string[]>(() => {
    try {
      const saved = JSON.parse(localStorage.getItem('vakt_dashboard_order') ?? '[]') as string[]
      // Merge: keep saved order but add any new widgets from defaultOrder
      const merged = saved.filter((id) => defaultOrder.includes(id))
      const added = defaultOrder.filter((id) => !merged.includes(id))
      const result = [...merged, ...added]
      return result.length > 0 ? result : defaultOrder
    } catch {
      return defaultOrder
    }
  })
  const saveOrder = (newOrder: string[]) => {
    setOrder(newOrder)
    localStorage.setItem('vakt_dashboard_order', JSON.stringify(newOrder))
  }
  return { order, saveOrder }
}

// ---------------------------------------------------------------------------
// Quick wins card
// ---------------------------------------------------------------------------

function useQuickWinsControls() {
  return useQuery<Control[]>({
    queryKey: ['secvitals', 'controls', 'quick-wins'],
    queryFn: () => apiFetch<Control[]>('/secvitals/controls?status=missing&limit=20'),
    staleTime: 5 * 60 * 1000,
  })
}

interface QuickWin {
  control: Control
  hint: string
}

function QuickWinsCard() {
  const navigate = useNavigate()
  const { data: controls } = useQuickWinsControls()

  const quickWins: QuickWin[] = (controls ?? [])
    .filter((c) => c.status === 'missing')
    .slice(0, 5)
    .map((c) => {
      const staleDays = c.last_reviewed_at
        ? Math.floor((Date.now() - new Date(c.last_reviewed_at).getTime()) / 86_400_000)
        : null
      if (staleDays !== null && staleDays > 30) {
        return { control: c, hint: `Seit ${staleDays.toString()} Tagen nicht überprüft` }
      }
      return { control: c, hint: 'Noch nicht gestartet — schnell umsetzbar' }
    })

  if (quickWins.length === 0) return null

  return (
    <div className="bg-surface border border-border rounded-xl p-5">
      <div className="flex items-center gap-2 mb-4">
        <Zap className="w-4 h-4 text-amber-500" aria-hidden="true" />
        <h2 className="text-sm font-semibold text-primary">Quick Wins ({quickWins.length})</h2>
        <span className="text-xs text-secondary">— kleine Maßnahmen, große Wirkung</span>
      </div>
      <div className="space-y-2">
        {quickWins.map(({ control, hint }) => (
          <div key={control.id} className="flex items-center justify-between gap-3 text-sm">
            <span className="text-primary font-medium truncate min-w-0">
              {control.control_id} {control.title}
            </span>
            <span className="text-xs text-secondary shrink-0 hidden sm:block">{hint}</span>
            <Button
              size="sm"
              variant="outline"
              className="h-7 text-xs shrink-0"
              onClick={() => { navigate(`/secvitals/controls/${control.id}`) }}
            >
              Öffnen
            </Button>
          </div>
        ))}
      </div>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Widget visibility config
// ---------------------------------------------------------------------------

const WIDGETS_KEY = 'vakt_dashboard_widgets'

type WidgetKey = 'compliance_score' | 'open_findings' | 'incidents' | 'recent_pages' | 'onboarding' | 'evidence_expiry'

const DEFAULT_WIDGETS: Record<WidgetKey, boolean> = {
  compliance_score: true,
  open_findings: true,
  incidents: true,
  recent_pages: true,
  onboarding: true,
  evidence_expiry: true,
}

const WIDGET_LABELS: Record<WidgetKey, string> = {
  compliance_score: 'Compliance-Score',
  open_findings: 'Offene Findings',
  incidents: 'Incidents',
  recent_pages: 'Zuletzt besucht',
  onboarding: 'Onboarding-Checkliste',
  evidence_expiry: 'Evidence-Ablauf',
}

function loadWidgets(): Record<WidgetKey, boolean> {
  try {
    const saved = JSON.parse(localStorage.getItem(WIDGETS_KEY) ?? '{}') as Partial<Record<WidgetKey, boolean>>
    return { ...DEFAULT_WIDGETS, ...saved }
  } catch {
    return DEFAULT_WIDGETS
  }
}

// ---------------------------------------------------------------------------
// Main dashboard
// ---------------------------------------------------------------------------

export default function Dashboard() {
  const { data: onboarding } = useOnboardingStatus()
  const [wizardOpen, setWizardOpen] = useState(false)

  const { data: scoreData, isLoading: scoreLoading, isError: scoreError } = useDashboardScore()
  const { data: agg, isLoading: aggLoading, isError: aggError } = useDashboardAggregate()
  const { data: scoreHistory } = useScoreHistory(30)
  const { data: critFindings, isLoading: findingsLoading } = useFindings({ severity: 'critical' })
  const { data: frameworks, isLoading: fwLoading } = useFrameworks()
  const { data: projects, isLoading: projLoading } = useProjects()
  const { data: campaigns, isLoading: campLoading } = useCampaigns()
  const { data: breaches, isLoading: breachLoading } = useBreaches()
  const { data: nextMilestone } = useNextMilestone()
  const recentPages = useRecentPages()
  const [widgets, setWidgets] = useState<Record<WidgetKey, boolean>>(() => loadWidgets())
  const [widgetMenuOpen, setWidgetMenuOpen] = useState(false)
  const widgetMenuRef = useRef<HTMLDivElement>(null)

  // Drag & drop widget ordering
  const { order: widgetOrder, saveOrder } = useDashboardOrder(DEFAULT_WIDGET_ORDER)
  const [editMode, setEditMode] = useState(false)
  const dragItem = useRef<string | null>(null)
  const dragOverItem = useRef<string | null>(null)

  const handleDragStart = useCallback((widgetId: string) => {
    dragItem.current = widgetId
  }, [])

  const handleDragOver = useCallback((e: React.DragEvent, widgetId: string) => {
    e.preventDefault()
    dragOverItem.current = widgetId
  }, [])

  const handleDrop = useCallback(() => {
    if (!dragItem.current || !dragOverItem.current || dragItem.current === dragOverItem.current) return
    const newOrder = [...widgetOrder]
    const fromIdx = newOrder.indexOf(dragItem.current)
    const toIdx = newOrder.indexOf(dragOverItem.current)
    if (fromIdx === -1 || toIdx === -1) return
    newOrder.splice(fromIdx, 1)
    newOrder.splice(toIdx, 0, dragItem.current)
    saveOrder(newOrder)
    dragItem.current = null
    dragOverItem.current = null
  }, [widgetOrder, saveOrder])

  // Close widget menu on outside click
  useEffect(() => {
    if (!widgetMenuOpen) return
    function handler(e: MouseEvent) {
      if (widgetMenuRef.current && !widgetMenuRef.current.contains(e.target as Node)) {
        setWidgetMenuOpen(false)
      }
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [widgetMenuOpen])

  function toggleWidget(key: WidgetKey) {
    setWidgets((prev) => {
      const next = { ...prev, [key]: !prev[key] }
      try { localStorage.setItem(WIDGETS_KEY, JSON.stringify(next)) } catch {}
      return next
    })
  }

  const kpiLoading = aggLoading

  // Compute 30-day trend delta from score history.
  const scoreTrend: number | null = (() => {
    if (!scoreHistory || scoreHistory.length < 2) return null
    return scoreHistory[scoreHistory.length - 1].score - scoreHistory[0].score
  })()

  const critCount = critFindings?.pagination?.total ?? null
  const fwCount = frameworks?.length ?? null
  const projCount = projects?.length ?? null
  const activeCampaignCount =
    campaigns?.filter((c) => c.status === 'running' || c.status === 'scheduled').length ?? null
  const openBreachCount = breaches?.filter((b) => b.status === 'open').length ?? null

  const STATS = [
    {
      label: 'Kritische Findings',
      value: fmt(critCount),
      icon: AlertTriangle,
      color: critCount ? 'text-[#ef4444]' : 'text-secondary',
      path: '/secpulse/findings?severity=critical',
      loading: findingsLoading,
    },
    {
      label: 'Frameworks aktiv',
      value: fmt(fwCount),
      icon: CheckCircle,
      color: fwCount ? 'text-[#22c55e]' : 'text-secondary',
      path: '/secvitals',
      loading: fwLoading,
    },
    {
      label: 'Vault-Projekte',
      value: fmt(projCount),
      icon: ShieldAlert,
      color: 'text-[#f59e0b]',
      path: '/secvault',
      loading: projLoading,
    },
    {
      label: 'Aktive Kampagnen',
      value: fmt(activeCampaignCount),
      icon: Activity,
      color: activeCampaignCount ? 'text-[#818cf8]' : 'text-secondary',
      path: '/secreflex',
      loading: campLoading,
    },
    {
      label: 'Offene Datenpannen',
      value: fmt(openBreachCount),
      icon: Flame,
      color: openBreachCount ? 'text-[#ef4444]' : 'text-secondary',
      path: '/secprivacy?filter=breach&status=open',
      loading: breachLoading,
    },
  ]

  const MODULES = [
    {
      label: 'Vakt Scan',
      description: 'Scanner-Orchestrierung & Vulnerability Management',
      icon: Bug,
      iconColor: 'text-[#ef4444]',
      badge: critCount != null ? `${critCount} kritisch` : '—',
      badgeColor: critCount ? 'text-[#ef4444]' : 'text-secondary',
      path: '/secpulse',
    },
    {
      label: 'Vakt Comply',
      description: 'Compliance & Dokumentation — NIS2, ISO 27001, BSI',
      icon: FileCheck,
      iconColor: 'text-[#22c55e]',
      badge: fwCount != null ? `${fwCount} Framework${fwCount === 1 ? '' : 's'}` : '—',
      badgeColor: fwCount ? 'text-[#22c55e]' : 'text-secondary',
      path: '/secvitals',
    },
    {
      label: 'Vakt Vault',
      description: 'Secrets Management, Rotation & Git-Scanning',
      icon: Key,
      iconColor: 'text-[#f59e0b]',
      badge: projCount != null ? `${projCount} Projekt${projCount === 1 ? '' : 'e'}` : '—',
      badgeColor: 'text-secondary',
      path: '/secvault',
    },
    {
      label: 'Vakt Aware',
      description: 'Phishing-Simulation & Security Awareness',
      icon: Fish,
      iconColor: 'text-[#818cf8]',
      badge: activeCampaignCount != null ? `${activeCampaignCount} aktiv` : '—',
      badgeColor: activeCampaignCount ? 'text-[#818cf8]' : 'text-secondary',
      path: '/secreflex',
    },
    {
      label: 'Vakt Privacy',
      description: 'DSGVO-Dokumentation — VVT, DPIA, AVV, Meldepflichten',
      icon: Eye,
      iconColor: 'text-[#06b6d4]',
      badge: openBreachCount != null ? `${openBreachCount} offen` : '—',
      badgeColor: openBreachCount ? 'text-[#ef4444]' : 'text-secondary',
      path: '/secprivacy',
    },
  ]

  return (
    <div className="flex flex-col lg:flex-row h-full">
      {/* Left panel — score + stats */}
      <div className="w-full lg:w-[260px] lg:shrink-0 border-b lg:border-b-0 lg:border-r border-border bg-surface flex flex-col">
        <div className="flex-1 p-6 overflow-auto">
          <div className="flex items-center justify-between mb-6">
            <h1 className="text-[20px] font-bold text-primary">Dashboard</h1>
            {/* Widget Toggle */}
            <div className="flex items-center gap-1">
              <button
                onClick={() => { setEditMode((v) => !v) }}
                aria-label={editMode ? 'Bearbeitung beenden' : 'Widgets sortieren'}
                title={editMode ? 'Bearbeitung beenden' : 'Widgets sortieren'}
                className={`p-1.5 rounded-md transition-colors ${editMode ? 'text-brand bg-brand/10' : 'text-secondary hover:text-primary hover:bg-muted/50'}`}
              >
                <GripVertical className="w-4 h-4" aria-hidden="true" />
              </button>
              <div className="relative" ref={widgetMenuRef}>
              <button
                onClick={() => setWidgetMenuOpen((o) => !o)}
                aria-label="Widgets konfigurieren"
                title="Widgets konfigurieren"
                className="p-1.5 rounded-md text-secondary hover:text-primary hover:bg-muted/50 transition-colors"
              >
                <Settings2 className="w-4 h-4" aria-hidden="true" />
              </button>
              {widgetMenuOpen && (
                <div className="absolute right-0 top-8 z-20 w-56 rounded-lg border border-border bg-surface shadow-xl p-3">
                  <p className="text-[10px] font-semibold text-secondary uppercase tracking-wider mb-2">Widgets</p>
                  <div className="space-y-2">
                    {(Object.keys(WIDGET_LABELS) as WidgetKey[]).map((key) => (
                      <div key={key} className="flex items-center justify-between gap-2">
                        <Label htmlFor={`widget-${key}`} className="text-[12px] text-primary cursor-pointer flex-1">
                          {WIDGET_LABELS[key]}
                        </Label>
                        <Switch
                          id={`widget-${key}`}
                          checked={widgets[key]}
                          onCheckedChange={() => toggleWidget(key)}
                        />
                      </div>
                    ))}
                  </div>
                </div>
              )}
              </div>
            </div>
          </div>

          <p className="text-[10px] font-semibold text-secondary uppercase tracking-wider mb-1 opacity-60">
            Security Score
          </p>
          <div className="flex items-end gap-1">
            {scoreLoading ? (
              <Skeleton className="h-12 w-20" />
            ) : (
              <p className={`text-[52px] font-black leading-none ${scoreColor(scoreData?.score)}`}>
                {scoreData?.score ?? '—'}
              </p>
            )}
            <p className="text-[16px] text-secondary mb-2">/ 100</p>
          </div>
          <div className="flex items-center gap-2 mt-1">
            <p className="text-[12px] text-secondary">Gesamtbewertung</p>
            {scoreTrend !== null && (
              <span
                className={`flex items-center gap-0.5 text-[11px] font-semibold ${scoreTrend > 0.5 ? 'text-[#22c55e]' : scoreTrend < -0.5 ? 'text-[#ef4444]' : 'text-secondary'}`}
                aria-label={`Trend: ${scoreTrend > 0 ? '+' : ''}${scoreTrend.toFixed(1)}%`}
              >
                {scoreTrend > 0.5 ? (
                  <TrendingUp className="w-3 h-3" aria-hidden="true" />
                ) : scoreTrend < -0.5 ? (
                  <TrendingDown className="w-3 h-3" aria-hidden="true" />
                ) : (
                  <Minus className="w-3 h-3" aria-hidden="true" />
                )}
                {scoreTrend > 0 ? '+' : ''}{scoreTrend.toFixed(1)}%
              </span>
            )}
          </div>

          <div className="h-px bg-border my-4" />

          <div className="space-y-1.5">
            {STATS.map(({ label, value, icon: Icon, color, path, loading }) => (
              <Link
                key={label}
                to={path}
                className="flex items-center justify-between px-3 py-2 rounded-md bg-surface border border-border hover:border-brand/60 transition-colors cursor-pointer"
              >
                <div className="flex items-center gap-2">
                  <Icon className={`w-3.5 h-3.5 ${color}`} />
                  <span className="text-[12px] text-primary">{label}</span>
                </div>
                {loading ? (
                  <Skeleton className="h-4 w-8" />
                ) : (
                  <span className={`text-[14px] font-bold ${color}`}>{value}</span>
                )}
              </Link>
            ))}
          </div>

          <div className="h-px bg-border my-4" />

          <p className="text-[10px] font-semibold text-secondary uppercase tracking-wider mb-1 opacity-60">
            Datenpannen
          </p>
          {breachLoading ? (
            <Skeleton className="h-4 w-32" />
          ) : openBreachCount === 0 ? (
            <p className="text-[12px] text-[#22c55e]">Keine offenen Datenpannen</p>
          ) : (
            <p className="text-[12px] text-[#ef4444]">
              {openBreachCount} offene Datenpanne{openBreachCount === 1 ? '' : 'n'}
            </p>
          )}
        </div>
      </div>

      {/* Right panel */}
      <div className="flex-1 overflow-auto p-6 space-y-6">
        {/* Getting Started Checklist */}
        {widgets.evidence_expiry && <GettingStartedChecklist />}

        {/* ── Zuletzt besucht ── */}
        {widgets.recent_pages && recentPages.length > 0 && (
          <section>
            <p className="text-[10px] font-semibold text-secondary uppercase tracking-wider mb-2 opacity-60">
              Zuletzt besucht
            </p>
            <div className="flex flex-wrap gap-2">
              {recentPages.map((page) => (
                <Link
                  key={page.path}
                  to={page.path}
                  className="flex items-center gap-1.5 px-2.5 py-1.5 rounded-md border border-border bg-surface hover:border-brand/60 transition-colors text-[12px] text-secondary hover:text-primary"
                  title={page.label}
                >
                  <span>{page.icon}</span>
                  <span className="font-medium">{page.label}</span>
                  <span className="text-[10px] text-secondary/60 ml-1">{relativeTime(new Date(page.visitedAt).toISOString())}</span>
                </Link>
              ))}
            </div>
          </section>
        )}

        {/* Error banner when main dashboard data fails */}
        {(scoreError || aggError) && (
          <div
            role="alert"
            className="flex items-center gap-2 rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive"
          >
            <AlertTriangle className="w-4 h-4 shrink-0" aria-hidden="true" />
            <span>Dashboard-Daten konnten nicht geladen werden.</span>
          </div>
        )}

        {/* Onboarding (preserved) */}
        {widgets.onboarding && onboarding && !onboarding.completed && !onboarding.dismissed && (
          <OnboardingBanner status={onboarding} onOpen={() => setWizardOpen(true)} />
        )}
        {onboarding && !onboarding.dismissed && (
          <OnboardingWizard
            open={wizardOpen}
            onClose={() => setWizardOpen(false)}
            status={onboarding}
          />
        )}

        {/* ── Compliance KPI cards ── */}
        {widgets.open_findings && <section>
          <h2 className="text-[14px] font-semibold text-primary mb-3">Compliance-Übersicht</h2>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
            <KPICard
              label="Offene CAPAs"
              value={agg?.open_capas}
              icon={ClipboardList}
              to="/secvitals/capas"
              critical
              isLoading={kpiLoading}
            />
            <KPICard
              label="Überfällige Controls"
              value={agg?.overdue_controls}
              icon={Clock}
              to="/secvitals/overdue-reviews"
              critical
              isLoading={kpiLoading}
            />
            <KPICard
              label="Kritische Risiken"
              value={agg?.critical_risks}
              icon={TriangleAlert}
              to="/secvitals/risks"
              critical
              isLoading={kpiLoading}
            />
            <KPICard
              label="Offene Aufgaben"
              value={agg?.overdue_tasks}
              icon={ListTodo}
              to="/secvitals/overdue-reviews"
              critical
              isLoading={kpiLoading}
            />
          </div>
        </section>}

        {/* ── Next audit milestone widget ── */}
        {nextMilestone && (
          <Link
            to="/secvitals/certification-timeline"
            className="flex items-center gap-3 rounded-lg border border-border bg-surface px-4 py-3 hover:border-brand/60 transition-colors"
          >
            <CalendarDays className={`w-5 h-5 shrink-0 ${
              (nextMilestone.days_remaining ?? 999) < 30 ? 'text-red-400' :
              (nextMilestone.days_remaining ?? 999) < 90 ? 'text-amber-400' :
              'text-green-400'
            }`} />
            <div className="flex-1 min-w-0">
              <p className="text-[12px] font-semibold text-primary truncate">
                Nächste Prüfung: {nextMilestone.title}
              </p>
              <p className="text-[11px] text-secondary">
                {nextMilestone.days_remaining === 0
                  ? 'Heute'
                  : nextMilestone.days_remaining != null && nextMilestone.days_remaining > 0
                  ? `in ${nextMilestone.days_remaining} Tagen`
                  : `${Math.abs(nextMilestone.days_remaining ?? 0)} Tage überfällig`}
              </p>
            </div>
            <ChevronRight className="w-4 h-4 text-brand shrink-0" />
          </Link>
        )}

        {/* ── Sortable widget section ── */}
        {editMode && (
          <p className="text-xs text-secondary flex items-center gap-1.5">
            <GripVertical className="w-3.5 h-3.5 text-brand" aria-hidden="true" />
            Widgets ziehen zum Sortieren — Klick auf den Griff-Button oben links zum Beenden
          </p>
        )}

        <div className="space-y-6">
          {widgetOrder.map((widgetId) => {
            const wrapperProps = editMode ? {
              draggable: true as const,
              onDragStart: () => { handleDragStart(widgetId) },
              onDragOver: (e: React.DragEvent) => { handleDragOver(e, widgetId) },
              onDrop: handleDrop,
              className: 'relative group/widget',
            } : { className: '' }

            const dragHandle = editMode ? (
              <div className="absolute -left-5 top-1/2 -translate-y-1/2 opacity-0 group-hover/widget:opacity-100 transition-opacity cursor-grab active:cursor-grabbing"
                aria-hidden="true">
                <GripVertical className="w-4 h-4 text-secondary" />
              </div>
            ) : null

            const widgetOpacity = editMode ? 'transition-opacity' : ''

            switch (widgetId) {
              case 'today':
                return (
                  <div key={widgetId} {...wrapperProps}>
                    {dragHandle}
                    <div className={widgetOpacity}>
                      <TodayWidget />
                    </div>
                  </div>
                )

              case 'score_history':
                return widgets.compliance_score ? (
                  <div key={widgetId} {...wrapperProps}>
                    {dragHandle}
                    <div className={widgetOpacity}>
                      <ComplianceProgressCard scores={agg?.framework_scores ?? []} isLoading={aggLoading} />
                      <div className="mt-4">
                        <ScoreHistoryCard />
                        {scoreHistory && scoreHistory.length >= 2 && (
                          <ScoreForecastHint entries={scoreHistory} />
                        )}
                      </div>
                    </div>
                  </div>
                ) : null

              case 'quick_wins':
                return (
                  <div key={widgetId} {...wrapperProps}>
                    {dragHandle}
                    <div className={widgetOpacity}>
                      <QuickWinsCard />
                    </div>
                  </div>
                )

              case 'compliance_progress':
                return null // merged into score_history above

              case 'frameworks':
                return (
                  <div key={widgetId} {...wrapperProps}>
                    {dragHandle}
                    <div className={widgetOpacity}>
                      <section className="rounded-lg border border-border bg-surface p-4">
                        <div className="flex items-center justify-between mb-3">
                          <h2 className="text-[13px] font-semibold text-primary">Framework-Fortschritt</h2>
                          {agg && (
                            <span className="text-[10px] text-secondary">
                              {agg.policies_approved} / {agg.policies_total} Richtlinien aktiv
                            </span>
                          )}
                        </div>
                        <FrameworkProgress scores={agg?.framework_scores ?? []} />
                      </section>
                    </div>
                  </div>
                )

              case 'risks':
                return (
                  <div key={widgetId} {...wrapperProps}>
                    {dragHandle}
                    <div className={widgetOpacity}>
                      <section className="rounded-lg border border-border bg-surface p-4">
                        <div className="flex items-center justify-between mb-3">
                          <h2 className="text-[13px] font-semibold text-primary">Top-5-Risiken</h2>
                          <Link to="/secvitals/risks" className="text-[10px] text-brand hover:underline">
                            Alle anzeigen
                          </Link>
                        </div>
                        <TopRisksList risks={agg?.top_risks ?? []} />
                      </section>
                    </div>
                  </div>
                )

              case 'activity':
                return (
                  <div key={widgetId} {...wrapperProps}>
                    {dragHandle}
                    <div className={widgetOpacity}>
                      <section className="rounded-lg border border-border bg-surface p-4">
                        <h2 className="text-[13px] font-semibold text-primary mb-3">Letzte Aktivitäten</h2>
                        <ActivityTimeline entries={agg?.recent_activity ?? []} />
                      </section>
                    </div>
                  </div>
                )

              case 'modules':
                return (
                  <div key={widgetId} {...wrapperProps}>
                    {dragHandle}
                    <div className={widgetOpacity}>
                      <section>
                        <div className="flex items-center justify-between mb-4">
                          <h2 className="text-[16px] font-semibold text-primary">Module</h2>
                        </div>
                        <div className="space-y-px">
                          {MODULES.map(({ label, description, icon: Icon, iconColor, badge, badgeColor, path }) => (
                            <Link
                              key={label}
                              to={path}
                              className="flex items-center justify-between py-3 border-b border-border hover:bg-muted/50 -mx-1 px-1 rounded-sm transition-colors group"
                            >
                              <div className="flex items-center gap-3">
                                <Icon className={`w-4 h-4 shrink-0 ${iconColor}`} />
                                <div>
                                  <p className="text-[13px] text-primary font-medium">{label}</p>
                                  <p className="text-[11px] text-secondary mt-0.5">{description}</p>
                                </div>
                              </div>
                              <div className="flex items-center gap-2 ml-4">
                                <span className={`text-[12px] font-medium ${badgeColor}`}>{badge}</span>
                                <ChevronRight className="w-3.5 h-3.5 text-brand opacity-0 group-hover:opacity-100 transition-opacity" />
                              </div>
                            </Link>
                          ))}
                        </div>
                      </section>
                    </div>
                  </div>
                )

              default:
                return null
            }
          })}
        </div>

        {/* ── Settings links (preserved) ── */}
        <section>
          <h2 className="text-[16px] font-semibold text-primary mb-4">Einstellungen</h2>
          <div className="space-y-px">
            <Link
              to="/settings/score-config"
              className="flex items-center justify-between py-3 border-b border-border hover:bg-muted/50 -mx-1 px-1 rounded-sm transition-colors group"
            >
              <div className="flex items-center gap-3">
                <Settings className="w-4 h-4 shrink-0 text-secondary" />
                <div>
                  <p className="text-[13px] text-primary font-medium">Score-Konfiguration</p>
                  <p className="text-[11px] text-secondary mt-0.5">Score-Formel und Gewichtungen anpassen</p>
                </div>
              </div>
              <ChevronRight className="w-3.5 h-3.5 text-brand opacity-0 group-hover:opacity-100 transition-opacity ml-4" />
            </Link>
            <Link
              to="/settings/alerting"
              className="flex items-center justify-between py-3 border-b border-border hover:bg-muted/50 -mx-1 px-1 rounded-sm transition-colors group"
            >
              <div className="flex items-center gap-3">
                <Settings className="w-4 h-4 shrink-0 text-secondary" />
                <div>
                  <p className="text-[13px] text-primary font-medium">Alerting</p>
                  <p className="text-[11px] text-secondary mt-0.5">Benachrichtigungskanäle verwalten</p>
                </div>
              </div>
              <ChevronRight className="w-3.5 h-3.5 text-brand opacity-0 group-hover:opacity-100 transition-opacity ml-4" />
            </Link>
          </div>
        </section>
      </div>
    </div>
  )
}
