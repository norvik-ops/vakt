import { lazy, Suspense } from 'react'
import { useTranslation } from 'react-i18next'
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
// ponytail: lazy-split removes ~106 KiB gzip Recharts from initial bundle (S98-1)
const ScoreHistoryCard = lazy(() => import('./ForecastChart').then(m => ({ default: m.ScoreHistoryCard })))
const ScoreForecastHint = lazy(() => import('./ForecastChart').then(m => ({ default: m.ScoreForecastHint })))
import { TodayWidget, MyTasksWidget, QuickWinsCard } from './DashboardWidgets'
import { OnboardingChecklist } from '../shared/components/OnboardingChecklist'
import type { WidgetKey } from './WidgetConfigPanel'
import type { DashboardAggregate } from '../hooks/useDashboard'
import type { ScoreHistoryEntry } from '../modules/vaktcomply/hooks/useScoreHistory'
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
  scoreError, aggError,
  recentPages, nextMilestone, kpiLoading,
  critCount, fwCount, projCount, activeCampaignCount, openBreachCount,
}: WidgetGridProps) {
  const { t } = useTranslation()

  const MODULES = [
    {
      label: 'Vakt Scan', description: t('dashboard.modules.scan.description'),
      icon: Bug, iconColor: 'text-severity-critical',
      badge: critCount != null ? t('dashboard.modules.badgeCritical', { count: critCount }) : '—',
      badgeColor: critCount ? 'text-severity-critical' : 'text-secondary',
      path: '/vaktscan',
    },
    {
      label: 'Vakt Comply', description: t('dashboard.modules.comply.description'),
      icon: FileCheck, iconColor: 'text-severity-low',
      badge: fwCount != null ? t('dashboard.modules.badgeFramework', { count: fwCount }) : '—',
      badgeColor: fwCount ? 'text-severity-low' : 'text-secondary',
      path: '/vaktcomply',
    },
    {
      label: 'Vakt Vault', description: t('dashboard.modules.vault.description'),
      icon: Key, iconColor: 'text-severity-medium',
      badge: projCount != null ? t('dashboard.modules.badgeProject', { count: projCount }) : '—',
      badgeColor: 'text-secondary',
      path: '/vaktvault',
    },
    {
      label: 'Vakt Aware', description: t('dashboard.modules.aware.description'),
      icon: Fish, iconColor: 'text-brand-hover',
      badge: activeCampaignCount != null ? t('dashboard.modules.badgeActive', { count: activeCampaignCount }) : '—',
      badgeColor: activeCampaignCount ? 'text-brand-hover' : 'text-secondary',
      path: '/vaktaware',
    },
    {
      label: 'Vakt Privacy', description: t('dashboard.modules.privacy.description'),
      icon: Eye, iconColor: 'text-severity-info',
      badge: openBreachCount != null ? t('dashboard.modules.badgeOpen', { count: openBreachCount }) : '—',
      badgeColor: openBreachCount ? 'text-severity-critical' : 'text-secondary',
      path: '/vaktprivacy',
    },
  ]

  return (
    <div className="flex-1 overflow-auto p-6 space-y-6">
      {/* S89-5: guided "first 30 days" path (self-hides when dismissed/complete). */}
      <OnboardingChecklist />

      {widgets.recent_pages && recentPages.length > 0 && (
        <section>
          <p className="text-[10px] font-semibold text-secondary uppercase tracking-wider mb-2 opacity-60">
            {t('dashboard.recentlyVisited')}
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
          <span>{t('dashboard.loadError')}</span>
        </div>
      )}

      {widgets.open_findings && (
        <section>
          <h2 className="text-[14px] font-semibold text-primary mb-3">{t('dashboard.complianceOverview')}</h2>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
            <KPICard label={t('dashboard.kpi.openCapas')} value={agg?.open_capas} icon={ClipboardList} to="/vaktcomply/capas" critical isLoading={kpiLoading} />
            <KPICard label={t('dashboard.kpi.overdueControls')} value={agg?.overdue_controls} icon={Clock} to="/vaktcomply/overdue-reviews" critical isLoading={kpiLoading} />
            <KPICard label={t('dashboard.kpi.criticalRisks')} value={agg?.critical_risks} icon={TriangleAlert} to="/vaktcomply/risks" critical isLoading={kpiLoading} />
            <KPICard label={t('dashboard.kpi.openTasks')} value={agg?.overdue_tasks} icon={ListTodo} to="/vaktcomply/overdue-reviews" critical isLoading={kpiLoading} />
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
              {t('dashboard.nextAudit', { title: nextMilestone.title })}
            </p>
            <p className="text-[11px] text-secondary">
              {nextMilestone.days_remaining === 0
                ? t('dashboard.today')
                : nextMilestone.days_remaining != null && nextMilestone.days_remaining > 0
                ? t('dashboard.inDays', { count: nextMilestone.days_remaining })
                : t('dashboard.overdueDays', { count: Math.abs(nextMilestone.days_remaining ?? 0) })}
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
                      <Suspense fallback={<div className="h-32 animate-pulse rounded bg-muted" />}>
                        <ScoreHistoryCard />
                        {scoreHistory && scoreHistory.length >= 2 && (
                          <ScoreForecastHint entries={scoreHistory} />
                        )}
                      </Suspense>
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
                        <h2 className="text-[13px] font-semibold text-primary">{t('dashboard.frameworkProgress')}</h2>
                        {agg && (
                          <span className="text-[10px] text-secondary">
                            {t('dashboard.policiesActive', { approved: agg.policies_approved, total: agg.policies_total })}
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
                        <h2 className="text-[13px] font-semibold text-primary">{t('dashboard.topRisks')}</h2>
                        <Link to="/vaktcomply/risks" className="text-[10px] text-brand hover:underline">{t('dashboard.showAll')}</Link>
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
                      <h2 className="text-[13px] font-semibold text-primary mb-3">{t('dashboard.recentActivity')}</h2>
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
                        <h2 className="text-[16px] font-semibold text-primary">{t('dashboard.modules.title')}</h2>
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
        <h2 className="text-[16px] font-semibold text-primary mb-4">{t('dashboard.settingsSection')}</h2>
        <div className="space-y-px">
          {[
            { to: '/settings/score-config', label: t('dashboard.scoreConfig'), desc: t('dashboard.scoreConfigDesc') },
            { to: '/settings/alerting', label: t('dashboard.alerting'), desc: t('dashboard.alertingDesc') },
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
