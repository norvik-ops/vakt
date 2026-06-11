// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

import { useState, useMemo } from 'react'
import { useParams, Link } from 'react-router-dom'
import { ArrowLeft, Download, ChevronDown, ChevronRight } from 'lucide-react'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Badge } from '../../../components/ui/badge'
import { Button } from '../../../components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '../../../components/ui/select'
import {
  useBSITargetObject,
  useBSICheckSheet,
  useBSICheckSummary,
  useSetBSICheckResult,
} from '../hooks/useBSICheck'
import type { BSIUmsetzungsstatus, BSICheckResult } from '../types'

// ── status helpers ─────────────────────────────────────────────────────────────

const STATUS_LABELS: Record<BSIUmsetzungsstatus, string> = {
  ja: 'Ja',
  teilweise: 'Teilweise',
  nein: 'Nein',
  entbehrlich: 'Entbehrlich',
}

const STATUS_COLORS: Record<BSIUmsetzungsstatus, string> = {
  ja: 'bg-green-900/30 text-green-300 border-green-700',
  teilweise: 'bg-yellow-900/30 text-yellow-300 border-yellow-700',
  nein: 'bg-red-900/30 text-red-300 border-red-700',
  entbehrlich: 'bg-surface2 text-secondary border-border',
}

// ── Summary gauge ──────────────────────────────────────────────────────────────

function SummaryBar({ pct, ja, teilweise, entbehrlich, nein, total }: {
  pct: number; ja: number; teilweise: number; entbehrlich: number; nein: number; total: number
}) {
  const color = pct >= 80 ? 'bg-green-500' : pct >= 50 ? 'bg-yellow-500' : 'bg-red-500'
  return (
    <div className="rounded-lg border border-border bg-surface p-4 flex flex-col gap-3">
      <div className="flex items-center justify-between">
        <span className="text-sm font-semibold text-primary">Umsetzungsgrad</span>
        <span className="text-2xl font-bold text-primary">{pct.toFixed(1)} %</span>
      </div>
      <div className="h-2 rounded-full bg-surface2 overflow-hidden">
        <div className={`h-full rounded-full transition-all ${color}`} style={{ width: `${pct}%` }} />
      </div>
      <div className="flex gap-3 text-xs text-secondary flex-wrap">
        <span className="text-green-400">✓ {ja}× ja</span>
        <span className="text-yellow-400">~ {teilweise}× teilweise</span>
        <span className="text-red-400">✗ {nein}× nein</span>
        <span className="text-secondary">○ {entbehrlich}× entbehrlich</span>
        <span className="ml-auto">{total} gesamt</span>
      </div>
    </div>
  )
}

// ── Inline row editor ──────────────────────────────────────────────────────────

