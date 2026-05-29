import { Link } from 'react-router-dom'
import {
  Bug, FileCheck, Key, Fish, Eye,
  AlertTriangle, ChevronRight, Settings,
  ClipboardList, Clock, TriangleAlert, ListTodo,
  CalendarDays, GripVertical,
} from 'lucide-react'
import {
  KPICard, FrameworkProgress, TopRisksList, ActivityTimeline,
  ComplianceProgressCard, relativeTime,
} from './DashboardComponents'
import { ScoreHistoryCard, ScoreForecastHint } from './ForecastChart'
import { TodayWidget, MyTasksWidget, QuickWinsCard } from './DashboardWidgets'
import { OnboardingBanner, OnboardingWizard } from '../components/OnboardingWizard'
import { GettingStartedChecklist } from '../shared/components/GettingStartedChecklist'
import type { WidgetKey } from './WidgetConfigPanel'
import type { DashboardAggregate } from '../hooks/useDashboard'
import type { ScoreHistoryEntry } from '../modules/vaktcomply/hooks/useScoreHistory'
import type { OnboardingStatus } from '../hooks/useOnboarding'
import type { RecentPage } from '../shared/hooks/useRecentPages'
import type { AuditMilestone } from '../modules/vaktcomply/types'

interface WidgetGridProps {
  widgetOrder: string[]
  editMode: boolean
  handleDragStart: (id: string) => void
  handleDragOver: (e: React.DragEvent, id: string) => void
  handleDrop: () => void
  agg: DashboardAggregate | undefined
  aggLoading: boolean
  scoreHistory: ScoreHistoryEntry[] | undefined
  widgets: Record<WidgetKey, boolean>
  onboarding: OnboardingStatus | undefined
  wizardOpen: boolean
  setWizardOpen: React.Dispatch<React.SetStateAction<boolean>>
  scoreError: boolean
  aggError: boolean
  recentPages: RecentPage[]
  nextMilestone: AuditMilestone | null | undefined
  kpiLoading: boolean
  critCount: number | null
  fwCount: number | null
  projCount: number | null
  activeCampaignCount: number | null
  openBreachCount: number | null
}

