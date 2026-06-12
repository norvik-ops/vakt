// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

import { useState, useMemo } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { ArrowLeft, Download, ChevronDown, ChevronRight } from 'lucide-react'
import { PageHeader } from '../../../shared/components/PageHeader'
import { ProGate } from '../../../shared/components/ProGate'
import { Badge } from '../../../components/ui/badge'
import { Button, buttonVariants } from '../../../components/ui/button'
import { cn } from '../../../lib/utils'
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

// ── stufe helpers ─────────────────────────────────────────────────────────────

const STUFE_COLORS: Record<string, string> = {
  basis:    'bg-blue-900/30 text-blue-300 border-blue-700',
  standard: 'bg-purple-900/30 text-purple-300 border-purple-700',
  erhoeht:  'bg-orange-900/30 text-orange-300 border-orange-700',
}

// ── status helpers ─────────────────────────────────────────────────────────────

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
  const { t } = useTranslation()
  const color = pct >= 80 ? 'bg-green-500' : pct >= 50 ? 'bg-yellow-500' : 'bg-red-500'
  return (
    <div className="rounded-lg border border-border bg-surface p-4 flex flex-col gap-3">
      <div className="flex items-center justify-between">
        <span className="text-sm font-semibold text-primary">{t('bsi.checkSheet.umsetzungsgrad')}</span>
        <span className="text-2xl font-bold text-primary">{pct.toFixed(1)} %</span>
      </div>
      <div className="h-2 rounded-full bg-surface2 overflow-hidden">
        <div className={`h-full rounded-full transition-all ${color}`} style={{ width: `${pct}%` }} />
      </div>
      <div className="flex gap-3 text-xs text-secondary flex-wrap">
        <span className="text-green-400">✓ {t('bsi.checkSheet.jaCount', { count: ja })}</span>
        <span className="text-yellow-400">~ {t('bsi.checkSheet.teilweiseCount', { count: teilweise })}</span>
        <span className="text-red-400">✗ {t('bsi.checkSheet.neinCount', { count: nein })}</span>
        <span className="text-secondary">○ {t('bsi.checkSheet.entbehrlichCount', { count: entbehrlich })}</span>
        <span className="ml-auto">{t('bsi.checkSheet.total', { count: total })}</span>
      </div>
    </div>
  )
}

// ── Inline row editor ──────────────────────────────────────────────────────────

