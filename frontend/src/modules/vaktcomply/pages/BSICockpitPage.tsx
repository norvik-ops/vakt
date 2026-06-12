// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Download, AlertTriangle } from 'lucide-react'
import { PageHeader } from '../../../shared/components/PageHeader'
import { ProGate } from '../../../shared/components/ProGate'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { useBSICockpit, useBSIGapReport } from '../hooks/useBSICheck'

// ── Gauge ──────────────────────────────────────────────────────────────────────

function GaugeCard({ pct }: { pct: number }) {
  const { t } = useTranslation()
  const color =
    pct >= 80 ? 'hsl(var(--color-green-500, 142 71% 45%))' :
    pct >= 50 ? 'hsl(var(--color-yellow-500, 48 96% 53%))' :
    'hsl(var(--color-red-500, 0 84% 60%))'
  const radius = 52
  const circumference = 2 * Math.PI * radius
  const label = `${pct.toFixed(0)}% — ${t('bsi.cockpit.gesamtumsetzungsgrad')}`

  return (
    <div className="rounded-lg border border-border bg-surface p-5 flex flex-col items-center gap-2">
      <p className="text-xs font-semibold text-secondary uppercase tracking-wide" aria-hidden="true">
        {t('bsi.cockpit.gesamtumsetzungsgrad')}
      </p>
      <svg
        width="128"
        height="72"
        viewBox="0 0 128 72"
        role="img"
        aria-label={label}
      >
        <path
          d="M 10 66 A 54 54 0 0 1 118 66"
          fill="none"
          stroke="hsl(var(--muted-foreground) / 0.25)"
          strokeWidth="12"
          strokeLinecap="round"
        />
        <path
          d="M 10 66 A 54 54 0 0 1 118 66"
          fill="none"
          stroke={color}
          strokeWidth="12"
          strokeLinecap="round"
          strokeDasharray={`${(circumference / 2) * (pct / 100)} ${circumference / 2}`}
        />
        <text x="64" y="62" textAnchor="middle" fill="currentColor" fontSize="18" fontWeight="bold">
          {pct.toFixed(0)}%
        </text>
      </svg>
    </div>
  )
}

// ── Heatmap ────────────────────────────────────────────────────────────────────

function heatColor(pct: number): string {
  if (pct >= 80) return 'bg-green-700/60'
  if (pct >= 60) return 'bg-yellow-700/60'
  if (pct >= 30) return 'bg-orange-700/60'
  return 'bg-red-700/60'
}

