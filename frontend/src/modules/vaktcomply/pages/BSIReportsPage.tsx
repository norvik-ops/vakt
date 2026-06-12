// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

import { useTranslation } from 'react-i18next'
import { FileDown, Clock, Hash } from 'lucide-react'
import { PageHeader } from '../../../shared/components/PageHeader'
import { ProGate } from '../../../shared/components/ProGate'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { useBSIReportExports } from '../hooks/useBSICheck'
import { apiFetch } from '../../../api/client'
import type { BSIReportType } from '../types'

// ── Report definitions ─────────────────────────────────────────────────────────

interface ReportDef {
  type: BSIReportType
  title: string
  description: string
  article: string
}

// ── Download helper ────────────────────────────────────────────────────────────

async function downloadReport(type: BSIReportType) {
  const blob = await apiFetch<Blob>(`/vaktcomply/bsi/reports/${type}`, {
    headers: { Accept: 'application/pdf' },
  })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `bsi-bericht-${type}.pdf`
  a.click()
  URL.revokeObjectURL(url)
}

// ── Report Card ────────────────────────────────────────────────────────────────

function ReportCard({ def, lastExport }: { def: ReportDef; lastExport?: { sha256: string; created_at: string } }) {
  const { t } = useTranslation()
  return (
    <div className="rounded-lg border border-border bg-surface p-4 flex items-start gap-4">
      <div className="flex-1 min-w-0 space-y-1">
        <div className="flex items-center gap-2">
          <span className="text-sm font-semibold text-primary">{def.title}</span>
          <Badge className="text-[10px] bg-surface2 text-secondary border-transparent font-mono">
            {def.type}
          </Badge>
          <Badge className="text-[10px] bg-blue-900/20 text-blue-300 border-blue-800">
            {def.article}
          </Badge>
        </div>
        <p className="text-xs text-secondary">{def.description}</p>
        {lastExport && (
          <div className="flex items-center gap-3 text-[11px] text-secondary mt-1">
            <span className="flex items-center gap-1">
              <Clock className="w-3 h-3" />
              {new Date(lastExport.created_at).toLocaleString(undefined, {
                day: '2-digit', month: '2-digit', year: 'numeric',
                hour: '2-digit', minute: '2-digit',
              })}
            </span>
            <span className="flex items-center gap-1 font-mono">
              <Hash className="w-3 h-3" />
              {lastExport.sha256.slice(0, 12)}…
            </span>
          </div>
        )}
      </div>
      <Button
        size="sm"
        variant="outline"
        onClick={() => void downloadReport(def.type)}
        className="shrink-0"
      >
        <FileDown className="w-4 h-4 mr-1" />
        {t('bsi.reports.pdf')}
      </Button>
    </div>
  )
}

// ── Page ───────────────────────────────────────────────────────────────────────

export default function BSIReportsPage() {
  const { t } = useTranslation()
  const { data: exports = [], isError, error } = useBSIReportExports()

  const REPORTS: ReportDef[] = [
    {
      type: 'A1',
      title: t('bsi.reports.A1_title'),
      description: t('bsi.reports.A1_desc'),
      article: 'BSI 200-2 Anhang A.1',
    },
    {
      type: 'A2',
      title: t('bsi.reports.A2_title'),
      description: t('bsi.reports.A2_desc'),
      article: 'BSI 200-2 Anhang A.2',
    },
    {
      type: 'A3',
      title: t('bsi.reports.A3_title'),
      description: t('bsi.reports.A3_desc'),
      article: 'BSI 200-2 Anhang A.3',
    },
    {
      type: 'A4',
      title: t('bsi.reports.A4_title'),
      description: t('bsi.reports.A4_desc'),
      article: 'BSI 200-2 Anhang A.4',
    },
    {
      type: 'A5',
      title: t('bsi.reports.A5_title'),
      description: t('bsi.reports.A5_desc'),
      article: 'BSI 200-3 Anhang A.5',
    },
    {
      type: 'A6',
      title: t('bsi.reports.A6_title'),
      description: t('bsi.reports.A6_desc'),
      article: 'BSI 200-2 Anhang A.6',
    },
    {
      type: 'full',
      title: t('bsi.reports.full_title'),
      description: t('bsi.reports.full_desc'),
      article: 'BSI 200-2/200-3',
    },
  ]

  function lastExportFor(type: BSIReportType) {
    return exports
      .filter((e) => e.report_type === type)
      .sort((a, b) => b.created_at.localeCompare(a.created_at))[0]
  }

  return (
    <ProGate error={isError ? error : null}>
    <div className="flex flex-col h-full">
      <PageHeader
        title={t('bsi.reports.title')}
        description={t('bsi.reports.description')}
      />

      <div className="p-6 space-y-3">
        <div className="rounded-lg border border-blue-800/40 bg-blue-900/10 p-3 text-xs text-blue-300">
          {t('bsi.reports.infoText')}
        </div>

        {REPORTS.map((def) => (
          <ReportCard key={def.type} def={def} lastExport={lastExportFor(def.type)} />
        ))}

        {exports.length > 0 && (
          <div className="mt-4">
            <p className="text-xs font-semibold text-secondary uppercase tracking-wide mb-2">
              {t('bsi.reports.exportHistory')}
            </p>
            <div className="rounded-lg border border-border bg-surface divide-y divide-border">
              {exports.slice(0, 10).map((e) => (
                <div key={e.id} className="flex items-center gap-3 px-4 py-2.5 text-xs">
                  <Badge className="text-[10px] bg-surface2 text-secondary border-transparent font-mono shrink-0">
                    {e.report_type}
                  </Badge>
                  <span className="text-secondary font-mono truncate flex-1">
                    {e.sha256.slice(0, 20)}…
                  </span>
                  <span className="text-secondary shrink-0">
                    {new Date(e.created_at).toLocaleString(undefined, {
                      day: '2-digit', month: '2-digit', year: 'numeric',
                      hour: '2-digit', minute: '2-digit',
                    })}
                  </span>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
    </ProGate>
  )
}
