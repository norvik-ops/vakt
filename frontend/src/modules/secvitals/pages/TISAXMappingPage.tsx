import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { ArrowLeft } from 'lucide-react'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from '../../../components/ui/table'
import { PageHeader } from '../../../shared/components/PageHeader'
import { ProGate } from '../../../shared/components/ProGate'
import { useTISAXISOMapping } from '../hooks/useTISAXMapping'
import type { MappingResult } from '../types'

function CoveredBadge({ covered }: { covered: boolean }) {
  if (covered) {
    return (
      <Badge variant="success" className="text-xs">
        Abgedeckt
      </Badge>
    )
  }
  return (
    <Badge variant="destructive" className="text-xs">
      Lücke
    </Badge>
  )
}

function MappingRow({ result }: { result: MappingResult }) {
  return (
    <TableRow>
      <TableCell>
        <div className="flex flex-col gap-0.5">
          <span className="font-mono text-xs text-secondary">{result.tisax_control_id}</span>
          <span className="text-sm font-medium">{result.tisax_control_title}</span>
        </div>
      </TableCell>
      <TableCell>
        {result.iso_control_id ? (
          <div className="flex flex-col gap-0.5">
            <span className="font-mono text-xs text-secondary">{result.iso_control_id}</span>
            <span className="text-sm">{result.iso_control_title}</span>
          </div>
        ) : (
          <span className="text-xs text-secondary italic">Kein Mapping vorhanden</span>
        )}
      </TableCell>
      <TableCell>
        <CoveredBadge covered={result.covered} />
      </TableCell>
    </TableRow>
  )
}

export default function TISAXMappingPage() {
  const navigate = useNavigate()
  const [gapsOnly, setGapsOnly] = useState(false)

  const { data: results, isLoading, isError, error } = useTISAXISOMapping()

  const filtered = gapsOnly ? (results ?? []).filter((r) => !r.covered) : (results ?? [])

  const coveredCount = (results ?? []).filter((r) => r.covered).length
  const totalCount = (results ?? []).length
  const gapCount = totalCount - coveredCount

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="TISAX ↔ ISO 27001 Abgleich"
        description="Zeigt für jede TISAX-Maßnahme die entsprechende ISO 27001-Kontrolle und ob sie bereits abgedeckt ist."
        actions={
          <Button variant="outline" size="sm" onClick={() => navigate(-1)}>
            <ArrowLeft className="w-4 h-4 mr-1" />
            Zurück
          </Button>
        }
      />

      <div className="flex-1 p-6 space-y-6">
        {isError && <ProGate error={error}><div className="text-sm text-red-400 p-4 bg-red-500/10 rounded-lg">Fehler beim Laden des Mappings.</div></ProGate>}
        {/* Summary row */}
        {!isLoading && !isError && results && results.length > 0 && (
          <div className="flex items-center gap-4 flex-wrap">
            <div className="flex items-center gap-2 px-3 py-1.5 bg-surface border border-border rounded-md">
              <Badge variant="success" className="text-xs">
                {coveredCount}
              </Badge>
              <span className="text-xs text-secondary">Abgedeckt</span>
            </div>
            <div className="flex items-center gap-2 px-3 py-1.5 bg-surface border border-border rounded-md">
              <Badge variant="destructive" className="text-xs">
                {gapCount}
              </Badge>
              <span className="text-xs text-secondary">Lücken</span>
            </div>
            <div className="flex items-center gap-2 px-3 py-1.5 bg-surface border border-border rounded-md">
              <span className="text-xs text-secondary">
                Gesamt: <span className="font-medium text-primary">{totalCount}</span>
              </span>
            </div>
          </div>
        )}

        {/* Toggle: Nur Lücken anzeigen */}
        <div className="flex items-center gap-3">
          <button
            type="button"
            role="switch"
            aria-checked={gapsOnly}
            onClick={() => setGapsOnly((v) => !v)}
            className={`relative inline-flex h-5 w-9 items-center rounded-full transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
              gapsOnly ? 'bg-brand' : 'bg-border'
            }`}
          >
            <span
              className={`inline-block h-3 w-3 transform rounded-full bg-white shadow-sm transition-transform ${
                gapsOnly ? 'translate-x-5' : 'translate-x-1'
              }`}
            />
          </button>
          <label
            className="text-sm cursor-pointer select-none"
            onClick={() => setGapsOnly((v) => !v)}
          >
            Nur Lücken anzeigen
          </label>
        </div>

        {/* Content */}
        {isLoading ? (
          <div className="flex items-center justify-center h-32">
            <div className="w-5 h-5 border-2 border-brand border-t-transparent rounded-full animate-spin" />
          </div>
        ) : !results || results.length === 0 ? (
          <div className="flex items-center gap-3 p-4 bg-surface border border-border rounded-lg text-sm text-secondary">
            <span className="text-lg">ℹ</span>
            <span>
              ISO 27001 noch nicht aktiviert oder keine TISAX-Maßnahmen vorhanden. Aktiviere beide
              Frameworks, um den Abgleich zu starten.
            </span>
          </div>
        ) : filtered.length === 0 ? (
          <p className="text-sm text-secondary py-8 text-center">
            Alle TISAX-Maßnahmen sind durch ISO 27001 abgedeckt.
          </p>
        ) : (
          <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-1/3">TISAX Control</TableHead>
                  <TableHead className="w-1/3">ISO 27001 Control</TableHead>
                  <TableHead className="w-32">Abgedeckt</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filtered.map((result) => (
                  <MappingRow key={result.tisax_control_id} result={result} />
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </div>
    </div>
  )
}
