import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
import { ArrowLeft } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
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
  const { t } = useTranslation()
  if (covered) {
    return (
      <Badge variant="success" className="text-xs">
        {t('vaktcomply.tisaxMapping.coveredBadge')}
      </Badge>
    )
  }
  return (
    <Badge variant="destructive" className="text-xs">
      {t('vaktcomply.tisaxMapping.gapBadge')}
    </Badge>
  )
}

function MappingRow({ result }: { result: MappingResult }) {
  const { t } = useTranslation()
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
          <span className="text-xs text-secondary italic">{t('vaktcomply.tisaxMapping.noMapping')}</span>
        )}
      </TableCell>
      <TableCell>
        <CoveredBadge covered={result.covered} />
      </TableCell>
    </TableRow>
  )
}

export default function TISAXMappingPage() {
  const { t } = useTranslation()
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
        title={t('vaktcomply.tisaxMapping.title')}
        description={t('vaktcomply.tisaxMapping.description')}
        actions={
          <Button variant="outline" size="sm" onClick={() => { navigate(-1); }}>
            <ArrowLeft className="w-4 h-4 mr-1" />
            {t('vaktcomply.tisaxMapping.back')}
          </Button>
        }
      />

      <div className="flex-1 p-6 space-y-6">
        {isError && <ProGate error={error}><div className="text-sm text-red-400 p-4 bg-red-500/10 rounded-lg">{t('vaktcomply.tisaxMapping.loadError')}</div></ProGate>}
        {/* Summary row */}
        {!isLoading && !isError && results && results.length > 0 && (
          <div className="flex items-center gap-4 flex-wrap">
            <div className="flex items-center gap-2 px-3 py-1.5 bg-surface border border-border rounded-md">
              <Badge variant="success" className="text-xs">
                {coveredCount}
              </Badge>
              <span className="text-xs text-secondary">{t('vaktcomply.tisaxMapping.covered')}</span>
            </div>
            <div className="flex items-center gap-2 px-3 py-1.5 bg-surface border border-border rounded-md">
              <Badge variant="destructive" className="text-xs">
                {gapCount}
              </Badge>
              <span className="text-xs text-secondary">{t('vaktcomply.tisaxMapping.gaps')}</span>
            </div>
            <div className="flex items-center gap-2 px-3 py-1.5 bg-surface border border-border rounded-md">
              <span className="text-xs text-secondary">
                {t('vaktcomply.tisaxMapping.total', { count: totalCount })}
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
            onClick={() => { setGapsOnly((v) => !v); }}
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
            onClick={() => { setGapsOnly((v) => !v); }}
          >
            {t('vaktcomply.tisaxMapping.gapsOnlyLabel')}
          </label>
        </div>

        {/* Content */}
        {isLoading ? (
          <div className="flex items-center justify-center h-32">
            <Spinner size="md" />
          </div>
        ) : !results || results.length === 0 ? (
          <div className="flex items-center gap-3 p-4 bg-surface border border-border rounded-lg text-sm text-secondary">
            <span className="text-lg">ℹ</span>
            <span>{t('vaktcomply.tisaxMapping.noData')}</span>
          </div>
        ) : filtered.length === 0 ? (
          <p className="text-sm text-secondary py-8 text-center">
            {t('vaktcomply.tisaxMapping.allCovered')}
          </p>
        ) : (
          <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-1/3">{t('vaktcomply.tisaxMapping.colTISAX')}</TableHead>
                  <TableHead className="w-1/3">{t('vaktcomply.tisaxMapping.colISO')}</TableHead>
                  <TableHead className="w-32">{t('vaktcomply.tisaxMapping.colCovered')}</TableHead>
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