function HeatmapTable({ rows }: { rows: { baustein_id: string; baustein_title: string; cells: { target_object_id: string; target_object_name: string; pct: number }[] }[] }) {
  const { t } = useTranslation()
  if (rows.length === 0) return <p className="text-sm text-secondary">{t('bsi.cockpit.noHeatmapData')}</p>

  const objects = rows[0]?.cells.map((c) => c.target_object_name) ?? []

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-xs">
        <thead>
          <tr>
            <th className="text-left font-medium text-secondary pb-2 pr-3 w-40">{t('bsi.cockpit.baustein')}</th>
            {objects.map((n) => (
              <th key={n} className="font-medium text-secondary pb-2 px-1 text-center max-w-[80px] truncate" title={n}>
                {n.length > 12 ? n.slice(0, 11) + '…' : n}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr key={row.baustein_id} className="border-t border-border/50">
              <td className="py-1 pr-3">
                <span className="font-mono text-[10px] text-secondary">{row.baustein_id}</span>
              </td>
              {row.cells.map((cell) => (
                <td key={cell.target_object_id} className="py-1 px-0.5">
                  <div
                    className={`rounded text-center text-[10px] font-medium py-0.5 px-1 ${heatColor(cell.pct)}`}
                    title={`${cell.target_object_name}: ${cell.pct.toFixed(0)}%`}
                  >
                    {cell.pct.toFixed(0)}
                  </div>
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

// ── Gap List ───────────────────────────────────────────────────────────────────

function GapList({ gaps }: { gaps: { anforderung_id: string; anforderung_title: string; baustein_id: string; affected_objects: number }[] }) {
  const { t } = useTranslation()
  if (gaps.length === 0) return <p className="text-sm text-secondary">{t('bsi.cockpit.noGaps')}</p>
  return (
    <div className="divide-y divide-border">
      {gaps.map((g) => (
        <div key={g.anforderung_id} className="flex items-start gap-2 py-2">
          <AlertTriangle className="w-3.5 h-3.5 text-yellow-400 shrink-0 mt-0.5" />
          <div className="min-w-0">
            <p className="text-xs font-medium text-primary truncate">{g.anforderung_title}</p>
            <p className="text-[11px] text-secondary">
              <span className="font-mono">{g.anforderung_id}</span>
              {' '}· {t('bsi.cockpit.affected', { count: g.affected_objects })}
            </p>
          </div>
          <Badge className="shrink-0 text-[10px] bg-surface2 text-secondary border-transparent ml-auto">
            {g.baustein_id}
          </Badge>
        </div>
      ))}
    </div>
  )
}

// ── CSV export ─────────────────────────────────────────────────────────────────

function useGapCSVDownload() {
  const { data: report } = useBSIGapReport()

  return function download() {
    if (!report) return
    const header = 'baustein_id,anforderung_id,anforderung_title,zielobjekt,umsetzungsstatus\n'
    const rows = report.gaps
      .map((g) =>
        `${g.baustein_id},${g.anforderung_id},"${g.anforderung_title.replace(/"/g, '""')}","${g.zielobjekt.replace(/"/g, '""')}",${g.umsetzungsstatus}`,
      )
      .join('\n')
    const blob = new Blob([header + rows], { type: 'text/csv' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = 'bsi-gap-report.csv'
    a.click()
    URL.revokeObjectURL(url)
  }
}

// ── Page ───────────────────────────────────────────────────────────────────────

export default function BSICockpitPage() {
  const { t } = useTranslation()
  const { data: cockpit, isLoading, isError, error } = useBSICockpit()
  const { data: gapReport } = useBSIGapReport()
  const downloadGapCSV = useGapCSVDownload()
  const [showAllGaps, setShowAllGaps] = useState(false)

  const visibleGaps = showAllGaps
    ? (gapReport?.gaps ?? [])
    : (gapReport?.gaps.slice(0, 10) ?? [])

  return (
    <ProGate error={isError ? error : null}>
    <div className="flex flex-col h-full">
      <PageHeader
        title={t('bsi.cockpit.title')}
        description={t('bsi.cockpit.description')}
        actions={
          <Button
            size="sm"
            variant="outline"
            onClick={downloadGapCSV}
            disabled={!gapReport || gapReport.gaps.length === 0}
          >
            <Download className="w-4 h-4 mr-1" />
            {t('bsi.cockpit.gapReportCSV')}
          </Button>
        }
      />

      <div className="p-6 space-y-6">
        {isLoading && <p className="text-sm text-secondary">{t('bsi.cockpit.loading')}</p>}

        {cockpit && (
          <>
            <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
              <GaugeCard pct={cockpit.overall_pct} />

              <div className="lg:col-span-2 rounded-lg border border-border bg-surface p-4 space-y-2">
                <p className="text-xs font-semibold text-secondary uppercase tracking-wide">
                  {t('bsi.cockpit.topGaps')}
                </p>
                <GapList gaps={cockpit.top_gaps} />
              </div>
            </div>

            {cockpit.heatmap.length > 0 && (
              <div className="rounded-lg border border-border bg-surface p-4 space-y-3">
                <p className="text-xs font-semibold text-secondary uppercase tracking-wide">
                  {t('bsi.cockpit.heatmapTitle')}
                </p>
                <div className="flex items-center gap-3 text-[11px] text-secondary flex-wrap">
                  <span className="flex items-center gap-1.5">
                    <span className="w-3 h-3 rounded bg-green-700/60 inline-block" /> ≥ 80%
                  </span>
                  <span className="flex items-center gap-1.5">
                    <span className="w-3 h-3 rounded bg-yellow-700/60 inline-block" /> 60–79%
                  </span>
                  <span className="flex items-center gap-1.5">
                    <span className="w-3 h-3 rounded bg-orange-700/60 inline-block" /> 30–59%
                  </span>
                  <span className="flex items-center gap-1.5">
                    <span className="w-3 h-3 rounded bg-red-700/60 inline-block" /> {'< 30%'}
                  </span>
                </div>
                <HeatmapTable rows={cockpit.heatmap} />
              </div>
            )}
          </>
        )}

        {gapReport && gapReport.gaps.length > 0 && (
          <div className="rounded-lg border border-border bg-surface p-4 space-y-3">
            <div className="flex items-center justify-between">
              <p className="text-xs font-semibold text-secondary uppercase tracking-wide">
                {t('bsi.cockpit.allGaps', { count: gapReport.gaps.length })}
              </p>
              <Button
                size="sm"
                variant="outline"
                onClick={downloadGapCSV}
              >
                <Download className="w-3.5 h-3.5 mr-1" />
                CSV
              </Button>
            </div>
            <div className="divide-y divide-border">
              {visibleGaps.map((g, i) => (
                <div key={i} className="flex items-start gap-3 py-2">
                  <span className="text-[11px] font-mono text-secondary w-[110px] shrink-0 mt-0.5">
                    {g.anforderung_id}
                  </span>
                  <div className="flex-1 min-w-0">
                    <p className="text-xs text-primary truncate">{g.anforderung_title}</p>
                    <p className="text-[11px] text-secondary">{g.zielobjekt}</p>
                  </div>
                  <Badge className="shrink-0 text-[10px] bg-red-900/30 text-red-300 border-red-700">
                    {g.umsetzungsstatus}
                  </Badge>
                </div>
              ))}
            </div>
            {gapReport.gaps.length > 10 && (
              <Button
                variant="ghost"
                size="sm"
                onClick={() => { setShowAllGaps((v) => !v) }}
                className="w-full text-xs"
              >
                {showAllGaps
                  ? t('bsi.cockpit.showLess')
                  : t('bsi.cockpit.showMore', { count: gapReport.gaps.length })}
              </Button>
            )}
          </div>
        )}
      </div>
    </div>
    </ProGate>
  )
}
