// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

import { FileDown, Clock, Hash } from 'lucide-react'
import { PageHeader } from '../../../shared/components/PageHeader'
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

const REPORTS: ReportDef[] = [
  {
    type: 'A1',
    title: 'Strukturanalyse',
    description: 'Zielobjekte, Typen und Absicherungsniveaus',
    article: 'BSI 200-2 Anhang A.1',
  },
  {
    type: 'A2',
    title: 'Schutzbedarfsfeststellung',
    description: 'CIA-Bewertung je Zielobjekt (Maximalprinzip)',
    article: 'BSI 200-2 Anhang A.2',
  },
  {
    type: 'A3',
    title: 'Modellierung',
    description: 'Baustein-Zuweisung je Zielobjekt',
    article: 'BSI 200-2 Anhang A.3',
  },
  {
    type: 'A4',
    title: 'IT-Grundschutz-Check',
    description: 'Umsetzungsstatus je Anforderung und Begründungen',
    article: 'BSI 200-2 Anhang A.4',
  },
  {
    type: 'A5',
    title: 'Risikoanalyse',
    description: 'Risikobewertungen und Behandlungsmaßnahmen',
    article: 'BSI 200-3 Anhang A.5',
  },
  {
    type: 'A6',
    title: 'Realisierungsplan',
    description: 'Offene Maßnahmen und Priorisierung',
    article: 'BSI 200-2 Anhang A.6',
  },
  {
    type: 'full',
    title: 'Vollständiger Bericht',
    description: 'Alle Anhänge A1–A6 in einem Dokument',
    article: 'BSI 200-2/200-3',
  },
]

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
              {new Date(lastExport.created_at).toLocaleString('de-DE', {
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
        PDF
      </Button>
    </div>
  )
}

// ── Page ───────────────────────────────────────────────────────────────────────

export default function BSIReportsPage() {
  const { data: exports = [] } = useBSIReportExports()

  function lastExportFor(type: BSIReportType) {
    return exports
      .filter((e) => e.report_type === type)
      .sort((a, b) => b.created_at.localeCompare(a.created_at))[0]
  }

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="BSI-Referenzberichte"
        description="Anhänge A1–A6 gemäß BSI 200-2 / 200-3 als PDF exportieren"
      />

      <div className="p-6 space-y-3">
        <div className="rounded-lg border border-blue-800/40 bg-blue-900/10 p-3 text-xs text-blue-300">
          Die Berichte werden live aus den aktuellen Daten generiert. Der SHA-256-Hash jedes
          Exports wird für die Audit-Nachverfolgbarkeit gespeichert.
        </div>

        {REPORTS.map((def) => (
          <ReportCard key={def.type} def={def} lastExport={lastExportFor(def.type)} />
        ))}

        {exports.length > 0 && (
          <div className="mt-4">
            <p className="text-xs font-semibold text-secondary uppercase tracking-wide mb-2">
              Exportverlauf
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
                    {new Date(e.created_at).toLocaleString('de-DE', {
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
  )
}
