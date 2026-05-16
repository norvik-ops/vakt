import { useState } from 'react'
import { Link } from 'react-router-dom'
import {
  Bug, FileCheck, Key, Fish, Eye,
  AlertTriangle, CheckCircle, ShieldAlert, Activity, Flame, ChevronRight, Settings,
  ClipboardList, Clock, TriangleAlert, ListTodo,
  Shield, FileText, User, Database,
} from 'lucide-react'
import { useFindings } from '../modules/secpulse/hooks/useFindings'
import { useFrameworks } from '../modules/secvitals/hooks/useFrameworks'
import { useProjects } from '../modules/secvault/hooks/useProjects'
import { useCampaigns } from '../modules/secreflex/hooks/useCampaigns'
import { useBreaches } from '../modules/secprivacy/hooks/useBreaches'
import { useDashboardScore, useDashboardAggregate } from '../hooks/useDashboard'
import type { RiskSummary, ActivityEntry } from '../hooks/useDashboard'
import { useOnboardingStatus } from '../hooks/useOnboarding'
import { OnboardingBanner, OnboardingWizard } from '../components/OnboardingWizard'
import { Skeleton } from '../components/ui/skeleton'

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
    >
      <div className="flex items-center gap-2 mb-1">
        <Icon className={`w-4 h-4 ${isAlert ? 'text-[#ef4444]' : 'text-secondary'}`} />
        <span className="text-[11px] text-secondary uppercase tracking-wider font-semibold">{label}</span>
      </div>
      {isLoading ? (
        <Skeleton className="h-8 w-16 mt-1" />
      ) : (
        <p className={`text-[32px] font-black leading-none ${isAlert ? 'text-[#ef4444]' : 'text-primary'}`}>
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
        return (
          <div key={fw.framework_id}>
            <div className="flex items-center justify-between mb-1">
              <span className="text-[12px] font-medium text-primary truncate max-w-[60%]">
                {fw.framework_name}
              </span>
              <span className="text-[12px] text-secondary shrink-0 ml-2">
                {fw.implemented_controls} / {fw.total_controls} · {pct}%
              </span>
            </div>
            <div className="h-1.5 rounded-full bg-border overflow-hidden">
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
            <span className="mt-0.5 p-1 rounded bg-border/60 shrink-0">
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
// Main dashboard
// ---------------------------------------------------------------------------

export default function Dashboard() {
  const { data: onboarding } = useOnboardingStatus()
  const [wizardOpen, setWizardOpen] = useState(false)

  const { data: scoreData, isLoading: scoreLoading } = useDashboardScore()
  const { data: agg, isLoading: aggLoading } = useDashboardAggregate()
  const { data: critFindings, isLoading: findingsLoading } = useFindings({ severity: 'critical' })
  const { data: frameworks, isLoading: fwLoading } = useFrameworks()
  const { data: projects, isLoading: projLoading } = useProjects()
  const { data: campaigns, isLoading: campLoading } = useCampaigns()
  const { data: breaches, isLoading: breachLoading } = useBreaches()

  const kpiLoading = aggLoading

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
          <h1 className="text-[20px] font-bold text-primary mb-6">Dashboard</h1>

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
          <p className="text-[12px] text-secondary mt-1">Gesamtbewertung</p>

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
        {/* Onboarding (preserved) */}
        {onboarding && !onboarding.completed && !onboarding.dismissed && (
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
        <section>
          <h2 className="text-[14px] font-semibold text-primary mb-3">Compliance-Übersicht</h2>
          <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
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
        </section>

        {/* ── Framework progress + Top risks ── */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          {/* Framework progress */}
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

          {/* Top 5 risks */}
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

        {/* ── Recent activity ── */}
        <section className="rounded-lg border border-border bg-surface p-4">
          <h2 className="text-[13px] font-semibold text-primary mb-3">Letzte Aktivitäten</h2>
          <ActivityTimeline entries={agg?.recent_activity ?? []} />
        </section>

        {/* ── Module list (preserved) ── */}
        <section>
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-[16px] font-semibold text-primary">Module</h2>
          </div>
          <div className="space-y-px">
            {MODULES.map(({ label, description, icon: Icon, iconColor, badge, badgeColor, path }) => (
              <Link
                key={label}
                to={path}
                className="flex items-center justify-between py-3 border-b border-border hover:bg-[#f1f5f9] dark:hover:bg-[#1E2235] -mx-1 px-1 rounded-sm transition-colors group"
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

        {/* ── Settings links (preserved) ── */}
        <section>
          <h2 className="text-[16px] font-semibold text-primary mb-4">Einstellungen</h2>
          <div className="space-y-px">
            <Link
              to="/settings/score-config"
              className="flex items-center justify-between py-3 border-b border-border hover:bg-[#f1f5f9] dark:hover:bg-[#1E2235] -mx-1 px-1 rounded-sm transition-colors group"
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
              className="flex items-center justify-between py-3 border-b border-border hover:bg-[#f1f5f9] dark:hover:bg-[#1E2235] -mx-1 px-1 rounded-sm transition-colors group"
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
