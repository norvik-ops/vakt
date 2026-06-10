import { Link } from 'react-router-dom'
import {
  AlertTriangle, CheckCircle, ShieldAlert, Activity, Flame,
  TrendingUp, TrendingDown, Minus, Settings2, GripVertical,
} from 'lucide-react'
import { Skeleton } from '../components/ui/skeleton'
import { Switch } from '../components/ui/switch'
import { Label } from '../components/ui/label'
import { scoreStrokeColor } from './DashboardComponents'
import { WIDGET_LABELS } from './WidgetConfigPanel'
import type { WidgetKey } from './WidgetConfigPanel'

// SVG horseshoe-style progress ring for the Security Score.
// 270° arc (25% gap at bottom), starts at 7:30 o'clock.
const RING = 120
const STROKE_W = 9
const RADIUS = (RING / 2) - (STROKE_W / 2)
const CIRC = 2 * Math.PI * RADIUS
const ARC = CIRC * 0.75   // 270° of full circle

function ScoreRing({ score, scoreTrend }: { score: number | undefined; scoreTrend: number | null }) {
  const val = score ?? 0
  const filled = ARC * (val / 100)
  const strokeColor = scoreStrokeColor(score)
  // Rotate so the gap sits at the bottom (start at top-left, 225° from standard 0°)
  const rotate = 'rotate(135 60 60)'

  return (
    <Link
      to="/settings/score-config"
      className="block w-[120px] hover:opacity-90 transition-opacity focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand rounded-full"
      aria-label={`Security Score: ${String(score ?? '—')} von 100. Klick öffnet die Score-Konfiguration.`}
      title="Aggregierte Sicherheitsbewertung: 0–49 schwach, 50–69 ausbaufähig, 70–89 gut, 90+ exzellent."
    >
      <svg viewBox={`0 0 ${RING} ${RING}`} width={RING} height={RING} aria-hidden="true">
        {/* track */}
        <circle
          cx="60" cy="60" r={RADIUS}
          fill="none"
          stroke="currentColor"
          strokeWidth={STROKE_W}
          strokeDasharray={`${ARC} ${CIRC}`}
          strokeLinecap="round"
          transform={rotate}
          className="text-border"
        />
        {/* progress */}
        <circle
          cx="60" cy="60" r={RADIUS}
          fill="none"
          stroke={strokeColor}
          strokeWidth={STROKE_W}
          strokeDasharray={`${filled} ${CIRC}`}
          strokeLinecap="round"
          transform={rotate}
          style={{ transition: 'stroke-dasharray 0.8s ease, stroke 0.4s ease' }}
        />
        {/* score value */}
        <text
          x="60" y="56"
          textAnchor="middle"
          dominantBaseline="middle"
          fontSize="26"
          fontWeight="900"
          fill={strokeColor}
          style={{ transition: 'fill 0.4s ease', fontFamily: 'inherit' }}
        >
          {score ?? '—'}
        </text>
        {/* /100 */}
        <text
          x="60" y="76"
          textAnchor="middle"
          dominantBaseline="middle"
          fontSize="11"
          fill="currentColor"
          className="text-secondary"
          style={{ fontFamily: 'inherit' }}
        >
          / 100
        </text>
      </svg>
      {scoreTrend !== null && (
        <div
          className={`flex items-center justify-center gap-0.5 mt-1 text-[11px] font-semibold ${
            scoreTrend > 0.5 ? 'text-severity-low' : scoreTrend < -0.5 ? 'text-severity-critical' : 'text-secondary'
          }`}
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
        </div>
      )}
      <p className="text-[10px] text-secondary text-center mt-0.5">Gesamtbewertung</p>
    </Link>
  )
}

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

        <p className="text-[10px] font-semibold text-secondary uppercase tracking-wider mb-2 opacity-60">
          Security Score
        </p>
        {scoreLoading ? (
          <div className="flex items-center justify-center w-[120px] h-[120px]">
            <Skeleton className="w-[120px] h-[120px] rounded-full" />
          </div>
        ) : (
          <ScoreRing score={scoreData?.score} scoreTrend={scoreTrend} />
        )}

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

      </div>
    </div>
  )
}
