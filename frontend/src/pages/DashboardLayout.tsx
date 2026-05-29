import { Link } from 'react-router-dom'
import {
  AlertTriangle, CheckCircle, ShieldAlert, Activity, Flame,
  TrendingUp, TrendingDown, Minus, Settings2, GripVertical,
} from 'lucide-react'
import { Skeleton } from '../components/ui/skeleton'
import { Switch } from '../components/ui/switch'
import { Label } from '../components/ui/label'
import { scoreColor } from './DashboardComponents'
import { WIDGET_LABELS } from './WidgetConfigPanel'
import type { WidgetKey } from './WidgetConfigPanel'

interface StatItem {
  label: string
  value: string
  icon: React.ElementType
  color: string
  path: string
  loading: boolean
}

interface DashboardLayoutProps {
  scoreLoading: boolean
  scoreData: { score: number } | undefined
  scoreTrend: number | null
  critCount: number | null
  findingsLoading: boolean
  fwCount: number | null
  fwLoading: boolean
  projCount: number | null
  projLoading: boolean
  activeCampaignCount: number | null
  campLoading: boolean
  openBreachCount: number | null
  breachLoading: boolean
  editMode: boolean
  setEditMode: React.Dispatch<React.SetStateAction<boolean>>
  widgets: Record<WidgetKey, boolean>
  toggleWidget: (key: WidgetKey) => void
  widgetMenuOpen: boolean
  setWidgetMenuOpen: React.Dispatch<React.SetStateAction<boolean>>
  widgetMenuRef: React.RefObject<HTMLDivElement>
}

export function DashboardLayout({
  scoreLoading, scoreData, scoreTrend,
  critCount, findingsLoading, fwCount, fwLoading,
  projCount, projLoading, activeCampaignCount, campLoading,
  openBreachCount, breachLoading,
  editMode, setEditMode, widgets, toggleWidget,
  widgetMenuOpen, setWidgetMenuOpen, widgetMenuRef,
}: DashboardLayoutProps) {
  const { fmt } = { fmt: (n: number | null) => (n == null ? '—' : n.toString()) }

  const STATS: StatItem[] = [
    {
      label: 'Kritische Findings', value: fmt(critCount),
      icon: AlertTriangle, color: critCount ? 'text-severity-critical' : 'text-secondary',
      path: '/vaktscan/findings?severity=critical', loading: findingsLoading,
    },
    {
      label: 'Frameworks aktiv', value: fmt(fwCount),
      icon: CheckCircle, color: fwCount ? 'text-severity-low' : 'text-secondary',
      path: '/vaktcomply', loading: fwLoading,
    },
    {
      label: 'Vault-Projekte', value: fmt(projCount),
      icon: ShieldAlert, color: 'text-severity-medium',
      path: '/vaktvault', loading: projLoading,
    },
    {
      label: 'Aktive Kampagnen', value: fmt(activeCampaignCount),
      icon: Activity, color: activeCampaignCount ? 'text-brand-hover' : 'text-secondary',
      path: '/vaktaware', loading: campLoading,
    },
    {
      label: 'Offene Datenpannen', value: fmt(openBreachCount),
      icon: Flame, color: openBreachCount ? 'text-severity-critical' : 'text-secondary',
      path: '/vaktprivacy?filter=breach&status=open', loading: breachLoading,
    },
  ]

  return (
    <div className="w-full lg:w-[260px] lg:shrink-0 border-b lg:border-b-0 lg:border-r border-border bg-surface flex flex-col">
      <div className="flex-1 p-6 overflow-auto">
        <div className="flex items-center justify-between mb-6">
          <h1 className="text-[20px] font-bold text-primary">Dashboard</h1>
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
                onClick={() => { setWidgetMenuOpen((o) => !o) }}
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
                        <Switch id={`widget-${key}`} checked={widgets[key]} onCheckedChange={() => { toggleWidget(key) }} />
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
        <div
          className="flex items-end gap-1"
          title="Aggregierte Sicherheitsbewertung aus offenen Findings, Control-Coverage und Risiken. 0–49 schwach, 50–69 ausbaufähig, 70–89 gut, 90+ exzellent. Klick auf die Zahl für Konfiguration und Gewichtungen."
        >
          {scoreLoading ? (
            <Skeleton className="h-12 w-20" />
          ) : (
            <Link
              to="/settings/score-config"
              className={`text-[52px] font-black leading-none ${scoreColor(scoreData?.score)} hover:underline decoration-2 underline-offset-4`}
              aria-label={`Security Score: ${String(scoreData?.score ?? '—')} von 100. Klick öffnet die Score-Konfiguration.`}
            >
              {scoreData?.score ?? '—'}
            </Link>
          )}
          <p className="text-[16px] text-secondary mb-2">/ 100</p>
        </div>
        <div className="flex items-center gap-2 mt-1">
          <p className="text-[12px] text-secondary">Gesamtbewertung</p>
          {scoreTrend !== null && (
            <span
              className={`flex items-center gap-0.5 text-[11px] font-semibold ${scoreTrend > 0.5 ? 'text-severity-low' : scoreTrend < -0.5 ? 'text-severity-critical' : 'text-secondary'}`}
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
          <p className="text-[12px] text-severity-low">Keine offenen Datenpannen</p>
        ) : (
          <p className="text-[12px] text-severity-critical">
            {openBreachCount} offene Datenpanne{openBreachCount === 1 ? '' : 'n'}
          </p>
        )}
      </div>
    </div>
  )
}
