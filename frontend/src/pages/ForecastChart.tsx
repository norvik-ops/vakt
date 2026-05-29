import { useState } from 'react'
import { TrendingUp, TrendingDown, Minus } from 'lucide-react'
import {
  AreaChart, Area, XAxis, YAxis, Tooltip, ResponsiveContainer,
} from 'recharts'
import { Skeleton } from '../components/ui/skeleton'
import { useScoreHistory } from '../modules/vaktcomply/hooks/useScoreHistory'
import type { ScoreHistoryEntry } from '../modules/vaktcomply/hooks/useScoreHistory'

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

export function ScoreForecastHint({ entries }: { entries: ScoreHistoryEntry[] }) {
  const sample = entries.slice(-4)
  if (sample.length < 2) return null
  const points = sample.map((e, i) => ({ x: i, y: e.score }))
  const futureX = points[points.length - 1].x + 6
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
        <span className="font-semibold text-severity-low">{forecastClamped}%</span> in ~6 Wochen
        {forecastClamped <= currentScore ? ' (Score bereits stabil)' : ''}.
      </p>
    )
  }
  return (
    <p className="text-[11px] text-secondary mt-2">
      Abwärtstrend — ohne Maßnahmen könnte der Score auf{' '}
      <span className="font-semibold text-severity-critical">{forecastClamped}%</span> in ~6 Wochen fallen.
    </p>
  )
}

function fmtAxisDate(iso: string): string {
  const parts = iso.split('-')
  if (parts.length !== 3) return iso
  return `${parts[2]}.${parts[1]}.`
}

interface ChartTooltipProps {
  active?: boolean
  payload?: Array<{ value: number; payload: ScoreHistoryEntry }>
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

export function ScoreHistoryCard() {
  const [days, setDays] = useState<30 | 90>(30)
  const { data: entries, isLoading } = useScoreHistory(days)

  let trendDelta: number | null = null
  if (entries && entries.length >= 2) {
    trendDelta = entries[entries.length - 1].score - entries[0].score
  }

  const chartData = entries?.map((e) => ({ ...e, label: fmtAxisDate(e.date) })) ?? []

  return (
    <section className="rounded-lg border border-border bg-surface p-4">
      <div className="flex items-center justify-between mb-3">
        <h2 className="text-[13px] font-semibold text-primary">Compliance-Verlauf</h2>
        <div className="flex items-center gap-2">
          {trendDelta !== null && (
            <span
              className={`flex items-center gap-0.5 text-[11px] font-semibold ${trendDelta > 0.5 ? 'text-severity-low' : trendDelta < -0.5 ? 'text-severity-critical' : 'text-secondary'}`}
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
              onClick={() => { setDays(30) }}
              aria-label="Verlauf 30 Tage anzeigen"
              aria-pressed={days === 30}
            >
              30 Tage
            </button>
            <button
              className={`px-2 py-1 transition-colors ${days === 90 ? 'bg-brand text-white' : 'bg-surface text-secondary hover:bg-border/50'}`}
              onClick={() => { setDays(90) }}
              aria-label="Verlauf 90 Tage anzeigen"
              aria-pressed={days === 90}
            >
              90 Tage
            </button>
          </div>
        </div>
      </div>

      {isLoading ? (
        <Skeleton className="h-[160px] w-full" />
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
              tickFormatter={(v: number) => `${String(v)}%`}
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