export function WidgetGrid({
  widgetOrder, editMode, handleDragStart, handleDragOver, handleDrop,
  agg, aggLoading, scoreHistory, widgets,
  onboarding, wizardOpen, setWizardOpen, scoreError, aggError,
  recentPages, nextMilestone, kpiLoading,
  critCount, fwCount, projCount, activeCampaignCount, openBreachCount,
}: WidgetGridProps) {
  const MODULES = [
    {
      label: 'Vakt Scan', description: 'Scanner-Orchestrierung & Vulnerability Management',
      icon: Bug, iconColor: 'text-severity-critical',
      badge: critCount != null ? `${String(critCount)} kritisch` : '—',
      badgeColor: critCount ? 'text-severity-critical' : 'text-secondary',
      path: '/vaktscan',
    },
    {
      label: 'Vakt Comply', description: 'Compliance & Dokumentation — NIS2, ISO 27001, BSI',
      icon: FileCheck, iconColor: 'text-severity-low',
      badge: fwCount != null ? `${String(fwCount)} Framework${fwCount === 1 ? '' : 's'}` : '—',
      badgeColor: fwCount ? 'text-severity-low' : 'text-secondary',
      path: '/vaktcomply',
    },
    {
      label: 'Vakt Vault', description: 'Secrets Management, Rotation & Git-Scanning',
      icon: Key, iconColor: 'text-severity-medium',
      badge: projCount != null ? `${String(projCount)} Projekt${projCount === 1 ? '' : 'e'}` : '—',
      badgeColor: 'text-secondary',
      path: '/vaktvault',
    },
    {
      label: 'Vakt Aware', description: 'Phishing-Simulation & Security Awareness',
      icon: Fish, iconColor: 'text-brand-hover',
      badge: activeCampaignCount != null ? `${String(activeCampaignCount)} aktiv` : '—',
      badgeColor: activeCampaignCount ? 'text-brand-hover' : 'text-secondary',
      path: '/vaktaware',
    },
    {
      label: 'Vakt Privacy', description: 'DSGVO-Dokumentation — VVT, DPIA, AVV, Meldepflichten',
      icon: Eye, iconColor: 'text-severity-info',
      badge: openBreachCount != null ? `${String(openBreachCount)} offen` : '—',
      badgeColor: openBreachCount ? 'text-severity-critical' : 'text-secondary',
      path: '/vaktprivacy',
    },
  ]

  return (
    <div className="flex-1 overflow-auto p-6 space-y-6">
      {widgets.evidence_expiry && <GettingStartedChecklist />}

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
                <span className="text-[10px] text-secondary/60 ml-1">
                  {relativeTime(new Date(page.visitedAt).toISOString())}
                </span>
              </Link>
            ))}
          </div>
        </section>
      )}

      {(scoreError || aggError) && (
        <div
          role="alert"
          className="flex items-center gap-2 rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive"
        >
          <AlertTriangle className="w-4 h-4 shrink-0" aria-hidden="true" />
          <span>Dashboard-Daten konnten nicht geladen werden.</span>
        </div>
      )}

      {widgets.onboarding && onboarding && !onboarding.completed && !onboarding.dismissed && (
        <OnboardingBanner status={onboarding} onOpen={() => { setWizardOpen(true) }} />
      )}
      {onboarding && !onboarding.dismissed && (
        <OnboardingWizard open={wizardOpen} onClose={() => { setWizardOpen(false) }} status={onboarding} />
      )}

      {widgets.open_findings && (
        <section>
          <h2 className="text-[14px] font-semibold text-primary mb-3">Compliance-Übersicht</h2>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
            <KPICard label="Offene CAPAs" value={agg?.open_capas} icon={ClipboardList} to="/vaktcomply/capas" critical isLoading={kpiLoading} />
            <KPICard label="Überfällige Controls" value={agg?.overdue_controls} icon={Clock} to="/vaktcomply/overdue-reviews" critical isLoading={kpiLoading} />
            <KPICard label="Kritische Risiken" value={agg?.critical_risks} icon={TriangleAlert} to="/vaktcomply/risks" critical isLoading={kpiLoading} />
            <KPICard label="Offene Aufgaben" value={agg?.overdue_tasks} icon={ListTodo} to="/vaktcomply/overdue-reviews" critical isLoading={kpiLoading} />
          </div>
        </section>
      )}

      {nextMilestone && (
        <Link
          to="/vaktcomply/certification-timeline"
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
                ? `in ${String(nextMilestone.days_remaining)} Tagen`
                : `${String(Math.abs(nextMilestone.days_remaining ?? 0))} Tage überfällig`}
            </p>
          </div>
          <ChevronRight className="w-4 h-4 text-brand shrink-0" />
        </Link>
      )}

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
            <div className="absolute -left-5 top-1/2 -translate-y-1/2 opacity-0 group-hover/widget:opacity-100 transition-opacity cursor-grab active:cursor-grabbing" aria-hidden="true">
              <GripVertical className="w-4 h-4 text-secondary" />
            </div>
          ) : null

          const widgetOpacity = editMode ? 'transition-opacity' : ''

          switch (widgetId) {
            case 'today':
              return (
                <div key={widgetId} {...wrapperProps}>
                  {dragHandle}
                  <div className={widgetOpacity}><TodayWidget /></div>
                </div>
              )
            case 'my_tasks':
              return (
                <div key={widgetId} {...wrapperProps}>
                  {dragHandle}
                  <div className={widgetOpacity}><MyTasksWidget /></div>
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
                  <div className={widgetOpacity}><QuickWinsCard /></div>
                </div>
              )
            case 'compliance_progress':
              return null
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
                        <Link to="/vaktcomply/risks" className="text-[10px] text-brand hover:underline">Alle anzeigen</Link>
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

      <section>
        <h2 className="text-[16px] font-semibold text-primary mb-4">Einstellungen</h2>
        <div className="space-y-px">
          {[
            { to: '/settings/score-config', label: 'Score-Konfiguration', desc: 'Score-Formel und Gewichtungen anpassen' },
            { to: '/settings/alerting', label: 'Alerting', desc: 'Benachrichtigungskanäle verwalten' },
          ].map(({ to, label, desc }) => (
            <Link
              key={to}
              to={to}
              className="flex items-center justify-between py-3 border-b border-border hover:bg-muted/50 -mx-1 px-1 rounded-sm transition-colors group"
            >
              <div className="flex items-center gap-3">
                <Settings className="w-4 h-4 shrink-0 text-secondary" />
                <div>
                  <p className="text-[13px] text-primary font-medium">{label}</p>
                  <p className="text-[11px] text-secondary mt-0.5">{desc}</p>
                </div>
              </div>
              <ChevronRight className="w-3.5 h-3.5 text-brand opacity-0 group-hover:opacity-100 transition-opacity ml-4" />
            </Link>
          ))}
        </div>
      </section>
    </div>
  )
}