function CheckRow({ row, targetObjectId }: { row: BSICheckResult; targetObjectId: string }) {
  const { t } = useTranslation()
  const setResult = useSetBSICheckResult(targetObjectId)

  const statusLabels: Record<BSIUmsetzungsstatus, string> = {
    ja: t('bsi.status.ja'),
    teilweise: t('bsi.status.teilweise'),
    nein: t('bsi.status.nein'),
    entbehrlich: t('bsi.status.entbehrlich'),
  }

  const stufeLabels: Record<string, string> = {
    basis: t('bsi.stufe.basis'),
    standard: t('bsi.stufe.standard'),
    erhoeht: t('bsi.stufe.erhoeht'),
  }

  function handleStatusChange(val: string) {
    setResult.mutate({ anforderungId: row.anforderung_id, umsetzungsstatus: val as BSIUmsetzungsstatus })
  }

  return (
    <div className="flex items-center gap-3 px-4 py-2.5 border-b border-border last:border-0">
      <div className="w-[110px] shrink-0">
        <span className="text-[11px] font-mono text-secondary">{row.anforderung_id}</span>
      </div>
      <p className="flex-1 text-[13px] text-primary min-w-0">{row.anforderung_title}</p>
      {row.requirement_level && (
        <Badge className={`text-[10px] border shrink-0 ${STUFE_COLORS[row.requirement_level] ?? ''}`}>
          {stufeLabels[row.requirement_level] ?? row.requirement_level}
        </Badge>
      )}
      <div className="w-[140px] shrink-0">
        <Select
          value={row.umsetzungsstatus}
          onValueChange={handleStatusChange}
          disabled={setResult.isPending}
        >
          <SelectTrigger className="h-7 text-xs">
            <SelectValue>
              <Badge className={`text-[11px] border ${STATUS_COLORS[row.umsetzungsstatus]}`}>
                {statusLabels[row.umsetzungsstatus]}
              </Badge>
            </SelectValue>
          </SelectTrigger>
          <SelectContent>
            {(Object.keys(statusLabels) as BSIUmsetzungsstatus[]).map((s) => (
              <SelectItem key={s} value={s}>
                <Badge className={`text-[11px] border ${STATUS_COLORS[s]}`}>{statusLabels[s]}</Badge>
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
    </div>
  )
}

// ── Grouped by Baustein ────────────────────────────────────────────────────────

function BausteinGroup({ bausteinId, rows, targetObjectId, defaultOpen = true }: {
  bausteinId: string
  rows: BSICheckResult[]
  targetObjectId: string
  defaultOpen?: boolean
}) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(defaultOpen)
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
        <span className="text-xs text-secondary">{rows.length} {t('bsi.checkSheet.anforderungen')}</span>
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
  const { t } = useTranslation()
  const { id = '' } = useParams<{ id: string }>()
  const { data: obj } = useBSITargetObject(id)
  const { data: sheet = [], isLoading, isError, error } = useBSICheckSheet(id)
  const { data: summary } = useBSICheckSummary(id)
  const [stufeFilter, setStufeFilter] = useState<'all' | 'basis' | 'standard' | 'erhoeht'>('all')

  const VIRTUALIZE_THRESHOLD = 200

  const filteredSheet = useMemo(() => {
    if (stufeFilter === 'all') return sheet
    return sheet.filter((r) => r.requirement_level === stufeFilter)
  }, [sheet, stufeFilter])

  const grouped = useMemo(() => {
    const map = new Map<string, BSICheckResult[]>()
    for (const row of filteredSheet) {
      const list = map.get(row.baustein_id) ?? []
      list.push(row)
      map.set(row.baustein_id, list)
    }
    return map
  }, [filteredSheet])

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
    <ProGate error={isError ? error : null}>
    <div className="flex flex-col h-full">
      <PageHeader
        title={obj ? t('bsi.checkSheet.titleWithName', { name: obj.name }) : t('bsi.checkSheet.title')}
        description={t('bsi.checkSheet.description')}
        actions={
          <div className="flex items-center gap-2">
            <Select value={stufeFilter} onValueChange={(v) => { setStufeFilter(v as typeof stufeFilter) }}>
              <SelectTrigger className="h-8 text-xs w-[120px]">
                <SelectValue placeholder={t('bsi.stufe.all')} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">{t('bsi.stufe.all')}</SelectItem>
                <SelectItem value="basis">{t('bsi.stufe.basis')}</SelectItem>
                <SelectItem value="standard">{t('bsi.stufe.standard')}</SelectItem>
                <SelectItem value="erhoeht">{t('bsi.stufe.erhoeht')}</SelectItem>
              </SelectContent>
            </Select>
            <Button size="sm" variant="outline" onClick={handleDownloadCSV} disabled={sheet.length === 0}>
              <Download className="w-4 h-4 mr-1" />
              CSV
            </Button>
            <Link to="/vaktcomply/bsi/target-objects" className={cn(buttonVariants({ variant: 'ghost', size: 'sm' }))}>
              <ArrowLeft className="w-4 h-4 mr-1" />
              {t('bsi.checkSheet.back')}
            </Link>
          </div>
        }
      />

      <div className="p-6 space-y-4">
        {obj && (obj.effective_c === 'sehr_hoch' || obj.effective_i === 'sehr_hoch' || obj.effective_a === 'sehr_hoch') &&
          obj.absicherungsniveau !== 'kern' && (
          <div className="rounded-lg border border-orange-800 bg-orange-900/20 px-4 py-3 flex gap-3">
            <span className="text-orange-400 text-lg shrink-0">⚠</span>
            <div className="text-sm text-orange-200">
              <strong>{t('bsi.kernHint.title')}</strong>{' '}
              {t('bsi.kernHint.body', { niveau: obj.absicherungsniveau })}
            </div>
          </div>
        )}

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

        {isLoading && <p className="text-sm text-secondary">{t('bsi.checkSheet.loading')}</p>}

        {!isLoading && sheet.length === 0 && (
          <div className="rounded-lg border border-dashed border-border p-8 text-center space-y-2">
            <p className="text-sm font-medium text-primary">{t('bsi.checkSheet.emptyTitle')}</p>
            <p className="text-xs text-secondary">
              {t('bsi.checkSheet.emptyDescription')}
            </p>
            <Link to="/vaktcomply/bsi-modeling" className={cn(buttonVariants({ variant: 'outline', size: 'sm' }), 'mt-2')}>
              {t('bsi.checkSheet.toModeling')}
            </Link>
          </div>
        )}

        {filteredSheet.length > VIRTUALIZE_THRESHOLD && (
          <div className="rounded-lg border border-amber-800 bg-amber-900/15 px-4 py-2.5 text-xs text-amber-300">
            {t('bsi.checkSheet.manyRowsHint', { count: filteredSheet.length })}
          </div>
        )}

        {Array.from(grouped.entries()).map(([bid, rows]) => (
          <BausteinGroup
            key={bid}
            bausteinId={bid}
            rows={rows}
            targetObjectId={id}
            defaultOpen={filteredSheet.length <= VIRTUALIZE_THRESHOLD}
          />
        ))}
      </div>
    </div>
    </ProGate>
  )
}