function CheckRow({ row, targetObjectId }: { row: BSICheckResult; targetObjectId: string }) {
  const setResult = useSetBSICheckResult(targetObjectId)
  const [saving, setSaving] = useState(false)

  async function handleStatusChange(val: string) {
    setSaving(true)
    await setResult.mutateAsync({ anforderungId: row.anforderung_id, umsetzungsstatus: val as BSIUmsetzungsstatus })
    setSaving(false)
  }

  return (
    <div className="flex items-center gap-3 px-4 py-2.5 border-b border-border last:border-0">
      <div className="w-[110px] shrink-0">
        <span className="text-[11px] font-mono text-secondary">{row.anforderung_id}</span>
      </div>
      <p className="flex-1 text-[13px] text-primary min-w-0">{row.anforderung_title}</p>
      <div className="w-[140px] shrink-0">
        <Select
          value={row.umsetzungsstatus}
          onValueChange={(v) => void handleStatusChange(v)}
          disabled={saving}
        >
          <SelectTrigger className="h-7 text-xs">
            <SelectValue>
              <Badge className={`text-[11px] border ${STATUS_COLORS[row.umsetzungsstatus]}`}>
                {STATUS_LABELS[row.umsetzungsstatus]}
              </Badge>
            </SelectValue>
          </SelectTrigger>
          <SelectContent>
            {(Object.keys(STATUS_LABELS) as BSIUmsetzungsstatus[]).map((s) => (
              <SelectItem key={s} value={s}>
                <Badge className={`text-[11px] border ${STATUS_COLORS[s]}`}>{STATUS_LABELS[s]}</Badge>
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
    </div>
  )
}

// ── Grouped by Baustein ────────────────────────────────────────────────────────

function BausteinGroup({ bausteinId, rows, targetObjectId }: {
  bausteinId: string
  rows: BSICheckResult[]
  targetObjectId: string
}) {
  const [open, setOpen] = useState(true)
  const jaCount = rows.filter((r) => r.umsetzungsstatus === 'ja').length
  const pct = Math.round((jaCount / rows.length) * 100)

  return (
    <div className="rounded-lg border border-border bg-surface overflow-hidden">
      <button
        onClick={() => { setOpen((v) => !v) }}
        className="w-full flex items-center gap-3 px-4 py-2.5 hover:bg-muted/30 text-left"
      >
        {open ? <ChevronDown className="w-4 h-4 text-secondary" /> : <ChevronRight className="w-4 h-4 text-secondary" />}
        <Badge className="bg-severity-info-bg/60 text-severity-info border-transparent text-[11px] font-mono shrink-0">
          {bausteinId}
        </Badge>
        <span className="text-sm font-medium text-primary flex-1">
          {rows[0]?.baustein_id ?? bausteinId}
        </span>
        <span className="text-xs text-secondary">{rows.length} Anforderungen</span>
        <span className={`text-xs font-medium ${pct >= 80 ? 'text-green-400' : pct >= 50 ? 'text-yellow-400' : 'text-red-400'}`}>
          {pct} %
        </span>
      </button>
      {open && (
        <div className="border-t border-border">
          {rows.map((r) => (
            <CheckRow key={r.anforderung_id} row={r} targetObjectId={targetObjectId} />
          ))}
        </div>
      )}
    </div>
  )
}

// ── Page ───────────────────────────────────────────────────────────────────────

export default function BSICheckSheetPage() {
  const { id = '' } = useParams<{ id: string }>()
  const { data: obj } = useBSITargetObject(id)
  const { data: sheet = [], isLoading } = useBSICheckSheet(id)
  const { data: summary } = useBSICheckSummary(id)

  const grouped = useMemo(() => {
    const map = new Map<string, BSICheckResult[]>()
    for (const row of sheet) {
      const list = map.get(row.baustein_id) ?? []
      list.push(row)
      map.set(row.baustein_id, list)
    }
    return map
  }, [sheet])

  function handleDownloadCSV() {
    const header = 'baustein_id,anforderung_id,anforderung_title,umsetzungsstatus\n'
    const rows = sheet
      .map((r) => `${r.baustein_id},${r.anforderung_id},"${r.anforderung_title}",${r.umsetzungsstatus}`)
      .join('\n')
    const blob = new Blob([header + rows], { type: 'text/csv' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `bsi-check-${id}.csv`
    a.click()
    URL.revokeObjectURL(url)
  }

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title={obj ? `IT-Grundschutz-Check: ${obj.name}` : 'IT-Grundschutz-Check'}
        description="Umsetzungsstatus je Anforderung erfassen"
        actions={
          <div className="flex items-center gap-2">
            <Button size="sm" variant="outline" onClick={handleDownloadCSV} disabled={sheet.length === 0}>
              <Download className="w-4 h-4 mr-1" />
              CSV
            </Button>
            <Button size="sm" variant="ghost" asChild>
              <Link to="/vaktcomply/bsi/target-objects">
                <ArrowLeft className="w-4 h-4 mr-1" />
                Zurück
              </Link>
            </Button>
          </div>
        }
      />

      <div className="p-6 space-y-4">
        {summary && (
          <SummaryBar
            pct={summary.umsetzungsgrad_pct}
            ja={summary.ja}
            teilweise={summary.teilweise}
            entbehrlich={summary.entbehrlich}
            nein={summary.nein}
            total={summary.total}
          />
        )}

        {isLoading && <p className="text-sm text-secondary">Lade Prüfbogen…</p>}

        {!isLoading && sheet.length === 0 && (
          <div className="rounded-lg border border-dashed border-border p-8 text-center space-y-2">
            <p className="text-sm font-medium text-primary">Keine Bausteine zugewiesen</p>
            <p className="text-xs text-secondary">
              Weisen Sie dem Zielobjekt zunächst Bausteine über die Modellierungsseite zu.
            </p>
            <Button size="sm" variant="outline" asChild className="mt-2">
              <Link to="/vaktcomply/bsi-modeling">Zur Modellierung</Link>
            </Button>
          </div>
        )}

        {Array.from(grouped.entries()).map(([bid, rows]) => (
          <BausteinGroup key={bid} bausteinId={bid} rows={rows} targetObjectId={id} />
        ))}
      </div>
    </div>
  )
}
