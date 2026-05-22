import { Link } from 'react-router-dom'
import {
  Shield, FileText, TriangleAlert, Flame, Eye, User, ClipboardList, Database,
} from 'lucide-react'
import { Skeleton } from '../components/ui/skeleton'
import type { RiskSummary, ActivityEntry, FrameworkScore } from '../hooks/useDashboard'

export function fmt(n: number | null | undefined): string {
  return n == null ? '—' : n.toString()
}

export function scoreColor(score: number | undefined): string {
  if (score == null) return 'text-secondary'
  if (score >= 70) return 'text-severity-low'
  if (score >= 40) return 'text-severity-medium'
  return 'text-severity-critical'
}

export function barColor(pct: number): string {
  if (pct >= 80) return 'bg-severity-low'
  if (pct >= 50) return 'bg-severity-medium'
  return 'bg-severity-critical'
}

export function riskBadgeColor(score: number): string {
  if (score >= 15) return 'bg-severity-critical/15 text-severity-critical'
  if (score >= 9) return 'bg-severity-medium/15 text-severity-medium'
  return 'bg-severity-low/15 text-severity-low'
}

export function entityIcon(entityType: string) {
  switch (entityType) {
    case 'control': return Shield
    case 'policy': return FileText
    case 'risk': return TriangleAlert
    case 'incident':
    case 'breach': return Flame
    case 'vvt':
    case 'dpia':
    case 'avv': return Eye
    case 'dsr': return User
    case 'audit': return ClipboardList
    default: return Database
  }
}

export function entityLabel(entityType: string): string {
  const map: Record<string, string> = {
    control: 'Control', policy: 'Richtlinie', risk: 'Risiko', incident: 'Vorfall',
    vvt: 'VVT', dpia: 'DPIA', avv: 'AVV', breach: 'Datenpanne',
    dsr: 'Betroffenenanfrage', audit: 'Audit',
  }
  return map[entityType] ?? entityType
}

export function relativeTime(isoString: string): string {
  const diff = Math.floor((Date.now() - new Date(isoString).getTime()) / 1000)
  if (diff < 60) return 'gerade eben'
  if (diff < 3600) return `vor ${String(Math.floor(diff / 60))} Min.`
  if (diff < 86400) return `vor ${String(Math.floor(diff / 3600))} Std.`
  if (diff < 2592000) return `vor ${String(Math.floor(diff / 86400))} Tagen`
  return `vor ${String(Math.floor(diff / 2592000))} Monaten`
}

export function actionLabel(action: string): string {
  const map: Record<string, string> = {
    create: 'erstellt', update: 'aktualisiert', delete: 'gelöscht',
    approve: 'genehmigt', export: 'exportiert', review: 'überprüft',
  }
  return map[action] ?? action
}

export function KPICard({
  label, value, icon: Icon, to, critical, isLoading,
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
      aria-label={`${label}: ${value != null ? String(value) : 'wird geladen'}`}
    >
      <div className="flex items-center gap-2 mb-1">
        <Icon className={`w-4 h-4 ${isAlert ? 'text-severity-critical' : 'text-secondary'}`} aria-hidden="true" />
        <span className="text-[11px] text-secondary uppercase tracking-wider font-semibold">{label}</span>
      </div>
      {isLoading ? (
        <Skeleton className="h-8 w-16 mt-1" aria-label={`${label} wird geladen`} />
      ) : (
        <p className={`text-[32px] font-black leading-none ${isAlert ? 'text-severity-critical' : 'text-primary'}`} aria-hidden="true">
          {value ?? '—'}
        </p>
      )}
    </Link>
  )
}

export function FrameworkProgress({ scores }: {
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
            <div
              className="h-1.5 rounded-full bg-border overflow-hidden"
              role="progressbar"
              aria-valuenow={pct}
              aria-valuemin={0}
              aria-valuemax={100}
              aria-labelledby={progressId}
              aria-label={`${fw.framework_name}: ${String(pct)}% umgesetzt`}
            >
              <div className={`h-full rounded-full transition-all ${color}`} style={{ width: `${String(pct)}%` }} />
            </div>
          </div>
        )
      })}
    </div>
  )
}

export const RISK_STATUS_LABELS: Record<string, string> = {
  open: 'Offen', in_review: 'In Prüfung', accepted: 'Akzeptiert',
  closed: 'Geschlossen', mitigated: 'Gemindert',
}

export function TopRisksList({ risks }: { risks: RiskSummary[] }) {
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

export function ActivityTimeline({ entries }: { entries: ActivityEntry[] }) {
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

export function ComplianceProgressCard({ scores, isLoading }: {
  scores: FrameworkScore[]
  isLoading?: boolean
}) {
  const totals = scores.reduce(
    (acc, fw) => { acc.total += fw.total_controls; acc.implemented += fw.implemented_controls; return acc },
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
            <span className={`text-[11px] font-medium ${pct >= 80 ? 'text-severity-low' : pct >= 50 ? 'text-severity-medium' : 'text-severity-critical'}`}>
              {totals.total - totals.implemented} offen
            </span>
          </div>
          <div
            className="h-2 rounded-full bg-border overflow-hidden"
            role="progressbar"
            aria-valuenow={pct}
            aria-valuemin={0}
            aria-valuemax={100}
            aria-label={`Gesamt-Compliance: ${String(pct)}%`}
          >
            <div className={`h-full rounded-full transition-all duration-500 ${color}`} style={{ width: `${String(pct)}%` }} />
          </div>
        </>
      )}
    </section>
  )
}
